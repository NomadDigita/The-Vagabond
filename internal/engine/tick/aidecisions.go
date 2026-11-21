package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"time"
)

const aiDecisionCadence = 20 * time.Minute

const aiRaidProbabilityWhenEligible = 0.40

const aiVsAIRaidProbabilityWhenEligible = 0.35

const aiFractionOfGarrisonCommitted = 0.65

const (
	aiFairnessNormalBandAbove = 3
	aiFairnessWideBandAbove   = 8
	aiFairnessWideBandChance  = 0.25
)

const (
	aiOverdueRaidThreshold  = 10 * time.Hour
	aiOverdueMinTargetLevel = 4
)

func (e *Engine) decideAIFactionActions(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT e.id, e.level, c.region
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.is_ai_faction = TRUE AND e.is_defeated = FALSE
		  AND (e.ai_last_decision_at IS NULL OR e.ai_last_decision_at <= CURRENT_TIMESTAMP - ($1 * INTERVAL '1 minute'))
		FOR UPDATE OF e`, aiDecisionCadence.Minutes())
	if err != nil {
		return fmt.Errorf("querying AI factions due for a decision: %w", err)
	}

	type dueFaction struct {
		id     string
		level  int
		region string
	}
	var due []dueFaction
	for rows.Next() {
		var f dueFaction
		if err := rows.Scan(&f.id, &f.level, &f.region); err == nil {
			due = append(due, f)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating AI factions due for a decision: %w", err)
	}

	for _, f := range due {
		if _, err := tx.ExecContext(ctx, "UPDATE encampments SET ai_last_decision_at = CURRENT_TIMESTAMP WHERE id = $1", f.id); err != nil {
			return fmt.Errorf("updating AI decision cadence for %s: %w", f.id, err)
		}

		if err := e.decideOneAIFaction(ctx, tx, f.id, f.level, f.region); err != nil {
			return fmt.Errorf("deciding action for AI faction %s: %w", f.id, err)
		}
	}
	return nil
}

type aiRaidTarget struct {
	id          string
	name        string
	level       int
	x, y        int
	region      string
	userID      int64
	isAIFaction bool
}

func (e *Engine) decideOneAIFaction(ctx context.Context, tx *sql.Tx, factionID string, factionLevel int, region string) error {
	fled, err := e.maybeAIFlee(ctx, tx, factionID)
	if err != nil {
		return fmt.Errorf("checking whether to flee: %w", err)
	}
	if fled {
		return nil
	}

	var factionClanID sql.NullString
	_ = tx.QueryRowContext(ctx, "SELECT clan_id FROM user_clans uc JOIN encampments e ON e.user_id = uc.user_id WHERE e.id = $1", factionID).Scan(&factionClanID)

	if overdue, err := e.pickOverdueRaidTarget(ctx, tx, factionID, factionLevel, factionClanID); err != nil {
		return fmt.Errorf("selecting overdue raid target: %w", err)
	} else if overdue != nil {
		return e.launchAIRaid(ctx, tx, factionID, *overdue)
	}

	target, err := e.pickFairAIRaidTarget(ctx, tx, factionID, factionLevel, factionClanID)
	if err != nil {
		return fmt.Errorf("selecting raid target: %w", err)
	}

	if target != nil {
		raidProbability := aiRaidProbabilityWhenEligible
		if target.isAIFaction {
			raidProbability = aiVsAIRaidProbabilityWhenEligible
		}
		if rand.Float64() < raidProbability {
			return e.launchAIRaid(ctx, tx, factionID, *target)
		}
	}

	undiscoveredCount, err := e.countUndiscoveredTargets(ctx, tx, factionID, region)
	if err != nil {
		return fmt.Errorf("counting undiscovered targets: %w", err)
	}
	if undiscoveredCount > 0 {
		return e.aiScout(ctx, tx, factionID, region)
	}

	reason := "no fair raid target and nothing left to discover in this continent"
	if target != nil {
		reason = "had a fair raid target but the probability roll missed this cycle"
	}
	_, _ = tx.ExecContext(ctx, "INSERT INTO ai_faction_decisions (encampment_id, intent, reason) VALUES ($1, 'idle', $2)", factionID, reason)
	return nil
}

func (e *Engine) countUndiscoveredTargets(ctx context.Context, tx *sql.Tx, observerCampID, region string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id <> $1
		  AND c.region = $2
		  AND NOT EXISTS (
				SELECT 1 FROM encampment_discoveries d
				WHERE d.observer_encampment_id = $1 AND d.target_encampment_id = e.id
			)`, observerCampID, region).Scan(&count)
	return count, err
}

func (e *Engine) aiScout(ctx context.Context, tx *sql.Tx, factionID, region string) error {
	var targetID, targetName string
	err := tx.QueryRowContext(ctx, `
		SELECT e.id, e.name
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id <> $1
		  AND c.region = $2
		  AND NOT EXISTS (
				SELECT 1 FROM encampment_discoveries d
				WHERE d.observer_encampment_id = $1 AND d.target_encampment_id = e.id
			)
		ORDER BY c.danger_level ASC, e.established_at ASC
		LIMIT 1`, factionID, region).Scan(&targetID, &targetName)
	if err == sql.ErrNoRows {
		_, _ = tx.ExecContext(ctx, "INSERT INTO ai_faction_decisions (encampment_id, intent, reason) VALUES ($1, 'idle', 'scout intent chosen but no undiscovered target remained by the time of selection')", factionID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("selecting scout target: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method)
		VALUES ($1, $2, 'ai_scout') ON CONFLICT DO NOTHING`, factionID, targetID)
	if err != nil {
		return fmt.Errorf("recording AI scout discovery: %w", err)
	}
	affected, _ := result.RowsAffected()
	reason := fmt.Sprintf("discovered %s", targetName)
	if affected == 0 {
		reason = fmt.Sprintf("attempted to discover %s but it was already discovered", targetName)
	}
	_, _ = tx.ExecContext(ctx, "INSERT INTO ai_faction_decisions (encampment_id, intent, target_encampment_id, reason) VALUES ($1, 'scout', $2, $3)", factionID, targetID, reason)
	return nil
}

func (e *Engine) pickFairAIRaidTarget(ctx context.Context, tx *sql.Tx, factionID string, factionLevel int, factionClanID sql.NullString) (*aiRaidTarget, error) {
	band := aiFairnessNormalBandAbove
	if rand.Float64() < aiFairnessWideBandChance {
		band = aiFairnessWideBandAbove
	}
	maxLevel := factionLevel + band

	rows, err := tx.QueryContext(ctx, `
		SELECT e.id, e.name, e.level, c.x, c.y, c.region, e.user_id, e.is_ai_faction
		FROM encampment_discoveries d
		JOIN encampments e ON e.id = d.target_encampment_id
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE d.observer_encampment_id = $1
		  AND e.level >= $2
		  AND e.level <= $3
		  AND NOT EXISTS (
				SELECT 1 FROM raids r
				WHERE r.attacker_id = $1 AND r.defender_id = e.id
				  AND r.state IN ('marching', 'engaged')
			)
		  AND NOT EXISTS (
				SELECT 1 FROM user_clans defender_membership
				JOIN clan_diplomacy cd ON (
					(cd.clan_a_id = $4 AND cd.clan_b_id = defender_membership.clan_id) OR
					(cd.clan_b_id = $4 AND cd.clan_a_id = defender_membership.clan_id)
				)
				WHERE defender_membership.user_id = e.user_id AND cd.status = 'active'
			)
		ORDER BY random()
		LIMIT 5`, factionID, factionLevel, maxLevel, factionClanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t aiRaidTarget
		if err := rows.Scan(&t.id, &t.name, &t.level, &t.x, &t.y, &t.region, &t.userID, &t.isAIFaction); err == nil {
			return &t, nil
		}
	}
	return nil, nil
}

func (e *Engine) pickOverdueRaidTarget(ctx context.Context, tx *sql.Tx, factionID string, factionLevel int, factionClanID sql.NullString) (*aiRaidTarget, error) {
	minLevel := factionLevel
	if aiOverdueMinTargetLevel > minLevel {
		minLevel = aiOverdueMinTargetLevel
	}

	var t aiRaidTarget
	err := tx.QueryRowContext(ctx, `
		SELECT e.id, e.name, e.level, c.x, c.y, c.region, e.user_id, e.is_ai_faction
		FROM encampment_discoveries d
		JOIN encampments e ON e.id = d.target_encampment_id
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE d.observer_encampment_id = $1
		  AND e.is_ai_faction = FALSE
		  AND e.level >= $2
		  AND NOT EXISTS (
				SELECT 1 FROM raids r
				WHERE r.attacker_id = $1 AND r.defender_id = e.id
				  AND r.state IN ('marching', 'engaged')
			)
		  AND NOT EXISTS (
				SELECT 1 FROM raids r
				WHERE r.defender_id = e.id
				  AND r.created_at > CURRENT_TIMESTAMP - ($3 * INTERVAL '1 minute')
			)
		  AND NOT EXISTS (
				SELECT 1 FROM user_clans defender_membership
				JOIN clan_diplomacy cd ON (
					(cd.clan_a_id = $4 AND cd.clan_b_id = defender_membership.clan_id) OR
					(cd.clan_b_id = $4 AND cd.clan_a_id = defender_membership.clan_id)
				)
				WHERE defender_membership.user_id = e.user_id AND cd.status = 'active'
			)
		ORDER BY random()
		LIMIT 1`, factionID, minLevel, aiOverdueRaidThreshold.Minutes(), factionClanID).
		Scan(&t.id, &t.name, &t.level, &t.x, &t.y, &t.region, &t.userID, &t.isAIFaction)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

const (
	aiFleeGarrisonThreshold = 10
	aiFleeCooldown          = 90 * 24 * time.Hour
	aiFleeCostFraction      = 0.50
)

func (e *Engine) maybeAIFlee(ctx context.Context, tx *sql.Tx, factionID string) (bool, error) {
	var soldiers, mechs int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers,0), COALESCE(mechs,0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", factionID).Scan(&soldiers, &mechs); err != nil {
		return false, fmt.Errorf("loading garrison for flee check: %w", err)
	}
	if soldiers+mechs >= aiFleeGarrisonThreshold {
		return false, nil
	}

	var lastGhost sql.NullTime
	if err := tx.QueryRowContext(ctx, "SELECT last_ghost_protocol_at FROM encampments WHERE id = $1", factionID).Scan(&lastGhost); err != nil {
		return false, fmt.Errorf("loading last Ghost Protocol time: %w", err)
	}
	if lastGhost.Valid && time.Since(lastGhost.Time) < aiFleeCooldown {
		return false, nil
	}

	var scrap, metal, crystal, dollars float64
	if err := tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", factionID).Scan(&scrap, &metal, &crystal, &dollars); err != nil {
		return false, fmt.Errorf("loading resources for flee cost: %w", err)
	}

	newX := rand.Intn(10000)
	newY := rand.Intn(10000)
	var newCoordID string
	err := tx.QueryRowContext(ctx, `
		INSERT INTO coordinates (x, y, biome, danger_level, region, terrain)
		VALUES ($1, $2, 'wasteland', 1, 'Unknown Sector', 'flat')
		ON CONFLICT (x, y) DO UPDATE SET x = EXCLUDED.x
		RETURNING id`, newX, newY).Scan(&newCoordID)
	if err != nil {
		return false, fmt.Errorf("finding new coordinates to flee to: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1, metal = metal - $2, crystal = crystal - $3, dollars = dollars - $4 WHERE encampment_id = $5",
		scrap*aiFleeCostFraction, metal*aiFleeCostFraction, crystal*aiFleeCostFraction, dollars*aiFleeCostFraction, factionID); err != nil {
		return false, fmt.Errorf("deducting flee cost: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE encampments SET coordinate_id = $1, last_ghost_protocol_at = CURRENT_TIMESTAMP WHERE id = $2", newCoordID, factionID); err != nil {
		return false, fmt.Errorf("relocating fleeing faction: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM known_locations WHERE target_encampment_id = $1", factionID); err != nil {
		return false, fmt.Errorf("clearing intel locks on fleeing faction: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO ai_faction_decisions (encampment_id, intent, reason) VALUES ($1, 'flee', $2)",
		factionID, fmt.Sprintf("garrison reduced to %d soldiers + %d mechs, invoked Ghost Protocol", soldiers, mechs)); err != nil {
		return false, fmt.Errorf("logging flee decision: %w", err)
	}

	return true, nil
}

func (e *Engine) launchAIRaid(ctx context.Context, tx *sql.Tx, factionID string, target aiRaidTarget) error {
	var originX, originY int
	var originRegion string
	err := tx.QueryRowContext(ctx, `
		SELECT c.x, c.y, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id = $1`, factionID).Scan(&originX, &originY, &originRegion)
	if err != nil {
		return fmt.Errorf("loading AI faction origin: %w", err)
	}

	var soldiers, mechs int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers,0), COALESCE(mechs,0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", factionID).Scan(&soldiers, &mechs)
	if err != nil {
		return fmt.Errorf("loading AI faction garrison: %w", err)
	}
	commitSoldiers := int(math.Floor(float64(soldiers) * aiFractionOfGarrisonCommitted))
	commitMechs := int(math.Floor(float64(mechs) * aiFractionOfGarrisonCommitted))
	if commitSoldiers <= 0 && commitMechs <= 0 {
		_, _ = tx.ExecContext(ctx, "INSERT INTO ai_faction_decisions (encampment_id, intent, target_encampment_id, reason) VALUES ($1, 'idle', $2, 'chose to raid but garrison was too small to commit any force')", factionID, target.id)
		return nil
	}

	steps := math.Abs(float64(target.x-originX)) + math.Abs(float64(target.y-originY))
	if steps == 0 {
		steps = 1
	}
	marchingMinutes := steps * 10.0
	if marchingMinutes < 1.0 {
		marchingMinutes = 1.0
	}
	marchDuration := time.Duration(marchingMinutes) * time.Minute
	resolveTime := time.Now().UTC().Add(marchDuration)
	legStartedAt := time.Now().UTC()

	var raidID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time, base_march_minutes,
			attacker_rations, attacker_ammo, attacker_electricity, attacker_logistics,
			origin_x, origin_y, destination_x, destination_y, origin_region, destination_region,
			leg_started_at, leg_total_minutes, movement_state)
		VALUES ($1, $2, 'marching', $3, $4, 100.0, 100.0, 100.0, 100.0, $5, $6, $7, $8, $9, $10, $11, $4, 'moving')
		RETURNING id`,
		factionID, target.id, resolveTime, marchingMinutes, originX, originY, target.x, target.y, originRegion, target.region, legStartedAt).
		Scan(&raidID)
	if err != nil {
		return fmt.Errorf("creating AI-launched raid: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO raid_forces (raid_id, soldiers_mobilized, mechs_mobilized, route_type)
		VALUES ($1, $2, $3, 'direct')`, raidID, commitSoldiers, commitMechs); err != nil {
		return fmt.Errorf("creating AI raid force: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE workshop_inventory SET soldiers = soldiers - $1, mechs = mechs - $2 WHERE encampment_id = $3`,
		commitSoldiers, commitMechs, factionID); err != nil {
		return fmt.Errorf("debiting AI faction garrison: %w", err)
	}

	if !target.isAIFaction {
		var factionName string
		_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", factionID).Scan(&factionName)
		_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)",
			fmt.Sprintf("⚠️ HOSTILE CONTACT: %s has deployed forces toward Outpost [%s].", factionName, target.name))
	}

	_, _ = tx.ExecContext(ctx, "INSERT INTO ai_faction_decisions (encampment_id, intent, target_encampment_id, resulting_raid_id, reason) VALUES ($1, 'raid', $2, $3, $4)",
		factionID, target.id, raidID, fmt.Sprintf("committed %d soldiers, %d mechs", commitSoldiers, commitMechs))
	return nil
}
