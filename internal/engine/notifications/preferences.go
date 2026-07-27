package notifications

import (
	"context"
	"database/sql"
)

// MutableCategories is the single source of truth for which notification
// categories a player is allowed to mute. Nothing outside this set can ever
// be silenced by notification_preferences - see
// MMO_WORLD_EVOLUTION_PLAN.md Phase 7 milestone 2: "without suppressing
// combat, discovery, or supply-loss alerts." Adding a new mutable category
// means deliberately deciding it's as low-stakes as route_status (a
// peaceful road pass, a cleared weather incident, a convoy arrival) - not
// just tagging more call sites with an existing one.
var MutableCategories = map[string]bool{
	"route_status": true,
}

// querier is satisfied by both *sql.DB and *sql.Tx, so tick-engine code
// (which always has an open transaction) and bot handlers (which usually
// don't) can share this same queuing helper.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// IsCategoryMuted reports whether userID has muted category. Categories
// outside MutableCategories are never muted, regardless of what's in the
// database - this is checked in Go, not left to a DB default, so a bad row
// or a future column added without updating MutableCategories can't
// accidentally suppress a combat/discovery/supply-loss alert.
func IsCategoryMuted(ctx context.Context, q querier, userID int64, category string) bool {
	if !MutableCategories[category] {
		return false
	}
	var muted bool
	switch category {
	case "route_status":
		err := q.QueryRowContext(ctx, "SELECT mute_route_status FROM notification_preferences WHERE user_id = $1", userID).Scan(&muted)
		if err != nil {
			return false // no row yet == default (unmuted)
		}
	}
	return muted
}

// Queue inserts a notification tagged with category, unless the recipient
// has muted that category. Combat/discovery/supply-loss call sites should
// keep using category "general" (or call Queue with a category that isn't
// in MutableCategories) so they're never subject to muting at all.
func Queue(ctx context.Context, q querier, userID int64, message, category string) error {
	if IsCategoryMuted(ctx, q, userID, category) {
		return nil
	}
	_, err := q.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, category) VALUES ($1, $2, FALSE, $3)", userID, message, category)
	return err
}
