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
		"Outpost Name: Military Engineering Hangar\n\n" +
		"Select options on your bottom menu deck to recruit troops or craft logistics vehicles."

	return c.Send(panelText, keyboards.WorkshopNavigation())
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
	queryInv := `SELECT soldiers, drones, mechs, nukes, COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(scouts,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0) FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&soldiers, &drones, &mechs, &nukes, &destroyers, &bombers, &scouts, &battlecruisers, &deathstars)

	scoutUnit, _ := content.FindUnit("scout")
	bcUnit, _ := content.FindUnit("battlecruiser")
	dsUnit, _ := content.FindUnit("deathstar")

	panelText := fmt.Sprintf(
		"🏭━━━━━━━━━━━━━━━━━━━━━━🏭\n"+
			"🪖⚙️ BARRACKS RECRUITMENT FORGE ⚙️🪖\n"+
			"🏭━━━━━━━━━━━━━━━━━━━━━━🏭\n\n"+
			"📦 CURRENT GARRISON:\n"+
			"🪖 Soldiers: %d  |  🛰️ Tactical Drones: %d\n"+
			"🤖 Mechs: %d  |  ☢️ Nuclear Weapons: %d\n"+
			"💥 Destroyers: %d  |  🛩️ Bombers: %d\n"+
			"🛵 Scout Walkers: %d  |  🚢👑 Battlecruisers: %d\n"+
			"🌑💀 Doomsday Rigs: %d\n\n"+
			"⚒️ MANUFACTURING BLUEPRINTS ⚒️\n"+
			"🪖 [Soldier] ➜ 💰50 Rations, 🔩10 Metal ➜ ⚔️ +10 Offense\n"+
			"🛰️ [Tactical Drone] ➜ 🔩100 Metal, 💎10 Crystal ➜ 🕵️ Spy Satellite / 🚨 Interceptor\n"+
			"🤖 [Colossus Mech] ➜ 🔩1000 Metal, 💎70 Crystal ➜ ⚔️ +350 Offense\n"+
			"☢️ [Nuclear Device] ➜ 🔩2500 Metal, 💎510 Crystal ➜ 💥 +1500 Detonation\n"+
			"💥 [Destroyer] ➜ 🔩800 Metal, 💎55 Crystal ➜ 🎯 Hard-counters Drones/Jets\n"+
			"🛩️ [Bomber] ➜ 🔩1300 Metal, 💎60 Crystal ➜ 🏰 Hard-counters Turrets\n"+
			"🛵 [%s] ➜ 🔩%.0f Metal ➜ %s\n"+
			"🚢👑 [%s] ➜ 🔩%.0f Metal, 💎%.0f Crystal ➜ %s\n"+
			"🌑💀 [%s] ➜ 🔩%.0f Metal, 💎%.0f Crystal, 🧠%.0f Neuro Cores ➜ %s\n"+
			"🏭━━━━━━━━━━━━━━━━━━━━━━🏭",
		soldiers, drones, mechs, nukes, destroyers, bombers, scouts, battlecruisers, deathstars,
		scoutUnit.Title, scoutUnit.Cost["metal"], scoutUnit.Flavor,
		bcUnit.Title, bcUnit.Cost["metal"], bcUnit.Cost["crystal"], bcUnit.Flavor,
		dsUnit.Title, dsUnit.Cost["metal"], dsUnit.Cost["crystal"], dsUnit.Cost["neuro_cores"], dsUnit.Flavor,
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

	selector.Inline(
		selector.Row(btnCraftSoldier, btnCraftDrone),
		selector.Row(btnCraftMech, btnCraftNuke),
		selector.Row(btnCraftDestroyer, btnCraftBomber),
		selector.Row(btnCraftScout, btnCraftBC),
		selector.Row(btnCraftDS),
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
	queryInv := `
		SELECT 
			COALESCE(buggies, 0), COALESCE(ships, 0), COALESCE(jets, 0), 
			COALESCE(haulers, 0), COALESCE(tankers, 0), COALESCE(rigs, 0) 
		FROM workshop_inventory 
		WHERE encampment_id = $1`
	
	_ = h.DB.QueryRowContext(ctx, queryInv, campID).Scan(&buggies, &ships, &jets, &haulers, &tankers, &rigs)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 LOGISTICS HANGAR FORGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚗 Scrap Buggies: %d | ⛵ Clipper Ships: %d | ✈️ Cargo Jets: %d\n"+
			"🚛 Resource Haulers: %d | 🛢️ Fuel Tankers: %d | 🔧 Recovery Rigs: %d\n\n"+
			"MANUFACTURING BLUEPRINTS:\n"+
			"🚗 [Scrap Buggy] — Cost: 100 Metal, 20 Oil (Land travel +25%% speed)\n"+
			"⛵ [Clipper Ship] — Cost: 300 Metal (Required to cross oceans)\n"+
			"✈️ [Cargo Jet] — Cost: 1000 Metal, 200 Hydrogen, 100 Oil (Reduces travel to flat 2h)\n\n"+
			"🚛 [Resource Hauler] — Cost: 500 Metal, 50 Oil (+5,000 battle loot cap)\n"+
			"🛡️ [Fuel Tanker] — Cost: 400 Metal, 100 Hydrogen (-20%% march fuel costs)\n"+
			"🛠️ [Recovery Rig] — Cost: 600 Metal, 50 Iron (-15%% mechanical casualties)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		buggies, ships, jets, haulers, tankers, rigs,
	)

	selector := &gopkg.ReplyMarkup{}
	btnCraftBuggy := selector.Data("🚗 Craft Buggy", "craft_item", "buggy")
	btnCraftShip := selector.Data("⛵ Craft Ship", "craft_item", "ship")
	btnCraftJet := selector.Data("✈️ Craft Jet", "craft_item", "cargo_jet")
	btnCraftHauler := selector.Data("🚛 Craft Hauler", "craft_item", "hauler")
	btnCraftTanker := selector.Data("🛡️ Craft Tanker", "craft_item", "tanker")
	btnCraftRig := selector.Data("🛠️ Craft Recovery Rig", "craft_item", "rig")

	selector.Inline(
		selector.Row(btnCraftBuggy, btnCraftShip),
		selector.Row(btnCraftJet),
		selector.Row(btnCraftHauler, btnCraftTanker),
		selector.Row(btnCraftRig),
	)

	return renderOrEdit(c, panelText, selector)
}

func (h *FactoryHandler) HandleCraftCallback(c gopkg.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	item := strings.ToLower(strings.TrimSpace(c.Args()[0]))

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Assembly failed."})
	}
	defer tx.Rollback()

	// Secure Hangar Allocator: Ensure the workshop row is fully allocated inside active transaction block
	queryUpsert := `
		INSERT INTO workshop_inventory (encampment_id) 
		VALUES ($1) 
		ON CONFLICT (encampment_id) DO UPDATE SET encampment_id = EXCLUDED.encampment_id`
	_, _ = tx.ExecContext(ctx, queryUpsert, campID)

	var rations, metal, crystal, hydrogen float64
	queryRes := `SELECT rations, metal, crystal, hydrogen FROM resources WHERE encampment_id = $1 FOR UPDATE`
	_ = tx.QueryRowContext(ctx, queryRes, campID).Scan(&rations, &metal, &crystal, &hydrogen)

	// Hangar capacity check: total units (all types) can't exceed the
	// Hangar-scaled cap. Deconstructing units (see deconstruct.go) frees
	// space back up.
	var hangarLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'hangar'", campID).Scan(&hangarLvl)
	maxCapacity := 50 + hangarLvl*20

	var totalUnits int
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(soldiers,0)+COALESCE(drones,0)+COALESCE(mechs,0)+COALESCE(nukes,0)+COALESCE(buggies,0)+COALESCE(ships,0)+COALESCE(jets,0)+
		       COALESCE(haulers,0)+COALESCE(tankers,0)+COALESCE(rigs,0)+COALESCE(destroyers,0)+COALESCE(bombers,0)+COALESCE(scouts,0)+
		       COALESCE(battlecruisers,0)+COALESCE(deathstars,0)
		FROM workshop_inventory WHERE encampment_id = $1`, campID).Scan(&totalUnits)

	if totalUnits >= maxCapacity {
		return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Hangar Full: %d/%d capacity used. Upgrade your Hangar (Infrastructure Grid) or /deconstruct unused units.", totalUnits, maxCapacity)})
	}

	var successAlert string

	switch item {
	case "soldier":
		if rations < 50.0 || metal < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 50 Rations, 10 Metal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - 50.0, metal = metal - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🪖 Soldier recruited successfully!"

	case "drone":
		if metal < 100.0 || crystal < 10.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 100 Metal, 10 Crystal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 100.0, crystal = crystal - 10.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET drones = drones + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛰️ Tactical Drone (Spy/Interceptor) assembled successfully!"

	case "mech":
		if metal < 1000.0 || crystal < 70.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 1000 Metal, 70 Crystal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 1000.0, crystal = crystal - 70.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET mechs = mechs + 1 WHERE encampment_id = $1", campID)
		successAlert = "🤖 Colossus Mech forged successfully!"

	case "nuke":
		if metal < 2500.0 || crystal < 510.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials!"})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 2500.0, crystal = crystal - 510.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nukes = nukes + 1 WHERE encampment_id = $1", campID)
		successAlert = "☢️ Nuclear Device assembled!"

	case "destroyer":
		if metal < 800.0 || crystal < 55.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 800 Metal, 55 Crystal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 800.0, crystal = crystal - 55.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET destroyers = destroyers + 1 WHERE encampment_id = $1", campID)
		successAlert = "💥 Destroyer forged successfully!"

	case "bomber":
		if metal < 1300.0 || crystal < 60.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 1300 Metal, 60 Crystal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 1300.0, crystal = crystal - 60.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET bombers = bombers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛩️ Bomber assembled successfully!"

	case "scout":
		scoutUnit, _ := content.FindUnit("scout")
		if metal < scoutUnit.Cost["metal"] {
			return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %.0f Metal.", scoutUnit.Cost["metal"])})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - $1 WHERE encampment_id = $2", scoutUnit.Cost["metal"], campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET scouts = scouts + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛵 Scout Walker rolled off the assembly line!"

	case "battlecruiser":
		bcUnit, _ := content.FindUnit("battlecruiser")
		if metal < bcUnit.Cost["metal"] || crystal < bcUnit.Cost["crystal"] {
			return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %.0f Metal, %.0f Crystal.", bcUnit.Cost["metal"], bcUnit.Cost["crystal"])})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - $1, crystal = crystal - $2 WHERE encampment_id = $3", bcUnit.Cost["metal"], bcUnit.Cost["crystal"], campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET battlecruisers = battlecruisers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🚢👑 BATTLECRUISER LAUNCHED! The pride of your fleet stands ready!"

	case "deathstar":
		var currentDS int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(deathstars, 0) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&currentDS)
		if currentDS >= 1 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Limit Reached: Only ONE Doomsday Rig can be commanded at a time."})
		}
		dsUnit, _ := content.FindUnit("deathstar")
		if metal < dsUnit.Cost["metal"] || crystal < dsUnit.Cost["crystal"] {
			return c.Respond(&gopkg.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Materials! Need %.0f Metal, %.0f Crystal, %.0f Neuro Cores.", dsUnit.Cost["metal"], dsUnit.Cost["crystal"], dsUnit.Cost["neuro_cores"])})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - $1, crystal = crystal - $2, neuro_cores = neuro_cores - $3 WHERE encampment_id = $4", dsUnit.Cost["metal"], dsUnit.Cost["crystal"], dsUnit.Cost["neuro_cores"], campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET deathstars = deathstars + 1 WHERE encampment_id = $1", campID)
		successAlert = "🌑💀👑 THE DOOMSDAY RIG IS OPERATIONAL! The Wasteland trembles at its shadow!"

	case "buggy":
		if metal < 120.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 120 Metal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 120.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET buggies = buggies + 1 WHERE encampment_id = $1", campID)
		successAlert = "🚗 Scrap Buggy crafted successfully!"

	case "ship":
		if metal < 300.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 300 Metal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 300.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET ships = ships + 1 WHERE encampment_id = $1", campID)
		successAlert = "⛵ Clipper Ship constructed!"

	case "cargo_jet", "jet":
		if metal < 1100.0 || hydrogen < 200.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials!"})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 1100.0, hydrogen = hydrogen - 200.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET jets = jets + 1 WHERE encampment_id = $1", campID)
		successAlert = "✈️ Cargo Jet constructed successfully!"

	case "hauler":
		if metal < 550.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 550 Metal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 550.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET haulers = haulers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🚛 Resource Hauler constructed successfully!"

	case "tanker":
		if metal < 400.0 || hydrogen < 100.0 {
			return c.Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 400 Metal, 100 Hydrogen."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 400.0, hydrogen = hydrogen - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET tankers = tankers + 1 WHERE encampment_id = $1", campID)
		successAlert = "🛡️ Fuel Tanker constructed!"

	case "rig":
		if metal < 650.0 {
			return gopkg.Context(c).Respond(&gopkg.CallbackResponse{Text: "❌ Insufficient Materials! Need 650 Metal."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 650.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET rigs = rigs + 1 WHERE encampment_id = $1", campID)
		successAlert = "🔧 Recovery Rig constructed!"
	}

	// Engineering Bay: reduces effective material waste on any successful
	// craft, refunding a flat amount scaled by building level rather than
	// rewriting every craft case's cost check individually.
	if successAlert != "" {
		var engineeringBayLvl int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'engineering_bay'", campID).Scan(&engineeringBayLvl)
		if engineeringBayLvl > 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal + $1, crystal = crystal + $2 WHERE encampment_id = $3", float64(engineeringBayLvl)*5.0, float64(engineeringBayLvl)*1.0, campID)
		}
	}

	// Dynamic Post-Commit Success Dispatcher: Only sends notification after database safely registers changes
	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing craft transaction: %v", err)
		return c.Respond(&gopkg.CallbackResponse{Text: "⚠️ Error writing inventory data."})
	}

	if successAlert != "" {
		_ = c.Respond(&gopkg.CallbackResponse{Text: successAlert})
	}

	if item == "buggy" || item == "ship" || item == "cargo_jet" || item == "jet" || item == "hauler" || item == "tanker" || item == "rig" {
		return h.HandleVehiclesPanel(c)
	}
	return h.HandleRecruitPanel(c)
}