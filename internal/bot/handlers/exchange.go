package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
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
		SELECT m.id, e.name, m.item_type, m.quantity, m.ask_type, m.ask_quantity
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
		listingsText = "📡 " + htmlItalic("Static: Connection interrupted.")
	} else {
		defer rows.Close()
		index := 1
		for rows.Next() {
			var listID, sellerName, itemType, askType string
			var qty int
			var askQty float64
			if err := rows.Scan(&listID, &sellerName, &itemType, &qty, &askType, &askQty); err == nil {
				listingsText += fmt.Sprintf("🏷️ [%d] Outpost: %s\n    %s %s | %s Asking: %s\n\n",
					index, htmlBold(htmlEscape(sellerName)), resourceEmoji(itemType), htmlCode(fmt.Sprintf("%d %s", qty, itemType)),
					askEmoji(askType), htmlCode(formatAsk(askType, askQty)))
				btnBuy := selector.Data(fmt.Sprintf("🛍️ Buy [%d]", index), "buy_listing", listID)
				buttons = append(buttons, selector.Row(btnBuy))
				index++
			}
		}
		if listingsText == "" {
			listingsText = "📋 " + htmlItalic("Board Clean: No active player listings currently on exchange.") + "\n\n"
		}
	}

	panelText := fmt.Sprintf(
		"💱 %s\n"+divider+"\n"+
			"%s\n\n"+
			"%s"+
			"%s\n"+
			"🔩 Sell 50 Metal — List on exchange for $150 Cash\n"+
			"☢️ Sell 20 Crystal — List on exchange for $300 Cash\n"+
			"💬 Or just type it, e.g. %s to barter directly for another resource instead of cash.\n"+
			divider,
		htmlBold("PLAYER AUCTION MARKET EXCHANGE"),
		htmlItalic("Buy raw tactical stockpiles listed by other active outposts:"),
		listingsText,
		htmlBold("📮 POST NEW LISTING"),
		htmlCode("list 300k scrap for 40k metal"),
	)

	btnPostSteel := selector.Data("🔩 List 50 Metal ($150)", "post_listing", "metal")
	btnPostUranium := selector.Data("☢️ List 20 Crystal ($300)", "post_listing", "crystal")

	buttons = append(buttons, selector.Row(btnPostSteel, btnPostUranium))
	selector.Inline(buttons...)

	return sendPanelWithNavHTML(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}

// validMarketResources are the resource columns tradeable through the
// public market exchange, keyed by their lowercase name as used
// everywhere else in this codebase (resourceEmoji, etc). Kept as an
// explicit allow-list rather than accepting any resources column name
// straight from a caller-controlled string, so the fmt.Sprintf column
// interpolation below can never receive anything but one of these
// three hardcoded literals.
var validMarketResources = map[string]string{
	"metal":   "metal",
	"crystal": "crystal",
	"scrap":   "scrap",
}

// marketResourceColumn resolves a (case-insensitive) resource name to
// its resources table column, reporting ok=false for anything not on
// the exchange allow-list.
func marketResourceColumn(resource string) (string, bool) {
	col, ok := validMarketResources[strings.ToLower(strings.TrimSpace(resource))]
	return col, ok
}

// capitalizeWord uppercases just the first byte - good enough for the
// small set of all-ASCII words this package ever needs to display
// (Metal/Crystal/Scrap/Dollars), without pulling in the now-deprecated
// strings.Title for such a small job.
func capitalizeWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatAsk renders what a listing is asking for in return - a dollar
// price ("$500") or a quantity of another resource ("40000 metal") -
// for consistent display across the panel, confirmation cards, and
// buyer/seller notifications.
func formatAsk(askType string, askQty float64) string {
	if askType == "dollars" {
		return fmt.Sprintf("$%.0f", askQty)
	}
	return fmt.Sprintf("%.0f %s", askQty, askType)
}

// askEmoji is resourceEmoji's counterpart for the "asking" side of a
// listing, which can be "dollars" (not itself a tradeable resource
// column) as well as any resource on the allow-list.
func askEmoji(askType string) string {
	if askType == "dollars" {
		return "💵"
	}
	return resourceEmoji(askType)
}

// doPostListing is the testable core shared by HandlePostListingCallback's
// two fixed quick-list buttons and the natural-language "list X for
// sale"/"list X for Y" command (see FEEDBACK_CHANGELOG_NLP_PLAN.md
// Milestone 3) - one code path so the AI-parsed flow enforces the
// exact same balance checks as the button-driven UI, matching
// doDispatchScoutMission's established convention.
//
// askType is either "dollars" (a cash listing, askQty is the dollar
// price - the original and still most common case) or another
// tradeable resource name (a barter listing, askQty is how much of
// that resource is wanted in return - askType must differ from
// resource, since bartering a resource for itself is meaningless).
//
// Returns the HTML-formatted message to show and, on failure, a
// non-nil error alongside a plain-text failure notice.
func (h *ExchangeHandler) doPostListing(ctx context.Context, campID string, resource string, qty int, askType string, askQty float64) (string, error) {
	column, ok := marketResourceColumn(resource)
	if !ok {
		return fmt.Sprintf("❌ %s isn't tradeable on the exchange - try Metal, Crystal, or Scrap.", htmlEscape(capitalizeWord(resource))), errors.New("unsupported resource")
	}
	if qty <= 0 {
		return "❌ Quantity must be positive.", errors.New("invalid quantity")
	}

	askType = strings.ToLower(strings.TrimSpace(askType))
	if askType == "" {
		askType = "dollars"
	}
	if askType != "dollars" {
		askColumn, ok := marketResourceColumn(askType)
		if !ok {
			return fmt.Sprintf("❌ Can't ask for %s in return - try Dollars, Metal, Crystal, or Scrap.", htmlEscape(capitalizeWord(askType))), errors.New("unsupported ask type")
		}
		if askColumn == column {
			return fmt.Sprintf("❌ Can't barter %s for itself - pick a different resource to ask for.", htmlEscape(capitalizeWord(column))), errors.New("ask type matches listed resource")
		}
	}
	if askQty <= 0 {
		return "❌ Asking amount must be positive.", errors.New("invalid asking amount")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Listing failed.", err
	}
	defer tx.Rollback()

	var current float64
	if err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1 FOR UPDATE", column), campID).Scan(&current); err != nil {
		return "⚠️ Error reading your reserves.", err
	}
	if current < float64(qty) {
		return fmt.Sprintf("❌ Insufficient %s! You have %s, need %s to list.",
			capitalizeWord(column), htmlCode(fmt.Sprintf("%.0f", current)), htmlCode(fmt.Sprintf("%d", qty))), errors.New("insufficient resource")
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = %s - $1 WHERE encampment_id = $2", column, column), qty, campID); err != nil {
		return "⚠️ Error committing listing.", err
	}

	// price_dollars is kept populated for dollar listings (0 for
	// barter listings) so existing readers of that column - the AI
	// faction auto-buy tick, econadvisor's market stats - keep working
	// against dollar listings unchanged; see the ask_type/ask_quantity
	// doc comment in internal/db/schema/schema.go.
	priceDollarsForRow := 0.0
	if askType == "dollars" {
		priceDollarsForRow = askQty
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO market_exchange (seller_id, item_type, quantity, price_dollars, ask_type, ask_quantity, is_sold) VALUES ($1, $2, $3, $4, $5, $6, FALSE)`,
		campID, column, qty, priceDollarsForRow, askType, askQty); err != nil {
		log.Printf("Failed executing market exchange insert: %v", err)
		return "⚠️ Error writing marketplace listing.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Listing failed.", err
	}

	return "💱 " + htmlBold("LISTING POSTED") + "\n" + divider + "\n" +
		fmt.Sprintf("%s %s listed for %s\n", resourceEmoji(column), htmlCode(fmt.Sprintf("%d %s", qty, column)), htmlCode(formatAsk(askType, askQty))) +
		divider, nil
}

func (h *ExchangeHandler) HandlePostListingCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid sender."})
	}

	item := c.Args()[0]

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	// The panel only ever offers these two quick-list quantities/prices
	// (see HandleExchangePanel's btnPostSteel/btnPostUranium) - the
	// natural-language flow posts arbitrary quantities/asks through
	// the same doPostListing core below instead.
	qty := 50
	price := 150.0
	if item == "crystal" {
		qty = 20
		price = 300.0
	}

	if _, err := h.doPostListing(ctx, campID, item, qty, "dollars", price); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Listing failed - see the panel for details."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "💱 Listing posted successfully on exchange!"})
	return h.HandleExchangePanel(c)
}

// doBuyListing is the testable core shared by HandleBuyListingCallback
// (buying a specific listing the player tapped in the exchange panel)
// and doBuyMarketItem below (buying a listing found by searching, for
// the natural-language "buy X" command - see
// FEEDBACK_CHANGELOG_NLP_PLAN.md Milestone 3) - one code path so both
// enforce the exact same balance/ownership/expiry checks, matching
// doPostListing's established convention. Unchanged logic from the
// pre-2026-08-05 HandleBuyListingCallback, just extracted so it no
// longer needs a telebot.Context.
//
// Returns a plain-text or HTML-safe message (caller decides how to
// send it) and, on failure, a non-nil error.
func (h *ExchangeHandler) doBuyListing(ctx context.Context, myCampID, listingID string) (string, error) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Transaction failed.", err
	}
	defer tx.Rollback()

	var sellerID, itemType, askType string
	var qty int
	var askQty float64
	var isSold bool

	query := `
		SELECT seller_id, item_type, quantity, ask_type, ask_quantity, is_sold
		FROM market_exchange 
		WHERE id = $1 FOR UPDATE`

	if err := tx.QueryRowContext(ctx, query, listingID).Scan(&sellerID, &itemType, &qty, &askType, &askQty, &isSold); err != nil {
		return "❌ Expired: This listing is no longer available.", err
	}

	if isSold {
		return "❌ Already sold.", errors.New("listing already sold")
	}
	if sellerID == myCampID {
		return "❌ You can't buy your own listing.", errors.New("cannot buy own listing")
	}

	itemColumn, ok := marketResourceColumn(itemType)
	if !ok {
		// Fail safely rather than fmt.Sprintf-ing an unrecognized
		// column name into a query - shouldn't happen for anything
		// listed through doPostListing, but a defensive floor for any
		// older/out-of-band row.
		return "❌ This listing is no longer valid.", errors.New("unrecognized item column")
	}

	askIsDollars := askType == "dollars"
	var askColumn string
	if !askIsDollars {
		col, ok := marketResourceColumn(askType)
		if !ok {
			return "❌ This listing is no longer valid.", errors.New("unrecognized ask column")
		}
		askColumn = col
	}

	// --- Buyer pays the ask (dollars or a bartered resource) ---
	if askIsDollars {
		var buyerDollars float64
		_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&buyerDollars)
		if buyerDollars < askQty {
			return fmt.Sprintf("❌ Insufficient Cash! Need $%.0f.", askQty), errors.New("insufficient dollars")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", askQty, myCampID); err != nil {
			return "⚠️ Transaction failed.", err
		}
	} else {
		var buyerAskBalance float64
		_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1 FOR UPDATE", askColumn), myCampID).Scan(&buyerAskBalance)
		if buyerAskBalance < askQty {
			return fmt.Sprintf("❌ Insufficient %s! Need %s.", capitalizeWord(askColumn), htmlCode(fmt.Sprintf("%.0f", askQty))), errors.New("insufficient ask resource")
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = %s - $1 WHERE encampment_id = $2", askColumn, askColumn), askQty, myCampID); err != nil {
			return "⚠️ Transaction failed.", err
		}
	}

	// --- Seller receives the ask, storage-cap clamped like every other resource-gain path ---
	sellerCap := storagecap.CapFor(ctx, tx, sellerID)
	if askIsDollars {
		var sellerDollars float64
		_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", sellerID).Scan(&sellerDollars)
		newSellerDollars, _ := storagecap.Clamp(sellerDollars, askQty, sellerCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newSellerDollars, sellerID)
	} else {
		var sellerAskCurrent float64
		_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1 FOR UPDATE", askColumn), sellerID).Scan(&sellerAskCurrent)
		newSellerAskVal, _ := storagecap.Clamp(sellerAskCurrent, askQty, sellerCap)
		_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", askColumn), newSellerAskVal, sellerID)
	}

	// --- Buyer receives the listed item, storage-cap clamped ---
	var buyerItemCurrent float64
	_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1", itemColumn), myCampID).Scan(&buyerItemCurrent)
	buyerCap := storagecap.CapFor(ctx, tx, myCampID)
	newBuyerVal, _ := storagecap.Clamp(buyerItemCurrent, float64(qty), buyerCap)
	_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", itemColumn), newBuyerVal, myCampID)

	_, _ = tx.ExecContext(ctx, "UPDATE market_exchange SET is_sold = TRUE WHERE id = $1", listingID)

	var sellerUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", sellerID).Scan(&sellerUserID)
	alertMsg := fmt.Sprintf("💱 %s: Another player has purchased your public listing for %s %s. +%s transferred instantly to reserves.",
		htmlBold("MARKET SALE"), htmlCode(fmt.Sprintf("%d", qty)), itemType, htmlCode(formatAsk(askType, askQty)))
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sellerUserID, alertMsg)

	if err := tx.Commit(); err != nil {
		return "⚠️ Transaction failed.", err
	}

	return fmt.Sprintf("🛍️✅ %s Bought %s %s for %s.", htmlBold("PURCHASE COMPLETE!"), htmlCode(fmt.Sprintf("%d", qty)), itemType, htmlCode(formatAsk(askType, askQty))), nil
}

func (h *ExchangeHandler) HandleBuyListingCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid listing."})
	}
	listingID := c.Args()[0]

	var myCampID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	if _, err := h.doBuyListing(ctx, myCampID, listingID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ That purchase couldn't be completed - it may be sold or expired."})
	}
	_ = c.Respond(&telebot.CallbackResponse{Text: "🛍️ Materials acquired successfully!"})

	return h.HandleExchangePanel(c)
}

// doBuyMarketItem is the testable core for the natural-language "buy X"
// command (nlpcommand.ActionBuyMarketItem, see
// FEEDBACK_CHANGELOG_NLP_PLAN.md Milestone 3's incremental-growth
// design). Unlike doBuyListing above (which buys one specific,
// already-known listing), this SEARCHES existing dollar-priced
// listings for the given resource and picks the best match, then
// delegates to doBuyListing for the actual transaction - so both entry
// points share identical balance/ownership/expiry enforcement.
//
// "Best match" = cheapest total price among listings that satisfy both
// minQty (buy at least this much) and maxDollars (pay no more than
// this), tie-broken by the smallest quantity (least overbuy beyond
// what was asked for). This game's market only ever sells whole
// listings, not partial fills (same constraint doBuyListing already
// has), so the quantity actually bought may exceed minQty - the
// returned message always states the real quantity and price bought,
// never a value the caller assumed going in.
func (h *ExchangeHandler) doBuyMarketItem(ctx context.Context, myCampID, resource string, minQty int, maxDollars float64) (string, error) {
	column, ok := marketResourceColumn(resource)
	if !ok {
		return fmt.Sprintf("❌ %s isn't tradeable on the exchange - try Metal, Crystal, or Scrap.", htmlEscape(capitalizeWord(resource))), errors.New("unsupported resource")
	}
	if minQty <= 0 {
		return "❌ Quantity must be positive.", errors.New("invalid quantity")
	}
	if maxDollars <= 0 {
		return "❌ Budget must be positive.", errors.New("invalid budget")
	}

	var listingID string
	err := h.DB.QueryRowContext(ctx,
		`SELECT id FROM market_exchange
		 WHERE item_type = $1 AND ask_type = 'dollars' AND is_sold = FALSE
		   AND quantity >= $2 AND ask_quantity <= $3 AND seller_id != $4
		 ORDER BY ask_quantity ASC, quantity ASC
		 LIMIT 1`,
		column, minQty, maxDollars, myCampID).Scan(&listingID)
	if err != nil {
		return fmt.Sprintf("❌ No listing currently matches - at least %s %s for $%.0f or less. Try a higher budget or check back later.",
			htmlCode(fmt.Sprintf("%d", minQty)), capitalizeWord(column), maxDollars), errors.New("no matching listing")
	}

	return h.doBuyListing(ctx, myCampID, listingID)
}
