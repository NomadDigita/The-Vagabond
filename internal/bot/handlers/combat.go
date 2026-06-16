package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type CombatHandler struct {
	DB *sql.DB
}

func NewCombatHandler(db *sql.DB) *CombatHandler {
	return &CombatHandler{DB: db}
}

// HandleRaidBoard displays other player bases available for attack
func (h *CombatHandler) HandleRaidBoard(c telebot.Context) error {
	// Trigger custom finding location action indicator
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	// ... remainder of file unchanged ...
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	// Get player's own encampment details
	var myCampID string
	var myCampName string
	err := h.DB.QueryRowContext(ctx, "SELECT id, name FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID, &myCampName)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	// Fetch up to 5 potential targets (excluding own)
	query := `
		SELECT e.id, e.name, u.first_name,
		       COALESCE((SELECT r.scrap FROM resources r WHERE r.encampment_id = e.id), 0) as scrap
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		WHERE e.id != $1
		LIMIT 5`

	rows, err := h.DB.QueryContext(ctx, query, myCampID)
	if err != nil {
		log.Printf("Failed scanning target outposts: %v", err)
		return c.Send("⚠️ Failed to load target database matrix.")
	}
	defer rows.Close()

	type target struct {
		id       string
		name     string
		owner    string
		lootable float64
	}

	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.name, &t.owner, &t.lootable); err == nil {
			targets = append(targets, t)
		}
	}

	dashboard := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚔️ TACTICAL TARGET MATRIX\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Select an active player outpost to launch a raiding mission. " +
		"March and resolution take exactly 15 seconds for testing.\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if len(targets) == 0 {
		dashboard += "⚠️ SENSORS CLEAN: No other active outposts detected in range."
	} else {
		for i, t := range targets {
			dashboard += fmt.Sprintf("[%d] Outpost: %s\n    Commander: %s\n    Estimated Loot: %.1f Scrap\n\n", i+1, t.name, t.owner, t.lootable)
			btn := selector.Data(fmt.Sprintf("⚔️ Raid [%d] (%s)", i+1, t.name), "launch_raid", myCampID, t.id)
			buttons = append(buttons, selector.Row(btn))
		}
	}
	dashboard += "━━━━━━━━━━━━━━━━━━━━━━"

	selector.Inline(buttons...)
	return c.Send(dashboard, selector, keyboards.MainNavigation())
}

// HandleLaunchRaidCallback registers a marching raid inside the database
func (h *CombatHandler) HandleLaunchRaidCallback(c telebot.Context) error {
	ctx := context.Background()

	attackerCampID := c.Args()[0]
	defenderCampID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database transaction error."})
	}
	defer tx.Rollback()

	// 1. Check if attacker has at least one troop to fight
	var troopCount int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM units WHERE encampment_id = $1", attackerCampID).Scan(&troopCount)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error querying troop configurations."})
	}

	if troopCount <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Forbidden: You must have at least 1 Drifter unit in your barracks to launch a raid."})
	}

	// 2. Set marching timer (15 seconds)
	resolveTime := time.Now().Add(15 * time.Second)

	insertRaid := `
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
		VALUES ($1, $2, 'marching', $3)`
	_, err = tx.ExecContext(ctx, insertRaid, attackerCampID, defenderCampID, resolveTime)
	if err != nil {
		log.Printf("Failed executing raid insert: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to register raid marching database entries."})
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction commit failure."})
	}

	return c.Respond(&telebot.CallbackResponse{Text: "🚀 Raiders deployed! Marching towards target... (15s remaining)"})
}
