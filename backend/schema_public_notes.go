package main

import (
	"github.com/pocketbase/pocketbase/core"
)

// A note about a book can be made public: friends then read it on the author's
// post about that book.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		notes, err := app.FindCollectionByNameOrId("notes")
		if err != nil {
			return err
		}
		notes.Fields.Add(&core.BoolField{Name: "public"})
		return app.Save(notes)
	}, func(app core.App) error {
		notes, err := app.FindCollectionByNameOrId("notes")
		if err != nil {
			return err
		}
		notes.Fields.RemoveByName("public")
		return app.Save(notes)
	}, "202611040001_public_notes.go")
}
