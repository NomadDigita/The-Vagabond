package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type SiloHandler struct {
	DB *sql.DB
}

func NewSiloHandler(db *sql.DB) *SiloHandler {
	return &SiloHandler{DB: db}
}

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

	if campLvl < 15 {
		return c.Send("❌ Silo Access Locked: Reach Outpost Core Level 15 to open Strategic Nuclear Silos.", keyboards.MainNavigation())
	}

	var nukes int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE((SELECT nukes FROM workshop_inventory WHERE encampment_id = $1), 0)", campID).Scan(&nukes)

	var piercingMissiles int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE((SELECT piercing_missiles FROM workshop_inventory WHERE encampment_id = $1), 0)", campID).Scan(&piercingMissiles)

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
				btnPierce := selector.Data(fmt.Sprintf("🎯 Pierce [%d]", index), "launch_piercing", tID)
				buttons = append(buttons, selector.Row(btnLaunch, btnPierce))
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
			"🚀 Active ICBM Warheads: %d warheads\n"+
			"🎯☢️ Piercing Missiles: %d warheads\n\n"+
			"TARGET ACQUISITION RADAR:\n"+
			"%s"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		nukes, piercingMissiles, targetsText,
	)

	selector.Inline(buttons...)
	return sendPanelWithNav(c, navCaptionCamp, keyboards.CampNavigation(), panelText, selector)
}

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

	var nukes int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(nukes, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&nukes)

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&electricity)

	if nukes <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: You must forge a Nuclear Device in the Workshop first."})
	}

	if electricity < 50.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Electricity: ICBM launch requires 50.0 Electricity Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nukes = nukes - 1 WHERE encampment_id = $1", myCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - 50.0 WHERE encampment_id = $1", myCampID)

	var attackerName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", myCampID).Scan(&attackerName)

	var defenderName string
	var defenderUserID int64

	// Handled separately if target is an AI target
	var isAI bool = targetCampID == "ai_drone_nest"
	if !isAI {
		err = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", targetCampID).Scan(&defenderName, &defenderUserID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Target encampment not found."})
		}
	} else {
		defenderName = "Rogue Drone Nest"
		defenderUserID = 0
	}

	var antiMissileLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'anti_missile'", targetCampID).Scan(&antiMissileLvl)

	if antiMissileLvl > 0 {
		interceptChance := 0.08 * float64(antiMissileLvl)
		if interceptChance > 0.80 {
			interceptChance = 0.80
		}
		if rand.Float64() < interceptChance {
			_ = tx.Commit()
			_ = c.Respond(&telebot.CallbackResponse{Text: "🚨 ICBM INTERCEPTED: Target's Anti-Missile Battery shot it down!"})

			if defenderUserID != 0 {
				defenderAlert := fmt.Sprintf(
					"🛡️ DEFENSE ALERT: ANTI-MISSILE BATTERY INTERCEPT!\n\n"+
						"Our Anti-Missile Battery successfully shot down an incoming tactical ICBM strike from Outpost [%s]!\n"+
						"💀 Casualties: 0 | Structural Damage: None.",
					attackerName,
				)
				_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)
			}

			return h.HandleSiloPanel(c)
		}
	}

	var defenderShields int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(nuclear_shields, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", targetCampID).Scan(&defenderShields)

	if defenderShields > 0 {
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET nuclear_shields = nuclear_shields - 1 WHERE encampment_id = $1", targetCampID)
		_ = tx.Commit()

		_ = c.Respond(&telebot.CallbackResponse{Text: "🚨 ICBM INTERCEPTED: Target shielding blocked the strike!"})

		if defenderUserID != 0 {
			defenderAlert := fmt.Sprintf(
				"🛡️ DEFENSE ALERT: ICBM SHIELD INTERCEPT!\n\n"+
					"Our Nuclear Shielding installations have successfully intercepted and destroyed an incoming tactical ICBM strike from Outpost [%s]!\n"+
					"💀 Casualties: 0 | Structural Damage: None. 1 Shielding charge depleted.",
				attackerName,
			)
			_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)
		}

		return h.HandleSiloPanel(c)
	}

	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = GREATEST(soldiers / 2, 0), mechs = GREATEST(mechs / 2, 0) WHERE encampment_id = $1", targetCampID)

	var defenderScrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", targetCampID).Scan(&defenderScrap)
	stolenScrap := defenderScrap * 0.50
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", stolenScrap, targetCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", stolenScrap, myCampID)

	modules := []string{"tent", "scrap_heap", "generator"}
	randomModule := modules[time.Now().UnixNano()%3]
	_, _ = tx.ExecContext(ctx, "UPDATE modules SET level = GREATEST(level - 1, 1) WHERE encampment_id = $1 AND type = $2", targetCampID, randomModule)

	_ = tx.Commit()

	_ = c.Respond(&telebot.CallbackResponse{Text: "💥 DETONATION: ICBM successfully detonated on target!"})

	newsHeadline := fmt.Sprintf("💥 DETONATION ALERT: Commander %s launched an ICBM. Outpost %s suffered catastrophic nuclear damage.", sender.FirstName, defenderName)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", newsHeadline)

	if defenderUserID != 0 {
		defenderAlert := fmt.Sprintf(
			"💥 CATASTROPHIC NUCLEAR ALERT: DIRECT IMPACT!\n\n"+
				"An ICBM warhead launched by Outpost [%s] has detonated directly on your base!\n\n"+
				"💀 Casualties: 50%% of all barracks troops vaporized.\n"+
				"🛠️ Structural Damage: Your [%s] level dropped by 1.\n"+
				"⚙️ Resource Looted: -%.1f Scrap stolen from warehouses.",
			attackerName, randomModule, stolenScrap,
		)
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)
	}

	return h.HandleSiloPanel(c)
}

// HandleLaunchPiercingMissileCallback fires a Piercing Missile: a Silo
// weapon built to punch through defenses rather than raid strength. It's
// far harder for an Anti-Missile Battery to intercept than a standard
// Nuke (half the intercept chance, half the cap) and is never stopped by
// Nuclear Shields at all - but instead of vaporizing troops, it directly
// strips levels off the target's Defense Grid turrets.
func (h *SiloHandler) HandleLaunchPiercingMissileCallback(c telebot.Context) error {
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

	var piercingMissiles int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(piercing_missiles, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&piercingMissiles)

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&electricity)

	if piercingMissiles <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: You must forge a Piercing Missile in the Workshop first."})
	}

	if electricity < 70.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Electricity: Piercing Missile launch requires 70.0 Electricity Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET piercing_missiles = piercing_missiles - 1 WHERE encampment_id = $1", myCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - 70.0 WHERE encampment_id = $1", myCampID)

	var attackerName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", myCampID).Scan(&attackerName)

	var defenderName string
	var defenderUserID int64

	var isAI bool = targetCampID == "ai_drone_nest"
	if !isAI {
		err = tx.QueryRowContext(ctx, "SELECT name, user_id FROM encampments WHERE id = $1", targetCampID).Scan(&defenderName, &defenderUserID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Target encampment not found."})
		}
	} else {
		defenderName = "Rogue Drone Nest"
		defenderUserID = 0
	}

	var antiMissileLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'anti_missile'", targetCampID).Scan(&antiMissileLvl)

	if antiMissileLvl > 0 {
		// Half the intercept chance and half the cap of a standard ICBM -
		// piercing warheads are built specifically to slip past this.
		interceptChance := 0.04 * float64(antiMissileLvl)
		if interceptChance > 0.40 {
			interceptChance = 0.40
		}
		if rand.Float64() < interceptChance {
			_ = tx.Commit()
			_ = c.Respond(&telebot.CallbackResponse{Text: "🚨 PIERCING MISSILE INTERCEPTED: Target's Anti-Missile Battery shot it down!"})

			if defenderUserID != 0 {
				defenderAlert := fmt.Sprintf(
					"🛡️ DEFENSE ALERT: ANTI-MISSILE BATTERY INTERCEPT!\n\n"+
						"Our Anti-Missile Battery successfully shot down an incoming Piercing Missile strike from Outpost [%s]!\n"+
						"💀 Casualties: 0 | Structural Damage: None.",
					attackerName,
				)
				_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)
			}

			return h.HandleSiloPanel(c)
		}
	}

	// Piercing Missiles ignore Nuclear Shields entirely - that's the
	// entire point of the weapon - and strip one level off EVERY Defense
	// Grid turret type instead of vaporizing troops or a random facility.
	turretTypes := []string{"light_laser", "heavy_laser", "gauss_cannon", "ion_cannon", "plasma_turret"}
	for _, t := range turretTypes {
		_, _ = tx.ExecContext(ctx, "UPDATE modules SET level = GREATEST(level - 1, 0) WHERE encampment_id = $1 AND type = $2", targetCampID, t)
	}

	var defenderScrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", targetCampID).Scan(&defenderScrap)
	stolenScrap := defenderScrap * 0.25
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", stolenScrap, targetCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", stolenScrap, myCampID)

	_ = tx.Commit()

	_ = c.Respond(&telebot.CallbackResponse{Text: "🎯 DETONATION: Piercing Missile punched straight through the Defense Grid!"})

	newsHeadline := fmt.Sprintf("🎯 PIERCING STRIKE ALERT: Commander %s launched a Piercing Missile. Outpost %s's Defense Grid turrets took direct structural damage.", sender.FirstName, defenderName)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", newsHeadline)

	if defenderUserID != 0 {
		defenderAlert := fmt.Sprintf(
			"🎯☢️ CATASTROPHIC ALERT: PIERCING MISSILE IMPACT!\n\n"+
				"A Piercing Missile launched by Outpost [%s] has slipped past your Nuclear Shields entirely and detonated directly on your Defense Grid!\n\n"+
				"🛠️ Structural Damage: Every turret type (Light/Heavy Laser, Gauss Cannon, Ion Cannon, Plasma Turret) dropped 1 level.\n"+
				"⚙️ Resource Looted: -%.1f Scrap stolen from warehouses.",
			attackerName, stolenScrap,
		)
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defenderUserID, defenderAlert)
	}

	return h.HandleSiloPanel(c)
}