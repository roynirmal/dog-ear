package main

import (
	"sort"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const notificationLimit = 50

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
	ID          string    `json:"id"`
	AuthorID    string    `json:"authorId"`
	Book        Book      `json:"book"`
	Status      string    `json:"status"`
	Progress    *int      `json:"progress,omitempty"`
	Rating      *int      `json:"rating,omitempty"`
	Thoughts    string    `json:"thoughts"`
	CreatedAt   string    `json:"createdAt"`
	LikedBy     []string  `json:"likedBy"`
	Comments    []Comment `json:"comments"`
	ShelfBookID string    `json:"shelfBookId,omitempty"`
	// CanLend is live: the author's copy is lendable and not on loan.
	CanLend   bool `json:"canLend"`
	LentOut   bool `json:"lentOut"`
	Requested bool `json:"requested"`
}
type Loan struct {
	ID          string `json:"id"`
	Direction   string `json:"direction"`
	FriendID    string `json:"friendId,omitempty"`
	Contact     string `json:"contact,omitempty"`
	Book        Book   `json:"book"`
	ShelfBookID string `json:"shelfBookId,omitempty"`
	Since       string `json:"since"`
	DueDate     string `json:"dueDate,omitempty"`
	ReturnedAt  string `json:"returnedAt,omitempty"`
	NudgedAt    string `json:"nudgedAt,omitempty"`
	Mine        bool   `json:"mine"`
}
type Request struct {
	ID          string `json:"id"`
	BookID      string `json:"bookId"`
	Book        Book   `json:"book"`
	RequesterID string `json:"requesterId"`
	CreatedAt   string `json:"createdAt"`
}
type ShelfBook struct {
	Book
	ID       string `json:"id"`
	Status   string `json:"status"`
	Lendable bool   `json:"lendable"`
	AddedAt  string `json:"addedAt"`
}

// FriendBook is a lendable book on a friend's shelf.
type FriendBook struct {
	Book
	ID        string `json:"id"`
	OwnerID   string `json:"ownerId"`
	Status    string `json:"status"`
	Available bool   `json:"available"`
	Requested bool   `json:"requested"`
}
type FriendRequest struct {
	ID        string `json:"id"`
	FromID    string `json:"fromId"`
	CreatedAt string `json:"createdAt"`
}
type Notification struct {
	ID        string `json:"id"`
	ActorID   string `json:"actorId"`
	PostID    string `json:"postId"`
	Book      Book   `json:"book"`
	CreatedAt string `json:"createdAt"`
	Read      bool   `json:"read"`
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
	Me             Person          `json:"me"`
	Code           string          `json:"code"`
	People         []Person        `json:"people"`
	Friends        []string        `json:"friends"`
	FriendRequests []FriendRequest `json:"friendRequests"`
	// SentRequests lists people you asked to be friends who have not answered.
	SentRequests  []string       `json:"sentRequests"`
	Posts         []Post         `json:"posts"`
	Loans         []Loan         `json:"loans"`
	Requests      []Request      `json:"requests"`
	Shelf         []ShelfBook    `json:"shelf"`
	Library       []FriendBook   `json:"library"`
	Notes         []Note         `json:"notes"`
	Notifications []Notification `json:"notifications"`
}

func newer(rs []*core.Record) {
	sort.Slice(rs, func(i, j int) bool {
		return rs[i].GetDateTime("created").Time().After(rs[j].GetDateTime("created").Time())
	})
}

func load(app core.App, u *core.Record) (*State, error) {
	uid := u.Id
	s := &State{Me: Person{uid, displayName(u)}, Code: u.GetString("code"), FriendRequests: []FriendRequest{}, SentRequests: []string{},
		Posts: []Post{}, Loans: []Loan{}, Requests: []Request{}, Shelf: []ShelfBook{}, Library: []FriendBook{}, Notes: []Note{}, Notifications: []Notification{}}
	friends, err := friendIDs(app, uid)
	if err != nil {
		return nil, err
	}
	s.Friends = friends
	isFriend := map[string]bool{}
	people := map[string]bool{uid: true}
	for _, id := range friends {
		people[id] = true
		isFriend[id] = true
	}

	// Shelf books: mine, my friends' lendable ones, and those behind feed posts.
	mine, err := app.FindAllRecords("books", dbx.HashExp{"owner": uid})
	if err != nil {
		return nil, err
	}
	library := []*core.Record{}
	if len(friends) > 0 {
		if library, err = app.FindAllRecords("books", dbx.In("owner", anyIDs(friends)...), dbx.HashExp{"lendable": true}); err != nil {
			return nil, err
		}
	}
	authors := anyIDs(append([]string{uid}, friends...))
	posts, err := app.FindAllRecords("posts", dbx.In("owner", authors...))
	if err != nil {
		return nil, err
	}
	newer(posts)
	if len(posts) > feedLimit {
		posts = posts[:feedLimit]
	}
	books := map[string]*core.Record{}
	for _, r := range append(mine, library...) {
		books[r.Id] = r
	}
	missing := []string{}
	for _, p := range posts {
		if id := p.GetString("shelfBook"); id != "" && books[id] == nil {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		extra, err := app.FindRecordsByIds("books", missing)
		if err != nil {
			return nil, err
		}
		for _, r := range extra {
			books[r.Id] = r
		}
	}
	onLoan := map[string]bool{}
	if len(books) > 0 {
		ids := make([]string, 0, len(books))
		for id := range books {
			ids = append(ids, id)
		}
		active, err := app.FindAllRecords("loans", dbx.In("shelfBook", anyIDs(ids)...), dbx.HashExp{"returned": ""})
		if err != nil {
			return nil, err
		}
		for _, l := range active {
			onLoan[l.GetString("shelfBook")] = true
		}
	}
	asked := map[string]bool{}
	myRequests, err := app.FindAllRecords("requests", dbx.HashExp{"requester": uid, "status": "pending"})
	if err != nil {
		return nil, err
	}
	for _, r := range myRequests {
		asked[r.GetString("book")] = true
	}

	postIDs := make([]string, len(posts))
	index := map[string]int{}
	for i, p := range posts {
		postIDs[i] = p.Id
		index[p.Id] = i
		view := Post{ID: p.Id, AuthorID: p.GetString("owner"), Book: decodeBook(p), Status: p.GetString("status"), Thoughts: p.GetString("thoughts"),
			CreatedAt: stamp(p, "created"), LikedBy: []string{}, Comments: []Comment{}, ShelfBookID: p.GetString("shelfBook")}
		if view.Status == "reading" {
			progress := p.GetInt("progress")
			view.Progress = &progress
		} else if rating := p.GetInt("rating"); rating > 0 {
			view.Rating = &rating
		}
		if b := books[view.ShelfBookID]; b != nil && b.GetBool("lendable") {
			view.LentOut = onLoan[b.Id]
			view.CanLend = !view.LentOut
			view.Requested = asked[b.Id]
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
	}

	for _, r := range mine {
		s.Shelf = append(s.Shelf, ShelfBook{shelfBookOf(r), r.Id, r.GetString("status"), r.GetBool("lendable"), stamp(r, "created")})
	}
	sort.Slice(s.Shelf, func(i, j int) bool { return s.Shelf[i].AddedAt > s.Shelf[j].AddedAt })
	for _, r := range library {
		s.Library = append(s.Library, FriendBook{shelfBookOf(r), r.Id, r.GetString("owner"), r.GetString("status"), !onLoan[r.Id], asked[r.Id]})
	}
	sort.Slice(s.Library, func(i, j int) bool { return s.Library[i].Title < s.Library[j].Title })

	// Borrow requests for my books, from people who are still friends.
	incoming, err := app.FindAllRecords("requests", dbx.HashExp{"owner": uid, "status": "pending"})
	if err != nil {
		return nil, err
	}
	newer(incoming)
	for _, r := range incoming {
		b := books[r.GetString("book")]
		if b == nil || !isFriend[r.GetString("requester")] {
			continue
		}
		s.Requests = append(s.Requests, Request{r.Id, b.Id, shelfBookOf(b), r.GetString("requester"), stamp(r, "created")})
	}

	friendRequests, err := app.FindAllRecords("friend_requests", dbx.HashExp{"to": uid, "status": "pending"})
	if err != nil {
		return nil, err
	}
	newer(friendRequests)
	for _, r := range friendRequests {
		s.FriendRequests = append(s.FriendRequests, FriendRequest{r.Id, r.GetString("from"), stamp(r, "created")})
		people[r.GetString("from")] = true
	}
	sent, err := app.FindAllRecords("friend_requests", dbx.HashExp{"from": uid})
	if err != nil {
		return nil, err
	}
	for _, r := range sent {
		s.SentRequests = append(s.SentRequests, r.GetString("to"))
	}

	loans, err := app.FindAllRecords("loans", dbx.Or(dbx.HashExp{"lender": uid}, dbx.HashExp{"borrower": uid}))
	if err != nil {
		return nil, err
	}
	for _, r := range loans {
		l := Loan{ID: r.Id, Contact: r.GetString("contact"), Book: decodeBook(r), ShelfBookID: r.GetString("shelfBook"), Since: r.GetString("since"),
			DueDate: r.GetString("due"), ReturnedAt: r.GetString("returned"), NudgedAt: r.GetString("nudged"), Mine: r.GetString("createdBy") == uid}
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

	notifications, err := app.FindRecordsByFilter("notifications", "user = {:u}", "-created", notificationLimit, 0, dbx.Params{"u": uid})
	if err != nil {
		return nil, err
	}
	for _, n := range notifications {
		p, err := find(app, "posts", n.GetString("post"))
		if err != nil {
			continue
		}
		s.Notifications = append(s.Notifications, Notification{n.Id, n.GetString("actor"), p.Id, decodeBook(p), stamp(n, "created"), n.GetBool("read")})
		people[n.GetString("actor")] = true
	}

	ids := make([]string, 0, len(people))
	for id := range people {
		ids = append(ids, id)
	}
	users, err := app.FindRecordsByIds("users", ids)
	if err != nil {
		return nil, err
	}
	s.People = []Person{}
	for _, p := range users {
		s.People = append(s.People, Person{p.Id, displayName(p)})
	}
	sort.Slice(s.People, func(i, j int) bool { return s.People[i].Name < s.People[j].Name })
	return s, nil
}
