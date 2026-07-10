package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

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
		"Rally your Soldiers and Mechs against these roaming colossi. Damage is shared - defeat one to split its loot pool by contribution!\n\n"

	rows, err := h.DB.QueryContext(ctx, "SELECT id, name, emoji, max_hp, current_hp, loot_pool_dollars FROM world_bosses ORDER BY max_hp ASC")
	if err != nil {
		return c.Send("⚠️ Unable to reach the World Boss tracking satellite.")
	}
	defer rows.Close()

	selector := &telebot.ReplyMarkup{}
	var buttonRows []telebot.Row

	for rows.Next() {
		var id, name, emoji string
		var maxHP, curHP, lootPool float64
		if err := rows.Scan(&id, &name, &emoji, &maxHP, &curHP, &lootPool); err != nil {
			continue
		}

		status := "🟢 ACTIVE"
		if curHP <= 0 {
			status = "💀 DEFEATED (respawning...)"
		}

		panelText += fmt.Sprintf(
			"%s %s\n%s  %.0f / %.0f HP\n💰 Loot Pool: $%.0f  |  %s\n\n",
			emoji, name, hpBar(curHP, maxHP), curHP, maxHP, lootPool, status,
		)

		if curHP > 0 {
			btn := selector.Data(fmt.Sprintf("%s Attack %s", emoji, name), "attack_boss", id)
			buttonRows = append(buttonRows, selector.Row(btn))
		}
	}

	panelText += "👹━━━━━━━━━━━━━━━━━━━━━━👹"
	selector.Inline(buttonRows...)

	return c.Send(panelText, selector)
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

	damage := float64(soldiers)*10.0 + float64(mechs)*400.0

	var curHP, maxHP, lootPool float64
	var bossName string
	err = tx.QueryRowContext(ctx, "SELECT current_hp, max_hp, loot_pool_dollars, name FROM world_bosses WHERE id = $1 FOR UPDATE", bossID).Scan(&curHP, &maxHP, &lootPool, &bossName)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ That boss is no longer tracked."})
	}
	if curHP <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "💀 This boss has already been defeated and is respawning!"})
	}

	newHP := curHP - damage
	killingBlow := newHP <= 0
	if newHP < 0 {
		newHP = 0
	}
	_, _ = tx.ExecContext(ctx, "UPDATE world_bosses SET current_hp = $1 WHERE id = $2", newHP, bossID)

	_, _ = tx.ExecContext(ctx, `
		INSERT INTO world_boss_contributions (boss_id, user_id, encampment_id, damage_dealt)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (boss_id, user_id) DO UPDATE SET damage_dealt = world_boss_contributions.damage_dealt + $4`,
		bossID, sender.ID, campID, damage)

	respMsg := fmt.Sprintf("💥 STRIKE LANDED: %.0f damage dealt to %s! (%.0f/%.0f HP remaining)", damage, bossName, newHP, maxHP)

	if killingBlow {
		if err := h.payoutBossLoot(ctx, tx, bossID, bossName, lootPool); err != nil {
			log.Printf("Boss loot payout failed: %v", err)
		}
		respMsg = fmt.Sprintf("☠️🎉 %s HAS BEEN SLAIN! Loot pool of $%.0f split among all contributors!", bossName, lootPool)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing boss attack: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error recording strike."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: respMsg})
	return h.HandleBossPanel(c)
}

// payoutBossLoot splits the loot pool proportional to each contributor's
// total damage dealt, resets the boss to full HP with a refreshed loot
// pool (10% larger, for long-term progression), and clears contributions.
func (h *BossHandler) payoutBossLoot(ctx context.Context, tx *sql.Tx, bossID, bossName string, lootPool float64) error {
	rows, err := tx.QueryContext(ctx, "SELECT user_id, encampment_id, damage_dealt FROM world_boss_contributions WHERE boss_id = $1", bossID)
	if err != nil {
		return err
	}

	type contributor struct {
		userID int64
		campID string
		damage float64
	}
	var contributors []contributor
	var totalDamage float64
	for rows.Next() {
		var ct contributor
		if scanErr := rows.Scan(&ct.userID, &ct.campID, &ct.damage); scanErr == nil {
			contributors = append(contributors, ct)
			totalDamage += ct.damage
		}
	}
	rows.Close()

	if totalDamage > 0 {
		for _, ct := range contributors {
			share := lootPool * (ct.damage / totalDamage)
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars + $1 WHERE encampment_id = $2", share, ct.campID)
			alertMsg := fmt.Sprintf("☠️🎉 BOSS SLAIN: %s\n\nYour squad dealt %.0f damage! You received 💵 $%.2f from the loot pool.", bossName, ct.damage, share)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ct.userID, alertMsg)
		}
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM world_boss_contributions WHERE boss_id = $1", bossID)
	_, _ = tx.ExecContext(ctx, `
		UPDATE world_bosses 
		SET current_hp = max_hp, loot_pool_dollars = loot_pool_dollars * 1.10, last_defeated_at = CURRENT_TIMESTAMP 
		WHERE id = $1`, bossID)

	return nil
}
