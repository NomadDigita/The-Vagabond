package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"gopkg.in/telebot.v3"
)

type RebellionHandler struct {
	DB *sql.DB
}

func NewRebellionHandler(db *sql.DB) *RebellionHandler {
	return &RebellionHandler{DB: db}
}

// renownRank maps total lifetime contribution to a SpaceHunt-style
// Rebellion standing title.
func renownRank(total float64) (string, string) {
	switch {
	case total >= 100000:
		return "🌟", "Rebel Hero"
	case total >= 25000:
		return "⚔️", "Vanguard"
	case total >= 5000:
		return "🎖️", "Operative"
	case total >= 1000:
		return "🤝", "Sympathizer"
	default:
		return "🔰", "Recruit"
	}
}

const donationAmount = 250.0

func (h *RebellionHandler) HandleRebellionPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var scrap float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1", campID).Scan(&scrap)

	var totalContributed float64
	_ = h.DB.QueryRowContext(ctx, "SELECT total_contributed FROM rebellion_support WHERE encampment_id = $1", campID).Scan(&totalContributed)

	rankEmoji, rankTitle := renownRank(totalContributed)

	panelText := fmt.Sprintf(
		"🚩━━━━━━━━━━━━━━━━━━━━━━🚩\n"+
			"✊ CONTACT THE REBELLION ✊\n"+
			"🚩━━━━━━━━━━━━━━━━━━━━━━🚩\n\n"+
			"\"The Old World fell, but we endure. Support the Rebellion's cause with Scrap, and we'll share what intel and salvaged tech we can spare.\"\n\n"+
			"%s YOUR STANDING: %s\n"+
			"📊 Lifetime Contribution: %.0f Scrap\n"+
			"♻️ Your Scrap Reserves: %.0f\n\n"+
			"💰 Donate %.0f Scrap ➜ Receive 🧠 %.0f Neuro Cores + Renown\n\n"+
			"🏆 TOP REBELLION SUPPORTERS:\n",
		rankEmoji, rankTitle, totalContributed, scrap, donationAmount, donationAmount*0.10,
	)

	rows, err := h.DB.QueryContext(ctx, `
		SELECT e.name, rs.total_contributed
		FROM rebellion_support rs
		JOIN encampments e ON e.id = rs.encampment_id
		ORDER BY rs.total_contributed DESC
		LIMIT 5`)
	if err == nil {
		rank := 1
		for rows.Next() {
			var name string
			var contrib float64
			if scanErr := rows.Scan(&name, &contrib); scanErr == nil {
				medal := medalFor(rank)
				emoji, title := renownRank(contrib)
				panelText += fmt.Sprintf("%s %d. %s — %s %s (%.0f)\n", medal, rank, name, emoji, title, contrib)
				rank++
			}
		}
		rows.Close()
	}

	panelText += "\n🚩━━━━━━━━━━━━━━━━━━━━━━🚩"

	selector := &telebot.ReplyMarkup{}
	btnDonate := selector.Data(fmt.Sprintf("💰 Donate %.0f Scrap", donationAmount), "rebellion_donate")
	selector.Inline(selector.Row(btnDonate))

	return c.Send(panelText, selector)
}

func (h *RebellionHandler) HandleRebellionDonateCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transmission failed."})
	}
	defer tx.Rollback()

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)

	if scrap < donationAmount {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Scrap! Need %.0f.", donationAmount)})
	}

	reward := donationAmount * 0.10

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1, neuro_cores = neuro_cores + $2 WHERE encampment_id = $3", donationAmount, reward, campID)
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO rebellion_support (encampment_id, total_contributed) VALUES ($1, $2)
		ON CONFLICT (encampment_id) DO UPDATE SET total_contributed = rebellion_support.total_contributed + $2`,
		campID, donationAmount)

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing rebellion donation: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error recording donation."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("✊ The Rebellion thanks you! +%.0f Neuro Cores received.", reward)})
	return h.HandleRebellionPanel(c)
}
