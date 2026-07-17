package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/tick"
	"gopkg.in/telebot.v3"
)

type AdminHandler struct {
	DB         *sql.DB
	TickEngine *tick.Engine
	AdminIDs   []int64
}

func NewAdminHandler(db *sql.DB, tickEngine *tick.Engine, adminIDStrs string) *AdminHandler {
	var ids []int64
	for _, s := range strings.Split(adminIDStrs, ",") {
		trimmed := strings.TrimSpace(s)
		if val, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			ids = append(ids, val)
		}
	}

	return &AdminHandler{
		DB:         db,
		TickEngine: tickEngine,
		AdminIDs:   ids,
	}
}

func (h *AdminHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

func (h *AdminHandler) HandleAdminPanel(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.", keyboards.MainNavigation())
	}

	selector := &telebot.ReplyMarkup{}
	btnTick := selector.Data("⚡ Force Tick", "admin_action", "tick")
	btnInject := selector.Data("🪙 Inject 5000 Resources", "admin_action", "inject")
	btnGift := selector.Data("💎 Gift Premium", "admin_action", "gift")
	btnMetrics := selector.Data("🛰️ Server Metrics", "admin_action", "server_metrics")

	selector.Inline(
		selector.Row(btnTick, btnInject),
		selector.Row(btnGift, btnMetrics),
	)

	return c.Send("🏛️ ADMIN OVERRIDE TERMINAL ACTIVATED\n\nDeploy overrides using the secure inline controls or bottom submenu deck.", selector, keyboards.AdminNavigation())
}

func (h *AdminHandler) HandleAdminActionCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil || !h.IsAdmin(sender.ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied: Authorized administrators only."})
	}

	action := c.Args()[0]
	switch action {
	case "tick":
		_ = c.Notify(telebot.Typing)
		h.TickEngine.ProcessTick()
		return c.Respond(&telebot.CallbackResponse{Text: "⚡ Master game tick successfully triggered!"})
	case "inject":
		ctx := context.Background()
		var campID string
		err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
		if err != nil || campID == "" {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error: Establish outpost first."})
		}
		_, _ = h.DB.ExecContext(ctx, `
			UPDATE resources 
			SET scrap = scrap + 5000.00, rations = rations + 5000.00, electricity = electricity + 5000.00, dollars = dollars + 5000.00,
			    metal = metal + 5000.00, crystal = crystal + 5000.00, hydrogen = hydrogen + 5000.00
			WHERE encampment_id = $1`, campID)
		return c.Respond(&telebot.CallbackResponse{Text: "🪙 5,000 of ALL resources permanently injected!"})
	case "gift":
		return c.Respond(&telebot.CallbackResponse{Text: "💡 Tip: Use `/admin_gift_premium [username] [days]` in chat."})
	case "server_metrics":
		var totalUsers, totalCamps int
		ctx := context.Background()
		_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
		_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM encampments").Scan(&totalCamps)
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		metricsReport := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"💻 ADMINISTRATIVE METRICS PANEL\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"👥 Total Survivors: %d\n"+
				"⛺ Total Encampments: %d\n\n"+
				"⚙️ Active Goroutines: %d\n"+
				"🧠 Allocated Memory: %.2f MB\n"+
				"🧩 GC Cycles Executed: %d\n"+
				"━━━━━━━━━━━━━━━━━━━━━━",
			totalUsers, totalCamps, runtime.NumGoroutine(),
			float64(memStats.Alloc)/1024.0/1024.0, memStats.NumGC,
		)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🛰️ Memory telemetry fetched!"})
		return c.Send(metricsReport, keyboards.AdminNavigation())
	}
	return nil
}

func (h *AdminHandler) HandleAdminTick(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	_ = c.Notify(telebot.Typing)
	h.TickEngine.ProcessTick()

	return c.Send("⚡ ADMIN SYSTEM OVERRIDE: Master game tick successfully triggered.")
}

func (h *AdminHandler) HandleAdminDBReset(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	ctx := context.Background()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Transaction initialization error.")
	}
	defer tx.Rollback()

	// Safe Refunding Loop: Refund all active campaigns (and staged co-op lobbies) before erasing states
	queryActiveRaids := `
		SELECT r.id, r.attacker_id, r.defender_id, ea.user_id as attacker_user_id, COALESCE(ed.user_id, 0) as defender_user_id, ea.name as attacker_name, COALESCE(ed.name, 'Rogue Drone Nest') as defender_name
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.state = 'marching' OR r.state = 'engaged' OR r.state = 'returning' OR r.state = 'staged'`
	
	rowsActive, errActive := tx.QueryContext(ctx, queryActiveRaids)
	if errActive == nil {
		type activeRaid struct {
			id             string
			attackerID     string
			attackerUserID int64
			defenderUserID int64
			attackerName   string
			defenderName   string
		}
		var active []activeRaid
		for rowsActive.Next() {
			var ar activeRaid
			var defID sql.NullString
			if err := rowsActive.Scan(&ar.id, &ar.attackerID, &defID, &ar.attackerUserID, &ar.defenderUserID, &ar.attackerName, &ar.defenderName); err == nil {
				active = append(active, ar)
			}
		}
		rowsActive.Close()

		for _, ar := range active {
			var sols, mechs, buggies int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(buggies_mobilized, 0) FROM raid_forces WHERE raid_id = $1", ar.id).Scan(&sols, &mechs, &buggies)
			
			// Refund primary forces
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2, buggies = buggies + $3 WHERE encampment_id = $4", sols, mechs, buggies, ar.attackerID)
			
			// Refund co-op members
			rowsCoop, errCoop := tx.QueryContext(ctx, "SELECT encampment_id, soldiers_contributed, mechs_contributed FROM raid_coop_members WHERE raid_id = $1", ar.id)
			if errCoop == nil {
				for rowsCoop.Next() {
					var hCampID string
					var hSols, hMechs int
					if err := rowsCoop.Scan(&hCampID, &hSols, &hMechs); err == nil {
						_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", hSols, hMechs, hCampID)
						
						var hUserID int64
						_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", hCampID).Scan(&hUserID)
						coopAlert := "↩️ ALLIANCE NOTICE: Ongoing campaign has been aborted due to an administrative database reset. Your contributed forces have returned safely."
						_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", hUserID, coopAlert)
					}
				}
				rowsCoop.Close()
			}

			// Send real-time notification alerts
			attackerAlert := fmt.Sprintf("↩️ SYSTEM UPDATE: Your ongoing campaign against Outpost [%s] has been aborted due to an administrative database reset. Remaining forces have returned safely.", ar.defenderName)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ar.attackerUserID, attackerAlert)

			if ar.defenderUserID != 0 {
				defenderAlert := fmt.Sprintf("🛡️ SYSTEM UPDATE: The hostile campaign marching towards your base from Outpost [%s] was aborted due to an administrative database reset.", ar.attackerName)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ar.defenderUserID, defenderAlert)
			}
		}
	}

	// Cascading deletes on dependent tables
	_, _ = tx.ExecContext(ctx, "DELETE FROM raids")
	_, _ = tx.ExecContext(ctx, "DELETE FROM world_news")
	_, _ = tx.ExecContext(ctx, "DELETE FROM arena_queue")
	_, _ = tx.ExecContext(ctx, "DELETE FROM spy_missions")
	_, _ = tx.ExecContext(ctx, "UPDATE coordinates SET x = 0, y = 0")

	_ = tx.Commit()

	log.Println("Database coordinates manual reset initiated...")
	rows, err := h.DB.QueryContext(ctx, "SELECT c.id, c.region FROM coordinates c WHERE c.x = 0 AND c.y = 0")
	if err == nil {
		defer rows.Close()
		type zeroCoord struct {
			id     string
			region string
		}
		var coords []zeroCoord
		for rows.Next() {
			var z zeroCoord
			if err := rows.Scan(&z.id, &z.region); err == nil {
				coords = append(coords, z)
			}
		}

		for _, cCoord := range coords {
			rSource := rand.NewSource(time.Now().UnixNano())
			rGen := rand.New(rSource)
			var x, y int
			switch cCoord.region {
			case "Africa":
				x = rGen.Intn(991) + 10
				y = rGen.Intn(991) + 10
			case "Europe":
				x = -(rGen.Intn(991) + 10)
				y = rGen.Intn(991) + 10
			case "Asia":
				x = rGen.Intn(991) + 10
				y = -(rGen.Intn(991) + 10)
			default:
				x = -(rGen.Intn(991) + 10)
				y = -(rGen.Intn(991) + 10)
			}

			_, _ = h.DB.ExecContext(ctx, "UPDATE coordinates SET x = $1, y = $2 WHERE id = $3 AND NOT EXISTS(SELECT 1 FROM coordinates WHERE x = $1 AND y = $2)", x, y, cCoord.id)
		}
	}

	return c.Send("⚡ ADMIN SYSTEM OVERRIDE: Database reset completed. Testing news cleared, queues flushed, active raids returned, and all coordinates redistributed securely.")
}

func (h *AdminHandler) HandleAdminGiftPremium(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	payload := c.Message().Payload
	args := strings.Split(payload, " ")
	if len(args) < 2 {
		return c.Send("⚠️ Syntax Error: Use `/admin_gift_premium [username] [days]`")
	}

	targetUser := args[0]
	days, err := strconv.Atoi(args[1])
	if err != nil {
		return c.Send("⚠️ Days parameter must be a valid integer.")
	}

	ctx := context.Background()

	var targetID int64
	err = h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE LOWER(username) = LOWER($1)", targetUser).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Send("❌ User Not Found.")
	}

	targetTime := time.Now().AddDate(0, 0, days)
	_, err = h.DB.ExecContext(ctx, "UPDATE users SET premium_until = $1 WHERE telegram_id = $2", targetTime, targetID)
	if err != nil {
		return c.Send("⚠️ Database error.")
	}

	alertMsg := fmt.Sprintf(
		"💎 PREMIUM STATUS GRANTED!\n\n"+
			"An Administrator has gifted you a Premium License for %d days.\n"+
			"Your Automation Agent and advanced HUD structures are now fully unlocked!",
		days,
	)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)

	return c.Send(fmt.Sprintf("⚡ ADMIN OVERRIDE: Granted %d days of Premium License to @%s.", days, targetUser))
}

func (h *AdminHandler) HandleAdminGiftResources(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	payload := c.Message().Payload
	args := strings.Split(payload, " ")
	if len(args) < 3 {
		return c.Send("⚠️ Syntax Error: Use `/admin_gift_resources [username] [resource_type] [amount]`\nTypes: scrap, rations, electricity, metal, crystal, hydrogen, dollars, neuro_cores")
	}

	targetUser := args[0]
	resType := strings.ToLower(strings.TrimSpace(args[1]))
	amount, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return c.Send("⚠️ Amount must be a valid float value.")
	}

	validColumns := map[string]string{
		"scrap": "scrap", "rations": "rations", "electricity": "electricity", "metal": "metal",
		"crystal": "crystal", "hydrogen": "hydrogen", "dollars": "dollars",
		"neuro_cores": "neuro_cores",
	}

	targetColumn, exists := validColumns[resType]
	if !exists {
		return c.Send("❌ Invalid resource type specified.")
	}

	ctx := context.Background()

	var targetID int64
	err = h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE LOWER(username) = LOWER($1)", targetUser).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Send("❌ User Not Found.")
	}

	queryUpdate := fmt.Sprintf("UPDATE resources SET %s = %s + $1 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $2)", targetColumn, targetColumn)
	_, err = h.DB.ExecContext(ctx, queryUpdate, amount, targetID)
	if err != nil {
		log.Printf("Failed executing admin gift: %v", err)
		return c.Send("⚠️ Database write error.")
	}

	alertMsg := fmt.Sprintf("⚡ GIFT RECEIVED: An Administrator has permanently added +%.1f %s directly to your outpost warehouse.", amount, strings.Title(resType))
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)

	return c.Send(fmt.Sprintf("⚡ ADMIN OVERRIDE: Gifted %.1f %s permanently to @%s.", amount, strings.Title(resType), targetUser))
}

func (h *AdminHandler) HandleAdminGive(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	ctx := context.Background()

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	query := `
		UPDATE resources 
		SET scrap = scrap + 5000.00, rations = rations + 5000.00, electricity = electricity + 5000.00, dollars = dollars + 5000.00,
		    metal = metal + 5000.00, crystal = crystal + 5000.00, hydrogen = hydrogen + 5000.00, neuro_cores = neuro_cores + 5000.00
		WHERE encampment_id = $1`

	_, err = h.DB.ExecContext(ctx, query, campID)
	if err != nil {
		log.Printf("Admin resource injection failed: %v", err)
		return c.Send("⚠️ Error executing resource injection.")
	}

	return c.Send("⚡ ADMIN OVERRIDE: Injected 5,000 of ALL Resources into your camp.")
}

func (h *AdminHandler) HandleAdminSetTaxRate(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	args := c.Args()
	if len(args) < 1 {
		return c.Send("⚠️ Usage: /settaxrate [0-10]")
	}

	rate, err := strconv.Atoi(args[0])
	if err != nil || rate < 0 || rate > 10 {
		return c.Send("⚠️ Tax rate must be a whole number between 0 and 10 (percent).")
	}

	ctx := context.Background()
	_, err = h.DB.ExecContext(ctx, "UPDATE tax_law SET tax_rate_percent = $1 WHERE id = 1", rate)
	if err != nil {
		log.Printf("Failed updating tax rate: %v", err)
		return c.Send("⚠️ Error updating tax law.")
	}

	return c.Send(fmt.Sprintf("💰 WASTELAND TAX LAW UPDATED: Daily rate is now %d%%.", rate))
}

func (h *AdminHandler) HandleAdminFaction(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	targetFaction := c.Message().Payload
	if targetFaction != "steel_vanguard" && targetFaction != "rust_nomads" {
		return c.Send("⚠️ Syntax Error: Use `/admin_faction steel_vanguard` or `/admin_faction rust_nomads`")
	}

	ctx := context.Background()

	_, err := h.DB.ExecContext(ctx, "UPDATE users SET faction = $1 WHERE telegram_id = $2", targetFaction, sender.ID)
	if err != nil {
		log.Printf("Admin faction force-swap failed: %v", err)
		return c.Send("⚠️ Error updating faction in database.")
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	_, _ = h.DB.ExecContext(ctx, "DELETE FROM heroes WHERE encampment_id = $1", campID)

	return c.Send(fmt.Sprintf("⚡ ADMIN OVERRIDE: Faction realigned to [%s]. Existing commander retired; check /hero to view your new commander.", targetFaction))
}

func (h *AdminHandler) HandleAdminBroadcast(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	broadcastMsg := c.Message().Payload
	if broadcastMsg == "" {
		return c.Send("⚠️ Broadcast Failed: Payload empty. Syntax: `/admin_broadcast [message]`")
	}

	ctx := context.Background()

	rows, err := h.DB.QueryContext(ctx, "SELECT telegram_id FROM users")
	if err != nil {
		log.Printf("Admin broadcast query failed: %v", err)
		return c.Send("⚠️ Broadcast Failed: Error reading user databases.")
	}
	defer rows.Close()

	var targets []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			targets = append(targets, id)
		}
	}

	formattedBroadcast := fmt.Sprintf(
		"🛰️ SYSTEM BROADCAST (DEVELOPER MSG):\n\n%s",
		broadcastMsg,
	)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Broadcast Failed: Database transaction error.")
	}
	defer tx.Rollback()

	insertQuery := `
		INSERT INTO notifications (user_id, message, is_sent) 
		VALUES ($1, $2, FALSE)`

	for _, targetID := range targets {
		_, _ = tx.ExecContext(ctx, insertQuery, targetID, formattedBroadcast)
	}

	_ = tx.Commit()
	return c.Send(fmt.Sprintf("📡 Broadcast successfully queued to %d users.", len(targets)))
}

func (h *AdminHandler) HandleAdminMetrics(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	var totalUsers, totalCamps int
	ctx := context.Background()
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM encampments").Scan(&totalCamps)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metricsReport := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"💻 ADMINISTRATIVE METRICS PANEL\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"DATABASE TELEMETRY:\n"+
			"👥 Total Survivors: %d\n"+
			"⛺ Total Encampments: %d\n\n"+
			"GO ENGINE VIRTUAL PROFILES:\n"+
			"⚙️ Active Goroutines: %d\n"+
			"🧠 Allocated Memory: %.2f MB\n"+
			"🧩 Total GC Cycles Executed: %d\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		totalUsers, totalCamps, runtime.NumGoroutine(),
		float64(memStats.Alloc)/1024.0/1024.0, memStats.NumGC,
	)

	return c.Send(metricsReport, keyboards.MainNavigation())
}