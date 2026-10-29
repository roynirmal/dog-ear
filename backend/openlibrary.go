package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Open Library asks API clients to identify themselves, which a browser cannot
// do, so searches go through the backend. Results are cached briefly in memory.
var openLibraryBase = "https://openlibrary.org"
var openLibraryClient = &http.Client{Timeout: 10 * time.Second}

const searchCacheSize = 200

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

	params := url.Values{"q": {query}, "limit": {"12"}, "fields": {"key,title,author_name,cover_i,first_publish_year"}}
	req, err := http.NewRequestWithContext(ctx, "GET", openLibraryBase+"/search.json?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Dog-ear/0.1 (self-hosted reading app)")
	res, err := openLibraryClient.Do(req)
	if err != nil {
		return nil, errors.New("Open Library is unreachable; enter the book by hand")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Open Library search failed; enter the book by hand")
	}
	var body struct {
		Docs []struct {
			Key     string   `json:"key"`
			Title   string   `json:"title"`
			Authors []string `json:"author_name"`
			Cover   int      `json:"cover_i"`
			Year    int      `json:"first_publish_year"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 2<<20)).Decode(&body); err != nil {
		return nil, errors.New("Open Library returned an unexpected response")
	}
	books := []Book{}
	for _, d := range body.Docs {
		b, err := cleanBook(&Book{Title: d.Title, Author: strings.Join(d.Authors, ", "), OLKey: d.Key, CoverID: d.Cover, Year: d.Year})
		if err != nil {
			continue // skip oversized or malformed entries rather than failing the search
		}
		books = append(books, b)
	}
	searchCache.Lock()
	if len(searchCache.m) >= searchCacheSize {
		searchCache.m = map[string]searchEntry{}
	}
	searchCache.m[key] = searchEntry{time.Now(), books}
	searchCache.Unlock()
	return books, nil
}
