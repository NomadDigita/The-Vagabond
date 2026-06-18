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

// HandleFactoryPanel renders the craft workstation HUD
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

	// Fetch or Initialize Workshop Inventory
	var tanks, shields, soldiers, drones, jets, mechs, nukes, buggies, ships int
	queryInv := `
		SELECT fusion_tanks, nuclear_shields, soldiers, drones, jets, mechs, nukes, COALESCE(buggies, 0), COALESCE(ships, 0) 
		FROM workshop_inventory WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&tanks, &shields, &soldiers, &drones, &jets, &mechs, &nukes, &buggies, &ships)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1)", campID)
	}

	var steel, uranium, hydrogen, iron, oil, gold float64
	queryRes := `SELECT steel, uranium, hydrogen, iron, oil, gold FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryRes, campID).Scan(&steel, &uranium, &hydrogen, &iron, &oil, &gold)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏭 MILITARY ASSEMBLY & WORKSHOP FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"BARRACKS STOCKS:\n"+
			"🪖 Soldiers: %d | 🛰️ Drones: %d | 🤖 Mechs: %d\n"+
			"🚜 Fusion Tanks: %d | 🛡️ Nuclear Shields: %d\n\n"+
			"LOGISTICS HANGAR STOCKS:\n"+
			"🚗 Scrap Buggies: %d | ⛵ Clipper Ships: %d | ✈️ Cargo Jets: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🚗 [Scrap Buggy] — Cost: 100 Steel, 20 Oil (Land travel +25%% speed)\n"+
			"⛵ [Clipper Ship] — Cost: 300 Steel, 50 Wood (Required to cross oceans)\n"+
			"✈️ [Cargo Jet] — Cost: 1000 Steel, 200 Hydrogen, 100 Oil (Reduces travel to flat 2h)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		soldiers, drones, mechs, tanks, shields, buggies, ships, jets,
	)

	selector := &gopkg.ReplyMarkup{}

	btnCraftBuggy := selector.Data("🚗 Craft Scrap Buggy", "craft_item", "buggy")
	btnCraftShip := selector.Data("⛵ Craft Clipper Ship", "craft_item", "ship")
	btnCraftJet := selector.Data("✈️ Craft Cargo Jet", "craft_item", "jet")

	selector.Inline(
		selector.Row(btnCraftBuggy, btnCraftShip),
		selector.Row(btnCraftJet),
	)

	// Send without a trailing Reply Keyboard parameter so that inline buttons display successfully
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

	case "jet":
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

	return h.HandleFactoryPanel(c)
}