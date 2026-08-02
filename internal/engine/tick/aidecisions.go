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

// aiVsAIRaidProbabilityWhenEligible is the separate probability used when
// a fair target happens to be another AI faction rather than a human.
// Raised 2026-08-01 from its original 0.12 ("rare background texture")
// per explicit project owner direction: AI-vs-AI raids should actually
// reach roughly twice-daily frequency, not stay a rare curiosity. At the
// 20-minute cadence (aiDecisionCadence) each faction gets ~72 decisions/
// day; 0.35 assumes a fair AI-vs-AI target is discovered and eligible on
// a meaningful minority of those cycles, which combined with the
// probability should land in the right neighborhood - this is a
// first-pass estimate to be corrected from real play, not a derived
// number, exactly like every other constant in this file.
const aiVsAIRaidProbabilityWhenEligible = 0.35

// aiFractionOfGarrisonCommitted caps how much of a faction's home garrison
// a single raid can commit, so a faction that raids never strips its own
// defense to zero - mirrors the spirit of the human "Manual Defense
// Garrison" concept (combat_road_encounters.go's loadBaseGarrisonForce),
// without reusing that exact mechanism (AI factions don't have a player-
// set garrison reserve to read).
const aiFractionOfGarrisonCommitted = 0.65

// aiFairnessNormalBandAbove, aiFairnessWideBandAbove, and
// aiFairnessWideBandChance replace the old "how far below itself may a
// faction raid" guardrail (aiMaxLevelsBelowSelfForFairTarget) with the
// inverted rule the project owner set 2026-08-01: "lower level can
// attack higher level but higher level can't attack lower level." A
// faction may now only raid a target at its own level or above, never
// below - this protects newer/weaker players and weaker AI factions
// from being farmed by anything stronger, while still letting a weak
// side "punch up" for a real fight. It applies identically regardless of
// whether the target is a human or another AI faction - the project
// owner was explicit that near-level-or-higher is the same rule on both
// sides (e.g. a level-9 faction reaching down to hit a level-7 one
// "should almost never" happen anymore, exactly the case this inversion
// closes off).
//
// aiFairnessNormalBandAbove is how far above its own level a faction
// will normally consider. aiFairnessWideBandAbove is an occasionally-
// used wider version so a faction that's currently the strongest thing
// it has discovered within the normal band doesn't go permanently idle
// - "sometimes widen the fairness band generally," per the project
// owner - and aiFairnessWideBandChance is how often the wide band gets
// used instead of the normal one. First-pass tuning guesses, same
// caveat as every other constant here: loosen or tighten after
// observing real play.
const (
	aiFairnessNormalBandAbove = 3
	aiFairnessWideBandAbove   = 8
	aiFairnessWideBandChance  = 0.25
)

// aiOverdueRaidThreshold and aiOverdueMinTargetLevel implement the
// second half of the project owner's 2026-08-01 direction: independent
// of the fairness band and the probability roll above, a real player at
// or above aiOverdueMinTargetLevel who hasn't been the defender in any
// raid for at least aiOverdueRaidThreshold, and who at least one AI
// faction has already legitimately discovered and could raid under the
// attack-up-only rule, gets raided on that faction's very next decision
// cycle - no roll, no exception. This is what "sometimes... a dedicated
// hasn't-been-raided-in-N-hours becomes eligible" means in practice: it
// guarantees periodic pressure on higher-level players instead of
// leaving it to chance whether the probability roll ever produces it.
// 10 hours means a continuously-eligible level-10+ player sees roughly
// 2-3 guaranteed hits/day from this mechanism alone (on top of whatever
// the normal roll adds), which is what the "2 to 4 raids daily for
// level 10+" ask was actually asking for. Deliberately restricted to
// real players, not AI-vs-AI - AI-vs-AI frequency is handled entirely by
// aiVsAIRaidProbabilityWhenEligible above. First-pass guess, same
// tuning caveat as the rest of this file.
const (
	aiOverdueRaidThreshold  = 10 * time.Hour
	aiOverdueMinTargetLevel = 4
)

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

	// A faction that leads or belongs to a clan with an active pact
	// (alliance or NAP) against another clan must not raid that clan's
	// members - the same rule HasActivePact enforces for human-launched
	// raids in combat.go. Resolved once per decision and threaded
	// through both target pickers below, rather than reimplementing the
	// clan lookup in each. NULL (not in a clan) simply excludes nothing,
	// since no clan_diplomacy row can match a NULL clan id.
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

// countUndiscoveredTargets mirrors the WHERE clause
// resolveExplorationDiscovery uses to find a human's next undiscovered
// target, adapted for an AI faction as the observer. Other AI factions now
// count as valid undiscovered targets too - AI-vs-AI conflict is in scope,
// see AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2 (which
// supersedes AI_FACTION_DECISION_LOOP_PLAN.md's "deliberately out of
// scope" note).
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

// aiScout discovers exactly one undiscovered target (a real player or,
// since this is now in scope, another AI faction) in the faction's home
// continent - the AI-faction-as-observer counterpart to
// resolveExplorationDiscovery. No notification is sent to the discovered
// party: a human's own exploration discovering a rival doesn't notify
// the rival either (discovery is silent both ways today), so this doesn't
// introduce a new asymmetry.
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

// pickFairAIRaidTarget returns an already-discovered target (a real
// player, or - now that AI-vs-AI conflict is in scope, see
// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2 - another AI
// faction) this faction could fairly raid, or nil if none qualifies.
// "Fairly" means at or above the faction's own level, up to
// aiFairnessNormalBandAbove levels above (or, on the
// aiFairnessWideBandChance roll, up to aiFairnessWideBandAbove) - the
// attack-up-only rule described where those constants are defined. The
// band applies identically regardless of who's on the other side;
// AI-vs-AI doesn't get a special exemption from it. factionClanID (may
// be NULL/invalid if the faction isn't in a clan) excludes any target
// whose clan has an active alliance or NAP with the faction's clan - the
// same rule HasActivePact enforces for human-launched raids.
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

// pickOverdueRaidTarget implements the guarantee mechanism described at
// aiOverdueRaidThreshold's definition: an already-discovered real player
// (never another AI faction - AI-vs-AI frequency is handled separately
// by aiVsAIRaidProbabilityWhenEligible) at or above aiOverdueMinTargetLevel,
// at or above this faction's own level (still no exemption from the
// attack-up-only rule - this only removes the probability roll, not the
// other guardrails), who hasn't been raided by anyone in at least
// aiOverdueRaidThreshold. A diplomatic pact still blocks the guarantee too
// (see factionClanID's doc on pickFairAIRaidTarget) - a truce shouldn't
// have a "unless the timer runs out" loophole. Returns nil if no such
// target exists, in which case the caller falls through to the normal
// probabilistic path.
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

// launchAIRaid creates a real raids row with factionID as attacker_id,
// shaped exactly like a human-launched raid (see
// AI_FACTION_DECISION_LOOP_PLAN.md's "Step 6" for why this precision
// matters - the entire downstream combat/road/loot/notification pipeline
// only works correctly if every field a human raid sets is set here too).
// aiFleeGarrisonThreshold is the "reduced below some threshold by
// repeated raiding" heuristic from section 3.4 - a faction with fewer
// than this many total soldiers+mechs left is treated as desperate
// enough to consider fleeing. Deliberately a small absolute number
// rather than a fraction of historical peak: simpler to reason about,
// and this session's own note on aiMaxLevelsBelowSelfForFairTarget
// applies here too - a tunable starting guess, not a precisely-right
// number.
//
// aiFleeCooldown and aiFleeCostFraction mirror jobs.go's
// ghostProtocolCooldown/ghostProtocolCostFraction exactly (an AI faction
// gets no special exemption from Ghost Protocol's real cost/cadence) -
// duplicated here rather than imported since bot/handlers and
// engine/tick are separate packages and those constants are
// intentionally unexported; if either value changes, update both.
const (
	aiFleeGarrisonThreshold = 10
	aiFleeCooldown          = 90 * 24 * time.Hour
	aiFleeCostFraction      = 0.50
)

// maybeAIFlee is the AI decision loop's use of Ghost Protocol (see
// jobs.go's HandleGhostProtocol for the human-facing equivalent and the
// full design rationale). Reuses the exact same cooldown and
// proportional-cost formula, so an AI faction gets no special exemption
// from Ghost Protocol's real cost - it just happens to be cheap in
// absolute terms when the faction is already down to near-nothing,
// which is exactly when this triggers.
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

	// Nobody reads a newsfeed of two bots fighting each other - suppress
	// the public world_news headline for AI-vs-AI raids, but still keep
	// the audit trail in ai_faction_decisions below (that should record
	// everything, headline or not).
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
