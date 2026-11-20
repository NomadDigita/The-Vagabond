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
	"gopkg.in/telebot.v3"
)

type FactoryHandler struct {
	DB *sql.DB
}

func NewFactoryHandler(db *sql.DB) *FactoryHandler {
	return &FactoryHandler{DB: db}
}

func renderOrEdit(c telebot.Context, text string, markup *telebot.ReplyMarkup) error {
	if c.Callback() != nil {
		return c.Edit(text, markup)
	}
	return c.Send(text, markup)
}

func renderOrEditHTML(c telebot.Context, text string, markup *telebot.ReplyMarkup) error {
	if c.Callback() != nil {
		return c.Edit(text, telebot.ModeHTML, markup)
	}
	return c.Send(text, telebot.ModeHTML, markup)
}

func (h *FactoryHandler) HandleFactoryPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID, campName string
	err := h.DB.QueryRowContext(ctx, "SELECT id, name FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campName)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}
	if err != nil {
		log.Printf("Failed to load workshop outpost for user %d: %v", sender.ID, err)
		return c.Send("⚠️ System connection error reading your outpost.", keyboards.MainNavigation())
	}

	panelText := "🏭 " + htmlBold("HEAVY WORKSHOP SECTOR SYSTEMS") + "\n" + divider + "\n" +
		"📛 Outpost Name: " + htmlCode(campName) + "\n\n" +
		htmlItalic("Select options on your bottom menu deck to recruit troops or craft logistics vehicles.")

	return c.Send(panelText, telebot.ModeHTML, keyboards.WorkshopNavigation())
}

func (h *FactoryHandler) HandleRecruitPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

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

	selector := &telebot.ReplyMarkup{}

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

func (h *FactoryHandler) HandleVehiclesPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

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

	selector := &telebot.ReplyMarkup{}
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

type craftSpec struct {
	dbColumn    string
	cost        map[string]float64
	label       string
	verb        string
	maxPerOrder int
	isVehicle   bool
}

const defaultMaxPerOrder = 20

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

func craftSpecFor(item string) (craftSpec, bool) {
	if item == "cargo_jet" {
		item = "jet"
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

func (h *FactoryHandler) HandleCraftCallback(c telebot.Context) error {
	item := strings.ToLower(strings.TrimSpace(c.Args()[0]))
	spec, ok := craftSpecFor(item)
	if !ok {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown item."})
	}

	costLine := formatCraftCost(spec.cost)
	panelText := fmt.Sprintf("⚒️ How many %s(s) to produce?\n\nCost per unit: %s", spec.label, costLine)

	selector := &telebot.ReplyMarkup{}
	var rows []telebot.Row
	var row []telebot.Btn
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

func (h *FactoryHandler) HandleCraftQuantityCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	args := c.Args()
	if len(args) < 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid crafting order."})
	}
	item := strings.ToLower(strings.TrimSpace(args[0]))
	var quantity int
	fmt.Sscanf(args[1], "%d", &quantity)

	if quantity == 0 {
		_ = c.Respond(&telebot.CallbackResponse{Text: "↩️ Order cancelled."})
		return h.HandleRecruitPanel(c)
	}

	spec, ok := craftSpecFor(item)
	if !ok {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown item."})
	}
	if quantity < 1 || quantity > spec.maxPerOrder {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Quantity must be between 1 and %d for this item.", spec.maxPerOrder)})
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Assembly failed."})
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
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("❌ Hangar Full: %d/%d capacity used, only room for %d more. Upgrade your Hangar or /deconstruct unused units.", totalUnits, maxCapacity, room)})
	}

	if item == "deathstar" {
		var currentDS, campLvl int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(deathstars, 0) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&currentDS)
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 1) FROM encampments WHERE id = $1", campID).Scan(&campLvl)
		maxDS := content.MaxDoomsdayRigs(campLvl)
		if currentDS+quantity > maxDS {
			return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("❌ Limit Reached: Outpost Level %d can command at most %d Doomsday Rig(s) total (you have %d). Level up to raise the cap.", campLvl, maxDS, currentDS)})
		}
	}

	totalCost := make(map[string]float64, len(spec.cost))
	for res, perUnit := range spec.cost {
		need := perUnit * float64(quantity)
		totalCost[res] = need
		if held[res] < need {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %s for %dx %s.", formatCraftCost(map[string]float64{res: need}), quantity, spec.label)})
		}
	}

	for res, amt := range totalCost {
		_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = %s - $1 WHERE encampment_id = $2", res, res), amt, campID)
	}
	_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE workshop_inventory SET %s = %s + $1 WHERE encampment_id = $2", spec.dbColumn, spec.dbColumn), quantity, campID)

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
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("✅ %dx %s %s successfully!", quantity, spec.label, spec.verb)})

	if spec.isVehicle {
		return h.HandleVehiclesPanel(c)
	}
	return h.HandleRecruitPanel(c)
}
