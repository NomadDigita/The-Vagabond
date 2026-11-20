package notifications

import (
	"context"
	"database/sql"
)

var MutableCategories = map[string]bool{
	"route_status": true,
	"scout_status": true,
}

type querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func IsCategoryMuted(ctx context.Context, q querier, userID int64, category string) bool {
	if !MutableCategories[category] {
		return false
	}
	var muted bool
	switch category {
	case "route_status":
		err := q.QueryRowContext(ctx, "SELECT mute_route_status FROM notification_preferences WHERE user_id = $1", userID).Scan(&muted)
		if err != nil {
			return false
		}
	case "scout_status":
		err := q.QueryRowContext(ctx, "SELECT mute_scout_status FROM notification_preferences WHERE user_id = $1", userID).Scan(&muted)
		if err != nil {
			return false
		}
	}
	return muted
}

func Queue(ctx context.Context, q querier, userID int64, message, category string, effectID ...string) error {
	if IsCategoryMuted(ctx, q, userID, category) {
		return nil
	}
	var effect string
	if len(effectID) > 0 {
		effect = effectID[0]
	}
	_, err := q.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, category, effect_id) VALUES ($1, $2, FALSE, $3, NULLIF($4, ''))", userID, message, category, effect)
	return err
}
