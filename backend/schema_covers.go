package main

import "github.com/pocketbase/pocketbase/core"

// Google Books cover thumbnails for shelf books.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		books, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}
		books.Fields.Add(text("coverUrl", 500, false))
		return app.Save(books)
	}, func(app core.App) error {
		books, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}
		books.Fields.RemoveByName("coverUrl")
		return app.Save(books)
	}, "202610310002_cover_url.go")
}
