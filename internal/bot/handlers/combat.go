package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
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
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	// Get player's own encampment details
	var myCampID string
	var myCampName string
	var myCampLvl int
	var myX, myY int

	queryMe := `
		SELECT e.id, e.name, e.level, c.x, c.y 
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.user_id = $1`

	err := h.DB.QueryRowContext(ctx, queryMe, sender.ID).Scan(&myCampID, &myCampName, &myCampLvl, &myX, &myY)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	// Fetch up to 5 potential targets (excluding own)
	queryTargets := `
		SELECT e.id, e.name, u.first_name, c.x, c.y,
		       COALESCE((SELECT r.scrap FROM resources r WHERE r.encampment_id = e.id), 0) as scrap
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id != $1
		LIMIT 5`

	rows, err := h.DB.QueryContext(ctx, queryTargets, myCampID)
	if err != nil {
		log.Printf("Failed scanning target outposts: %v", err)
		return c.Send("⚠️ Failed to load target database matrix.", keyboards.CombatNavigation())
	}
	defer rows.Close()

	type target struct {
		id       string
		name     string
		owner    string
		x, y     int
		lootable float64
	}

	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.name, &t.owner, &t.x, &t.y, &t.lootable); err == nil {
			targets = append(targets, t)
		}
	}

	dashboard := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚔️ TACTICAL TARGET MATRIX\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Select an active player outpost to launch a raiding mission.\n" +
		"Distance determines travel times and rations consumed.\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if len(targets) == 0 {
		dashboard += "⚠️ SENSORS CLEAN: No other active outposts detected in range."
	} else {
		for i, t := range targets {
			// Calculate grid distance step requirements
			steps := math.Abs(float64(t.x-myX)) + math.Abs(float64(t.y-myY))
			if steps == 0 {
				steps = 1
			}
			marchTime := int(steps * 15) // 15s per coordinate step

			dashboard += fmt.Sprintf("[%d] Outpost: %s (Sector %d,%d)\n    Commander: %s\n    Travel Steps: %.0f | March Time: %ds\n    Estimated Loot: %.1f Scrap\n\n", i+1, t.name, t.x, t.y, t.owner, steps, marchTime, t.lootable)
			btn := selector.Data(fmt.Sprintf("⚔️ Raid [%d]", i+1), "launch_raid", myCampID, t.id, fmt.Sprintf("%.0f", steps))
			buttons = append(buttons, selector.Row(btn))
		}
	}
	dashboard += "━━━━━━━━━━━━━━━━━━━━━━"

	selector.Inline(buttons...)
	return c.Send(dashboard, selector, keyboards.CombatNavigation())
}

// HandleLaunchRaidCallback registers a marching raid inside the database and alerts the defender
func (h *CombatHandler) HandleLaunchRaidCallback(c telebot.Context) error {
	ctx := context.Background()

	attackerCampID := c.Args()[0]
	defenderCampID := c.Args()[1]
	stepsStr := c.Args()[2]

	steps, _ := strconv.ParseFloat(stepsStr, 64)
	marchDuration := time.Duration(steps*15) * time.Second

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database transaction error."})
	}
	defer tx.Rollback()

	// Verify Attacker has troops and sufficient rations (10 Rations consumed per step)
	requiredRations := steps * 10.0
	var rations float64
	_ = tx.QueryRowContext(ctx, "SELECT rations FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerCampID).Scan(&rations)

	if rations < requiredRations {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Rations! Need %.0f food to march this distance.", requiredRations)})
	}

	var troopCount int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM units WHERE encampment_id = $1", attackerCampID).Scan(&troopCount)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error querying troop configurations."})
	}

	if troopCount <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Forbidden: You must have at least 1 Drifter unit in your barracks."})
	}

	// Fetch names
	var attackerName string
	var attackerUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", attackerCampID).Scan(&attackerName, &attackerUserID)

	var defenderName string
	var defenderUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", defenderCampID).Scan(&defenderName, &defenderUserID)

	// Deduct rations
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - $1 WHERE encampment_id = $2", requiredRations, attackerCampID)

	// Queue the raid marching state
	resolveTime := time.Now().Add(marchDuration)
	insertRaid := `
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
		VALUES ($1, $2, 'marching', $3)`
	_, err = tx.ExecContext(ctx, insertRaid, attackerCampID, defenderCampID, resolveTime)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to register raid."})
	}

	// 5. Send Alert warning to defender instantly via Realtime Notification system!
	defenderAlert := fmt.Sprintf(
		"🚨 RADAR ALERT: HOSTILE MARCH DETECTED!\n\n"+
			"Our sensors have detected an incoming raiding march from Outpost [%s].\n"+
			"Estimated Arrival: %s (in %s).\n\n"+
			"Fortify your outer gates and defenses immediately!",
		attackerName, resolveTime.UTC().Format("15:04:05"), marchDuration.String(),
	)
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🚀 Raiders deployed! Marching towards target..."})

	// Play cinematic frame-by-frame text feed
	go func() {
		frames := []string{
			"🛰️ TACTICAL SCANS: Synchronizing spatial vectors...",
			"🚀 MARCHING: Troops travelling coordinates step-by-step...",
			"⚡ ENGAGING: Arrived at defender perimeters! clashing defenses...",
		}
		for _, f := range frames {
			formatted := fmt.Sprintf(
				"━━━━━━━━━━━━━━━━━━━━━━\n"+
					"🛡️ EXPEDITION EXPEDITION PATH: %s\n"+
					"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
					"Status: %s\n"+
					"Estimated Travel Duration: %s",
				attackerName, f, marchDuration.String(),
			)
			_ = c.Edit(formatted)
			time.Sleep(3 * time.Second)
		}
	}()

	return nil
}
