package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/models"
	"gopkg.in/telebot.v3"
)

type HeroHandler struct {
	DB *sql.DB
}

func NewHeroHandler(db *sql.DB) *HeroHandler {
	return &HeroHandler{DB: db}
}

// HandleHeroPanel displays the commander statistics, trait bonuses, and injuries
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

	var hero models.Hero
	queryHero := `SELECT id, name, trait, injuries, battles_survived FROM heroes WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryHero, campID).Scan(&hero.ID, &hero.Name, &hero.Trait, &hero.Injuries, &hero.BattlesSurvived)

	if errors.Is(err, sql.ErrNoRows) {
		// Create the default Hero Commander
		heroName := fmt.Sprintf("Ghost-%d", sender.ID%100)
		insertHero := `
			INSERT INTO heroes (encampment_id, name, trait, injuries, battles_survived) 
			VALUES ($1, $2, 'Never Retreat', 'Scarred Chest', 0) 
			RETURNING id, name, trait, injuries, battles_survived`

		err = h.DB.QueryRowContext(ctx, insertHero, campID).Scan(&hero.ID, &hero.Name, &hero.Trait, &hero.Injuries, &hero.BattlesSurvived)
		if err != nil {
			log.Printf("Failed writing hero: %v", err)
			return c.Send("⚠️ Failed to write commander metrics.", keyboards.CampNavigation())
		}
	} else if err != nil {
		log.Printf("Failed scanning hero: %v", err)
		return c.Send("⚠️ Database error querying commander metrics.", keyboards.CampNavigation())
	}

	dashboard := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"👥 CO-ORDINATOR COMMANDER HUD\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Your leader remembers every clash, scar, and survival outcome.\n\n"+
			"COMMANDER TELEMETRY:\n"+
			"👤 Call Sign: %s\n"+
			"🎗️ Psychological Trait: %s\n"+
			"🩺 Sustained Injuries: %s\n"+
			"🎖️ Survived Conflicts: %d battles\n\n"+
			"PSYCHOLOGICAL BONUSES:\n"+
			"⚡ [Never Retreat] reduces morale consumption across barracks during ticks.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		hero.Name, hero.Trait, hero.Injuries, hero.BattlesSurvived,
	)

	return c.Send(dashboard, keyboards.CampNavigation())
}
