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
	var tanks, shields, soldiers, drones, jets, mechs, nukes int
	queryInv := `
		SELECT fusion_tanks, nuclear_shields, soldiers, drones, jets, mechs, nukes 
		FROM workshop_inventory WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&tanks, &shields, &soldiers, &drones, &jets, &mechs, &nukes)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1)", campID)
	}

	var steel, uranium, hydrogen, iron, oil, gold, diamond float64
	queryRes := `SELECT steel, uranium, hydrogen, iron, oil, gold, diamond FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryRes, campID).Scan(&steel, &uranium, &hydrogen, &iron, &oil, &gold, &diamond)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏭 MILITARY ASSEMBLY & WORKSHOP FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"BARRACKS STOCKS:\n"+
			"🪖 Soldiers: %d | 🛰️ Drones: %d | ✈️ Jets: %d\n"+
			"🤖 Mechs: %d | ☢️ Nuclear Devices: %d\n"+
			"🚜 Fusion Tanks: %d | 🛡️ Nuclear Shields: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🪖 [Soldier] - Cost: 50 Rations, 10 Iron (+10 Offense)\n"+
			"🛰️ [Spy Drone] - Cost: 100 Iron, 10 Silver (+25 Offense)\n"+
			"✈️ [Fighter Jet] - Cost: 400 Iron, 50 Oil, 50 Hydrogen (+120 Offense)\n"+
			"🤖 [Colossus Mech] - Cost: 1000 Iron, 50 Uranium, 20 Gold (+350 Offense)\n"+
			"☢️ [Nuclear Device] - Cost: 2500 Iron, 500 Uranium, 100 Gold, 10 Diamonds (+1500 Offense)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		soldiers, drones, jets, mechs, nukes, tanks, shields,
	)

	selector := &gopkg.ReplyMarkup{}

	btnCraftSoldier := selector.Data("🪖 Recruit Soldier", "craft_item", "soldier")
	btnCraftDrone := selector.Data("🛰️ Assemble Spy Drone", "craft_item", "drone")
	btnCraftJet := selector.Data("✈️ Build Fighter Jet", "craft_item", "jet")
	btnCraftMech := selector.Data("🤖 Forge Colossus Mech", "craft_item", "mech")
	btnCraftNuke := selector.Data("☢️ Assemble Nuclear Weapon", "craft_item", "nuke")

	selector.Inline(
		selector.Row(btnCraftSoldier, btnCraftDrone),
		selector.Row(btnCraftJet, btnCraftMech),
		selector.Row(btnCraftNuke),
	)

	return c.Send(panelText, selector)
}

// HandleCraftCallback processes blueprints execution
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

	var rations, scrap, steel, uranium, hydrogen, iron, oil, gold, silver, diamond float64
	queryRes := `SELECT rations, scrap, steel, uranium, hydrogen, iron, oil, gold, silver, diamond FROM resources WHERE encampment_id = $1 FOR UPDATE`
	_ = tx.QueryRowContext(ctx, queryRes, campID).Scan(&rations, &scrap, &steel, &uranium, &hydrogen, &iron, &oil, &gold, &silver, &diamond)

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
		if steel < 200.0 || hydrogen < 80.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 200 Steel, 80 Hydrogen."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 200.0, hydrogen = hydrogen - 80.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET fusion_tanks = fusion_tanks + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🚜 Fusion armor deployed!"})
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing craft transaction: %v", err)
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	return h.HandleFactoryPanel(c)
}
