package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"
)

const aiFactionSpawnInterval = 24 * time.Hour / 3

const aiFactionSpawnStartingLevel = 1

var aiFactionSpawnRegions = []string{"Africa", "Europe", "Asia", "Americas"}

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

func (e *Engine) spawnNewAIFactions(ctx context.Context, tx *sql.Tx) error {
	var lastSpawnAt sql.NullTime
	var spawnedCount int
	if err := tx.QueryRowContext(ctx, "SELECT last_ai_spawn_at, ai_factions_spawned_count FROM world_state WHERE id = 1 FOR UPDATE").Scan(&lastSpawnAt, &spawnedCount); err != nil {
		return fmt.Errorf("loading world_state for AI spawn cadence: %w", err)
	}
	if lastSpawnAt.Valid && time.Since(lastSpawnAt.Time) < aiFactionSpawnInterval {
		return nil
	}

	region := aiFactionSpawnRegions[spawnedCount%len(aiFactionSpawnRegions)]
	name := fmt.Sprintf("%s %s", aiFactionSpawnPrefixes[rand.Intn(len(aiFactionSpawnPrefixes))], aiFactionSpawnSuffixes[rand.Intn(len(aiFactionSpawnSuffixes))])
	key := fmt.Sprintf("ai_spawn_%d", spawnedCount+1)
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

	_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)",
		fmt.Sprintf("📡 NEW SETTLEMENT DETECTED: Outpost [%s] has established itself over %s.", name, region))

	return nil
}
