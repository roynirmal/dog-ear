package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testShelfJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestScanShelfMapsTilesAndUsesGoogle(t *testing.T) {
	gemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
			t.Errorf("missing api key header: %q", got)
		}
		var body struct {
			Contents []struct {
				Parts []struct {
					InlineData *struct {
						MIMEType string `json:"mime_type"`
						Data     string `json:"data"`
					} `json:"inline_data"`
				} `json:"parts"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 2 || body.Contents[0].Parts[0].InlineData == nil {
			t.Errorf("unexpected Gemini request shape")
		}
		book, _ := json.Marshal(map[string]any{"books": []map[string]any{{
			"title": "Dune", "author": "Frank Herbert", "shelf": "top",
			"bbox_2d": []int{100, 100, 900, 900}, "visible_text": "DUNE FRANK HERBERT",
			"confidence": "high",
		}}})
		json.NewEncoder(w).Encode(map[string]any{"candidates": []map[string]any{{
			"content": map[string]any{"parts": []map[string]any{{"text": string(book)}}},
		}}})
	}))
	defer gemini.Close()
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{
			"id": "dune-id",
			"volumeInfo": map[string]any{
				"title": "Dune", "authors": []string{"Frank Herbert"},
				"publishedDate": "1965", "imageLinks": map[string]any{"thumbnail": "https://books.google.com/cover.jpg"},
			},
		}}})
	}))
	defer google.Close()

	oldGemini, oldGoogle, oldModel := geminiBase, googleBooksBase, geminiModel
	geminiBase, googleBooksBase = gemini.URL, google.URL
	defer func() { geminiBase, googleBooksBase, geminiModel = oldGemini, oldGoogle, oldModel }()
	t.Setenv("GEMINI_API_KEY", "test-key")

	result, err := scanShelf(context.Background(), testShelfJPEG(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Books) != 4 {
		t.Fatalf("expected one book per tile, got %d", len(result.Books))
	}
	for _, b := range result.Books {
		if !b.Matched || b.OLKey != "gb:dune-id" || b.Title != "Dune" || b.Author != "Frank Herbert" {
			t.Fatalf("book was not enriched: %+v", b)
		}
		if b.BBox[0] < 0 || b.BBox[2] > 1 || b.BBox[1] < 0 || b.BBox[3] > 1 {
			t.Fatalf("bbox outside the image: %+v", b.BBox)
		}
		if b.BBox[2] <= b.BBox[0] || b.BBox[3] <= b.BBox[1] {
			t.Fatalf("degenerate bbox: %+v", b.BBox)
		}
	}
	if result.Model != "gemini-3.1-flash-lite" {
		t.Fatalf("unexpected model %q", result.Model)
	}
}

func TestScanShelfRequiresKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := scanShelf(context.Background(), testShelfJPEG(t)); err == nil {
		t.Fatal("scan without a key should fail")
	}
}

func TestMergeScannedPrefersTitledOverlap(t *testing.T) {
	box := [4]float64{0.1, 0.1, 0.5, 0.2}
	books := []ScannedBook{
		{Title: "", Confidence: "high", BBox: box},
		{Title: "Dune", Confidence: "medium", BBox: box},
		{Title: "Other", Confidence: "high", BBox: [4]float64{0.1, 0.6, 0.5, 0.7}},
	}
	merged := mergeScanned(books)
	if len(merged) != 2 {
		t.Fatalf("unexpected merge: %+v", merged)
	}
	found := false
	for _, b := range merged {
		if b.Title == "Dune" {
			found = true
		}
		if b.Title == "" {
			t.Fatalf("untitled duplicate survived: %+v", merged)
		}
	}
	if !found {
		t.Fatalf("titled entry was dropped: %+v", merged)
	}
}

func TestTitleScore(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64
	}{
		{"Dune", "Dune", 0.99},
		{"Hur vi ser på varandra", "HUR VISER PA VARANDRA", 0.85},
		{"The Complete Sherlock Holmes", "The Complete Sherlock Holmes", 0.99},
		{"Dune", "Neuromancer", 0.0},
	}
	for _, c := range cases {
		if got := titleScore(c.a, c.b); got < c.min {
			t.Errorf("titleScore(%q, %q) = %.2f, want >= %.2f", c.a, c.b, got, c.min)
		}
	}
}

func TestShelfAddMany(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	books := []Book{
		{Title: "Dune", Author: "Frank Herbert"},
		{Title: "Circe", Author: "Madeline Miller"},
		{Title: "dune", Author: "frank herbert"},
	}
	s := call(t, app, maya, "shelf-add-many", Input{Books: books, Status: "want"})
	if len(s.Shelf) != 2 {
		t.Fatalf("expected two deduplicated books, got %d", len(s.Shelf))
	}
	if _, err := execute(app, maya, "shelf-add-many", Input{Books: []Book{{Title: "X"}}, Status: "lost"}); err == nil {
		t.Fatal("invalid status accepted")
	}
	if _, err := execute(app, maya, "shelf-add-many", Input{}); err == nil {
		t.Fatal("empty list accepted")
	}
}

func TestShelfScanHTTP(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	token, err := maya.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	gemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		book, _ := json.Marshal(map[string]any{"books": []map[string]any{{
			"title": "Dune", "author": "Frank Herbert", "shelf": "top",
			"bbox_2d": []int{100, 100, 900, 900}, "visible_text": "DUNE", "confidence": "high",
		}}})
		json.NewEncoder(w).Encode(map[string]any{"candidates": []map[string]any{{
			"content": map[string]any{"parts": []map[string]any{{"text": string(book)}}},
		}}})
	}))
	defer gemini.Close()
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}})
	}))
	defer google.Close()
	oldGemini, oldGoogle := geminiBase, googleBooksBase
	geminiBase, googleBooksBase = gemini.URL, google.URL
	defer func() { geminiBase, googleBooksBase = oldGemini, oldGoogle }()
	t.Setenv("GEMINI_API_KEY", "test-key")

	h := testHandler(t, app)
	upload := func(auth string, withImage bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if withImage {
			part, err := writer.CreateFormFile("image", "shelf.jpg")
			if err != nil {
				t.Fatal(err)
			}
			part.Write(testShelfJPEG(t))
		}
		writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/dogear/shelf-scan", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	if rr := upload("", true); rr.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous scan allowed: %d", rr.Code)
	}
	if rr := upload(token, false); rr.Code != http.StatusBadRequest {
		t.Fatalf("scan without a photo: %d", rr.Code)
	}
	rr := upload(token, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("scan failed: %d %s", rr.Code, rr.Body.String())
	}
	var result ScanResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Books) != 4 || result.Books[0].Title != "Dune" {
		t.Fatalf("unexpected scan result: %+v", result)
	}
	if result.ScansLeft != scanDailyLimit-1 {
		t.Fatalf("scansLeft = %d, want %d", result.ScansLeft, scanDailyLimit-1)
	}

	// The successful scan used one; claim the rest and expect a 429.
	for i := 0; i < scanDailyLimit-1; i++ {
		if _, _, err := reserveScan(app, maya.Id); err != nil {
			t.Fatal(err)
		}
	}
	if rr := upload(token, true); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit scan status = %d, want 429", rr.Code)
	}
}

func TestScanQuota(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)

	var day string
	for i := 0; i < scanDailyLimit; i++ {
		var left int
		var err error
		day, left, err = reserveScan(app, maya.Id)
		if err != nil {
			t.Fatalf("scan %d: %v", i+1, err)
		}
		if want := scanDailyLimit - i - 1; left != want {
			t.Fatalf("scan %d left = %d, want %d", i+1, left, want)
		}
	}
	if _, _, err := reserveScan(app, maya.Id); !errors.Is(err, errScanLimit) {
		t.Fatalf("expected the daily limit, got %v", err)
	}
	refundScan(app, maya.Id, day)
	if _, left, err := reserveScan(app, maya.Id); err != nil || left != 0 {
		t.Fatalf("refunded scan not reusable: left=%d err=%v", left, err)
	}
	if _, left, err := reserveScan(app, theo.Id); err != nil || left != scanDailyLimit-1 {
		t.Fatalf("quota leaked across users: left=%d err=%v", left, err)
	}
}
