package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/models"
	"gopkg.in/telebot.v3"
)

type OnboardingHandler struct {
	DB *sql.DB
}

func NewOnboardingHandler(db *sql.DB) *OnboardingHandler {
	return &OnboardingHandler{DB: db}
}

func (h *OnboardingHandler) HandleStart(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("sender details missing from context")
	}

	ctx := context.Background()

	var user models.User
	queryUser := `SELECT telegram_id, username, first_name, state, COALESCE(faction, '') FROM users WHERE telegram_id = $1`
	err := h.DB.QueryRowContext(ctx, queryUser, sender.ID).Scan(&user.TelegramID, &user.Username, &user.FirstName, &user.State, &user.Faction)

	if err == nil {
		if user.Faction == "" {
			return h.renderFactionChoice(c, sender.ID)
		}

		var camp models.Encampment
		var res models.Resources
		var region string
		var myX, myY int

		queryCamp := `
			SELECT e.id, e.name, r.scrap, r.rations, r.energy, c.region, c.x, c.y 
			FROM encampments e
			JOIN resources r ON r.encampment_id = e.id
			JOIN coordinates c ON c.id = e.coordinate_id
			WHERE e.user_id = $1`
		
		err = h.DB.QueryRowContext(ctx, queryCamp, user.TelegramID).Scan(&camp.ID, &camp.Name, &res.Scrap, &res.Rations, &res.Energy, &region, &myX, &myY)
		if err != nil {
			log.Printf("Failed to query existing player details: %v", err)
			return c.Send("⚠️ System error reclaiming session database.", keyboards.MainNavigation())
		}

		_ = h.DB.QueryRowContext(ctx, "UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE telegram_id = $1", user.TelegramID)

		var activeMiners int
		_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(miners_assigned), 0) FROM active_mining_queues WHERE encampment_id = $1 AND is_completed = FALSE", camp.ID).Scan(&activeMiners)

		var ownedMiners int
		_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(miners, 1) FROM workshop_inventory WHERE encampment_id = $1", camp.ID).Scan(&ownedMiners)

		var outboundCount int
		_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM raids WHERE attacker_id = $1 AND (state = 'marching' OR state = 'engaged')", camp.ID).Scan(&outboundCount)

		var inboundExists bool
		_ = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM raids WHERE defender_id = $1 AND state = 'marching')", camp.ID).Scan(&inboundExists)

		systemState := "🟢 SECURE (NOMINAL)"
		if inboundExists {
			systemState = "🔴 WARNING (HOSTILE RAID IN TRANSIT)"
		}

		dashboard := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"📡 VAGABOND SYSTEM TERMINAL\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Welcome back, Commander %s.\n\n"+
				"SYSTEM TELEMETRY HUD:\n"+
				"📡 State: %s\n"+
				"⛏️ Miners: %d / %d active | Idle: %d\n"+
				"🚀 Transits: %d outbound transits running\n\n"+
				"Faction: %s\n"+
				"🌍 Territory: Encampment located in [%s]\n"+
				"⛺ Encampment: %s\n"+
				"📍 Location: [X: %d, Y: %d]\n\n"+
				"CURRENT RESOURCE BALANCES:\n"+
				"⚙️ Scrap: %.1f\n"+
				"🥫 Rations: %.1f\n"+
				"🔋 Energy Cells: %.1f\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Use the command manual below to learn terminal shortcuts.",
			user.FirstName, systemState, activeMiners, ownedMiners, ownedMiners-activeMiners, outboundCount,
			formatFactionLabel(user.Faction), region, camp.Name, myX, myY, res.Scrap, res.Rations, res.Energy,
		)

		selector := &telebot.ReplyMarkup{}
		btnWarehouse := selector.Data("📦 Warehouse Stocks", "view_warehouse")
		btnManual := selector.Data("📖 Survival Manual", "view_manual")

		selector.Inline(
			selector.Row(btnWarehouse, btnManual),
		)

		return c.Send(dashboard, selector, keyboards.MainNavigation())
	}

	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("Database check execution failure: %v", err)
		return c.Send("⚠️ Database reading failure.", keyboards.MainNavigation())
	}

	return h.renderFactionChoice(c, sender.ID)
}

func (h *OnboardingHandler) renderFactionChoice(c telebot.Context, senderID int64) error {
	selector := &telebot.ReplyMarkup{}
	btnVanguard := selector.Data("🛡️ Steel Vanguard", "join_faction", "steel_vanguard", fmt.Sprintf("%d", senderID))
	btnNomads := selector.Data("⚙️ Rust Nomads", "join_faction", "rust_nomads", fmt.Sprintf("%d", senderID))

	selector.Inline(
		selector.Row(btnVanguard),
		selector.Row(btnNomads),
	)

	welcomeText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚠️ SYSTEM INTRUSION DETECTED\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"WARNING: Faction registration required. Deploy your core systems:\n\n" +
		"🛡️ [Steel Vanguard]\n" +
		"High-Tech remnant order. Focuses on energy conservation.\n" +
		"Starting Bonus: +50.0 Energy Cells\n\n" +
		"⚙️ [Rust Nomads]\n" +
		"Scrappy survival coalition. Focuses on resource collection.\n" +
		"Starting Bonus: +150.0 Scrap\n" +
		"━━━━━━━━━━━━━━━━━━━━━━"

	return c.Send(welcomeText, selector)
}

func (h *OnboardingHandler) HandleHelp(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	helpManual := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"📖 SURVIVAL TRAINING MANUAL & TUTORIAL\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Welcome, survivor. This guide explains the core operational loops:\n\n" +
		"⛺ [⛺ Outpost Camp Menu]\n" +
		"• Structural Upgrades: Spend Scrap to level up Tent, Scrap Heap, and Generator.\n" +
		"• Automation Agent: Gated module. Automatically builds facilities and gathers Scrap.\n\n" +
		"⚔️ [⚔️ Tactical Combat Menu]\n" +
		"• Scan Targets: Locate neighboring outposts. Launch raids using your Soldiers/Enforcers.\n" +
		"• Wasteland Radio: Real-time broadcast news detailing sector collapses, storms, and wars.\n\n" +
		"🏦 [🏦 System Economy Menu]\n" +
		"• Financial Vault: Deposit Scrap to earn interest or secure emergency credit lines.\n" +
		"• Clan Alliances: Establish or join forces (capped at 15 members). Trigger alliance wars.\n" +
		"• Heavy Workshop: Spend heavy resources (Steel, Uranium, Hydrogen) to assemble Fusion Tanks.\n\n" +
		"💡 SYSTEM TIP: Tapping '⬅️ Back to HQ' at any time restores the mother navigation keyboard.\n" +
		"━━━━━━━━━━━━━━━━━━━━━━"

	return c.Send(helpManual, keyboards.MainNavigation())
}

func (h *OnboardingHandler) HandleFactionCallback(c telebot.Context) error {
	ctx := context.Background()

	faction := c.Args()[0]
	telegramIDStr := c.Args()[1]

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error parsing registration payload."})
	}

	sender := c.Sender()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database transaction failure."})
	}
	defer tx.Rollback()

	insertUser := `
		INSERT INTO users (telegram_id, username, first_name, state, faction) 
		VALUES ($1, $2, $3, 'active', $4)
		ON CONFLICT (telegram_id) 
		DO UPDATE SET faction = $4, state = 'active'`
	_, err = tx.ExecContext(ctx, insertUser, telegramID, sender.Username, sender.FirstName, faction)
	if err != nil {
		log.Printf("Failed registering faction user: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database writing registration error."})
	}

	continents := []string{"Africa", "Europe", "Asia", "Americas"}
	spawnedContinent := continents[sender.ID%4]

	var coordID string
	var x, y int
	var success bool

	// Decouple Seeding Loop: Seeding random source exactly once outside of the coordinate calculation loop
	rSource := rand.NewSource(time.Now().UnixNano() + sender.ID)
	rGen := rand.New(rSource)

	for attempt := 0; attempt < 15; attempt++ {
		// Style Refactor (QF1003): Substituted sequential if-else logic with clean Go switch matching
		switch spawnedContinent {
		case "Africa":
			x = rGen.Intn(991) + 10 // [10, 1000]
			y = rGen.Intn(991) + 10 // [10, 1000]
		case "Europe":
			x = -(rGen.Intn(991) + 10) // [-1000, -10]
			y = rGen.Intn(991) + 10 // [10, 1000]
		case "Asia":
			x = rGen.Intn(991) + 10 // [10, 1000]
			y = -(rGen.Intn(991) + 10) // [-1000, -10]
		default: // Americas
			x = -(rGen.Intn(991) + 10) // [-1000, -10]
			y = -(rGen.Intn(991) + 10) // [-1000, -10]
		}

		biome := "wasteland"
		if rGen.Float64() < 0.30 {
			biome = "ruins"
		}

		insertCoord := `
			INSERT INTO coordinates (x, y, biome, danger_level, region, terrain) 
			VALUES ($1, $2, $3, 1, $4, $3) 
			ON CONFLICT (x, y) DO NOTHING
			RETURNING id`
		
		err = tx.QueryRowContext(ctx, insertCoord, x, y, biome, spawnedContinent).Scan(&coordID)
		if err == nil {
			success = true
			break
		}
	}

	if !success {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Spawning coordinate allocator failed. Please retry registration."})
	}

	var campID string
	queryCampExists := `SELECT id FROM encampments WHERE user_id = $1`
	err = tx.QueryRowContext(ctx, queryCampExists, telegramID).Scan(&campID)
	if errors.Is(err, sql.ErrNoRows) {
		campName := fmt.Sprintf("Outpost-%d", telegramID%1000)
		insertCamp := `
			INSERT INTO encampments (user_id, name, coordinate_id, level) 
			VALUES ($1, $2, $3, 1) 
			RETURNING id`
		err = tx.QueryRowContext(ctx, insertCamp, telegramID, campName, coordID).Scan(&campID)
		if err != nil {
			log.Printf("Failed creating encampment: %v", err)
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Camp allocation error."})
		}

		startingScrap := 1000.0
		startingEnergy := 250.0
		if faction == "steel_vanguard" {
			startingEnergy += 500.0
		} else {
			startingScrap += 1500.0
		}

		insertRes := `
			INSERT INTO resources (encampment_id, scrap, rations, energy, neuro_cores) 
			VALUES ($1, $2, 50.00, $3, 0.00)`
		_, err = tx.ExecContext(ctx, insertRes, campID, startingScrap, startingEnergy)
		if err != nil {
			log.Printf("Failed creating resources: %v", err)
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Resource allocation error."})
		}

		_, _ = tx.ExecContext(ctx, "INSERT INTO workshop_inventory (encampment_id) VALUES ($1) ON CONFLICT DO NOTHING", campID)
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Ledger completion error."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🛰️ Faction system deployed! Welcome survivor."})

	welcome := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛰️ COGNITIVE CORE BOOTED\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Welcome to the wastes, Commander %s.\n"+
			"Your terminal is now integrated into [%s].\n"+
			"Your base has successfully spawned in territory: [%s]\n"+
			"📍 Location Coordinates: [X: %d, Y: %d]\n\n"+
			"Ready to check your commander statistics or modules.",
		sender.FirstName, formatFactionLabel(faction), spawnedContinent, x, y,
	)

	return c.Send(welcome, keyboards.MainNavigation())
}

func formatFactionLabel(f string) string {
	if f == "steel_vanguard" {
		return "🛡️ Steel Vanguard"
	}
	return "⚙️ Rust Nomads"
}