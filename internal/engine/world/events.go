package world

import (
	"context"
	"database/sql"
)

type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func ActiveEventFor(ctx context.Context, q Queryer, continent string) string {
	if continent == "" {
		return "nominal"
	}
	var eventType string
	err := q.QueryRowContext(ctx,
		`SELECT event_type FROM world_events
		 WHERE continent = $1 AND expires_at > CURRENT_TIMESTAMP
		 ORDER BY expires_at DESC LIMIT 1`, continent).Scan(&eventType)
	if err != nil {
		return "nominal"
	}
	return eventType
}

func ActiveEventsByContinent(ctx context.Context, q Queryer) map[string]string {
	result := make(map[string]string, len(Continents))

	rows, err := q.QueryContext(ctx,
		`SELECT DISTINCT ON (continent) continent, event_type
		 FROM world_events
		 WHERE expires_at > CURRENT_TIMESTAMP
		 ORDER BY continent, expires_at DESC`)
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var continent, eventType string
		if scanErr := rows.Scan(&continent, &eventType); scanErr == nil {
			result[continent] = eventType
		}
	}
	return result
}
