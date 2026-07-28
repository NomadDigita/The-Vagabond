package notifications

import (
	"context"
	"database/sql"
)

// broadcastQuerier is satisfied by both *sql.DB and *sql.Tx - same
// convention as querier in notifications.go, extended with the raw
// QueryContext this file needs to enumerate recipients before calling
// Queue() for each one.
type broadcastQuerier interface {
	querier
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// QueueToRegion delivers message to every real player whose encampment
// currently sits in region - the "regional" broadcast shape from
// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.2, used for weather
// and anything else tied to a continent/region. AI factions are excluded
// (e.is_ai_faction = FALSE): they have no Telegram session to notify.
// Goes through the same Queue() mute-aware path as any other
// notification, so if category is ever made mutable, muted players are
// still respected.
func QueueToRegion(ctx context.Context, q broadcastQuerier, region, message, category string) error {
	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT e.user_id
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE c.region = $1 AND e.is_ai_faction = FALSE`, region)
	if err != nil {
		return err
	}
	var userIDs []int64
	for rows.Next() {
		var userID int64
		if scanErr := rows.Scan(&userID); scanErr == nil {
			userIDs = append(userIDs, userID)
		}
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return rowsErr
	}

	for _, userID := range userIDs {
		if err := Queue(ctx, q, userID, message, category); err != nil {
			return err
		}
	}
	return nil
}

// QueueToAllPlayers delivers message to every real player, server-wide -
// the "global" broadcast shape from
// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.2, used for things
// like a tax rate change. AI factions are excluded via their synthetic
// users.state = 'ai_faction' marker (see cmd/bot/main.go's AI civilization
// seeding) - they have no Telegram session to notify either.
func QueueToAllPlayers(ctx context.Context, q broadcastQuerier, message, category string) error {
	rows, err := q.QueryContext(ctx, `SELECT telegram_id FROM users WHERE state <> 'ai_faction'`)
	if err != nil {
		return err
	}
	var userIDs []int64
	for rows.Next() {
		var userID int64
		if scanErr := rows.Scan(&userID); scanErr == nil {
			userIDs = append(userIDs, userID)
		}
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return rowsErr
	}

	for _, userID := range userIDs {
		if err := Queue(ctx, q, userID, message, category); err != nil {
			return err
		}
	}
	return nil
}
