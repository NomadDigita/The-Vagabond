package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/models"
	"gopkg.in/telebot.v3"
)

// OnboardingHandler manages player registration flows.
type OnboardingHandler struct {
	DB *sql.DB
}

// NewOnboardingHandler builds a clean registration handler.
func NewOnboardingHandler(db *sql.DB) *OnboardingHandler {
	return &OnboardingHandler{DB: db}
}

// HandleStart catches the telegram /start command.
func (h *OnboardingHandler) HandleStart(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("sender details missing from context")
	}

	ctx := context.Background()
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Failed to initialize database transaction: %v", err)
		return c.Send("⚠️ Terminal error: Failed to access system database.")
	}
	defer tx.Rollback()

	// 1. Check if user already exists
	var user models.User
	queryUser := `SELECT telegram_id, username, first_name, state FROM users WHERE telegram_id = $1`
	err = tx.QueryRowContext(ctx, queryUser, sender.ID).Scan(&user.TelegramID, &user.Username, &user.FirstName, &user.State)

	if err == nil {
		// Existing Player Found
		var camp models.Encampment
		var res models.Resources
		queryCamp := `
			SELECT e.name, r.scrap, r.rations, r.energy 
			FROM encampments e
			JOIN resources r ON r.encampment_id = e.id
			WHERE e.user_id = $1`
		
		err = tx.QueryRowContext(ctx, queryCamp, user.TelegramID).Scan(&camp.Name, &res.Scrap, &res.Rations, &res.Energy)
		if err != nil {
			log.Printf("Failed to query existing player details: %v", err)
			return c.Send("⚠️ System error reclaiming session database.")
		}

		_ = tx.Commit()

		// Return Terminal HQ Dashboard to existing user
		dashboard := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"📡 VAGABOND SYSTEM TERMINAL\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"Welcome back, Commander %s.\n\n"+
				"⛺ Encampment: %s\n"+
				"📍 Location: [X: 0, Y: 0] (Secure Core Zone)\n\n"+
				"CURRENT RESOURCE BALANCES:\n"+
				"⚙️ Scrap: %.1f\n"+
				"🥫 Rations: %.1f\n"+
				"🔋 Energy Cells: %.1f\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"All modules online. Use physical inputs to execute commands.",
			user.FirstName, camp.Name, res.Scrap, res.Rations, res.Energy,
		)
		return c.Send(dashboard)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("Database check execution failure: %v", err)
		return c.Send("⚠️ Database reading failure.")
	}

	// 2. Register New User
	insertUser := `
		INSERT INTO users (telegram_id, username, first_name, state) 
		VALUES ($1, $2, $3, 'active')`
	_, err = tx.ExecContext(ctx, insertUser, sender.ID, sender.Username, sender.FirstName)
	if err != nil {
		log.Printf("Failed inserting player registration: %v", err)
		return c.Send("⚠️ Failed to write profile database registration.")
	}

	// 3. Ensure base map coordinates exist (Default starting coordinate at x=0, y=0)
	var coordID string
	queryCoord := `SELECT id FROM coordinates WHERE x = 0 AND y = 0`
	err = tx.QueryRowContext(ctx, queryCoord).Scan(&coordID)
	if errors.Is(err, sql.ErrNoRows) {
		insertCoord := `
			INSERT INTO coordinates (x, y, biome, danger_level) 
			VALUES (0, 0, 'wasteland', 1) 
			RETURNING id`
		err = tx.QueryRowContext(ctx, insertCoord).Scan(&coordID)
		if err != nil {
			log.Printf("Failed writing map coordinate default entry: %v", err)
			return c.Send("⚠️ Failed to generate spatial coordinate structures.")
		}
	} else if err != nil {
		log.Printf("Coordinate query execution failure: %v", err)
		return c.Send("⚠️ Coordinate mapping reading error.")
	}

	// 4. Create Player Encampment
	var campID string
	campName := fmt.Sprintf("Outpost-%d", sender.ID%1000)
	insertCamp := `
		INSERT INTO encampments (user_id, name, coordinate_id, level) 
		VALUES ($1, $2, $3, 1) 
		RETURNING id`
	err = tx.QueryRowContext(ctx, insertCamp, sender.ID, campName, coordID).Scan(&campID)
	if err != nil {
		log.Printf("Failed executing camp registration entry: %v", err)
		return c.Send("⚠️ Failed to register structural camp databases.")
	}

	// 5. Allocate Starting Resources (100 Scrap, 50 Rations, 25 Energy)
	insertRes := `
		INSERT INTO resources (encampment_id, scrap, rations, energy, neuro_cores) 
		VALUES ($1, 100.00, 50.00, 25.00, 0.00)`
	_, err = tx.ExecContext(ctx, insertRes, campID)
	if err != nil {
		log.Printf("Failed allocating user onboarding resources: %v", err)
		return c.Send("⚠️ Resource provisioning allocation error.")
	}

	// Commit complete transaction atomically
	if err = tx.Commit(); err != nil {
		log.Printf("Onboarding transaction commit failure: %v", err)
		return c.Send("⚠️ Transaction persistence deployment failure.")
	}

	// Send Cinematic Onboarding Console Message
	welcome := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"💀 SYSTEM INTRUSION DETECTED\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"WARNING: Organic presence recognized.\n\n"+
			"Initializing life support telemetry...\n"+
			"Initializing terminal ID: [%d]...\n"+
			"Deploying Encampment: %s [Sector: 0, 0]\n\n"+
			"INITIAL SYSTEM PROVISIONING:\n"+
			"⚙️ Scrap: 100.0\n"+
			"🥫 Rations: 50.0\n"+
			"🔋 Energy Cells: 25.0\n\n"+
			"Survival is the only core directive.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"The world continues, with or without you.",
		sender.ID, campName,
	)
	return c.Send(welcome)
}