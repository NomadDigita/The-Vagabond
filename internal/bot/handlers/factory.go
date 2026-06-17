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
	var tanks int
	var shields int
	queryInv := `SELECT fusion_tanks, nuclear_shields FROM workshop_inventory WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&tanks, &shields)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id, fusion_tanks, nuclear_shields) VALUES ($1, 0, 0)", campID)
		tanks = 0
		shields = 0
	}

	var steel, uranium, hydrogen float64
	queryRes := `SELECT steel, uranium, hydrogen FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryRes, campID).Scan(&steel, &uranium, &hydrogen)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏭 HEAVY ARMS WORKSHOP FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Spend heavy raw components to craft advanced military and defensive assets.\n\n"+
			"AVAILABLE MATERIALS:\n"+
			"🧱 Steel Stock: %.1f tons\n"+
			"☢️ Uranium Stock: %.1f kg\n"+
			"🎈 Hydrogen Stock: %.1f L\n\n"+
			"WORKSHOP INVENTORY:\n"+
			"🚜 Fusion Tanks: %d active vehicles\n"+
			"🛡️ Nuclear Shielding: %d installations\n\n"+
			"ASSEMBLY BLUEPRINTS:\n"+
			"🚜 [Fusion Tank] — Costs: 100 Steel, 50 Hydrogen (+50%% Offensive Power)\n"+
			"🛡️ [Nuclear Shielding] — Costs: 200 Steel, 20 Uranium (Reduces PvP loot loss by 50%%)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		steel, uranium, hydrogen, tanks, shields,
	)

	selector := &gopkg.ReplyMarkup{}

	btnCraftTank := selector.Data("🚜 Craft Fusion Tank", "craft_item", "tank", campID)
	btnCraftShield := selector.Data("🛡️ Install Nuclear Shield", "craft_item", "shield", campID)

	selector.Inline(
		selector.Row(btnCraftTank),
		selector.Row(btnCraftShield),
	)

	return c.Send(panelText, selector, keyboards.CombatNavigation())
}

// HandleCraftCallback processes blueprints execution
func (h *FactoryHandler) HandleCraftCallback(c gopkg.Context) error {
	ctx := context.Background()

	item := c.Args()[0]
	campID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Assembly failed."})
	}
	defer tx.Rollback()

	var steel, uranium, hydrogen float64
	queryRes := `SELECT steel, uranium, hydrogen FROM resources WHERE encampment_id = $1 FOR UPDATE`
	_ = tx.QueryRowContext(ctx, queryRes, campID).Scan(&steel, &uranium, &hydrogen)

	switch item {
	case "tank":
		if steel < 100.0 || hydrogen < 50.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 100 Steel, 50 Hydrogen."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 100.0, hydrogen = hydrogen - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET fusion_tanks = fusion_tanks + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🚜 Fusion Tank crafted successfully!"})

	case "shield":
		if steel < 200.0 || uranium < 20.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 200 Steel, 20 Uranium."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET steel = steel - 200.0, uranium = uranium - 20.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nuclear_shields = nuclear_shields + 1 WHERE encampment_id = $1", campID)
		_ = c.Respond(&gopkg.CallbackResponse{Text: "🛡️ Nuclear Shielding installed successfully!"})
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing craft transaction: %v", err)
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	return h.HandleFactoryPanel(c)
}