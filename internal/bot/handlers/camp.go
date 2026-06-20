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

type CampHandler struct {
	DB       *sql.DB
	AdminIDs []int64
}

func NewCampHandler(db *sql.DB, adminIDStrs string) *CampHandler {
	var ids []int64
	for _, s := range strings.Split(adminIDStrs, ",") {
		trimmed := strings.TrimSpace(s)
		if val, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			ids = append(ids, val)
		}
	}
	return &CampHandler{
		DB:       db,
		AdminIDs: ids,
	}
}

// IsAdmin checks if the sender is an authorized developer
func (h *CampHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

// HandleCamp renders the main outpost summary HUD and updates the bottom keyboard to CampNavigation
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
			"Select options on your bottom menu deck to trigger upgrades, check automation, view heroes, or actively mine.",
		campLvl, campName, scrap, tentLvl, heapLvl, genLvl,
	)

	return c.Send(panelText, keyboards.CampNavigation())
}

// HandleStructuralUpgrades renders ONLY the inline buttons
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

// HandleActiveMining renders the manual extraction workstation HUD (Stage 3 Complete)
func (h *CampHandler) HandleActiveMining(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	var campLvl int
	_ = h.DB.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)

	var energy, iron, oil, gold, silver, diamond, uranium, steel float64
	query := `SELECT energy, iron, oil, gold, silver, diamond, uranium, steel FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, query, campID).Scan(&energy, &iron, &oil, &gold, &silver, &diamond, &uranium, &steel)

	// Fetch Miner metrics
	var ownedMiners int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(miners, 1) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&ownedMiners)

	var activeMiners int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(miners_assigned), 0) FROM active_mining_queues WHERE encampment_id = $1 AND is_completed = FALSE", campID).Scan(&activeMiners)

	idleMiners := ownedMiners - activeMiners
	maxMiners := (campLvl / 5) + 1
	if maxMiners > 7 {
		maxMiners = 7
	}

	minerCost := ownedMiners * 500

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"⛏️ HEAVY EXTRACTION WORKSTATION [PRO]\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Assign available miners to start resource sweeps:\n\n"+
			"🔋 Energy Cells: %.1f cells\n"+
			"👥 Miners Stationed: %d / %d active | Idle: %d miners\n"+
			"🏛️ Max Miner Capacity Cap: %d miners (Level %d Core)\n\n"+
			"EXTRACTION QUEUE BLUEPRINTS (5m Duration):\n"+
			"🪨 [Extract Iron] — Costs: 5.0 Energy (+20.0 Iron / miner)\n"+
			"🛢️ [Pump Oil] — Costs: 5.0 Energy (+10.0 Oil / miner)\n"+
			"🪙 [Mine Gold] — Costs: 10.0 Energy (+5.0 Gold / miner)\n"+
			"🥈 [Mine Silver] — Costs: 5.0 Energy (+10.0 Silver / miner)\n"+
			"💎 [Mine Diamonds] — Costs: 15.0 Energy (+1.0 Diamond / miner)\n"+
			"☢️ [Mine Uranium] — Costs: 20.0 Energy (+5.0 Uranium / miner)\n"+
			"🎈 [Pump Hydrogen] — Costs: 15.0 Energy (+10.0 Hydrogen / miner)\n"+
			"🧱 [Forging Steel] — Costs: 10.0 Energy (+20.0 Steel / miner)\n\n"+
			"🛒 MINER SHOP DECK:\n"+
			"👥 Recruit Miner -> Cost: %d Scrap",
		energy, activeMiners, ownedMiners, idleMiners, maxMiners, campLvl, minerCost,
	)

	selector := &telebot.ReplyMarkup{}
	btnIron := selector.Data("🪨 Iron", "mine_action", "iron")
	btnOil := selector.Data("🛢️ Oil", "mine_action", "oil")
	btnGold := selector.Data("🪙 Gold", "mine_action", "gold")
	btnSilver := selector.Data("🥈 Silver", "mine_action", "silver")
	btnDiamond := selector.Data("💎 Diamond", "mine_action", "diamond")
	btnUranium := selector.Data("☢️ Uranium", "mine_action", "uranium")
	btnHydrogen := selector.Data("🎈 Hydrogen", "mine_action", "hydrogen")
	btnSteel := selector.Data("🧱 Steel", "mine_action", "steel")
	
	btnBuyMiner := selector.Data(fmt.Sprintf("Recruit Miner (%d Scrap)", minerCost), "mine_action", "buy_miner")

	selector.Inline(
		selector.Row(btnIron, btnOil),
		selector.Row(btnGold, btnSilver),
		selector.Row(btnDiamond, btnUranium),
		selector.Row(btnHydrogen, btnSteel),
		selector.Row(btnBuyMiner),
	)

	return c.Send(panelText, selector)
}

// HandleMineCallback handles purchasing miners and scheduling time-locked mining queues (Stage 3 Complete)
func (h *CampHandler) HandleMineCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	mineType := c.Args()[0]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Mining transaction failed."})
	}
	defer tx.Rollback()

	// Resolves target campID inside atomic database transactions to clear checking glitches
	var campID string
	var campLvl int
	err = tx.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Establish outpost camp first using /start"})
	}

	var ownedMiners int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(miners, 1) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&ownedMiners)

	var activeMiners int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(miners_assigned), 0) FROM active_mining_queues WHERE encampment_id = $1 AND is_completed = FALSE FOR UPDATE", campID).Scan(&activeMiners)

	idleMiners := ownedMiners - activeMiners

	if mineType == "buy_miner" {
		maxMiners := (campLvl / 5) + 1
		if maxMiners > 7 {
			maxMiners = 7
		}

		if ownedMiners >= maxMiners {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Miner Cap Reached: Core level %d limits you to %d miners.", campLvl, maxMiners)})
		}

		cost := ownedMiners * 500
		var scrap float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)

		if scrap < float64(cost) {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap: Need %d.", cost)})
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET miners = miners + 1 WHERE encampment_id = $1", campID)

		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: "🛒 Miner successfully recruited!"})
		return h.HandleActiveMining(c)
	}

	if idleMiners <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: All miners are currently engaged in active queues!"})
	}

	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&energy)

	var cost float64
	switch mineType {
	case "iron", "oil", "silver":
		cost = 5.0
	case "gold", "steel":
		cost = 10.0
	case "diamond":
		cost = 15.0
	case "hydrogen":
		cost = 15.0
	case "uranium":
		cost = 20.0
	}

	if energy < cost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Energy: Required %.1f cells.", cost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - $1 WHERE encampment_id = $2", cost, campID)

	readyAt := time.Now().UTC().Add(5 * time.Minute)
	queryInsertQueue := `
		INSERT INTO active_mining_queues (encampment_id, resource_type, miners_assigned, ready_at, is_completed)
		VALUES ($1, $2, 1, $3, FALSE)`
	
	_, err = tx.ExecContext(ctx, queryInsertQueue, campID, mineType, readyAt)
	if err != nil {
		log.Printf("Failed registering mining queue task: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing mining task queue."})
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("⛏️ Miner deployed! %s extraction in progress (5m)...", mineType)})
	return h.HandleActiveMining(c)
}

// HandleMutationsPanel renders biological modification workstations
func (h *CampHandler) HandleMutationsPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	var synaptic, salvage, bio int
	query := `SELECT synaptic_lvl, salvage_lvl, bio_lvl FROM mutation_states WHERE encampment_id = $1`
	err := h.DB.QueryRowContext(ctx, query, campID).Scan(&synaptic, &salvage, &bio)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO mutation_states (encampment_id) VALUES ($1)", campID)
		synaptic = 1
		salvage = 1
		bio = 1
	}

	var uranium, neuro float64
	_ = h.DB.QueryRowContext(ctx, "SELECT uranium, neuro_cores FROM resources WHERE encampment_id = $1", campID).Scan(&uranium, &neuro)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🧬 GENETIC MUTATION CORE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Spend radioactive Uranium and Neuro Cores to mutate cellular properties:\n\n"+
			"RADIOACTIVE STOCKS:\n"+
			"☢️ Uranium Stock: %.1f kg | 🧠 Neuro Cores: %.0f\n\n"+
			"MUTATION INDEXES:\n"+
			"🧠 [Synaptic Accel Lvl %d / 5] (Cost: 20 Uranium, 5 Neuro)\n"+
			"   Reduces Automated Agent energy use by 10%% per level.\n\n"+
			"🦾 [Cybernetic Salvage Lvl %d / 5] (Cost: 20 Uranium, 5 Neuro)\n"+
			"   Boosts passive Scrap mining yield by 15%% per level.\n\n"+
			"🧬 [Biospheric Adaptation Lvl %d / 5] (Cost: 20 Uranium, 5 Neuro)\n"+
			"   Reduces battle casualties by 10%% per level.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		uranium, neuro, synaptic, salvage, bio,
	)

	selector := &telebot.ReplyMarkup{}
	btnMutateSynaptic := selector.Data(fmt.Sprintf("🧠 Synaptic (%d)", synaptic+1), "mutate_mod", "synaptic")
	btnMutateSalvage := selector.Data(fmt.Sprintf("🦾 Salvage (%d)", salvage+1), "mutate_mod", "salvage")
	btnMutateBio := selector.Data(fmt.Sprintf("🧬 Bio (%d)", bio+1), "mutate_mod", "bio")

	selector.Inline(
		selector.Row(btnMutateSynaptic),
		selector.Row(btnMutateSalvage, btnMutateBio),
	)

	return c.Send(panelText, selector)
}

// HandleMutationCallback processes biological mutations
func (h *CampHandler) HandleMutationCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	modType := c.Args()[0]

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Mutation failed."})
	}
	defer tx.Rollback()

	var synaptic, salvage, bio int
	_ = tx.QueryRowContext(ctx, "SELECT synaptic_lvl, salvage_lvl, bio_lvl FROM mutation_states WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&synaptic, &salvage, &bio)

	var uranium, neuro float64
	_ = tx.QueryRowContext(ctx, "SELECT uranium, neuro_cores FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&uranium, &neuro)

	var currentLvl int
	var dbColumn string
	switch modType {
	case "synaptic":
		currentLvl = synaptic
		dbColumn = "synaptic_lvl"
	case "salvage":
		currentLvl = salvage
		dbColumn = "salvage_lvl"
	case "bio":
		currentLvl = bio
		dbColumn = "bio_lvl"
	}

	if currentLvl >= 5 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Max Mutation: Cellular adaptions are fully optimized."})
	}

	if uranium < 20.0 || neuro < 5.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Assets! Need 20 Uranium, 5 Neuro Cores."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET uranium = uranium - 20.0, neuro_cores = neuro_cores - 5.0 WHERE encampment_id = $1", campID)
	queryUpdate := fmt.Sprintf("UPDATE mutation_states SET %s = %s + 1 WHERE encampment_id = $1", dbColumn, dbColumn)
	_, _ = tx.ExecContext(ctx, queryUpdate, campID)

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing mutations: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing mutations state."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🧬 Genetic cellular property mutated successfully!"})
	return h.HandleMutationsPanel(c)
}

// HandleUpgradeCallback manages the inline upgrade actions (Dynamic campID & Admin lookup)
func (h *CampHandler) HandleUpgradeCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	
	moduleType := c.Args()[0]

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

	isAdmin := h.IsAdmin(sender.ID)
	
	if moduleType == "camp_core" {
		if campLvl >= 30 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Max Level reached (Level 30)."})
		}

		cost := campLvl * 500
		if isAdmin {
			cost = 0
		}

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

	if isAdmin {
		_, err = tx.ExecContext(ctx, "INSERT INTO modules (encampment_id, type, level, is_upgrading) VALUES ($1, $2, $3, FALSE) ON CONFLICT (encampment_id, type) DO UPDATE SET level = $3, is_upgrading = FALSE", campID, moduleType, currentLvl+1)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Admin Override write failure."})
		}
		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("⚡ ADMIN OVERRIDE: %s instantly upgraded to Level %d for free!", moduleType, currentLvl+1)})
		return h.HandleStructuralUpgrades(c)
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