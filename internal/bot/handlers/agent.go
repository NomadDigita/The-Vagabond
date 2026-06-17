package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
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
	for _, s := range stringSlice(adminIDStrs, ",") {
		if val, err := strconv.ParseInt(s, 10, 64); err == nil {
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

// HandleAgent renders the high-end automation manager control panel with Premium checks
func (h *AgentHandler) HandleAgent(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	// Check if user has premium status or is Admin
	var premiumUntil sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT premium_until FROM users WHERE telegram_id = $1", sender.ID).Scan(&premiumUntil)

	isPremium := h.IsAdmin(sender.ID)
	if premiumUntil.Valid && premiumUntil.Time.After(time.Now()) {
		isPremium = true
	}

	var isInstalled bool
	var isActive bool
	var mode string
	query := `
		SELECT EXISTS(SELECT 1 FROM agent_tasks WHERE user_id = $1),
		       COALESCE((SELECT is_active FROM agent_tasks WHERE user_id = $1), FALSE),
		       COALESCE((SELECT mode FROM agent_tasks WHERE user_id = $1), 'collector')
		FROM (SELECT 1) as dummy`

	err := h.DB.QueryRowContext(ctx, query, sender.ID).Scan(&isInstalled, &isActive, &mode)
	if err != nil {
		log.Printf("Failed querying agent status: %v", err)
		return c.Send("⚠️ System connection error reading agent configuration.", keyboards.MainNavigation())
	}

	if !isInstalled {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO agent_tasks (user_id, mode, is_active) VALUES ($1, 'collector', FALSE) ON CONFLICT DO NOTHING", sender.ID)
		mode = "collector"
	}

	var energy float64
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(r.energy, 0) FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1", sender.ID).Scan(&energy)

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
			"🔋 Fuel Reserve: %.1f Energy Cells\n\n"+
			"UPKEEP REQUIREMENTS:\n"+
			"⚡ Consumes 2.0 Energy Cells per tick.\n"+
			"⚠️ Agent auto-shuts down if reserves hit 0.\n\n"+
			"BEHAVIOR MODES:\n"+
			"🛠️ [Collector]: Auto-scavenges +2.0 Scrap, +1.0 Rations per tick.\n"+
			"🏗️ [Builder]: Auto-upgrades camp modules if Scrap permits.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		statusLabel, licenseText, mode, energy,
	)

	selector := &telebot.ReplyMarkup{}
	senderIDStr := strconv.FormatInt(sender.ID, 10)

	toggleLabel := "⚡ Boot Agent"
	if isActive {
		toggleLabel = "🔌 Shutdown Agent"
	}

	btnToggle := selector.Data(toggleLabel, "toggle_agent", senderIDStr)
	btnModeCollector := selector.Data("🛠️ Collector Mode", "set_agent_mode", "collector", senderIDStr)
	btnModeBuilder := selector.Data("🏗️ Builder Mode", "set_agent_mode", "builder", senderIDStr)

	selector.Inline(
		selector.Row(btnToggle),
		selector.Row(btnModeCollector, btnModeBuilder),
	)

	// Send without a trailing Reply Keyboard parameter so that inline buttons display successfully
	return c.Send(panelText, selector)
}

// HandleToggleAgentCallback handles switching the active state with premium checks
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
		var energy float64
		_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(r.energy, 0) FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1", userID).Scan(&energy)
		if energy < 2.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Boot Failed: Insufficient Energy."})
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

// HandleSetModeCallback toggles the behavior of the agent
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

func stringSlice(s, sep string) []string {
	var out []string
	for _, val := range stringSliceRaw(s, sep) {
		trimmed := trimSpace(val)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func stringSliceRaw(s, sep string) []string {
	var res []string
	start := 0
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			res = append(res, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	res = append(res, s[start:])
	return res
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
