package main

import (
	"crypto/rand"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

var ErrAccess = errors.New("access denied")
var ErrNotFound = errors.New("not found")

const (
	maxTitle    = 300
	maxAuthor   = 200
	maxName     = 120
	maxThoughts = 5000
	maxComment  = 1000
	maxNote     = 10000
	maxTags     = 10
	maxTagLen   = 32
	feedLimit   = 200
)

var shelfStatuses = map[string]bool{"want": true, "reading": true, "finished": true}

type Book struct {
	Title   string `json:"title"`
	Author  string `json:"author"`
	OLKey   string `json:"olKey,omitempty"`
	CoverID int    `json:"coverId,omitempty"`
	Year    int    `json:"year,omitempty"`
}

type Input struct {
	ID        string   `json:"id"`
	BookID    string   `json:"bookId"`
	ShelfBook string   `json:"shelfBook"`
	Code      string   `json:"code"`
	Book      *Book    `json:"book"`
	Status    string   `json:"status"`
	Lendable  *bool    `json:"lendable"`
	Progress  *int     `json:"progress"`
	Rating    *int     `json:"rating"`
	Thoughts  string   `json:"thoughts"`
	CanLend   bool     `json:"canLend"`
	Text      string   `json:"text"`
	Tags      []string `json:"tags"`
	Direction string   `json:"direction"`
	Friend    string   `json:"friend"`
	Contact   string   `json:"contact"`
	Due       string   `json:"due"`
	Accept    bool     `json:"accept"`
}

func randomCode() string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 16)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func stamp(r *core.Record, field string) string {
	return r.GetDateTime(field).Time().UTC().Format(time.RFC3339)
}

func clean(s string, max int, label string, required bool) (string, error) {
	s = strings.TrimSpace(s)
	if required && s == "" {
		return "", errors.New(label + " is required")
	}
	if utf8.RuneCountInString(s) > max {
		return "", errors.New(label + " is too long")
	}
	return s, nil
}

func cleanBook(b *Book) (Book, error) {
	if b == nil {
		return Book{}, errors.New("book is required")
	}
	var err error
	out := *b
	if out.Title, err = clean(out.Title, maxTitle, "title", true); err != nil {
		return out, err
	}
	if out.Author, err = clean(out.Author, maxAuthor, "author", false); err != nil {
		return out, err
	}
	if out.OLKey, err = clean(out.OLKey, 64, "Open Library key", false); err != nil {
		return out, err
	}
	if out.CoverID < 0 || out.Year < 0 || out.Year > 3000 {
		return out, errors.New("invalid book details")
	}
	return out, nil
}

// bookKey identifies "the same book" across people: the Open Library work when
// known, otherwise title and author.
func bookKey(b Book) string {
	if b.OLKey != "" {
		return b.OLKey
	}
	return strings.ToLower(b.Title) + "|" + strings.ToLower(b.Author)
}

func cleanTags(tags []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimLeft(strings.TrimSpace(t), "#"))
		t = strings.Join(strings.Fields(t), "-")
		if t == "" || seen[t] {
			continue
		}
		if utf8.RuneCountInString(t) > maxTagLen {
			return nil, errors.New("tags must be 32 characters or fewer")
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) > maxTags {
		return nil, errors.New("a note can have at most 10 tags")
	}
	return out, nil
}

func cleanDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", errors.New("due date must be YYYY-MM-DD")
	}
	return s, nil
}

func displayName(u *core.Record) string {
	if name := u.GetString("name"); name != "" {
		return name
	}
	return strings.Split(u.Email(), "@")[0]
}

func anyIDs(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func friendIDs(app core.App, uid string) ([]string, error) {
	rs, err := app.FindAllRecords("friendships", dbx.Or(dbx.HashExp{"a": uid}, dbx.HashExp{"b": uid}))
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, r := range rs {
		if r.GetString("a") == uid {
			ids = append(ids, r.GetString("b"))
		} else {
			ids = append(ids, r.GetString("a"))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func pair(x, y string) (string, string) {
	if x < y {
		return x, y
	}
	return y, x
}

func areFriends(app core.App, x, y string) bool {
	a, b := pair(x, y)
	r, err := app.FindFirstRecordByFilter("friendships", "a = {:a} && b = {:b}", dbx.Params{"a": a, "b": b})
	return err == nil && r != nil
}

// haveMutualFriend limits friend requests to people you can actually come
// across in the app: commenters and authors on posts you can see.
func haveMutualFriend(app core.App, x, y string) (bool, error) {
	xs, err := friendIDs(app, x)
	if err != nil {
		return false, err
	}
	ys, err := friendIDs(app, y)
	if err != nil {
		return false, err
	}
	seen := map[string]bool{}
	for _, id := range xs {
		seen[id] = true
	}
	for _, id := range ys {
		if seen[id] {
			return true, nil
		}
	}
	return false, nil
}

func makeFriends(app core.App, x, y string) error {
	if x == y {
		return nil
	}
	if !areFriends(app, x, y) {
		r, err := newRecord(app, "friendships")
		if err != nil {
			return err
		}
		a, b := pair(x, y)
		r.Set("a", a)
		r.Set("b", b)
		if err := app.Save(r); err != nil {
			return err
		}
	}
	// Any requests between the two are settled by the friendship.
	pending, err := app.FindAllRecords("friend_requests", dbx.Or(
		dbx.HashExp{"from": x, "to": y}, dbx.HashExp{"from": y, "to": x}))
	if err != nil {
		return err
	}
	for _, r := range pending {
		if err := app.Delete(r); err != nil {
			return err
		}
	}
	return nil
}

func find(app core.App, collection, id string) (*core.Record, error) {
	if id == "" {
		return nil, ErrNotFound
	}
	r, err := app.FindRecordById(collection, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return r, nil
}

// visiblePost returns a post the user wrote or a friend shared.
func visiblePost(app core.App, uid, id string) (*core.Record, error) {
	r, err := find(app, "posts", id)
	if err != nil {
		return nil, err
	}
	if owner := r.GetString("owner"); owner != uid && !areFriends(app, uid, owner) {
		return nil, ErrNotFound
	}
	return r, nil
}

func owned(app core.App, collection, field, uid, id string) (*core.Record, error) {
	r, err := find(app, collection, id)
	if err != nil {
		return nil, err
	}
	if r.GetString(field) != uid {
		return nil, ErrNotFound
	}
	return r, nil
}

func decodeBook(r *core.Record) Book {
	var b Book
	_ = r.UnmarshalJSONField("book", &b)
	return b
}

func shelfBookOf(r *core.Record) Book {
	return Book{r.GetString("title"), r.GetString("author"), r.GetString("olKey"), r.GetInt("coverId"), r.GetInt("year")}
}

func newRecord(app core.App, collection string) (*core.Record, error) {
	c, err := app.FindCachedCollectionByNameOrId(collection)
	if err != nil {
		return nil, err
	}
	return core.NewRecord(c), nil
}

// findShelf returns the owner's copy of a book, matched by Open Library key or
// case-insensitive title and author.
func findShelf(app core.App, owner string, b Book) (*core.Record, error) {
	rs, err := app.FindAllRecords("books", dbx.HashExp{"owner": owner})
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		sameKey := b.OLKey != "" && r.GetString("olKey") == b.OLKey
		sameText := strings.EqualFold(r.GetString("title"), b.Title) && strings.EqualFold(r.GetString("author"), b.Author)
		if sameKey || sameText {
			return r, nil
		}
	}
	return nil, nil
}

// upsertShelf adds or updates a shelf book. lendable nil keeps an existing
// setting and defaults new books to lendable unless they are only wanted.
func upsertShelf(app core.App, uid string, b Book, status string, lendable *bool) (*core.Record, error) {
	r, err := findShelf(app, uid, b)
	if err != nil {
		return nil, err
	}
	if r == nil {
		if r, err = newRecord(app, "books"); err != nil {
			return nil, err
		}
		r.Set("owner", uid)
		r.Set("title", b.Title)
		r.Set("author", b.Author)
		r.Set("olKey", b.OLKey)
		r.Set("coverId", b.CoverID)
		r.Set("year", b.Year)
		r.Set("lendable", status != "want")
	}
	if lendable != nil {
		r.Set("lendable", *lendable)
	}
	r.Set("status", status)
	return r, app.Save(r)
}

func lentOut(app core.App, shelfBookID string) bool {
	if shelfBookID == "" {
		return false
	}
	r, err := app.FindFirstRecordByFilter("loans", "shelfBook = {:b} && returned = ''", dbx.Params{"b": shelfBookID})
	return err == nil && r != nil
}

// notifyStartedReading tells each current friend, once per book, that the
// author started reading it.
func notifyStartedReading(app core.App, author string, post *core.Record, b Book) error {
	friends, err := friendIDs(app, author)
	if err != nil {
		return err
	}
	key := bookKey(b)
	for _, friend := range friends {
		existing, _ := app.FindFirstRecordByFilter("notifications", "user = {:u} && actor = {:a} && bookKey = {:k}",
			dbx.Params{"u": friend, "a": author, "k": key})
		if existing != nil {
			continue
		}
		n, err := newRecord(app, "notifications")
		if err != nil {
			return err
		}
		n.Set("user", friend)
		n.Set("actor", author)
		n.Set("post", post.Id)
		n.Set("bookKey", key)
		if err := app.Save(n); err != nil {
			return err
		}
	}
	return nil
}

func deleteAll(app core.App, collection string, where dbx.Expression) error {
	rs, err := app.FindAllRecords(collection, where)
	if err != nil {
		return err
	}
	for _, r := range rs {
		if err := app.Delete(r); err != nil {
			return err
		}
	}
	return nil
}

func ensureCode(app core.App, u *core.Record) error {
	if u.GetString("code") != "" {
		return nil
	}
	fresh, err := app.FindRecordById("users", u.Id)
	if err != nil {
		return err
	}
	if fresh.GetString("code") == "" {
		fresh.Set("code", randomCode())
		if err := app.Save(fresh); err != nil {
			return err
		}
	}
	u.Set("code", fresh.GetString("code"))
	return nil
}

func execute(app core.App, u *core.Record, action string, in Input) (*State, error) {
	if u == nil || u.Collection().Name != "users" || !u.Verified() {
		return nil, ErrAccess
	}
	var state *State
	err := app.RunInTransaction(func(tx core.App) error {
		if err := ensureCode(tx, u); err != nil {
			return err
		}
		if err := mutate(tx, u, action, in); err != nil {
			return err
		}
		var err error
		state, err = load(tx, u)
		return err
	})
	return state, err
}

func mutate(app core.App, u *core.Record, action string, in Input) error {
	uid := u.Id
	switch action {
	case "state":
		return nil

	case "befriend":
		code := strings.TrimSpace(in.Code)
		if code == "" || len(code) > 64 {
			return errors.New("invite code is required")
		}
		other, err := app.FindFirstRecordByFilter("users", "code = {:code}", dbx.Params{"code": code})
		if err != nil || other == nil {
			return errors.New("this invite link is not valid")
		}
		return makeFriends(app, uid, other.Id) // own link or a repeat is harmless

	case "unfriend":
		a, b := pair(uid, in.ID)
		r, err := app.FindFirstRecordByFilter("friendships", "a = {:a} && b = {:b}", dbx.Params{"a": a, "b": b})
		if err != nil || r == nil {
			return ErrNotFound
		}
		return app.Delete(r)

	case "friend-request":
		other, err := find(app, "users", in.ID)
		if err != nil || other.Id == uid {
			return ErrNotFound
		}
		if areFriends(app, uid, other.Id) {
			return nil
		}
		mutual, err := haveMutualFriend(app, uid, other.Id)
		if err != nil {
			return err
		}
		if !mutual {
			return ErrNotFound
		}
		// They already asked you: asking back is accepting.
		reverse, err := app.FindAllRecords("friend_requests", dbx.HashExp{"from": other.Id, "to": uid, "status": "pending"})
		if err != nil {
			return err
		}
		if len(reverse) > 0 {
			return makeFriends(app, uid, other.Id)
		}
		existing, err := app.FindAllRecords("friend_requests", dbx.HashExp{"from": uid, "to": other.Id})
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			return nil // pending, or quietly declined
		}
		r, err := newRecord(app, "friend_requests")
		if err != nil {
			return err
		}
		r.Set("from", uid)
		r.Set("to", other.Id)
		r.Set("status", "pending")
		return app.Save(r)

	case "friend-respond":
		r, err := owned(app, "friend_requests", "to", uid, in.ID)
		if err != nil || r.GetString("status") != "pending" {
			return ErrNotFound
		}
		if in.Accept {
			return makeFriends(app, uid, r.GetString("from"))
		}
		r.Set("status", "declined")
		return app.Save(r)

	case "shelf-add":
		b, err := cleanBook(in.Book)
		if err != nil {
			return err
		}
		if !shelfStatuses[in.Status] {
			return errors.New("invalid shelf status")
		}
		_, err = upsertShelf(app, uid, b, in.Status, in.Lendable)
		return err

	case "shelf-update":
		r, err := owned(app, "books", "owner", uid, in.ID)
		if err != nil {
			return err
		}
		if in.Status == "" && in.Lendable == nil {
			return errors.New("nothing to update")
		}
		if in.Status != "" {
			if !shelfStatuses[in.Status] {
				return errors.New("invalid shelf status")
			}
			r.Set("status", in.Status)
		}
		if in.Lendable != nil {
			r.Set("lendable", *in.Lendable)
		}
		return app.Save(r)

	case "shelf-remove":
		r, err := owned(app, "books", "owner", uid, in.ID)
		if err != nil {
			return err
		}
		// Notes outlive the shelf entry; they simply lose their book link.
		notes, err := app.FindAllRecords("notes", dbx.HashExp{"owner": uid, "book": r.Id})
		if err != nil {
			return err
		}
		for _, n := range notes {
			n.Set("book", "")
			if err := app.Save(n); err != nil {
				return err
			}
		}
		if err := deleteAll(app, "requests", dbx.HashExp{"book": r.Id, "status": "pending"}); err != nil {
			return err
		}
		return app.Delete(r)

	case "note-save":
		text, err := clean(in.Text, maxNote, "note", true)
		if err != nil {
			return err
		}
		tags, err := cleanTags(in.Tags)
		if err != nil {
			return err
		}
		bookID := strings.TrimSpace(in.BookID)
		if bookID != "" {
			if _, err := owned(app, "books", "owner", uid, bookID); err != nil {
				return err
			}
		}
		var r *core.Record
		if in.ID != "" {
			if r, err = owned(app, "notes", "owner", uid, in.ID); err != nil {
				return err
			}
		} else {
			if r, err = newRecord(app, "notes"); err != nil {
				return err
			}
			r.Set("owner", uid)
		}
		r.Set("text", text)
		r.Set("tags", tags)
		r.Set("book", bookID)
		return app.Save(r)

	case "note-delete":
		r, err := owned(app, "notes", "owner", uid, in.ID)
		if err != nil {
			return err
		}
		return app.Delete(r)

	case "post":
		b, err := cleanBook(in.Book)
		if err != nil {
			return err
		}
		thoughts, err := clean(in.Thoughts, maxThoughts, "thoughts", false)
		if err != nil {
			return err
		}
		r, err := newRecord(app, "posts")
		if err != nil {
			return err
		}
		switch in.Status {
		case "reading":
			if in.Progress == nil || *in.Progress < 0 || *in.Progress > 100 {
				return errors.New("progress must be between 0 and 100")
			}
			r.Set("progress", *in.Progress)
		case "finished":
			if in.Rating != nil {
				if *in.Rating < 1 || *in.Rating > 5 {
					return errors.New("rating must be between 1 and 5")
				}
				r.Set("rating", *in.Rating)
			}
		default:
			return errors.New("invalid reading status")
		}
		// A book you read belongs on your shelf; "friends can borrow my copy"
		// is that shelf book's lending setting.
		shelf, err := upsertShelf(app, uid, b, in.Status, &in.CanLend)
		if err != nil {
			return err
		}
		r.Set("owner", uid)
		r.Set("book", b)
		r.Set("status", in.Status)
		r.Set("thoughts", thoughts)
		r.Set("canLend", in.CanLend)
		r.Set("shelfBook", shelf.Id)
		if err := app.Save(r); err != nil {
			return err
		}
		if in.Status == "reading" {
			return notifyStartedReading(app, uid, r, b)
		}
		return nil

	case "post-delete":
		r, err := owned(app, "posts", "owner", uid, in.ID)
		if err != nil {
			return err
		}
		for _, collection := range []string{"likes", "comments", "notifications"} {
			if err := deleteAll(app, collection, dbx.HashExp{"post": r.Id}); err != nil {
				return err
			}
		}
		return app.Delete(r)

	case "like":
		p, err := visiblePost(app, uid, in.ID)
		if err != nil {
			return err
		}
		existing, _ := app.FindFirstRecordByFilter("likes", "post = {:p} && user = {:u}", dbx.Params{"p": p.Id, "u": uid})
		if existing != nil {
			return app.Delete(existing)
		}
		r, err := newRecord(app, "likes")
		if err != nil {
			return err
		}
		r.Set("post", p.Id)
		r.Set("user", uid)
		return app.Save(r)

	case "comment":
		p, err := visiblePost(app, uid, in.ID)
		if err != nil {
			return err
		}
		text, err := clean(in.Text, maxComment, "comment", true)
		if err != nil {
			return err
		}
		r, err := newRecord(app, "comments")
		if err != nil {
			return err
		}
		r.Set("post", p.Id)
		r.Set("user", uid)
		r.Set("text", text)
		return app.Save(r)

	case "request":
		book, err := find(app, "books", in.ID)
		if err != nil {
			return err
		}
		owner := book.GetString("owner")
		if owner == uid || !areFriends(app, uid, owner) {
			return ErrNotFound
		}
		if !book.GetBool("lendable") || lentOut(app, book.Id) {
			return errors.New("this book is not available to borrow right now")
		}
		existing, _ := app.FindFirstRecordByFilter("requests", "book = {:b} && requester = {:u} && status = 'pending'", dbx.Params{"b": book.Id, "u": uid})
		if existing != nil {
			return nil
		}
		r, err := newRecord(app, "requests")
		if err != nil {
			return err
		}
		r.Set("book", book.Id)
		r.Set("owner", owner)
		r.Set("requester", uid)
		r.Set("status", "pending")
		return app.Save(r)

	case "respond":
		r, err := owned(app, "requests", "owner", uid, in.ID)
		if err != nil || r.GetString("status") != "pending" {
			return ErrNotFound
		}
		if !in.Accept {
			r.Set("status", "declined")
			return app.Save(r)
		}
		requester := r.GetString("requester")
		if !areFriends(app, uid, requester) {
			return errors.New("you can only lend to friends")
		}
		book, err := owned(app, "books", "owner", uid, r.GetString("book"))
		if err != nil {
			return err
		}
		if lentOut(app, book.Id) {
			return errors.New("this book is already lent out; mark it returned first")
		}
		r.Set("status", "accepted")
		if err := app.Save(r); err != nil {
			return err
		}
		loan, err := newRecord(app, "loans")
		if err != nil {
			return err
		}
		loan.Set("lender", uid)
		loan.Set("borrower", requester)
		loan.Set("book", shelfBookOf(book))
		loan.Set("shelfBook", book.Id)
		loan.Set("createdBy", uid)
		loan.Set("since", now())
		return app.Save(loan)

	case "loan":
		b, err := cleanBook(in.Book)
		if err != nil {
			return err
		}
		due, err := cleanDate(in.Due)
		if err != nil {
			return err
		}
		contact, err := clean(in.Contact, maxName, "name", false)
		if err != nil {
			return err
		}
		other := strings.TrimSpace(in.Friend)
		if other != "" {
			if !areFriends(app, uid, other) {
				return errors.New("you can only log loans with your friends")
			}
			contact = ""
		} else if contact == "" {
			return errors.New("choose a friend or enter a name")
		}
		r, err := newRecord(app, "loans")
		if err != nil {
			return err
		}
		var lender string
		switch in.Direction {
		case "lent":
			lender = uid
			r.Set("lender", uid)
			r.Set("borrower", other)
		case "borrowed":
			lender = other
			r.Set("lender", other)
			r.Set("borrower", uid)
		default:
			return errors.New("invalid loan direction")
		}
		// Link the lender's shelf copy so friends see it as lent out.
		shelfBook := ""
		if in.ShelfBook != "" {
			if in.Direction != "lent" {
				return errors.New("only books you lend come from your shelf")
			}
			book, err := owned(app, "books", "owner", uid, in.ShelfBook)
			if err != nil {
				return err
			}
			if lentOut(app, book.Id) {
				return errors.New("this book is already lent out; mark it returned first")
			}
			shelfBook = book.Id
		} else if lender != "" {
			if match, err := findShelf(app, lender, b); err != nil {
				return err
			} else if match != nil && !lentOut(app, match.Id) {
				shelfBook = match.Id
			}
		}
		r.Set("shelfBook", shelfBook)
		r.Set("contact", contact)
		r.Set("book", b)
		r.Set("createdBy", uid)
		r.Set("since", now())
		r.Set("due", due)
		return app.Save(r)

	case "return", "unreturn", "nudge", "loan-delete":
		r, err := find(app, "loans", in.ID)
		if err != nil {
			return err
		}
		lender, borrower := r.GetString("lender"), r.GetString("borrower")
		if lender != uid && borrower != uid {
			return ErrNotFound
		}
		switch action {
		case "return":
			if r.GetString("returned") == "" {
				r.Set("returned", now())
			}
		case "unreturn":
			if r.GetString("returned") != "" && lentOut(app, r.GetString("shelfBook")) {
				return errors.New("that copy is lent out again; it cannot be on two loans at once")
			}
			r.Set("returned", "")
		case "nudge":
			if lender != uid || borrower == "" || r.GetString("returned") != "" {
				return errors.New("only the lender can nudge an app friend")
			}
			r.Set("nudged", now())
		case "loan-delete":
			if r.GetString("createdBy") != uid {
				return errors.New("only the person who logged this loan can delete it")
			}
			return app.Delete(r)
		}
		return app.Save(r)

	case "notifications-read":
		unread, err := app.FindAllRecords("notifications", dbx.HashExp{"user": uid, "read": false})
		if err != nil {
			return err
		}
		for _, n := range unread {
			n.Set("read", true)
			if err := app.Save(n); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("unknown action")
}
