package handlers

import (
	"context"
	"database/sql"
	"fmt"
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
	var rations, ammo, electricity, logistics float64
	if err := tx.QueryRowContext(ctx, `
		SELECT attacker_id, COALESCE(attacker_rations,100), COALESCE(attacker_ammo,100),
		       COALESCE(attacker_electricity,100), COALESCE(attacker_logistics,100)
		FROM raids WHERE id = $1`, raidID).Scan(&attackerID, &rations, &ammo, &electricity, &logistics); err == nil {
		f.SuppliesOut = (rations <= 0 && ammo <= 0) || (electricity <= 0 && logistics <= 0)
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

// loadBaseGarrisonForce loads a passive base's standing home-defense
// reserve (workshop_inventory.garrisoned_soldiers/garrisoned_mechs - units
// the owner explicitly withheld from every draft, see hero.go's Manual
// Defense Garrison) as a roadcombat.FieldForce. This is deliberately a
// lighter force than the base's full raid-defense computation (no defense
// grid, turrets, or drafted-but-not-yet-launched units): a road ambush is a
// field skirmish against whoever the base kept at home, not a full siege.
// A commander who wants to actually raid the base's economy for loot still
// needs to launch a real raid through the normal /raid flow, with all of
// that system's existing protections (shields, pacts, etc.) intact.
func (h *CombatHandler) loadBaseGarrisonForce(ctx context.Context, tx *sql.Tx, encampmentID string) (roadcombat.FieldForce, error) {
	var f roadcombat.FieldForce
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(garrisoned_soldiers,0), COALESCE(garrisoned_mechs,0)
		FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE`, encampmentID).
		Scan(&f.Soldiers, &f.Mechs)
	if err != nil {
		return f, err
	}
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(military_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", encampmentID).Scan(&f.MilitaryTechLvl)
	if f.MilitaryTechLvl < 1 {
		f.MilitaryTechLvl = 1
	}
	return f, nil
}

// applyBaseGarrisonSurvivors writes back the post-battle survivor counts
// for a base's home-defense reserve.
func (h *CombatHandler) applyBaseGarrisonSurvivors(ctx context.Context, tx *sql.Tx, encampmentID string, s roadcombat.FieldForce) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE workshop_inventory SET garrisoned_soldiers = $1, garrisoned_mechs = $2
		WHERE encampment_id = $3`, s.Soldiers, s.Mechs, encampmentID)
	return err
}

// HandleRoadBaseEncounterCallback resolves the Attack/Continue decision for
// a road-vs-base encounter (MMO_WORLD_EVOLUTION_PLAN.md Phase 4 milestone
// 2). Only the expedition's commander gets a choice here - the base didn't
// choose to be in the road's path, so there is no symmetric "both sides
// must agree to continue" step like the expedition-vs-expedition version;
// Attack resolves immediately, and doing nothing before the deadline is a
// peaceful pass (handled by the tick engine's expireRoadBaseEncounters).
func (h *CombatHandler) HandleRoadBaseEncounterCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid road encounter action."})
	}
	action := c.Args()[0]
	encounterID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var raidID, encampmentID, status string
	err = tx.QueryRowContext(ctx, `
		SELECT raid_id, encampment_id, status
		FROM road_base_encounters WHERE id = $1 FOR UPDATE`, encounterID).Scan(&raidID, &encampmentID, &status)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ This road encounter no longer exists."})
	}
	if status != "pending" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ This road encounter has already been resolved."})
	}

	var callerCampID string
	var attackerID string
	_ = tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", raidID).Scan(&attackerID)
	if err := tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&callerCampID); err != nil || callerCampID != attackerID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only the expedition commander can issue this order."})
	}

	if action == "continue" {
		if err := h.resolveRoadBaseEncounterContinue(ctx, tx, encounterID, raidID, "voluntary"); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to resolve encounter."})
		}
		_ = tx.Commit()
		return c.Send("➡️ Your column bypasses the outpost and continues on its way.", keyboards.MainNavigation())
	}

	if action != "attack" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown road encounter order."})
	}

	myForce, err := h.loadRoadFieldForce(ctx, tx, raidID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not load your force composition."})
	}
	garrison, err := h.loadBaseGarrisonForce(ctx, tx, encampmentID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not load the outpost's home garrison."})
	}

	myPower := roadcombat.Power(myForce, 1.0)
	garrisonPower := roadcombat.Power(garrison, 1.0)
	result := roadcombat.ResolveBattle(myPower, garrisonPower)

	myLost := roadcombat.CasualtiesFor(myForce, result.ACasualtyFraction)
	garrisonLost := roadcombat.CasualtiesFor(garrison, result.BCasualtyFraction)
	mySurvivors := roadcombat.Survivors(myForce, myLost)
	garrisonSurvivors := roadcombat.Survivors(garrison, garrisonLost)

	if err := h.applyRoadSurvivors(ctx, tx, raidID, mySurvivors); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to apply casualties."})
	}
	if err := h.applyBaseGarrisonSurvivors(ctx, tx, encampmentID, garrisonSurvivors); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to apply casualties."})
	}

	outcome := "battle"
	attackerWon := result.AWon
	if result.Draw {
		outcome = "draw"
	}

	_, _ = tx.ExecContext(ctx, `
		UPDATE road_base_encounters SET status = 'resolved', outcome = $1, resolved_at = CURRENT_TIMESTAMP
		WHERE id = $2`, outcome, encounterID)

	// The expedition continues its journey from exactly where it paused,
	// unless it has nothing left to march with. No resource looting here
	// (see loadBaseGarrisonForce) - a skirmish costs lives, not the base's
	// economy, which stays behind the normal /raid protections.
	if mySurvivors.TotalUnits() <= 0 {
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed', movement_state = 'moving', active_base_encounter_id = NULL WHERE id = $1", raidID)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sender.ID,
			"💀 COLUMN DESTROYED: Your expedition was wiped out attacking the outpost. No survivors returned.")
	} else {
		_, _ = tx.ExecContext(ctx, `
			UPDATE raids SET movement_state = 'moving', active_base_encounter_id = NULL,
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    paused_at = NULL
			WHERE id = $1`, raidID)
	}

	var attackerHeadline string
	switch {
	case result.Draw:
		attackerHeadline = "⚔️ ROAD SKIRMISH - INCONCLUSIVE: Your column and the outpost's garrison traded fire and disengaged."
	case attackerWon:
		attackerHeadline = "🏆 ROAD SKIRMISH WON: Your column overpowered the outpost's home garrison."
	default:
		attackerHeadline = "💥 ROAD SKIRMISH LOST: The outpost's garrison repelled your column."
	}
	attackerReport := fmt.Sprintf(
		"%s\n\nYour Power Rating: %.0f | Garrison Power Rating: %.0f\nYour Casualties: %d Soldiers, %d Mechs\nGarrison Casualties: %d Soldiers, %d Mechs\n",
		attackerHeadline, myPower, garrisonPower, myLost.Soldiers, myLost.Mechs, garrisonLost.Soldiers, garrisonLost.Mechs,
	)
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sender.ID, attackerReport)

	var defenderUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", encampmentID).Scan(&defenderUserID)
	if defenderUserID != 0 {
		var attackerName string
		_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerID).Scan(&attackerName)
		var defenderHeadline string
		switch {
		case result.Draw:
			defenderHeadline = "⚔️ ROAD SKIRMISH - INCONCLUSIVE: Your garrison traded fire with a passing column and both disengaged."
		case !attackerWon:
			defenderHeadline = fmt.Sprintf("🏆 GARRISON HELD: Your home defense repelled an attack from [%s].", attackerName)
		default:
			defenderHeadline = fmt.Sprintf("💥 GARRISON OVERRUN: A passing column from [%s] defeated your home garrison in a road skirmish. Your base itself was not raided.", attackerName)
		}
		defenderReport := fmt.Sprintf(
			"%s\n\nYour Garrison Power: %.0f | Enemy Power: %.0f\nGarrison Casualties: %d Soldiers, %d Mechs\n",
			defenderHeadline, garrisonPower, myPower, garrisonLost.Soldiers, garrisonLost.Mechs,
		)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderReport)
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to commit road skirmish resolution."})
	}

	resultText := "⚔️ Skirmish joined! Check your notifications for the full battle report."
	_ = c.Respond(&telebot.CallbackResponse{Text: resultText})
	return c.Send(resultText, keyboards.MainNavigation())
}

// resolveRoadBaseEncounterContinue is the explicit-Continue / timeout
// counterpart for a road-vs-base encounter - same shape as
// resolveRoadEncounterContinue, but only one raid to unfreeze since the
// base side was never paused (it was never moving to begin with).
func (h *CombatHandler) resolveRoadBaseEncounterContinue(ctx context.Context, tx *sql.Tx, encounterID, raidID, outcome string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE road_base_encounters SET status = 'resolved', outcome = $1, resolved_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND status = 'pending'`, outcome, encounterID)
	if err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `
		UPDATE raids
		SET movement_state = 'moving', active_base_encounter_id = NULL,
		    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
		    paused_at = NULL
		WHERE id = $1 AND active_base_encounter_id = $2`, raidID, encounterID)
	return nil
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
