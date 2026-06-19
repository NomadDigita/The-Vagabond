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

// HandleHeroPanel displays the commander statistics, level, XP, and injuries with interactive buttons
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

	// Resilient local scan variables to prevent struct validation/NULL field mismatch crashes
	var heroID, heroName, heroTrait, heroInjuries, heroSuperpower string
	var battlesSurvived, lvl, xp int

	queryHero := `
		SELECT id, name, trait, injuries, battles_survived, superpower, level, xp 
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
			RETURNING id, name, trait, injuries, battles_survived, superpower, level, xp`
		
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

	selector.Inline(
		selector.Row(btnTrain, btnHeal),
	)

	return c.Send(dashboard, selector)
}

// HandleHeroCallback processes commander training, XP leveling, and medical healing
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
	_ = tx.QueryRowContext(ctx, "SELECT level, xp FROM heroes WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&currentLvl, &currentXp)

	switch action {
	case "train":
		if scrap < 50.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap! Need 50."})
		}
		
		newXp := currentXp + 20
		newLvl := currentLvl
		if newXp >= 100 {
			newLvl++
			newXp = 0
			// Queue Level-up alert
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