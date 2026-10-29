package main

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

// Every application collection is superuser-only. Clients read and write
// through the transactional /api/dogear actions, which do the authorization.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		lockUsers(users)
		users.Fields.Add(&core.TextField{Name: "name", Max: 160}, &core.TextField{Name: "code", Max: 64})
		users.AddIndex("idx_users_code", false, "code", "")
		self := "id = @request.auth.id"
		users.ViewRule = &self
		if err := app.Save(users); err != nil {
			return err
		}
		collections := []struct {
			name    string
			fields  []core.Field
			indexes [][]string // name, unique ("1"/""), columns
		}{
			{"friendships", []core.Field{text("a", 15, true), text("b", 15, true)},
				[][]string{{"idx_friendships_pair", "1", "a, b"}, {"idx_friendships_b", "", "b"}}},
			{"books", []core.Field{text("owner", 15, true), text("title", maxTitle, true), text("author", maxAuthor, false),
				text("olKey", 64, false), &core.NumberField{Name: "coverId", OnlyInt: true}, &core.NumberField{Name: "year", OnlyInt: true},
				text("status", 16, true)},
				[][]string{{"idx_books_owner", "", "owner"}}},
			{"notes", []core.Field{text("owner", 15, true), text("book", 15, false), text("text", maxNote, true), &core.JSONField{Name: "tags", MaxSize: 4096}},
				[][]string{{"idx_notes_owner", "", "owner"}}},
			{"posts", []core.Field{text("owner", 15, true), &core.JSONField{Name: "book", MaxSize: 4096}, text("status", 16, true),
				&core.NumberField{Name: "progress", OnlyInt: true}, &core.NumberField{Name: "rating", OnlyInt: true},
				text("thoughts", maxThoughts, false), &core.BoolField{Name: "canLend"}},
				[][]string{{"idx_posts_owner", "", "owner"}}},
			{"likes", []core.Field{text("post", 15, true), text("user", 15, true)},
				[][]string{{"idx_likes_pair", "1", "post, user"}}},
			{"comments", []core.Field{text("post", 15, true), text("user", 15, true), text("text", maxComment, true)},
				[][]string{{"idx_comments_post", "", "post"}}},
			{"requests", []core.Field{text("post", 15, true), text("owner", 15, true), text("requester", 15, true), text("status", 16, true)},
				[][]string{{"idx_requests_owner", "", "owner"}, {"idx_requests_requester", "", "requester"}}},
			// lender/borrower are user IDs; at most one may be empty, in which case
			// contact names the person outside the app.
			{"loans", []core.Field{text("lender", 15, false), text("borrower", 15, false), text("contact", maxName, false),
				&core.JSONField{Name: "book", MaxSize: 4096}, text("createdBy", 15, true), text("since", 40, true),
				text("due", 10, false), text("returned", 40, false), text("nudged", 40, false)},
				[][]string{{"idx_loans_lender", "", "lender"}, {"idx_loans_borrower", "", "borrower"}}},
		}
		for _, spec := range collections {
			c := core.NewBaseCollection(spec.name)
			for _, f := range spec.fields {
				c.Fields.Add(f)
			}
			c.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true}, &core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
			for _, idx := range spec.indexes {
				c.AddIndex(idx[0], idx[1] == "1", idx[2], "")
			}
			if err := app.Save(c); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		for _, name := range []string{"loans", "requests", "comments", "likes", "posts", "notes", "books", "friendships", "users"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			if err = app.Delete(c); err != nil {
				return err
			}
		}
		return nil
	}, "202610300001_dogear.go")
}

func text(name string, max int, required bool) *core.TextField {
	return &core.TextField{Name: name, Max: max, Required: required}
}

func lockUsers(users *core.Collection) {
	oauthCreate := `@request.context = "oauth2"`
	users.CreateRule = &oauthCreate
	users.ListRule = nil
	users.UpdateRule = nil
	users.DeleteRule = nil
	users.ManageRule = nil
	users.OAuth2.MappedFields.AvatarURL = "" // no unnecessary profile-image storage
	users.PasswordAuth.Enabled = false
	users.OTP.Enabled = false
	users.AuthAlert.Enabled = false
}

func configure(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	// Environment overrides; admin UI configuration remains supported when absent.
	if id, secret := os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"); id != "" && secret != "" {
		users.OAuth2.Enabled = true
		users.OAuth2.Providers = []core.OAuth2ProviderConfig{{Name: "google", ClientId: id, ClientSecret: secret}}
	}
	lockUsers(users)
	return app.Save(users)
}
