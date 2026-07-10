package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/content"
	gopkg "gopkg.in/telebot.v3"
)

type FactoryHandler struct {
	DB *sql.DB
}

func NewFactoryHandler(db *sql.DB) *FactoryHandler {
	return &FactoryHandler{DB: db}
}

// renderOrEdit dynamically edits the current message if accessed via callbacks, saving layout space and preventing desyncs
func renderOrEdit(c gopkg.Context, text string, markup *gopkg.ReplyMarkup) error {
	if c.Callback() != nil {
		return c.Edit(text, markup)
	}
	return c.Send(text, markup)
}

func (h *FactoryHandler) HandleFactoryPanel(c gopkg.Context) error {
	_ = c.Notify(gopkg.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	panelText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"🏭 HEAVY WORKSHOP SECTOR SYSTEMS\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Outpost Name: Military Engineering Hangar\n\n" +
		"Select options on your bottom menu deck to recruit troops or craft logistics vehicles."

	return c.Send(panelText, keyboards.WorkshopNavigation())
}

func (h *FactoryHandler) HandleRecruitPanel(c gopkg.Context) error {
	_ = c.Notify(gopkg.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	// Secure Hangar Allocator: Ensure the workshop row is fully allocated and locked
	queryUpsert := `
		INSERT INTO workshop_inventory (encampment_id) 
		VALUES ($1) 
		ON CONFLICT (encampment_id) 
		DO UPDATE SET encampment_id = EXCLUDED.encampment_id`
	_, err := h.DB.ExecContext(ctx, queryUpsert, campID)
	if err != nil {
		log.Printf("Failed to allocate hangar row: %v", err)
	}

	var soldiers, drones, mechs, nukes, destroyers, bombers, scouts, battlecruisers, deathstars int
	queryInv := `SELECT soldiers, drones, mechs, nukes, COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(scouts,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0) FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&soldiers, &drones, &mechs, &nukes, &destroyers, &bombers, &scouts, &battlecruisers, &deathstars)

	scoutUnit, _ := content.FindUnit("scout")
	bcUnit, _ := content.FindUnit("battlecruiser")
	dsUnit, _ := content.FindUnit("deathstar")

	panelText := fmt.Sprintf(
		"🏭━━━━━━━━━━━━━━━━━━━━━━🏭\n"+
			"🪖⚙️ BARRACKS RECRUITMENT FORGE ⚙️🪖\n"+
			"🏭━━━━━━━━━━━━━━━━━━━━━━🏭\n\n"+
			"📦 CURRENT GARRISON:\n"+
			"🪖 Soldiers: %d  |  🛰️ Tactical Drones: %d\n"+
			"🤖 Mechs: %d  |  ☢️ Nuclear Weapons: %d\n"+
			"💥 Destroyers: %d  |  🛩️ Bombers: %d\n"+
			"🛵 Scout Walkers: %d  |  🚢👑 Battlecruisers: %d\n"+
			"🌑💀 Doomsday Rigs: %d\n\n"+
			"⚒️ MANUFACTURING BLUEPRINTS ⚒️\n"+
			"🪖 [Soldier] ➜ 💰50 Rations, 🔩10 Iron ➜ ⚔️ +10 Offense\n"+
			"🛰️ [Tactical Drone] ➜ 🔩100 Iron, 🥈10 Silver ➜ 🕵️ Spy Satellite / 🚨 Interceptor\n"+
			"🤖 [Colossus Mech] ➜ 🧱1000 Steel, ☢️50 Uranium, 🥇20 Gold ➜ ⚔️ +350 Offense\n"+
			"☢️ [Nuclear Device] ➜ 🧱2500 Steel, ☢️500 Uranium, 💎10 Diamonds ➜ 💥 +1500 Detonation\n"+
			"💥 [Destroyer] ➜ 🧱800 Steel, ☢️40 Uranium, 🥇15 Gold ➜ 🎯 Hard-counters Drones/Jets\n"+
			"🛩️ [Bomber] ➜ 🧱1200 Steel, ☢️60 Uranium, 🛢️100 Oil ➜ 🏰 Hard-counters Turrets\n"+
			"🛵 [%s] ➜ 🔩%.0f Iron, 🛢️%.0f Oil ➜ %s\n"+
			"🚢👑 [%s] ➜ 🧱%.0f Steel, ☢️%.0f Uranium, 🥇%.0f Gold, 💎%.0f Diamonds ➜ %s\n"+
			"🌑💀 [%s] ➜ 🧱%.0f Steel, ☢️%.0f Uranium, 🥇%.0f Gold, 💎%.0f Diamonds, 🧠%.0f Neuro Cores ➜ %s\n"+
			"🏭━━━━━━━━━━━━━━━━━━━━━━🏭",
		soldiers, drones, mechs, nukes, destroyers, bombers, scouts, battlecruisers, deathstars,
		scoutUnit.Title, scoutUnit.Cost["iron"], scoutUnit.Cost["oil"], scoutUnit.Flavor,
		bcUnit.Title, bcUnit.Cost["steel"], bcUnit.Cost["uranium"], bcUnit.Cost["gold"], bcUnit.Cost["diamond"], bcUnit.Flavor,
		dsUnit.Title, dsUnit.Cost["steel"], dsUnit.Cost["uranium"], dsUnit.Cost["gold"], dsUnit.Cost["diamond"], dsUnit.Cost["neuro_cores"], dsUnit.Flavor,
	)

	selector := &gopkg.ReplyMarkup{}

	btnCraftSoldier := selector.Data("🪖 Recruit Soldier", "craft_item", "soldier")
	btnCraftDrone := selector.Data("🛰️ Assemble Drone", "craft_item", "drone")
	btnCraftMech := selector.Data("🤖 Forge Mech", "craft_item", "mech")
	btnCraftNuke := selector.Data("☢️ Forge Nuke", "craft_item", "nuke")
	btnCraftDestroyer := selector.Data("💥 Forge Destroyer", "craft_item", "destroyer")
	btnCraftBomber := selector.Data("🛩️ Forge Bomber", "craft_item", "bomber")
	btnCraftScout := selector.Data("🛵 Build Scout", "craft_item", "scout")
	btnCraftBC := selector.Data("🚢👑 Forge Battlecruiser", "craft_item", "battlecruiser")
	btnCraftDS := selector.Data("🌑💀 Forge Doomsday Rig", "craft_item", "deathstar")

	selector.Inline(
		selector.Row(btnCraftSoldier, btnCraftDrone),
		selector.Row(btnCraftMech, btnCraftNuke),
		selector.Row(btnCraftDestroyer, btnCraftBomber),
		selector.Row(btnCraftScout, btnCraftBC),
		selector.Row(btnCraftDS),
	)

	return renderOrEdit(c, panelText, selector)
}

func (h *FactoryHandler) HandleVehiclesPanel(c gopkg.Context) error {
	_ = c.Notify(gopkg.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	// Secure Hangar Allocator: Ensure the workshop row is fully allocated and locked
	queryUpsert := `
		INSERT INTO workshop_inventory (encampment_id) 
		VALUES ($1) 
		ON CONFLICT (encampment_id) 
		DO UPDATE SET encampment_id = EXCLUDED.encampment_id`
	_, err := h.DB.ExecContext(ctx, queryUpsert, campID)
	if err != nil {
		log.Printf("Failed to allocate hangar row: %v", err)
	}

	var buggies, ships, jets, haulers, tankers, rigs int
	queryInv := `
		SELECT 
			COALESCE(buggies, 0), COALESCE(ships, 0), COALESCE(jets, 0), 
			COALESCE(haulers, 0), COALESCE(tankers, 0), COALESCE(rigs, 0) 
		FROM workshop_inventory 
		WHERE encampment_id = $1`
	
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&buggies, &ships, &jets, &haulers, &tankers, &rigs)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 LOGISTICS HANGAR FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 Scrap Buggies: %d | ⛵ Clipper Ships: %d | ✈️ Cargo Jets: %d\n"+
			"🚛 Resource Haulers: %d | 🛢️ Fuel Tankers: %d | 🔧 Recovery Rigs: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🚗 [Scrap Buggy] — Cost: 100 Steel, 20 Oil (Land travel +25%% speed)\n"+
			"⛵ [Clipper Ship] — Cost: 300 Steel (Required to cross oceans)\n"+
			"✈️ [Cargo Jet] — Cost: 1000 Steel, 200 Hydrogen, 100 Oil (Reduces travel to flat 2h)\n\n"+
			"🚛 [Resource Hauler] — Cost: 500 Steel, 50 Oil (+5,000 battle loot cap)\n"+
			"🛡️ [Fuel Tanker] — Cost: 400 Steel, 100 Hydrogen (-20%% march fuel costs)\n"+
			"🛠️ [Recovery Rig] — Cost: 600 Steel, 50 Iron (-15%% mechanical casualties)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		buggies, ships, jets, haulers, tankers, rigs,
	)

	selector := &gopkg.ReplyMarkup{}
	btnCraftBuggy := selector.Data("🚗 Craft Buggy", "craft_item", "buggy")
	btnCraftShip := selector.Data("⛵ Craft Ship", "craft_item", "ship")
	btnCraftJet := selector.Data("✈️ Craft Jet", "craft_item", "cargo_jet")
	btnCraftHauler := selector.Data("🚛 Craft Hauler", "craft_item", "hauler")
	btnCraftTanker := selector.Data("🛡️ Craft Tanker", "craft_item", "tanker")
	btnCraftRig := selector.Data("🛠️ Craft Recovery Rig", "craft_item", "rig")

	selector.Inline(
		selector.Row(btnCraftBuggy, btnCraftShip),
		selector.Row(btnCraftJet),
		selector.Row(btnCraftHauler, btnCraftTanker),
		selector.Row(btnCraftRig),
	)

	return renderOrEdit(c, panelText, selector)
}

func (h *FactoryHandler) HandleCraftCallback(c gopkg.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	item := strings.ToLower(strings.TrimSpace(c.Args()[0]))

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Assembly failed."})
	}
	defer tx.Rollback()

	// Secure Hangar Allocator: Ensure the workshop row is fully allocated inside active transaction block
	queryUpsert := `
		INSERT INTO workshop_inventory (encampment_id) 
		VALUES ($1) 
		ON CONFLICT (encampment_id) DO UPDATE SET encampment_id = EXCLUDED.encampment_id`
	_, _ = tx.ExecContext(ctx, queryUpsert, campID)

	var rations, steel, uranium, hydrogen, iron, oil, gold, silver, diamond float64
	queryRes := `SELECT rations, steel, uranium, hydrogen, iron, oil, gold, silver, diamond FROM resources WHERE encampment_id = $1 FOR UPDATE`
	_ = tx.QueryRowContext(ctx, queryRes, campID).Scan(&rations, &steel, &uranium, &hydrogen, &iron, &oil, &gold, &silver, &diamond)

	var successAlert string

	switch item {
	case "soldier":
		if rations < 50.0 || iron < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 50 Rations, 10 Iron."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - 50.0, iron = iron - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🪖 Soldier recruited successfully!"

	case "drone":
		if iron < 100.0 || silver < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 100 Iron, 10 Silver."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET iron = iron - 100.0, silver = silver - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET drones = drones + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛰️ Tactical Drone (Spy/Interceptor) assembled successfully!"

	case "mech":
		if steel < 1000.0 || uranium < 50.0 || gold < 20.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 1000 Steel, 50 Uranium, 20 Gold."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 1000.0, uranium = uranium - 50.0, gold = gold - 20.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET mechs = mechs + 1 WHERE encampment_id = $1", campID)
		successAlert = "🤖 Colossus Mech forged successfully!"

	case "nuke":
		if steel < 2500.0 || uranium < 500.0 || gold < 100.0 || diamond < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials!"})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 2500.0, uranium = uranium - 500.0, gold = gold - 100.0, diamond = diamond - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nukes = nukes + 1 WHERE encampment_id = $1", campID)
		successAlert = "☢️ Nuclear Device assembled!"

	case "destroyer":
		if steel < 800.0 || uranium < 40.0 || gold < 15.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 800 Steel, 40 Uranium, 15 Gold."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 800.0, uranium = uranium - 40.0, gold = gold - 15.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET destroyers = destroyers + 1 WHERE encampment_id = $1", campID)
		successAlert = "💥 Destroyer forged successfully!"

	case "bomber":
		if steel < 1200.0 || uranium < 60.0 || oil < 100.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 1200 Steel, 60 Uranium, 100 Oil."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 1200.0, uranium = uranium - 60.0, oil = oil - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET bombers = bombers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛩️ Bomber assembled successfully!"

	case "scout":
		scoutUnit, _ := content.FindUnit("scout")
		if iron < scoutUnit.Cost["iron"] || oil < scoutUnit.Cost["oil"] {
			return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %.0f Iron, %.0f Oil.", scoutUnit.Cost["iron"], scoutUnit.Cost["oil"])})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET iron = iron - $1, oil = oil - $2 WHERE encampment_id = $3", scoutUnit.Cost["iron"], scoutUnit.Cost["oil"], campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET scouts = scouts + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛵 Scout Walker rolled off the assembly line!"

	case "battlecruiser":
		bcUnit, _ := content.FindUnit("battlecruiser")
		if steel < bcUnit.Cost["steel"] || uranium < bcUnit.Cost["uranium"] || gold < bcUnit.Cost["gold"] || diamond < bcUnit.Cost["diamond"] {
			return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %.0f Steel, %.0f Uranium, %.0f Gold, %.0f Diamonds.", bcUnit.Cost["steel"], bcUnit.Cost["uranium"], bcUnit.Cost["gold"], bcUnit.Cost["diamond"])})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - $1, uranium = uranium - $2, gold = gold - $3, diamond = diamond - $4 WHERE encampment_id = $5", bcUnit.Cost["steel"], bcUnit.Cost["uranium"], bcUnit.Cost["gold"], bcUnit.Cost["diamond"], campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET battlecruisers = battlecruisers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🚢👑 BATTLECRUISER LAUNCHED! The pride of your fleet stands ready!"

	case "deathstar":
		var currentDS int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(deathstars, 0) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&currentDS)
		if currentDS >= 1 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Limit Reached: Only ONE Doomsday Rig can be commanded at a time."})
		}
		dsUnit, _ := content.FindUnit("deathstar")
		if steel < dsUnit.Cost["steel"] || uranium < dsUnit.Cost["uranium"] || gold < dsUnit.Cost["gold"] || diamond < dsUnit.Cost["diamond"] {
			return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %.0f Steel, %.0f Uranium, %.0f Gold, %.0f Diamonds, %.0f Neuro Cores.", dsUnit.Cost["steel"], dsUnit.Cost["uranium"], dsUnit.Cost["gold"], dsUnit.Cost["diamond"], dsUnit.Cost["neuro_cores"])})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - $1, uranium = uranium - $2, gold = gold - $3, diamond = diamond - $4, neuro_cores = neuro_cores - $5 WHERE encampment_id = $6", dsUnit.Cost["steel"], dsUnit.Cost["uranium"], dsUnit.Cost["gold"], dsUnit.Cost["diamond"], dsUnit.Cost["neuro_cores"], campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET deathstars = deathstars + 1 WHERE encampment_id = $1", campID)
		successAlert = "🌑💀👑 THE DOOMSDAY RIG IS OPERATIONAL! The Wasteland trembles at its shadow!"

	case "buggy":
		if steel < 100.0 || oil < 20.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 100 Steel, 20 Oil."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 100.0, oil = oil - 20.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET buggies = buggies + 1 WHERE encampment_id = $1", campID)
		successAlert = "🚗 Scrap Buggy crafted successfully!"

	case "ship":
		if steel < 300.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 300 Steel."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 300.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET ships = ships + 1 WHERE encampment_id = $1", campID)
		successAlert = "⛵ Clipper Ship constructed!"

	case "cargo_jet", "jet":
		if steel < 1000.0 || hydrogen < 200.0 || oil < 100.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials!"})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 1000.0, hydrogen = hydrogen - 200.0, oil = oil - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET jets = jets + 1 WHERE encampment_id = $1", campID)
		successAlert = "✈️ Cargo Jet constructed successfully!"

	case "hauler":
		if steel < 500.0 || oil < 50.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 500 Steel, 50 Oil."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 500.0, oil = oil - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET haulers = haulers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🚛 Resource Hauler constructed successfully!"

	case "tanker":
		if steel < 400.0 || hydrogen < 100.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 400 Steel, 100 Hydrogen."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 400.0, hydrogen = hydrogen - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET tankers = tankers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛡️ Fuel Tanker constructed!"

	case "rig":
		if steel < 600.0 || iron < 50.0 {
			return gopkg.Context(c).Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 600 Steel, 50 Iron."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 600.0, iron = iron - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET rigs = rigs + 1 WHERE encampment_id = $1", campID)
		successAlert = "🔧 Recovery Rig constructed!"
	}

	// Dynamic Post-Commit Success Dispatcher: Only sends notification after database safely registers changes
	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing craft transaction: %v", err)
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	if successAlert != "" {
		_ = c.Respond(&gopkg.CallbackResponse{Text: successAlert})
	}

	if item == "buggy" || item == "ship" || item == "cargo_jet" || item == "jet" || item == "hauler" || item == "tanker" || item == "rig" {
		return h.HandleVehiclesPanel(c)
	}
	return h.HandleRecruitPanel(c)
}