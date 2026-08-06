package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/roadcombat"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"github.com/NomadDigita/The-Vagabond/internal/game/worldintel"
)

// scoutMissionPingInterval gates BOTH the actual discovery-roll cadence
// during the searching phase AND the periodic "still searching"/"en route
// home" status pings, off a single scout_missions.last_status_notified_at
// column - mirroring the AI decision loop's cadence-gate pattern
// (ai_last_decision_at), per AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md
// section 3.5's own note.
//
// This is a deliberate interpretation of an underspecified detail in the
// plan doc, flagged explicitly rather than silently decided: the doc's
// literal wording ("each tick... roll ExplorationDiscoveryChance") would,
// taken at face value against a 60-second default tick interval, resolve
// almost every search within 1-2 minutes - that trivializes "search until
// found, no ETA" into "search for basically no time." Gating the roll on
// the same ~30-minute cadence as the status ping keeps the probabilistic
// model (worldintel.ExplorationDiscoveryChance) meaningful while still
// giving the player periodic feedback that something is happening.
const scoutMissionPingInterval = 30 * time.Minute

// scoutBonusDiscoveryChance is a flat, modest chance per gate-open during
// the return leg that the party stumbles onto a second target passing by
// - mirroring discoverRouteContacts' "reveal a base while marching past
// it" mechanic for raids, but rolled rather than guaranteed-on-proximity
// since a scout mission's return path (unlike a raid's fixed origin/
// destination line) has no interpolatable route until it actually finds
// its primary target.
const scoutBonusDiscoveryChance = 0.15

// scoutResourceRatePerHourPerScout calibrates the resource trickle from
// section 3.6. A normal /newjobexplore dispatch averages roughly a
// 32-minute round trip for a flat ~150-400 (avg ~275) single-resource
// payout, uncommitted to any manpower. Scouting is a much longer,
// open-ended, manpower-committing mission, so it's priced per scout per
// hour instead of flat-per-dispatch: at 1 scout for a short ~2-hour
// mission that's ~100 total value (less than one normal dispatch, since
// nothing was guaranteed and manpower sat idle from other tasks the
// whole time); at 5 scouts on a long ~4-hour mission that's ~1000 (a
// genuinely meaningful haul for a long, risky-feeling commitment). Split
// 60/40 scrap/metal, matching the two most common exploration reward
// tiers' relative weighting. Tunable starting constant, not a
// precisely-right number - same spirit as this session's other new
// tunables (ghostProtocolCostFraction, aiFleeGarrisonThreshold, etc.).
const scoutResourceRatePerHourPerScout = 50.0

// processScoutMissions is the tick pass entry point, registered in
// engine.go's ProcessTick phase list.
func (e *Engine) processScoutMissions(ctx context.Context, tx *sql.Tx) error {
	if err := e.processSearchingScoutMissions(ctx, tx); err != nil {
		return fmt.Errorf("processing searching scout missions: %w", err)
	}
	if err := e.completeReturnedScoutMissions(ctx, tx); err != nil {
		return fmt.Errorf("completing returned scout missions: %w", err)
	}
	if err := e.pingInTransitScoutMissions(ctx, tx); err != nil {
		return fmt.Errorf("pinging in-transit scout missions: %w", err)
	}
	return nil
}

type searchingScoutMission struct {
	id              string
	encampmentID    string
	scoutsCommitted int
	lastNotified    sql.NullTime
	userID          int64
	originX         int
	originY         int
}

// processSearchingScoutMissions gates each mission's discovery roll on
// scoutMissionPingInterval, then either transitions it to 'returning' (a
// hit) or sends a "still searching" ping (a miss).
func (e *Engine) processSearchingScoutMissions(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT sm.id, sm.encampment_id, sm.scouts_committed, sm.last_status_notified_at,
		       e.user_id, c.x, c.y
		FROM scout_missions sm
		JOIN encampments e ON e.id = sm.encampment_id
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE sm.phase = 'searching'`)
	if err != nil {
		return err
	}
	var missions []searchingScoutMission
	for rows.Next() {
		var m searchingScoutMission
		if scanErr := rows.Scan(&m.id, &m.encampmentID, &m.scoutsCommitted, &m.lastNotified, &m.userID, &m.originX, &m.originY); scanErr == nil {
			missions = append(missions, m)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range missions {
		if m.lastNotified.Valid && time.Since(m.lastNotified.Time) < scoutMissionPingInterval {
			continue
		}
		if rand.Float64() < worldintel.ExplorationDiscoveryChance(m.scoutsCommitted) {
			if err := e.scoutMissionFindsTarget(ctx, tx, m); err != nil {
				log.Printf("Failed resolving scout mission %s discovery: %v", m.id, err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE scout_missions SET last_status_notified_at = CURRENT_TIMESTAMP WHERE id = $1", m.id); err != nil {
			log.Printf("Failed updating scout mission %s ping timestamp: %v", m.id, err)
			continue
		}
		if isRealPlayer(m.userID) {
			if err := notifications.Queue(ctx, tx, m.userID, "🔭 "+htmlBoldTick("SCOUTING UPDATE")+": your Scout Walkers continue searching the wasteland - no contact yet.", "scout_status"); err != nil {
				log.Printf("Failed queuing scout mission %s status ping: %v", m.id, err)
			}
		}
	}
	return nil
}

// scoutMissionFindsTarget picks a target using the same no-omniscience
// selection query aiScout/resolveExplorationDiscovery already use
// (undiscovered-by-this-observer, ordered by danger/established_at) -
// but deliberately without a region filter, since long-range scouting is
// meant to search the whole planet, not just the home continent (unlike
// aiScout, which is intentionally home-continent-scoped for the AI
// decision loop). A real player or an AI faction are both valid finds,
// consistent with this session's earlier AI-vs-AI parity work.
func (e *Engine) scoutMissionFindsTarget(ctx context.Context, tx *sql.Tx, m searchingScoutMission) error {
	var targetID, targetName string
	var targetUserID int64
	var targetX, targetY int
	var targetRegion string
	err := tx.QueryRowContext(ctx, `
		SELECT e.id, e.name, e.user_id, c.x, c.y, c.region
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id <> $1
		  AND NOT EXISTS (
				SELECT 1 FROM encampment_discoveries d
				WHERE d.observer_encampment_id = $1 AND d.target_encampment_id = e.id
			)
		ORDER BY c.danger_level ASC, e.established_at ASC
		LIMIT 1`, m.encampmentID).Scan(&targetID, &targetName, &targetUserID, &targetX, &targetY, &targetRegion)
	if err == sql.ErrNoRows {
		// Nothing left anywhere in the world for this observer to find -
		// not an error, just keep searching.
		if _, execErr := tx.ExecContext(ctx, "UPDATE scout_missions SET last_status_notified_at = CURRENT_TIMESTAMP WHERE id = $1", m.id); execErr != nil {
			return execErr
		}
		if isRealPlayer(m.userID) {
			return notifications.Queue(ctx, tx, m.userID, "🔭 "+htmlBoldTick("SCOUTING UPDATE")+": your Scout Walkers continue searching the wasteland - no contact yet.", "scout_status")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("selecting scout mission target: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method)
		VALUES ($1, $2, 'scout_mission') ON CONFLICT DO NOTHING`, m.encampmentID, targetID); err != nil {
		return fmt.Errorf("recording scout mission discovery: %w", err)
	}
	// known_locations is the coordinate SNAPSHOT (see schema.go's comment
	// on the table) - locked now, at discovery time, distinct from the
	// permanent boolean relationship just recorded above.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO known_locations (observer_encampment_id, target_encampment_id, x, y, region)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (observer_encampment_id, target_encampment_id)
		DO UPDATE SET x = EXCLUDED.x, y = EXCLUDED.y, region = EXCLUDED.region, locked_at = CURRENT_TIMESTAMP`,
		m.encampmentID, targetID, targetX, targetY, targetRegion); err != nil {
		return fmt.Errorf("locking known location: %w", err)
	}

	// Same Manhattan-distance march formula launchAIRaid already uses
	// (steps * 10 minutes, minimum 1 minute) - one consistent notion of
	// travel time across the codebase rather than a scouting-specific one.
	steps := math.Abs(float64(targetX-m.originX)) + math.Abs(float64(targetY-m.originY))
	marchingMinutes := steps * 10.0
	if marchingMinutes < 1.0 {
		marchingMinutes = 1.0
	}
	now := time.Now().UTC()
	returnETA := now.Add(time.Duration(marchingMinutes * float64(time.Minute)))

	if _, err := tx.ExecContext(ctx, `
		UPDATE scout_missions SET
			phase = 'returning',
			found_target_encampment_id = $1, found_at = $2, found_x = $3, found_y = $4, found_region = $5,
			return_eta = $6, return_leg_started_at = $2, return_leg_total_minutes = $7,
			origin_x = $8, origin_y = $9, last_status_notified_at = $2
		WHERE id = $10`,
		targetID, now, targetX, targetY, targetRegion, returnETA, marchingMinutes, m.originX, m.originY, m.id); err != nil {
		return fmt.Errorf("transitioning scout mission to returning: %w", err)
	}

	if isRealPlayer(m.userID) {
		// "general" (non-mutable), not "scout_status": the periodic
		// "still searching" ping above is routine chatter and fine to
		// mute, but this is the actual discovery event - the entire
		// payoff of the mission - and MutableCategories' own contract
		// (notifications/preferences.go) is explicit that discovery
		// alerts must never be suppressible. This was previously tagged
		// "route_status" by mistake (before scouting got its own
		// mutable category), meaning a player who'd muted the routine
		// pings would silently lose this one too.
		if err := notifications.Queue(ctx, tx, m.userID, fmt.Sprintf(
			"🎯 %s\n\nYour Scout Walkers located Outpost %s. Its position is now locked in your intel and marked in your Tactical Target Matrix.\n\n🚶 Beginning the journey home - ETA %s.",
			htmlBoldTick("CONTACT!"), htmlBoldTick(htmlEscapeTick(targetName)), htmlCodeTick(formatDurationTick(time.Duration(marchingMinutes*float64(time.Minute))))), "general"); err != nil {
			return err
		}
	}
	// Being found is genuine information for the target too - matches
	// discoverRouteContacts' existing "being seen is reciprocal
	// knowledge" precedent for raids, though a scout sighting doesn't
	// grant the target reciprocal discovery of the *scout's* base (a
	// distant, passive survey team isn't the same as a raid crossing
	// your road) - just a heads-up that they're now on someone's radar.
	if isRealPlayer(targetUserID) {
		if err := notifications.Queue(ctx, tx, targetUserID,
			"📡 "+htmlBoldTick("SPOTTED")+": A distant scouting party has located and locked in your outpost's position. You may want to check your defenses.", "general"); err != nil {
			return err
		}
	}
	return nil
}

type returningScoutMission struct {
	id              string
	encampmentID    string
	scoutsCommitted int
	startedAt       time.Time
	userID          int64
	bonusName       sql.NullString
}

// completeReturnedScoutMissions credits the resource trickle (section
// 3.6), returns the committed scouts to workshop_inventory, and marks
// the mission complete once return_eta has passed.
func (e *Engine) completeReturnedScoutMissions(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT sm.id, sm.encampment_id, sm.scouts_committed, sm.started_at, e.user_id, be.name
		FROM scout_missions sm
		JOIN encampments e ON e.id = sm.encampment_id
		LEFT JOIN encampments be ON be.id = sm.bonus_discovery_encampment_id
		WHERE sm.phase = 'returning' AND sm.return_eta <= CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	var missions []returningScoutMission
	for rows.Next() {
		var m returningScoutMission
		if scanErr := rows.Scan(&m.id, &m.encampmentID, &m.scoutsCommitted, &m.startedAt, &m.userID, &m.bonusName); scanErr == nil {
			missions = append(missions, m)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range missions {
		totalHours := time.Since(m.startedAt).Hours()
		if totalHours <= 0 {
			totalHours = 1.0 / 60.0
		}
		totalValue := scoutResourceRatePerHourPerScout * totalHours * float64(m.scoutsCommitted)
		scrapGain := totalValue * 0.6
		metalGain := totalValue * 0.4

		var scrap, metal float64
		if err := tx.QueryRowContext(ctx, "SELECT scrap, metal FROM resources WHERE encampment_id = $1 FOR UPDATE", m.encampmentID).Scan(&scrap, &metal); err != nil {
			log.Printf("Failed reading resources for returning scout mission %s: %v", m.id, err)
			continue
		}
		storageCap := storagecap.CapFor(ctx, tx, m.encampmentID)
		newScrap, discardedScrap := storagecap.Clamp(scrap, scrapGain, storageCap)
		newMetal, discardedMetal := storagecap.Clamp(metal, metalGain, storageCap)
		if _, err := tx.ExecContext(ctx, "UPDATE resources SET scrap = $1, metal = $2 WHERE encampment_id = $3", newScrap, newMetal, m.encampmentID); err != nil {
			log.Printf("Failed crediting scout mission %s resources: %v", m.id, err)
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE workshop_inventory SET scouts = scouts + $1 WHERE encampment_id = $2", m.scoutsCommitted, m.encampmentID); err != nil {
			log.Printf("Failed returning scouts from mission %s: %v", m.id, err)
			continue
		}

		summary := fmt.Sprintf("%s %.0f Scrap, %s %.0f Metal",
			resourceEmojiTick("scrap"), scrapGain-discardedScrap, resourceEmojiTick("metal"), metalGain-discardedMetal)
		if _, err := tx.ExecContext(ctx, "UPDATE scout_missions SET phase = 'complete', completed_at = CURRENT_TIMESTAMP, resources_returned_summary = $1 WHERE id = $2", summary, m.id); err != nil {
			log.Printf("Failed marking scout mission %s complete: %v", m.id, err)
			continue
		}

		if !isRealPlayer(m.userID) {
			continue
		}
		message := fmt.Sprintf("🔭✅ %s\n\n%s Scout Walkers are home safely, bringing back:\n%s",
			htmlBoldTick("SCOUT PARTY RETURNED"), htmlCodeTick(fmt.Sprintf("%d", m.scoutsCommitted)), summary)
		if m.bonusName.Valid {
			message += fmt.Sprintf("\n\n🎯 Along the way home, they also spotted Outpost %s - added to your Tactical Target Matrix.", htmlBoldTick(htmlEscapeTick(m.bonusName.String)))
		}
		if err := notifications.Queue(ctx, tx, m.userID, message, "scout_status"); err != nil {
			log.Printf("Failed queuing scout mission %s completion notification: %v", m.id, err)
		}
	}
	return nil
}

type inTransitScoutMission struct {
	id                string
	encampmentID      string
	userID            int64
	lastNotified      sql.NullTime
	returnLegStarted  time.Time
	returnLegMinutes  float64
	returnETA         time.Time
	originX, originY  int
	foundX, foundY    int
	foundRegion       string
	bonusAlreadyFound bool
}

// pingInTransitScoutMissions handles the still-en-route case: a periodic
// "ETA" ping, plus a chance (scoutBonusDiscoveryChance, gated on the same
// cadence) of a bonus mid-journey discovery mirroring
// discoverRouteContacts' "reveal a base while marching past it" behavior
// for raids.
func (e *Engine) pingInTransitScoutMissions(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT sm.id, sm.encampment_id, e.user_id, sm.last_status_notified_at,
		       sm.return_leg_started_at, sm.return_leg_total_minutes, sm.return_eta,
		       sm.found_x, sm.found_y, sm.origin_x, sm.origin_y, sm.found_region,
		       (sm.bonus_discovery_encampment_id IS NOT NULL)
		FROM scout_missions sm
		JOIN encampments e ON e.id = sm.encampment_id
		WHERE sm.phase = 'returning' AND sm.return_eta > CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	var missions []inTransitScoutMission
	for rows.Next() {
		var m inTransitScoutMission
		if scanErr := rows.Scan(&m.id, &m.encampmentID, &m.userID, &m.lastNotified,
			&m.returnLegStarted, &m.returnLegMinutes, &m.returnETA,
			&m.foundX, &m.foundY, &m.originX, &m.originY, &m.foundRegion, &m.bonusAlreadyFound); scanErr == nil {
			missions = append(missions, m)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range missions {
		if m.lastNotified.Valid && time.Since(m.lastNotified.Time) < scoutMissionPingInterval {
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE scout_missions SET last_status_notified_at = CURRENT_TIMESTAMP WHERE id = $1", m.id); err != nil {
			log.Printf("Failed updating in-transit scout mission %s ping timestamp: %v", m.id, err)
			continue
		}

		if !m.bonusAlreadyFound && rand.Float64() < scoutBonusDiscoveryChance {
			if err := e.scoutMissionBonusDiscovery(ctx, tx, m); err != nil {
				log.Printf("Failed resolving scout mission %s bonus discovery: %v", m.id, err)
			}
		}

		if !isRealPlayer(m.userID) {
			continue
		}
		remaining := time.Until(m.returnETA)
		if remaining < 0 {
			remaining = 0
		}
		if err := notifications.Queue(ctx, tx, m.userID, fmt.Sprintf("🚶 %s: your Scout Walkers are en route home. ETA: %s.",
			htmlBoldTick("EN ROUTE"), htmlCodeTick(formatDurationTick(remaining))), "scout_status"); err != nil {
			log.Printf("Failed queuing scout mission %s en-route ping: %v", m.id, err)
		}
	}
	return nil
}

// scoutMissionBonusDiscovery interpolates the return leg's current
// position (reusing roadcombat's exact progress/position math, the same
// way discoverRouteContacts does for raids) and looks for an undiscovered
// target within one tile.
func (e *Engine) scoutMissionBonusDiscovery(ctx context.Context, tx *sql.Tx, m inTransitScoutMission) error {
	progress := roadcombat.RouteProgress(m.returnLegStarted, m.returnLegMinutes, time.Now().UTC())
	pos := roadcombat.CurrentPosition("returning", m.originX, m.originY, m.foundX, m.foundY, progress)
	currentX := int(math.Round(pos.X))
	currentY := int(math.Round(pos.Y))

	var bonusID, bonusName string
	var bonusUserID int64
	err := tx.QueryRowContext(ctx, `
		SELECT e.id, e.name, e.user_id
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id <> $1
		  AND ABS(c.x - $2) + ABS(c.y - $3) <= 1
		  AND NOT EXISTS (
				SELECT 1 FROM encampment_discoveries d
				WHERE d.observer_encampment_id = $1 AND d.target_encampment_id = e.id
			)
		ORDER BY ABS(c.x - $2) + ABS(c.y - $3), e.established_at
		LIMIT 1`, m.encampmentID, currentX, currentY).Scan(&bonusID, &bonusName, &bonusUserID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("locating scout mission bonus discovery: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method)
		VALUES ($1, $2, 'scout_mission') ON CONFLICT DO NOTHING`, m.encampmentID, bonusID)
	if err != nil {
		return fmt.Errorf("recording scout mission bonus discovery: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE scout_missions SET bonus_discovery_encampment_id = $1 WHERE id = $2", bonusID, m.id); err != nil {
		return fmt.Errorf("recording scout mission bonus discovery id: %w", err)
	}
	if isRealPlayer(bonusUserID) {
		if err := notifications.Queue(ctx, tx, bonusUserID,
			"📡 "+htmlBoldTick("SPOTTED")+": A distant scouting party passed near your outpost and spotted your position.", "general"); err != nil {
			return err
		}
	}
	return nil
}
