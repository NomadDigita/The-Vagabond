package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gopkg.in/telebot.v3"
)

type EtherHandler struct {
	DB *sql.DB
}

func NewEtherHandler(db *sql.DB) *EtherHandler {
	return &EtherHandler{DB: db}
}

type etherDeal struct {
	key      string
	emoji    string
	title    string
	cost     float64
	resource string
	amount   float64
}

var etherDeals = []etherDeal{
	{"ether_metal", "🔩", "Metal", 10.0, "metal", 500.0},
	{"ether_crystal", "💎", "Crystal", 20.0, "crystal", 200.0},
	{"ether_scrap", "⚙️", "Scrap", 8.0, "scrap", 400.0},
	{"ether_neuro", "🧠", "Neuro Cores", 15.0, "neuro_cores", 100.0},
	{"ether_dollars", "💵", "Cash", 12.0, "dollars", 300.0},
}

// HandleEtherShop (/ether) converts Ether - a slow-trickling passive
// resource - into a burst of other goods, matching SpaceHunt's Ether
// Shop feature.
func (h *EtherHandler) HandleEtherShop(c telebot.Context) error {
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

	var ether float64
	_ = h.DB.QueryRowContext(ctx, "SELECT ether FROM resources WHERE encampment_id = $1", campID).Scan(&ether)

	panelText := fmt.Sprintf(
		"🔮━━━━━━━━━━━━━━━━━━━━━━🔮\n"+
			"✨ THE ETHER SHOP ✨\n"+
			"🔮━━━━━━━━━━━━━━━━━━━━━━🔮\n\n"+
			"🔮 Your Ether: %.2f\n"+
			"💡 Ether trickles in slowly from Technology research over time.\n\n"+
			"⚗️ CONVERSION DEALS:\n",
		ether,
	)

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	for _, d := range etherDeals {
		panelText += fmt.Sprintf("%s %.0f Ether ➜ %.0f %s\n", d.emoji, d.cost, d.amount, d.title)
		btn := selector.Data(fmt.Sprintf("%s Convert for %s", d.emoji, d.title), "ether_convert", d.key)
		buttons = append(buttons, selector.Row(btn))
	}

	panelText += "🔮━━━━━━━━━━━━━━━━━━━━━━🔮"
	selector.Inline(buttons...)

	return c.Send(panelText, selector)
}

func (h *EtherHandler) HandleEtherConvertCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	key := c.Args()[0]
	var deal *etherDeal
	for i := range etherDeals {
		if etherDeals[i].key == key {
			deal = &etherDeals[i]
			break
		}
	}
	if deal == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Unknown deal."})
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Conversion failed."})
	}
	defer tx.Rollback()

	var ether float64
	_ = tx.QueryRowContext(ctx, "SELECT ether FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&ether)

	var tradeBeaconLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'trade_beacon'", campID).Scan(&tradeBeaconLvl)
	discount := 1.0 - (float64(tradeBeaconLvl) * 0.03)
	if discount < 0.5 {
		discount = 0.5
	}
	effectiveCost := deal.cost * discount

	if ether < effectiveCost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Ether! Need %.1f, you have %.2f.", effectiveCost, ether)})
	}

	query := fmt.Sprintf("UPDATE resources SET ether = ether - $1, %s = %s + $2 WHERE encampment_id = $3", deal.resource, deal.resource)
	_, _ = tx.ExecContext(ctx, query, effectiveCost, deal.amount, campID)

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error saving conversion."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("✨ Converted %.1f Ether into %.0f %s!", effectiveCost, deal.amount, deal.title)})
	return h.HandleEtherShop(c)
}
