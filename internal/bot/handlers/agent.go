package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type AgentHandler struct {
	DB *sql.DB
}

func NewAgentHandler(db *sql.DB) *AgentHandler {
	return &AgentHandler{DB: db}
}

// HandleAgent renders the high-end automation manager control panel
func (h *AgentHandler) HandleAgent(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	// Fetch current agent task record or initialize default
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
		// Initialize starting record
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO agent_tasks (user_id, mode, is_active) VALUES ($1, 'collector', FALSE) ON CONFLICT DO NOTHING", sender.ID)
		mode = "collector"
	}

	// Fetch current energy cells to show fuel levels
	var energy float64
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(r.energy, 0) FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1", sender.ID).Scan(&energy)

	statusLabel := "🔴 STANDBY (OFFLINE)"
	if isActive {
		statusLabel = "🟢 ACTIVE (RUNNING...)"
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🧠 COGNITIVE AGENT MODULE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Your tactical agent handles offline outpost automation.\n\n"+
			"SYSTEM STATE:\n"+
			"🤖 Agent Status: %s\n"+
			"⚙️ Operational Mode: %s\n"+
			"🔋 Fuel Reserve: %.1f Energy Cells\n\n"+
			"UPKEEP REQUIREMENTS:\n"+
			"⚡ Consumes 2.0 Energy Cells per tick.\n"+
			"⚠️ Agent auto-shuts down if reserves hit 0.\n\n"+
			"BEHAVIOR MODES:\n"+
			"🛠️ [Collector]: Auto-scavenges +2.0 Scrap, +1.0 Rations per tick.\n"+
			"🏗️ [Builder]: Auto-upgrades camp modules if Scrap permits.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		statusLabel, mode, energy,
	)

	selector := &telebot.ReplyMarkup{}

	// Convert int64 Sender ID to string to resolve compiler type requirements
	senderIDStr := strconv.FormatInt(sender.ID, 10)

	// Create control buttons
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

	return c.Send(panelText, selector, keyboards.MainNavigation())
}

// HandleToggleAgentCallback handles switching the active state of the offline automation agent
func (h *AgentHandler) HandleToggleAgentCallback(c telebot.Context) error {
	ctx := context.Background()
	userID := c.Args()[0]

	var currentActive bool
	err := h.DB.QueryRowContext(ctx, "SELECT is_active FROM agent_tasks WHERE user_id = $1", userID).Scan(&currentActive)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error accessing agent settings."})
	}

	newActive := !currentActive

	// If booting, verify they have enough starting energy
	if newActive {
		var energy float64
		_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(r.energy, 0) FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1", userID).Scan(&energy)
		if energy < 2.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Boot Failed: Insufficient Energy. Need at least 2.0 cells."})
		}
	}

	_, err = h.DB.ExecContext(ctx, "UPDATE agent_tasks SET is_active = $1 WHERE user_id = $2", newActive, userID)
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
