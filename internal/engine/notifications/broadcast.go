package notifications

import (
	"context"
	"database/sql"
)

type broadcastQuerier interface {
	querier
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

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
