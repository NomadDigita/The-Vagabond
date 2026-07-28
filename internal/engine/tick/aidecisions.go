package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// aiDecisionCadence is how often a single AI faction gets to decide
// anything. See AI_FACTION_DECISION_LOOP_PLAN.md's "Step 1: cadence gate"
// for why 20 minutes was picked (roughly one of the 8 seeded factions
// deciding something every 2-3 minutes world-wide).
const aiDecisionCadence = 20 * time.Minute

// aiRaidProbabilityWhenEligible is rolled even when a faction has a valid,
// fair target - so AI aggression reads as a threat that sometimes doesn't
// come, not a metronome. See the plan doc's "Step 3: intent selection".
const aiRaidProbabilityWhenEligible = 0.40

// aiFractionOfGarrisonCommitted caps how much of a faction's home garrison
// a single raid can commit, so a faction that raids never strips its own
// defense to zero - mirrors the spirit of the human "Manual Defense
// Garrison" concept (combat_road_encounters.go's loadBaseGarrisonForce),
// without reusing that exact mechanism (AI factions don't have a player-
// set garrison reserve to read).
const aiFractionOfGarrisonCommitted = 0.65

// aiMaxLevelsBelowSelfForFairTarget: an AI faction will not raid a real
// player whose level is more than this many levels below its own. This is
// the fairness guardrail AI_FACTION_DECISION_LOOP_PLAN.md calls for -
// protecting weak players from unrestrained AI aggression is a designer
// responsibility, not something the AI "chooses". Deliberately
// conservative; loosen only after observing AI factions are too passive
// in practice, not by guessing up front.
const aiMaxLevelsBelowSelfForFairTarget = 2

// decideAIFactionActions is the second half of Phase 6 (persistent AI
// civilizations) - see AI_FACTION_DECISION_LOOP_PLAN.md for the full
// design this implements. growAICivilizations (Phase 6 foundational tier)
// already makes AI factions grow passively; this makes them periodically
// decide to scout an undiscovered target or launch a real raid, using
// nothing but what they've legitimately discovered through
// encampment_discoveries - the same no-omniscience rule a human plays by.
func (e *Engine) decideAIFactionActions(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT e.id, e.level, c.region
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.is_ai_faction = TRUE
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
		// Mark the decision as made immediately - this is both the
		// cadence gate for next time and (combined with the FOR UPDATE
		// above) the concurrency guard: a second tick worker reading
		// this row after this UPDATE commits will see a fresh
		// ai_last_decision_at and skip it.
		if _, err := tx.ExecContext(ctx, "UPDATE encampments SET ai_last_decision_at = CURRENT_TIMESTAMP WHERE id = $1", f.id); err != nil {
			return fmt.Errorf("updating AI decision cadence for %s: %w", f.id, err)
		}

		if err := e.decideOneAIFaction(ctx, tx, f.id, f.level, f.region); err != nil {
			return fmt.Errorf("deciding action for AI faction %s: %w", f.id, err)
		}
	}
	return nil
}

// aiRaidTarget is a real player's encampment an AI faction has already
// discovered and could fairly raid.
type aiRaidTarget struct {
	id     string
	name   string
	level  int
	x, y   int
	region string
	userID int64
}

func (e *Engine) decideOneAIFaction(ctx context.Context, tx *sql.Tx, factionID string, factionLevel int, region string) error {
	target, err := e.pickFairAIRaidTarget(ctx, tx, factionID, factionLevel)
	if err != nil {
		return fmt.Errorf("selecting raid target: %w", err)
	}

	if target != nil && rand.Float64() < aiRaidProbabilityWhenEligible {
		return e.launchAIRaid(ctx, tx, factionID, *target)
	}

	undiscoveredCount, err := e.countUndiscoveredRealPlayers(ctx, tx, factionID, region)
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

// countUndiscoveredRealPlayers mirrors the WHERE clause
// resolveExplorationDiscovery uses to find a human's next undiscovered
// target, adapted for an AI faction as the observer and explicitly
// excluding other AI factions - AI-vs-AI raids are out of scope, see
// AI_FACTION_DECISION_LOOP_PLAN.md's "Deliberately out of scope".
func (e *Engine) countUndiscoveredRealPlayers(ctx context.Context, tx *sql.Tx, observerCampID, region string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id <> $1
		  AND e.is_ai_faction = FALSE
		  AND c.region = $2
		  AND NOT EXISTS (
				SELECT 1 FROM encampment_discoveries d
				WHERE d.observer_encampment_id = $1 AND d.target_encampment_id = e.id
			)`, observerCampID, region).Scan(&count)
	return count, err
}

// aiScout discovers exactly one undiscovered real player in the faction's
// home continent - the AI-faction-as-observer counterpart to
// resolveExplorationDiscovery. No notification is sent to the discovered
// player: a human's own exploration discovering a rival doesn't notify
// the rival either (discovery is silent both ways today), so this doesn't
// introduce a new asymmetry.
func (e *Engine) aiScout(ctx context.Context, tx *sql.Tx, factionID, region string) error {
	var targetID, targetName string
	err := tx.QueryRowContext(ctx, `
		SELECT e.id, e.name
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id <> $1
		  AND e.is_ai_faction = FALSE
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

// pickFairAIRaidTarget returns an already-discovered real player this
// faction could raid without exceeding aiMaxLevelsBelowSelfForFairTarget,
// or nil if none qualifies. Never considers another AI faction a target.
func (e *Engine) pickFairAIRaidTarget(ctx context.Context, tx *sql.Tx, factionID string, factionLevel int) (*aiRaidTarget, error) {
	minLevel := factionLevel - aiMaxLevelsBelowSelfForFairTarget
	if minLevel < 1 {
		minLevel = 1
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT e.id, e.name, e.level, c.x, c.y, c.region, e.user_id
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
		ORDER BY random()
		LIMIT 5`, factionID, minLevel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t aiRaidTarget
		if err := rows.Scan(&t.id, &t.name, &t.level, &t.x, &t.y, &t.region, &t.userID); err == nil {
			return &t, nil
		}
	}
	return nil, nil
}

// launchAIRaid creates a real raids row with factionID as attacker_id,
// shaped exactly like a human-launched raid (see
// AI_FACTION_DECISION_LOOP_PLAN.md's "Step 6" for why this precision
// matters - the entire downstream combat/road/loot/notification pipeline
// only works correctly if every field a human raid sets is set here too).
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

	var factionName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", factionID).Scan(&factionName)
	_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)",
		fmt.Sprintf("⚠️ HOSTILE CONTACT: %s has deployed forces toward Outpost [%s].", factionName, target.name))

	_, _ = tx.ExecContext(ctx, "INSERT INTO ai_faction_decisions (encampment_id, intent, target_encampment_id, resulting_raid_id, reason) VALUES ($1, 'raid', $2, $3, $4)",
		factionID, target.id, raidID, fmt.Sprintf("committed %d soldiers, %d mechs", commitSoldiers, commitMechs))
	return nil
}
