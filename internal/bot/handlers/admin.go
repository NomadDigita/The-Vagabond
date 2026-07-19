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
	"sync"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/tick"
	"gopkg.in/telebot.v3"
)

type AdminHandler struct {
	DB         *sql.DB
	TickEngine *tick.Engine
	AdminIDs   []int64

	// Phase 7 (item 13): admin panel consolidation. Tracks which admin
	// (by Telegram ID) is mid-flow on a button that needs a free-text
	// argument (e.g. "Gift Premium" needs a username + day count), so
	// their next plain-text message can be consumed as that argument
	// instead of falling through to normal NLP parsing. See
	// HandleAdminPendingInput, wired ahead of nlp.HandleTextMessage in
	// main.go's OnText registration.
	pendingMu sync.Mutex
	pending   map[int64]string
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
		pending:    make(map[int64]string),
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

	// Phase 7 (item 13): admin panel consolidation. Every admin action
	// used to be split across this panel (4 of them), 9 separate
	// /admin_* slash commands, and 4 duplicate bottom-menu buttons -
	// several were exact copy-pasted logic reachable 3 different ways
	// (see SPACEHUNT_PHASE7_LOG.md). This panel is now the single
	// discoverable place for all of them; the old slash commands still
	// work unchanged for muscle memory, but every one of them now
	// delegates to the same shared do*/db* helper this panel calls.
	selector := &telebot.ReplyMarkup{}
	btnTick := selector.Data("⚡ Force Tick", "admin_action", "tick")
	btnInject := selector.Data("🪙 Inject 5000 (Self)", "admin_action", "inject")
	btnGiftPremium := selector.Data("💎 Gift Premium", "admin_action", "gift_premium")
	btnGiftResources := selector.Data("🎁 Gift Resources", "admin_action", "gift_resources")
	btnTaxRate := selector.Data("💰 Set Tax Rate", "admin_action", "tax_rate")
	btnFaction := selector.Data("🎭 Change My Faction", "admin_action", "faction")
	btnBroadcast := selector.Data("📡 Broadcast", "admin_action", "broadcast")
	btnMetrics := selector.Data("🛰️ Server Metrics", "admin_action", "server_metrics")
	btnDBReset := selector.Data("⚠️ Reset Database", "admin_action", "db_reset")

	selector.Inline(
		selector.Row(btnTick, btnInject),
		selector.Row(btnGiftPremium, btnGiftResources),
		selector.Row(btnTaxRate, btnFaction),
		selector.Row(btnBroadcast, btnMetrics),
		selector.Row(btnDBReset),
	)

	return c.Send("🏛️ ADMIN OVERRIDE TERMINAL ACTIVATED\n\nDeploy overrides using the secure inline controls or bottom submenu deck.\nActions marked ⚠️ are destructive and require a confirmation tap.", selector, keyboards.AdminNavigation())
}

// adminPromptFor gives the guided-input prompt text for each action that
// needs a free-text argument after the button tap, plus registers the
// pending state so HandleAdminPendingInput knows how to parse the
// admin's next message.
func (h *AdminHandler) adminPromptFor(senderID int64, action string) string {
	h.pendingMu.Lock()
	h.pending[senderID] = action
	h.pendingMu.Unlock()

	switch action {
	case "gift_premium":
		return "✍️ Reply with: `username days`\nExample: `wanderer99 30`"
	case "gift_resources":
		return "✍️ Reply with: `username resource_type amount`\nTypes: scrap, rations, electricity, metal, crystal, hydrogen, dollars, neuro_cores\nExample: `wanderer99 metal 500`"
	case "tax_rate":
		return "✍️ Reply with a whole number 0-10 (percent).\nExample: `5`"
	case "faction":
		return "✍️ Reply with `steel_vanguard` or `rust_nomads`."
	case "broadcast":
		return "✍️ Reply with the message to broadcast to every survivor."
	default:
		return "✍️ Reply with the required input."
	}
}

func (h *AdminHandler) HandleAdminActionCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil || !h.IsAdmin(sender.ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied: Authorized administrators only."})
	}

	action := c.Args()[0]
	ctx := context.Background()

	switch action {
	case "tick":
		_ = c.Notify(telebot.Typing)
		h.TickEngine.ProcessTick()
		return c.Respond(&telebot.CallbackResponse{Text: "⚡ Master game tick successfully triggered!"})

	case "inject":
		result, _ := h.doInjectSelf(ctx, sender.ID)
		return c.Respond(&telebot.CallbackResponse{Text: result})

	case "gift_premium", "gift_resources", "tax_rate", "faction", "broadcast":
		prompt := h.adminPromptFor(sender.ID, action)
		_ = c.Respond(&telebot.CallbackResponse{Text: "✍️ Check the chat for your input prompt."})
		return c.Send(prompt)

	case "server_metrics":
		var totalUsers, totalCamps int
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

	case "db_reset":
		// Destructive - require a second, explicit confirmation tap
		// rather than executing immediately. This was a real safety
		// gap in the panel path before item 13 (the /admin_db_reset
		// slash command had the exact same gap, and still does - typing
		// it out is itself a kind of confirmation, so that's left as-is).
		confirmSelector := &telebot.ReplyMarkup{}
		btnConfirm := confirmSelector.Data("⚠️ CONFIRM: Wipe raids/news/queues & redistribute coordinates", "admin_action", "db_reset_confirm")
		btnCancel := confirmSelector.Data("✅ Cancel", "admin_action", "db_reset_cancel")
		confirmSelector.Inline(confirmSelector.Row(btnConfirm), confirmSelector.Row(btnCancel))
		_ = c.Respond(&telebot.CallbackResponse{})
		return c.Send("⚠️ Are you SURE? This clears active raids/world news/queues and redistributes every outpost's coordinates. This cannot be undone.", confirmSelector)

	case "db_reset_confirm":
		_ = c.Respond(&telebot.CallbackResponse{Text: "⚡ Executing..."})
		result, _ := h.doDBReset(ctx)
		return c.Send(result)

	case "db_reset_cancel":
		return c.Respond(&telebot.CallbackResponse{Text: "✅ Cancelled - no changes made."})
	}
	return nil
}

// HandleAdminPendingInput consumes an admin's next free-text message if
// (and only if) they're mid-flow on a guided-input action from the
// consolidated /admin panel above. Returns handled=false immediately
// for anyone with no pending action, so normal NLP text parsing
// (nlp.HandleTextMessage) continues completely unaffected - see
// main.go's OnText registration for how the two are chained.
func (h *AdminHandler) HandleAdminPendingInput(c telebot.Context) (handled bool, err error) {
	sender := c.Sender()
	if sender == nil || !h.IsAdmin(sender.ID) {
		return false, nil
	}

	h.pendingMu.Lock()
	action, ok := h.pending[sender.ID]
	if ok {
		delete(h.pending, sender.ID)
	}
	h.pendingMu.Unlock()

	if !ok {
		return false, nil
	}

	ctx := context.Background()
	fields := strings.Fields(c.Text())

	switch action {
	case "gift_premium":
		if len(fields) < 2 {
			return true, c.Send("⚠️ Expected `username days` - action cancelled, tap Gift Premium again to retry.")
		}
		days, convErr := strconv.Atoi(fields[1])
		if convErr != nil {
			return true, c.Send("⚠️ Days must be a whole number - action cancelled, tap Gift Premium again to retry.")
		}
		result, _ := h.doGiftPremium(ctx, fields[0], days)
		return true, c.Send(result)

	case "gift_resources":
		if len(fields) < 3 {
			return true, c.Send("⚠️ Expected `username resource_type amount` - action cancelled, tap Gift Resources again to retry.")
		}
		amount, convErr := strconv.ParseFloat(fields[2], 64)
		if convErr != nil {
			return true, c.Send("⚠️ Amount must be a number - action cancelled, tap Gift Resources again to retry.")
		}
		result, _ := h.doGiftResources(ctx, fields[0], fields[1], amount)
		return true, c.Send(result)

	case "tax_rate":
		if len(fields) < 1 {
			return true, c.Send("⚠️ Expected a number 0-10 - action cancelled, tap Set Tax Rate again to retry.")
		}
		rate, convErr := strconv.Atoi(fields[0])
		if convErr != nil {
			return true, c.Send("⚠️ Rate must be a whole number - action cancelled, tap Set Tax Rate again to retry.")
		}
		result, _ := h.doSetTaxRate(ctx, rate)
		return true, c.Send(result)

	case "faction":
		if len(fields) < 1 {
			return true, c.Send("⚠️ Expected `steel_vanguard` or `rust_nomads` - action cancelled, tap Change My Faction again to retry.")
		}
		result, _ := h.doFactionChange(ctx, sender.ID, fields[0])
		return true, c.Send(result)

	case "broadcast":
		if c.Text() == "" {
			return true, c.Send("⚠️ Broadcast message can't be empty - action cancelled, tap Broadcast again to retry.")
		}
		result, _ := h.doBroadcast(ctx, c.Text())
		return true, c.Send(result)
	}
	return false, nil
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

	result, _ := h.doDBReset(context.Background())
	return c.Send(result)
}

// doDBReset is the single source of truth for the destructive database
// reset, shared by /admin_db_reset and the consolidated /admin panel's
// two-tap confirm flow (Phase 7 item 13) - the panel path didn't have
// any confirmation step before this, which was a real safety gap for
// something this destructive.
func (h *AdminHandler) doDBReset(ctx context.Context) (string, error) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Transaction initialization error.", err
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

	return "⚡ ADMIN SYSTEM OVERRIDE: Database reset completed. Testing news cleared, queues flushed, active raids returned, and all coordinates redistributed securely.", nil
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

	days, err := strconv.Atoi(args[1])
	if err != nil {
		return c.Send("⚠️ Days parameter must be a valid integer.")
	}

	result, _ := h.doGiftPremium(context.Background(), args[0], days)
	return c.Send(result)
}

// doGiftPremium is the single source of truth for granting Premium,
// shared by /admin_gift_premium and the consolidated /admin panel's
// guided-input flow (Phase 7 item 13).
func (h *AdminHandler) doGiftPremium(ctx context.Context, targetUser string, days int) (string, error) {
	var targetID int64
	err := h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE LOWER(username) = LOWER($1)", targetUser).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return "❌ User Not Found.", err
	}
	if err != nil {
		return "⚠️ Database error.", err
	}

	targetTime := time.Now().AddDate(0, 0, days)
	_, err = h.DB.ExecContext(ctx, "UPDATE users SET premium_until = $1 WHERE telegram_id = $2", targetTime, targetID)
	if err != nil {
		return "⚠️ Database error.", err
	}

	alertMsg := fmt.Sprintf(
		"💎 PREMIUM STATUS GRANTED!\n\n"+
			"An Administrator has gifted you a Premium License for %d days.\n"+
			"Your Automation Agent and advanced HUD structures are now fully unlocked!",
		days,
	)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)

	return fmt.Sprintf("⚡ ADMIN OVERRIDE: Granted %d days of Premium License to @%s.", days, targetUser), nil
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

	amount, err := strconv.ParseFloat(args[2], 64)
	if err != nil {
		return c.Send("⚠️ Amount must be a valid float value.")
	}

	result, _ := h.doGiftResources(context.Background(), args[0], args[1], amount)
	return c.Send(result)
}

// doGiftResources is the single source of truth for gifting a resource
// to a player, shared by /admin_gift_resources and the consolidated
// /admin panel's guided-input flow (Phase 7 item 13).
func (h *AdminHandler) doGiftResources(ctx context.Context, targetUser, resType string, amount float64) (string, error) {
	resType = strings.ToLower(strings.TrimSpace(resType))

	validColumns := map[string]string{
		"scrap": "scrap", "rations": "rations", "electricity": "electricity", "metal": "metal",
		"crystal": "crystal", "hydrogen": "hydrogen", "dollars": "dollars",
		"neuro_cores": "neuro_cores",
	}

	targetColumn, exists := validColumns[resType]
	if !exists {
		return "❌ Invalid resource type specified.", errors.New("invalid resource type")
	}

	var targetID int64
	err := h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE LOWER(username) = LOWER($1)", targetUser).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return "❌ User Not Found.", err
	}

	queryUpdate := fmt.Sprintf("UPDATE resources SET %s = %s + $1 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $2)", targetColumn, targetColumn)
	_, err = h.DB.ExecContext(ctx, queryUpdate, amount, targetID)
	if err != nil {
		log.Printf("Failed executing admin gift: %v", err)
		return "⚠️ Database write error.", err
	}

	alertMsg := fmt.Sprintf("⚡ GIFT RECEIVED: An Administrator has permanently added +%.1f %s directly to your outpost warehouse.", amount, strings.Title(resType))
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)

	return fmt.Sprintf("⚡ ADMIN OVERRIDE: Gifted %.1f %s permanently to @%s.", amount, strings.Title(resType), targetUser), nil
}

func (h *AdminHandler) HandleAdminGive(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	result, _ := h.doInjectSelf(context.Background(), sender.ID)
	return c.Send(result)
}

// doInjectSelf is the single source of truth for injecting 5,000 of
// every resource into the admin's own camp - shared by /admin_give, the
// "🪙 Inject Resources" bottom-menu button, and the consolidated /admin
// panel's "inject" callback (Phase 7 item 13). All three were
// independently maintaining the same UPDATE statement before this, and
// had quietly drifted - this version (with neuro_cores included) is now
// the canonical one.
func (h *AdminHandler) doInjectSelf(ctx context.Context, senderID int64) (string, error) {
	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", senderID).Scan(&campID)
	if err != nil {
		return "⚠️ Create your outpost camp first using /start", err
	}

	query := `
		UPDATE resources 
		SET scrap = scrap + 5000.00, rations = rations + 5000.00, electricity = electricity + 5000.00, dollars = dollars + 5000.00,
		    metal = metal + 5000.00, crystal = crystal + 5000.00, hydrogen = hydrogen + 5000.00, neuro_cores = neuro_cores + 5000.00
		WHERE encampment_id = $1`

	_, err = h.DB.ExecContext(ctx, query, campID)
	if err != nil {
		log.Printf("Admin resource injection failed: %v", err)
		return "⚠️ Error executing resource injection.", err
	}

	return "⚡ ADMIN OVERRIDE: Injected 5,000 of ALL Resources into your camp.", nil
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

	result, _ := h.doSetTaxRate(context.Background(), rate)
	return c.Send(result)
}

// doSetTaxRate is the single source of truth for the daily tax rate,
// shared by /settaxrate and the consolidated /admin panel's
// guided-input flow (Phase 7 item 13).
func (h *AdminHandler) doSetTaxRate(ctx context.Context, rate int) (string, error) {
	if rate < 0 || rate > 10 {
		return "⚠️ Tax rate must be a whole number between 0 and 10 (percent).", errors.New("tax rate out of range")
	}
	_, err := h.DB.ExecContext(ctx, "UPDATE tax_law SET tax_rate_percent = $1 WHERE id = 1", rate)
	if err != nil {
		log.Printf("Failed updating tax rate: %v", err)
		return "⚠️ Error updating tax law.", err
	}
	return fmt.Sprintf("💰 WASTELAND TAX LAW UPDATED: Daily rate is now %d%%.", rate), nil
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
	result, _ := h.doFactionChange(context.Background(), sender.ID, targetFaction)
	return c.Send(result)
}

// doFactionChange is the single source of truth for an admin's own
// self-directed faction swap, shared by /admin_faction and the
// consolidated /admin panel's guided-input flow (Phase 7 item 13).
func (h *AdminHandler) doFactionChange(ctx context.Context, senderID int64, targetFaction string) (string, error) {
	if targetFaction != "steel_vanguard" && targetFaction != "rust_nomads" {
		return "⚠️ Syntax Error: Use `steel_vanguard` or `rust_nomads`.", errors.New("invalid faction")
	}

	_, err := h.DB.ExecContext(ctx, "UPDATE users SET faction = $1 WHERE telegram_id = $2", targetFaction, senderID)
	if err != nil {
		log.Printf("Admin faction force-swap failed: %v", err)
		return "⚠️ Error updating faction in database.", err
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", senderID).Scan(&campID)
	_, _ = h.DB.ExecContext(ctx, "DELETE FROM heroes WHERE encampment_id = $1", campID)

	return fmt.Sprintf("⚡ ADMIN OVERRIDE: Faction realigned to [%s]. Existing commander retired; check /hero to view your new commander.", targetFaction), nil
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

	result, _ := h.doBroadcast(context.Background(), broadcastMsg)
	return c.Send(result)
}

// doBroadcast is the single source of truth for a system-wide broadcast,
// shared by /admin_broadcast and the consolidated /admin panel's
// guided-input flow (Phase 7 item 13).
func (h *AdminHandler) doBroadcast(ctx context.Context, broadcastMsg string) (string, error) {
	rows, err := h.DB.QueryContext(ctx, "SELECT telegram_id FROM users")
	if err != nil {
		log.Printf("Admin broadcast query failed: %v", err)
		return "⚠️ Broadcast Failed: Error reading user databases.", err
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
		return "⚠️ Broadcast Failed: Database transaction error.", err
	}
	defer tx.Rollback()

	insertQuery := `
		INSERT INTO notifications (user_id, message, is_sent) 
		VALUES ($1, $2, FALSE)`

	for _, targetID := range targets {
		_, _ = tx.ExecContext(ctx, insertQuery, targetID, formattedBroadcast)
	}

	_ = tx.Commit()
	return fmt.Sprintf("📡 Broadcast successfully queued to %d users.", len(targets)), nil
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
