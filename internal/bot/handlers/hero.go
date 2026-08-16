package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/gear"
	"github.com/NomadDigita/The-Vagabond/internal/game/herodepth"
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
		"👥 %s\n"+divider+"\n"+
			"%s\n\n"+
			"%s\n"+
			"👤 Name: %s\n"+
			"🎗️ Psychological Trait: %s\n"+
			"🩺 Injuries Sustained: %s\n"+
			"⚡ Faction Superpower: %s\n"+
			"📊 Training Progress: %s\n"+
			"🎖️ Survived Conflicts: %s\n\n"+
			"%s\n"+
			"🏋️ Train Commander — Cost: 50 Scrap (+20 XP)\n"+
			"💊 Heal Injury — Cost: 50 Rations (Heals sustained scars)\n"+
			divider,
		htmlBold(fmt.Sprintf("CO-ORDINATOR COMMANDER HUD [LEVEL %d]", lvl)),
		htmlItalic("Your leader remembers every clash, scar, and survival outcome."),
		htmlBold("COMMANDER STATUS"),
		htmlEscape(heroName),
		htmlEscape(heroTrait),
		htmlEscape(heroInjuries),
		htmlEscape(heroSuperpower),
		htmlCode(fmt.Sprintf("%d / 100 XP", xp)),
		htmlCode(fmt.Sprintf("%d battles", battlesSurvived)),
		htmlBold("COMMANDER TRAINING MODULES"),
	)

	selector := &telebot.ReplyMarkup{}
	btnTrain := selector.Data("🏋️ Train Commander", "hero_action", "train")
	btnHeal := selector.Data("💊 Heal Injury", "hero_action", "heal")
	btnGarrison := selector.Data("🛡️ Manual Defense Garrison", "hero_action", "garrison")
	btnSkills := selector.Data("🌳 Skill Tree", "hero_skills")
	btnHeroGear := selector.Data("⚙️ Hero Gear", "hero_gear")

	selector.Inline(
		selector.Row(btnTrain, btnHeal),
		selector.Row(btnGarrison),
		selector.Row(btnSkills, btnHeroGear),
	)

	return sendPanelWithNavHTML(c, navCaptionCamp, keyboards.CampNavigation(), dashboard, selector)
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

		var heroID string
		var faction string
		_ = tx.QueryRowContext(ctx, "SELECT id FROM heroes WHERE encampment_id = $1", campID).Scan(&heroID)
		_ = tx.QueryRowContext(ctx, "SELECT faction FROM users WHERE telegram_id = $1", sender.ID).Scan(&faction)

		// NEXT_GENERATION_FEATURES_PLAN.md Phase 6: unlocked skill
		// nodes discount the Scrap cost of training (EffectTrainingCostPct)
		// and a hero-equipped gear item (see herodepth.HeroXPBonusPct)
		// boosts the XP earned - two independent Phase 6 systems this
		// one action exercises together.
		trainCost := 50.0 * herodepth.TrainingCostMultiplier(ctx, tx, heroID)
		if scrap < trainCost {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap! Need %.0f.", trainCost)})
		}
		xpBonusPct := herodepth.HeroXPBonusPct(ctx, tx, campID, func(key string) (float64, bool) {
			def, ok := gear.ByKey(key)
			return def.ResourceBonusPct, ok
		})
		xpGain := int(20.0 * (1.0 + xpBonusPct/100.0))

		newXp := currentXp + xpGain
		newLvl := currentLvl
		for newXp >= 100 {
			newLvl++
			newXp -= 100
			levelUpMsg := fmt.Sprintf("🏆 %s: Your Hero commander has successfully reached Level %s! +1 skill point available - spend it on /hero_skills.", htmlBold("COMMANDER LEVEL UP"), htmlCode(fmt.Sprintf("%d", newLvl)))
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sender.ID, levelUpMsg)
			_, _ = tx.ExecContext(ctx, "UPDATE heroes SET skill_points = skill_points + 1 WHERE id = $1", heroID)
		}
		if newLvl > currentLvl && heroID != "" {
			if err := herodepth.DeliverStoryBeats(ctx, tx, sender.ID, heroID, faction, newLvl); err != nil {
				log.Printf("Failed delivering hero story beats: %v", err)
			}
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", trainCost, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE heroes SET level = $1, xp = $2 WHERE encampment_id = $3", newLvl, newXp, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🏋️ Commander training completed successfully! +%d XP.", xpGain)})

	case "heal":
		var heroID string
		_ = tx.QueryRowContext(ctx, "SELECT id FROM heroes WHERE encampment_id = $1", campID).Scan(&heroID)
		healCost := 50.0 * herodepth.HealCostMultiplier(ctx, tx, heroID)
		if rations < healCost {
			return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Rations! Need %.0f.", healCost)})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - $1 WHERE encampment_id = $2", healCost, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE heroes SET injuries = 'Perfect Health' WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💊 Injuries fully cured! Commander is in perfect health."})
	}

	_ = tx.Commit()
	return h.HandleHeroPanel(c)
}

// HandleHeroSkillsPanel (🌳 Skill Tree button) shows the three branches
// for the player's own faction, with a node's status
// (locked/unlocked/affordable) and an Unlock button on anything the
// hero can currently afford with a free skill point.
func (h *HeroHandler) HandleHeroSkillsPanel(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID, faction string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Send("⚠️ Setup your outpost camp first using /start", keyboards.MainNavigation())
	}
	_ = h.DB.QueryRowContext(ctx, "SELECT faction FROM users WHERE telegram_id = $1", sender.ID).Scan(&faction)

	var heroID string
	var points int
	_ = h.DB.QueryRowContext(ctx, "SELECT id, skill_points FROM heroes WHERE encampment_id = $1", campID).Scan(&heroID, &points)

	unlocked := make(map[string]bool)
	rows, err := h.DB.QueryContext(ctx, "SELECT skill_key FROM hero_skills_unlocked WHERE hero_id = $1", heroID)
	if err == nil {
		for rows.Next() {
			var key string
			if scanErr := rows.Scan(&key); scanErr == nil {
				unlocked[key] = true
			}
		}
		rows.Close()
	}

	panelText := "🌳 " + htmlBold("HERO SKILL TREE") + "\n" + divider + "\n" +
		fmt.Sprintf("💠 Skill Points: %s\n\n", htmlCode(fmt.Sprintf("%d", points)))

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	branches := []herodepth.Branch{}
	seen := map[herodepth.Branch]bool{}
	for _, n := range herodepth.NodesForFaction(faction) {
		if !seen[n.Branch] {
			branches = append(branches, n.Branch)
			seen[n.Branch] = true
		}
	}

	for _, branch := range branches {
		panelText += htmlBold(capitalizeWord(string(branch))) + "\n"
		for _, n := range herodepth.NodesForFaction(faction) {
			if n.Branch != branch {
				continue
			}
			if unlocked[n.Key] {
				panelText += fmt.Sprintf("✅ %s - %s\n", htmlEscape(n.Name), htmlEscape(n.Description))
			} else {
				panelText += fmt.Sprintf("🔒 %s - %s\n", htmlEscape(n.Name), htmlEscape(n.Description))
				if points > 0 {
					buttons = append(buttons, selector.Row(selector.Data(fmt.Sprintf("💠 Unlock %s", n.Name), "hero_skill_unlock", n.Key)))
				}
			}
		}
		panelText += "\n"
	}

	panelText += divider
	selector.Inline(buttons...)
	return sendPanelWithNavHTML(c, navCaptionCamp, keyboards.CampNavigation(), panelText, selector)
}

// HandleHeroSkillUnlockCallback fires when a player taps an Unlock
// button on HandleHeroSkillsPanel.
func (h *HeroHandler) HandleHeroSkillUnlockCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid request."})
	}

	var campID, faction string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}
	_ = h.DB.QueryRowContext(ctx, "SELECT faction FROM users WHERE telegram_id = $1", sender.ID).Scan(&faction)

	var heroID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM heroes WHERE encampment_id = $1", campID).Scan(&heroID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Visit /hero first to recruit a commander."})
	}

	if err := herodepth.UnlockSkill(ctx, h.DB, heroID, faction, c.Args()[0]); err != nil {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "❌ Couldn't unlock that skill."})
	}
	_ = c.Respond(&telebot.CallbackResponse{Text: "✅ Skill unlocked!"})
	return h.HandleHeroSkillsPanel(c)
}

// HandleHeroGearPanel (⚙️ Hero Gear button) shows every owned item
// with a "hero-equip" button - distinct from Phase 3's camp-level
// /gear equip, since a hero can independently carry one item for its
// own XP-gain bonus (see herodepth.HeroXPBonusPct) while the camp
// separately benefits from whatever's equipped at the camp level.
func (h *HeroHandler) HandleHeroGearPanel(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Send("⚠️ Setup your outpost camp first using /start", keyboards.MainNavigation())
	}

	panelText := "⚙️ " + htmlBold("HERO GEAR") + "\n" + divider + "\n" +
		htmlItalic("Your commander can carry one item independently of your camp's own equipped loadout - it grants an XP bonus per training session instead of a resource bonus.") + "\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	rows, err := h.DB.QueryContext(ctx, "SELECT gear_key, quantity, equipped_by_hero FROM player_gear WHERE encampment_id = $1 AND quantity > 0", campID)
	any := false
	if err == nil {
		for rows.Next() {
			var key string
			var qty int
			var byHero bool
			if scanErr := rows.Scan(&key, &qty, &byHero); scanErr == nil {
				def, ok := gear.ByKey(key)
				if !ok {
					continue
				}
				any = true
				status := ""
				if byHero {
					status = " ✅ carried by hero"
				}
				panelText += fmt.Sprintf("%s %s (+%.0f%% XP if carried)%s\n", def.Icon, htmlEscape(def.Name), def.ResourceBonusPct, status)
				if !byHero {
					buttons = append(buttons, selector.Row(selector.Data(fmt.Sprintf("⚙️ Carry %s", def.Name), "hero_gear_equip", def.Key)))
				}
			}
		}
		rows.Close()
	}
	if !any {
		panelText += htmlItalic("Nothing owned yet - visit /gear to craft or find items.") + "\n"
	}

	panelText += "\n" + divider
	selector.Inline(buttons...)
	return sendPanelWithNavHTML(c, navCaptionCamp, keyboards.CampNavigation(), panelText, selector)
}

// HandleHeroGearEquipCallback fires when a player taps a Carry button
// on HandleHeroGearPanel.
func (h *HeroHandler) HandleHeroGearEquipCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid request."})
	}
	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	if err := herodepth.EquipToHero(ctx, h.DB, campID, c.Args()[0]); err != nil {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "❌ Couldn't equip that item to your hero."})
	}
	_ = c.Respond(&telebot.CallbackResponse{Text: "⚙️ Your commander is now carrying that item!"})
	return h.HandleHeroGearPanel(c)
}
