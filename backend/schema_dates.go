package main

import "github.com/pocketbase/pocketbase/core"

// Start and finish months (YYYY-MM) for shelf books.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		books, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}
		books.Fields.Add(text("started", 7, false), text("finished", 7, false))
		return app.Save(books)
	}, func(app core.App) error {
		books, err := app.FindCollectionByNameOrId("books")
		if err != nil {
			return err
		}
		books.Fields.RemoveByName("started")
		books.Fields.RemoveByName("finished")
		return app.Save(books)
	}, "202610310001_reading_dates.go")
}
