package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	gopkg "gopkg.in/telebot.v3"
)

type FactoryHandler struct {
	DB *sql.DB
}

func NewFactoryHandler(db *sql.DB) *FactoryHandler {
	return &FactoryHandler{DB: db}
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

	// Secure Hangar Allocator: Upsert row before reading inventory to prevent ErrNoRows defaults
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)

	var soldiers, drones, mechs, nukes int
	queryInv := `SELECT soldiers, drones, mechs, nukes FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&soldiers, &drones, &mechs, &nukes)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🪖 BARRACKS RECRUITMENT FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🪖 Soldiers: %d | 🛰️ Spy Devices: %d\n"+
			"🤖 Mechs: %d | ☢️ Nuclear Weapons: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🪖 [Soldier] — Cost: 50 Rations, 10 Iron (+10 Offense)\n"+
			"🛰️ [Spy Device] — Cost: 100 Iron, 10 Silver (+25 Recon)\n"+
			"🤖 [Colossus Mech] — Cost: 1000 Steel, 50 Uranium, 20 Gold (+350 Offense)\n"+
			"☢️ [Nuclear Device] — Cost: 2500 Steel, 500 Uranium, 10 Diamonds (+1500 Detonation)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		soldiers, drones, mechs, nukes,
	)

	selector := &gopkg.ReplyMarkup{}

	btnCraftSoldier := selector.Data("🪖 Recruit Soldier", "craft_item", "soldier")
	btnCraftDrone := selector.Data("🛰️ Assemble Spy Device", "craft_item", "drone")
	btnCraftMech := selector.Data("🤖 Forge Mech", "craft_item", "mech")
	btnCraftNuke := selector.Data("☢️ Forge Nuke", "craft_item", "nuke")

	selector.Inline(
		selector.Row(btnCraftSoldier, btnCraftDrone),
		selector.Row(btnCraftMech, btnCraftNuke),
	)

	return c.Send(panelText, selector)
}

func (h *FactoryHandler) HandleVehiclesPanel(c gopkg.Context) error {
	_ = c.Notify(gopkg.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	// Secure Hangar Allocator: Upsert row before reading inventory to prevent ErrNoRows defaults
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)

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
	btnCraftJet := selector.Data("✈️ Craft Jet", "craft_item", "jet")
	btnCraftHauler := selector.Data("🚛 Craft Hauler", "craft_item", "hauler")
	btnCraftTanker := selector.Data("🛡️ Craft Tanker", "craft_item", "tanker")
	btnCraftRig := selector.Data("🛠️ Craft Recovery Rig", "craft_item", "rig")

	selector.Inline(
		selector.Row(btnCraftBuggy, btnCraftShip),
		selector.Row(btnCraftJet),
		selector.Row(btnCraftHauler, btnCraftTanker),
		selector.Row(btnCraftRig),
	)

	return c.Send(panelText, selector)
}

func (h *FactoryHandler) HandleCraftCallback(c gopkg.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	item := c.Args()[0]

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Assembly failed."})
	}
	defer tx.Rollback()

	// Secure Hangar Allocator: Ensure the workshop row is fully allocated inside active transaction block
	_, _ = tx.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)

	var rations, steel, uranium, hydrogen, iron, oil, gold, silver, diamond float64
	queryRes := `SELECT rations, steel, uranium, hydrogen, iron, oil, gold, silver, diamond FROM resources WHERE encampment_id = $1 FOR UPDATE`
	_ = tx.QueryRowContext(ctx, queryRes, campID).Scan(&rations, &steel, &uranium, &hydrogen, &iron, &oil, &gold, &silver, &diamond)

	switch item {
	case "soldier":
		if rations < 50.0 || iron < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 50 Rations, 10 Iron."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - 50.0, iron = iron - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🪖 Soldier recruited successfully!"})

	case "drone":
		if iron < 100.0 || silver < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 100 Iron, 10 Silver."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET iron = iron - 100.0, silver = silver - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET drones = drones + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🛰️ Spy Device assembled!"})

	case "mech":
		if steel < 1000.0 || uranium < 50.0 || gold < 20.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 1000 Steel, 50 Uranium, 20 Gold."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 1000.0, uranium = uranium - 50.0, gold = gold - 20.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET mechs = mechs + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🤖 Colossus Mech forged successfully!"})

	case "nuke":
		if steel < 2500.0 || uranium < 500.0 || gold < 100.0 || diamond < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials!"})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 2500.0, uranium = uranium - 500.0, gold = gold - 100.0, diamond = diamond - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nukes = nukes + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "☢️ Nuclear Device assembled!"})

	case "buggy":
		if steel < 100.0 || oil < 20.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 100 Steel, 20 Oil."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 100.0, oil = oil - 20.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET buggies = buggies + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🚗 Scrap Buggy crafted successfully!"})

	case "ship":
		if steel < 300.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 300 Steel."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 300.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET ships = ships + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "⛵ Clipper Ship constructed!"})

	case "cargo_jet":
		if steel < 1000.0 || hydrogen < 200.0 || oil < 100.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials!"})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 1000.0, hydrogen = hydrogen - 200.0, oil = oil - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET jets = jets + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "✈️ Cargo Jet constructed successfully!"})

	case "hauler":
		if steel < 500.0 || oil < 50.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 500 Steel, 50 Oil."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 500.0, oil = oil - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET haulers = haulers + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🚛 Resource Hauler constructed successfully!"})

	case "tanker":
		if steel < 400.0 || hydrogen < 100.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 400 Steel, 100 Hydrogen."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 400.0, hydrogen = hydrogen - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET tankers = tankers + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🛡️ Fuel Tanker constructed!"})

	case "rig":
		if steel < 600.0 || iron < 50.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 600 Steel, 50 Iron."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 600.0, iron = iron - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET rigs = rigs + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🛠️ Recovery Rig constructed!"})
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing craft transaction: %v", err)
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	if item == "buggy" || item == "ship" || item == "cargo_jet" || item == "hauler" || item == "tanker" || item == "rig" {
		return h.HandleVehiclesPanel(c)
	}
	return h.HandleRecruitPanel(c)
}