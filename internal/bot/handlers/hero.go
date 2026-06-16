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

	// Fetch player faction
	var faction string
	_ = h.DB.QueryRowContext(ctx, "SELECT faction FROM users WHERE telegram_id = $1", sender.ID).Scan(&faction)

	var hero models.Hero
	queryHero := `SELECT id, name, trait, injuries, battles_survived, superpower FROM heroes WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryHero, campID).Scan(&hero.ID, &hero.Name, &hero.Trait, &hero.Injuries, &hero.BattlesSurvived, &hero.Superpower)

	if errors.Is(err, sql.ErrNoRows) {
		// Define empty variable fields first to prevent compiler shadowing issues
		var heroName string
		var heroTrait string
		var heroInjury string
		var heroSuperpower string

		if faction == "steel_vanguard" {
			heroName = "Iron Warden"
			heroTrait = "Fortress Tactician"
			heroInjury = "Scarred Eye"
			heroSuperpower = "🛡️ Kinetic Barrier (Reduces incoming troop damage by 15%)"
		} else {
			heroName = "Waste Phantom"
			heroTrait = "Salvage Specialist"
			heroInjury = "Cybernetic Hand"
			heroSuperpower = "⚙️ Scrap Recovery (+10% combat scrap loot)"
		}

		insertHero := `
			INSERT INTO heroes (encampment_id, name, trait, injuries, battles_survived, superpower) 
			VALUES ($1, $2, $3, $4, 0, $5) 
			RETURNING id, name, trait, injuries, battles_survived, superpower`

		err = h.DB.QueryRowContext(ctx, insertHero, campID, heroName, heroTrait, heroInjury, heroSuperpower).Scan(&hero.ID, &hero.Name, &hero.Trait, &hero.Injuries, &hero.BattlesSurvived, &hero.Superpower)
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
			"👥 CO-ORDINATOR COMMANDER HUD\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Your leader remembers every clash, scar, and survival outcome.\n\n"+
			"COMMANDER STATUS:\n"+
			"👤 Name: %s\n"+
			"🎗️ Psychological Trait: %s\n"+
			"🩺 Injuries Sustained: %s\n"+
			"⚡ Faction Superpower: %s\n"+
			"🎖️ Survived Conflicts: %d battles\n\n"+
			"COMMANDER OVERVIEWS:\n"+
			"The commander influences dynamic combat resolutions depending on their unlocked superpowers.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		hero.Name, hero.Trait, hero.Injuries, hero.Superpower, hero.BattlesSurvived,
	)

	return c.Send(dashboard, keyboards.CampNavigation())
}
