package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
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

// HandleRaidBoard displays player targets and offline AI training skirmishes
func (h *CombatHandler) HandleRaidBoard(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	var myCampID string
	var myCampName string
	var myX, myY int

	queryMe := `
		SELECT e.id, e.name, c.x, c.y 
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.user_id = $1`

	err := h.DB.QueryRowContext(ctx, queryMe, sender.ID).Scan(&myCampID, &myCampName, &myX, &myY)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	queryTargets := `
		SELECT e.id, e.name, u.first_name, c.x, c.y,
		       COALESCE((SELECT r.scrap FROM resources r WHERE r.encampment_id = e.id), 0) as scrap
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id != $1
		LIMIT 5`

	rows, err := h.DB.QueryContext(ctx, queryTargets, myCampID)
	if err != nil {
		log.Printf("Failed scanning target outposts: %v", err)
		return c.Send("⚠️ Failed to load target database matrix.", keyboards.CombatNavigation())
	}
	defer rows.Close()

	type target struct {
		id       string
		name     string
		owner    string
		x, y     int
		lootable float64
	}

	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.name, &t.owner, &t.x, &t.y, &t.lootable); err == nil {
			targets = append(targets, t)
		}
	}

	dashboard := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚔️ TACTICAL TARGET MATRIX\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Search target usernames using `/scout [username]`.\n" +
		"Staged expeditions require coordinate marching and rations.\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if len(targets) > 0 {
		for i, t := range targets {
			steps := math.Abs(float64(t.x-myX)) + math.Abs(float64(t.y-myY))
			if steps == 0 {
				steps = 1
			}
			marchTime := int(steps * 15)

			dashboard += fmt.Sprintf("[%d] Outpost: %s (Sector %d,%d)\n    Commander: %s\n    Travel Steps: %.0f | March Time: %ds\n    Estimated Loot: %.1f Scrap\n\n", i+1, t.name, t.x, t.y, t.owner, steps, marchTime, t.lootable)
			btnRaid := selector.Data(fmt.Sprintf("⚔️ Raid [%d]", i+1), "launch_raid", t.id)
			btnSpy := selector.Data(fmt.Sprintf("🛰️ Spy [%d]", i+1), "spy_action", t.id)
			buttons = append(buttons, selector.Row(btnRaid, btnSpy))
		}
	}

	dashboard += "🤖 AI TRAINING SKIRMISH TARGETS:\n" +
		"[AI] Rogue Drone Nest (Sector 1,1)\n" +
		"    Loot Yield: +50 Scrap | March Time: 15s\n\n"

	btnAI := selector.Data("🤖 Skirmish Rogue Drones", "launch_raid", "ai_drone_nest")
	buttons = append(buttons, selector.Row(btnAI))

	dashboard += "━━━━━━━━━━━━━━━━━━━━━━"

	selector.Inline(buttons...)
	return c.Send(dashboard, selector)
}

// HandleScout performs a username-based target search
func (h *CombatHandler) HandleScout(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	targetUsername := c.Message().Payload
	if targetUsername == "" {
		return c.Send("⚠️ Syntax Error: Use `/scout [telegram_username]` (without the @ symbol).")
	}

	ctx := context.Background()

	var tID string
	var tName string
	var tOwner string
	var tX, tY int
	var tScrap float64

	query := `
		SELECT e.id, e.name, u.first_name, c.x, c.y, r.scrap
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		JOIN coordinates c ON c.id = e.coordinate_id
		JOIN resources r ON r.encampment_id = e.id
		WHERE LOWER(u.username) = LOWER($1)`

	err := h.DB.QueryRowContext(ctx, query, targetUsername).Scan(&tID, &tName, &tOwner, &tX, &tY, &tScrap)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Send("❌ Target Not Found: No active outpost registered to that Telegram username.")
	} else if err != nil {
		log.Printf("Scouting database scan failed: %v", err)
		return c.Send("⚠️ Error scanning target parameters.")
	}

	report := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛰️ TARGET SCOUT INTEL\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Target Outpost: %s\n"+
			"Commander callsign: %s\n"+
			"Wasteland Location: Sector [%d, %d]\n"+
			"Lootable Vault Reserves: %.1f Scrap\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		tName, tOwner, tX, tY, tScrap,
	)

	selector := &telebot.ReplyMarkup{}
	btnRaid := selector.Data("⚔️ Launch Staged Expedition", "launch_raid", tID)
	btnSpy := selector.Data("🛰️ Intercept Signal", "spy_action", tID)

	selector.Inline(selector.Row(btnRaid, btnSpy))

	return c.Send(report, selector)
}

// HandleSpyCallback intercepts target telemetry by consuming 20 Energy Cells
func (h *CombatHandler) HandleSpyCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	targetCampID := c.Args()[0]

	var myCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Spying initialization error."})
	}
	defer tx.Rollback()

	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&energy)

	if energy < 20.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Energy: Spying requires 20.0 Energy Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - 20.0 WHERE encampment_id = $1", myCampID)

	var targetName string
	var rations float64
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", targetCampID).Scan(&targetName)
	_ = tx.QueryRowContext(ctx, "SELECT rations FROM resources WHERE encampment_id = $1", targetCampID).Scan(&rations)

	var soldiers int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT soldiers FROM workshop_inventory WHERE encampment_id = $1), 0)", targetCampID).Scan(&soldiers)

	_ = tx.Commit()

	_ = c.Respond(&telebot.CallbackResponse{Text: "📡 Signals Intercepted! Decoding telemetry..."})

	spyReport := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛰️ DECODED SIGNAL TELEMETRY\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Target Outpost: %s\n\n"+
			"INTERCEPTED METRICS:\n"+
			"🥫 Food Stocks: %.1f Rations\n"+
			"🪖 Infantry Power: %d Soldiers\n\n"+
			"Sensors suggest defenses are active.",
		targetName, rations, soldiers,
	)

	return c.Send(spyReport, keyboards.CombatNavigation())
}

// HandleLaunchRaidCallback registers a marching raid inside the database and alerts the defender
func (h *CombatHandler) HandleLaunchRaidCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	defenderCampID := c.Args()[0]

	var attackerCampID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&attackerCampID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error resolving Outpost."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database transaction error."})
	}
	defer tx.Rollback()

	var troopCount int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM units WHERE encampment_id = $1", attackerCampID).Scan(&troopCount)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error querying troop configurations."})
	}

	if troopCount <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Forbidden: You must have at least 1 unit to raid."})
	}

	var attackerName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerCampID).Scan(&attackerName)

	marchDuration := 15 * time.Second
	resolveTime := time.Now().Add(marchDuration)

	if defenderCampID == "ai_drone_nest" {
		insertRaid := `
			INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
			VALUES ($1, '00000000-0000-0000-0000-000000000000', 'marching', $2)
			RETURNING id`
		var raidID string
		err = tx.QueryRowContext(ctx, insertRaid, attackerCampID, resolveTime).Scan(&raidID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to register skirmish."})
		}
		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: "🤖 Skirmish launched! Marching on Drone Nest..."})
		return h.renderExpeditionPanel(c, raidID, attackerName, resolveTime)
	}

	var defenderName string
	var defenderUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", defenderCampID).Scan(&defenderName, &defenderUserID)

	var raidID string
	insertRaid := `
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
		VALUES ($1, $2, 'marching', $3)
		RETURNING id`
	err = tx.QueryRowContext(ctx, insertRaid, attackerCampID, defenderCampID, resolveTime).Scan(&raidID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to register raid."})
	}

	defenderAlert := fmt.Sprintf(
		"🚨 RADAR ALERT: HOSTILE RAID INBOUND!\n\n"+
			"Our sensors have detected a hostile staged raid marching from Outpost [%s].\n"+
			"Estimated Arrival Time: %s.\n\n"+
			"Upgrade your Tent or fortify your facilities immediately!",
		attackerName, resolveTime.UTC().Format("15:04:05"),
	)
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🚀 Raiders deployed! Marching towards target..."})

	return h.renderExpeditionPanel(c, raidID, attackerName, resolveTime)
}

func (h *CombatHandler) renderExpeditionPanel(c telebot.Context, raidID string, attackerName string, resolveTime time.Time) error {
	diff := time.Until(resolveTime)
	timeLeft := int(diff.Seconds())
	if timeLeft < 0 {
		timeLeft = 0
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚀 ACTIVE MILITARY EXPEDITION HUD\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Outpost Force: %s\n"+
			"Estimated Arrival: %s (%ds remaining)\n\n"+
			"TACTICAL COMMAND CONTROLS:\n"+
			"⚡ [Speed Up] - Costs 100 Scrap. Advances arrival by 30s.\n"+
			"↩️ [Abort] - Recalls troops instantly back to base.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		attackerName, resolveTime.UTC().Format("15:04:05"), timeLeft,
	)

	selector := &telebot.ReplyMarkup{}
	btnSpeed := selector.Data("⚡ Speed Up (100 Scrap)", "exp_action", "speed", raidID)
	btnAbort := selector.Data("↩️ Abort Expedition", "exp_action", "abort", raidID)

	selector.Inline(
		selector.Row(btnSpeed),
		selector.Row(btnAbort),
	)

	return c.Send(panelText, selector)
}

// HandleExpeditionActions processes inline tactical movements
func (h *CombatHandler) HandleExpeditionActions(c telebot.Context) error {
	ctx := context.Background()
	action := c.Args()[0]
	raidID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var state string
	var attackerID string
	var resolveTime time.Time
	err = tx.QueryRowContext(ctx, "SELECT state, attacker_id, resolve_time FROM raids WHERE id = $1 FOR UPDATE", raidID).Scan(&state, &attackerID, &resolveTime)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Expired: This expedition has already concluded."})
	}

	if state != "marching" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already concluded."})
	}

	switch action {
	case "speed":
		var scrap float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&scrap)
		if scrap < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap. Speed Up costs 100."})
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 100.0 WHERE encampment_id = $1", attackerID)
		newResolve := resolveTime.Add(-30 * time.Second)
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", newResolve, raidID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "⚡ Speed boosted! Arrival time advanced."})
		resolveTime = newResolve

	case "abort":
		_, _ = tx.ExecContext(ctx, "DELETE FROM raids WHERE id = $1", raidID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "↩️ Mission aborted!"})
		_ = tx.Commit()
		return c.Send("↩️ Expedition aborted. Forces returned safely to barracks.", keyboards.MainNavigation())
	}

	_ = tx.Commit()

	var attackerName string
	_ = h.DB.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerID).Scan(&attackerName)

	return h.renderExpeditionPanel(c, raidID, attackerName, resolveTime)
}
