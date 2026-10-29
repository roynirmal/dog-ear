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
	Code      string   `json:"code"`
	Book      *Book    `json:"book"`
	Status    string   `json:"status"`
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

type Person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Comment struct {
	ID        string `json:"id"`
	AuthorID  string `json:"authorId"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}
type Post struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"authorId"`
	Book      Book      `json:"book"`
	Status    string    `json:"status"`
	Progress  *int      `json:"progress,omitempty"`
	Rating    *int      `json:"rating,omitempty"`
	Thoughts  string    `json:"thoughts"`
	CreatedAt string    `json:"createdAt"`
	LikedBy   []string  `json:"likedBy"`
	Comments  []Comment `json:"comments"`
	CanLend   bool      `json:"canLend"`
	Requested bool      `json:"requested"`
}
type Loan struct {
	ID         string `json:"id"`
	Direction  string `json:"direction"`
	FriendID   string `json:"friendId,omitempty"`
	Contact    string `json:"contact,omitempty"`
	Book       Book   `json:"book"`
	Since      string `json:"since"`
	DueDate    string `json:"dueDate,omitempty"`
	ReturnedAt string `json:"returnedAt,omitempty"`
	NudgedAt   string `json:"nudgedAt,omitempty"`
	Mine       bool   `json:"mine"`
}
type Request struct {
	ID          string `json:"id"`
	PostID      string `json:"postId"`
	Book        Book   `json:"book"`
	RequesterID string `json:"requesterId"`
	CreatedAt   string `json:"createdAt"`
}
type ShelfBook struct {
	Book
	ID      string `json:"id"`
	Status  string `json:"status"`
	AddedAt string `json:"addedAt"`
}
type Note struct {
	ID        string   `json:"id"`
	BookID    string   `json:"bookId,omitempty"`
	Text      string   `json:"text"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}
type State struct {
	Me       Person      `json:"me"`
	Code     string      `json:"code"`
	People   []Person    `json:"people"`
	Friends  []string    `json:"friends"`
	Posts    []Post      `json:"posts"`
	Loans    []Loan      `json:"loans"`
	Requests []Request   `json:"requests"`
	Shelf    []ShelfBook `json:"shelf"`
	Notes    []Note      `json:"notes"`
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

func newRecord(app core.App, collection string) (*core.Record, error) {
	c, err := app.FindCachedCollectionByNameOrId(collection)
	if err != nil {
		return nil, err
	}
	return core.NewRecord(c), nil
}

// upsertShelf keeps the poster's shelf in step with what they share.
func upsertShelf(app core.App, uid string, b Book, status string) error {
	rs, err := app.FindAllRecords("books", dbx.HashExp{"owner": uid})
	if err != nil {
		return err
	}
	var r *core.Record
	for _, candidate := range rs {
		sameKey := b.OLKey != "" && candidate.GetString("olKey") == b.OLKey
		sameText := strings.EqualFold(candidate.GetString("title"), b.Title) && strings.EqualFold(candidate.GetString("author"), b.Author)
		if sameKey || sameText {
			r = candidate
			break
		}
	}
	if r == nil {
		if r, err = newRecord(app, "books"); err != nil {
			return err
		}
		r.Set("owner", uid)
		r.Set("title", b.Title)
		r.Set("author", b.Author)
		r.Set("olKey", b.OLKey)
		r.Set("coverId", b.CoverID)
		r.Set("year", b.Year)
	}
	r.Set("status", status)
	return app.Save(r)
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
		if other.Id == uid || areFriends(app, uid, other.Id) {
			return nil // harmless repeat or own link
		}
		r, err := newRecord(app, "friendships")
		if err != nil {
			return err
		}
		a, b := pair(uid, other.Id)
		r.Set("a", a)
		r.Set("b", b)
		return app.Save(r)

	case "unfriend":
		a, b := pair(uid, in.ID)
		r, err := app.FindFirstRecordByFilter("friendships", "a = {:a} && b = {:b}", dbx.Params{"a": a, "b": b})
		if err != nil || r == nil {
			return ErrNotFound
		}
		return app.Delete(r)

	case "shelf-add":
		b, err := cleanBook(in.Book)
		if err != nil {
			return err
		}
		if !shelfStatuses[in.Status] {
			return errors.New("invalid shelf status")
		}
		return upsertShelf(app, uid, b, in.Status)

	case "shelf-update":
		r, err := owned(app, "books", "owner", uid, in.ID)
		if err != nil {
			return err
		}
		if !shelfStatuses[in.Status] {
			return errors.New("invalid shelf status")
		}
		r.Set("status", in.Status)
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
		r.Set("owner", uid)
		r.Set("book", b)
		r.Set("status", in.Status)
		r.Set("thoughts", thoughts)
		r.Set("canLend", in.CanLend)
		if err := app.Save(r); err != nil {
			return err
		}
		return upsertShelf(app, uid, b, in.Status)

	case "post-delete":
		r, err := owned(app, "posts", "owner", uid, in.ID)
		if err != nil {
			return err
		}
		for _, collection := range []string{"likes", "comments", "requests"} {
			children, err := app.FindAllRecords(collection, dbx.HashExp{"post": r.Id})
			if err != nil {
				return err
			}
			for _, c := range children {
				if err := app.Delete(c); err != nil {
					return err
				}
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
		p, err := visiblePost(app, uid, in.ID)
		if err != nil {
			return err
		}
		if p.GetString("owner") == uid || !p.GetBool("canLend") {
			return errors.New("this book is not available to borrow")
		}
		existing, _ := app.FindFirstRecordByFilter("requests", "post = {:p} && requester = {:u} && status = 'pending'", dbx.Params{"p": p.Id, "u": uid})
		if existing != nil {
			return nil
		}
		r, err := newRecord(app, "requests")
		if err != nil {
			return err
		}
		r.Set("post", p.Id)
		r.Set("owner", p.GetString("owner"))
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
		p, err := find(app, "posts", r.GetString("post"))
		if err != nil {
			return err
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
		loan.Set("borrower", r.GetString("requester"))
		loan.Set("book", decodeBook(p))
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
				return errors.New("you can only log loans with friends in your circle")
			}
			contact = ""
		} else if contact == "" {
			return errors.New("choose a friend or enter a name")
		}
		r, err := newRecord(app, "loans")
		if err != nil {
			return err
		}
		switch in.Direction {
		case "lent":
			r.Set("lender", uid)
			r.Set("borrower", other)
		case "borrowed":
			r.Set("lender", other)
			r.Set("borrower", uid)
		default:
			return errors.New("invalid loan direction")
		}
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
	}
	return errors.New("unknown action")
}

func load(app core.App, u *core.Record) (*State, error) {
	uid := u.Id
	s := &State{Me: Person{uid, displayName(u)}, Code: u.GetString("code"), Posts: []Post{}, Loans: []Loan{}, Requests: []Request{}, Shelf: []ShelfBook{}, Notes: []Note{}}
	friends, err := friendIDs(app, uid)
	if err != nil {
		return nil, err
	}
	s.Friends = friends
	people := map[string]bool{uid: true}
	for _, id := range friends {
		people[id] = true
	}

	authors := anyIDs(append([]string{uid}, friends...))
	posts, err := app.FindAllRecords("posts", dbx.In("owner", authors...))
	if err != nil {
		return nil, err
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].GetDateTime("created").Time().After(posts[j].GetDateTime("created").Time())
	})
	if len(posts) > feedLimit {
		posts = posts[:feedLimit]
	}
	postIDs := make([]string, len(posts))
	index := map[string]int{}
	for i, p := range posts {
		postIDs[i] = p.Id
		index[p.Id] = i
		view := Post{ID: p.Id, AuthorID: p.GetString("owner"), Book: decodeBook(p), Status: p.GetString("status"), Thoughts: p.GetString("thoughts"),
			CreatedAt: stamp(p, "created"), LikedBy: []string{}, Comments: []Comment{}, CanLend: p.GetBool("canLend")}
		if view.Status == "reading" {
			progress := p.GetInt("progress")
			view.Progress = &progress
		} else if rating := p.GetInt("rating"); rating > 0 {
			view.Rating = &rating
		}
		s.Posts = append(s.Posts, view)
	}
	if len(postIDs) > 0 {
		likes, err := app.FindAllRecords("likes", dbx.In("post", anyIDs(postIDs)...))
		if err != nil {
			return nil, err
		}
		for _, l := range likes {
			p := &s.Posts[index[l.GetString("post")]]
			p.LikedBy = append(p.LikedBy, l.GetString("user"))
			people[l.GetString("user")] = true
		}
		comments, err := app.FindAllRecords("comments", dbx.In("post", anyIDs(postIDs)...))
		if err != nil {
			return nil, err
		}
		sort.Slice(comments, func(i, j int) bool {
			return comments[i].GetDateTime("created").Time().Before(comments[j].GetDateTime("created").Time())
		})
		for _, c := range comments {
			p := &s.Posts[index[c.GetString("post")]]
			p.Comments = append(p.Comments, Comment{c.Id, c.GetString("user"), c.GetString("text"), stamp(c, "created")})
			people[c.GetString("user")] = true
		}
		mine, err := app.FindAllRecords("requests", dbx.HashExp{"requester": uid, "status": "pending"})
		if err != nil {
			return nil, err
		}
		for _, r := range mine {
			if i, ok := index[r.GetString("post")]; ok {
				s.Posts[i].Requested = true
			}
		}
	}

	incoming, err := app.FindAllRecords("requests", dbx.HashExp{"owner": uid, "status": "pending"})
	if err != nil {
		return nil, err
	}
	for _, r := range incoming {
		p, err := find(app, "posts", r.GetString("post"))
		if err != nil {
			continue
		}
		s.Requests = append(s.Requests, Request{r.Id, p.Id, decodeBook(p), r.GetString("requester"), stamp(r, "created")})
		people[r.GetString("requester")] = true
	}
	sort.Slice(s.Requests, func(i, j int) bool { return s.Requests[i].CreatedAt > s.Requests[j].CreatedAt })

	loans, err := app.FindAllRecords("loans", dbx.Or(dbx.HashExp{"lender": uid}, dbx.HashExp{"borrower": uid}))
	if err != nil {
		return nil, err
	}
	for _, r := range loans {
		l := Loan{ID: r.Id, Contact: r.GetString("contact"), Book: decodeBook(r), Since: r.GetString("since"), DueDate: r.GetString("due"),
			ReturnedAt: r.GetString("returned"), NudgedAt: r.GetString("nudged"), Mine: r.GetString("createdBy") == uid}
		if r.GetString("lender") == uid {
			l.Direction, l.FriendID = "lent", r.GetString("borrower")
		} else {
			l.Direction, l.FriendID = "borrowed", r.GetString("lender")
		}
		if l.FriendID != "" {
			people[l.FriendID] = true
		}
		s.Loans = append(s.Loans, l)
	}
	sort.Slice(s.Loans, func(i, j int) bool { return s.Loans[i].Since > s.Loans[j].Since })

	shelf, err := app.FindAllRecords("books", dbx.HashExp{"owner": uid})
	if err != nil {
		return nil, err
	}
	for _, r := range shelf {
		s.Shelf = append(s.Shelf, ShelfBook{Book{r.GetString("title"), r.GetString("author"), r.GetString("olKey"), r.GetInt("coverId"), r.GetInt("year")},
			r.Id, r.GetString("status"), stamp(r, "created")})
	}
	sort.Slice(s.Shelf, func(i, j int) bool { return s.Shelf[i].AddedAt > s.Shelf[j].AddedAt })

	notes, err := app.FindAllRecords("notes", dbx.HashExp{"owner": uid})
	if err != nil {
		return nil, err
	}
	for _, r := range notes {
		tags := []string{}
		_ = r.UnmarshalJSONField("tags", &tags)
		s.Notes = append(s.Notes, Note{r.Id, r.GetString("book"), r.GetString("text"), tags, stamp(r, "created"), stamp(r, "updated")})
	}
	sort.Slice(s.Notes, func(i, j int) bool { return s.Notes[i].UpdatedAt > s.Notes[j].UpdatedAt })

	ids := make([]string, 0, len(people))
	for id := range people {
		ids = append(ids, id)
	}
	users, err := app.FindRecordsByIds("users", ids)
	if err != nil {
		return nil, err
	}
	for _, p := range users {
		s.People = append(s.People, Person{p.Id, displayName(p)})
	}
	sort.Slice(s.People, func(i, j int) bool { return s.People[i].Name < s.People[j].Name })
	return s, nil
}
