package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type CombatHandler struct {
	DB *sql.DB
}

func NewCombatHandler(db *sql.DB) *CombatHandler {
	return &CombatHandler{DB: db}
}

// HandleRaidBoard displays other player bases available for attack
func (h *CombatHandler) HandleRaidBoard(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	// Get player's own encampment details
	var myCampID string
	var myCampName string
	err := h.DB.QueryRowContext(ctx, "SELECT id, name FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID, &myCampName)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	// Fetch up to 5 potential targets (excluding own)
	query := `
		SELECT e.id, e.name, u.first_name,
		       COALESCE((SELECT r.scrap FROM resources r WHERE r.encampment_id = e.id), 0) as scrap
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		WHERE e.id != $1
		LIMIT 5`

	rows, err := h.DB.QueryContext(ctx, query, myCampID)
	if err != nil {
		log.Printf("Failed scanning target outposts: %v", err)
		return c.Send("⚠️ Failed to load target database matrix.", keyboards.CombatNavigation())
	}
	defer rows.Close()

	type target struct {
		id       string
		name     string
		owner    string
		lootable float64
	}

	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.name, &t.owner, &t.lootable); err == nil {
			targets = append(targets, t)
		}
	}

	dashboard := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚔️ TACTICAL TARGET MATRIX\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Select an active player outpost to launch a raiding mission.\n" +
		"Live battle simulation plays in place.\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if len(targets) == 0 {
		dashboard += "⚠️ SENSORS CLEAN: No other active outposts detected in range."
	} else {
		for i, t := range targets {
			dashboard += fmt.Sprintf("[%d] Outpost: %s\n    Commander: %s\n    Estimated Loot: %.1f Scrap\n\n", i+1, t.name, t.owner, t.lootable)
			btn := selector.Data(fmt.Sprintf("⚔️ Raid [%d] (%s)", i+1, t.name), "launch_raid", myCampID, t.id)
			buttons = append(buttons, selector.Row(btn))
		}
	}
	dashboard += "━━━━━━━━━━━━━━━━━━━━━━"

	selector.Inline(buttons...)
	return c.Send(dashboard, selector, keyboards.CombatNavigation())
}

// HandleLaunchRaidCallback registers a marching raid inside the database and plays the battle cinematic in-place
func (h *CombatHandler) HandleLaunchRaidCallback(c telebot.Context) error {
	ctx := context.Background()

	attackerCampID := c.Args()[0]
	defenderCampID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database transaction error."})
	}
	defer tx.Rollback()

	// 1. Check forces
	var attackForce int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM units WHERE encampment_id = $1", attackerCampID).Scan(&attackForce)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error querying troop configurations."})
	}

	if attackForce <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Forbidden: You need at least 1 unit to launch a raid."})
	}

	// Fetch target names
	var attackerName string
	var attackerUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", attackerCampID).Scan(&attackerName, &attackerUserID)

	var defenderName string
	var defenderUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", defenderCampID).Scan(&defenderName, &defenderUserID)

	// Commit setup transaction
	_ = tx.Commit()

	_ = c.Respond(&telebot.CallbackResponse{Text: "🚀 Raid launched! Tactical telemetry online."})

	// --- BATTLE CINEMATIC ENGINE: PLAYBACK LOOP ---
	frames := []string{
		"🛰️ SENSORS ACTIVE: Calibrating targeting matrix...\n[██░░░░░░░░] 20%",
		"🚀 MARCHING: Raiders deployed on target vector...\n[████░░░░░░] 40%",
		"⚡ GRID CONTACT: Breaching outpost defense perimeter...\n[██████░░░] 60%",
		"💥 INTENSE CLASH: Trading micro-laser fire and scrap shrapnel...\n[████████░░] 80%",
		"📊 CONCLUDING: persistance matrices settling...\n[██████████] 100%",
	}

	for _, frame := range frames {
		formattedFrame := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"🚨 LIVE CLASH STREAM: %s\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
				"%s\n\n"+
				"Please do not close this transmission panel.",
			attackerName, frame,
		)
		_ = c.Edit(formattedFrame)
		time.Sleep(1 * time.Second)
	}

	// 2. Perform Combat calculations instantly to resolve the cinematic
	resolveTx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Combat calculation transaction failed.")
	}
	defer resolveTx.Rollback()

	var defenseForce int
	_ = resolveTx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM units WHERE encampment_id = $1", defenderCampID).Scan(&defenseForce)

	var defLevel int
	_ = resolveTx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", defenderCampID).Scan(&defLevel)
	if defLevel == 0 {
		defLevel = 1
	}
	defenseShieldMultiplier := 1.0 + (float64(defLevel) * 0.15)

	attackerOffenseRating := float64(attackForce) * 15.0
	defenderDefenseRating := float64(defenseForce) * 10.0 * defenseShieldMultiplier

	attackerCasualties := 0
	defenderCasualties := 0
	stolenScrap := 0.0

	var victory bool
	if attackerOffenseRating > defenderDefenseRating {
		victory = true
		defenderCasualties = defenseForce
		attackerCasualties = attackForce / 2

		// Loot calculations
		var defenderScrap float64
		_ = resolveTx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", defenderCampID).Scan(&defenderScrap)
		stolenScrap = defenderScrap * 0.40
		if stolenScrap < 0 {
			stolenScrap = 0
		}

		_, _ = resolveTx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", stolenScrap, attackerCampID)
		_, _ = resolveTx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0) WHERE encampment_id = $2", stolenScrap, defenderCampID)
	} else {
		attackerCasualties = attackForce
		defenderCasualties = defenseForce / 3
	}

	// Apply troop changes
	if attackerCasualties > 0 {
		_, _ = resolveTx.ExecContext(ctx, "UPDATE units SET quantity = GREATEST(quantity - $1, 0) WHERE encampment_id = $2", attackerCasualties, attackerCampID)
	}
	if defenderCasualties > 0 {
		_, _ = resolveTx.ExecContext(ctx, "UPDATE units SET quantity = GREATEST(quantity - $1, 0) WHERE encampment_id = $2", defenderCasualties, defenderCampID)
	}
	_, _ = resolveTx.ExecContext(ctx, "DELETE FROM units WHERE quantity <= 0")

	// Increment Hero Experience on victory
	if victory {
		_, _ = resolveTx.ExecContext(ctx, "UPDATE heroes SET battles_survived = battles_survived + 1 WHERE encampment_id = $1", attackerCampID)
	}

	// Commit the combat state changes
	_ = resolveTx.Commit()

	// 3. Render final Cinematic Summary Frame
	outcomeTitle := "🏆 VICTORY!"
	outcomeBody := fmt.Sprintf(
		"⚙️ Loot Recovered: +%.1f Scrap\n💀 Attack Losses: -%d drifters",
		stolenScrap, attackerCasualties,
	)
	if !victory {
		outcomeTitle = "☠️ MISSION FAILED"
		outcomeBody = fmt.Sprintf(
			"❌ Your attackers were completely wiped out.\n💀 Losses: -%d drifters",
			attackerCasualties,
		)
	}

	finalReport := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"%s — CONCLUDING TRANSMISSION\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Target: %s\n\n"+
			"OUTCOME REPORT:\n"+
			"%s\n\n"+
			"Wasteland metrics updated. Commander telemetry logged.",
		outcomeTitle, defenderName, outcomeBody,
	)

	// Send defender alert push asynchronously
	defenderAlert := fmt.Sprintf(
		"🚨 OUTPOST UNDER ATTACK!\n\n"+
			"Attacker Outpost: %s\n"+
			"Intruders breached your perimeter wall.\n"+
			"⚙️ Scrap Lost: %.1f\n"+
			"💀 Defense Casualties: %d units lost.",
		attackerName, stolenScrap, defenderCasualties,
	)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)

	return c.Send(finalReport, keyboards.CombatNavigation())
}
