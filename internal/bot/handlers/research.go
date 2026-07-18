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

// MaxResearchLevel caps every tech node. Raised from the old 5-level cap to
// give the tree real long-term progression depth.
const MaxResearchLevel = 20

// researchNode describes one branch of the tree. dbColumn must match a real
// column on research_states.
type researchNode struct {
	key      string // used in callback data
	dbColumn string
	emoji    string
	title    string
	desc     string
}

var researchTree = []researchNode{
	{"econ", "econ_tech_lvl", "⚡", "Technology", "Reduces Automated Agent electricity consumption."},
	{"production", "production_tech_lvl", "⚙️", "Production", "Increases passive Scrap Heap mining speed."},
	{"integrity", "integrity_tech_lvl", "🩹", "Integrity", "Reduces casualties suffered by your units in combat."},
	{"defense", "defense_tech_lvl", "🛡️", "Shields", "Strengthens your Outpost's defensive rating against raids."},
	{"intel", "intel_tech_lvl", "🧠", "Intelligence", "Improves spy satellite intercept odds & counter-intel."},
	{"speed", "speed_tech_lvl", "🚀", "Thrusters", "Reduces march/travel time for raids and scouts."},
	{"military", "military_tech_lvl", "🦾", "Weapons", "Multiplies Mech and offensive unit combat ratings."},
}

// researchCost returns the Neuro Core cost to advance from currentLvl to currentLvl+1.
func researchCost(currentLvl int) int {
	return currentLvl * 8
}

// fetchResearchLevels reads all 7 tech levels for an encampment, initializing
// the row if it doesn't exist yet.
func (h *ResearchHandler) fetchResearchLevels(ctx context.Context, campID string) (map[string]int, error) {
	levels := make(map[string]int)

	row := h.DB.QueryRowContext(ctx, `
		SELECT econ_tech_lvl, production_tech_lvl, integrity_tech_lvl, 
		       defense_tech_lvl, intel_tech_lvl, speed_tech_lvl, military_tech_lvl 
		FROM research_states WHERE encampment_id = $1`, campID)

	var econ, production, integrity, defense, intel, speed, military int
	err := row.Scan(&econ, &production, &integrity, &defense, &intel, &speed, &military)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO research_states (encampment_id) VALUES ($1) ON CONFLICT (encampment_id) DO NOTHING", campID)
		econ, production, integrity, defense, intel, speed, military = 1, 1, 1, 1, 1, 1, 1
	} else if err != nil {
		return nil, err
	}

	levels["econ"] = econ
	levels["production"] = production
	levels["integrity"] = integrity
	levels["defense"] = defense
	levels["intel"] = intel
	levels["speed"] = speed
	levels["military"] = military

	return levels, nil
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

	levels, err := h.fetchResearchLevels(ctx, campID)
	if err != nil {
		return c.Send("⚠️ System connection error reading research database.")
	}

	var neuro float64
	_ = h.DB.QueryRowContext(ctx, "SELECT neuro_cores FROM resources WHERE encampment_id = $1", campID).Scan(&neuro)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🧪 COGNITIVE RESEARCH WORKSTATION\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Unlock lost technology blueprints by analyzing valuable Neuro Cores.\n\n"+
			"🧠 Neuro Cores Stock: %.0f cores\n\n"+
			"TECHNOLOGY UPGRADE TREES:\n",
		neuro,
	)

	selector := &telebot.ReplyMarkup{}
	var rows []telebot.Row

	for _, node := range researchTree {
		lvl := levels[node.key]
		if lvl >= MaxResearchLevel {
			panelText += fmt.Sprintf("%s [%s Lvl %d/%d] MAX\n   %s\n\n", node.emoji, node.title, lvl, MaxResearchLevel, node.desc)
			continue
		}
		cost := researchCost(lvl)
		panelText += fmt.Sprintf("%s [%s Lvl %d/%d] (Cost: %d Neuro Cores)\n   %s\n\n", node.emoji, node.title, lvl, MaxResearchLevel, cost, node.desc)
		btn := selector.Data(fmt.Sprintf("%s %s (Lvl %d)", node.emoji, node.title, lvl+1), "upgrade_tech", node.key)
		rows = append(rows, selector.Row(btn))
	}

	panelText += "━━━━━━━━━━━━━━━━━━━━━━"
	selector.Inline(rows...)

	return sendPanelWithNav(c, navCaptionCamp, keyboards.CampNavigation(), panelText, selector)
}

// HandleUpgradeTechCallback manages spending neuro cores to level up research nodes
func (h *ResearchHandler) HandleUpgradeTechCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	techKey := c.Args()[0]

	var targetNode *researchNode
	for i := range researchTree {
		if researchTree[i].key == techKey {
			targetNode = &researchTree[i]
			break
		}
	}
	if targetNode == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Unknown research node."})
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Upgrades failed."})
	}
	defer tx.Rollback()

	_, _ = tx.ExecContext(ctx, "INSERT INTO research_states (encampment_id) VALUES ($1) ON CONFLICT (encampment_id) DO NOTHING", campID)

	var currentLvl int
	queryLvl := fmt.Sprintf("SELECT %s FROM research_states WHERE encampment_id = $1 FOR UPDATE", targetNode.dbColumn)
	_ = tx.QueryRowContext(ctx, queryLvl, campID).Scan(&currentLvl)

	if currentLvl >= MaxResearchLevel {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Max Level: This tech node is already fully researched."})
	}

	cost := researchCost(currentLvl)

	var neuro float64
	_ = tx.QueryRowContext(ctx, "SELECT neuro_cores FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&neuro)

	if neuro < float64(cost) {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Cores! Need %d.", cost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET neuro_cores = neuro_cores - $1 WHERE encampment_id = $2", cost, campID)
	queryUpdate := fmt.Sprintf("UPDATE research_states SET %s = %s + 1 WHERE encampment_id = $1", targetNode.dbColumn, targetNode.dbColumn)
	_, _ = tx.ExecContext(ctx, queryUpdate, campID)

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing tech upgrades: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing research state."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🧪 %s upgraded to Level %d!", targetNode.title, currentLvl+1)})
	return h.HandleResearchPanel(c)
}
