package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Book search queries Open Library and Google Books together. Open Library is
// the main catalogue; Google Books fills in titles it lacks, such as many
// self-published and Kindle-only books. Either may fail without failing the
// search. Open Library asks API clients to identify themselves, which a browser
// cannot do, so searches go through the backend. Results are cached briefly.
var openLibraryBase = "https://openlibrary.org"
var googleBooksBase = "https://www.googleapis.com"
var searchClient = &http.Client{Timeout: 10 * time.Second}

const (
	searchCacheSize = 200
	searchLimit     = 12 // per catalogue
	mergedLimit     = 16
	userAgent       = "Dog-ear/0.1 (self-hosted reading app)"
)

type searchEntry struct {
	at    time.Time
	books []Book
}

var searchCache = struct {
	sync.Mutex
	m map[string]searchEntry
}{m: map[string]searchEntry{}}

func searchBooks(ctx context.Context, query string) ([]Book, error) {
	query = strings.Join(strings.Fields(query), " ")
	if utf8.RuneCountInString(query) < 2 {
		return []Book{}, nil
	}
	if utf8.RuneCountInString(query) > 200 {
		return nil, errors.New("search is too long")
	}
	key := strings.ToLower(query)
	searchCache.Lock()
	if e, ok := searchCache.m[key]; ok && time.Since(e.at) < time.Hour {
		searchCache.Unlock()
		return e.books, nil
	}
	searchCache.Unlock()

	var wg sync.WaitGroup
	var ol, gb []Book
	var olErr, gbErr error
	wg.Add(2)
	go func() { defer wg.Done(); ol, olErr = searchOpenLibrary(ctx, query) }()
	go func() { defer wg.Done(); gb, gbErr = searchGoogleBooks(ctx, query) }()
	wg.Wait()
	if olErr != nil && gbErr != nil {
		return nil, errors.New("book search is unavailable right now; enter the book by hand")
	}
	books := mergeResults(ol, gb)
	// Cache only complete answers so a source that was briefly down is retried.
	if olErr == nil && gbErr == nil {
		searchCache.Lock()
		if len(searchCache.m) >= searchCacheSize {
			searchCache.m = map[string]searchEntry{}
		}
		searchCache.m[key] = searchEntry{time.Now(), books}
		searchCache.Unlock()
	}
	return books, nil
}

// mergeResults alternates between the two catalogues so each one's best
// matches appear near the top, dropping duplicates (same title and first
// author) and stopping at the merged limit. A common title can fill one
// catalogue's whole page with other books, so appending would hide the other's.
func mergeResults(primary, extra []Book) []Book {
	books := []Book{}
	seen := map[string]bool{}
	sameBook := func(b Book) string {
		author, _, _ := strings.Cut(b.Author, ",")
		return strings.ToLower(strings.TrimSpace(b.Title)) + "|" + strings.ToLower(strings.TrimSpace(author))
	}
	add := func(b Book) {
		if k := sameBook(b); !seen[k] && len(books) < mergedLimit {
			seen[k] = true
			books = append(books, b)
		}
	}
	for i := 0; i < len(primary) || i < len(extra); i++ {
		if i < len(primary) {
			add(primary[i])
		}
		if i < len(extra) {
			add(extra[i])
		}
	}
	return books
}

func fetchJSON(ctx context.Context, rawURL string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := searchClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New(res.Status)
	}
	return json.NewDecoder(http.MaxBytesReader(nil, res.Body, 2<<20)).Decode(v)
}

func searchOpenLibrary(ctx context.Context, query string) ([]Book, error) {
	params := url.Values{"q": {query}, "limit": {strconv.Itoa(searchLimit)}, "fields": {"key,title,author_name,cover_i,first_publish_year"}}
	var body struct {
		Docs []struct {
			Key     string   `json:"key"`
			Title   string   `json:"title"`
			Authors []string `json:"author_name"`
			Cover   int      `json:"cover_i"`
			Year    int      `json:"first_publish_year"`
		} `json:"docs"`
	}
	if err := fetchJSON(ctx, openLibraryBase+"/search.json?"+params.Encode(), &body); err != nil {
		return nil, err
	}
	books := []Book{}
	for _, d := range body.Docs {
		b, err := cleanBook(&Book{Title: d.Title, Author: strings.Join(d.Authors, ", "), OLKey: d.Key, CoverID: d.Cover, Year: d.Year})
		if err != nil {
			continue // skip oversized or malformed entries rather than failing the search
		}
		books = append(books, b)
	}
	return books, nil
}

// searchGoogleBooks uses GOOGLE_BOOKS_API_KEY when set; without it, requests
// share Google's small anonymous quota.
func searchGoogleBooks(ctx context.Context, query string) ([]Book, error) {
	params := url.Values{"q": {query}, "maxResults": {"10"}, "printType": {"books"},
		"fields": {"items(id,volumeInfo(title,authors,publishedDate,imageLinks/thumbnail))"}}
	if key := os.Getenv("GOOGLE_BOOKS_API_KEY"); key != "" {
		params.Set("key", key)
	}
	var body struct {
		Items []struct {
			ID   string `json:"id"`
			Info struct {
				Title     string   `json:"title"`
				Authors   []string `json:"authors"`
				Published string   `json:"publishedDate"`
				Images    struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}
	if err := fetchJSON(ctx, googleBooksBase+"/books/v1/volumes?"+params.Encode(), &body); err != nil {
		return nil, err
	}
	books := []Book{}
	for _, it := range body.Items {
		year := 0
		if len(it.Info.Published) >= 4 {
			year, _ = strconv.Atoi(it.Info.Published[:4])
		}
		cover := strings.Replace(it.Info.Images.Thumbnail, "http://", "https://", 1)
		cover = strings.Replace(cover, "&edge=curl", "", 1)
		b, err := cleanBook(&Book{Title: it.Info.Title, Author: strings.Join(it.Info.Authors, ", "), OLKey: googleKeyPrefix + it.ID, CoverURL: cover, Year: year})
		if err != nil {
			continue
		}
		books = append(books, b)
	}
	return books, nil
}
