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

type BossHandler struct {
	DB *sql.DB
}

func NewBossHandler(db *sql.DB) *BossHandler {
	return &BossHandler{DB: db}
}

func hpBar(current, max float64) string {
	if max <= 0 {
		return "▱▱▱▱▱▱▱▱▱▱"
	}
	pct := current / max
	if pct < 0 {
		pct = 0
	}
	filled := int(pct * 10)
	bar := ""
	for i := 0; i < 10; i++ {
		if i < filled {
			bar += "▰"
		} else {
			bar += "▱"
		}
	}
	return bar
}

const (
	tier2KillRequirement = 5
	tier3KillRequirement = 3
)

func (h *BossHandler) tierUnlocked(ctx context.Context, tier int) (unlocked bool, lowerTierKills, required int) {
	if tier <= 1 {
		return true, 0, 0
	}
	required = tier2KillRequirement
	if tier == 3 {
		required = tier3KillRequirement
	}
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(total_kills), 0) FROM world_bosses WHERE tier = $1", tier-1).Scan(&lowerTierKills)
	return lowerTierKills >= required, lowerTierKills, required
}

func (h *BossHandler) HandleBossPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "☠️ " + htmlBold("WASTELAND WORLD BOSSES") + " ☠️\n" + divider + "\n\n" +
		htmlItalic("⚠️ REAL ENGAGEMENT: committing Soldiers/Mechs sends them MARCHING - they're at risk for real. The boss retaliates on arrival, and only survivors march home. Damage is shared - defeat one to split its loot pool by contribution!") + "\n\n"

	rows, err := h.DB.QueryContext(ctx, "SELECT id, name, emoji, max_hp, current_hp, loot_pool_dollars, retaliation_rating, tier, is_gauntlet, enrage_threshold_pct FROM world_bosses ORDER BY tier ASC, max_hp ASC")
	if err != nil {
		return c.Send("⚠️ Unable to reach the World Boss tracking satellite.")
	}
	defer rows.Close()

	selector := &telebot.ReplyMarkup{}
	var buttonRows []telebot.Row

	for rows.Next() {
		var id, name, emoji string
		var maxHP, curHP, lootPool, retaliation, enrageThreshold float64
		var tier int
		var isGauntlet bool
		if err := rows.Scan(&id, &name, &emoji, &maxHP, &curHP, &lootPool, &retaliation, &tier, &isGauntlet, &enrageThreshold); err != nil {
			continue
		}

		unlocked, lowerKills, required := h.tierUnlocked(ctx, tier)
		tierLabel := fmt.Sprintf("Tier %d", tier)
		if isGauntlet {
			tierLabel += " 🔥 GAUNTLET"
		}

		if !unlocked {
			lockedMsg := fmt.Sprintf("Locked - requires %d/%d Tier %d kills server-wide first.", lowerKills, required, tier-1)
			panelText += fmt.Sprintf("%s %s (%s)\n🔒 %s\n\n", emoji, htmlBold(name), tierLabel, htmlItalic(lockedMsg))
			continue
		}

		status := "🟢 ACTIVE"
		if curHP <= 0 {
			status = "💀 DEFEATED (respawning...)"
		}

		dropInfo := "Chance-based Epic/Legendary drop on kill"
		if isGauntlet {
			dropInfo = fmt.Sprintf("🔥 Enrages below %.0f%% HP (2x retaliation) - GUARANTEED Epic/Legendary drop on kill", enrageThreshold)
		}

		panelText += fmt.Sprintf(
			"%s %s (%s)\n%s\n💰 Loot Pool: %s  |  ⚔️ Danger: %s  |  %s\n%s\n\n",
			emoji, htmlBold(name), tierLabel, htmlCode(fmt.Sprintf("%s %.0f/%.0f HP", hpBar(curHP, maxHP), curHP, maxHP)),
			htmlCode(fmt.Sprintf("$%.0f", lootPool)), htmlCode(fmt.Sprintf("%.0f%%", retaliation)), status,
			htmlItalic(dropInfo),
		)

		if curHP > 0 {
			btn := selector.Data(fmt.Sprintf("%s Attack %s", emoji, name), "attack_boss", id)
			buttonRows = append(buttonRows, selector.Row(btn))
		}
	}

	panelText += divider
	selector.Inline(buttonRows...)

	return sendPanelWithNavHTML(c, navCaptionMain, keyboards.MainNavigation(), panelText, selector)
}

func (h *BossHandler) HandleAttackBossCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	bossID := c.Args()[0]

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Strike failed."})
	}
	defer tx.Rollback()

	var soldiers, mechs int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers,0), COALESCE(mechs,0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&soldiers, &mechs)

	if soldiers+mechs <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You have no Soldiers or Mechs garrisoned to commit to the strike!"})
	}

	var bossCurHP, bossMaxHP float64
	var bossName string
	var bossTier int
	err = tx.QueryRowContext(ctx, "SELECT current_hp, max_hp, name, tier FROM world_bosses WHERE id = $1", bossID).Scan(&bossCurHP, &bossMaxHP, &bossName, &bossTier)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ That boss is no longer tracked."})
	}
	if unlocked, _, _ := h.tierUnlocked(ctx, bossTier); !unlocked {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "🔒 This Tier isn't unlocked yet - defeat more lower-tier bosses server-wide first."})
	}
	if bossCurHP <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "💀 This boss has already been defeated and is respawning!"})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers - $1, mechs = mechs - $2 WHERE encampment_id = $3", soldiers, mechs, campID)

	const marchMinutes = 8.0
	resolveTime := time.Now().UTC().Add(marchMinutes * time.Minute)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO world_boss_attacks (boss_id, user_id, encampment_id, soldiers_committed, mechs_committed, state, resolve_time, march_minutes)
		VALUES ($1, $2, $3, $4, $5, 'marching', $6, $7)`,
		bossID, sender.ID, campID, soldiers, mechs, resolveTime, marchMinutes)
	if err != nil {
		log.Printf("Failed creating world boss attack: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error deploying strike force."})
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing boss attack launch: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error recording strike."})
	}

	_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: fmt.Sprintf("🚀 STRIKE FORCE DEPLOYED: 🪖 %d Soldiers, 🤖 %d Mechs marching to engage %s! ETA: %.0f minutes.", soldiers, mechs, bossName, marchMinutes)})
	return h.HandleBossPanel(c)
}
