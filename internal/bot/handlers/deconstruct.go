package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/content"
	"gopkg.in/telebot.v3"
)

type DeconstructHandler struct {
	DB *sql.DB
}

func NewDeconstructHandler(db *sql.DB) *DeconstructHandler {
	return &DeconstructHandler{DB: db}
}

// deconstructRefund describes what a single unit gives back when scrapped,
// at a flat 40% of its original build cost (see factory.go HandleCraftCallback).
type deconstructRefund struct {
	key     string
	emoji   string
	title   string
	column  string // workshop_inventory column
	refunds map[string]float64
}

var deconstructTable = []deconstructRefund{
	{"soldier", "🪖", "Soldier", "soldiers", map[string]float64{"rations": 20.0, "iron": 4.0}},
	{"drone", "🛰️", "Tactical Drone", "drones", map[string]float64{"iron": 40.0, "silver": 4.0}},
	{"mech", "🤖", "Colossus Mech", "mechs", map[string]float64{"steel": 400.0, "uranium": 20.0, "gold": 8.0}},
	{"nuke", "☢️", "Nuclear Device", "nukes", map[string]float64{"steel": 1000.0, "uranium": 200.0, "gold": 40.0, "diamond": 4.0}},
	{"destroyer", "💥", "Destroyer", "destroyers", map[string]float64{"steel": 320.0, "uranium": 16.0, "gold": 6.0}},
	{"bomber", "🛩️", "Bomber", "bombers", map[string]float64{"steel": 480.0, "uranium": 24.0, "oil": 40.0}},
	{"scout", "🛵", "Scout Walker", "scouts", content.MustFindUnit("scout").DeconstructRefund()},
	{"battlecruiser", "🚢👑", "Battlecruiser", "battlecruisers", content.MustFindUnit("battlecruiser").DeconstructRefund()},
	{"deathstar", "🌑💀", "Doomsday Rig", "deathstars", content.MustFindUnit("deathstar").DeconstructRefund()},
	{"buggy", "🚗", "Scrap Buggy", "buggies", map[string]float64{"steel": 40.0, "oil": 8.0}},
	{"ship", "⛵", "Clipper Ship", "ships", map[string]float64{"steel": 120.0}},
	{"jet", "✈️", "Cargo Jet", "jets", map[string]float64{"steel": 400.0, "hydrogen": 80.0, "oil": 40.0}},
	{"hauler", "🚛", "Resource Hauler", "haulers", map[string]float64{"steel": 200.0, "oil": 20.0}},
	{"tanker", "🛡️", "Fuel Tanker", "tankers", map[string]float64{"steel": 160.0, "hydrogen": 40.0}},
	{"rig", "🔧", "Recovery Rig", "rigs", map[string]float64{"steel": 240.0, "iron": 20.0}},
}

func (h *DeconstructHandler) HandleDeconstructPanel(c telebot.Context) error {
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

	inventory, err := h.fetchInventory(ctx, campID)
	if err != nil {
		return c.Send("⚠️ System connection error reading hangar inventory.")
	}

	panelText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"♻️ DECONSTRUCTION BAY\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Scrap unwanted units to recover 40% of their build materials and free up Hangar capacity.\n\n"

	selector := &telebot.ReplyMarkup{}
	var rows []telebot.Row
	anyUnits := false

	for _, d := range deconstructTable {
		count := inventory[d.column]
		if count <= 0 {
			continue
		}
		anyUnits = true
		panelText += fmt.Sprintf("%s [%s] — In Hangar: %d\n", d.emoji, d.title, count)
		btn := selector.Data(fmt.Sprintf("%s Scrap 1 %s", d.emoji, d.title), "deconstruct_item", d.key)
		rows = append(rows, selector.Row(btn))
	}

	if !anyUnits {
		panelText += "Your Hangar is empty — nothing to deconstruct yet."
	}

	panelText += "\n━━━━━━━━━━━━━━━━━━━━━━"
	selector.Inline(rows...)

	return renderOrEdit(c, panelText, selector)
}

func (h *DeconstructHandler) fetchInventory(ctx context.Context, campID string) (map[string]int, error) {
	inventory := make(map[string]int)

	var soldiers, drones, mechs, nukes, buggies, ships, jets, haulers, tankers, rigs, destroyers, bombers, scouts, battlecruisers, deathstars int
	query := `SELECT COALESCE(soldiers,0), COALESCE(drones,0), COALESCE(mechs,0), COALESCE(nukes,0), 
	          COALESCE(buggies,0), COALESCE(ships,0), COALESCE(jets,0), 
	          COALESCE(haulers,0), COALESCE(tankers,0), COALESCE(rigs,0),
	          COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(scouts,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0)
	          FROM workshop_inventory WHERE encampment_id = $1`
	err := h.DB.QueryRowContext(ctx, query, campID).Scan(&soldiers, &drones, &mechs, &nukes, &buggies, &ships, &jets, &haulers, &tankers, &rigs, &destroyers, &bombers, &scouts, &battlecruisers, &deathstars)
	if errors.Is(err, sql.ErrNoRows) {
		return inventory, nil
	} else if err != nil {
		return nil, err
	}

	inventory["soldiers"] = soldiers
	inventory["drones"] = drones
	inventory["mechs"] = mechs
	inventory["nukes"] = nukes
	inventory["buggies"] = buggies
	inventory["ships"] = ships
	inventory["jets"] = jets
	inventory["haulers"] = haulers
	inventory["tankers"] = tankers
	inventory["rigs"] = rigs
	inventory["destroyers"] = destroyers
	inventory["bombers"] = bombers
	inventory["scouts"] = scouts
	inventory["battlecruisers"] = battlecruisers
	inventory["deathstars"] = deathstars

	return inventory, nil
}

func (h *DeconstructHandler) HandleDeconstructCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	key := c.Args()[0]

	var target *deconstructRefund
	for i := range deconstructTable {
		if deconstructTable[i].key == key {
			target = &deconstructTable[i]
			break
		}
	}
	if target == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Unknown unit type."})
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Deconstruction failed."})
	}
	defer tx.Rollback()

	var count int
	queryCount := fmt.Sprintf("SELECT COALESCE(%s, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", target.column)
	_ = tx.QueryRowContext(ctx, queryCount, campID).Scan(&count)

	if count <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have any of these to deconstruct."})
	}

	queryDecrement := fmt.Sprintf("UPDATE workshop_inventory SET %s = %s - 1 WHERE encampment_id = $1", target.column, target.column)
	_, _ = tx.ExecContext(ctx, queryDecrement, campID)

	refundSummary := ""
	for resourceCol, amount := range target.refunds {
		queryRefund := fmt.Sprintf("UPDATE resources SET %s = %s + $1 WHERE encampment_id = $2", resourceCol, resourceCol)
		_, _ = tx.ExecContext(ctx, queryRefund, amount, campID)
		refundSummary += fmt.Sprintf("+%.0f %s ", amount, resourceCol)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing deconstruct transaction: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("♻️ %s scrapped! Recovered: %s", target.title, refundSummary)})
	return h.HandleDeconstructPanel(c)
}
