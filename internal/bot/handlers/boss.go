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

// hpBar renders a simple 10-segment visual HP bar, SpaceHunt-style.
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

func (h *BossHandler) HandleBossPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "👹━━━━━━━━━━━━━━━━━━━━━━👹\n" +
		"☠️ WASTELAND WORLD BOSSES ☠️\n" +
		"👹━━━━━━━━━━━━━━━━━━━━━━👹\n\n" +
		"⚠️ REAL ENGAGEMENT: committing Soldiers/Mechs sends them MARCHING - they're at risk for real. The boss retaliates on arrival, and only survivors march home. Damage is shared - defeat one to split its loot pool by contribution!\n\n"

	rows, err := h.DB.QueryContext(ctx, "SELECT id, name, emoji, max_hp, current_hp, loot_pool_dollars, retaliation_rating FROM world_bosses ORDER BY max_hp ASC")
	if err != nil {
		return c.Send("⚠️ Unable to reach the World Boss tracking satellite.")
	}
	defer rows.Close()

	selector := &telebot.ReplyMarkup{}
	var buttonRows []telebot.Row

	for rows.Next() {
		var id, name, emoji string
		var maxHP, curHP, lootPool, retaliation float64
		if err := rows.Scan(&id, &name, &emoji, &maxHP, &curHP, &lootPool, &retaliation); err != nil {
			continue
		}

		status := "🟢 ACTIVE"
		if curHP <= 0 {
			status = "💀 DEFEATED (respawning...)"
		}

		panelText += fmt.Sprintf(
			"%s %s\n%s  %.0f / %.0f HP\n💰 Loot Pool: $%.0f  |  ⚔️ Danger: %.0f%%  |  %s\n\n",
			emoji, name, hpBar(curHP, maxHP), curHP, maxHP, lootPool, retaliation, status,
		)

		if curHP > 0 {
			btn := selector.Data(fmt.Sprintf("%s Attack %s", emoji, name), "attack_boss", id)
			buttonRows = append(buttonRows, selector.Row(btn))
		}
	}

	panelText += "👹━━━━━━━━━━━━━━━━━━━━━━👹"
	selector.Inline(buttonRows...)

	return c.Send(panelText, selector, keyboards.MainNavigation())
}

// HandleAttackBossCallback commits the caller's ENTIRE current standing
// soldiers+mechs garrison as an instant strike against a world boss (no
// travel time, no casualties - bosses are a cooperative PvE sink, not a
// PvP raid). Damage dealt is tracked per-player for the eventual loot split.
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
	err = tx.QueryRowContext(ctx, "SELECT current_hp, max_hp, name FROM world_bosses WHERE id = $1", bossID).Scan(&bossCurHP, &bossMaxHP, &bossName)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ That boss is no longer tracked."})
	}
	if bossCurHP <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "💀 This boss has already been defeated and is respawning!"})
	}

	// Committed troops are actually at risk now - deduct them from the
	// garrison immediately, exactly like drafting into a real raid. The
	// boss retaliates on arrival, and only survivors march home.
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

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🚀 STRIKE FORCE DEPLOYED: 🪖 %d Soldiers, 🤖 %d Mechs marching to engage %s! ETA: %.0f minutes.", soldiers, mechs, bossName, marchMinutes)})
	return h.HandleBossPanel(c)
}
