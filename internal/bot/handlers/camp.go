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

// HandleCamp renders the main outpost summary HUD
func (h *CampHandler) HandleCamp(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

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

	tentLvl := h.getModuleLevel(ctx, campID, "tent")
	heapLvl := h.getModuleLevel(ctx, campID, "scrap_heap")
	genLvl := h.getModuleLevel(ctx, campID, "generator")

	var scrap float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1", campID).Scan(&scrap)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"⛺ OUTPOST SECTOR SYSTEMS [LEVEL %d / 30]\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Outpost Name: %s\n"+
			"Available Scrap: %.1f\n\n"+
			"FACILITY MODULES:\n"+
			"⛺ [Tent] — Level %d\n"+
			"⚙️ [Scrap Heap] — Level %d\n"+
			"⚡ [Generator] — Level %d\n\n"+
			"Use the structural menu below to trigger upgrades.",
		campLvl, campName, scrap, tentLvl, heapLvl, genLvl,
	)

	return c.Send(panelText, keyboards.CampNavigation())
}

// HandleStructuralUpgrades renders ONLY the inline buttons. Removed campID from buttons (64-byte safe).
func (h *CampHandler) HandleStructuralUpgrades(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	var campLvl int
	_ = h.DB.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)

	tentLvl := h.getModuleLevel(ctx, campID, "tent")
	heapLvl := h.getModuleLevel(ctx, campID, "scrap_heap")
	genLvl := h.getModuleLevel(ctx, campID, "generator")

	tentCost := tentLvl * 150
	heapCost := heapLvl * 150
	genCost := genLvl * 150
	campUpgradeCost := campLvl * 500

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏗️ STRUCTURAL UPGRADE PANEL\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Select a facility to upgrade:\n\n"+
			"⛺ [Tent Lvl %d] -> Cost: %d Scrap\n"+
			"⚙️ [Scrap Heap Lvl %d] -> Cost: %d Scrap\n"+
			"⚡ [Generator Lvl %d] -> Cost: %d Scrap\n"+
			"🏛️ [Core Outpost Lvl %d] -> Cost: %d Scrap\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		tentLvl+1, tentCost, heapLvl+1, heapCost, genLvl+1, genCost, campLvl+1, campUpgradeCost,
	)

	selector := &telebot.ReplyMarkup{}

	// Passed only moduleType parameter to remain 64-byte safe
	btnUpgradeTent := selector.Data(fmt.Sprintf("⛺ Tent (%d)", tentLvl+1), "upgrade_mod", "tent")
	btnUpgradeHeap := selector.Data(fmt.Sprintf("⚙️ Heap (%d)", heapLvl+1), "upgrade_mod", "scrap_heap")
	btnUpgradeGen := selector.Data(fmt.Sprintf("⚡ Gen (%d)", genLvl+1), "upgrade_mod", "generator")
	btnUpgradeCamp := selector.Data(fmt.Sprintf("🏛️ Core (%d)", campLvl+1), "upgrade_mod", "camp_core")

	selector.Inline(
		selector.Row(btnUpgradeTent, btnUpgradeHeap),
		selector.Row(btnUpgradeGen, btnUpgradeCamp),
	)

	return c.Send(panelText, selector)
}

// HandleUpgradeCallback manages the inline upgrade actions (Dynamic campID lookup)
func (h *CampHandler) HandleUpgradeCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	moduleType := c.Args()[0]

	// Dynamically resolve campID inside handler to prevent 64-byte callback errors
	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error resolving Outpost."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction initialization error."})
	}
	defer tx.Rollback()

	var campLvl int
	_ = tx.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1 FOR UPDATE", campID).Scan(&campLvl)

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)

	if moduleType == "camp_core" {
		if campLvl >= 30 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Max Level reached (Level 30)."})
		}

		cost := campLvl * 500
		if scrap < float64(cost) {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap! Need %d.", cost)})
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE encampments SET level = level + 1 WHERE id = $1", campID)

		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🏆 Outpost Core upgraded to Level %d!", campLvl+1)})
		return h.HandleStructuralUpgrades(c)
	}

	currentLvl := h.getModuleLevel(ctx, campID, moduleType)
	cost := currentLvl * 150

	if currentLvl >= campLvl {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Prerequisite Block: Module levels cannot exceed your Outpost Core level."})
	}

	var exists bool
	_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", campID).Scan(&exists)
	if exists {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Construction Queue Busy."})
	}

	if scrap < float64(cost) {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap! Need %d.", cost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)

	readyAt := time.Now().Add(20 * time.Second)
	upsertModule := `
		INSERT INTO modules (encampment_id, type, level, is_upgrading, upgrade_ready_at)
		VALUES ($1, $2, $3, TRUE, $4)
		ON CONFLICT (encampment_id, type)
		DO UPDATE SET is_upgrading = TRUE, upgrade_ready_at = $4`

	_, err = tx.ExecContext(ctx, upsertModule, campID, moduleType, currentLvl, readyAt)
	if err != nil {
		log.Printf("Failed executing module upgrade: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing building configurations."})
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🏗️ Construction of %s Level %d started!", moduleType, currentLvl+1)})
	return h.HandleStructuralUpgrades(c)
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
