package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

func (e *Engine) runAIJobs(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT id FROM encampments WHERE is_ai_faction = TRUE AND is_defeated = FALSE")
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
