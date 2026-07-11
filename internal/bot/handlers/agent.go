package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type AgentHandler struct {
	DB       *sql.DB
	AdminIDs []int64
}

func NewAgentHandler(db *sql.DB, adminIDStrs string) *AgentHandler {
	var ids []int64
	for _, s := range strings.Split(adminIDStrs, ",") {
		trimmed := strings.TrimSpace(s)
		if val, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			ids = append(ids, val)
		}
	}
	return &AgentHandler{DB: db, AdminIDs: ids}
}

func (h *AgentHandler) IsAdmin(id int64) bool {
	for _, a := range h.AdminIDs {
		if a == id {
			return true
		}
	}
	return false
}

func (h *AgentHandler) HandleAgent(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	var premiumUntil sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT premium_until FROM users WHERE telegram_id = $1", sender.ID).Scan(&premiumUntil)

	isPremium := h.IsAdmin(sender.ID)
	if premiumUntil.Valid && premiumUntil.Time.After(time.Now()) {
		isPremium = true
	}

	var isActive bool
	var mode string
	err := h.DB.QueryRowContext(ctx, "SELECT is_active, mode FROM agent_tasks WHERE user_id = $1", sender.ID).Scan(&isActive, &mode)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO agent_tasks (user_id, mode, is_active) VALUES ($1, 'collector', FALSE) ON CONFLICT DO NOTHING", sender.ID)
		isActive = false
		mode = "collector"
	} else if err != nil {
		log.Printf("Failed scanning agent config for %d: %v", sender.ID, err)
		return c.Send("⚠️ System connection error reading agent configuration.", keyboards.CampNavigation())
	}

	var electricity float64
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(r.electricity, 0) FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1", sender.ID).Scan(&electricity)

	statusLabel := "🔴 STANDBY (OFFLINE)"
	if isActive {
		statusLabel = "🟢 ACTIVE (RUNNING...)"
	}

	licenseText := "⚠️ NO LICENSE (LOCKED)"
	if isPremium {
		licenseText = "💎 PREMIUM GRANTED"
		if premiumUntil.Valid {
			licenseText = fmt.Sprintf("💎 PREMIUM (Expires: %s)", premiumUntil.Time.UTC().Format("2006-01-02"))
		}
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🧠 COGNITIVE AGENT MODULE [PRO]\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Your tactical agent handles offline outpost automation.\n\n"+
			"SYSTEM STATE:\n"+
			"🤖 Agent Status: %s\n"+
			"🏷️ License: %s\n"+
			"⚙️ Operational Mode: %s\n"+
			"🔋 Fuel Reserve: %.1f Electricity Cells\n\n"+
			"UPKEEP REQUIREMENTS:\n"+
			"⚡ Consumes 0.2 Electricity Cells per tick.\n"+
			"⚠️ Agent auto-shuts down if reserves hit 0.\n\n"+
			"BEHAVIOR MODES:\n"+
			"🛠️ [Collector]: Auto-scavenges +5.0 Scrap, +2.0 Rations per tick.\n"+
			"💱 [Collector Ω]: Auto-refines metals/fuels +15.0 Iron, +8.0 Oil, +10.0 Metal, +5.0 Hydrogen.\n"+
			"💎 [Collector Precious]: Auto-mines rare assets +5.0 Silver, +2.0 Gold, +1.0 Crystal, +0.1 Diamonds, +1.0 Neuro.\n"+
			"🏗️ [Builder]: Auto-upgrades lowest modules if Scrap permits.\n"+
			"🪖 [Military]: Auto-recruits Soldiers if Rations permit.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		statusLabel, licenseText, mode, electricity,
	)

	selector := &telebot.ReplyMarkup{}
	senderIDStr := strconv.FormatInt(sender.ID, 10)

	toggleLabel := "⚡ Boot Agent"
	if isActive {
		toggleLabel = "🔌 Shutdown Agent"
	}

	btnToggle := selector.Data(toggleLabel, "toggle_agent", senderIDStr)
	btnModeCollector := selector.Data("🛠️ Collector", "set_agent_mode", "collector", senderIDStr)
	btnModeCollectorOmega := selector.Data("💱 Collector Ω", "set_agent_mode", "collector_omega", senderIDStr)
	btnModeCollectorPrecious := selector.Data("💎 Precious", "set_agent_mode", "collector_precious", senderIDStr)
	btnModeBuilder := selector.Data("🏗️ Builder", "set_agent_mode", "builder", senderIDStr)
	btnModeMilitary := selector.Data("🪖 Military", "set_agent_mode", "military", senderIDStr)

	selector.Inline(
		selector.Row(btnToggle),
		selector.Row(btnModeCollector, btnModeCollectorOmega),
		selector.Row(btnModeCollectorPrecious),
		selector.Row(btnModeBuilder, btnModeMilitary),
	)

	return c.Send(panelText, selector)
}

func (h *AgentHandler) HandleToggleAgentCallback(c telebot.Context) error {
	ctx := context.Background()
	userIDStr := c.Args()[0]

	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	var premiumUntil sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT premium_until FROM users WHERE telegram_id = $1", userID).Scan(&premiumUntil)

	isPremium := h.IsAdmin(userID)
	if premiumUntil.Valid && premiumUntil.Time.After(time.Now()) {
		isPremium = true
	}

	if !isPremium {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Premium Required: This module is restricted to premium survivors."})
	}

	var currentActive bool
	_ = h.DB.QueryRowContext(ctx, "SELECT is_active FROM agent_tasks WHERE user_id = $1", userID).Scan(&currentActive)

	newActive := !currentActive

	if newActive {
		var electricity float64
		_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(r.electricity, 0) FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1", userID).Scan(&electricity)
		if electricity < 0.2 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Boot Failed: Insufficient Electricity."})
		}
	}

	_, err := h.DB.ExecContext(ctx, "UPDATE agent_tasks SET is_active = $1 WHERE user_id = $2", newActive, userID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed updating configuration state."})
	}

	alert := "🤖 Cognitive Agent booted successfully!"
	if !newActive {
		alert = "🔌 Cognitive Agent shut down."
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: alert})
	return h.HandleAgent(c)
}

func (h *AgentHandler) HandleSetModeCallback(c telebot.Context) error {
	ctx := context.Background()
	targetMode := c.Args()[0]
	userID := c.Args()[1]

	_, err := h.DB.ExecContext(ctx, "UPDATE agent_tasks SET mode = $1 WHERE user_id = $2", targetMode, userID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed updating behavioral profile."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("⚙️ Mode switched to: %s", targetMode)})
	return h.HandleAgent(c)
}