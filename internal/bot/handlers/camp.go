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

// HandleCamp renders the detailed building and leveling console
func (h *CampHandler) HandleCamp(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	// Query current levels of encampment
	var campID string
	var campName string
	var campLvl int
	queryCamp := `SELECT id, name, level FROM encampments WHERE user_id = $1`
	err := h.DB.QueryRowContext(ctx, queryCamp, sender.ID).Scan(&campID, &campName, &campLvl)
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

	// Build costs
	tentCost := tentLvl * 150
	heapCost := heapLvl * 150
	genCost := genLvl * 150
	campUpgradeCost := campLvl * 500

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"⛺ OUTPOST SECTOR SYSTEMS [LEVEL %d / 30]\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Outpost Name: %s\n"+
			"Available Scrap: %.1f\n\n"+
			"MODULE DETAILS:\n"+
			"⛺ [Tent] — Level %d\n"+
			"   Provides Storage. Next Upgrade: %d Scrap\n\n"+
			"⚙️ [Scrap Heap] — Level %d\n"+
			"   Produces Scrap. Next Upgrade: %d Scrap\n\n"+
			"⚡ [Generator] — Level %d\n"+
			"   Produces Energy. Next Upgrade: %d Scrap\n\n"+
			"🏛️ [Core Outpost Upgrade] — Lvl %d -> Lvl %d\n"+
			"   Increases Barracks limits. Cost: %d Scrap\n",
		campLvl, campName, scrap, tentLvl, tentCost, heapLvl, heapCost, genLvl, genCost, campLvl, campLvl+1, campUpgradeCost,
	)

	if upgradingModule != "" {
		panelText += fmt.Sprintf("\n🏗️ CONSTRUCTION PROGRESS:\nUpgrading [%s] (%ds remaining)\n", upgradingModule, timeLeft)
	}
	panelText += "━━━━━━━━━━━━━━━━━━━━━━"

	selector := &telebot.ReplyMarkup{}

	btnUpgradeTent := selector.Data(fmt.Sprintf("🔨 Tent (Lvl %d)", tentLvl+1), "upgrade_mod", "tent", campID)
	btnUpgradeHeap := selector.Data(fmt.Sprintf("🔨 Scrap Heap (Lvl %d)", heapLvl+1), "upgrade_mod", "scrap_heap", campID)
	btnUpgradeGen := selector.Data(fmt.Sprintf("🔨 Generator (Lvl %d)", genLvl+1), "upgrade_mod", "generator", campID)
	btnUpgradeCamp := selector.Data(fmt.Sprintf("🏛️ Core Lvl %d", campLvl+1), "upgrade_mod", "camp_core", campID)

	selector.Inline(
		selector.Row(btnUpgradeTent, btnUpgradeHeap),
		selector.Row(btnUpgradeGen, btnUpgradeCamp),
	)

	return c.Send(panelText, selector, keyboards.CampNavigation())
}

// HandleUpgradeCallback manages the inline upgrade action verification and queuing
func (h *CampHandler) HandleUpgradeCallback(c telebot.Context) error {
	ctx := context.Background()

	moduleType := c.Args()[0]
	campID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction initialization error."})
	}
	defer tx.Rollback()

	var campLvl int
	_ = tx.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1 FOR UPDATE", campID).Scan(&campLvl)

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)

	// 1. Handle Core Outpost Upgrade up to Level 30
	if moduleType == "camp_core" {
		if campLvl >= 30 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Max Level: Your Outpost Core is already max level (Level 30)."})
		}

		cost := campLvl * 500
		if scrap < float64(cost) {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap! Need %d.", cost)})
		}

		// Deduct and increment level instantly for outpost cores
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE encampments SET level = level + 1 WHERE id = $1", campID)

		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🏆 Outpost upgraded to Level %d!", campLvl+1)})
		return h.HandleCamp(c)
	}

	// 2. Standard modules upgrades
	currentLvl := h.getModuleLevel(ctx, campID, moduleType)
	cost := currentLvl * 150

	// ADMIN ULTIMATE POWER OVERRIDE: Instant & Free Upgrades
	isAdmin := false
	var userID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", campID).Scan(&userID)

	// Read admin IDs directly
	adminHandler := NewAdminHandler(h.DB, nil, "6582793388") // Adjust to your ID
	if adminHandler.IsAdmin(userID) {
		isAdmin = true
		cost = 0 // Cost becomes free
	}

	if currentLvl >= campLvl && moduleType != "camp_core" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Prerequisite Block: Module levels cannot exceed your Outpost Core level."})
	}

	if isAdmin {
		// Admin Bypass: Write level-up instantly
		_, err = tx.ExecContext(ctx, "INSERT INTO modules (encampment_id, type, level, is_upgrading) VALUES ($1, $2, $3, FALSE) ON CONFLICT (encampment_id, type) DO UPDATE SET level = $3, is_upgrading = FALSE", campID, moduleType, currentLvl+1)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Admin Override write failure."})
		}
		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("⚡ ADMIN OVERRIDE: %s instantly upgraded to Level %d for free!", moduleType, currentLvl+1)})
		return h.HandleCamp(c)
	}

	// Normal Player Flow: Check queue and resources... (remainder of normal player code stays unchanged)

	if currentLvl >= campLvl {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Prerequisite Block: Module levels cannot exceed your Outpost Core level."})
	}

	var exists bool
	_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", campID).Scan(&exists)
	if exists {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Construction Queue Full: Wait for current build to finish."})
	}

	if scrap < float64(cost) {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap! Need %d.", cost)})
	}

	// Deduct and Queue Upgrade
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)

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
	return h.HandleCamp(c)
}

func (h *CampHandler) getModuleLevel(ctx context.Context, campID string, modType string) int {
	var lvl int
	err := h.DB.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = $2", campID, modType).Scan(&lvl)
	if err != nil {
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
