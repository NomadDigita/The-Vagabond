package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"
)

// aiFactionSpawnInterval is how often exactly one new AI faction is
// spawned into the world. Set to 24h/3 so three new factions appear per
// day, per the project owner's direct answer 2026-08-01 to
// AI_AND_SCOUTING_EXPANSION_PLAN.md item 1's open question ("3/day").
// Deliberately a fixed interval rather than a daily-quota-with-reset
// scheme - that avoids any day-boundary ambiguity (which timezone counts
// as "today," whether a quota resets mid-tick) while producing the same
// long-run rate, and mirrors how aiDecisionCadence already handles
// per-faction pacing with a plain interval rather than a calendar quota.
const aiFactionSpawnInterval = 24 * time.Hour / 3

// aiFactionSpawnStartingLevel is deliberately the weakest tier in the
// game - even lower than the original 8 seeded factions' weaker starting
// level of 3 (see seedAICivilizations in cmd/bot/main.go) - so a
// continuously growing AI population doesn't front-load the world with
// instantly-dangerous new threats. growAICivilizations' existing passive
// leveling ramps a spawn up over time exactly like it already does for
// the original 8; there's nothing spawn-specific about how a faction
// grows once it exists, and nothing spawn-specific about how
// decideAIFactionActions scouts or raids with it either.
const aiFactionSpawnStartingLevel = 1

// aiFactionSpawnRegions mirrors the four-quadrant continent scheme
// seedAICivilizations uses (coordinates.region), cycled round-robin by
// the spawn counter below so growth stays roughly balanced across
// continents rather than clustering in whichever one gets rolled most.
var aiFactionSpawnRegions = []string{"Africa", "Europe", "Asia", "Americas"}

// aiFactionSpawnPrefixes and aiFactionSpawnSuffixes generate a flavor
// name in the same two-word "Adjective Faction-type" shape as the
// original 8 hand-authored names (e.g. "Ironclad Directive", "Sandrunner
// Clan") via a simple combiner rather than a much larger authored list -
// ai_faction_key, not the display name, is what guarantees uniqueness,
// so two spawned factions sharing a display name is harmless flavor
// overlap, not a bug.
var aiFactionSpawnPrefixes = []string{
	"Ironclad", "Sandrunner", "Veridian", "Ashfall", "Lotus", "Crimson",
	"Frontier", "Dustbowl", "Scorched", "Rustbelt", "Obsidian", "Wraith",
	"Voidmark", "Emberline", "Hollowpoint", "Graywater", "Blacksun",
	"Ferrous", "Nightshade", "Copperhead",
}

var aiFactionSpawnSuffixes = []string{
	"Directive", "Clan", "Compact", "Remnant", "Dominion", "Syndicate",
	"Collective", "Militia", "Coalition", "Vanguard", "Legion", "Cartel",
	"Accord", "Enclave", "Warband", "Assembly", "Concord", "Brigade",
	"Consortium", "Pact",
}

// spawnNewAIFactions is Phase 6's continuous-growth counterpart to
// seedAICivilizations' one-time boot seed - see
// AI_AND_SCOUTING_EXPANSION_PLAN.md item 1 for the full design record,
// including the project owner's exact answer this implements. A brand
// new AI faction is created as nothing but another is_ai_faction = TRUE
// encampments row with matching users/resources/workshop_inventory rows
// - the same shape seedAICivilizations produces - so it needs zero
// special-casing anywhere else: growAICivilizations grows it,
// decideAIFactionActions scouts/raids with it, and a human's own
// exploration can discover it exactly like the original 8.
//
// world_state.id = 1's last_ai_spawn_at is the world-scoped (not
// per-faction) cadence gate; ai_factions_spawned_count both records how
// many have spawned and supplies each new faction's unique
// ai_faction_key/telegram_id, so it's read/written under the same
// FOR UPDATE lock as the cadence check to stay race-free across
// concurrent tick workers, mirroring how decideAIFactionActions guards
// against a double-decision with FOR UPDATE OF e.
func (e *Engine) spawnNewAIFactions(ctx context.Context, tx *sql.Tx) error {
	var lastSpawnAt sql.NullTime
	var spawnedCount int
	if err := tx.QueryRowContext(ctx, "SELECT last_ai_spawn_at, ai_factions_spawned_count FROM world_state WHERE id = 1 FOR UPDATE").Scan(&lastSpawnAt, &spawnedCount); err != nil {
		return fmt.Errorf("loading world_state for AI spawn cadence: %w", err)
	}
	if lastSpawnAt.Valid && time.Since(lastSpawnAt.Time) < aiFactionSpawnInterval {
		return nil // not due yet
	}

	region := aiFactionSpawnRegions[spawnedCount%len(aiFactionSpawnRegions)]
	name := fmt.Sprintf("%s %s", aiFactionSpawnPrefixes[rand.Intn(len(aiFactionSpawnPrefixes))], aiFactionSpawnSuffixes[rand.Intn(len(aiFactionSpawnSuffixes))])
	key := fmt.Sprintf("ai_spawn_%d", spawnedCount+1)
	// Comfortably clear of the hand-seeded -900001..-900008 range so the
	// two schemes can never collide.
	telegramID := int64(-2000000 - spawnedCount)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (telegram_id, username, first_name, state, faction)
		VALUES ($1, $2, $2, 'ai_faction', 'ai')
		ON CONFLICT (telegram_id) DO NOTHING`, telegramID, name); err != nil {
		return fmt.Errorf("seeding user for spawned AI faction: %w", err)
	}

	var x, y int
	var coordID string
	placed := false
	for attempt := 0; attempt < 100; attempt++ {
		switch region {
		case "Africa":
			x = rand.Intn(991) + 10
			y = rand.Intn(991) + 10
		case "Europe":
			x = -(rand.Intn(991) + 10)
			y = rand.Intn(991) + 10
		case "Asia":
			x = rand.Intn(991) + 10
			y = -(rand.Intn(991) + 10)
		default:
			x = -(rand.Intn(991) + 10)
			y = -(rand.Intn(991) + 10)
		}
		err := tx.QueryRowContext(ctx, `
			INSERT INTO coordinates (x, y, biome, danger_level, region, terrain)
			VALUES ($1, $2, 'wasteland', $3, $4, 'plains')
			ON CONFLICT (x, y) DO NOTHING
			RETURNING id`, x, y, aiFactionSpawnStartingLevel, region).Scan(&coordID)
		if err == nil {
			placed = true
			break
		}
	}
	if !placed {
		return fmt.Errorf("spawning new AI faction: no free coordinate found in %s after 100 attempts", region)
	}

	var campID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO encampments (user_id, name, coordinate_id, level, is_ai_faction, ai_faction_key)
		VALUES ($1, $2, $3, $4, TRUE, $5)
		RETURNING id`, telegramID, name, coordID, aiFactionSpawnStartingLevel, key).Scan(&campID); err != nil {
		return fmt.Errorf("creating spawned AI faction encampment: %w", err)
	}

	startScrap := float64(aiFactionSpawnStartingLevel) * 400.0
	startMetal := float64(aiFactionSpawnStartingLevel) * 200.0
	startRations := float64(aiFactionSpawnStartingLevel) * 150.0
	startElectricity := float64(aiFactionSpawnStartingLevel) * 100.0
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO resources (encampment_id, scrap, metal, rations, electricity, crystal)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		campID, startScrap, startMetal, startRations, startElectricity, float64(aiFactionSpawnStartingLevel)*0.5); err != nil {
		return fmt.Errorf("seeding resources for spawned AI faction: %w", err)
	}

	startSoldiers := aiFactionSpawnStartingLevel * 15
	startMechs := aiFactionSpawnStartingLevel * 2
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workshop_inventory (encampment_id, soldiers, mechs, buggies)
		VALUES ($1, $2, $3, 1)`, campID, startSoldiers, startMechs); err != nil {
		return fmt.Errorf("seeding garrison for spawned AI faction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "UPDATE world_state SET last_ai_spawn_at = CURRENT_TIMESTAMP, ai_factions_spawned_count = ai_factions_spawned_count + 1 WHERE id = 1"); err != nil {
		return fmt.Errorf("updating AI spawn cadence: %w", err)
	}

	// World-visible but low-key flavor, same spirit as an AI raid's
	// world_news headline in launchAIRaid - a new outpost appearing is
	// public information, not a targeted alert, so it doesn't go through
	// the notifications.QueueToRegion path weather events use.
	_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)",
		fmt.Sprintf("📡 NEW SETTLEMENT DETECTED: Outpost [%s] has established itself over %s.", name, region))

	return nil
}
