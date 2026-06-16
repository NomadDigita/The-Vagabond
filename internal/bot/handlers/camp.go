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

type CampHandler struct {
	DB *sql.DB
}

func NewCampHandler(db *sql.DB) *CampHandler {
	return &CampHandler{DB: db}
}

// HandleCamp renders the detailed building console
func (h *CampHandler) HandleCamp(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	// Query current levels of modules or default to 1 if not exists
	var campID string
	var campName string
	queryCamp := `SELECT id, name FROM encampments WHERE user_id = $1`
	err := h.DB.QueryRowContext(ctx, queryCamp, sender.ID).Scan(&campID, &campName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Send("⚠️ Access Denied: Establish your camp first using /start.")
		}
		return c.Send("⚠️ System connection error reading camp database.")
	}

	// Fetch or initialize standard modules
	tentLvl := h.getModuleLevel(ctx, campID, "tent")
	heapLvl := h.getModuleLevel(ctx, campID, "scrap_heap")
	genLvl := h.getModuleLevel(ctx, campID, "generator")

	// Check if any modules are currently upgrading
	upgradingModule, timeLeft := h.getUpgradingModule(ctx, campID)

	// Fetch current resources
	var scrap float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1", campID).Scan(&scrap)

	// Build costs (Level * 150 scrap)
	tentCost := tentLvl * 150
	heapCost := heapLvl * 150
	genCost := genLvl * 150

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"⛺ OUTPOST CONTROL MODULES\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Camp Name: %s\n"+
			"Available Scrap: %.1f\n\n"+
			"CURRENT UPGRADES:\n"+
			"⛺ [Tent] — Level %d\n"+
			"   Provides Storage. Next Upgrade: %d Scrap\n\n"+
			"⚙️ [Scrap Heap] — Level %d\n"+
			"   Produces Scrap. Next Upgrade: %d Scrap\n\n"+
			"⚡ [Generator] — Level %d\n"+
			"   Produces Energy. Next Upgrade: %d Scrap\n",
		campName, scrap, tentLvl, tentCost, heapLvl, heapCost, genLvl, genCost,
	)

	if upgradingModule != "" {
		panelText += fmt.Sprintf("\n🏗️ CONSTRUCTION PROGRESS:\nUpgrading [%s] (%ds remaining)\n", upgradingModule, timeLeft)
	}
	panelText += "━━━━━━━━━━━━━━━━━━━━━━"

	// Create physical inline control interface
	selector := &telebot.ReplyMarkup{}

	btnUpgradeTent := selector.Data(fmt.Sprintf("🔨 Tent (Lvl %d)", tentLvl+1), "upgrade_mod", "tent", campID)
	btnUpgradeHeap := selector.Data(fmt.Sprintf("🔨 Scrap Heap (Lvl %d)", heapLvl+1), "upgrade_mod", "scrap_heap", campID)
	btnUpgradeGen := selector.Data(fmt.Sprintf("🔨 Generator (Lvl %d)", genLvl+1), "upgrade_mod", "generator", campID)

	selector.Inline(
		selector.Row(btnUpgradeTent),
		selector.Row(btnUpgradeHeap),
		selector.Row(btnUpgradeGen),
	)

	return c.Send(panelText, selector, keyboards.MainNavigation())
}

// HandleUpgradeCallback manages the inline upgrade action verification and queuing
func (h *CampHandler) HandleUpgradeCallback(c telebot.Context) error {
	ctx := context.Background()

	// Parse button data
	moduleType := c.Args()[0]
	campID := c.Args()[1]

	// 1. Get current level
	currentLvl := h.getModuleLevel(ctx, campID, moduleType)
	cost := currentLvl * 150

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction initialization error."})
	}
	defer tx.Rollback()

	// 2. Check if another building is currently upgrading
	var exists bool
	_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", campID).Scan(&exists)
	if exists {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Construction Queue Full: Wait for current build to finish."})
	}

	// 3. Verify Scrap cost balance
	var scrap float64
	err = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error querying resource databases."})
	}

	if scrap < float64(cost) {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap! Need %d.", cost)})
	}

	// 4. Deduct Scrap from resources
	_, err = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing ledger updates."})
	}

	// 5. Insert or Update target module to initiate timer
	readyAt := time.Now().Add(20 * time.Second)
	upsertModule := `
		INSERT INTO modules (encampment_id, type, level, is_upgrading, upgrade_ready_at)
		VALUES ($1, $2, $3, TRUE, $4)
		ON CONFLICT (encampment_id, type)
		DO UPDATE SET is_upgrading = TRUE, upgrade_ready_at = $4`

	_, err = tx.ExecContext(ctx, upsertModule, campID, moduleType, currentLvl, readyAt)
	if err != nil {
		log.Printf("Failed executing module upsert upgrades: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing building configurations."})
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database writing commit failure."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🏗️ Construction of %s Level %d started!", moduleType, currentLvl+1)})

	// Refresh the control screen
	return h.HandleCamp(c)
}

func (h *CampHandler) getModuleLevel(ctx context.Context, campID string, modType string) int {
	var lvl int
	err := h.DB.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = $2", campID, modType).Scan(&lvl)
	if err != nil {
		// Initialize records to level 1 if missing
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO modules (encampment_id, type, level) VALUES ($1, $2, 1) ON CONFLICT DO NOTHING", campID, modType)
		return 1
	}
	return lvl
}

func (h *CampHandler) getUpgradingModule(ctx context.Context, campID string) (string, int) {
	var modType string
	var readyAt time.Time
	err := h.DB.QueryRowContext(ctx, "SELECT type, upgrade_ready_at FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE LIMIT 1", campID).Scan(&modType, &readyAt)
	if err != nil {
		return "", 0
	}
	diff := time.Until(readyAt)
	if diff < 0 {
		return "", 0
	}
	return modType, int(diff.Seconds())
}
