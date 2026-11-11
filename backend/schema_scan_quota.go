package main

import "github.com/pocketbase/pocketbase/core"

// One row per user per UTC day, so the shelf-scan quota survives restarts and
// is enforced atomically without keeping counters in memory.
func init() {
	core.AppMigrations.Register(func(app core.App) error {
		c := core.NewBaseCollection("scan_usage")
		c.Fields.Add(
			text("owner", 15, true),
			text("day", 10, true),
			&core.NumberField{Name: "scans", OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		c.AddIndex("idx_scan_usage_owner_day", true, "owner, day", "")
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("scan_usage")
		if err != nil {
			return err
		}
		return app.Delete(c)
	}, "202611110001_scan_quota.go")
}
