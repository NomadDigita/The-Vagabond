package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

// runAIJobs mirrors the /jobs panel's resource-only utility actions for
// AI factions - project owner direction 2026-08-01: "every single thing
// a human can do." Split into its own file/phase rather than folded into
// growAICivilizations, since that function was already large and these
// actions are a genuinely separate category (one-off utility spends, not
// passive growth or governance) - matches how aispawning.go and
// aidecisions.go are already split out by concern.
//
// Deliberately covers only the resource-only actions from jobs.go:
// GatherSunlight, RepairUnits, RepairBuildings, HyperSpeed,
// OrbitalManeuver, ExtendPlanet. HandleTeleport is deliberately excluded
// - relocating already has a real equivalent for AI factions via
// maybeAIFlee/Ghost Protocol in aidecisions.go, and a second, unrelated
// relocation mechanic risks working against Item 1's continent-balanced
// spawn placement for no real benefit.
func (e *Engine) runAIJobs(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM encampments WHERE is_ai_faction = TRUE")
	if err != nil {
		return fmt.Errorf("querying AI factions for jobs: %w", err)
	}
	var factionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			factionIDs = append(factionIDs, id)
		}
	}
	rows.Close()

	for _, campID := range factionIDs {
		// Gather Sunlight: mirrors HandleGatherSunlight exactly - a free
		// 150 Electricity burst, gated by the same 30-minute
		// last_sunlight_at cooldown a human's own repeated taps would
		// hit.
		if rand.Float64() < 0.02 {
			var lastSunlight sql.NullTime
			_ = tx.QueryRowContext(ctx, "SELECT last_sunlight_at FROM encampments WHERE id = $1", campID).Scan(&lastSunlight)
			if !lastSunlight.Valid || time.Since(lastSunlight.Time) >= 30*time.Minute {
				const gain = 150.0
				var curElectricity float64
				_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1", campID).Scan(&curElectricity)
				cap := storagecap.CapFor(ctx, tx, campID)
				newElectricity, _ := storagecap.Clamp(curElectricity, gain, cap)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = $1 WHERE encampment_id = $2", newElectricity, campID)
				_, _ = tx.ExecContext(ctx, "UPDATE encampments SET last_sunlight_at = CURRENT_TIMESTAMP WHERE id = $1", campID)
			}
		}

		// Repair Units: mirrors HandleRepairUnits exactly - 200 Scrap
		// for +5 Soldiers.
		if rand.Float64() < 0.02 {
			const cost = 200.0
			const repaired = 5
			var scrap float64
			_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)
			if scrap >= cost {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1 WHERE encampment_id = $2", repaired, campID)
			}
		}

		// Repair Buildings (rush construction): mirrors
		// HandleRepairBuildings exactly - 150 Scrap halves the
		// remaining time on whichever module is currently upgrading.
		// Only relevant now that AI factions actually queue module
		// upgrades (see growAICivilizations's facility-upgrade block),
		// so this naturally does nothing until that's true for a given
		// faction - no ordering dependency needed between the two.
		if rand.Float64() < 0.02 {
			const cost = 150.0
			var moduleID string
			var readyAt time.Time
			err := tx.QueryRowContext(ctx, "SELECT id, upgrade_ready_at FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE ORDER BY upgrade_ready_at ASC LIMIT 1 FOR UPDATE", campID).Scan(&moduleID, &readyAt)
			if err == nil {
				var scrap float64
				_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)
				if scrap >= cost {
					remaining := time.Until(readyAt)
					newReady := readyAt.Add(-remaining / 2)
					_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
					_, _ = tx.ExecContext(ctx, "UPDATE modules SET upgrade_ready_at = $1 WHERE id = $2", newReady, moduleID)
				}
			}
		}

		// HyperSpeed: mirrors HandleHyperSpeed exactly - 300
		// Electricity halves the remaining time on the AI faction's
		// nearest active raid/mission. Only relevant once a faction has
		// an outbound raid via launchAIRaid, same non-dependency
		// reasoning as Repair Buildings above.
		if rand.Float64() < 0.02 {
			const cost = 300.0
			var raidID string
			var resolveTime time.Time
			err := tx.QueryRowContext(ctx, "SELECT id, resolve_time FROM raids WHERE attacker_id = $1 AND state IN ('marching','engaged','returning') ORDER BY resolve_time ASC LIMIT 1 FOR UPDATE", campID).Scan(&raidID, &resolveTime)
			if err == nil && time.Until(resolveTime) >= time.Minute {
				var electricity float64
				_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
				if electricity >= cost {
					remaining := time.Until(resolveTime)
					newResolve := resolveTime.Add(-remaining / 2)
					_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID)
					_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", newResolve, raidID)
				}
			}
		}

		// Orbital Maneuver: mirrors HandleOrbitalManeuver exactly - 400
		// Electricity for a +30% defense buff (orbital_buff_until) for
		// 2 hours. A faction under raid pressure benefits from this the
		// same way a human under pressure would reach for it.
		if rand.Float64() < 0.01 {
			const cost = 400.0
			const buffDuration = 2 * time.Hour
			var electricity float64
			_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
			if electricity >= cost {
				buffUntil := time.Now().UTC().Add(buffDuration)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID)
				_, _ = tx.ExecContext(ctx, "UPDATE encampments SET orbital_buff_until = $1 WHERE id = $2", buffUntil, campID)
			}
		}

		// Extend Planet: mirrors HandleExtendPlanet exactly - a
		// permanent +1000 storage cap, cost scaling with
		// extension_lvl+1 (500 Metal / 100 Crystal at level 0,
		// doubling each level). Deliberately the rarest of these rolls -
		// it's the human "Jobs" panel's biggest, least-frequent
		// investment too, not something tapped casually.
		if rand.Float64() < 0.005 {
			var extensionLvl int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(extension_lvl,0) FROM encampments WHERE id = $1 FOR UPDATE", campID).Scan(&extensionLvl)
			metalCost := float64(500 * (extensionLvl + 1))
			crystalCost := float64(100 * (extensionLvl + 1))
			var metal, crystal float64
			_ = tx.QueryRowContext(ctx, "SELECT metal, crystal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&metal, &crystal)
			if metal >= metalCost && crystal >= crystalCost {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - $1, crystal = crystal - $2 WHERE encampment_id = $3", metalCost, crystalCost, campID)
				_, _ = tx.ExecContext(ctx, "UPDATE encampments SET extension_lvl = extension_lvl + 1 WHERE id = $1", campID)
			}
		}
	}

	return nil
}
