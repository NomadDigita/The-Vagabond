package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"

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

// HandleStart renders the main terminal dashboard and binds inline controls (HQ HUD)
func (h *OnboardingHandler) HandleStart(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("sender details missing from context")
	}

	ctx := context.Background()

	// Check if user exists
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
		queryCamp := `
			SELECT e.name, r.scrap, r.rations, r.energy, c.region 
			FROM encampments e
			JOIN resources r ON r.encampment_id = e.id
			JOIN coordinates c ON c.id = e.coordinate_id
			WHERE e.user_id = $1`
		
		err = h.DB.QueryRowContext(ctx, queryCamp, user.TelegramID).Scan(&camp.Name, &res.Scrap, &res.Rations, &res.Energy, &region)
		if err != nil {
			log.Printf("Failed to query existing player details: %v", err)
			return c.Send("⚠️ System error reclaiming session database.", keyboards.MainNavigation())
		}

		_ = h.DB.QueryRowContext(ctx, "UPDATE users SET last_active = CURRENT_TIMESTAMP WHERE telegram_id = $1", user.TelegramID)

		dashboard := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"📡 VAGABOND SYSTEM TERMINAL\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Welcome back, Commander %s.\n\n"+
				"Faction: %s\n"+
				"🌍 Territory: Encampment located in [%s]\n"+
				"⛺ Encampment: %s\n"+
				"📍 Location: [X: 0, Y: 0] (Secure Core Zone)\n\n"+
				"CURRENT RESOURCE BALANCES:\n"+
				"⚙️ Scrap: %.1f\n"+
				"🥫 Rations: %.1f\n"+
				"🔋 Energy Cells: %.1f\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Use the command manual below to learn terminal shortcuts.",
			user.FirstName, formatFactionLabel(user.Faction), region, camp.Name, res.Scrap, res.Rations, res.Energy,
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
		"💀 SYSTEM INTRUSION DETECTED\n" +
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

// HandleHelp renders the complete interactive system tutorial walkthrough
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

// HandleFactionCallback writes registration details depending on faction selection
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
	queryCoord := `SELECT id FROM coordinates WHERE x = 0 AND y = 0 AND region = $1`
	err = tx.QueryRowContext(ctx, queryCoord, spawnedContinent).Scan(&coordID)
	if errors.Is(err, sql.ErrNoRows) {
		insertCoord := `
			INSERT INTO coordinates (x, y, biome, danger_level, region, terrain) 
			VALUES (0, 0, 'wasteland', 1, $1, 'wasteland') 
			RETURNING id`
		err = tx.QueryRowContext(ctx, insertCoord, spawnedContinent).Scan(&coordID)
		if err != nil {
			log.Printf("Failed creating coordinates: %v", err)
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Mapping allocation error."})
		}
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
			"Your base has successfully spawned in territory: [%s]\n\n"+
			"Ready to check your commander statistics or modules.",
		sender.FirstName, formatFactionLabel(faction), spawnedContinent,
	)

	return c.Send(welcome, keyboards.MainNavigation())
}

func formatFactionLabel(f string) string {
	if f == "steel_vanguard" {
		return "🛡️ Steel Vanguard"
	}
	return "⚙️ Rust Nomads"
}