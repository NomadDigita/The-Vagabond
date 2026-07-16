package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

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
	{"soldier", "🪖", "Soldier", "soldiers", map[string]float64{"rations": 20.0, "metal": 4.0}},
	{"drone", "🛰️", "Tactical Drone", "drones", map[string]float64{"metal": 40.0, "crystal": 4.0}},
	{"mech", "🤖", "Colossus Mech", "mechs", map[string]float64{"metal": 400.0, "crystal": 28.0}},
	{"nuke", "☢️", "Nuclear Device", "nukes", map[string]float64{"metal": 1000.0, "crystal": 244.0}},
	{"destroyer", "💥", "Destroyer", "destroyers", map[string]float64{"metal": 320.0, "crystal": 22.0}},
	{"bomber", "🛩️", "Bomber", "bombers", map[string]float64{"metal": 520.0, "crystal": 24.0}},
	{"scout", "🛵", "Scout Walker", "scouts", content.MustFindUnit("scout").DeconstructRefund()},
	{"battlecruiser", "🚢👑", "Battlecruiser", "battlecruisers", content.MustFindUnit("battlecruiser").DeconstructRefund()},
	{"deathstar", "🌑💀", "Doomsday Rig", "deathstars", content.MustFindUnit("deathstar").DeconstructRefund()},
	{"buggy", "🚗", "Scrap Buggy", "buggies", map[string]float64{"metal": 48.0}},
	{"ship", "⛵", "Clipper Ship", "ships", map[string]float64{"metal": 120.0}},
	{"jet", "✈️", "Cargo Jet", "jets", map[string]float64{"metal": 440.0, "hydrogen": 80.0}},
	{"hauler", "🚛", "Resource Hauler", "haulers", map[string]float64{"metal": 220.0}},
	{"tanker", "🛡️", "Fuel Tanker", "tankers", map[string]float64{"metal": 160.0, "hydrogen": 40.0}},
	{"rig", "🔧", "Recovery Rig", "rigs", map[string]float64{"metal": 260.0}},
	{"liberator", "🦅", "Liberator", "liberators", content.MustFindUnit("liberator").DeconstructRefund()},
	{"wraith", "👻", "Wraith", "wraiths", content.MustFindUnit("wraith").DeconstructRefund()},
	{"observer", "👁️", "Observer", "observers", content.MustFindUnit("observer").DeconstructRefund()},
	{"guardian", "🛡️🤖", "Guardian", "guardians", content.MustFindUnit("guardian").DeconstructRefund()},
	{"piercing_missile", "🎯☢️", "Piercing Missile", "piercing_missiles", content.MustFindUnit("piercing_missile").DeconstructRefund()},
	{"cargo_mk1", "🚚", "Cargo Ship Mk I", "cargo_mk1", content.MustFindUnit("cargo_mk1").DeconstructRefund()},
	{"cargo_mk2", "🚚🚚", "Cargo Ship Mk II", "cargo_mk2", content.MustFindUnit("cargo_mk2").DeconstructRefund()},
	{"cargo_mk3", "🚚🚚🚚", "Cargo Ship Mk III", "cargo_mk3", content.MustFindUnit("cargo_mk3").DeconstructRefund()},
}

// HandleDeconstructCommand implements "/deconstruct <n> <unit>" as a bulk
// text shortcut alongside the existing one-tap-per-unit panel button. With
// no arguments it falls back to the original panel view unchanged.
func (h *DeconstructHandler) HandleDeconstructCommand(c telebot.Context) error {
	args := c.Args()
	if len(args) < 2 {
		return h.HandleDeconstructPanel(c)
	}

	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	n, convErr := strconv.Atoi(args[0])
	if convErr != nil || n <= 0 {
		return c.Send("⚠️ Amount must be a positive whole number, e.g. /deconstruct 20 mechs")
	}

	unitArg := strings.ToLower(strings.TrimSuffix(args[1], "s"))
	var target *deconstructRefund
	for i := range deconstructTable {
		if deconstructTable[i].key == unitArg || strings.ToLower(deconstructTable[i].key) == unitArg {
			target = &deconstructTable[i]
			break
		}
	}
	// Handle known irregular plurals that a plain TrimSuffix("s") misses.
	if target == nil {
		irregular := map[string]string{"buggie": "buggy", "batterie": "battery"}
		if alt, ok := irregular[unitArg]; ok {
			for i := range deconstructTable {
				if deconstructTable[i].key == alt {
					target = &deconstructTable[i]
					break
				}
			}
		}
	}
	if target == nil {
		return c.Send("⚠️ Unrecognized unit type for deconstruction.")
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if campID == "" {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Deconstruction failed.")
	}
	defer tx.Rollback()

	var count int
	_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(%s, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", target.column), campID).Scan(&count)
	if count <= 0 {
		return c.Send(fmt.Sprintf("❌ You don't have any %s to deconstruct.", target.title))
	}

	scrapped := n
	if scrapped > count {
		scrapped = count
	}

	_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE workshop_inventory SET %s = %s - $1 WHERE encampment_id = $2", target.column, target.column), scrapped, campID)

	refundSummary := ""
	for resourceCol, amount := range target.refunds {
		total := amount * float64(scrapped)
		_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = %s + $1 WHERE encampment_id = $2", resourceCol, resourceCol), total, campID)
		refundSummary += fmt.Sprintf("+%.0f %s ", total, resourceCol)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing bulk deconstruct transaction: %v", err)
		return c.Send("⚠️ Error writing inventory data.")
	}

	note := ""
	if scrapped < n {
		note = fmt.Sprintf(" (only had %d available)", scrapped)
	}
	return c.Send(fmt.Sprintf("♻️ Scrapped %d %s%s! Recovered: %s", scrapped, target.title, note, refundSummary))
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
	var liberators, wraiths, observers, guardians, piercingMissiles, cargoMk1, cargoMk2, cargoMk3 int
	query := `SELECT COALESCE(soldiers,0), COALESCE(drones,0), COALESCE(mechs,0), COALESCE(nukes,0), 
	          COALESCE(buggies,0), COALESCE(ships,0), COALESCE(jets,0), 
	          COALESCE(haulers,0), COALESCE(tankers,0), COALESCE(rigs,0),
	          COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(scouts,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0),
	          COALESCE(liberators,0), COALESCE(wraiths,0), COALESCE(observers,0), COALESCE(guardians,0), COALESCE(piercing_missiles,0),
	          COALESCE(cargo_mk1,0), COALESCE(cargo_mk2,0), COALESCE(cargo_mk3,0)
	          FROM workshop_inventory WHERE encampment_id = $1`
	err := h.DB.QueryRowContext(ctx, query, campID).Scan(&soldiers, &drones, &mechs, &nukes, &buggies, &ships, &jets, &haulers, &tankers, &rigs, &destroyers, &bombers, &scouts, &battlecruisers, &deathstars,
		&liberators, &wraiths, &observers, &guardians, &piercingMissiles, &cargoMk1, &cargoMk2, &cargoMk3)
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
	inventory["liberators"] = liberators
	inventory["wraiths"] = wraiths
	inventory["observers"] = observers
	inventory["guardians"] = guardians
	inventory["piercing_missiles"] = piercingMissiles
	inventory["cargo_mk1"] = cargoMk1
	inventory["cargo_mk2"] = cargoMk2
	inventory["cargo_mk3"] = cargoMk3

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
