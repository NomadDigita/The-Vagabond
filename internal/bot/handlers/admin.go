package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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

// IsAdmin checks if the sender is an authorized developer
func (h *AdminHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

// HandleAdminPanel renders the Admin control board
func (h *AdminHandler) HandleAdminPanel(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.", keyboards.MainNavigation())
	}

	return c.Send("🏛️ ADMIN OVERRIDE TERMINAL ACTIVATED\n\nDeploy overrides using the submenu buttons below.", keyboards.AdminNavigation())
}

// HandleAdminTick manually triggers an instantaneous master loop iteration
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

// HandleAdminGiftPremium grants premium access to a username
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

// HandleAdminGiftResources grants Scrap permanently to a username
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
	if len(args) < 2 {
		return c.Send("⚠️ Syntax Error: Use `/admin_gift_resources [username] [scrap_amount]`")
	}

	targetUser := args[0]
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return c.Send("⚠️ Amount must be a valid float value.")
	}

	ctx := context.Background()

	var targetID int64
	err = h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE LOWER(username) = LOWER($1)", targetUser).Scan(&targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Send("❌ User Not Found.")
	}

	_, err = h.DB.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $2)", amount, targetID)
	if err != nil {
		return c.Send("⚠️ Database write error.")
	}

	alertMsg := fmt.Sprintf("⚡ GIFT RECEIVED: An Administrator has permanently added +%.1f Scrap directly to your outpost warehouse.", amount)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)

	return c.Send(fmt.Sprintf("⚡ ADMIN OVERRIDE: Gifted %.1f Scrap permanently to @%s.", amount, targetUser))
}

// HandleAdminGive injects resources instantly into the admin's outpost
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
		SET scrap = scrap + 5000.00, rations = rations + 5000.00, energy = energy + 5000.00, dollars = dollars + 5000.00
		WHERE encampment_id = $1`

	_, err = h.DB.ExecContext(ctx, query, campID)
	if err != nil {
		log.Printf("Admin resource injection failed: %v", err)
		return c.Send("⚠️ Error executing resource injection.")
	}

	return c.Send("⚡ ADMIN OVERRIDE: Injected 5,000 Scrap, Rations, Energy, and Dollars into your camp.")
}

// HandleAdminFaction force-swaps the admin's faction alignment
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

// HandleAdminBroadcast pushes a global alert to all registered users instantly
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
	rows.Close()

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
		_, err := tx.ExecContext(ctx, insertQuery, targetID, formattedBroadcast)
		if err != nil {
			log.Printf("Failed executing broadcast queue write for %d: %v", targetID, err)
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Admin broadcast transaction failed: %v", err)
		return c.Send("⚠️ Broadcast Failed: Database commit error.")
	}

	return c.Send(fmt.Sprintf("🛰️ Broadcast successfully dispatched to all %d active system lines.", len(targets)))
}

// HandleAdminMetrics displays live memory profiles and database statistics
func (h *AdminHandler) HandleAdminMetrics(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	_ = c.Notify(telebot.Typing)

	ctx := context.Background()

	var totalUsers int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)

	var totalCamps int
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