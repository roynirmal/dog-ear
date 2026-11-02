package main

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Reading progress lives on the shelf book, so updating it changes what every
// post about that read shows instead of creating a new post.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		books, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}
		books.Fields.Add(&core.NumberField{Name: "progress", OnlyInt: true}, text("progressAt", 40, false))
		if err := app.Save(books); err != nil {
			return err
		}
		// Carry over each reading book's progress from its latest reading post.
		reading, err := app.FindAllRecords("books", dbx.HashExp{"status": "reading"})
		if err != nil {
			return err
		}
		for _, b := range reading {
			posts, err := app.FindRecordsByFilter("posts", "shelfBook = {:b} && status = 'reading'", "-created", 1, 0, dbx.Params{"b": b.Id})
			if err != nil {
				return err
			}
			if len(posts) == 0 {
				continue
			}
			b.Set("progress", posts[0].GetInt("progress"))
			b.Set("progressAt", stamp(posts[0], "created"))
			if err := app.Save(b); err != nil {
				return err
			}
		}
		return nil
	}, func(app core.App) error {
		books, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}
		books.Fields.RemoveByName("progress")
		books.Fields.RemoveByName("progressAt")
		return app.Save(books)
	}, "202611020001_reading_progress.go")
}
