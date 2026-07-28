package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/roadcombat"
)

// evaluateRoadBaseEncounters completes MMO_WORLD_EVOLUTION_PLAN.md Phase 4
// milestone 2 ("expeditions and bases"): discoverRouteContacts already
// reveals a passive base to a passing expedition, but never forced a
// decision - a column would just keep marching. This mirrors
// evaluateRoadEncounters (expedition-vs-expedition) but against a
// stationary, real-player-owned base instead of a second moving column.
// AI/seeded settlements are excluded via encampments.is_ai_faction = FALSE
// (they run their own recon/skirmish flow; see resolveExplorationDiscovery
// / Phase 2). Earlier versions of this query tried to exclude them by
// requiring a JOIN on users, on the assumption AI-owned encampments had no
// real users row - Phase 6 (persistent AI civilizations, built afterward)
// actually gives each AI faction a genuine users row (required by
// encampments.user_id's FK/UNIQUE constraint), which silently broke that
// assumption. Caught by TestEvaluateRoadBaseEncountersExcludesAIFactionBases.
//
// Runs its own fresh query rather than reusing evaluateRoadEncounters'
// in-memory movers slice, since that slice may already be stale within the
// same tick (a raid resolved into an expedition-vs-expedition encounter
// there is no longer eligible here).
func (e *Engine) evaluateRoadBaseEncounters(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, r.attacker_id, ea.user_id, ea.name, r.state,
		       COALESCE(r.leg_started_at, r.created_at, CURRENT_TIMESTAMP),
		       COALESCE(r.leg_total_minutes, r.base_march_minutes, 15.0),
		       r.origin_x, r.origin_y, r.destination_x, r.destination_y,
		       r.origin_region, r.destination_region
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		WHERE r.state IN ('marching', 'returning')
		  AND r.movement_state = 'moving'
		  AND r.active_encounter_id IS NULL
		  AND r.active_base_encounter_id IS NULL
		  AND r.origin_x IS NOT NULL AND r.destination_x IS NOT NULL
		  AND r.origin_region IS NOT NULL AND r.destination_region IS NOT NULL
		ORDER BY r.id`)
	if err != nil {
		return fmt.Errorf("querying road-base-encounter candidates: %w", err)
	}
	defer rows.Close()

	var movers []roadMover
	for rows.Next() {
		var m roadMover
		var legStartedAt time.Time
		var legTotalMinutes float64
		var ox, oy, dx, dy int
		var originRegion, destRegion string
		if err := rows.Scan(&m.raidID, &m.attackerID, &m.attackerUserID, &m.attackerName, &m.state,
			&legStartedAt, &legTotalMinutes, &ox, &oy, &dx, &dy, &originRegion, &destRegion); err != nil {
			return fmt.Errorf("scanning road-base-encounter candidate: %w", err)
		}
		progress := roadcombat.RouteProgress(legStartedAt.UTC(), legTotalMinutes, time.Now().UTC())
		m.pos = roadcombat.CurrentPosition(m.state, ox, oy, dx, dy, progress)
		m.region = originRegion
		if progress >= 0.5 {
			m.region = destRegion
		}
		movers = append(movers, m)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating road-base-encounter candidates: %w", err)
	}

	for _, m := range movers {
		var baseID, baseName string
		var baseUserID int64
		var baseX, baseY int
		err := tx.QueryRowContext(ctx, `
			SELECT e.id, e.name, e.user_id, c.x, c.y
			FROM encampments e
			JOIN coordinates c ON c.id = e.coordinate_id
			WHERE e.id <> $1
			  AND e.is_ai_faction = FALSE
			  AND c.region = $2
			  AND ABS(c.x - $3) <= $4 AND ABS(c.y - $5) <= $4
			ORDER BY ABS(c.x - $3) + ABS(c.y - $5)
			LIMIT 1`,
			m.attackerID, m.region, int(m.pos.X), int(roadcombat.EncounterRadius), int(m.pos.Y)).
			Scan(&baseID, &baseName, &baseUserID, &baseX, &baseY)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return fmt.Errorf("locating road-base-encounter candidate: %w", err)
		}
		if !roadcombat.InEncounterRange(m.pos, roadcombat.Position{X: float64(baseX), Y: float64(baseY)}) {
			continue
		}

		// Cooldown: same shape as evaluateRoadEncounters, so a
		// "Continue" past this base isn't immediately re-asked.
		var recentCount int
		_ = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM road_base_encounters
			WHERE raid_id = $1 AND encampment_id = $2
			  AND status = 'resolved' AND resolved_at > $3`,
			m.raidID, baseID, time.Now().UTC().Add(-roadcombat.EncounterCooldown)).Scan(&recentCount)
		if recentCount > 0 {
			continue
		}

		if rand.Float64() >= roadcombat.EncounterRollChance {
			continue
		}

		deadline := time.Now().UTC().Add(roadcombat.ResponseWindow)
		var encounterID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO road_base_encounters (raid_id, encampment_id, location_x, location_y, response_deadline)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
			RETURNING id`, m.raidID, baseID, baseX, baseY, deadline).Scan(&encounterID)
		if err != nil {
			if err == sql.ErrNoRows {
				continue // another tick worker already created this pending encounter
			}
			return fmt.Errorf("creating road-base encounter: %w", err)
		}

		// Freeze the column exactly like an expedition-vs-expedition
		// encounter - no arrival/return processing until resolved.
		_, _ = tx.ExecContext(ctx, `
			UPDATE raids SET movement_state = 'encounter_pending', active_base_encounter_id = $1, paused_at = CURRENT_TIMESTAMP
			WHERE id = $2`, encounterID, m.raidID)

		// Discovery is reciprocal even if nobody chooses to fight,
		// matching discoverRouteContacts' existing behavior.
		_, _ = tx.ExecContext(ctx, `INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'route') ON CONFLICT DO NOTHING`, m.attackerID, baseID)
		_, _ = tx.ExecContext(ctx, `INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'route') ON CONFLICT DO NOTHING`, baseID, m.attackerID)

		deadlineSeconds := int(roadcombat.ResponseWindow.Seconds())
		if isRealPlayer(m.attackerUserID) {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", m.attackerUserID,
				fmt.Sprintf("🚧 %s: Your expedition passed close to Outpost %s.\nYou have %s to decide: Attack, or Continue on your way (open your ⚔️ Expedition Radar to choose). Taking no action lets your column pass it peacefully.",
					htmlBoldTick("ROAD CONTACT"), htmlCodeTick(htmlEscapeTick(baseName)), htmlCodeTick(fmt.Sprintf("%ds", deadlineSeconds))))
		}
		if baseUserID != 0 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", baseUserID,
				fmt.Sprintf("📡 %s: A foreign expedition commanded by %s passed close to your outpost. It may attack; your home garrison stands ready.",
					htmlBoldTick("ROAD CONTACT"), htmlCodeTick(htmlEscapeTick(m.attackerName))))
		}
	}

	return e.expireRoadBaseEncounters(ctx, tx)
}

// expireRoadBaseEncounters resolves any pending base encounter whose
// response window has lapsed as a peaceful pass, and unfreezes the column
// from exactly the position it paused at - same shape as
// expireRoadEncounters/resolveEncounterAsContinue for expedition-vs-expedition.
func (e *Engine) expireRoadBaseEncounters(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, raid_id FROM road_base_encounters
		WHERE status = 'pending' AND response_deadline <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying expired road-base encounters: %w", err)
	}

	type expired struct {
		id     string
		raidID string
	}
	var list []expired
	for rows.Next() {
		var x expired
		if err := rows.Scan(&x.id, &x.raidID); err == nil {
			list = append(list, x)
		}
	}
	rows.Close()

	for _, x := range list {
		_, err := tx.ExecContext(ctx, `
			UPDATE road_base_encounters SET status = 'resolved', outcome = 'pass', resolved_at = CURRENT_TIMESTAMP
			WHERE id = $1 AND status = 'pending'`, x.id)
		if err != nil {
			return fmt.Errorf("expiring road-base encounter: %w", err)
		}
		_, _ = tx.ExecContext(ctx, `
			UPDATE raids
			SET movement_state = 'moving', active_base_encounter_id = NULL,
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    paused_at = NULL
			WHERE id = $1 AND active_base_encounter_id = $2`, x.raidID, x.id)

		var userID int64
		var attackerID string
		if err := tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", x.raidID).Scan(&attackerID); err == nil {
			_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", attackerID).Scan(&userID)
			if userID != 0 {
				_ = notifications.Queue(ctx, tx, userID,
					"🛣️ "+htmlBoldTick("ROAD CONTACT RESOLVED")+": Your column continued on its way without engaging the outpost.", "route_status")
			}
		}
	}
	return nil
}
