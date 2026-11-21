package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/roadcombat"
	"gopkg.in/telebot.v3"
)

func isRealPlayer(userID int64) bool {
	return userID > 0
}

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

func (h *CombatHandler) applyRoadSurvivors(ctx context.Context, tx *sql.Tx, raidID string, s roadcombat.FieldForce) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE raid_forces SET soldiers_mobilized = $1, mechs_mobilized = $2, destroyers_mobilized = $3,
		    bombers_mobilized = $4, battlecruisers_mobilized = $5, deathstars_mobilized = $6,
		    liberators_mobilized = $7, wraiths_mobilized = $8
		WHERE raid_id = $9`,
		s.Soldiers, s.Mechs, s.Destroyers, s.Bombers, s.Battlecruisers, s.Deathstars, s.Liberators, s.Wraiths, raidID)
	return err
}

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
		summary += fmt.Sprintf("   %s: %s captured\n", field.label, htmlCode(fmt.Sprintf("%.1f", share)))
	}
	return summary
}

func (h *CombatHandler) HandleRoadEncounterCallback(c telebot.Context) error {
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

	var callerCampID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&callerCampID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only the expedition commander can issue this order."})
	}
	var myRaidID string
	switch callerCampID {
	case "":
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only the expedition commander can issue this order."})
	default:
		var raidAAttacker, raidBAttacker string
		_ = tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", raidAID).Scan(&raidAAttacker)
		_ = tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", raidBID).Scan(&raidBAttacker)
		switch callerCampID {
		case raidAAttacker:
			myRaidID = raidAID
		case raidBAttacker:
			myRaidID = raidBID
		default:
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Only the expedition commander can issue this order."})
		}
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
			return c.Send("➡️ "+htmlBold("Both columns agreed to continue on their way.")+" Your expedition resumes its journey.", telebot.ModeHTML, keyboards.MainNavigation())
		}

		_ = tx.Commit()
		return c.Respond(&telebot.CallbackResponse{Text: "➡️ Order given: continue if they also stand down. Your column holds position until they answer or the window lapses."})
	}

	if action != "attack" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown road encounter order."})
	}

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
	} else if result.AWon {
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
			if isRealPlayer(wipedUserID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", wipedUserID,
					"💀 "+htmlBold("COLUMN DESTROYED")+": Your expedition was wiped out in a road battle. No survivors returned.")
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
		if !isRealPlayer(userID) {
			continue
		}
		var headline string
		switch {
		case result.Draw:
			headline = "⚔️ " + htmlBold("ROAD BATTLE - INCONCLUSIVE") + ": Both columns traded fire and disengaged."
		case side.won:
			headline = "🏆 " + htmlBold("ROAD BATTLE WON") + ": Your column overpowered the enemy expedition."
		default:
			headline = "💥 " + htmlBold("ROAD BATTLE LOST") + ": Your column was overpowered on the road."
		}
		report := fmt.Sprintf(
			"%s\n"+divider+"\nYour Power Rating: %s | Enemy Power Rating: %s\nCasualties: %s Soldiers, %s Mechs, %s Capital Units\n",
			headline, htmlCode(fmt.Sprintf("%.0f", side.power)), htmlCode(fmt.Sprintf("%.0f", side.enemyPow)),
			htmlCode(fmt.Sprintf("%d", side.lost.Soldiers)), htmlCode(fmt.Sprintf("%d", side.lost.Mechs)),
			htmlCode(fmt.Sprintf("%d", side.lost.Destroyers+side.lost.Bombers+side.lost.Battlecruisers+side.lost.Deathstars+side.lost.Liberators+side.lost.Wraiths)),
		)
		if side.won && cargoSummary != "" {
			report += fmt.Sprintf("\n🎒 %s\n%s", htmlBold("CARGO CAPTURED"), cargoSummary)
		}
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", userID, report)
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to commit road battle resolution."})
	}

	resultText := "⚔️ " + htmlBold("Road battle joined!") + " Check your notifications for the full battle report."
	_ = c.Respond(&telebot.CallbackResponse{Text: resultText})
	return c.Send(resultText, telebot.ModeHTML, keyboards.MainNavigation())
}

const aiGarrisonReserveFraction = 0.20

func (h *CombatHandler) loadBaseGarrisonForce(ctx context.Context, tx *sql.Tx, encampmentID string, isAIFaction bool) (roadcombat.FieldForce, error) {
	var f roadcombat.FieldForce
	if isAIFaction {
		var soldiers, mechs int
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(soldiers,0), COALESCE(mechs,0)
			FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE`, encampmentID).
			Scan(&soldiers, &mechs); err != nil {
			return f, err
		}
		f.Soldiers = int(float64(soldiers) * aiGarrisonReserveFraction)
		f.Mechs = int(float64(mechs) * aiGarrisonReserveFraction)
	} else {
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(garrisoned_soldiers,0), COALESCE(garrisoned_mechs,0)
			FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE`, encampmentID).
			Scan(&f.Soldiers, &f.Mechs); err != nil {
			return f, err
		}
	}
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(military_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", encampmentID).Scan(&f.MilitaryTechLvl)
	if f.MilitaryTechLvl < 1 {
		f.MilitaryTechLvl = 1
	}
	return f, nil
}

func (h *CombatHandler) applyBaseGarrisonCasualties(ctx context.Context, tx *sql.Tx, encampmentID string, lost roadcombat.FieldForce, isAIFaction bool) error {
	if isAIFaction {
		_, err := tx.ExecContext(ctx, `
			UPDATE workshop_inventory SET
				soldiers = GREATEST(soldiers - $1, 0),
				mechs = GREATEST(mechs - $2, 0)
			WHERE encampment_id = $3`, lost.Soldiers, lost.Mechs, encampmentID)
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE workshop_inventory SET
			garrisoned_soldiers = GREATEST(garrisoned_soldiers - $1, 0),
			garrisoned_mechs = GREATEST(garrisoned_mechs - $2, 0)
		WHERE encampment_id = $3`, lost.Soldiers, lost.Mechs, encampmentID)
	return err
}

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
		return c.Send("➡️ "+htmlBold("Your column bypasses the outpost")+" and continues on its way.", telebot.ModeHTML, keyboards.MainNavigation())
	}

	if action != "attack" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown road encounter order."})
	}

	myForce, err := h.loadRoadFieldForce(ctx, tx, raidID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not load your force composition."})
	}
	var baseIsAIFaction bool
	_ = tx.QueryRowContext(ctx, "SELECT is_ai_faction FROM encampments WHERE id = $1", encampmentID).Scan(&baseIsAIFaction)
	garrison, err := h.loadBaseGarrisonForce(ctx, tx, encampmentID, baseIsAIFaction)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not load the outpost's home garrison."})
	}

	myPower := roadcombat.Power(myForce, 1.0)
	garrisonPower := roadcombat.Power(garrison, 1.0)
	result := roadcombat.ResolveBattle(myPower, garrisonPower)

	myLost := roadcombat.CasualtiesFor(myForce, result.ACasualtyFraction)
	garrisonLost := roadcombat.CasualtiesFor(garrison, result.BCasualtyFraction)
	mySurvivors := roadcombat.Survivors(myForce, myLost)

	if err := h.applyRoadSurvivors(ctx, tx, raidID, mySurvivors); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to apply casualties."})
	}
	if err := h.applyBaseGarrisonCasualties(ctx, tx, encampmentID, garrisonLost, baseIsAIFaction); err != nil {
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

	if mySurvivors.TotalUnits() <= 0 {
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed', movement_state = 'moving', active_base_encounter_id = NULL WHERE id = $1", raidID)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sender.ID,
			"💀 "+htmlBold("COLUMN DESTROYED")+": Your expedition was wiped out attacking the outpost. No survivors returned.")
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
		attackerHeadline = "⚔️ " + htmlBold("ROAD SKIRMISH - INCONCLUSIVE") + ": Your column and the outpost's garrison traded fire and disengaged."
	case attackerWon:
		attackerHeadline = "🏆 " + htmlBold("ROAD SKIRMISH WON") + ": Your column overpowered the outpost's home garrison."
	default:
		attackerHeadline = "💥 " + htmlBold("ROAD SKIRMISH LOST") + ": The outpost's garrison repelled your column."
	}
	attackerReport := fmt.Sprintf(
		"%s\n"+divider+"\nYour Power Rating: %s | Garrison Power Rating: %s\nYour Casualties: %s Soldiers, %s Mechs\nGarrison Casualties: %s Soldiers, %s Mechs\n",
		attackerHeadline, htmlCode(fmt.Sprintf("%.0f", myPower)), htmlCode(fmt.Sprintf("%.0f", garrisonPower)),
		htmlCode(fmt.Sprintf("%d", myLost.Soldiers)), htmlCode(fmt.Sprintf("%d", myLost.Mechs)),
		htmlCode(fmt.Sprintf("%d", garrisonLost.Soldiers)), htmlCode(fmt.Sprintf("%d", garrisonLost.Mechs)),
	)
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sender.ID, attackerReport)

	var defenderUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", encampmentID).Scan(&defenderUserID)
	if isRealPlayer(defenderUserID) {
		var attackerName string
		_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerID).Scan(&attackerName)
		var defenderHeadline string
		switch {
		case result.Draw:
			defenderHeadline = "⚔️ " + htmlBold("ROAD SKIRMISH - INCONCLUSIVE") + ": Your garrison traded fire with a passing column and both disengaged."
		case !attackerWon:
			defenderHeadline = fmt.Sprintf("🏆 %s: Your home defense repelled an attack from %s.", htmlBold("GARRISON HELD"), htmlCode(htmlEscape(attackerName)))
		default:
			defenderHeadline = fmt.Sprintf("💥 %s: A passing column from %s defeated your home garrison in a road skirmish. Your base itself was not raided.", htmlBold("GARRISON OVERRUN"), htmlCode(htmlEscape(attackerName)))
		}
		defenderReport := fmt.Sprintf(
			"%s\n"+divider+"\nYour Garrison Power: %s | Enemy Power: %s\nGarrison Casualties: %s Soldiers, %s Mechs\n",
			defenderHeadline, htmlCode(fmt.Sprintf("%.0f", garrisonPower)), htmlCode(fmt.Sprintf("%.0f", myPower)),
			htmlCode(fmt.Sprintf("%d", garrisonLost.Soldiers)), htmlCode(fmt.Sprintf("%d", garrisonLost.Mechs)),
		)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderReport)
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to commit road skirmish resolution."})
	}

	resultText := "⚔️ " + htmlBold("Skirmish joined!") + " Check your notifications for the full battle report."
	_ = c.Respond(&telebot.CallbackResponse{Text: resultText})
	return c.Send(resultText, telebot.ModeHTML, keyboards.MainNavigation())
}

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
			if isRealPlayer(userID) {
				_ = notifications.Queue(ctx, tx, userID,
					"🛣️ "+htmlBold("ROAD CONTACT RESOLVED")+": Both columns continued on their way without engaging.", "route_status")
			}
		}
	}
	return nil
}

const convoySupplyPackage = 50.0

const convoyScrapCostBase = 300.0
const convoyScrapCostPerDistanceUnit = 4.0
const convoyMetalCost = 150.0

const convoyMaxPairs = 3

var convoyPctOptions = []int{25, 50, 75, 100}

func convoyCarriedPct(pct, pairs int) float64 {
	total := float64(pct * pairs)
	if total > 100.0 {
		total = 100.0
	}
	return total
}

func convoyCost(distance float64, pct, pairs int) (scrapCost, metalCost float64) {
	scrapCost = (convoyScrapCostBase + distance*convoyScrapCostPerDistanceUnit) * float64(pairs) * (float64(pct) / 50.0)
	metalCost = convoyMetalCost * float64(pairs)
	return scrapCost, metalCost
}

func (h *CombatHandler) convoyTargetDistance(ctx context.Context, targetRaidID, attackerID string) (distance float64, homeX, homeY int, targetPos roadcombat.Position, err error) {
	var state string
	var originX, originY, destX, destY int
	var legStartedAt time.Time
	var legTotalMinutes float64
	var pausedAt sql.NullTime
	err = h.DB.QueryRowContext(ctx, `
		SELECT state, COALESCE(origin_x,0), COALESCE(origin_y,0), COALESCE(destination_x,0), COALESCE(destination_y,0),
		       COALESCE(leg_started_at, created_at, CURRENT_TIMESTAMP), COALESCE(leg_total_minutes, base_march_minutes, 15.0), paused_at
		FROM raids WHERE id = $1`, targetRaidID).Scan(
		&state, &originX, &originY, &destX, &destY, &legStartedAt, &legTotalMinutes, &pausedAt)
	if err != nil {
		return 0, 0, 0, roadcombat.Position{}, err
	}

	effectiveNow := time.Now().UTC()
	if pausedAt.Valid {
		effectiveNow = pausedAt.Time.UTC()
	}
	progress := roadcombat.RouteProgress(legStartedAt.UTC(), legTotalMinutes, effectiveNow)
	targetPos = roadcombat.CurrentPosition(state, originX, originY, destX, destY, progress)

	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(c.x,0), COALESCE(c.y,0) FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", attackerID).Scan(&homeX, &homeY)
	distance = roadcombat.Distance(roadcombat.Position{X: float64(homeX), Y: float64(homeY)}, targetPos)
	return distance, homeX, homeY, targetPos, nil
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (h *CombatHandler) HandleConvoyConfigPanel(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid convoy order."})
	}
	targetRaidID := args[0]
	pct, pairs := 50, 1
	if len(args) >= 2 {
		if v, convErr := strconv.Atoi(args[1]); convErr == nil {
			pct = v
		}
	}
	if len(args) >= 3 {
		if v, convErr := strconv.Atoi(args[2]); convErr == nil {
			pairs = v
		}
	}
	if pairs < 1 {
		pairs = 1
	}
	if pairs > convoyMaxPairs {
		pairs = convoyMaxPairs
	}

	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid sender context."})
	}

	var attackerID, movementState string
	_ = h.DB.QueryRowContext(ctx, "SELECT attacker_id, COALESCE(movement_state,'moving') FROM raids WHERE id = $1", targetRaidID).Scan(&attackerID, &movementState)
	if attackerID == "" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ That expedition no longer exists."})
	}
	var callerCampID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&callerCampID); err != nil || callerCampID != attackerID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only the expedition commander can configure this convoy."})
	}
	if movementState != "awaiting_reinforcement" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ That column is not currently awaiting reinforcement."})
	}

	var haulers, tankers int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(haulers,0), COALESCE(tankers,0) FROM workshop_inventory WHERE encampment_id = $1", attackerID).Scan(&haulers, &tankers)
	available := haulers
	if tankers < available {
		available = tankers
	}
	if pairs > available && available > 0 {
		pairs = available
	}

	distance, _, _, _, distErr := h.convoyTargetDistance(ctx, targetRaidID, attackerID)
	if distErr != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Could not chart a route to that column."})
	}
	travelMinutes := roadcombat.ConvoyTravelMinutes(distance)
	carried := convoyCarriedPct(pct, pairs)
	scrapCost, metalCost := convoyCost(distance, pct, pairs)

	selector := &telebot.ReplyMarkup{}
	var pctBtns []telebot.Btn
	for _, opt := range convoyPctOptions {
		label := fmt.Sprintf("%d%%/pair", opt)
		if opt == pct {
			label = "✅ " + label
		}
		pctBtns = append(pctBtns, selector.Data(label, "convoy_cfg", targetRaidID, strconv.Itoa(opt), strconv.Itoa(pairs)))
	}
	var pairBtns []telebot.Btn
	for p := 1; p <= convoyMaxPairs; p++ {
		label := fmt.Sprintf("%d Pair%s", p, pluralS(p))
		if p == pairs {
			label = "✅ " + label
		}
		pairBtns = append(pairBtns, selector.Data(label, "convoy_cfg", targetRaidID, strconv.Itoa(pct), strconv.Itoa(p)))
	}
	confirmBtn := selector.Data("🚚 Confirm Dispatch", "dispatch_convoy", targetRaidID, strconv.Itoa(pct), strconv.Itoa(pairs))
	cancelBtn := selector.Data("❌ Cancel", "convoy_cancel", targetRaidID)
	selector.Inline(selector.Row(pctBtns...), selector.Row(pairBtns...), selector.Row(confirmBtn), selector.Row(cancelBtn))

	panelText := fmt.Sprintf(
		"🚚 %s\n"+divider+"\n"+
			"Package: %s%% rations/ammo/electricity/logistics per pair\n"+
			"Transports: %s Hauler+Tanker pair(s) (you have %d available)\n"+
			"Total delivered: %s%% of each gauge (caps at 100%%)\n\n"+
			"Cost: %s Scrap, %s Metal\n"+
			"ETA: ~%s minutes\n\n"+
			"%s\n"+divider,
		htmlBold("CONFIGURE RESUPPLY CONVOY"),
		htmlCode(strconv.Itoa(pct)), htmlCode(strconv.Itoa(pairs)), available,
		htmlCode(fmt.Sprintf("%.0f", carried)),
		htmlCode(fmt.Sprintf("%.0f", scrapCost)), htmlCode(fmt.Sprintf("%.0f", metalCost)),
		htmlCode(fmt.Sprintf("%.0f", travelMinutes)),
		htmlItalic("Pick a package size and transport count, then confirm."),
	)
	return renderOrEditHTML(c, panelText, selector)
}

func (h *CombatHandler) HandleConvoyCancel(c telebot.Context) error {
	_ = c.Respond(&telebot.CallbackResponse{Text: "❌ Convoy order cancelled."})
	if c.Callback() != nil {
		return c.Edit("❌ Convoy order cancelled. No resources spent.", keyboards.MainNavigation())
	}
	return c.Send("❌ Convoy order cancelled. No resources spent.", keyboards.MainNavigation())
}

func (h *CombatHandler) HandleDispatchConvoy(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid convoy order."})
	}
	args := c.Args()
	targetRaidID := args[0]
	pct, pairs := 50, 1
	if len(args) >= 2 {
		if v, convErr := strconv.Atoi(args[1]); convErr == nil {
			pct = v
		}
	}
	if len(args) >= 3 {
		if v, convErr := strconv.Atoi(args[2]); convErr == nil {
			pairs = v
		}
	}
	if pairs < 1 {
		pairs = 1
	}
	if pairs > convoyMaxPairs {
		pairs = convoyMaxPairs
	}

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
	scrapCost, metalCost := convoyCost(distance, pct, pairs)
	carried := convoyCarriedPct(pct, pairs)

	var haulers, tankers int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(haulers,0), COALESCE(tankers,0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&haulers, &tankers)
	if haulers < pairs || tankers < pairs {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("❌ Convoy Requires Transports: You need at least %d available Hauler(s) and %d available Tanker(s) at home to dispatch this convoy.", pairs, pairs)})
	}

	var homeScrap, homeMetal float64
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(scrap,0), COALESCE(metal,0) FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&homeScrap, &homeMetal)
	if homeScrap < scrapCost || homeMetal < metalCost {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("❌ Insufficient Materiel: This convoy costs %.0f Scrap and %.0f Metal.", scrapCost, metalCost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1, metal = metal - $2 WHERE encampment_id = $3", scrapCost, metalCost, attackerID)
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET haulers = haulers - $1, tankers = tankers - $1 WHERE encampment_id = $2", pairs, attackerID)

	resolveTime := time.Now().UTC().Add(time.Duration(travelMinutes) * time.Minute)
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO supply_convoys (home_encampment_id, target_raid_id, rations_carried, ammo_carried, electricity_carried, logistics_carried,
		    haulers_committed, tankers_committed, origin_x, origin_y, destination_x, destination_y, resolve_time)
		VALUES ($1, $2, $3, $3, $3, $3, $9, $9, $4, $5, $6, $7, $8)`,
		attackerID, targetRaidID, carried, homeX, homeY, int(math.Round(targetPos.X)), int(math.Round(targetPos.Y)), resolveTime, pairs)

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to dispatch convoy."})
	}

	etaMinutes := int(travelMinutes)
	_ = c.Respond(&telebot.CallbackResponse{Text: "🚚 Convoy dispatched!"})
	return c.Send(fmt.Sprintf(
		"🚚 %s\n"+divider+"\n%d Hauler(s) + %d Tanker(s) committed, carrying %s%% rations/ammo/electricity/logistics.\nCost: %s Scrap, %s Metal.\nETA: ~%s minutes.\n\nYour stranded column will resume its journey automatically once the convoy arrives.",
		htmlBold("RESUPPLY CONVOY DISPATCHED"), pairs, pairs,
		htmlCode(fmt.Sprintf("%.0f", carried)), htmlCode(fmt.Sprintf("%.0f", scrapCost)),
		htmlCode(fmt.Sprintf("%.0f", metalCost)), htmlCode(fmt.Sprintf("%d", etaMinutes)),
	), telebot.ModeHTML, keyboards.MainNavigation())
}

const breakCampCrystalPerSeverityHour = 0.75
const breakCampMinCrystalCost = 3.0
const breakCampMaxCrystalCost = 60.0

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
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("❌ Insufficient Crystal: breaking camp early here costs 🔮 %.1f, you have %.1f.", crystalCost, crystal)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - $1 WHERE encampment_id = $2", crystalCost, attackerID)
	_, _ = tx.ExecContext(ctx, "INSERT INTO speedup_usage_log (encampment_id, scrap_spent, dollars_spent, crystal_spent) VALUES ($1, 0, 0, $2)", attackerID, crystalCost)
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
	return c.Send(fmt.Sprintf("🔮 %s\n\nSpent 🔮 %s Crystal to break camp ahead of schedule. Your column resumes its journey immediately.",
		htmlBold("CAMP BROKEN EARLY"), htmlCode(fmt.Sprintf("%.1f", crystalCost))), telebot.ModeHTML, keyboards.MainNavigation())
}
