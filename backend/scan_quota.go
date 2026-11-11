package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const scanDailyLimit = 10

var errScanLimit = errors.New("daily shelf scan limit reached")

// reserveScan atomically claims one shelf scan for the user's current UTC day.
// It returns how many scans are left afterwards. The caller refunds the claim
// if the scan itself fails.
func reserveScan(app core.App, uid string) (day string, left int, err error) {
	day = time.Now().UTC().Format("2006-01-02")
	for attempt := 0; attempt < 3; attempt++ {
		record, findErr := app.FindFirstRecordByFilter(
			"scan_usage",
			"owner = {:owner} && day = {:day}",
			dbx.Params{"owner": uid, "day": day},
		)
		if findErr == nil {
			result, execErr := app.DB().NewQuery(
				"UPDATE scan_usage SET scans = scans + 1, updated = {:updated} WHERE id = {:id} AND scans < {:limit}",
			).Bind(dbx.Params{
				"updated": now(),
				"id":      record.Id,
				"limit":   scanDailyLimit,
			}).Execute()
			if execErr != nil {
				return day, 0, execErr
			}
			changed, rowsErr := result.RowsAffected()
			if rowsErr != nil {
				return day, 0, rowsErr
			}
			if changed == 0 {
				return day, 0, errScanLimit
			}
			return day, scanRemaining(app, uid, day), nil
		}
		if !errors.Is(findErr, sql.ErrNoRows) {
			return day, 0, findErr
		}

		collection, colErr := app.FindCollectionByNameOrId("scan_usage")
		if colErr != nil {
			return day, 0, colErr
		}
		record = core.NewRecord(collection)
		record.Set("owner", uid)
		record.Set("day", day)
		record.Set("scans", 1)
		if saveErr := app.Save(record); saveErr != nil {
			// Another request may have created today's row between the find and
			// the insert; retry against that row.
			if attempt == 2 {
				return day, 0, saveErr
			}
			continue
		}
		return day, scanDailyLimit - 1, nil
	}
	return day, 0, errors.New("could not check the shelf scan limit")
}

func scanRemaining(app core.App, uid, day string) int {
	var used int
	err := app.DB().NewQuery(
		"SELECT scans FROM scan_usage WHERE owner = {:owner} AND day = {:day}",
	).Bind(dbx.Params{"owner": uid, "day": day}).Row(&used)
	if err != nil {
		return 0
	}
	left := scanDailyLimit - used
	if left < 0 {
		return 0
	}
	return left
}

// refundScan gives a failed scan back to the user. Best effort: the next scan
// still fails closed if the refund cannot be applied.
func refundScan(app core.App, uid, day string) {
	_, _ = app.DB().NewQuery(
		"UPDATE scan_usage SET scans = CASE WHEN scans > 0 THEN scans - 1 ELSE 0 END, updated = {:updated} WHERE owner = {:owner} AND day = {:day}",
	).Bind(dbx.Params{"updated": now(), "owner": uid, "day": day}).Execute()
}
