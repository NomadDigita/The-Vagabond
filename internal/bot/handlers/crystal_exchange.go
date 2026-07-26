package handlers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"gopkg.in/telebot.v3"
)

// crystalConversionRate is how much of a resource 1 Crystal converts into.
// Crystal is the rarest and most valuable resource in the game (see
// MMO_WORLD_EVOLUTION_PLAN.md), so every rate here is deliberately large -
// this is meant to be a genuine "cash out my rarest resource for a big
// stockpile of whatever I'm actually short on" lever, not a rounding-error
// trade. Rates scale with how commonly needed the target resource is:
// resources every player burns constantly (Scrap, Metal, Rations) convert
// generously; resources that are already scarce or high-value in their own
// right (Neuro Cores, Ether) convert conservatively, since they don't need
// the help and a generous rate there would just let Crystal launder into
// a second rare currency for free.
type crystalConversion struct {
	resourceKey string
	label       string
	emoji       string
	rate        float64 // amount of resourceKey per 1 Crystal
}

var crystalConversionTable = []crystalConversion{
	{"scrap", "Scrap", "⚙️", 12000},
	{"metal", "Metal", "🔩", 10000},
	{"rations", "Rations", "🍞", 8000},
	{"electricity", "Electricity", "⚡", 5000},
	{"hydrogen", "Hydrogen", "💧", 3000},
	{"dollars", "Dollars", "💵", 2000},
	{"neuro_cores", "Neuro-Cores", "🧠", 500},
	{"ether", "Ether", "✨", 100},
}

// HandleCrystalExchangePanel shows the player's Crystal balance and one
// button per convertible resource, offering fixed 1/5/25-Crystal presets so
// the conversion amount never needs free-text input.
func (h *EconomyHandler) HandleCrystalExchangePanel(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Invalid sender."})
	}

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	var crystal float64
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(crystal,0) FROM resources WHERE encampment_id = $1", campID).Scan(&crystal)

	text := fmt.Sprintf(
		"🔮━━━━━━━━━━━━━━━━━━━━━━🔮\n"+
			"CRYSTAL EXCHANGE\n"+
			"🔮━━━━━━━━━━━━━━━━━━━━━━🔮\n\n"+
			"💎 Your Crystal: %.2f\n\n"+
			"Crystal is the rarest resource in the world. Convert it into a "+
			"large stockpile of any other resource - the rate depends on how "+
			"commonly that resource is needed; everyday materiel converts "+
			"generously, other rare currencies conservatively.\n\n"+
			"Rates (per 1 💎 Crystal):\n",
		crystal,
	)
	for _, conv := range crystalConversionTable {
		text += fmt.Sprintf("%s %s: %.0f\n", conv.emoji, conv.label, conv.rate)
	}
	text += "\n🔮━━━━━━━━━━━━━━━━━━━━━━🔮"

	selector := &telebot.ReplyMarkup{}
	var rows []telebot.Row
	for _, conv := range crystalConversionTable {
		btn1 := selector.Data(fmt.Sprintf("%s x1", conv.emoji), "crystal_exchange", conv.resourceKey, "1")
		btn5 := selector.Data(fmt.Sprintf("%s x5", conv.emoji), "crystal_exchange", conv.resourceKey, "5")
		btn25 := selector.Data(fmt.Sprintf("%s x25", conv.emoji), "crystal_exchange", conv.resourceKey, "25")
		rows = append(rows, selector.Row(btn1, btn5, btn25))
	}
	selector.Inline(rows...)

	return c.Send(text, selector)
}

// HandleCrystalExchangeCallback performs the conversion: resourceKey|amount.
func (h *EconomyHandler) HandleCrystalExchangeCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid exchange request."})
	}
	resourceKey := c.Args()[0]
	crystalAmount, err := strconv.ParseFloat(c.Args()[1], 64)
	if err != nil || crystalAmount <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid Crystal amount."})
	}

	var conv *crystalConversion
	for i := range crystalConversionTable {
		if crystalConversionTable[i].resourceKey == resourceKey {
			conv = &crystalConversionTable[i]
			break
		}
	}
	if conv == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown resource."})
	}

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error resolving Outpost."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var crystal float64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(crystal,0) FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&crystal); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Could not read Crystal balance."})
	}
	if crystal < crystalAmount {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Crystal: you have %.2f, need %.2f.", crystal, crystalAmount)})
	}

	payout := conv.rate * crystalAmount
	storageCap := storagecap.CapFor(ctx, tx, campID)

	var current float64
	// resourceKey is only ever one of the fixed literals in
	// crystalConversionTable above (never taken verbatim from outside
	// that table), so it is safe to interpolate into the column list.
	query := fmt.Sprintf("SELECT COALESCE(%s,0) FROM resources WHERE encampment_id = $1 FOR UPDATE", conv.resourceKey)
	_ = tx.QueryRowContext(ctx, query, campID).Scan(&current)

	newAmount, discarded := storagecap.Clamp(current, payout, storageCap)
	actualGain := newAmount - current
	wasClamped := discarded > 0

	updateQuery := fmt.Sprintf("UPDATE resources SET crystal = crystal - $1, %s = $2 WHERE encampment_id = $3", conv.resourceKey)
	if _, err := tx.ExecContext(ctx, updateQuery, crystalAmount, newAmount, campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Exchange failed."})
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Exchange failed to commit."})
	}

	respText := fmt.Sprintf("🔮 Converted %.2f Crystal into %s %.0f %s!", crystalAmount, conv.emoji, actualGain, conv.label)
	if wasClamped {
		respText += "\n⚠️ Warehouse capacity capped the gain - some value was lost. Upgrade storage before converting large batches."
	}
	_ = c.Respond(&telebot.CallbackResponse{Text: "🔮 Exchange complete!"})
	return c.Send(respText, keyboards.MainNavigation())
}
