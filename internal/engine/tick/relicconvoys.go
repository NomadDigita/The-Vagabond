package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
)

// RARE_WORLD_FEATURES_PLAN.md Phase 1: Relic Convoys.
//
// A vanishingly rare, server-wide race: a pre-war relic convoy surfaces
// at a random coordinate, broadcast to every player at once, and the
// first person to tap Claim (see internal/bot/handlers/relicconvoy.go)
// wins a permanent title plus a cash windfall. There is no travel time
// and no coordinate-proximity requirement - the entire mechanic IS the
// race, deliberately kept mechanically simple so the "rare and
// exciting" read isn't buried under friction. See the plan doc's
// section 1 for the full design rationale.

// relicConvoySpawnChance is the per-tick probability of a new relic
// convoy spawning, checked only when no convoy is currently active (see
// relicConvoySpawn below) - so this is genuinely "how rare is the next
// one", not diluted by a backlog. At the tick engine's ~60s cadence,
// 0.15% averages roughly one spawn every ~11 hours of continuous
// uptime - a real "stop what you're doing" moment, not background hum.
//
// A var, not a const, so relicconvoys_test.go can deterministically
// force a spawn (set to 1.0, restored via t.Cleanup) instead of
// looping thousands of real-Postgres round trips waiting for a
// legitimate 0.15% roll to land.
var relicConvoySpawnChance = 0.0015

// relicConvoyWindowMinutes is how long an unclaimed relic convoy stays
// live before vanishing - long enough that players across timezones/
// session patterns have a real shot, short enough to stay an event
// rather than a standing fixture.
const relicConvoyWindowMinutes = 3 * 60.0

// relicConvoyMinReward/relicConvoyMaxReward bound the cash windfall a
// claimed relic pays out. Scaled off the game's own Crystal-conversion
// economy (crystal_exchange.go's dollars rate of 2000/Crystal) rather
// than an arbitrary number - roughly 6-10 Crystal's worth of cash for
// doing nothing but tapping first.
const (
	relicConvoyMinReward = 12000.0
	relicConvoyMaxReward = 20000.0
)

// relicNames is the curated flavor pool a new convoy's name is drawn
// from, so the broadcast headline reads as a named, memorable event
// ("The Last Aurora Convoy is passing through...") rather than a
// generic "convoy #4821".
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

// relicConvoySpawn is the "relic_convoy_spawn" tick phase - see
// engine.go's ProcessTick phase list. Rolls a new convoy into existence
// only when none is currently active, so at most one is ever live at a
// time, server-wide.
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
		// No seeded world geography yet (e.g. a brand new deployment) -
		// nothing to anchor a convoy to, just skip this tick rather
		// than erroring the whole phase.
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

// relicConvoyExpire is the "relic_convoy_expire" tick phase - clears
// convoys nobody claimed in time, with a "moved on" broadcast, matching
// the weather engine's own "conditions have cleared" pattern rather
// than silently vanishing.
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
