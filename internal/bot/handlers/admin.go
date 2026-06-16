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

	return c.Send("⚡ ADMIN SYSTEM OVERRIDE: Master game tick successfully triggered and resolved.")
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

	// Inject resources
	query := `
		UPDATE resources 
		SET scrap = scrap + 5000.00, rations = rations + 5000.00, energy = energy + 5000.00 
		WHERE encampment_id = $1`

	_, err = h.DB.ExecContext(ctx, query, campID)
	if err != nil {
		log.Printf("Admin resource injection failed: %v", err)
		return c.Send("⚠️ Error executing resource injection.")
	}

	return c.Send("⚡ ADMIN OVERRIDE: Injected 5,000 Scrap, Rations, and Energy Cells into your camp.")
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

	// Update faction profile
	_, err := h.DB.ExecContext(ctx, "UPDATE users SET faction = $1 WHERE telegram_id = $2", targetFaction, sender.ID)
	if err != nil {
		log.Printf("Admin faction force-swap failed: %v", err)
		return c.Send("⚠️ Error updating faction in database.")
	}

	// Delete existing hero so the next load spawns the new faction hero commander
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
