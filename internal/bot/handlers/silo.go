package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time" // Added missing time package

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type SiloHandler struct {
	DB *sql.DB
}

func NewSiloHandler(db *sql.DB) *SiloHandler {
	return &SiloHandler{DB: db}
}

// HandleSiloPanel renders the strategic nuclear weapons panel and target selection
func (h *SiloHandler) HandleSiloPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	var campLvl int
	err := h.DB.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	// 1. Enforce level requirement (Core level 15+)
	if campLvl < 15 {
		return c.Send("❌ Silo Access Locked: Reach Outpost Core Level 15 to open Strategic Nuclear Silos.", keyboards.MainNavigation())
	}

	// Fetch nuclear warhead stocks
	var nukes int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE((SELECT nukes FROM workshop_inventory WHERE encampment_id = $1), 0)", campID).Scan(&nukes)

	// Fetch up to 3 potential rival targets
	queryTargets := `
		SELECT e.id, e.name, u.first_name 
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		WHERE e.id != $1
		LIMIT 3`

	rows, err := h.DB.QueryContext(ctx, queryTargets, campID)
	var targetsText string
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if err != nil {
		log.Printf("Silo target scanning failed: %v", err)
		targetsText = "📡 Static: Target matrix scanning offline."
	} else {
		defer rows.Close()
		index := 1
		for rows.Next() {
			var tID, tName, tOwner string
			if err := rows.Scan(&tID, &tName, &tOwner); err == nil {
				targetsText += fmt.Sprintf("[%d] Outpost: %s | Commander: %s\n", index, tName, tOwner)
				btnLaunch := selector.Data(fmt.Sprintf("🚀 Detonate [%d]", index), "launch_icbm", tID)
				buttons = append(buttons, selector.Row(btnLaunch))
				index++
			}
		}
		if targetsText == "" {
			targetsText = "⚠️ Radar Clean: No rival outposts detected in strike range.\n"
		}
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"☢️ STRATEGIC SILO STRIKE DECK\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Deploy crafted inter-continental nuclear devices to vaporize rival targets.\n\n"+
			"SILO STORAGE INVENTORY:\n"+
			"🚀 Active ICBM Warheads: %d warheads\n\n"+
			"TARGET ACQUISITION RADAR:\n"+
			"%s"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		nukes, targetsText,
	)

	selector.Inline(buttons...)
	return c.Send(panelText, selector)
}

// HandleLaunchICBMCallback processes the strike, checks defenses, and applies nuclear debuffs
func (h *SiloHandler) HandleLaunchICBMCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	targetCampID := c.Args()[0]

	var myCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Launch failed."})
	}
	defer tx.Rollback()

	// 1. Verify stocks
	var nukes int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(nukes, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&nukes)

	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&energy)

	if nukes <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: You must forge a Nuclear Device in the Workshop first."})
	}

	if energy < 50.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Energy: ICBM launch requires 50.0 Energy Cells."})
	}

	// Deduct resources
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nukes = nukes - 1 WHERE encampment_id = $1", myCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - 50.0 WHERE encampment_id = $1", myCampID)

	// Fetch participant details
	var attackerName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", myCampID).Scan(&attackerName)

	var defenderName string
	var defenderUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", targetCampID).Scan(&defenderName, &defenderUserID)

	// 2. Check Defender Shields (Nuclear Shielding)
	var defenderShields int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(nuclear_shields, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", targetCampID).Scan(&defenderShields)

	if defenderShields > 0 {
		// Shield successfully intercepts the strike
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nuclear_shields = nuclear_shields - 1 WHERE encampment_id = $1", targetCampID)

		// Commit transaction
		_ = tx.Commit()

		_ = c.Respond(&telebot.CallbackResponse{Text: "🚨 ICBM INTERCEPTED: Target shielding blocked the strike!"})

		// Notify defender instantly
		defenderAlert := fmt.Sprintf(
			"🛡️ DEFENSE ALERT: ICBM SHIELD INTERCEPT!\n\n"+
				"Our Nuclear Shielding installations have successfully intercepted and destroyed an incoming tactical ICBM strike from Outpost [%s]!\n"+
				"💀 Casualties: 0 | Structural Damage: None. 1 Shielding charge depleted.",
			attackerName,
		)
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)

		return h.HandleSiloPanel(c)
	}

	// 3. No Shields: ICBM Detonation sequence triggers
	// Vaporize 50% of current troops
	_, _ = tx.ExecContext(ctx, "UPDATE units SET quantity = GREATEST(quantity / 2, 0) WHERE encampment_id = $1", targetCampID)
	_, _ = tx.ExecContext(ctx, "DELETE FROM units WHERE quantity <= 0")

	// Steal 50% of current Scrap reserves
	var defenderScrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", targetCampID).Scan(&defenderScrap)
	stolenScrap := defenderScrap * 0.50
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", stolenScrap, targetCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", stolenScrap, myCampID)

	// Destroy 1 random module level (Tent, Scrap Heap, or Generator drops by 1)
	modules := []string{"tent", "scrap_heap", "generator"}
	randomModule := modules[time.Now().UnixNano()%3]
	_, _ = tx.ExecContext(ctx, "UPDATE modules SET level = GREATEST(level - 1, 1) WHERE encampment_id = $1 AND type = $2", targetCampID, randomModule)

	_ = tx.Commit()

	_ = c.Respond(&telebot.CallbackResponse{Text: "💥 DETONATION: ICBM successfully detonated on target!"})

	// Log global news headline
	newsHeadline := fmt.Sprintf("💥 DETONATION ALERT: Commander %s launched an ICBM. Outpost %s suffered catastrophic nuclear damage.", sender.FirstName, defenderName)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", newsHeadline)

	// Notify defender
	defenderAlert := fmt.Sprintf(
		"💥 CATASTROPHIC NUCLEAR ALERT: DIRECT IMPACT!\n\n"+
			"An ICBM warhead launched by Outpost [%s] has detonated directly on your base!\n\n"+
			"💀 Casualties: 50%% of all barracks troops vaporized.\n"+
			"🛠️ Structural Damage: Your [%s] level dropped by 1.\n"+
			"⚙️ Resource Looted: -%.1f Scrap stolen from warehouses.",
		attackerName, randomModule, stolenScrap,
	)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)

	return h.HandleSiloPanel(c)
}
