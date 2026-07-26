package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/roadcombat"
	"gopkg.in/telebot.v3"
)

// loadRoadFieldForce loads the mobile combat force, supply status, and
// military tech level for one side of a road encounter, in the shape
// roadcombat.Power/CasualtiesFor expect. It intentionally ignores buggies,
// ships, jets, nukes, and every logistics/transport unit - those units
// escort and supply the column but don't fight in a field battle, mirroring
// how they're excluded from the base-raid attack rating.
func (h *CombatHandler) loadRoadFieldForce(ctx context.Context, tx *sql.Tx, raidID string) (roadcombat.FieldForce, error) {
	var f roadcombat.FieldForce
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(soldiers_mobilized,0), COALESCE(mechs_mobilized,0), COALESCE(destroyers_mobilized,0),
		       COALESCE(bombers_mobilized,0), COALESCE(battlecruisers_mobilized,0), COALESCE(deathstars_mobilized,0),
		       COALESCE(liberators_mobilized,0), COALESCE(wraiths_mobilized,0)
		FROM raid_forces WHERE raid_id = $1 FOR UPDATE`, raidID).Scan(
		&f.Soldiers, &f.Mechs, &f.Destroyers, &f.Bombers, &f.Battlecruisers, &f.Deathstars, &f.Liberators, &f.Wraiths)
	if err != nil {
		return f, err
	}

	var attackerID string
	var rations, ammo float64
	var highTechOffline bool
	if err := tx.QueryRowContext(ctx, `
		SELECT attacker_id, COALESCE(attacker_rations,100), COALESCE(attacker_ammo,100), high_tech_offline
		FROM raids WHERE id = $1`, raidID).Scan(&attackerID, &rations, &ammo, &highTechOffline); err == nil {
		f.SuppliesOut = rations <= 0 && ammo <= 0
		f.HighTechOffline = highTechOffline
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(military_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", attackerID).Scan(&f.MilitaryTechLvl)
	}
	if f.MilitaryTechLvl < 1 {
		f.MilitaryTechLvl = 1
	}
	return f, nil
}

// applyRoadSurvivors overwrites a raid_forces row's combat-unit columns
// with the post-battle survivor counts. Non-combat columns (buggies,
// transports, cargo ships) are untouched - they never took part in the
// fight and still need to carry the column (and any captured loot) home.
func (h *CombatHandler) applyRoadSurvivors(ctx context.Context, tx *sql.Tx, raidID string, s roadcombat.FieldForce) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE raid_forces SET soldiers_mobilized = $1, mechs_mobilized = $2, destroyers_mobilized = $3,
		    bombers_mobilized = $4, battlecruisers_mobilized = $5, deathstars_mobilized = $6,
		    liberators_mobilized = $7, wraiths_mobilized = $8
		WHERE raid_id = $9`,
		s.Soldiers, s.Mechs, s.Destroyers, s.Bombers, s.Battlecruisers, s.Deathstars, s.Liberators, s.Wraiths, raidID)
	return err
}

// captureCargo moves a capped share of every stolen_* resource type the
// loser is carrying over to the winner, matching the "steal resources (not
// just one resource, but many, including Crystal)" requirement even for a
// road ambush rather than only a base raid. Returns a human-readable
// summary of what changed hands (for the battle report), or "" if the
// loser had nothing worth taking.
func (h *CombatHandler) captureCargo(ctx context.Context, tx *sql.Tx, winnerRaidID, loserRaidID string) string {
	type cargoField struct {
		column string
		label  string
	}
	fields := []cargoField{
		{"stolen_scrap", "Scrap"},
		{"stolen_metal", "Metal"},
		{"stolen_crystal", "🔮 Crystal"},
		{"stolen_rations", "Rations"},
		{"stolen_electricity", "Electricity"},
		{"stolen_hydrogen", "Hydrogen"},
		{"stolen_neuro_cores", "Neuro-Cores"},
		{"stolen_dollars", "Dollars"},
	}

	summary := ""
	for _, field := range fields {
		var carried float64
		_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(%s,0) FROM raids WHERE id = $1 FOR UPDATE", field.column), loserRaidID).Scan(&carried)
		share := roadcombat.CargoShare(carried, 0)
		if share <= 0 {
			continue
		}
		_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE raids SET %s = %s - $1 WHERE id = $2", field.column, field.column), share, loserRaidID)
		_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE raids SET %s = %s + $1 WHERE id = $2", field.column, field.column), share, winnerRaidID)
		summary += fmt.Sprintf("   %s: %.1f captured\n", field.label, share)
	}
	return summary
}

// HandleRoadEncounterCallback resolves the Attack/Continue decision from
// the Expedition Radar panel for a Phase 4 road encounter
// (MMO_WORLD_EVOLUTION_PLAN.md). Attack is unilateral and resolves
// immediately - a real ambush doesn't wait for the victim's permission.
// Continue is reciprocal: it only resolves the encounter once BOTH sides
// have chosen it (or the response window times out), matching "War may
// happen, or both parties may simply continue on their way."
func (h *CombatHandler) HandleRoadEncounterCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 3 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid road encounter action."})
	}
	action := c.Args()[0]
	encounterID := c.Args()[1]
	myRaidID := c.Args()[2]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var raidAID, raidBID, status, decisionA, decisionB string
	var deadline time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT raid_a_id, raid_b_id, status, COALESCE(decision_a,''), COALESCE(decision_b,''), response_deadline
		FROM road_encounters WHERE id = $1 FOR UPDATE`, encounterID).Scan(&raidAID, &raidBID, &status, &decisionA, &decisionB, &deadline)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ This road encounter no longer exists."})
	}
	if status != "pending" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ This road encounter has already been resolved."})
	}
	if myRaidID != raidAID && myRaidID != raidBID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ That expedition is not party to this encounter."})
	}

	var callerCampID string
	var myAttackerID string
	_ = tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", myRaidID).Scan(&myAttackerID)
	if err := tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&callerCampID); err != nil || callerCampID != myAttackerID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only the expedition commander can issue this order."})
	}

	otherRaidID := raidBID
	if myRaidID == raidBID {
		otherRaidID = raidAID
	}

	if action == "continue" {
		if myRaidID == raidAID {
			_, _ = tx.ExecContext(ctx, "UPDATE road_encounters SET decision_a = 'continue' WHERE id = $1", encounterID)
			decisionA = "continue"
		} else {
			_, _ = tx.ExecContext(ctx, "UPDATE road_encounters SET decision_b = 'continue' WHERE id = $1", encounterID)
			decisionB = "continue"
		}

		if decisionA == "continue" && decisionB == "continue" {
			if err := h.resolveRoadEncounterContinue(ctx, tx, encounterID, raidAID, raidBID, "mutual"); err != nil {
				return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to resolve encounter."})
			}
			_ = tx.Commit()
			return c.Send("➡️ Both columns agreed to continue on their way. Your expedition resumes its journey.", keyboards.MainNavigation())
		}

		_ = tx.Commit()
		return c.Respond(&telebot.CallbackResponse{Text: "➡️ Order given: continue if they also stand down. Your column holds position until they answer or the window lapses."})
	}

	if action != "attack" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown road encounter order."})
	}

	// Attack is unilateral - resolve the field battle right now.
	myForce, err := h.loadRoadFieldForce(ctx, tx, myRaidID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not load your force composition."})
	}
	otherForce, err := h.loadRoadFieldForce(ctx, tx, otherRaidID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not load the opposing force composition."})
	}

	myPower := roadcombat.Power(myForce, 1.0)
	otherPower := roadcombat.Power(otherForce, 1.0)
	result := roadcombat.ResolveBattle(myPower, otherPower)

	myLost := roadcombat.CasualtiesFor(myForce, result.ACasualtyFraction)
	otherLost := roadcombat.CasualtiesFor(otherForce, result.BCasualtyFraction)
	mySurvivors := roadcombat.Survivors(myForce, myLost)
	otherSurvivors := roadcombat.Survivors(otherForce, otherLost)

	if err := h.applyRoadSurvivors(ctx, tx, myRaidID, mySurvivors); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to apply casualties."})
	}
	if err := h.applyRoadSurvivors(ctx, tx, otherRaidID, otherSurvivors); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to apply casualties."})
	}

	var winnerRaidID, loserRaidID string
	outcome := "battle"
	cargoSummary := ""
	if result.Draw {
		outcome = "draw"
	} else if result.AWon == (myRaidID == raidAID) {
		// "AWon" is expressed in terms of raid_a/raid_b; translate to
		// winner/loser in terms of the two actual raid IDs.
		winnerRaidID, loserRaidID = myRaidID, otherRaidID
	} else {
		winnerRaidID, loserRaidID = otherRaidID, myRaidID
	}
	if winnerRaidID != "" {
		cargoSummary = h.captureCargo(ctx, tx, winnerRaidID, loserRaidID)
	}

	_, _ = tx.ExecContext(ctx, `
		UPDATE road_encounters SET status = 'resolved', outcome = $1, winner_raid_id = NULLIF($2,''), resolved_at = CURRENT_TIMESTAMP,
		    decision_a = CASE WHEN raid_a_id = $3 THEN 'attack' ELSE decision_a END,
		    decision_b = CASE WHEN raid_b_id = $3 THEN 'attack' ELSE decision_b END
		WHERE id = $4`, outcome, winnerRaidID, myRaidID, encounterID)

	// Both survivors continue their journey from the exact point they
	// paused - a road battle costs lives and cargo, not the whole trip.
	// A side reduced to zero combat units has nothing left to march with
	// and is pulled from the road entirely.
	for _, side := range []struct {
		raidID    string
		survivors roadcombat.FieldForce
	}{
		{myRaidID, mySurvivors},
		{otherRaidID, otherSurvivors},
	} {
		if side.survivors.TotalUnits() <= 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed', movement_state = 'moving', active_encounter_id = NULL WHERE id = $1", side.raidID)
			var wipedUserID int64
			_ = tx.QueryRowContext(ctx, "SELECT ea.user_id FROM raids r JOIN encampments ea ON ea.id = r.attacker_id WHERE r.id = $1", side.raidID).Scan(&wipedUserID)
			if wipedUserID != 0 {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", wipedUserID,
					"💀 COLUMN DESTROYED: Your expedition was wiped out in a road battle. No survivors returned.")
			}
			continue
		}
		_, _ = tx.ExecContext(ctx, `
			UPDATE raids SET movement_state = 'moving', active_encounter_id = NULL,
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    resolve_time = resolve_time + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    paused_at = NULL
			WHERE id = $1`, side.raidID)
	}

	// Battle report notifications for both commanders.
	for _, side := range []struct {
		raidID    string
		userIDCol string
		won       bool
		lost      roadcombat.FieldForce
		power     float64
		enemyPow  float64
	}{
		{myRaidID, "", winnerRaidID == myRaidID, myLost, myPower, otherPower},
		{otherRaidID, "", winnerRaidID == otherRaidID, otherLost, otherPower, myPower},
	} {
		var userID int64
		_ = tx.QueryRowContext(ctx, "SELECT ea.user_id FROM raids r JOIN encampments ea ON ea.id = r.attacker_id WHERE r.id = $1", side.raidID).Scan(&userID)
		if userID == 0 {
			continue
		}
		var headline string
		switch {
		case result.Draw:
			headline = "⚔️ ROAD BATTLE - INCONCLUSIVE: Both columns traded fire and disengaged."
		case side.won:
			headline = "🏆 ROAD BATTLE WON: Your column overpowered the enemy expedition."
		default:
			headline = "💥 ROAD BATTLE LOST: Your column was overpowered on the road."
		}
		report := fmt.Sprintf(
			"%s\n\nYour Power Rating: %.0f | Enemy Power Rating: %.0f\nCasualties: %d Soldiers, %d Mechs, %d Capital Units\n",
			headline, side.power, side.enemyPow, side.lost.Soldiers, side.lost.Mechs,
			side.lost.Destroyers+side.lost.Bombers+side.lost.Battlecruisers+side.lost.Deathstars+side.lost.Liberators+side.lost.Wraiths,
		)
		if side.won && cargoSummary != "" {
			report += fmt.Sprintf("\n🎒 CARGO CAPTURED:\n%s", cargoSummary)
		}
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", userID, report)
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to commit road battle resolution."})
	}

	resultText := "⚔️ Road battle joined! Check your notifications for the full battle report."
	_ = c.Respond(&telebot.CallbackResponse{Text: resultText})
	return c.Send(resultText, keyboards.MainNavigation())
}

// resolveRoadEncounterContinue is the mutual-Continue counterpart to the
// tick engine's timeout resolution (internal/engine/tick/engine.go
// resolveEncounterAsContinue) - duplicated at this small size rather than
// exported across packages, since the two call sites hold different
// transaction contexts (bot handler vs. tick engine) and the logic is a
// handful of UPDATEs, not enough to justify a shared dependency.
func (h *CombatHandler) resolveRoadEncounterContinue(ctx context.Context, tx *sql.Tx, encounterID, raidAID, raidBID, outcome string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE road_encounters SET status = 'resolved', outcome = $1, resolved_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND status = 'pending'`, outcome, encounterID)
	if err != nil {
		return err
	}
	for _, raidID := range []string{raidAID, raidBID} {
		_, _ = tx.ExecContext(ctx, `
			UPDATE raids
			SET movement_state = 'moving', active_encounter_id = NULL,
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    resolve_time = resolve_time + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    paused_at = NULL
			WHERE id = $1 AND active_encounter_id = $2`, raidID, encounterID)

		var userID int64
		var attackerID string
		if err := tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", raidID).Scan(&attackerID); err == nil {
			_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", attackerID).Scan(&userID)
			if userID != 0 {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", userID,
					"🛣️ ROAD CONTACT RESOLVED: Both columns continued on their way without engaging.")
			}
		}
	}
	return nil
}

// convoySupplyPackage is the fixed resupply amount a standard convoy
// carries per field-supply gauge (rations/ammo/electricity/logistics are
// all 0-100 scales, see attacker_* columns on raids). A real logistics
// operation restocks a meaningful chunk, not a full instant refill - the
// stranded column still has to make the most of it and may need a second
// convoy on a very long remaining leg.
const convoySupplyPackage = 50.0

// convoyScrapCostBase / convoyScrapCostPerUnit price a convoy by distance:
// a nearby resupply is cheap, reinforcing a column halfway across the
// world costs real materiel, matching "using [logistics] mechanics should
// be very expensive" in spirit (this is the Phase 5 equivalent of Phase
// 4's expensive march speed-up).
const convoyScrapCostBase = 300.0
const convoyScrapCostPerDistanceUnit = 4.0
const convoyMetalCost = 150.0

// HandleDispatchConvoy implements the "reinforcement resources... transported
// by dedicated resource units" requirement: a commander with a column
// halted at movement_state = 'awaiting_reinforcement' can send a convoy
// from home carrying rations/ammo/electricity/logistics, gated on actually
// having a hauler and a tanker to commit (matching the existing raid-launch
// rule that transports must be real, staged units) and a scrap/metal cost
// that scales with the real distance to the stranded column's current,
// frozen position.
func (h *CombatHandler) HandleDispatchConvoy(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid convoy order."})
	}
	targetRaidID := c.Args()[0]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var attackerID, movementState, state string
	var originX, originY, destX, destY int
	var legStartedAt time.Time
	var legTotalMinutes float64
	err = tx.QueryRowContext(ctx, `
		SELECT attacker_id, COALESCE(movement_state,'moving'), state,
		       COALESCE(origin_x,0), COALESCE(origin_y,0), COALESCE(destination_x,0), COALESCE(destination_y,0),
		       COALESCE(leg_started_at, created_at, CURRENT_TIMESTAMP), COALESCE(leg_total_minutes, base_march_minutes, 15.0)
		FROM raids WHERE id = $1 FOR UPDATE`, targetRaidID).Scan(
		&attackerID, &movementState, &state, &originX, &originY, &destX, &destY, &legStartedAt, &legTotalMinutes)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ That expedition no longer exists."})
	}

	var callerCampID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&callerCampID); err != nil || callerCampID != attackerID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only the expedition commander can dispatch a convoy for this column."})
	}
	if movementState != "awaiting_reinforcement" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ That column is not currently awaiting reinforcement."})
	}

	var existing int
	_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM supply_convoys WHERE target_raid_id = $1 AND state = 'marching'", targetRaidID).Scan(&existing)
	if existing > 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ A resupply convoy is already en route to this column."})
	}

	// Since the column is frozen (movement_state != 'moving'), paused_at
	// anchors its true current position - it isn't advancing while it
	// waits, so this is exactly where the convoy needs to travel to.
	var pausedAt sql.NullTime
	_ = tx.QueryRowContext(ctx, "SELECT paused_at FROM raids WHERE id = $1", targetRaidID).Scan(&pausedAt)
	effectiveNow := time.Now().UTC()
	if pausedAt.Valid {
		effectiveNow = pausedAt.Time.UTC()
	}
	progress := roadcombat.RouteProgress(legStartedAt.UTC(), legTotalMinutes, effectiveNow)
	targetPos := roadcombat.CurrentPosition(state, originX, originY, destX, destY, progress)

	var homeX, homeY int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(c.x,0), COALESCE(c.y,0) FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", attackerID).Scan(&homeX, &homeY)
	distance := roadcombat.Distance(roadcombat.Position{X: float64(homeX), Y: float64(homeY)}, targetPos)
	travelMinutes := roadcombat.ConvoyTravelMinutes(distance)
	scrapCost := convoyScrapCostBase + distance*convoyScrapCostPerDistanceUnit

	var haulers, tankers int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(haulers,0), COALESCE(tankers,0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&haulers, &tankers)
	if haulers < 1 || tankers < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Convoy Requires Transports: You need at least 1 available Hauler and 1 available Tanker at home to dispatch a resupply convoy."})
	}

	var homeScrap, homeMetal float64
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(scrap,0), COALESCE(metal,0) FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&homeScrap, &homeMetal)
	if homeScrap < scrapCost || homeMetal < convoyMetalCost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materiel: Dispatching this convoy costs %.0f Scrap and %.0f Metal.", scrapCost, convoyMetalCost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1, metal = metal - $2 WHERE encampment_id = $3", scrapCost, convoyMetalCost, attackerID)
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET haulers = haulers - 1, tankers = tankers - 1 WHERE encampment_id = $1", attackerID)

	resolveTime := time.Now().UTC().Add(time.Duration(travelMinutes) * time.Minute)
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO supply_convoys (home_encampment_id, target_raid_id, rations_carried, ammo_carried, electricity_carried, logistics_carried,
		    haulers_committed, tankers_committed, origin_x, origin_y, destination_x, destination_y, resolve_time)
		VALUES ($1, $2, $3, $3, $3, $3, 1, 1, $4, $5, $6, $7, $8)`,
		attackerID, targetRaidID, convoySupplyPackage, homeX, homeY, int(math.Round(targetPos.X)), int(math.Round(targetPos.Y)), resolveTime)

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to dispatch convoy."})
	}

	etaMinutes := int(travelMinutes)
	_ = c.Respond(&telebot.CallbackResponse{Text: "🚚 Convoy dispatched!"})
	return c.Send(fmt.Sprintf(
		"🚚 RESUPPLY CONVOY DISPATCHED\n\n1 Hauler + 1 Tanker committed, carrying %.0f%% rations/ammo/electricity/logistics.\nCost: %.0f Scrap, %.0f Metal.\nETA: ~%d minutes.\n\nYour stranded column will resume its journey automatically once the convoy arrives.",
		convoySupplyPackage, scrapCost, convoyMetalCost, etaMinutes,
	), keyboards.MainNavigation())
}

// breakCampCrystalPerSeverityHour / breakCampMinCrystalCost /
// breakCampMaxCrystalCost price Phase 5 milestone 5's "pay to clear a
// weather camp early" option: cost scales with severity and how much time
// is actually left (clearing a camp that's about to expire anyway should
// be cheap; clearing a severe camp with a day and a half left should be
// genuinely expensive), with a documented floor and ceiling so it's never
// free and never unboundedly punishing.
const breakCampCrystalPerSeverityHour = 0.75
const breakCampMinCrystalCost = 3.0
const breakCampMaxCrystalCost = 60.0

// handleBreakCampEarly implements the priced early-clear: paying Crystal
// (deliberately - Crystal is the rarest resource, so this is an
// "exceptional, expensive" bypass, matching the plan's milestone 5
// requirement) resolves the active route_incidents row immediately via the
// same leg/resolve_time-shift resume logic used everywhere else in Phase
// 3-5, rather than a separate, disconnected "instant done" shortcut.
func (h *CombatHandler) handleBreakCampEarly(ctx context.Context, tx *sql.Tx, c telebot.Context, raidID string) error {
	var incidentID string
	var severity int
	var clearedAt time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT ri.id, ri.severity, ri.cleared_at
		FROM route_incidents ri
		JOIN raids r ON r.active_incident_id = ri.id
		WHERE r.id = $1 AND ri.resolved = FALSE FOR UPDATE OF ri`, raidID).Scan(&incidentID, &severity, &clearedAt)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ No active weather incident found for this column."})
	}

	remainingHours := time.Until(clearedAt.UTC()).Hours()
	if remainingHours < 0 {
		remainingHours = 0
	}
	crystalCost := breakCampCrystalPerSeverityHour * float64(severity) * remainingHours
	if crystalCost < breakCampMinCrystalCost {
		crystalCost = breakCampMinCrystalCost
	}
	if crystalCost > breakCampMaxCrystalCost {
		crystalCost = breakCampMaxCrystalCost
	}

	var attackerID string
	var crystal float64
	_ = tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", raidID).Scan(&attackerID)
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(crystal,0) FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&crystal)
	if crystal < crystalCost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Crystal: breaking camp early here costs 🔮 %.1f, you have %.1f.", crystalCost, crystal)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - $1 WHERE encampment_id = $2", crystalCost, attackerID)
	_, _ = tx.ExecContext(ctx, "UPDATE route_incidents SET resolved = TRUE WHERE id = $1", incidentID)
	_, _ = tx.ExecContext(ctx, `
		UPDATE raids
		SET movement_state = 'moving', active_incident_id = NULL,
		    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
		    resolve_time = resolve_time + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
		    paused_at = NULL
		WHERE id = $1`, raidID)

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to break camp."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔮 Camp broken early!"})
	return c.Send(fmt.Sprintf("🔮 CAMP BROKEN EARLY\n\nSpent 🔮 %.1f Crystal to break camp ahead of schedule. Your column resumes its journey immediately.", crystalCost), keyboards.MainNavigation())
}
