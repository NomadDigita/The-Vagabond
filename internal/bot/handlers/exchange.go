package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"gopkg.in/telebot.v3"
)

// exchangePageSize is how many listings the "📖 View All Listings" page
// (HandleExchangePageCallback) shows per page - large enough to be worth
// a dedicated table view over the panel's 3-newest teaser, small enough
// that a page still fits comfortably in one Telegram message.
const exchangePageSize = 8

// bulkBuyPresets are the quick quantities offered on the "🛒 Buy in Bulk"
// menu (HandleBulkBuyMenuCallback) for each tradeable resource - common
// round numbers a player is likely to want without having to type a
// custom "buy X" command themselves.
var bulkBuyPresets = []int{500, 1000, 5000}

type ExchangeHandler struct {
	DB *sql.DB
}

func NewExchangeHandler(db *sql.DB) *ExchangeHandler {
	return &ExchangeHandler{DB: db}
}

// exchangeListingRow is one row of market_exchange joined against its
// seller's outpost name - shared shape between the teaser panel
// (HandleExchangePanel) and the full paginated view
// (HandleExchangePageCallback) so both render off the same query/table
// logic instead of drifting apart.
type exchangeListingRow struct {
	id, sellerName, itemType, askType string
	qty                               int
	askQty                            float64
}

// fetchExchangeListings pulls up to limit active (unsold) listings,
// offset rows in, newest-first ties broken by insertion order. Also
// returns the total count of active listings so callers can decide
// whether a "next page" / "view all" affordance is worth showing.
func (h *ExchangeHandler) fetchExchangeListings(ctx context.Context, limit, offset int) ([]exchangeListingRow, int, error) {
	var total int
	if err := h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM market_exchange WHERE is_sold = FALSE").Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := h.DB.QueryContext(ctx, `
		SELECT m.id, e.name, m.item_type, m.quantity, m.ask_type, m.ask_quantity
		FROM market_exchange m
		JOIN encampments e ON e.id = m.seller_id
		WHERE m.is_sold = FALSE
		ORDER BY m.id
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	var listings []exchangeListingRow
	for rows.Next() {
		var l exchangeListingRow
		if err := rows.Scan(&l.id, &l.sellerName, &l.itemType, &l.qty, &l.askType, &l.askQty); err == nil {
			listings = append(listings, l)
		}
	}
	return listings, total, nil
}

// renderListingsTable renders 2+ listings as a native Rich Message
// <table> (Bot API 10.1) instead of hand-padded card-style text - real
// columns that actually line up, rather than the emoji+label
// approximation every other panel in this codebase still uses because
// plain ModeHTML has no table primitive at all. A single listing (or
// zero) falls through to card rendering - a one-row table has no
// alignment benefit over a card and would just be a table-shaped waste
// of vertical space. Buy buttons are numbered against page (1-indexed
// within the page, matching the visible row numbers) so a caller
// showing page 2 still gets buttons labelled [1]/[2]/... rather than
// carrying over the previous page's absolute numbering.
func renderListingsTable(listings []exchangeListingRow, selector *telebot.ReplyMarkup) (string, []telebot.Row) {
	var buttons []telebot.Row
	if len(listings) == 0 {
		return "📋 " + htmlItalic("Board Clean: No active player listings currently on exchange.") + "\n\n", buttons
	}

	if len(listings) >= 2 {
		var table strings.Builder
		table.WriteString("<table bordered striped>\n<tr><th>Outpost</th><th>Selling</th><th>Asking</th></tr>\n")
		for i, l := range listings {
			table.WriteString(fmt.Sprintf("<tr><td>[%d] %s</td><td>%s %s</td><td>%s %s</td></tr>\n",
				i+1, htmlEscape(l.sellerName),
				resourceEmoji(l.itemType), htmlEscape(fmt.Sprintf("%d %s", l.qty, l.itemType)),
				askEmoji(l.askType), htmlEscape(formatAsk(l.askType, l.askQty))))
			btnBuy := selector.Data(fmt.Sprintf("🛍️ Buy [%d]", i+1), "buy_listing", l.id)
			buttons = append(buttons, selector.Row(btnBuy))
		}
		table.WriteString("</table>")
		return table.String() + "\n\n", buttons
	}

	var text string
	for i, l := range listings {
		text += fmt.Sprintf("🏷️ [%d] Outpost: %s\n    %s %s | %s Asking: %s\n\n",
			i+1, htmlBold(htmlEscape(l.sellerName)), resourceEmoji(l.itemType), htmlCode(fmt.Sprintf("%d %s", l.qty, l.itemType)),
			askEmoji(l.askType), htmlCode(formatAsk(l.askType, l.askQty)))
		btnBuy := selector.Data(fmt.Sprintf("🛍️ Buy [%d]", i+1), "buy_listing", l.id)
		buttons = append(buttons, selector.Row(btnBuy))
	}
	return text, buttons
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

	selector := &telebot.ReplyMarkup{}
	listings, total, err := h.fetchExchangeListings(ctx, 3, 0)
	var listingsText string
	var buttons []telebot.Row

	if err != nil {
		log.Printf("Failed scanning exchange listings: %v", err)
		listingsText = "📡 " + htmlItalic("Static: Connection interrupted.")
	} else {
		listingsText, buttons = renderListingsTable(listings, selector)
	}

	panelText := fmt.Sprintf(
		"💱 %s\n"+divider+"\n"+
			"%s\n\n"+
			"%s"+
			"%s\n"+
			"🔩 Sell 50 Metal — List on exchange for $150 Cash\n"+
			"☢️ Sell 20 Crystal — List on exchange for $300 Cash\n"+
			"💬 Or just type it, e.g. %s to barter for another resource, or %s to name any price you like - dynamic listings, any resource, any ask.\n"+
			divider,
		htmlBold("PLAYER AUCTION MARKET EXCHANGE"),
		htmlItalic("Buy raw tactical stockpiles listed by other active outposts:"),
		listingsText,
		htmlBold("📮 POST NEW LISTING"),
		htmlCode("list 300k scrap for 40k metal"),
		htmlCode("list 75 crystal for $2500"),
	)

	btnPostSteel := selector.Data("🔩 List 50 Metal ($150)", "post_listing", "metal")
	btnPostUranium := selector.Data("☢️ List 20 Crystal ($300)", "post_listing", "crystal")
	buttons = append(buttons, selector.Row(btnPostSteel, btnPostUranium))

	// Only surface "View All" when there's actually more than the
	// teaser's 3 rows to see - no point offering a page-1-of-1 button.
	var navRow []telebot.Btn
	if total > len(listings) {
		navRow = append(navRow, selector.Data(fmt.Sprintf("📖 View All Listings (%d)", total), "market_page", "0"))
	}
	navRow = append(navRow, selector.Data("🛒 Buy in Bulk", "bulk_buy_menu", "0"))
	buttons = append(buttons, selector.Row(navRow...))
	buttons = append(buttons, selector.Row(selector.Data("✏️ Custom Listing (Any Price)", "custom_listing_info", "0")))

	selector.Inline(buttons...)

	if len(listings) >= 2 {
		return sendPanelWithNavRich(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
	}
	return sendPanelWithNavHTML(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}

// HandleExchangePageCallback shows exchangePageSize listings at a time
// as a full Rich Message table, with Prev/Next paging - the "📖 View
// All Listings" counterpart to HandleExchangePanel's 3-newest teaser,
// for players who want to browse the whole board rather than just the
// most recent posts.
func (h *ExchangeHandler) HandleExchangePageCallback(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	page := 0
	if len(c.Args()) > 0 {
		if p, err := strconv.Atoi(c.Args()[0]); err == nil {
			page = p
		}
	}

	// page == -1 is the "Back to Market" button's target - route it to
	// the normal teaser panel instead of querying a negative SQL
	// OFFSET for an empty "page -1" table.
	if page < 0 {
		return h.HandleExchangePanel(c)
	}

	listings, total, err := h.fetchExchangeListings(ctx, exchangePageSize, page*exchangePageSize)
	if err != nil {
		log.Printf("Failed scanning exchange listings page: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Couldn't load listings - try again."})
	}
	// An out-of-range page (e.g. the last page just emptied out from
	// someone buying) falls back to page 0 rather than showing a
	// confusing empty table with dead Prev/Next buttons.
	if len(listings) == 0 && page > 0 {
		page = 0
		listings, total, err = h.fetchExchangeListings(ctx, exchangePageSize, 0)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Couldn't load listings - try again."})
		}
	}

	selector := &telebot.ReplyMarkup{}
	listingsText, buttons := renderListingsTable(listings, selector)

	totalPages := (total + exchangePageSize - 1) / exchangePageSize
	if totalPages < 1 {
		totalPages = 1
	}
	panelText := fmt.Sprintf(
		"💱 %s\n"+divider+"\n"+
			"%s (page %d/%d, %d active)\n\n"+
			"%s"+
			divider,
		htmlBold("FULL MARKET EXCHANGE BOARD"),
		htmlItalic("All active player listings:"), page+1, totalPages, total,
		listingsText,
	)

	var navRow []telebot.Btn
	if page > 0 {
		navRow = append(navRow, selector.Data("◀️ Prev", "market_page", strconv.Itoa(page-1)))
	}
	if (page+1)*exchangePageSize < total {
		navRow = append(navRow, selector.Data("Next ▶️", "market_page", strconv.Itoa(page+1)))
	}
	if len(navRow) > 0 {
		buttons = append(buttons, selector.Row(navRow...))
	}
	buttons = append(buttons, selector.Row(selector.Data("💱 Back to Market", "market_page", "-1")))
	selector.Inline(buttons...)

	if len(listings) >= 2 {
		return sendPanelWithNavRich(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
	}
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

// doBuyListing is doBuyListingQty's whole-listing convenience wrapper -
// buys everything the listing has, exactly like the pre-partial-fill
// behavior. Kept as its own name since it's still the right call for
// HandleBuyListingCallback's single "🛍️ Buy" button (there's no
// quantity to type there - tapping it always means "the whole thing")
// and for existing tests.
func (h *ExchangeHandler) doBuyListing(ctx context.Context, myCampID, listingID string) (string, error) {
	return h.doBuyListingQty(ctx, myCampID, listingID, 0)
}

// doBuyListingQty is the testable core shared by HandleBuyListingCallback
// (buying a specific listing the player tapped in the exchange panel),
// doBuyMarketItem below (buying a listing found by searching, for the
// natural-language "buy X" command - see FEEDBACK_CHANGELOG_NLP_PLAN.md
// Milestone 3), and the "🛒 Buy in Bulk" quick-preset menu - one code
// path so all three enforce the exact same balance/ownership/expiry
// checks, matching doPostListing's established convention.
//
// requestedQty is how many units of the listed item to buy. A value
// <= 0, or >= the listing's remaining quantity, buys the whole listing
// (the original, pre-partial-fill behavior) and marks it sold. A
// smaller positive value buys only that many units - the ask amount is
// pro-rated by the same fraction, the seller keeps the remainder
// listed at the same per-unit price, and is_sold stays FALSE. This is
// what lets a player buy 500 metal out of an 800-metal listing instead
// of being forced to take (and pay for) all 800.
//
// Ask amounts are pro-rated to 2 decimal places (math.Round(x*100)/100)
// so repeated partial fills of the same listing can't drift the
// remaining ask_quantity away from what the per-unit price actually
// implies through accumulated floating-point error.
//
// Returns a plain-text or HTML-safe message (caller decides how to
// send it) and, on failure, a non-nil error.
func (h *ExchangeHandler) doBuyListingQty(ctx context.Context, myCampID, listingID string, requestedQty int) (string, error) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Transaction failed.", err
	}
	defer tx.Rollback()

	var sellerID, itemType, askType string
	var listedQty int
	var listedAskQty float64
	var isSold bool

	query := `
		SELECT seller_id, item_type, quantity, ask_type, ask_quantity, is_sold
		FROM market_exchange 
		WHERE id = $1 FOR UPDATE`

	if err := tx.QueryRowContext(ctx, query, listingID).Scan(&sellerID, &itemType, &listedQty, &askType, &listedAskQty, &isSold); err != nil {
		return "❌ Expired: This listing is no longer available.", err
	}

	if isSold {
		return "❌ Already sold.", errors.New("listing already sold")
	}
	if sellerID == myCampID {
		return "❌ You can't buy your own listing.", errors.New("cannot buy own listing")
	}

	// Clamp to what's actually available - a partial-fill request for
	// more than remains just becomes "buy everything left", same as
	// requestedQty <= 0.
	buyQty := requestedQty
	if buyQty <= 0 || buyQty >= listedQty {
		buyQty = listedQty
	}
	fullFill := buyQty == listedQty

	// Pro-rate the ask by the fraction of the listing being bought -
	// full-fill keeps the exact original ask_quantity rather than
	// recomputing it through the fraction (avoids a 1.0-fraction
	// rounding no-op turning into an off-by-a-cent diff).
	askQty := listedAskQty
	if !fullFill {
		fraction := float64(buyQty) / float64(listedQty)
		askQty = math.Round(listedAskQty*fraction*100) / 100
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
			return fmt.Sprintf("❌ Insufficient Cash! Need $%.2f.", askQty), errors.New("insufficient dollars")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", askQty, myCampID); err != nil {
			return "⚠️ Transaction failed.", err
		}
	} else {
		var buyerAskBalance float64
		_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1 FOR UPDATE", askColumn), myCampID).Scan(&buyerAskBalance)
		if buyerAskBalance < askQty {
			return fmt.Sprintf("❌ Insufficient %s! Need %s.", capitalizeWord(askColumn), htmlCode(fmt.Sprintf("%.2f", askQty))), errors.New("insufficient ask resource")
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
	newBuyerVal, _ := storagecap.Clamp(buyerItemCurrent, float64(buyQty), buyerCap)
	_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", itemColumn), newBuyerVal, myCampID)

	if fullFill {
		_, _ = tx.ExecContext(ctx, "UPDATE market_exchange SET is_sold = TRUE WHERE id = $1", listingID)
	} else {
		// Partial fill: shrink the listing by exactly what was bought,
		// leaving the remainder live at the same per-unit price for
		// the next buyer.
		remainingQty := listedQty - buyQty
		remainingAskQty := math.Round((listedAskQty-askQty)*100) / 100
		remainingPriceDollars := 0.0
		if askIsDollars {
			remainingPriceDollars = remainingAskQty
		}
		_, _ = tx.ExecContext(ctx,
			"UPDATE market_exchange SET quantity = $1, ask_quantity = $2, price_dollars = $3 WHERE id = $4",
			remainingQty, remainingAskQty, remainingPriceDollars, listingID)
	}

	var sellerUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", sellerID).Scan(&sellerUserID)
	saleWord := "purchased your public listing"
	if !fullFill {
		saleWord = "purchased part of your public listing"
	}
	alertMsg := fmt.Sprintf("💱 %s: Another player has %s for %s %s. +%s transferred instantly to reserves.",
		htmlBold("MARKET SALE"), saleWord, htmlCode(fmt.Sprintf("%d", buyQty)), itemType, htmlCode(formatAsk(askType, askQty)))
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sellerUserID, alertMsg)

	if err := tx.Commit(); err != nil {
		return "⚠️ Transaction failed.", err
	}

	return fmt.Sprintf("🛍️✅ %s Bought %s %s for %s.", htmlBold("PURCHASE COMPLETE!"), htmlCode(fmt.Sprintf("%d", buyQty)), itemType, htmlCode(formatAsk(askType, askQty))), nil
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
// design) and the "🛒 Buy in Bulk" quick-preset menu. Unlike
// doBuyListing (which buys one specific, already-known listing), this
// SEARCHES existing dollar-priced listings for the given resource and
// picks the best match, then delegates to doBuyListingQty for the
// actual transaction - so both entry points share identical
// balance/ownership/expiry enforcement.
//
// "Best match" = cheapest per-unit price among listings that have at
// least minQty available and whose per-unit price × minQty fits within
// maxDollars, tie-broken by the smallest listing (least left over for
// other buyers). Buying now partially fills the winning listing for
// exactly minQty rather than being forced to take the whole thing -
// e.g. wanting 500 metal out of an 800-metal listing buys exactly 500
// and leaves 300 listed for the next buyer - unless minQty is at least
// the listing's full quantity, in which case doBuyListingQty's normal
// "buy everything available" full-fill applies. The returned message
// always states the real quantity and price bought.
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

	// Filter/order by PER-UNIT price (ask_quantity / quantity), not the
	// listing's total ask - a large listing whose whole-lot price
	// exceeds maxDollars can still be an affordable partial fill for
	// just minQty units, and the old total-ask filter wrongly excluded
	// those.
	var listingID string
	err := h.DB.QueryRowContext(ctx,
		`SELECT id FROM market_exchange
		 WHERE item_type = $1 AND ask_type = 'dollars' AND is_sold = FALSE
		   AND quantity >= $2 AND (ask_quantity / quantity) * $2 <= $3 AND seller_id != $4
		 ORDER BY (ask_quantity / quantity) ASC, quantity ASC
		 LIMIT 1`,
		column, minQty, maxDollars, myCampID).Scan(&listingID)
	if err != nil {
		return fmt.Sprintf("❌ No listing currently matches - at least %s %s for $%.0f or less. Try a higher budget or check back later.",
			htmlCode(fmt.Sprintf("%d", minQty)), capitalizeWord(column), maxDollars), errors.New("no matching listing")
	}

	return h.doBuyListingQty(ctx, myCampID, listingID, minQty)
}

// HandleBulkBuyMenuCallback shows the "🛒 Buy in Bulk" quick-preset
// menu - one tappable button per (resource, preset quantity)
// combination, each of which buys as much of that preset as the
// player's current cash affords via doBuyMarketItem's cheapest-match
// partial-fill search, without needing to type a natural-language "buy
// X" command by hand.
func (h *ExchangeHandler) HandleBulkBuyMenuCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid sender."})
	}
	ctx := context.Background()

	var campID string
	var dollars float64
	if err := h.DB.QueryRowContext(ctx, `
		SELECT e.id, r.dollars FROM encampments e
		JOIN resources r ON r.encampment_id = e.id
		WHERE e.user_id = $1`, sender.ID).Scan(&campID, &dollars); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row
	for _, resource := range []string{"metal", "crystal", "scrap"} {
		var row []telebot.Btn
		for _, qty := range bulkBuyPresets {
			label := fmt.Sprintf("%s %d %s", resourceEmoji(resource), qty, capitalizeWord(resource))
			row = append(row, selector.Data(label, "bulk_buy", resource, strconv.Itoa(qty)))
		}
		buttons = append(buttons, selector.Row(row...))
	}
	buttons = append(buttons, selector.Row(selector.Data("💱 Back to Market", "market_page", "-1")))
	selector.Inline(buttons...)

	panelText := fmt.Sprintf(
		"🛒 %s\n"+divider+"\n"+
			"%s\n\n"+
			"💵 Cash on hand: %s\n\n"+
			"%s"+
			divider,
		htmlBold("BUY IN BULK"),
		htmlItalic("Tap a preset to buy the cheapest matching listing(s), spending as much of your budget as needed - partial fills mean you get exactly the amount shown even from a bigger listing."),
		htmlCode(fmt.Sprintf("$%.0f", dollars)),
		htmlItalic("Want a custom amount? Just type it, e.g. \"buy 750 metal for $3000\".\n\n"),
	)

	return sendPanelWithNavHTML(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}

// HandleBulkBuyCallback executes one preset tapped from
// HandleBulkBuyMenuCallback - buys up to the preset quantity, spending
// no more than the player's current full cash balance (so it always
// finds the cheapest matching listing the player can afford, the same
// as doBuyMarketItem's normal budget cap, just pre-filled with "my
// whole wallet" instead of a typed-in number).
func (h *ExchangeHandler) HandleBulkBuyCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid bulk buy request."})
	}
	resource := c.Args()[0]
	qty, err := strconv.Atoi(c.Args()[1])
	if err != nil || qty <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid quantity."})
	}

	var campID string
	var dollars float64
	if err := h.DB.QueryRowContext(ctx, `
		SELECT e.id, r.dollars FROM encampments e
		JOIN resources r ON r.encampment_id = e.id
		WHERE e.user_id = $1`, sender.ID).Scan(&campID, &dollars); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}
	if dollars <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ No cash available to spend."})
	}

	if _, err := h.doBuyMarketItem(ctx, campID, resource, qty, dollars); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ No matching listing within your budget right now."})
	}
	_ = c.Respond(&telebot.CallbackResponse{Text: "🛍️ Bulk purchase complete!"})

	return h.HandleExchangePanel(c)
}

// HandleCustomListingInfoCallback answers the "✏️ Custom Listing (Any
// Price)" button. There's no separate custom-listing flow to build -
// doPostListing (via the natural-language "list X for Y" command,
// FEEDBACK_CHANGELOG_NLP_PLAN.md Milestone 3) already accepts any
// quantity and any ask (cash at any price, or a barter of any other
// tradeable resource) - so this button's whole job is to surface that
// capability with a couple of concrete examples for players who
// haven't discovered the typed-command flow yet.
func (h *ExchangeHandler) HandleCustomListingInfoCallback(c telebot.Context) error {
	_ = c.Respond(&telebot.CallbackResponse{Text: "✏️ See below for how to list at any price."})
	text := "✏️ " + htmlBold("CUSTOM LISTING - ANY PRICE") + "\n" + divider + "\n" +
		"The two panel buttons are just quick presets - to list any quantity at any price, just type it as a message:\n\n" +
		"💵 " + htmlItalic("Cash price:") + "\n" + htmlCode("list 120 metal for $450") + "\n\n" +
		"🔄 " + htmlItalic("Barter for another resource:") + "\n" + htmlCode("list 300k scrap for 40k metal") + "\n\n" +
		"Any quantity, any price - the market always accepts your own number, it's never limited to the preset buttons.\n" +
		divider
	return c.Send(text, telebot.ModeHTML)
}
