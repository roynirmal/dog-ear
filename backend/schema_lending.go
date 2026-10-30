package main

import "github.com/pocketbase/pocketbase/core"

// Lending from the shelf, friend requests and "started reading" notifications.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		books, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}
		books.Fields.Add(&core.BoolField{Name: "lendable"})
		if err := app.Save(books); err != nil {
			return err
		}
		posts, err := app.FindCollectionByNameOrId("posts")
		if err != nil {
			return err
		}
		posts.Fields.Add(text("shelfBook", 15, false))
		if err := app.Save(posts); err != nil {
			return err
		}
		// Borrow requests now point at the owner's shelf book; post is optional
		// context for requests made from the feed.
		requests, err := app.FindCollectionByNameOrId("requests")
		if err != nil {
			return err
		}
		if f, ok := requests.Fields.GetByName("post").(*core.TextField); ok {
			f.Required = false
		}
		requests.Fields.Add(text("book", 15, true))
		requests.AddIndex("idx_requests_book", false, "book", "")
		if err := app.Save(requests); err != nil {
			return err
		}
		loans, err := app.FindCollectionByNameOrId("loans")
		if err != nil {
			return err
		}
		loans.Fields.Add(text("shelfBook", 15, false))
		loans.AddIndex("idx_loans_shelf_book", false, "shelfBook", "")
		if err := app.Save(loans); err != nil {
			return err
		}

		friendRequests := core.NewBaseCollection("friend_requests")
		friendRequests.Fields.Add(text("from", 15, true), text("to", 15, true), text("status", 16, true),
			&core.AutodateField{Name: "created", OnCreate: true}, &core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
		friendRequests.AddIndex("idx_friend_requests_pair", true, "`from`, `to`", "")
		friendRequests.AddIndex("idx_friend_requests_to", false, "`to`", "")
		if err := app.Save(friendRequests); err != nil {
			return err
		}

		notifications := core.NewBaseCollection("notifications")
		notifications.Fields.Add(text("user", 15, true), text("actor", 15, true), text("post", 15, true),
			text("bookKey", 400, true), &core.BoolField{Name: "read"},
			&core.AutodateField{Name: "created", OnCreate: true}, &core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
		// One notification per friend per book, ever.
		notifications.AddIndex("idx_notifications_once", true, "user, actor, bookKey", "")
		return app.Save(notifications)
	}, func(app core.App) error {
		for _, name := range []string{"notifications", "friend_requests"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			if err = app.Delete(c); err != nil {
				return err
			}
		}
		for name, fields := range map[string][]string{"books": {"lendable"}, "posts": {"shelfBook"}, "requests": {"book"}, "loans": {"shelfBook"}} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			for _, f := range fields {
				c.Fields.RemoveByName(f)
			}
			c.RemoveIndex("idx_requests_book")
			c.RemoveIndex("idx_loans_shelf_book")
			if err = app.Save(c); err != nil {
				return err
			}
		}
		return nil
	}, "202610300002_lending.go")
}
