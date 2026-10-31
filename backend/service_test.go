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

func TestBorrowFromShelfBecomesSharedLoan(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)
	befriend(t, app, maya, theo)
	s := call(t, app, maya, "post", Input{Book: book("Circe"), Status: "finished", Rating: intp(5), CanLend: true})
	call(t, app, maya, "post", Input{Book: book("Mine"), Status: "finished"})
	call(t, app, maya, "shelf-add", Input{Book: book("Wishlist"), Status: "want"})
	s = call(t, app, maya, "state", Input{})
	shelf := map[string]ShelfBook{}
	for _, b := range s.Shelf {
		shelf[b.Title] = b
	}
	if !shelf["Circe"].Lendable || shelf["Mine"].Lendable || shelf["Wishlist"].Lendable {
		t.Fatal("lendable defaults wrong", s.Shelf)
	}
	circe := shelf["Circe"].ID

	ts := call(t, app, theo, "state", Input{})
	if len(ts.Library) != 1 || ts.Library[0].ID != circe || !ts.Library[0].Available {
		t.Fatal("friend should see only lendable books", ts.Library)
	}
	for _, p := range ts.Posts {
		if p.CanLend != (p.Book.Title == "Circe") {
			t.Fatal("post lendability wrong", p)
		}
	}
	if _, err := execute(app, theo, "request", Input{ID: shelf["Mine"].ID}); err == nil {
		t.Fatal("requested a book that is not lendable")
	}
	if _, err := execute(app, maya, "request", Input{ID: circe}); !errors.Is(err, ErrNotFound) {
		t.Fatal("requested own book", err)
	}
	outsider := testUser(t, app, "outsider@example.com", true)
	if _, err := execute(app, outsider, "request", Input{ID: circe}); !errors.Is(err, ErrNotFound) {
		t.Fatal("non-friend requested", err)
	}
	ts = call(t, app, theo, "request", Input{ID: circe})
	call(t, app, theo, "request", Input{ID: circe}) // idempotent
	if !ts.Library[0].Requested {
		t.Fatal("request not reflected")
	}
	s = call(t, app, maya, "state", Input{})
	if len(s.Requests) != 1 || s.Requests[0].RequesterID != theo.Id || s.Requests[0].BookID != circe {
		t.Fatal("owner does not see request", s.Requests)
	}
	if _, err := execute(app, theo, "respond", Input{ID: s.Requests[0].ID, Accept: true}); !errors.Is(err, ErrNotFound) {
		t.Fatal("requester accepted own request", err)
	}
	s = call(t, app, maya, "respond", Input{ID: s.Requests[0].ID, Accept: true})
	if len(s.Requests) != 0 || len(s.Loans) != 1 || s.Loans[0].Direction != "lent" || s.Loans[0].FriendID != theo.Id || s.Loans[0].ShelfBookID != circe {
		t.Fatal("accept did not create a loan", s.Loans)
	}
	loanID := s.Loans[0].ID
	if _, err := execute(app, maya, "loan", Input{Direction: "lent", Contact: "Sam", Book: book("Circe"), ShelfBook: circe}); err == nil {
		t.Fatal("lent the same copy twice")
	}
	ts = call(t, app, theo, "state", Input{})
	if len(ts.Loans) != 1 || ts.Loans[0].Direction != "borrowed" || ts.Loans[0].Book.Title != "Circe" {
		t.Fatal("borrower does not see loan", ts.Loans)
	}
	if ts.Library[0].Available || ts.Posts[len(ts.Posts)-1].CanLend {
		t.Fatal("lent-out book still offered")
	}
	if _, err := execute(app, theo, "request", Input{ID: circe}); err == nil {
		t.Fatal("requested a lent-out book")
	}
	if _, err := execute(app, theo, "nudge", Input{ID: loanID}); err == nil {
		t.Fatal("borrower nudged")
	}
	if call(t, app, maya, "nudge", Input{ID: loanID}).Loans[0].NudgedAt == "" {
		t.Fatal("nudge not recorded")
	}
	if call(t, app, theo, "return", Input{ID: loanID}).Library[0].Available != true {
		t.Fatal("returned book not available again")
	}
	if call(t, app, maya, "unreturn", Input{ID: loanID}).Loans[0].ReturnedAt != "" {
		t.Fatal("undo return failed")
	}
	if _, err := execute(app, theo, "loan-delete", Input{ID: loanID}); err == nil {
		t.Fatal("non-creator deleted loan")
	}
	if _, err := execute(app, outsider, "return", Input{ID: loanID}); !errors.Is(err, ErrNotFound) {
		t.Fatal("outsider touched loan", err)
	}
	f := false
	call(t, app, maya, "shelf-update", Input{ID: shelf["Wishlist"].ID, Lendable: &f})
	tr := true
	if ts = call(t, app, maya, "shelf-update", Input{ID: shelf["Mine"].ID, Lendable: &tr}); !func() bool {
		for _, b := range ts.Shelf {
			if b.Title == "Mine" {
				return b.Lendable && b.Status == "finished"
			}
		}
		return false
	}() {
		t.Fatal("lendable toggle failed or changed status")
	}
}

func TestFriendRequestsBetweenFriendsOfFriends(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)
	june := testUser(t, app, "june@example.com", true)
	stranger := testUser(t, app, "stranger@example.com", true)
	befriend(t, app, maya, theo)
	befriend(t, app, maya, june)
	postID := call(t, app, maya, "post", Input{Book: book("Piranesi"), Status: "finished"}).Posts[0].ID
	call(t, app, june, "comment", Input{ID: postID, Text: "Loved it"})

	ts := call(t, app, theo, "state", Input{})
	found := false
	for _, p := range ts.People {
		found = found || p.ID == june.Id
	}
	if !found {
		t.Fatal("commenter missing from people")
	}
	if _, err := execute(app, stranger, "friend-request", Input{ID: theo.Id}); !errors.Is(err, ErrNotFound) {
		t.Fatal("request without a mutual friend", err)
	}
	ts = call(t, app, theo, "friend-request", Input{ID: june.Id})
	call(t, app, theo, "friend-request", Input{ID: june.Id}) // repeat is harmless
	if len(ts.SentRequests) != 1 || ts.SentRequests[0] != june.Id {
		t.Fatal("sent request not listed", ts.SentRequests)
	}
	js := call(t, app, june, "state", Input{})
	if len(js.FriendRequests) != 1 || js.FriendRequests[0].FromID != theo.Id {
		t.Fatal("incoming request not listed", js.FriendRequests)
	}
	if _, err := execute(app, theo, "friend-respond", Input{ID: js.FriendRequests[0].ID, Accept: true}); !errors.Is(err, ErrNotFound) {
		t.Fatal("sender accepted own request", err)
	}
	js = call(t, app, june, "friend-respond", Input{ID: js.FriendRequests[0].ID, Accept: true})
	if len(js.Friends) != 2 || len(js.FriendRequests) != 0 {
		t.Fatal("accept did not befriend", js.Friends)
	}
	if ts = call(t, app, theo, "state", Input{}); len(ts.SentRequests) != 0 || len(ts.Friends) != 2 {
		t.Fatal("friendship not settled for sender")
	}

	// Asking someone who already asked you accepts; declining is quiet.
	sam := testUser(t, app, "sam@example.com", true)
	befriend(t, app, june, sam)
	call(t, app, sam, "friend-request", Input{ID: theo.Id})
	if len(call(t, app, theo, "friend-request", Input{ID: sam.Id}).FriendRequests) != 0 || !areFriends(app, theo.Id, sam.Id) {
		t.Fatal("mutual request did not befriend")
	}
	call(t, app, sam, "friend-request", Input{ID: maya.Id})
	ms := call(t, app, maya, "state", Input{})
	call(t, app, maya, "friend-respond", Input{ID: ms.FriendRequests[0].ID, Accept: false})
	if ss := call(t, app, sam, "friend-request", Input{ID: maya.Id}); len(ss.SentRequests) != 1 {
		t.Fatal("declined request should still read as sent")
	}
	if len(call(t, app, maya, "state", Input{}).FriendRequests) != 0 {
		t.Fatal("declined request came back")
	}
}

func TestStartedReadingNotifiesOncePerFriendPerBook(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)
	june := testUser(t, app, "june@example.com", true)
	befriend(t, app, maya, theo)
	befriend(t, app, maya, june)
	dune := &Book{Title: "Dune", Author: "Frank Herbert", OLKey: "/works/OL1W"}
	call(t, app, maya, "post", Input{Book: dune, Status: "reading", Progress: intp(5)})
	call(t, app, maya, "post", Input{Book: dune, Status: "reading", Progress: intp(50)})
	call(t, app, maya, "post", Input{Book: dune, Status: "finished"})
	call(t, app, maya, "post", Input{Book: book("Emma"), Status: "finished"})
	ts := call(t, app, theo, "state", Input{})
	if len(ts.Notifications) != 1 || ts.Notifications[0].ActorID != maya.Id || ts.Notifications[0].Book.Title != "Dune" || ts.Notifications[0].Read {
		t.Fatal("expected one unread notification", ts.Notifications)
	}
	if len(call(t, app, maya, "state", Input{}).Notifications) != 0 {
		t.Fatal("author notified about own post")
	}
	if ts = call(t, app, theo, "notifications-read", Input{}); !ts.Notifications[0].Read {
		t.Fatal("not marked read")
	}
	if len(call(t, app, june, "state", Input{}).Notifications) != 1 {
		t.Fatal("other friend not notified")
	}
	// Theo reading Dune notifies Maya separately: it is per friend, not per book.
	call(t, app, theo, "post", Input{Book: dune, Status: "reading", Progress: intp(1)})
	if len(call(t, app, maya, "state", Input{}).Notifications) != 1 {
		t.Fatal("friend reading the same book did not notify")
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
	for _, collection := range []string{"posts", "loans", "notes", "friendships", "friend_requests", "notifications"} {
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

func TestReadingMonthsFollowStatusAndCanBeEdited(t *testing.T) {
	app := testApp(t)
	maya := testUser(t, app, "maya@example.com", true)
	theo := testUser(t, app, "theo@example.com", true)
	befriend(t, app, maya, theo)
	now := thisMonth()
	str := func(s string) *string { return &s }
	shelf := func(s *State, title string) ShelfBook {
		for _, b := range s.Shelf {
			if b.Title == title {
				return b
			}
		}
		t.Fatal("missing", title)
		return ShelfBook{}
	}

	s := call(t, app, maya, "shelf-add", Input{Book: book("Dune"), Status: "want"})
	if b := shelf(s, "Dune"); b.Started != "" || b.Finished != "" {
		t.Fatal("want-to-read book has dates", b)
	}
	s = call(t, app, maya, "post", Input{Book: book("Dune"), Status: "reading", Progress: intp(5)})
	if b := shelf(s, "Dune"); b.Started != now || b.Finished != "" || s.Posts[0].Started != now {
		t.Fatal("starting to read should stamp this month", b, s.Posts[0])
	}
	id := shelf(s, "Dune").ID
	s = call(t, app, maya, "shelf-update", Input{ID: id, Started: str("2026-01")})
	s = call(t, app, maya, "post", Input{Book: book("Dune"), Status: "reading", Progress: intp(60)})
	if b := shelf(s, "Dune"); b.Started != "2026-01" {
		t.Fatal("a progress update must keep the start month", b)
	}
	s = call(t, app, maya, "post", Input{Book: book("Dune"), Status: "finished", Rating: intp(5)})
	if b := shelf(s, "Dune"); b.Started != "2026-01" || b.Finished != now {
		t.Fatal("finishing should stamp the finish month", b)
	}
	if ts := call(t, app, theo, "state", Input{}); ts.Posts[0].Started != "2026-01" || ts.Posts[0].Finished != now {
		t.Fatal("friends should see the reading months on posts", ts.Posts[0])
	}
	if _, err := execute(app, maya, "shelf-update", Input{ID: id, Started: str("2026-02"), Finished: str("2025-12")}); err == nil {
		t.Fatal("finish before start accepted")
	}
	if _, err := execute(app, maya, "shelf-update", Input{ID: id, Finished: str("2999-01")}); err == nil {
		t.Fatal("future month accepted")
	}
	if _, err := execute(app, maya, "shelf-update", Input{ID: id, Started: str("Jan 2026")}); err == nil {
		t.Fatal("bad month format accepted")
	}
	s = call(t, app, maya, "shelf-update", Input{ID: id, Started: str("")})
	if b := shelf(s, "Dune"); b.Started != "" || b.Finished != now {
		t.Fatal("clearing the start month failed", b)
	}

	// Finishing straight away can record both months; re-reading starts over.
	s = call(t, app, maya, "shelf-add", Input{Book: book("Emma"), Status: "finished", Started: str("2025-06"), Finished: str("2025-08")})
	if b := shelf(s, "Emma"); b.Started != "2025-06" || b.Finished != "2025-08" {
		t.Fatal("explicit months not saved", b)
	}
	s = call(t, app, maya, "shelf-update", Input{ID: shelf(s, "Emma").ID, Status: "reading"})
	if b := shelf(s, "Emma"); b.Started != now || b.Finished != "" {
		t.Fatal("re-reading should start over", b)
	}
	s = call(t, app, maya, "shelf-update", Input{ID: shelf(s, "Emma").ID, Status: "want"})
	if b := shelf(s, "Emma"); b.Started != "" || b.Finished != "" {
		t.Fatal("want to read should clear dates", b)
	}
}
