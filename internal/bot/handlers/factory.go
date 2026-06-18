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

// HandleFactoryPanel renders the maternal heavy workshop intro and updates bottom menu context
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
		"Outpost Name: Military Engineering\n\n" +
		"Select options on your bottom menu deck to recruit troops or craft logistics vehicles."

	return c.Send(panelText, keyboards.WorkshopNavigation())
}

// HandleRecruitPanel renders barracks forge options with inline buttons
func (h *FactoryHandler) HandleRecruitPanel(c gopkg.Context) error {
	_ = c.Notify(gopkg.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	var soldiers, drones, mechs, nukes int
	queryInv := `SELECT soldiers, drones, mechs, nukes FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&soldiers, &drones, &mechs, &nukes)

	var rations, iron, steel, uranium, gold, diamond float64
	queryRes := `SELECT rations, iron, steel, uranium, gold, diamond FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryRes, campID).Scan(&rations, &iron, &steel, &uranium, &gold, &diamond)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🪖 BARRACKS RECRUITMENT FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🪖 Soldiers: %d | 🛰️ Drones: %d\n"+
			"🤖 Mechs: %d | ☢️ Nuclear Weapons: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🪖 [Soldier] — Cost: 50 Rations, 10 Iron (+10 Offense)\n"+
			"🛰️ [Spy Drone] — Cost: 100 Iron, 10 Silver (+25 Offense)\n"+
			"🤖 [Colossus Mech] — Cost: 1000 Steel, 50 Uranium, 20 Gold (+350 Offense)\n"+
			"☢️ [Nuclear Device] — Cost: 2500 Steel, 500 Uranium, 100 Gold, 10 Diamonds (+1500 Offense)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		soldiers, drones, mechs, nukes,
	)

	selector := &gopkg.ReplyMarkup{}

	btnCraftSoldier := selector.Data("🪖 Recruit Soldier", "craft_item", "soldier")
	btnCraftDrone := selector.Data("🛰️ Assemble Drone", "craft_item", "drone")
	btnCraftMech := selector.Data("🤖 Forge Mech", "craft_item", "mech")
	btnCraftNuke := selector.Data("☢️ Forge Nuke", "craft_item", "nuke")

	selector.Inline(
		selector.Row(btnCraftSoldier, btnCraftDrone),
		selector.Row(btnCraftMech, btnCraftNuke),
	)

	return c.Send(panelText, selector)
}

// HandleVehiclesPanel renders logistics hangar options with inline buttons
func (h *FactoryHandler) HandleVehiclesPanel(c gopkg.Context) error {
	_ = c.Notify(gopkg.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	var buggies, ships, jets int
	queryInv := `SELECT COALESCE(buggies, 0), COALESCE(ships, 0), COALESCE(jets, 0) FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&buggies, &ships, &jets)

	var steel, oil, hydrogen float64
	queryRes := `SELECT steel, oil, hydrogen FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryRes, campID).Scan(&steel, &oil, &hydrogen)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 LOGISTICS HANGAR FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 Scrap Buggies: %d | ⛵ Clipper Ships: %d | ✈️ Cargo Jets: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🚗 [Scrap Buggy] — Cost: 100 Steel, 20 Oil (Land travel +25%% speed)\n"+
			"⛵ [Clipper Ship] — Cost: 300 Steel (Required to cross oceans)\n"+
			"✈️ [Cargo Jet] — Cost: 1000 Steel, 200 Hydrogen, 100 Oil (Reduces travel to flat 2h)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		buggies, ships, jets,
	)

	selector := &gopkg.ReplyMarkup{}

	btnCraftBuggy := selector.Data("🚗 Craft Scrap Buggy", "craft_item", "buggy")
	btnCraftShip := selector.Data("⛵ Craft Clipper Ship", "craft_item", "ship")
	btnCraftJet := selector.Data("✈️ Craft Cargo Jet", "craft_item", "cargo_jet")

	selector.Inline(
		selector.Row(btnCraftBuggy, btnCraftShip),
		selector.Row(btnCraftJet),
	)

	return c.Send(panelText, selector)
}

// HandleCraftCallback processes blueprints execution
func (h *FactoryHandler) HandleCraftCallback(c gopkg.Context) error {
	ctx := context.Background()
	sender := gopkg.Context(c).Sender()

	item := c.Args()[0]

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Assembly failed."})
	}
	defer tx.Rollback()

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
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🛰️ Spy Drone constructed!"})

	case "jet":
		if iron < 400.0 || oil < 50.0 || hydrogen < 50.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 400 Iron, 50 Oil, 50 Hydrogen."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET iron = iron - 400.0, oil = oil - 50.0, hydrogen = hydrogen - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET jets = jets + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "✈️ Fighter Jet constructed!"})

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
		_ = c.Respond(&gopkg.CallbackResponse{Text: "☢️ Nuclear strike weapon assembled!"})

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
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 1000 Steel, 200 Hydrogen, 100 Oil."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 1000.0, hydrogen = hydrogen - 200.0, oil = oil - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET jets = jets + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "✈️ Cargo Jet constructed successfully!"})
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing craft transaction: %v", err)
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	// Refreshes panel context cleanly depending on what was bought
	if item == "buggy" || item == "ship" || item == "cargo_jet" {
		return h.HandleVehiclesPanel(c)
	}
	return h.HandleRecruitPanel(c)
}