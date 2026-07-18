package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type HeroHandler struct {
	DB *sql.DB
}

func NewHeroHandler(db *sql.DB) *HeroHandler {
	return &HeroHandler{DB: db}
}

func (h *HeroHandler) HandleHeroPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Setup your outpost camp first using /start", keyboards.MainNavigation())
	}

	var faction string
	_ = h.DB.QueryRowContext(ctx, "SELECT faction FROM users WHERE telegram_id = $1", sender.ID).Scan(&faction)

	var heroID, heroName, heroTrait, heroInjuries, heroSuperpower string
	var battlesSurvived, lvl, xp int

	// Added robust COALESCE filters to safely handle nullable DB fields
	queryHero := `
		SELECT id, name, trait, injuries, battles_survived, COALESCE(superpower, ''), COALESCE(level, 1), COALESCE(xp, 0) 
		FROM heroes 
		WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryHero, campID).Scan(&heroID, &heroName, &heroTrait, &heroInjuries, &battlesSurvived, &heroSuperpower, &lvl, &xp)

	if errors.Is(err, sql.ErrNoRows) {
		if faction == "steel_vanguard" {
			heroName = "Iron Warden"
			heroTrait = "Fortress Tactician"
			heroInjuries = "Scarred Eye"
			heroSuperpower = "🛡️ Kinetic Barrier (Reduces incoming damage by 15%)"
		} else {
			heroName = "Waste Phantom"
			heroTrait = "Salvage Specialist"
			heroInjuries = "Cybernetic Hand"
			heroSuperpower = "⚙️ Scrap Recovery (+10% combat scrap loot)"
		}

		insertHero := `
			INSERT INTO heroes (encampment_id, name, trait, injuries, battles_survived, superpower, level, xp) 
			VALUES ($1, $2, $3, $4, 0, $5, 1, 0) 
			RETURNING id, name, trait, injuries, battles_survived, COALESCE(superpower, ''), COALESCE(level, 1), COALESCE(xp, 0)`
		
		err = h.DB.QueryRowContext(ctx, insertHero, campID, heroName, heroTrait, heroInjuries, heroSuperpower).Scan(&heroID, &heroName, &heroTrait, &heroInjuries, &battlesSurvived, &heroSuperpower, &lvl, &xp)
		if err != nil {
			log.Printf("Failed creating elite hero: %v", err)
			return c.Send("⚠️ Failed to write commander metrics.", keyboards.CampNavigation())
		}
	} else if err != nil {
		log.Printf("Failed scanning hero: %v", err)
		return c.Send("⚠️ Database error querying commander metrics.", keyboards.CampNavigation())
	}

	dashboard := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"👥 CO-ORDINATOR COMMANDER HUD [LEVEL %d]\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Your leader remembers every clash, scar, and survival outcome.\n\n"+
			"COMMANDER STATUS:\n"+
			"👤 Name: %s\n"+
			"🎗️ Psychological Trait: %s\n"+
			"🩺 Injuries Sustained: %s\n"+
			"⚡ Faction Superpower: %s\n"+
			"📊 Training Progress: %d / 100 XP\n"+
			"🎖️ Survived Conflicts: %d battles\n\n"+
			"COMMANDER TRAINING MODULES:\n"+
			"🏋️ [Train Commander] — Cost: 50 Scrap (+20 XP)\n"+
			"💊 [Heal Injury] — Cost: 50 Rations (Heals sustained scars)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		lvl, heroName, heroTrait, heroInjuries, heroSuperpower, xp, battlesSurvived,
	)

	selector := &telebot.ReplyMarkup{}
	btnTrain := selector.Data("🏋️ Train Commander", "hero_action", "train")
	btnHeal := selector.Data("💊 Heal Injury", "hero_action", "heal")
	btnGarrison := selector.Data("🛡️ Manual Defense Garrison", "hero_action", "garrison")

	selector.Inline(
		selector.Row(btnTrain, btnHeal),
		selector.Row(btnGarrison),
	)

	return sendPanelWithNav(c, navCaptionCamp, keyboards.CampNavigation(), dashboard, selector)
}

// HandleGarrisonPanel shows the player's current garrison reservation
// (how many Soldiers/Mechs are locked at home vs. free to draft into a
// campaign) and lets them adjust it. This is the "player manually assigns
// defensive units" / "withdraw defenders anytime" half of the Hero
// Commander system - garrisoned units always still count for defense
// (nothing is subtracted from combat), they're simply protected from
// being pulled into an outgoing raid draft.
func (h *HeroHandler) HandleGarrisonPanel(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Setup your outpost camp first using /start", keyboards.MainNavigation())
	}

	var soldiers, mechs, garrSoldiers, garrMechs int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(soldiers,0), COALESCE(mechs,0), COALESCE(garrisoned_soldiers,0), COALESCE(garrisoned_mechs,0) FROM workshop_inventory WHERE encampment_id = $1", campID).
		Scan(&soldiers, &mechs, &garrSoldiers, &garrMechs)

	freeSoldiers := soldiers - garrSoldiers
	freeMechs := mechs - garrMechs

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛡️ MANUAL DEFENSE GARRISON\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Garrisoned units always defend, but can NEVER be drafted into an outgoing raid - lock in a home guard you won't accidentally send away.\n\n"+
			"🪖 Soldiers: %d garrisoned / %d total (%d draftable)\n"+
			"🤖 Mechs: %d garrisoned / %d total (%d draftable)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		garrSoldiers, soldiers, freeSoldiers, garrMechs, mechs, freeMechs,
	)

	selector := &telebot.ReplyMarkup{}
	btnPlusSol := selector.Data("🪖 +10 Garrison", "garrison_adjust", "soldier", "inc")
	btnMinusSol := selector.Data("🪖 -10 Garrison", "garrison_adjust", "soldier", "dec")
	btnPlusMech := selector.Data("🤖 +5 Garrison", "garrison_adjust", "mech", "inc")
	btnMinusMech := selector.Data("🤖 -5 Garrison", "garrison_adjust", "mech", "dec")
	selector.Inline(
		selector.Row(btnPlusSol, btnMinusSol),
		selector.Row(btnPlusMech, btnMinusMech),
	)

	if c.Callback() != nil {
		return c.Edit(panelText, selector)
	}
	return c.Send(panelText, selector)
}

// HandleGarrisonAdjustCallback moves units between the general draftable
// pool and the locked garrison reserve. "inc" reserves more units at home
// (soldiers in steps of 10, mechs in steps of 5); "dec" withdraws them
// back to the general pool, freeing them up to draft into a raid again.
func (h *HeroHandler) HandleGarrisonAdjustCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	unitType := c.Args()[0]
	action := c.Args()[1]

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Adjustment failed."})
	}
	defer tx.Rollback()

	var soldiers, mechs, garrSoldiers, garrMechs int
	err = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers,0), COALESCE(mechs,0), COALESCE(garrisoned_soldiers,0), COALESCE(garrisoned_mechs,0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", campID).
		Scan(&soldiers, &mechs, &garrSoldiers, &garrMechs)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Garrison records inaccessible."})
	}

	step := 10
	column := "garrisoned_soldiers"
	current := garrSoldiers
	total := soldiers
	if unitType == "mech" {
		step = 5
		column = "garrisoned_mechs"
		current = garrMechs
		total = mechs
	}

	newVal := current
	if action == "inc" {
		newVal += step
		if newVal > total {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Not enough units in inventory to garrison that many."})
		}
	} else {
		newVal -= step
		if newVal < 0 {
			newVal = 0
		}
	}

	query := fmt.Sprintf("UPDATE workshop_inventory SET %s = $1 WHERE encampment_id = $2", column)
	if _, err := tx.ExecContext(ctx, query, newVal, campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Garrison update failed."})
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🛡️ Garrison reservation updated."})
	return h.HandleGarrisonPanel(c)
}

func (h *HeroHandler) HandleHeroCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	action := c.Args()[0]

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Action failed."})
	}
	defer tx.Rollback()

	var scrap, rations float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap, rations FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap, &rations)

	var currentLvl int
	var currentXp int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 1), COALESCE(xp, 0) FROM heroes WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&currentLvl, &currentXp)

	switch action {
	case "garrison":
		_ = tx.Rollback()
		return h.HandleGarrisonPanel(c)

	case "train":
		if scrap < 50.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap! Need 50."})
		}
		
		newXp := currentXp + 20
		newLvl := currentLvl
		if newXp >= 100 {
			newLvl++
			newXp = 0
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, '🏆 COMMANDER LEVEL UP: Your Hero commander has successfully reached Level ' || $2::text || '!', FALSE)", sender.ID, newLvl)
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE heroes SET level = $1, xp = $2 WHERE encampment_id = $3", newLvl, newXp, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🏋️ Commander training completed successfully! +20 XP."})

	case "heal":
		if rations < 50.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Rations! Need 50."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - 50.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE heroes SET injuries = 'Perfect Health' WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💊 Injuries fully cured! Commander is in perfect health."})
	}

	_ = tx.Commit()
	return h.HandleHeroPanel(c)
}