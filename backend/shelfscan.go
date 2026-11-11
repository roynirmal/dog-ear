package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/text/unicode/norm"
)

// Shelf scanning sends one photo through Gemini in four overlapping sections,
// asks for every visible book with a bounding box, then resolves each title
// against Google Books. The image itself is never stored.

const (
	maxShelfImage   = 12 << 20 // upload cap
	scanLongEdge    = 1600     // downscale before tiling to bound payload and cost
	scanTileScale   = 2        // upscale crops so spine text survives tokenisation
	scanMaxBooks    = 150
	scanMaxPerTile  = 120
	scanMaxOutput   = 8192
	scanTimeout     = 75 * time.Second
	scanGoogleLimit = 4 // concurrent catalogue lookups
)

var (
	geminiBase   = "https://generativelanguage.googleapis.com/v1beta"
	geminiModel  = "gemini-3.1-flash-lite"
	geminiClient = &http.Client{Timeout: 90 * time.Second}
)

type ScannedBook struct {
	Title      string     `json:"title"`
	Author     string     `json:"author"`
	OLKey      string     `json:"olKey,omitempty"`
	CoverID    int        `json:"coverId,omitempty"`
	CoverURL   string     `json:"coverUrl,omitempty"`
	Year       int        `json:"year,omitempty"`
	RawTitle   string     `json:"rawTitle"`
	RawAuthor  string     `json:"rawAuthor"`
	Confidence string     `json:"confidence"`
	BBox       [4]float64 `json:"bbox"` // [ymin, xmin, ymax, xmax], 0..1
	Matched    bool       `json:"matched"`
}

type ScanResult struct {
	Books      []ScannedBook `json:"books"`
	Unreadable int           `json:"unreadable"`
	Model      string        `json:"model"`
	ScansLeft  int           `json:"scansLeft"`
	Warning    string        `json:"warning,omitempty"`
}

type shelfTile struct {
	name   string
	x0, y0 float64
	x1, y1 float64
}

// Four overlapping sections, the same split used in the bookshelf experiment.
var shelfTiles = []shelfTile{
	{"top-left", 0.00, 0.02, 0.52, 0.35},
	{"top-right", 0.48, 0.02, 1.00, 0.35},
	{"bottom-left", 0.00, 0.37, 0.52, 0.91},
	{"bottom-right", 0.48, 0.37, 1.00, 0.91},
}

const scanPrompt = `You are cataloguing a photograph of a bookshelf for a personal library app.

Find every distinct physical book visible in the image, on both shelves. Include:
- upright books (read the spine);
- books lying flat or stacked horizontally (read the cover/spine edge);
- partially visible books at the edges, as long as a book is clearly present.

Do NOT include shelf boards, watermark text, or non-book objects.

For each book return:
- title: the title as printed. Use "" if no title is readable.
- author: author as printed, or "".
- shelf: "top" or "bottom".
- bbox_2d: the book's tight bounding box as [ymin, xmin, ymax, xmax],
  integers normalized to 0-1000 relative to the image.
- visible_text: the exact spine/cover text you used, so a human can audit it.
- confidence: "high" if the title is clearly readable, "medium" if partly
  readable/uncertain, "low" if guessed or only fragments are visible.

Rules:
- One entry per physical book. Do not emit duplicate boxes for one book.
- Ignore any watermark text and image captions.
- Never invent a title to fill a box; an unreadable book gets title "".
- Read titles in their original language.
- Keep bounding boxes inside 0-1000 and make them fit the book, not the shelf.

This image is one zoomed section of a larger bookshelf photo. Return bounding
boxes relative to THIS section only.`

var scanSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"books": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":        map[string]any{"type": "string"},
					"author":       map[string]any{"type": "string"},
					"shelf":        map[string]any{"type": "string", "enum": []string{"top", "bottom"}},
					"bbox_2d":      map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
					"visible_text": map[string]any{"type": "string"},
					"confidence":   map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
				},
				"required":         []string{"title", "author", "shelf", "bbox_2d", "visible_text", "confidence"},
				"propertyOrdering": []string{"title", "author", "shelf", "bbox_2d", "visible_text", "confidence"},
			},
		},
	},
	"required":         []string{"books"},
	"propertyOrdering": []string{"books"},
}

type rawScanBook struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	BBox       []int  `json:"bbox_2d"`
	Confidence string `json:"confidence"`
}

// scanShelf runs the whole image-to-books pipeline. Callers set the deadline.
func scanShelf(ctx context.Context, data []byte) (ScanResult, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return ScanResult{}, errors.New("shelf scanning is not configured: GEMINI_API_KEY is missing")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = geminiModel
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return ScanResult{}, errors.New("that image could not be read; use a JPEG, PNG or WebP photo")
	}
	if cfg.Width*cfg.Height > 32_000_000 {
		return ScanResult{}, errors.New("that photo is too large; use one under 32 megapixels")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ScanResult{}, errors.New("that image could not be read; use a JPEG, PNG or WebP photo")
	}
	img = downscaleImage(img, scanLongEdge)

	type tileOut struct {
		books []ScannedBook
		err   error
	}
	outs := make([]tileOut, len(shelfTiles))
	var wg sync.WaitGroup
	for i, tile := range shelfTiles {
		wg.Add(1)
		go func(i int, tile shelfTile) {
			defer wg.Done()
			crop := cropImage(img, tile)
			raw, err := callGeminiTile(ctx, key, model, crop)
			if err != nil {
				outs[i].err = err
				return
			}
			outs[i].books = mapTileBooks(raw, tile)
		}(i, tile)
	}
	wg.Wait()

	result := ScanResult{Model: model, Books: []ScannedBook{}}
	var all []ScannedBook
	var failures int
	var firstErr error
	for _, out := range outs {
		if out.err != nil {
			failures++
			if firstErr == nil {
				firstErr = out.err
			}
			continue
		}
		all = append(all, out.books...)
	}
	if failures == len(outs) {
		return result, firstErr
	}
	if failures > 0 {
		result.Warning = "One section could not be read. Check the list carefully."
	}

	merged := mergeScanned(all)
	for _, b := range merged {
		if strings.TrimSpace(b.Title) == "" {
			result.Unreadable++
			continue
		}
		result.Books = append(result.Books, b)
	}
	enrichScanned(ctx, result.Books)

	sort.SliceStable(result.Books, func(i, j int) bool {
		a, b := result.Books[i].BBox, result.Books[j].BBox
		if math.Abs(a[0]-b[0]) > 0.08 {
			return a[0] < b[0]
		}
		return a[1] < b[1]
	})
	if len(result.Books) > scanMaxBooks {
		result.Books = result.Books[:scanMaxBooks]
	}
	return result, nil
}

func downscaleImage(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}
	scale := float64(maxSide) / float64(max(w, h))
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

func cropImage(img image.Image, tile shelfTile) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rect := image.Rect(
		b.Min.X+int(tile.x0*float64(w)),
		b.Min.Y+int(tile.y0*float64(h)),
		b.Min.X+int(tile.x1*float64(w)),
		b.Min.Y+int(tile.y1*float64(h)),
	)
	if rect.Dx() < 1 || rect.Dy() < 1 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx()*scanTileScale, rect.Dy()*scanTileScale))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, rect, draw.Src, nil)
	return dst
}

type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiThinking struct {
	ThinkingLevel string `json:"thinkingLevel"`
}

type geminiConfig struct {
	Temperature      float64         `json:"temperature"`
	MaxOutputTokens  int             `json:"maxOutputTokens"`
	ResponseMIMEType string          `json:"responseMimeType"`
	ResponseSchema   any             `json:"responseSchema"`
	MediaResolution  string          `json:"mediaResolution,omitempty"`
	ThinkingConfig   *geminiThinking `json:"thinkingConfig,omitempty"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiConfig    `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func callGeminiTile(ctx context.Context, key, model string, img image.Image) ([]rawScanBook, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	cfg := geminiConfig{
		Temperature:      0.1,
		MaxOutputTokens:  scanMaxOutput,
		ResponseMIMEType: "application/json",
		ResponseSchema:   scanSchema,
	}
	if strings.HasPrefix(model, "gemini-3") {
		cfg.MediaResolution = "MEDIA_RESOLUTION_HIGH"
		cfg.ThinkingConfig = &geminiThinking{ThinkingLevel: "low"}
	}
	body := geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: "You are a precise OCR and object-detection assistant. You never invent book titles."}}},
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{
				{InlineData: &geminiInline{MIMEType: "image/jpeg", Data: base64.StdEncoding.EncodeToString(buf.Bytes())}},
				{Text: scanPrompt},
			},
		}},
		GenerationConfig: cfg,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/models/%s:generateContent", strings.TrimRight(geminiBase, "/"), model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", key)
	resp, err := geminiClient.Do(req)
	if err != nil {
		return nil, errors.New("the book scanner is unavailable right now")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, errors.New("the book scanner returned an unreadable response")
	}
	var out geminiResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, errors.New("the book scanner returned an unreadable response")
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, errors.New("the book scanner is rate limiting right now; try again in a minute")
		}
		return nil, fmt.Errorf("the book scanner failed (HTTP %d)", resp.StatusCode)
	}
	if len(out.Candidates) == 0 {
		return nil, errors.New("the book scanner returned no result")
	}
	var text strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		text.WriteString(p.Text)
	}
	var parsed struct {
		Books []rawScanBook `json:"books"`
	}
	if err := json.Unmarshal([]byte(text.String()), &parsed); err != nil {
		return nil, errors.New("the book scanner returned an unexpected result")
	}
	return parsed.Books, nil
}

func mapTileBooks(raw []rawScanBook, tile shelfTile) []ScannedBook {
	out := make([]ScannedBook, 0, len(raw))
	for _, b := range raw {
		if len(out) >= scanMaxPerTile {
			break
		}
		if len(b.BBox) != 4 {
			continue
		}
		v := [4]int{}
		for i := range v {
			v[i] = b.BBox[i]
			if v[i] < 0 {
				v[i] = 0
			}
			if v[i] > 1000 {
				v[i] = 1000
			}
		}
		if v[2] <= v[0] || v[3] <= v[1] {
			continue
		}
		title := strings.TrimSpace(b.Title)
		author := strings.TrimSpace(b.Author)
		confidence := b.Confidence
		if confidence != "high" && confidence != "medium" {
			confidence = "low"
		}
		out = append(out, ScannedBook{
			Title:      title,
			Author:     author,
			RawTitle:   title,
			RawAuthor:  author,
			Confidence: confidence,
			BBox: [4]float64{
				tile.y0 + float64(v[0])/1000*(tile.y1-tile.y0),
				tile.x0 + float64(v[1])/1000*(tile.x1-tile.x0),
				tile.y0 + float64(v[2])/1000*(tile.y1-tile.y0),
				tile.x0 + float64(v[3])/1000*(tile.x1-tile.x0),
			},
		})
	}
	return out
}

// mergeScanned drops overlapping boxes from adjacent tiles, preferring an entry
// that has a title and a higher confidence.
func mergeScanned(books []ScannedBook) []ScannedBook {
	sort.SliceStable(books, func(i, j int) bool {
		if (books[i].Title == "") != (books[j].Title == "") {
			return books[i].Title != ""
		}
		return confidenceRank(books[i].Confidence) < confidenceRank(books[j].Confidence)
	})
	kept := make([]ScannedBook, 0, len(books))
	for _, b := range books {
		duplicate := false
		for _, k := range kept {
			if bboxIoU(b.BBox, k.BBox) > 0.55 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			kept = append(kept, b)
		}
	}
	return kept
}

func confidenceRank(c string) int {
	switch c {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}

func bboxIoU(a, b [4]float64) float64 {
	iy0 := math.Max(a[0], b[0])
	ix0 := math.Max(a[1], b[1])
	iy1 := math.Min(a[2], b[2])
	ix1 := math.Min(a[3], b[3])
	inter := math.Max(0, iy1-iy0) * math.Max(0, ix1-ix0)
	if inter == 0 {
		return 0
	}
	areaA := (a[2] - a[0]) * (a[3] - a[1])
	areaB := (b[2] - b[0]) * (b[3] - b[1])
	union := areaA + areaB - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

// enrichScanned is best-effort: a failed catalogue lookup leaves the raw OCR
// text in place so the reader can still review and correct it.
func enrichScanned(ctx context.Context, books []ScannedBook) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < scanGoogleLimit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					return
				}
				b := &books[idx]
				match, ok := matchGoogleBook(ctx, b.RawTitle, b.RawAuthor)
				if !ok {
					continue
				}
				b.Title = match.Title
				b.Author = match.Author
				b.OLKey = match.OLKey
				b.CoverID = match.CoverID
				b.CoverURL = match.CoverURL
				b.Year = match.Year
				b.Matched = true
			}
		}()
	}
	for i := range books {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func matchGoogleBook(ctx context.Context, title, author string) (Book, bool) {
	// First try a precise title+author query. If the author was misread this
	// finds nothing, so fall back to a title-only query with a stricter bar.
	if author != "" {
		query := fmt.Sprintf(`intitle:"%s" inauthor:"%s"`, escapeQuery(title), escapeQuery(author))
		if candidates, err := searchGoogleBooks(ctx, query); err == nil {
			if book, ok := bestGoogleCandidate(title, author, candidates, 0.72); ok {
				return book, true
			}
		}
	}
	query := fmt.Sprintf(`intitle:"%s"`, escapeQuery(title))
	candidates, err := searchGoogleBooks(ctx, query)
	if err != nil {
		return Book{}, false
	}
	book, ok := bestGoogleCandidate(title, author, candidates, 0.85)
	if !ok {
		return Book{}, false
	}
	authorAgrees := author != "" && book.Author != "" && titleScore(author, book.Author) >= 0.30
	// A title-only match can collide with an unrelated book of the same name.
	// Without an author agreement, require a distinctive multi-word title.
	if !authorAgrees && len(strings.Fields(normScanText(title))) < 4 {
		return Book{}, false
	}
	// A title-only match can still carry a wrong author; keep the reader's text.
	if author != "" && book.Author != "" && !authorAgrees {
		book.Author = author
	}
	return book, true
}

func bestGoogleCandidate(title, author string, candidates []Book, threshold float64) (Book, bool) {
	best := -1
	bestScore := 0.0
	for i, c := range candidates {
		score := matchScore(title, c.Title)
		if author != "" && c.Author != "" && titleScore(author, c.Author) >= 0.30 {
			score += 0.06
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	if best < 0 || bestScore < threshold {
		return Book{}, false
	}
	return candidates[best], true
}

func escapeQuery(s string) string {
	s = strings.ReplaceAll(s, `"`, " ")
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

func normScanText(s string) string {
	s = strings.ToLower(norm.NFD.String(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// matchScore is stricter than titleScore: a short title contained in a much
// longer one is a different book, not a subtitle match.
func matchScore(a, b string) float64 {
	x, y := normScanText(a), normScanText(b)
	if x == "" || y == "" {
		return 0
	}
	if x == y {
		return 1
	}
	shorter, longer := x, y
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	if len(shorter) >= 4 && float64(len(shorter)) >= 0.7*float64(len(longer)) && strings.Contains(longer, shorter) {
		return 0.95
	}
	xt, yt := strings.Fields(x), strings.Fields(y)
	seen := map[string]bool{}
	for _, t := range xt {
		seen[t] = true
	}
	inter := 0
	for _, t := range yt {
		if seen[t] {
			inter++
		}
	}
	union := len(xt) + len(yt) - inter
	jaccard := 0.0
	if union > 0 {
		jaccard = float64(inter) / float64(union)
	}
	xr, yr := []rune(x), []rune(y)
	longest := len(xr)
	if len(yr) > longest {
		longest = len(yr)
	}
	lev := 0.0
	if longest > 0 {
		lev = 1 - float64(levenshtein(xr, yr))/float64(longest)
	}
	return math.Max(jaccard, lev)
}

func titleScore(a, b string) float64 {
	x, y := normScanText(a), normScanText(b)
	if x == "" || y == "" {
		return 0
	}
	if x == y {
		return 1
	}
	if len(x) >= 4 && (strings.Contains(x, y) || strings.Contains(y, x)) {
		return 0.95
	}
	xt, yt := strings.Fields(x), strings.Fields(y)
	seen := map[string]bool{}
	for _, t := range xt {
		seen[t] = true
	}
	inter := 0
	for _, t := range yt {
		if seen[t] {
			inter++
		}
	}
	union := len(xt) + len(yt) - inter
	jaccard := 0.0
	if union > 0 {
		jaccard = float64(inter) / float64(union)
	}
	xr, yr := []rune(x), []rune(y)
	longest := len(xr)
	if len(yr) > longest {
		longest = len(yr)
	}
	lev := 0.0
	if longest > 0 {
		lev = 1 - float64(levenshtein(xr, yr))/float64(longest)
	}
	return math.Max(jaccard, lev)
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = minInt(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func minInt(vs ...int) int {
	m := vs[0]
	for _, v := range vs[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

// shelfScanHandler accepts a multipart photo and returns candidates for review.
func shelfScanHandler(e *core.RequestEvent) error {
	ctx, cancel := context.WithTimeout(e.Request.Context(), scanTimeout)
	defer cancel()
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, maxShelfImage)
	// Stream the upload into memory instead of ParseMultipartForm, which can
	// spill large parts to temporary files.
	reader, err := e.Request.MultipartReader()
	if err != nil {
		return apis.NewBadRequestError("Choose a bookshelf photo first.", nil)
	}
	var data []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return apis.NewBadRequestError("That photo could not be read.", nil)
		}
		if part.FormName() != "image" {
			part.Close()
			continue
		}
		data, err = io.ReadAll(io.LimitReader(part, maxShelfImage+1))
		part.Close()
		if err != nil {
			return apis.NewBadRequestError("That photo could not be read.", nil)
		}
		break
	}
	if len(data) == 0 {
		return apis.NewBadRequestError("Choose a bookshelf photo first.", nil)
	}
	if len(data) > maxShelfImage {
		return apis.NewBadRequestError("That photo is too large. Use one under 12 MB.", nil)
	}
	day, scansLeft, err := reserveScan(e.App, e.Auth.Id)
	if errors.Is(err, errScanLimit) {
		return apis.NewApiError(http.StatusTooManyRequests,
			"You’ve reached today’s limit of 10 shelf scans. Try again tomorrow.", nil)
	}
	if err != nil {
		return apis.NewApiError(http.StatusInternalServerError, "Could not check your shelf scan limit. Try again.", nil)
	}
	result, err := scanShelf(ctx, data)
	if err != nil {
		refundScan(e.App, e.Auth.Id, day)
		if strings.Contains(err.Error(), "not configured") {
			return apis.NewApiError(http.StatusServiceUnavailable, err.Error(), nil)
		}
		if ctx.Err() != nil {
			return apis.NewApiError(http.StatusGatewayTimeout, "The book scanner took too long. Try again with fewer books in frame.", nil)
		}
		return apis.NewApiError(http.StatusBadGateway, err.Error(), nil)
	}
	result.ScansLeft = scansLeft
	return e.JSON(http.StatusOK, result)
}
