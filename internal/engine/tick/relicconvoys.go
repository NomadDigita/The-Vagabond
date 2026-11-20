package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
)

var relicConvoySpawnChance = 0.0015

const relicConvoyWindowMinutes = 3 * 60.0

const (
	relicConvoyMinReward = 12000.0
	relicConvoyMaxReward = 20000.0
)

var relicNames = []string{
	"The Last Aurora Convoy",
	"The Chrome Reliquary",
	"The Sunken Archive Caravan",
	"The Ashfall Pilgrimage",
	"The Obsidian Vanguard",
	"The Glasslands Procession",
	"The Rustbound Legacy Train",
	"The Emberlit Wayfarers",
	"The Hollow Crown Convoy",
	"The Starlit Reliquary March",
}

func (e *Engine) relicConvoySpawn(ctx context.Context, tx *sql.Tx) error {
	var activeExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM relic_convoys WHERE claimed_by IS NULL AND expires_at > CURRENT_TIMESTAMP)
	`).Scan(&activeExists); err != nil {
		return fmt.Errorf("checking for an active relic convoy: %w", err)
	}
	if activeExists {
		return nil
	}
	if rand.Float64() >= relicConvoySpawnChance {
		return nil
	}

	var coordinateID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM coordinates ORDER BY random() LIMIT 1").Scan(&coordinateID); err != nil {
		return nil
	}

	relicName := relicNames[rand.Intn(len(relicNames))]
	reward := relicConvoyMinReward + rand.Float64()*(relicConvoyMaxReward-relicConvoyMinReward)

	var convoyID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO relic_convoys (relic_name, coordinate_id, reward_dollars, expires_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP + ($4 * INTERVAL '1 minute'))
		RETURNING id`,
		relicName, coordinateID, reward, relicConvoyWindowMinutes).Scan(&convoyID); err != nil {
		return fmt.Errorf("spawning relic convoy: %w", err)
	}

	broadcast := fmt.Sprintf(
		"🏺 RELIC CONVOY SIGHTED: %s has surfaced somewhere in the wasteland, carrying a fortune in pre-war salvage! First outpost to reach /relic and tap Claim keeps it all - %s.",
		relicName, fmt.Sprintf("$%.0f", reward))
	if err := notifications.QueueToAllPlayers(ctx, tx, broadcast, "general"); err != nil {
		return fmt.Errorf("broadcasting relic convoy spawn: %w", err)
	}
	return nil
}

func (e *Engine) relicConvoyExpire(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, relic_name FROM relic_convoys
		WHERE claimed_by IS NULL AND expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying expired relic convoys: %w", err)
	}
	type expired struct{ id, name string }
	var toExpire []expired
	for rows.Next() {
		var x expired
		if scanErr := rows.Scan(&x.id, &x.name); scanErr == nil {
			toExpire = append(toExpire, x)
		}
	}
	rows.Close()

	for _, x := range toExpire {
		if _, err := tx.ExecContext(ctx, "DELETE FROM relic_convoys WHERE id = $1", x.id); err != nil {
			return fmt.Errorf("clearing expired relic convoy %s: %w", x.id, err)
		}
		broadcast := fmt.Sprintf("🏺 %s has moved on, unclaimed - its fortune lost to the wasteland once more.", x.name)
		if err := notifications.QueueToAllPlayers(ctx, tx, broadcast, "general"); err != nil {
			return fmt.Errorf("broadcasting relic convoy expiry: %w", err)
		}
	}
	return nil
}
