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
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
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

// renderOrEditHTML is identical to renderOrEdit but renders with
// Telegram HTML parse mode - only use at a call site where `text` has
// already been built with the htmlBold/htmlItalic/htmlCode/htmlEscape
// helpers in render.go.
func renderOrEditHTML(c gopkg.Context, text string, markup *gopkg.ReplyMarkup) error {
	if c.Callback() != nil {
		return c.Edit(text, gopkg.ModeHTML, markup)
	}
	return c.Send(text, gopkg.ModeHTML, markup)
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

	panelText := "🏭 " + htmlBold("HEAVY WORKSHOP SECTOR SYSTEMS") + "\n" + divider + "\n" +
		"📛 Outpost Name: " + htmlCode("Military Engineering Hangar") + "\n\n" +
		htmlItalic("Select options on your bottom menu deck to recruit troops or craft logistics vehicles.")

	return c.Send(panelText, gopkg.ModeHTML, keyboards.WorkshopNavigation())
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
	var liberators, wraiths, observers, guardians, piercingMissiles int
	queryInv := `SELECT soldiers, drones, mechs, nukes, COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(scouts,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0), COALESCE(liberators,0), COALESCE(wraiths,0), COALESCE(observers,0), COALESCE(guardians,0), COALESCE(piercing_missiles,0) FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&soldiers, &drones, &mechs, &nukes, &destroyers, &bombers, &scouts, &battlecruisers, &deathstars, &liberators, &wraiths, &observers, &guardians, &piercingMissiles)

	scoutUnit, _ := content.FindUnit("scout")
	bcUnit, _ := content.FindUnit("battlecruiser")
	dsUnit, _ := content.FindUnit("deathstar")
	libUnit, _ := content.FindUnit("liberator")
	wrUnit, _ := content.FindUnit("wraith")
	obsUnit, _ := content.FindUnit("observer")
	gdUnit, _ := content.FindUnit("guardian")
	pmUnit, _ := content.FindUnit("piercing_missile")

	var campLvl int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(level,1) FROM encampments WHERE id = $1", campID).Scan(&campLvl)
	maxDS := content.MaxDoomsdayRigs(campLvl)

	panelText := fmt.Sprintf(
		"🏭━━━━━━━━━━━━━━━━━━━━━━🏭\n"+
			"🪖⚙️ BARRACKS RECRUITMENT FORGE ⚙️🪖\n"+
			"🏭━━━━━━━━━━━━━━━━━━━━━━🏭\n\n"+
			"📦 CURRENT GARRISON:\n"+
			"🪖 Soldiers: %d  |  🛰️ Tactical Drones: %d\n"+
			"🤖 Mechs: %d  |  ☢️ Nuclear Weapons: %d\n"+
			"💥 Destroyers: %d  |  🛩️ Bombers: %d\n"+
			"🛵 Scout Walkers: %d  |  🚢👑 Battlecruisers: %d\n"+
			"🌑💀 Doomsday Rigs: %d / %d (cap rises with Outpost level)\n"+
			"🦅 Liberators: %d  |  👻 Wraiths: %d\n"+
			"👁️ Observers: %d  |  🛡️🤖 Guardians: %d\n"+
			"🎯☢️ Piercing Missiles: %d\n\n"+
			"⚒️ MANUFACTURING BLUEPRINTS ⚒️\n"+
			"🪖 [Soldier] ➜ 💰50 Rations, 🔩10 Metal ➜ ⚔️ +10 Offense\n"+
			"🛰️ [Tactical Drone] ➜ 🔩100 Metal, 🔮10 Crystal ➜ 🕵️ Spy Satellite / 🚨 Interceptor\n"+
			"🤖 [Colossus Mech] ➜ 🔩1000 Metal, 🔮70 Crystal ➜ ⚔️ +350 Offense\n"+
			"☢️ [Nuclear Device] ➜ 🔩2500 Metal, 🔮510 Crystal ➜ 💥 +1500 Detonation\n"+
			"💥 [Destroyer] ➜ 🔩800 Metal, 🔮55 Crystal ➜ 🎯 Hard-counters Drones/Jets\n"+
			"🛩️ [Bomber] ➜ 🔩1300 Metal, 🔮60 Crystal ➜ 🏰 Hard-counters Turrets\n"+
			"🛵 [%s] ➜ 🔩%.0f Metal ➜ %s\n"+
			"🚢👑 [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ %s\n"+
			"🌑💀 [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal, 🧠%.0f Neuro Cores ➜ %s\n"+
			"🦅 [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ %s\n"+
			"👻 [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ %s\n"+
			"👁️ [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ %s\n"+
			"🛡️🤖 [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ %s\n"+
			"🎯☢️ [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ %s\n"+
			"🏭━━━━━━━━━━━━━━━━━━━━━━🏭",
		soldiers, drones, mechs, nukes, destroyers, bombers, scouts, battlecruisers, deathstars, maxDS, liberators, wraiths, observers, guardians, piercingMissiles,
		scoutUnit.Title, scoutUnit.Cost["metal"], scoutUnit.Flavor,
		bcUnit.Title, bcUnit.Cost["metal"], bcUnit.Cost["crystal"], bcUnit.Flavor,
		dsUnit.Title, dsUnit.Cost["metal"], dsUnit.Cost["crystal"], dsUnit.Cost["neuro_cores"], dsUnit.Flavor,
		libUnit.Title, libUnit.Cost["metal"], libUnit.Cost["crystal"], libUnit.Flavor,
		wrUnit.Title, wrUnit.Cost["metal"], wrUnit.Cost["crystal"], wrUnit.Flavor,
		obsUnit.Title, obsUnit.Cost["metal"], obsUnit.Cost["crystal"], obsUnit.Flavor,
		gdUnit.Title, gdUnit.Cost["metal"], gdUnit.Cost["crystal"], gdUnit.Flavor,
		pmUnit.Title, pmUnit.Cost["metal"], pmUnit.Cost["crystal"], pmUnit.Flavor,
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
	btnCraftLiberator := selector.Data("🦅 Forge Liberator", "craft_item", "liberator")
	btnCraftWraith := selector.Data("👻 Forge Wraith", "craft_item", "wraith")
	btnCraftObserver := selector.Data("👁️ Build Observer", "craft_item", "observer")
	btnCraftGuardian := selector.Data("🛡️🤖 Forge Guardian", "craft_item", "guardian")
	btnCraftPiercing := selector.Data("🎯☢️ Forge Piercing Missile", "craft_item", "piercing_missile")

	selector.Inline(
		selector.Row(btnCraftSoldier, btnCraftDrone),
		selector.Row(btnCraftMech, btnCraftNuke),
		selector.Row(btnCraftDestroyer, btnCraftBomber),
		selector.Row(btnCraftScout, btnCraftBC),
		selector.Row(btnCraftDS),
		selector.Row(btnCraftLiberator, btnCraftWraith),
		selector.Row(btnCraftObserver, btnCraftGuardian),
		selector.Row(btnCraftPiercing),
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
	var cargoMk1, cargoMk2, cargoMk3 int
	queryInv := `
		SELECT 
			COALESCE(buggies, 0), COALESCE(ships, 0), COALESCE(jets, 0), 
			COALESCE(haulers, 0), COALESCE(tankers, 0), COALESCE(rigs, 0),
			COALESCE(cargo_mk1, 0), COALESCE(cargo_mk2, 0), COALESCE(cargo_mk3, 0)
		FROM workshop_inventory 
		WHERE encampment_id = $1`

	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&buggies, &ships, &jets, &haulers, &tankers, &rigs, &cargoMk1, &cargoMk2, &cargoMk3)

	cm1Unit, _ := content.FindUnit("cargo_mk1")
	cm2Unit, _ := content.FindUnit("cargo_mk2")
	cm3Unit, _ := content.FindUnit("cargo_mk3")

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 LOGISTICS HANGAR FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 Scrap Buggies: %d | ⛵ Clipper Ships: %d | ✈️ Cargo Jets: %d\n"+
			"🚛 Resource Haulers: %d | 🛢️ Fuel Tankers: %d | 🔧 Recovery Rigs: %d\n"+
			"🚚 Cargo Mk I: %d | 🚚🚚 Cargo Mk II: %d | 🚚🚚🚚 Cargo Mk III: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🚗 [Scrap Buggy] — Cost: 100 Metal, 20 Oil (Land travel +25%% speed)\n"+
			"⛵ [Clipper Ship] — Cost: 300 Metal (Required to cross oceans)\n"+
			"✈️ [Cargo Jet] — Cost: 1000 Metal, 200 Hydrogen, 100 Oil (Reduces travel to flat 2h)\n\n"+
			"🚛 [Resource Hauler] — Cost: 500 Metal, 50 Oil (+5,000 battle loot cap)\n"+
			"🛡️ [Fuel Tanker] — Cost: 400 Metal, 100 Hydrogen (-20%% march fuel costs)\n"+
			"🛠️ [Recovery Rig] — Cost: 600 Metal, 50 Iron (-15%% mechanical casualties)\n\n"+
			"🚚 [%s] ➜ 🔩%.0f Metal ➜ Further reduces return-march loot weight penalty\n"+
			"🚚🚚 [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ Substantially reduces return-march loot weight penalty\n"+
			"🚚🚚🚚 [%s] ➜ 🔩%.0f Metal, 🔮%.0f Crystal ➜ Massively reduces return-march loot weight penalty\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		buggies, ships, jets, haulers, tankers, rigs, cargoMk1, cargoMk2, cargoMk3,
		cm1Unit.Title, cm1Unit.Cost["metal"],
		cm2Unit.Title, cm2Unit.Cost["metal"], cm2Unit.Cost["crystal"],
		cm3Unit.Title, cm3Unit.Cost["metal"], cm3Unit.Cost["crystal"],
	)

	selector := &gopkg.ReplyMarkup{}
	btnCraftBuggy := selector.Data("🚗 Craft Buggy", "craft_item", "buggy")
	btnCraftShip := selector.Data("⛵ Craft Ship", "craft_item", "ship")
	btnCraftJet := selector.Data("✈️ Craft Jet", "craft_item", "cargo_jet")
	btnCraftHauler := selector.Data("🚛 Craft Hauler", "craft_item", "hauler")
	btnCraftTanker := selector.Data("🛡️ Craft Tanker", "craft_item", "tanker")
	btnCraftRig := selector.Data("🛠️ Craft Recovery Rig", "craft_item", "rig")
	btnCraftCargo1 := selector.Data("🚚 Craft Cargo Mk I", "craft_item", "cargo_mk1")
	btnCraftCargo2 := selector.Data("🚚🚚 Craft Cargo Mk II", "craft_item", "cargo_mk2")
	btnCraftCargo3 := selector.Data("🚚🚚🚚 Craft Cargo Mk III", "craft_item", "cargo_mk3")

	selector.Inline(
		selector.Row(btnCraftBuggy, btnCraftShip),
		selector.Row(btnCraftJet),
		selector.Row(btnCraftHauler, btnCraftTanker),
		selector.Row(btnCraftRig),
		selector.Row(btnCraftCargo1, btnCraftCargo2, btnCraftCargo3),
	)

	return renderOrEdit(c, panelText, selector)
}

// craftSpec is one buildable item's cost/target-column/labeling, used by
// the generic multi-quantity crafting flow below. Previously each item
// was a fully separate switch case in HandleCraftCallback, hand-coding
// the same cost-check/deduct/increment shape 20+ times with quantity
// hardcoded to 1 - that made "produce more than one at a time" require
// rewriting every case identically (and easy to get inconsistent across
// them). This table is the single place quantity, cost, and target
// column live now; the actual crafting logic in HandleCraftQuantityCallback
// is written once and applies to every item.
type craftSpec struct {
	dbColumn    string             // workshop_inventory column to increment
	cost        map[string]float64 // resources table column -> cost PER UNIT
	label       string             // display name, singular (a trailing "s" is added for plural messaging)
	verb        string             // past-tense verb for the success message, e.g. "recruited"
	maxPerOrder int                // hard ceiling on quantity for a single craft order
	isVehicle   bool               // true => after crafting, return to the Vehicles panel instead of Recruit
}

// defaultMaxPerOrder is the "up to 20x at once" ceiling for ordinary
// units. Units that deliberately get a lower ceiling say so explicitly
// in their own craftSpec entry below, with a comment explaining why -
// never silently omit maxPerOrder and fall back to this for something
// that shouldn't get it.
const defaultMaxPerOrder = 20

// hardcodedCraftSpecs covers the units that predate the content package's
// canonical registry (internal/game/content/units.go's doc comment: "the
// SpaceHunt-revival content spine... every new unit added going forward
// should be defined here first" - these are the ones from before that
// convention started, not yet migrated). Costs here are unchanged from
// the original single-unit switch cases - this refactor changes HOW
// quantity is handled, not what anything costs.
var hardcodedCraftSpecs = map[string]craftSpec{
	"soldier": {
		dbColumn: "soldiers", cost: map[string]float64{"rations": 50.0, "metal": 10.0},
		label: "Soldier", verb: "recruited", maxPerOrder: defaultMaxPerOrder,
	},
	"drone": {
		dbColumn: "drones", cost: map[string]float64{"metal": 100.0, "crystal": 10.0},
		label: "Tactical Drone", verb: "assembled", maxPerOrder: defaultMaxPerOrder,
	},
	"mech": {
		dbColumn: "mechs", cost: map[string]float64{"metal": 1000.0, "crystal": 70.0},
		label: "Colossus Mech", verb: "forged", maxPerOrder: defaultMaxPerOrder,
	},
	"nuke": {
		dbColumn: "nukes", cost: map[string]float64{"metal": 2500.0, "crystal": 510.0},
		label: "Nuclear Device", verb: "assembled",
		// Deliberately excluded from the 20x default: a single Nuclear
		// Device already carries +1500 Detonation, by far the highest
		// power-per-unit in the game. Twenty in one order would let a
		// single craft action reshape the server's balance far more than
		// any other bulk order - capped low enough that mass-producing
		// them still takes real, deliberate repetition.
		maxPerOrder: 3,
	},
	"destroyer": {
		dbColumn: "destroyers", cost: map[string]float64{"metal": 800.0, "crystal": 55.0},
		label: "Destroyer", verb: "forged", maxPerOrder: defaultMaxPerOrder,
	},
	"bomber": {
		dbColumn: "bombers", cost: map[string]float64{"metal": 1300.0, "crystal": 60.0},
		label: "Bomber", verb: "assembled", maxPerOrder: defaultMaxPerOrder,
	},
	"buggy": {
		dbColumn: "buggies", cost: map[string]float64{"metal": 120.0},
		label: "Scrap Buggy", verb: "crafted", maxPerOrder: defaultMaxPerOrder, isVehicle: true,
	},
	"ship": {
		dbColumn: "ships", cost: map[string]float64{"metal": 300.0},
		label: "Clipper Ship", verb: "constructed", maxPerOrder: defaultMaxPerOrder, isVehicle: true,
	},
	"jet": {
		dbColumn: "jets", cost: map[string]float64{"metal": 1100.0, "hydrogen": 200.0},
		label: "Cargo Jet", verb: "constructed", maxPerOrder: defaultMaxPerOrder, isVehicle: true,
	},
	"hauler": {
		dbColumn: "haulers", cost: map[string]float64{"metal": 550.0},
		label: "Resource Hauler", verb: "constructed", maxPerOrder: defaultMaxPerOrder, isVehicle: true,
	},
	"tanker": {
		dbColumn: "tankers", cost: map[string]float64{"metal": 400.0, "hydrogen": 100.0},
		label: "Fuel Tanker", verb: "constructed", maxPerOrder: defaultMaxPerOrder, isVehicle: true,
	},
	"rig": {
		dbColumn: "rigs", cost: map[string]float64{"metal": 650.0},
		label: "Recovery Rig", verb: "constructed", maxPerOrder: defaultMaxPerOrder, isVehicle: true,
	},
}

// craftSpecFor resolves an item key to its craftSpec, checking the
// pre-content-package units above first, then falling back to
// content.FindUnit for everything registered there (scout,
// battlecruiser, deathstar, liberator, wraith, observer, guardian,
// piercing_missile, cargo_mk1/2/3) - reused live rather than copied, so
// this table can never drift out of sync with the canonical registry.
func craftSpecFor(item string) (craftSpec, bool) {
	if item == "cargo_jet" {
		item = "jet" // button callback data uses "cargo_jet"; same item, one spec entry
	}
	if spec, ok := hardcodedCraftSpecs[item]; ok {
		return spec, true
	}
	unit, ok := content.FindUnit(item)
	if !ok {
		return craftSpec{}, false
	}
	maxPerOrder := defaultMaxPerOrder
	if item == "deathstar" {
		// Doomsday Rigs already have their own total-fleet cap tied to
		// Outpost level (content.MaxDoomsdayRigs, checked separately in
		// HandleCraftQuantityCallback since it's a cap on units OWNED,
		// not units per order). Offering a "20x" button that will almost
		// always fail against a cap that's usually 1-5 total would just
		// be a confusing, near-always-rejected option - keep the
		// per-order ceiling low and let the real cap be the binding one.
		maxPerOrder = 3
	}
	return craftSpec{
		dbColumn:    unit.Column,
		cost:        unit.Cost,
		label:       unit.Title,
		verb:        "forged",
		maxPerOrder: maxPerOrder,
		isVehicle:   item == "cargo_mk1" || item == "cargo_mk2" || item == "cargo_mk3",
	}, true
}

// quantityOptions returns the quantity buttons to offer for a given
// spec's maxPerOrder - always includes 1x, then scales up to (and
// including) maxPerOrder without offering an option above it.
func quantityOptions(maxPerOrder int) []int {
	all := []int{1, 5, 10, 20}
	var out []int
	for _, q := range all {
		if q <= maxPerOrder {
			out = append(out, q)
		}
	}
	if len(out) == 0 || out[len(out)-1] != maxPerOrder {
		out = append(out, maxPerOrder)
	}
	return out
}

// HandleCraftCallback ("\fcraft_item") is the first tap on a "Recruit X"/
// "Forge X" button - it no longer crafts immediately, it opens the
// quantity picker (HandleCraftQuantityCallback does the actual crafting).
// This two-step shape mirrors how every other consequential action in
// this game already confirms before committing resources (e.g. the road-
// encounter Attack/Continue buttons) rather than firing on the very
// first tap.
func (h *FactoryHandler) HandleCraftCallback(c gopkg.Context) error {
	item := strings.ToLower(strings.TrimSpace(c.Args()[0]))
	spec, ok := craftSpecFor(item)
	if !ok {
		return c.Respond(&gopkg.CallbackResponse{Text: "❌ Unknown item."})
	}

	costLine := formatCraftCost(spec.cost)
	panelText := fmt.Sprintf("⚒️ How many %s(s) to produce?\n\nCost per unit: %s", spec.label, costLine)

	selector := &gopkg.ReplyMarkup{}
	var rows []gopkg.Row
	var row []gopkg.Btn
	for _, qty := range quantityOptions(spec.maxPerOrder) {
		btn := selector.Data(fmt.Sprintf("%dx", qty), "craft_qty", item, fmt.Sprintf("%d", qty))
		row = append(row, btn)
		if len(row) == 2 {
			rows = append(rows, selector.Row(row...))
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, selector.Row(row...))
	}
	btnCancel := selector.Data("↩️ Cancel", "craft_qty", item, "0")
	rows = append(rows, selector.Row(btnCancel))
	selector.Inline(rows...)

	return renderOrEdit(c, panelText, selector)
}

// formatCraftCost renders a per-unit cost map as a single readable line,
// in a stable order (resources package convention elsewhere in this
// codebase always lists Rations/Metal/Crystal/Hydrogen/Electricity/
// Dollars/Neuro Cores in that order - matched here so cost lines read
// consistently with every other panel in the game).
func formatCraftCost(cost map[string]float64) string {
	order := []struct{ key, label, emoji string }{
		{"rations", "Rations", "🌾"},
		{"metal", "Metal", "🔩"},
		{"crystal", "Crystal", "🔮"},
		{"hydrogen", "Hydrogen", "💧"},
		{"electricity", "Electricity", "⚡"},
		{"neuro_cores", "Neuro Cores", "🧠"},
		{"dollars", "Dollars", "💵"},
	}
	var parts []string
	for _, o := range order {
		if amt, ok := cost[o.key]; ok && amt > 0 {
			parts = append(parts, fmt.Sprintf("%s%.0f %s", o.emoji, amt, o.label))
		}
	}
	if len(parts) == 0 {
		return "free"
	}
	return strings.Join(parts, ", ")
}

// HandleCraftQuantityCallback ("\fcraft_qty") performs the actual craft
// for the quantity chosen on HandleCraftCallback's picker. Every cost
// deduction and inventory increment happens ONCE per order, scaled by
// quantity (a single `soldiers = soldiers + $1` with quantity as the
// parameter) rather than looping the original single-unit logic N times -
// both simpler and avoids N separate round-trips for what the player
// experiences as one action.
func (h *FactoryHandler) HandleCraftQuantityCallback(c gopkg.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	args := c.Args()
	if len(args) < 2 {
		return c.Respond(&gopkg.CallbackResponse{Text: "❌ Invalid crafting order."})
	}
	item := strings.ToLower(strings.TrimSpace(args[0]))
	var quantity int
	fmt.Sscanf(args[1], "%d", &quantity)

	if quantity == 0 {
		_ = c.Respond(&gopkg.CallbackResponse{Text: "↩️ Order cancelled."})
		return h.HandleRecruitPanel(c)
	}

	spec, ok := craftSpecFor(item)
	if !ok {
		return c.Respond(&gopkg.CallbackResponse{Text: "❌ Unknown item."})
	}
	if quantity < 1 || quantity > spec.maxPerOrder {
		return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Quantity must be between 1 and %d for this item.", spec.maxPerOrder)})
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Assembly failed."})
	}
	defer tx.Rollback()

	queryUpsert := `
		INSERT INTO workshop_inventory (encampment_id) 
		VALUES ($1) 
		ON CONFLICT (encampment_id) DO UPDATE SET encampment_id = EXCLUDED.encampment_id`
	_, _ = tx.ExecContext(ctx, queryUpsert, campID)

	var resourceLevels struct{ rations, metal, crystal, hydrogen, neuroCores, dollars, electricity float64 }
	_ = tx.QueryRowContext(ctx, `
		SELECT rations, metal, crystal, hydrogen, COALESCE(neuro_cores,0), COALESCE(dollars,0), COALESCE(electricity,0)
		FROM resources WHERE encampment_id = $1 FOR UPDATE`, campID).
		Scan(&resourceLevels.rations, &resourceLevels.metal, &resourceLevels.crystal, &resourceLevels.hydrogen,
			&resourceLevels.neuroCores, &resourceLevels.dollars, &resourceLevels.electricity)
	held := map[string]float64{
		"rations": resourceLevels.rations, "metal": resourceLevels.metal, "crystal": resourceLevels.crystal,
		"hydrogen": resourceLevels.hydrogen, "neuro_cores": resourceLevels.neuroCores,
		"dollars": resourceLevels.dollars, "electricity": resourceLevels.electricity,
	}

	// Hangar capacity check, scaled by quantity - see the original
	// single-unit version of this check (still the same cap formula,
	// still the same set of columns summed) for why the cap exists.
	// 10x pass: 500 + hangarLvl*200 (was 50 + hangarLvl*20) - keep in
	// sync with the identical formula in engine/agent/agent.go's
	// auto-recruit path.
	var hangarLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'hangar'", campID).Scan(&hangarLvl)
	maxCapacity := 500 + hangarLvl*200
	var totalUnits int
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(soldiers,0)+COALESCE(drones,0)+COALESCE(mechs,0)+COALESCE(nukes,0)+COALESCE(buggies,0)+COALESCE(ships,0)+COALESCE(jets,0)+
		       COALESCE(haulers,0)+COALESCE(tankers,0)+COALESCE(rigs,0)+COALESCE(destroyers,0)+COALESCE(bombers,0)+COALESCE(scouts,0)+
		       COALESCE(battlecruisers,0)+COALESCE(deathstars,0)+COALESCE(liberators,0)+COALESCE(wraiths,0)+COALESCE(observers,0)+
		       COALESCE(guardians,0)+COALESCE(piercing_missiles,0)+COALESCE(cargo_mk1,0)+COALESCE(cargo_mk2,0)+COALESCE(cargo_mk3,0)
		FROM workshop_inventory WHERE encampment_id = $1`, campID).Scan(&totalUnits)
	if totalUnits+quantity > maxCapacity {
		room := maxCapacity - totalUnits
		if room < 0 {
			room = 0
		}
		return c.Respond(&gopkg.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("❌ Hangar Full: %d/%d capacity used, only room for %d more. Upgrade your Hangar or /deconstruct unused units.", totalUnits, maxCapacity, room)})
	}

	// Doomsday Rig's total-fleet cap (tied to Outpost level, not
	// per-order) - see craftSpecFor's comment on why this stays a
	// separate check rather than folding into maxPerOrder.
	if item == "deathstar" {
		var currentDS, campLvl int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(deathstars, 0) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&currentDS)
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 1) FROM encampments WHERE id = $1", campID).Scan(&campLvl)
		maxDS := content.MaxDoomsdayRigs(campLvl)
		if currentDS+quantity > maxDS {
			return c.Respond(&gopkg.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("❌ Limit Reached: Outpost Level %d can command at most %d Doomsday Rig(s) total (you have %d). Level up to raise the cap.", campLvl, maxDS, currentDS)})
		}
	}

	totalCost := make(map[string]float64, len(spec.cost))
	for res, perUnit := range spec.cost {
		need := perUnit * float64(quantity)
		totalCost[res] = need
		if held[res] < need {
			return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %s for %dx %s.", formatCraftCost(map[string]float64{res: need}), quantity, spec.label)})
		}
	}

	for res, amt := range totalCost {
		_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = %s - $1 WHERE encampment_id = $2", res, res), amt, campID)
	}
	_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE workshop_inventory SET %s = %s + $1 WHERE encampment_id = $2", spec.dbColumn, spec.dbColumn), quantity, campID)

	// Engineering Bay refund - same flat-per-craft-ACTION amount as
	// before (not multiplied by quantity): a 20-unit order is still one
	// trip through the assembly line, not twenty separate ones.
	var engineeringBayLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'engineering_bay'", campID).Scan(&engineeringBayLvl)
	if engineeringBayLvl > 0 {
		var curMetal, curCrystal float64
		_ = tx.QueryRowContext(ctx, "SELECT metal, crystal FROM resources WHERE encampment_id = $1", campID).Scan(&curMetal, &curCrystal)
		storageCap := storagecap.CapFor(ctx, tx, campID)
		newMetal, _ := storagecap.Clamp(curMetal, float64(engineeringBayLvl)*5.0, storageCap)
		newCrystal, _ := storagecap.Clamp(curCrystal, float64(engineeringBayLvl)*1.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = $1, crystal = $2 WHERE encampment_id = $3", newMetal, newCrystal, campID)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing craft transaction: %v", err)
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	_ = c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("✅ %dx %s %s successfully!", quantity, spec.label, spec.verb)})

	if spec.isVehicle {
		return h.HandleVehiclesPanel(c)
	}
	return h.HandleRecruitPanel(c)
}
