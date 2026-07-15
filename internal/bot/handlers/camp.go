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

func (h *CampHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

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

// defenseModule describes one Defense Grid structure. All of these ride on
// the same generic `modules` table + HandleUpgradeCallback pipeline used by
// tent/scrap_heap/generator, so adding a new turret type needs no new
// schema or upgrade logic - just an entry here.
type defenseModule struct {
	moduleType string
	emoji      string
	title      string
	desc       string
}

var defenseGridModules = []defenseModule{
	{"light_laser", "🟡", "Light Laser", "Cheap early turret. Small defense rating bonus."},
	{"heavy_laser", "🔴", "Heavy Laser", "Solid anti-personnel turret. Moderate defense bonus."},
	{"gauss_cannon", "🎯", "Gauss Cannon", "Heavy anti-armor turret. Strong defense bonus."},
	{"ion_cannon", "🔵", "Ion Cannon", "Anti-drone/anti-air turret. Strong defense bonus."},
	{"plasma_turret", "🟣", "Plasma Turret", "Top-tier turret. Massive defense bonus."},
	{"anti_missile", "🚀🛡️", "Anti-Missile Battery", "Chance to auto-intercept incoming ICBM strikes for free."},
	{"warehouse", "📦", "Warehouse", "Increases resource storage capacity."},
}

// HandleDefenseGridPanel renders the turret/defensive structure upgrade panel.
func (h *CampHandler) HandleDefenseGridPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	var campLvl int
	_ = h.DB.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)

	panelText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"🛡️ DEFENSE GRID CONTROL\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Fortify your Outpost with turrets and utility structures.\n\n"

	selector := &telebot.ReplyMarkup{}
	var rows []telebot.Row

	for _, mod := range defenseGridModules {
		lvl := h.getModuleLevel(ctx, campID, mod.moduleType)
		cost := lvl * 150
		panelText += fmt.Sprintf("%s [%s Lvl %d] -> Cost: %d Scrap\n   %s\n\n", mod.emoji, mod.title, lvl, cost, mod.desc)
		btn := selector.Data(fmt.Sprintf("%s %s (%d)", mod.emoji, mod.title, lvl+1), "upgrade_mod", mod.moduleType)
		rows = append(rows, selector.Row(btn))
	}

	panelText += "━━━━━━━━━━━━━━━━━━━━━━"
	selector.Inline(rows...)

	return renderOrEdit(c, panelText, selector)
}

// infrastructureModules mirrors defenseGridModules but for economy/utility
// buildings. Same generic modules table + HandleUpgradeCallback pipeline -
// no new upgrade logic needed, just new type strings.
var infrastructureModules = []defenseModule{
	{"hangar", "🛬", "Hangar", "Increases maximum unit capacity (+20 per level)."},
	{"radar", "📡", "Radar", "Improves early-warning intel and counter-espionage odds."},
	{"solar_panel", "☀️", "Solar Panel", "Generates bonus Electricity independent of your Generator."},
	{"starport", "🚀", "Starport", "Reduces fuel cost for launching raids."},
	{"technology_center", "🧪", "Technology Center", "Boosts passive Ether generation rate."},
	{"trade_beacon", "📶", "Trade Beacon", "Discounts Ether Shop conversion costs."},
	{"small_shield", "🛡️", "Small Planetary Shield", "Reduces loot taken when raided (stacks with Nuclear Shields)."},
	{"large_shield", "🛡️🛡️", "Large Planetary Shield", "Major reduction of loot taken when raided."},
	{"engineering_bay", "⚙️", "Engineering Bay", "Reduces unit crafting material costs."},
	{"metal_mine", "🔩", "Metal Mine", "Passive Metal generation every tick."},
	{"crystal_mine", "💎", "Crystal Mine", "Passive Crystal generation every tick."},
}

// HandleInfrastructureGridPanel renders the economy/utility building panel.
func (h *CampHandler) HandleInfrastructureGridPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	panelText := "🏗️━━━━━━━━━━━━━━━━━━━━━━🏗️\n" +
		"🏗️ INFRASTRUCTURE GRID 🏗️\n" +
		"🏗️━━━━━━━━━━━━━━━━━━━━━━🏗️\n" +
		"Economy & utility buildings that keep your Outpost running.\n\n"

	selector := &telebot.ReplyMarkup{}
	var rows []telebot.Row

	for _, mod := range infrastructureModules {
		lvl := h.getModuleLevel(ctx, campID, mod.moduleType)
		cost := lvl * 150
		panelText += fmt.Sprintf("%s [%s Lvl %d] -> Cost: %d Scrap\n   %s\n\n", mod.emoji, mod.title, lvl, cost, mod.desc)
		btn := selector.Data(fmt.Sprintf("%s %s (%d)", mod.emoji, mod.title, lvl+1), "upgrade_mod", mod.moduleType)
		rows = append(rows, selector.Row(btn))
	}

	panelText += "🏗️━━━━━━━━━━━━━━━━━━━━━━🏗️"
	selector.Inline(rows...)

	return renderOrEdit(c, panelText, selector)
}

func (h *CampHandler) HandleActiveMining(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	var campLvl int
	_ = h.DB.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)

	var electricity float64
	query := `SELECT electricity FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, query, campID).Scan(&electricity)

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

	// Retrieve User alerts subscription settings
	var alertSubscribed bool
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(idle_miner_notifications, FALSE) FROM users WHERE telegram_id = $1", sender.ID).Scan(&alertSubscribed)

	subStatus := "🔕 Unsubscribed"
	toggleAlertLabel := "🔔 Subscribe to Idle Alerts"
	if alertSubscribed {
		subStatus = "🔔 Subscribed"
		toggleAlertLabel = "🔕 Unsubscribe from Idle Alerts"
	}

	var activeQueuesText string = ""
	rowsActive, errActive := h.DB.QueryContext(ctx, "SELECT resource_type, ready_at FROM active_mining_queues WHERE encampment_id = $1 AND is_completed = FALSE", campID)
	if errActive == nil {
		defer rowsActive.Close()
		hasQueues := false
		for rowsActive.Next() {
			var rType string
			var rReady time.Time
			if err := rowsActive.Scan(&rType, &rReady); err == nil {
				if !hasQueues {
					activeQueuesText += "⚙️ ACTIVE EXTRACTION PROCESSORS:\n"
					hasQueues = true
				}
				timeLeft := int(rReady.UTC().Sub(time.Now().UTC()).Seconds())
				if timeLeft < 0 {
					timeLeft = 0
				}
				activeQueuesText += fmt.Sprintf("• ⛏️ %s Miner: %ds remaining to finish extraction\n", strings.Title(rType), timeLeft)
			}
		}
		if hasQueues {
			activeQueuesText += "\n"
		}
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"⛏️ HEAVY EXTRACTION WORKSTATION [PRO]\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Assign available miners to start resource sweeps:\n\n"+
			"⚡ Electricity Cells: %.1f cells\n"+
			"👥 Miners Stationed: %d / %d active | Idle: %d miners\n"+
			"🏛️ Max Miner Capacity Cap: %d miners (Level %d Core)\n"+
			"📡 Idle Alerts Option: %s\n\n"+
			"%s"+
			"EXTRACTION QUEUE BLUEPRINTS (5m Duration):\n"+
			"🔩 [Forge Metal] — Costs: 10.0 Electricity (+20.0 Metal / miner)\n"+
			"💎 [Mine Crystal] — Costs: 20.0 Electricity (+5.0 Crystal / miner)\n"+
			"🎈 [Pump Hydrogen] — Costs: 15.0 Electricity (+10.0 Hydrogen / miner)\n\n"+
			"🛒 MINER SHOP DECK:\n"+
			"👥 Recruit Miner -> Cost: %d Scrap",
		electricity, activeMiners, ownedMiners, idleMiners, maxMiners, campLvl, subStatus, activeQueuesText, minerCost,
	)

	selector := &telebot.ReplyMarkup{}
	btnMetal := selector.Data("🔩 Metal", "mine_action", "metal")
	btnCrystal := selector.Data("💎 Crystal", "mine_action", "crystal")
	btnHydrogen := selector.Data("🎈 Hydrogen", "mine_action", "hydrogen")

	btnBuyMiner := selector.Data(fmt.Sprintf("Recruit Miner (%d Scrap)", minerCost), "mine_action", "buy_miner")
	btnToggleAlert := selector.Data(toggleAlertLabel, "mine_action", "toggle_alerts")

	selector.Inline(
		selector.Row(btnMetal, btnCrystal),
		selector.Row(btnHydrogen),
		selector.Row(btnBuyMiner),
		selector.Row(btnToggleAlert),
	)

	return c.Send(panelText, selector)
}

func (h *CampHandler) HandleMineCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	mineType := c.Args()[0]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Mining transaction failed."})
	}
	defer tx.Rollback()

	var campID string
	var campLvl int
	err = tx.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Establish outpost camp first using /start"})
	}

	if mineType == "toggle_alerts" {
		var currentSub bool
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(idle_miner_notifications, FALSE) FROM users WHERE telegram_id = $1 FOR UPDATE", sender.ID).Scan(&currentSub)
		
		_, _ = tx.ExecContext(ctx, "UPDATE users SET idle_miner_notifications = $1 WHERE telegram_id = $2", !currentSub, sender.ID)
		
		_ = tx.Commit()
		
		text := "🔕 Unsubscribed from idle miner alerts."
		if !currentSub {
			text = "🔔 Subscribed to idle miner alerts! You will be warned if workers remain idle."
		}
		_ = c.Respond(&telebot.CallbackResponse{Text: text})
		return h.HandleActiveMining(c)
	}

	var ownedMiners int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(miners, 1) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&ownedMiners)

	var activeMiners int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(miners_assigned), 0) FROM active_mining_queues WHERE encampment_id = $1 AND is_completed = FALSE", campID).Scan(&activeMiners)

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

	var electricity float64
	err = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
	if err != nil {
		log.Printf("Failed scanning electricity cells inside active transaction: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database error querying electricity reserves."})
	}

	var cost float64
	switch mineType {
	case "metal":
		cost = 10.0
	case "crystal":
		cost = 20.0
	case "hydrogen":
		cost = 15.0
	}

	if electricity < cost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Electricity: Required %.1f cells.", cost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID)

	readyAt := time.Now().UTC().Add(5 * time.Minute)
	queryInsertQueue := `
		INSERT INTO active_mining_queues (encampment_id, resource_type, miners_assigned, ready_at, is_completed)
		VALUES ($1, $2, 1, $3, FALSE)`
	
	_, err = tx.ExecContext(ctx, queryInsertQueue, campID, mineType, readyAt)
	if err != nil {
		log.Printf("Failed registering mining queue task: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing mining task queue."})
	}

	// Cinematic Drilling Telemetry animation loop
	msg, err := c.Bot().Send(c.Recipient(), "⛏️ COUPLING EXTRACTION DRILLS...")
	if err == nil {
		time.Sleep(300 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "⛏️ POSITIONING MINERS...\n[▰▱▱▱▱▱▱▱▱▱] 10% - Igniting heavy engine buffers...")
		time.Sleep(300 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "⛏️ POSITIONING MINERS...\n[▰▰▰▰▰▱▱▱▱▱] 50% - Calibrating resource selectors...")
		time.Sleep(300 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "⛏️ POSITIONING MINERS...\n[▰▰▰▰▰▰▰▰▰▰] 100% - Miner locked in position! Commencing extraction...")
		time.Sleep(300 * time.Millisecond)
		_ = c.Bot().Delete(msg)
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("⛏️ Miner deployed! %s extraction in progress (5m)...", mineType)})
	return h.HandleActiveMining(c)
}

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

	var crystal, neuro float64
	_ = h.DB.QueryRowContext(ctx, "SELECT crystal, neuro_cores FROM resources WHERE encampment_id = $1", campID).Scan(&crystal, &neuro)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🧬 GENETIC MUTATION CORE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Spend radioactive Crystal and Neuro Cores to mutate cellular properties:\n\n"+
			"RADIOACTIVE STOCKS:\n"+
			"☢️ Crystal Stock: %.1f kg | 🧠 Neuro Cores: %.0f\n\n"+
			"MUTATION INDEXES:\n"+
			"🧠 [Synaptic Accel Lvl %d / 5] (Cost: 20 Crystal, 5 Neuro)\n"+
			"   Reduces Automated Agent electricity use by 10%% per level.\n\n"+
			"🦾 [Cybernetic Salvage Lvl %d / 5] (Cost: 20 Crystal, 5 Neuro)\n"+
			"   Boosts passive Scrap mining yield by 15%% per level.\n\n"+
			"🧬 [Biospheric Adaptation Lvl %d / 5] (Cost: 20 Crystal, 5 Neuro)\n"+
			"   Reduces battle casualties by 10%% per level.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		crystal, neuro, synaptic, salvage, bio,
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

	var crystal, neuro float64
	_ = tx.QueryRowContext(ctx, "SELECT crystal, neuro_cores FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&crystal, &neuro)

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

	if crystal < 20.0 || neuro < 5.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Assets! Need 20 Crystal, 5 Neuro Cores."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - 20.0, neuro_cores = neuro_cores - 5.0 WHERE encampment_id = $1", campID)
	queryUpdate := fmt.Sprintf("UPDATE mutation_states SET %s = %s + 1 WHERE encampment_id = $1", dbColumn, dbColumn)
	_, _ = tx.ExecContext(ctx, queryUpdate, campID)

	// Cinematic Cellular Splicing Animation
	msg, errAnim := c.Bot().Send(c.Recipient(), "🧬 EXTRACTING CELLULAR NUCLEI FOR MUTATION...")
	if errAnim == nil {
		time.Sleep(350 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🧬 SPLICING GENETIC STRANDS...\n[▰▱▱▱▱▱▱▱▱▱] 20%\n🧠 Neuro-synaptic pathways opened.")
		time.Sleep(350 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🧬 SYNTHESIZING CELLULAR ADAPTATIONS...\n[▰▰▰▰▰▱▱▱▱▱] 60%\n☢️ Crystal radiation fallout stabilized.")
		time.Sleep(350 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🧬 COMMITTING CELLULAR MUTATIONS...\n[▰▰▰▰▰▰▰▰▰▰] 100%\n🏆 Genetic structure realigned successfully!")
		time.Sleep(350 * time.Millisecond)
		_ = c.Bot().Delete(msg)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing mutations: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing mutations state."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🧬 Genetic cellular property mutated successfully!"})
	return h.HandleMutationsPanel(c)
}

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

	var activeWeather string
	_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

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

		// Cinematic Construction Upgrade Loader
		msg, errAnim := c.Bot().Send(c.Recipient(), "🏗️ INITIALIZING STRUCTURAL CONVERSION BLUEPRINTS...")
		if errAnim == nil {
			time.Sleep(350 * time.Millisecond)
			_, _ = c.Bot().Edit(msg, "🏗️ ASSEMBLING MATERIALS...\n[▰▱▱▱▱▱▱▱▱▱] 15%\n⚙️ Structural frames aligned.")
			time.Sleep(350 * time.Millisecond)
			
			buildDelay := "Nominal construct speeds."
			if activeWeather == "acid_rain" {
				buildDelay = "⚠️ Acid Rain alert: Corrosive fallout detected (+100% build delays)."
			}
			
			_, _ = c.Bot().Edit(msg, fmt.Sprintf("🏗️ STRUCTURAL STRESSTESTS...\n[▰▰▰▰▰▱▱▱▱▱] 50%%\n🔩 Build modifiers: %s", buildDelay))
			time.Sleep(350 * time.Millisecond)
			_, _ = c.Bot().Edit(msg, "🏗️ SECURING PERIMETERS...\n[▰▰▰▰▰▰▰▰▰▰] 100%\n🏆 Blueprint committed! Foundations laid down successfully.")
			time.Sleep(350 * time.Millisecond)
			_ = c.Bot().Delete(msg)
		}

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

	// Cinematic Construction Upgrade Loader
	msg, errAnim := c.Bot().Send(c.Recipient(), "🏗️ INITIALIZING STRUCTURAL CONVERSION BLUEPRINTS...")
	if errAnim == nil {
		time.Sleep(350 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🏗️ ASSEMBLING MATERIALS...\n[▰▱▱▱▱▱▱▱▱▱] 15%\n⚙️ Structural frames aligned.")
		time.Sleep(350 * time.Millisecond)
		
		buildDelay := "Nominal construct speeds."
		if activeWeather == "acid_rain" {
			buildDelay = "⚠️ Acid Rain alert: Corrosive fallout detected (+100% build delays)."
		}
		
		_, _ = c.Bot().Edit(msg, fmt.Sprintf("🏗️ STRUCTURAL STRESSTESTS...\n[▰▰▰▰▰▱▱▱▱▱] 50%%\n🔩 Build modifiers: %s", buildDelay))
		time.Sleep(350 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🏗️ SECURING PERIMETERS...\n[▰▰▰▰▰▰▰▰▰▰] 100%\n🏆 Blueprint committed! Foundations laid down successfully.")
		time.Sleep(350 * time.Millisecond)
		_ = c.Bot().Delete(msg)
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🏗️ Construction of %s Level %d started!", moduleType, currentLvl+1)})
	return h.HandleStructuralUpgrades(c)
}

func (h *CampHandler) getModuleLevel(ctx context.Context, campID string, modType string) int {
	var lvl int
	query := `
		INSERT INTO modules (encampment_id, type, level) 
		VALUES ($1, $2, 1) 
		ON CONFLICT (encampment_id, type) 
		DO UPDATE SET level = modules.level 
		RETURNING level`
	err := h.DB.QueryRowContext(ctx, query, campID, modType).Scan(&lvl)
	if err != nil {
		log.Printf("Error resolving module level for type %s on camp %s: %v", modType, campID, err)
		return 1
	}
	return lvl
}