package world

import (
	"context"
	"database/sql"
)

// Queryer is satisfied by both *sql.DB and *sql.Tx, so callers already
// inside a tick transaction and callers making a one-off query from a
// bot handler can both use the same lookup helpers below.
type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// ActiveEventFor returns the currently active world event type for a
// given continent ("nominal" if none is active, the row has expired, or
// the continent is unrecognized/empty - e.g. a rogue AI nest with no
// real region). Every mechanical consumer of world events (combat
// resolution, passive resource generation, march timing, construction
// flavor text) should resolve its own encampment's continent via
// coordinates.region and call this instead of reading world_state
// directly, which is no longer written to.
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

// ActiveEventsByContinent returns every continent's currently active
// event type in one query, for callers that process many encampments in
// a single pass (e.g. the passive resource tick) and would otherwise pay
// for one query per encampment. Continents with no active event are
// simply absent from the map - callers should treat a missing key the
// same as "nominal".
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
