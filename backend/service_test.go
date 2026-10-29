package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func testApp(t *testing.T) core.App {
	t.Helper()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	register(app)
	return app
}

func testUser(t *testing.T, app core.App, email string, verified bool) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	r.SetEmail(email)
	r.SetVerified(verified)
	r.SetPassword("test-fixture-password-only")
	r.Set("name", strings.Split(email, "@")[0])
	if err = app.Save(r); err != nil {
		t.Fatal(err)
	}
	return r
}

func call(t *testing.T, app core.App, u *core.Record, action string, in Input) *State {
	t.Helper()
	out, err := execute(app, u, action, in)
	if err != nil {
		t.Fatal(action, err)
	}
	return out
}

func befriend(t *testing.T, app core.App, x, y *core.Record) {
	t.Helper()
	code := call(t, app, x, "state", Input{}).Code
	call(t, app, y, "befriend", Input{Code: code})
}

func intp(v int) *int { return &v }

func book(title string) *Book { return &Book{Title: title, Author: "Someone"} }

func TestFriendsSeeEachOthersPostsOnly(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)
	stranger := testUser(t, app, "stranger@example.com", true)
	unverified := testUser(t, app, "pending@example.com", false)

	if _, err := execute(app, unverified, "state", Input{}); !errors.Is(err, ErrAccess) {
		t.Fatal("unverified user allowed", err)
	}
	s := call(t, app, maya, "post", Input{Book: book("Piranesi"), Status: "reading", Progress: intp(40), Thoughts: "Tides!", CanLend: true})
	postID := s.Posts[0].ID
	if len(s.Shelf) != 1 || s.Shelf[0].Status != "reading" {
		t.Fatal("post did not add the book to the shelf", s.Shelf)
	}
	if len(call(t, app, theo, "state", Input{}).Posts) != 0 {
		t.Fatal("non-friend sees post")
	}
	if _, err := execute(app, stranger, "like", Input{ID: postID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("stranger liked a hidden post", err)
	}
	if _, err := execute(app, stranger, "befriend", Input{Code: "not-a-code"}); err == nil {
		t.Fatal("invalid code accepted")
	}

	befriend(t, app, maya, theo)
	befriend(t, app, maya, theo) // repeat is harmless
	s = call(t, app, theo, "like", Input{ID: postID})
	s = call(t, app, theo, "comment", Input{ID: postID, Text: "  Love it  "})
	if len(s.Friends) != 1 || len(s.Posts) != 1 || len(s.Posts[0].LikedBy) != 1 || s.Posts[0].Comments[0].Text != "Love it" {
		t.Fatalf("friend view wrong: %+v", s.Posts)
	}
	s = call(t, app, theo, "like", Input{ID: postID})
	if len(s.Posts[0].LikedBy) != 0 {
		t.Fatal("like did not toggle off")
	}
	if _, err := execute(app, theo, "post-delete", Input{ID: postID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("friend deleted someone else's post", err)
	}
	if _, err := execute(app, theo, "post", Input{Book: book("X"), Status: "reading", Progress: intp(101)}); err == nil {
		t.Fatal("progress over 100 accepted")
	}

	call(t, app, theo, "unfriend", Input{ID: maya.Id})
	if len(call(t, app, maya, "state", Input{}).Friends) != 0 {
		t.Fatal("unfriend is not mutual")
	}
}

func TestBorrowRequestBecomesSharedLoan(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)
	befriend(t, app, maya, theo)
	postID := call(t, app, maya, "post", Input{Book: book("Circe"), Status: "finished", Rating: intp(5), CanLend: true}).Posts[0].ID
	noLend := call(t, app, maya, "post", Input{Book: book("Mine"), Status: "finished"}).Posts[0].ID

	if _, err := execute(app, theo, "request", Input{ID: noLend}); err == nil {
		t.Fatal("requested a book that is not lendable")
	}
	if _, err := execute(app, maya, "request", Input{ID: postID}); err == nil {
		t.Fatal("requested own book")
	}
	s := call(t, app, theo, "request", Input{ID: postID})
	call(t, app, theo, "request", Input{ID: postID}) // idempotent
	if !s.Posts[1].Requested && !s.Posts[0].Requested {
		t.Fatal("request not reflected")
	}
	s = call(t, app, maya, "state", Input{})
	if len(s.Requests) != 1 || s.Requests[0].RequesterID != theo.Id {
		t.Fatal("owner does not see request", s.Requests)
	}
	if _, err := execute(app, theo, "respond", Input{ID: s.Requests[0].ID, Accept: true}); !errors.Is(err, ErrNotFound) {
		t.Fatal("requester accepted own request", err)
	}
	s = call(t, app, maya, "respond", Input{ID: s.Requests[0].ID, Accept: true})
	if len(s.Requests) != 0 || len(s.Loans) != 1 || s.Loans[0].Direction != "lent" || s.Loans[0].FriendID != theo.Id {
		t.Fatal("accept did not create a loan", s.Loans)
	}
	loanID := s.Loans[0].ID
	t2 := call(t, app, theo, "state", Input{})
	if len(t2.Loans) != 1 || t2.Loans[0].Direction != "borrowed" || t2.Loans[0].Book.Title != "Circe" {
		t.Fatal("borrower does not see loan", t2.Loans)
	}
	if _, err := execute(app, theo, "nudge", Input{ID: loanID}); err == nil {
		t.Fatal("borrower nudged")
	}
	if call(t, app, maya, "nudge", Input{ID: loanID}).Loans[0].NudgedAt == "" {
		t.Fatal("nudge not recorded")
	}
	if call(t, app, theo, "return", Input{ID: loanID}).Loans[0].ReturnedAt == "" {
		t.Fatal("borrower could not mark returned")
	}
	if call(t, app, maya, "unreturn", Input{ID: loanID}).Loans[0].ReturnedAt != "" {
		t.Fatal("undo return failed")
	}
	if _, err := execute(app, theo, "loan-delete", Input{ID: loanID}); err == nil {
		t.Fatal("non-creator deleted loan")
	}
	outsider := testUser(t, app, "outsider@example.com", true)
	if _, err := execute(app, outsider, "return", Input{ID: loanID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("outsider touched loan", err)
	}
}

func TestLoansWithContactsAndValidation(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	stranger := testUser(t, app, "stranger@example.com", true)
	if _, err := execute(app, maya, "loan", Input{Direction: "lent", Friend: stranger.Id, Book: book("A")}); err == nil {
		t.Fatal("loan to a non-friend user accepted")
	}
	if _, err := execute(app, maya, "loan", Input{Direction: "lent", Book: book("A")}); err == nil {
		t.Fatal("loan without a person accepted")
	}
	if _, err := execute(app, maya, "loan", Input{Direction: "lent", Contact: "Grandma", Book: book("A"), Due: "next week"}); err == nil {
		t.Fatal("bad due date accepted")
	}
	s := call(t, app, maya, "loan", Input{Direction: "borrowed", Contact: "Grandma", Book: book("Rebecca"), Due: "2026-12-01"})
	if s.Loans[0].Contact != "Grandma" || s.Loans[0].Direction != "borrowed" || s.Loans[0].DueDate != "2026-12-01" || !s.Loans[0].Mine {
		t.Fatal("contact loan wrong", s.Loans[0])
	}
	if _, err := execute(app, maya, "nudge", Input{ID: s.Loans[0].ID}); err == nil {
		t.Fatal("nudged someone outside the app")
	}
	if len(call(t, app, maya, "loan-delete", Input{ID: s.Loans[0].ID}).Loans) != 0 {
		t.Fatal("creator could not delete loan")
	}
}

func TestShelfAndTaggedNotesArePrivate(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)
	befriend(t, app, maya, theo)
	s := call(t, app, maya, "shelf-add", Input{Book: &Book{Title: "Dune", Author: "Frank Herbert", OLKey: "/works/OL1W", CoverID: 7}, Status: "want"})
	s = call(t, app, maya, "shelf-add", Input{Book: &Book{Title: "dune", Author: "frank herbert"}, Status: "reading"})
	if len(s.Shelf) != 1 || s.Shelf[0].Status != "reading" || s.Shelf[0].CoverID != 7 {
		t.Fatal("shelf did not deduplicate", s.Shelf)
	}
	bookID := s.Shelf[0].ID
	if _, err := execute(app, maya, "shelf-add", Input{Book: book("X"), Status: "lost"}); err == nil {
		t.Fatal("invalid status accepted")
	}
	s = call(t, app, maya, "note-save", Input{BookID: bookID, Text: "Fear is the mind-killer", Tags: []string{"#Quotes", "quotes", " sci fi "}})
	if len(s.Notes) != 1 || strings.Join(s.Notes[0].Tags, ",") != "quotes,sci-fi" || s.Notes[0].BookID != bookID {
		t.Fatal("note tags not normalized", s.Notes)
	}
	noteID := s.Notes[0].ID
	if _, err := execute(app, theo, "note-save", Input{ID: noteID, Text: "hijack"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("friend edited note", err)
	}
	if _, err := execute(app, theo, "note-save", Input{BookID: bookID, Text: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatal("friend linked note to someone else's book", err)
	}
	if ts := call(t, app, theo, "state", Input{}); len(ts.Notes) != 0 || len(ts.Shelf) != 0 {
		t.Fatal("notes or shelf leaked to friend")
	}
	tooMany := make([]string, 11)
	for i := range tooMany {
		tooMany[i] = string(rune('a' + i))
	}
	if _, err := execute(app, maya, "note-save", Input{Text: "x", Tags: tooMany}); err == nil {
		t.Fatal("too many tags accepted")
	}
	s = call(t, app, maya, "shelf-remove", Input{ID: bookID})
	if len(s.Shelf) != 0 || len(s.Notes) != 1 || s.Notes[0].BookID != "" {
		t.Fatal("removing a book should keep its notes unlinked", s.Notes)
	}
	if len(call(t, app, maya, "note-delete", Input{ID: noteID}).Notes) != 0 {
		t.Fatal("note not deleted")
	}
}

func testHandler(t *testing.T, app core.App) http.Handler {
	t.Helper()
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	se := &core.ServeEvent{App: app, Router: r}
	if err := app.OnServe().Trigger(se, func(e *core.ServeEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	h, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestHTTPRequiresAuthAndRejectsUnknownFields(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	token, err := maya.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	h := testHandler(t, app)
	do := func(path, body, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}
	if rr := do("/api/dogear/state", "{}", ""); rr.Code != 401 {
		t.Fatal("anonymous state allowed", rr.Code)
	}
	if rr := do("/api/dogear/state", `{"evil":1}`, token); rr.Code != 400 {
		t.Fatal("unknown field accepted", rr.Code)
	}
	rr := do("/api/dogear/state", "{}", token)
	var s State
	if rr.Code != 200 || json.Unmarshal(rr.Body.Bytes(), &s) != nil || s.Me.ID != maya.Id || s.Code == "" {
		t.Fatal("state failed", rr.Code, rr.Body.String())
	}
	for _, collection := range []string{"posts", "loans", "notes", "friendships"} {
		req := httptest.NewRequest("GET", "/api/collections/"+collection+"/records", nil)
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 403 {
			t.Fatal("direct collection access allowed:", collection, rec.Code)
		}
	}
}

func TestOpenLibrarySearchIsMappedAndCached(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if !strings.EqualFold(r.URL.Query().Get("q"), "piranesi") || r.Header.Get("User-Agent") == "" {
			t.Errorf("unexpected request %s", r.URL)
		}
		json.NewEncoder(w).Encode(map[string]any{"docs": []map[string]any{
			{"key": "/works/OL1W", "title": "Piranesi", "author_name": []string{"Susanna Clarke"}, "cover_i": 10, "first_publish_year": 2020},
			{"key": "/works/OL2W", "title": ""},
		}})
	}))
	defer server.Close()
	old := openLibraryBase
	openLibraryBase = server.URL
	defer func() { openLibraryBase = old }()

	books, err := searchBooks(context.Background(), "  Piranesi ")
	if err != nil || len(books) != 1 || books[0].Author != "Susanna Clarke" || books[0].CoverID != 10 || books[0].Year != 2020 {
		t.Fatal(books, err)
	}
	searchBooks(context.Background(), "piranesi")
	if calls != 1 {
		t.Fatal("search was not cached", calls)
	}
	if books, _ := searchBooks(context.Background(), "a"); len(books) != 0 || calls != 1 {
		t.Fatal("single-character search hit Open Library")
	}
}
