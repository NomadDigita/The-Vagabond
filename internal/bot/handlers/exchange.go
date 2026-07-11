package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type ExchangeHandler struct {
	DB *sql.DB
}

func NewExchangeHandler(db *sql.DB) *ExchangeHandler {
	return &ExchangeHandler{DB: db}
}

func (h *ExchangeHandler) HandleExchangePanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	query := `
		SELECT m.id, e.name, m.item_type, m.quantity, m.price_dollars 
		FROM market_exchange m
		JOIN encampments e ON e.id = m.seller_id
		WHERE m.is_sold = FALSE 
		LIMIT 3`
	
	rows, err := h.DB.QueryContext(ctx, query)
	var listingsText string
	var buttons []telebot.Row
	selector := &telebot.ReplyMarkup{}

	if err != nil {
		log.Printf("Failed scanning exchange listings: %v", err)
		listingsText = "📡 Static: Connection interrupted."
	} else {
		defer rows.Close()
		index := 1
		for rows.Next() {
			var listID, sellerName, itemType string
			var qty int
			var price float64
			if err := rows.Scan(&listID, &sellerName, &itemType, &qty, &price); err == nil {
				listingsText += fmt.Sprintf("[%d] Outpost: %s\n    Item: %d %s | Price: $%0.f\n\n", index, sellerName, qty, itemType, price)
				btnBuy := selector.Data(fmt.Sprintf("🛍️ Buy [%d]", index), "buy_listing", listID)
				buttons = append(buttons, selector.Row(btnBuy))
				index++
			}
		}
		if listingsText == "" {
			listingsText = "📋 Board Clean: No active player listings currently on exchange.\n\n"
		}
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"💱 PLAYER AUCTION MARKET EXCHANGE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Buy raw tactical stockpiles listed by other active outposts:\n\n"+
			"%s"+
			"POST NEW LISTING:\n"+
			"🧱 [Sell 50 Metal] — List on exchange for $150 Cash\n"+
			"☢️ [Sell 20 Crystal] — List on exchange for $300 Cash\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		listingsText,
	)

	btnPostSteel := selector.Data("🧱 List 50 Metal ($150)", "post_listing", "metal")
	btnPostUranium := selector.Data("☢️ List 20 Crystal ($300)", "post_listing", "crystal")

	buttons = append(buttons, selector.Row(btnPostSteel, btnPostUranium))
	selector.Inline(buttons...)

	return c.Send(panelText, selector)
}

func (h *ExchangeHandler) HandlePostListingCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	item := c.Args()[0]

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Listing failed."})
	}
	defer tx.Rollback()

	var metal, crystal float64
	_ = tx.QueryRowContext(ctx, "SELECT metal, crystal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&metal, &crystal)

	qty := 50
	price := 150.0

	switch item {
	case "metal":
		if metal < 50.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Metal! Need 50 tons to list."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 50.0 WHERE encampment_id = $1", campID)
		qty = 50
		price = 150.0

	case "crystal":
		if crystal < 20.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Crystal! Need 20 kg to list."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - 20.0 WHERE encampment_id = $1", campID)
		qty = 20
		price = 300.0
	}

	query := `
		INSERT INTO market_exchange (seller_id, item_type, quantity, price_dollars, is_sold) 
		VALUES ($1, $2, $3, $4, FALSE)`
	_, err = tx.ExecContext(ctx, query, campID, item, qty, price)
	if err != nil {
		log.Printf("Failed executing market exchange insert: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing marketplace listings."})
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "💱 Listing posted successfully on exchange!"})
	return h.HandleExchangePanel(c)
}

func (h *ExchangeHandler) HandleBuyListingCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	listingID := c.Args()[0]

	var myCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var sellerID string
	var itemType string
	var qty int
	var price float64
	var isSold bool

	query := `
		SELECT seller_id, item_type, quantity, price_dollars, is_sold 
		FROM market_exchange 
		WHERE id = $1 FOR UPDATE`
	
	err = tx.QueryRowContext(ctx, query, listingID).Scan(&sellerID, &itemType, &qty, &price, &isSold)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Expired: This listing is no longer available."})
	}

	if isSold {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already sold."})
	}

	if sellerID == myCampID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You can't buy your own listing."})
	}

	var dollars float64
	_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&dollars)

	if dollars < price {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Cash! Need $%.0f.", price)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", price, myCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars + $1 WHERE encampment_id = $2", price, sellerID)

	columnName := "metal"
	if itemType == "crystal" {
		columnName = "crystal"
	}
	queryTransfer := fmt.Sprintf("UPDATE resources SET %s = %s + $1 WHERE encampment_id = $2", columnName, columnName)
	_, _ = tx.ExecContext(ctx, queryTransfer, qty, myCampID)

	_, _ = tx.ExecContext(ctx, "UPDATE market_exchange SET is_sold = TRUE WHERE id = $1", listingID)

	var sellerUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", sellerID).Scan(&sellerUserID)
	alertMsg := fmt.Sprintf("💱 MARKET SALE: Another player has purchased your public listing for %d %s. +$%.0f transferred instantly to reserves.", qty, itemType, price)
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sellerUserID, alertMsg)

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🛍️ Materials acquired successfully!"})

	return h.HandleExchangePanel(c)
}