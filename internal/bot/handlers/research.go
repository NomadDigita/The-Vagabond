package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type ResearchHandler struct {
	DB *sql.DB
}

func NewResearchHandler(db *sql.DB) *ResearchHandler {
	return &ResearchHandler{DB: db}
}

// HandleResearchPanel renders the science research status
func (h *ResearchHandler) HandleResearchPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

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

	// Fetch or Initialize Research State
	var econLvl, defenseLvl, militaryLvl int
	query := `SELECT econ_tech_lvl, defense_tech_lvl, military_tech_lvl FROM research_states WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, query, campID).Scan(&econLvl, &defenseLvl, &militaryLvl)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO research_states (encampment_id) VALUES ($1)", campID)
		econLvl = 1
		defenseLvl = 1
		militaryLvl = 1
	}

	var neuro float64
	_ = h.DB.QueryRowContext(ctx, "SELECT neuro_cores FROM resources WHERE encampment_id = $1", campID).Scan(&neuro)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🧪 COGNITIVE RESEARCH WORKSTATION\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Unlock lost technology blueprints by analyzing valuable Neuro Cores.\n\n"+
			"RESEARCH RESERVES:\n"+
			"🧠 Neuro Cores Stock: %.0f cores\n\n"+
			"TECHNOLOGY UPGRADE TREES:\n"+
			"🔋 [Neuro-Efficiency Lvl %d / 5] (Cost: %d Neuro Cores)\n"+
			"   Reduces Automated Agent energy consumption by 15%% per level.\n\n"+
			"⚙️ [Scrap Overclock Lvl %d / 5] (Cost: %d Neuro Cores)\n"+
			"   Increases passive Scrap Heap mining speed by 20%% per level.\n\n"+
			"🦾 [Mech Armor Plating Lvl %d / 5] (Cost: %d Neuro Cores)\n"+
			"   Multiplies Colossus Mech offensive ratings by 25%% per level.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		neuro, econLvl, econLvl*5, defenseLvl, defenseLvl*5, militaryLvl, militaryLvl*5,
	)

	selector := &telebot.ReplyMarkup{}

	btnUpgradeEcon := selector.Data(fmt.Sprintf("🔋 Neuro-Efficiency (%d)", econLvl+1), "upgrade_tech", "econ")
	btnUpgradeDef := selector.Data(fmt.Sprintf("⚙️ Scrap Overclock (%d)", defenseLvl+1), "upgrade_tech", "defense")
	btnUpgradeMil := selector.Data(fmt.Sprintf("🦾 Mech Plating (%d)", militaryLvl+1), "upgrade_tech", "military")

	selector.Inline(
		selector.Row(btnUpgradeEcon),
		selector.Row(btnUpgradeDef),
		selector.Row(btnUpgradeMil),
	)

	return c.Send(panelText, selector)
}

// HandleUpgradeTechCallback manages spending neuro cores to level up science structures
func (h *ResearchHandler) HandleUpgradeTechCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	techType := c.Args()[0]

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Upgrades failed."})
	}
	defer tx.Rollback()

	var econLvl, defenseLvl, militaryLvl int
	_ = tx.QueryRowContext(ctx, "SELECT econ_tech_lvl, defense_tech_lvl, military_tech_lvl FROM research_states WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&econLvl, &defenseLvl, &militaryLvl)

	var neuro float64
	_ = tx.QueryRowContext(ctx, "SELECT neuro_cores FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&neuro)

	var cost int
	var currentLvl int
	var dbColumn string

	switch techType {
	case "econ":
		currentLvl = econLvl
		cost = econLvl * 5
		dbColumn = "econ_tech_lvl"
	case "defense":
		currentLvl = defenseLvl
		cost = defenseLvl * 5
		dbColumn = "defense_tech_lvl"
	case "military":
		currentLvl = militaryLvl
		cost = militaryLvl * 5
		dbColumn = "military_tech_lvl"
	}

	if currentLvl >= 5 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Max Level: This tech node is already fully researched."})
	}

	if neuro < float64(cost) {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Cores! Need %d.", cost)})
	}

	// Deduct and increment level
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET neuro_cores = neuro_cores - $1 WHERE encampment_id = $2", cost, campID)
	queryUpdate := fmt.Sprintf("UPDATE research_states SET %s = %s + 1 WHERE encampment_id = $1", dbColumn, dbColumn)
	_, _ = tx.ExecContext(ctx, queryUpdate, campID)

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing tech upgrades: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing research state."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🧪 Technology node upgraded successfully!"})
	return h.HandleResearchPanel(c)
}