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
	// Parse comma-separated Admin IDs from environment
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

	// Invoke the tick engine directly
	h.TickEngine.ProcessTick()

	return c.Send("⚡ ADMIN SYSTEM OVERRIDE: Master game tick successfully triggered and resolved.")
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

	// Select all registered Telegram users
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

	// Insert announcements into notifications
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

	// Query total user counts
	var totalUsers int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)

	// Query total encampments
	var totalCamps int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM encampments").Scan(&totalCamps)

	// Fetch standard Go memory metrics
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
