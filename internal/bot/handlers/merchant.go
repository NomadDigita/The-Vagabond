package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"gopkg.in/telebot.v3"
)

type MerchantHandler struct {
	DB *sql.DB
}

func NewMerchantHandler(db *sql.DB) *MerchantHandler {
	return &MerchantHandler{DB: db}
}

func (h *MerchantHandler) HandleMerchantPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🛒 " + htmlBold("THE WANDERING MERCHANT") + "\n" + divider + "\n\n"
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	rows, err := h.DB.QueryContext(ctx, `
		SELECT id, item_type, quantity, price_dollars
		FROM merchant_offers
		WHERE purchased_by IS NULL AND expires_at > CURRENT_TIMESTAMP
		ORDER BY item_type`)

	type offer struct {
		id, itemType string
		qty          int
		price        float64
	}
	var offers []offer
	if err == nil {
		for rows.Next() {
			var o offer
			if scanErr := rows.Scan(&o.id, &o.itemType, &o.qty, &o.price); scanErr == nil {
				offers = append(offers, o)
			}
		}
		rows.Close()
	}

	if len(offers) == 0 {
		panelText += "📡 " + htmlItalic("The Merchant isn't in range right now. Rare, rotating bargains - keep an eye on server announcements.") + "\n\n"
	} else {
		panelText += htmlItalic("Steeply discounted lots, first come first served - gone once bought out or the window closes:") + "\n\n"
		var offerRows [][3]string
		for i, o := range offers {
			offerRows = append(offerRows, [3]string{
				fmt.Sprintf("[%d] %s", i+1, resourceEmoji(o.itemType)),
				htmlCode(fmt.Sprintf("%d %s", o.qty, o.itemType)),
				htmlCode(fmt.Sprintf("$%.0f", o.price)),
			})
			btnBuy := selector.Data(fmt.Sprintf("🛍️ Buy [%d]", i+1), "merchant_buy", o.id)
			buttons = append(buttons, selector.Row(btnBuy))
		}
		panelText += rankTable("Price", offerRows)
	}

	panelText += "\n" + divider
	selector.Inline(buttons...)

	return sendPanelWithNavHTML(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}

func (h *MerchantHandler) doBuyMerchantOffer(ctx context.Context, myCampID, offerID string) (string, error) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Transaction failed.", err
	}
	defer tx.Rollback()

	var itemType string
	var qty int
	var price float64
	var purchasedBy sql.NullString
	var expired bool
	if err := tx.QueryRowContext(ctx, `
		SELECT item_type, quantity, price_dollars, purchased_by, expires_at <= CURRENT_TIMESTAMP
		FROM merchant_offers WHERE id = $1 FOR UPDATE`, offerID).Scan(&itemType, &qty, &price, &purchasedBy, &expired); err != nil {
		return "❌ That offer no longer exists.", err
	}
	if purchasedBy.Valid {
		return "❌ Too slow! Someone else already bought this lot.", errors.New("offer already purchased")
	}
	if expired {
		return "❌ This offer has expired - the Merchant has moved on.", errors.New("offer expired")
	}

	itemColumn, ok := marketResourceColumn(itemType)
	if !ok {
		return "❌ This offer is no longer valid.", errors.New("unrecognized item column")
	}

	var buyerDollars float64
	_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&buyerDollars)
	if buyerDollars < price {
		return fmt.Sprintf("❌ Insufficient Cash! Need $%.0f.", price), errors.New("insufficient dollars")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", price, myCampID); err != nil {
		return "⚠️ Transaction failed.", err
	}

	var buyerItemCurrent float64
	_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1", itemColumn), myCampID).Scan(&buyerItemCurrent)
	buyerCap := storagecap.CapFor(ctx, tx, myCampID)
	newBuyerVal, _ := storagecap.Clamp(buyerItemCurrent, float64(qty), buyerCap)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", itemColumn), newBuyerVal, myCampID); err != nil {
		return "⚠️ Transaction failed.", err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE merchant_offers SET purchased_by = $1, purchased_at = CURRENT_TIMESTAMP WHERE id = $2", myCampID, offerID); err != nil {
		return "⚠️ Transaction failed.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Transaction failed to commit.", err
	}

	return fmt.Sprintf("🛍️✅ %s Bought %s %s from the Wandering Merchant for %s.",
		htmlBold("BARGAIN SECURED!"), htmlCode(fmt.Sprintf("%d", qty)), itemType, htmlCode(fmt.Sprintf("$%.0f", price))), nil
}

func (h *MerchantHandler) HandleMerchantBuyCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid purchase request."})
	}
	offerID := c.Args()[0]

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	msg, err := h.doBuyMerchantOffer(ctx, campID, offerID)
	if err != nil {
		_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: msg})
		return h.HandleMerchantPanel(c)
	}

	_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "🛍️ Bargain secured!"})
	if sendErr := c.Send(msg, telebot.ModeHTML, keyboards.EconomyNavigation()); sendErr != nil {
		return sendErr
	}
	return h.HandleMerchantPanel(c)
}
