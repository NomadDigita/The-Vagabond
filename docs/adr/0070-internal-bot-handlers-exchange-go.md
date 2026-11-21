# ADR 0070: Decisions from `internal/bot/handlers/exchange.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/exchange.go` and moved here so the source no longer carries them. Each entry is the original comment.

## exchangePageSize is how many listings the "📖 View All Listings" page

Removed from `internal/bot/handlers/exchange.go:19`.

~~~~text
exchangePageSize is how many listings the "📖 View All Listings" page
(HandleExchangePageCallback) shows per page - large enough to be worth
a dedicated table view over the panel's 3-newest teaser, small enough
that a page still fits comfortably in one Telegram message.
~~~~


## bulkBuyPresets are the quick quantities offered on the "🛒 Buy in Bulk"

Removed from `internal/bot/handlers/exchange.go:25`.

~~~~text
bulkBuyPresets are the quick quantities offered on the "🛒 Buy in Bulk"
menu (HandleBulkBuyMenuCallback) for each tradeable resource - common
round numbers a player is likely to want without having to type a
custom "buy X" command themselves.
~~~~


## exchangeListingRow is one row of market_exchange joined against its

Removed from `internal/bot/handlers/exchange.go:39`.

~~~~text
exchangeListingRow is one row of market_exchange joined against its
seller's outpost name - shared shape between the teaser panel
(HandleExchangePanel) and the full paginated view
(HandleExchangePageCallback) so both render off the same query/table
logic instead of drifting apart.
~~~~


## fetchExchangeListings pulls up to limit active (unsold) listings,

Removed from `internal/bot/handlers/exchange.go:50`.

~~~~text
fetchExchangeListings pulls up to limit active (unsold) listings,
offset rows in, newest-first ties broken by insertion order. Also
returns the total count of active listings so callers can decide
whether a "next page" / "view all" affordance is worth showing.
~~~~


## renderListingsTable renders 2+ listings as a native Rich Message

Removed from `internal/bot/handlers/exchange.go:82`.

~~~~text
renderListingsTable renders 2+ listings as a native Rich Message
<table> (Bot API 10.1) instead of hand-padded card-style text - real
columns that actually line up, rather than the emoji+label
approximation every other panel in this codebase still uses because
plain ModeHTML has no table primitive at all. A single listing (or
zero) falls through to card rendering - a one-row table has no
alignment benefit over a card and would just be a table-shaped waste
of vertical space. Buy buttons are numbered against page (1-indexed
within the page, matching the visible row numbers) so a caller
showing page 2 still gets buttons labelled [1]/[2]/... rather than
carrying over the previous page's absolute numbering.
~~~~


## Only surface "View All" when there's actually more than the

Removed from `internal/bot/handlers/exchange.go:174`.

~~~~text
Only surface "View All" when there's actually more than the
teaser's 3 rows to see - no point offering a page-1-of-1 button.
~~~~


## page == -1 is the "Back to Market" button's target - route it to

Removed from `internal/bot/handlers/exchange.go:208`.

~~~~text
page == -1 is the "Back to Market" button's target - route it to
the normal teaser panel instead of querying a negative SQL
OFFSET for an empty "page -1" table.
~~~~


## An out-of-range page (e.g. the last page just emptied out from

Removed from `internal/bot/handlers/exchange.go:220`.

~~~~text
An out-of-range page (e.g. the last page just emptied out from
someone buying) falls back to page 0 rather than showing a
confusing empty table with dead Prev/Next buttons.
~~~~


## validMarketResources are the resource columns tradeable through the

Removed from `internal/bot/handlers/exchange.go:267`.

~~~~text
validMarketResources are the resource columns tradeable through the
public market exchange, keyed by their lowercase name as used
everywhere else in this codebase (resourceEmoji, etc). Kept as an
explicit allow-list rather than accepting any resources column name
straight from a caller-controlled string, so the fmt.Sprintf column
interpolation below can never receive anything but one of these
three hardcoded literals.
~~~~


## capitalizeWord uppercases just the first byte - good enough for the

Removed from `internal/bot/handlers/exchange.go:288`.

~~~~text
capitalizeWord uppercases just the first byte - good enough for the
small set of all-ASCII words this package ever needs to display
(Metal/Crystal/Scrap/Dollars), without pulling in the now-deprecated
strings.Title for such a small job.
~~~~


## formatAsk renders what a listing is asking for in return - a dollar

Removed from `internal/bot/handlers/exchange.go:299`.

~~~~text
formatAsk renders what a listing is asking for in return - a dollar
price ("$500") or a quantity of another resource ("40000 metal") -
for consistent display across the panel, confirmation cards, and
buyer/seller notifications.
~~~~


## askEmoji is resourceEmoji's counterpart for the "asking" side of a

Removed from `internal/bot/handlers/exchange.go:310`.

~~~~text
askEmoji is resourceEmoji's counterpart for the "asking" side of a
listing, which can be "dollars" (not itself a tradeable resource
column) as well as any resource on the allow-list.
~~~~


## doPostListing is the testable core shared by HandlePostListingCallback's

Removed from `internal/bot/handlers/exchange.go:320`.

~~~~text
doPostListing is the testable core shared by HandlePostListingCallback's
two fixed quick-list buttons and the natural-language "list X for
sale"/"list X for Y" command (see FEEDBACK_CHANGELOG_NLP_PLAN.md
Milestone 3) - one code path so the AI-parsed flow enforces the
exact same balance checks as the button-driven UI, matching
doDispatchScoutMission's established convention.

askType is either "dollars" (a cash listing, askQty is the dollar
price - the original and still most common case) or another
tradeable resource name (a barter listing, askQty is how much of
that resource is wanted in return - askType must differ from
resource, since bartering a resource for itself is meaningless).

Returns the HTML-formatted message to show and, on failure, a
non-nil error alongside a plain-text failure notice.
~~~~


## price_dollars is kept populated for dollar listings (0 for

Removed from `internal/bot/handlers/exchange.go:380`.

~~~~text
price_dollars is kept populated for dollar listings (0 for
barter listings) so existing readers of that column - the AI
faction auto-buy tick, econadvisor's market stats - keep working
against dollar listings unchanged; see the ask_type/ask_quantity
doc comment in internal/db/schema/schema.go.
~~~~


## The panel only ever offers these two quick-list quantities/prices

Removed from `internal/bot/handlers/exchange.go:419`.

~~~~text
The panel only ever offers these two quick-list quantities/prices
(see HandleExchangePanel's btnPostSteel/btnPostUranium) - the
natural-language flow posts arbitrary quantities/asks through
the same doPostListing core below instead.
~~~~


## doBuyListing is doBuyListingQty's whole-listing convenience wrapper -

Removed from `internal/bot/handlers/exchange.go:438`.

~~~~text
doBuyListing is doBuyListingQty's whole-listing convenience wrapper -
buys everything the listing has, exactly like the pre-partial-fill
behavior. Kept as its own name since it's still the right call for
HandleBuyListingCallback's single "🛍️ Buy" button (there's no
quantity to type there - tapping it always means "the whole thing")
and for existing tests.
~~~~


## doBuyListingQty is the testable core shared by HandleBuyListingCallback

Removed from `internal/bot/handlers/exchange.go:448`.

~~~~text
doBuyListingQty is the testable core shared by HandleBuyListingCallback
(buying a specific listing the player tapped in the exchange panel),
doBuyMarketItem below (buying a listing found by searching, for the
natural-language "buy X" command - see FEEDBACK_CHANGELOG_NLP_PLAN.md
Milestone 3), and the "🛒 Buy in Bulk" quick-preset menu - one code
path so all three enforce the exact same balance/ownership/expiry
checks, matching doPostListing's established convention.

requestedQty is how many units of the listed item to buy. A value
<= 0, or >= the listing's remaining quantity, buys the whole listing
(the original, pre-partial-fill behavior) and marks it sold. A
smaller positive value buys only that many units - the ask amount is
pro-rated by the same fraction, the seller keeps the remainder
listed at the same per-unit price, and is_sold stays FALSE. This is
what lets a player buy 500 metal out of an 800-metal listing instead
of being forced to take (and pay for) all 800.

Ask amounts are pro-rated to 2 decimal places (math.Round(x*100)/100)
so repeated partial fills of the same listing can't drift the
remaining ask_quantity away from what the per-unit price actually
implies through accumulated floating-point error.

Returns a plain-text or HTML-safe message (caller decides how to
send it) and, on failure, a non-nil error.
~~~~


## Clamp to what's actually available - a partial-fill request for

Removed from `internal/bot/handlers/exchange.go:500`.

~~~~text
Clamp to what's actually available - a partial-fill request for
more than remains just becomes "buy everything left", same as
requestedQty <= 0.
~~~~


## Pro-rate the ask by the fraction of the listing being bought -

Removed from `internal/bot/handlers/exchange.go:509`.

~~~~text
Pro-rate the ask by the fraction of the listing being bought -
full-fill keeps the exact original ask_quantity rather than
recomputing it through the fraction (avoids a 1.0-fraction
rounding no-op turning into an off-by-a-cent diff).
~~~~


## Fail safely rather than fmt.Sprintf-ing an unrecognized

Removed from `internal/bot/handlers/exchange.go:521`.

~~~~text
Fail safely rather than fmt.Sprintf-ing an unrecognized
column name into a query - shouldn't happen for anything
listed through doPostListing, but a defensive floor for any
older/out-of-band row.
~~~~


## Partial fill: shrink the listing by exactly what was bought,

Removed from `internal/bot/handlers/exchange.go:583`.

~~~~text
Partial fill: shrink the listing by exactly what was bought,
leaving the remainder live at the same per-unit price for
the next buyer.
~~~~


## userIDForCamp resolves a campID's owning Telegram user_id - used by

Removed from `internal/bot/handlers/exchange.go:618`.

~~~~text
userIDForCamp resolves a campID's owning Telegram user_id - used by
achievement/quest progress hooks (internal/game/progression) that
need a userID but only have a campID in scope. Returns 0 (never a
real or AI-faction user_id) if not found, so callers can check
`!= 0` rather than threading an error through a best-effort side
call.
~~~~


## doBuyMarketItem is the testable core for the natural-language "buy X"

Removed from `internal/bot/handlers/exchange.go:651`.

~~~~text
doBuyMarketItem is the testable core for the natural-language "buy X"
command (nlpcommand.ActionBuyMarketItem, see
FEEDBACK_CHANGELOG_NLP_PLAN.md Milestone 3's incremental-growth
design) and the "🛒 Buy in Bulk" quick-preset menu. Unlike
doBuyListing (which buys one specific, already-known listing), this
SEARCHES existing dollar-priced listings for the given resource and
picks the best match, then delegates to doBuyListingQty for the
actual transaction - so both entry points share identical
balance/ownership/expiry enforcement.

"Best match" = cheapest per-unit price among listings that have at
least minQty available and whose per-unit price × minQty fits within
maxDollars, tie-broken by the smallest listing (least left over for
other buyers). Buying now partially fills the winning listing for
exactly minQty rather than being forced to take the whole thing -
e.g. wanting 500 metal out of an 800-metal listing buys exactly 500
and leaves 300 listed for the next buyer - unless minQty is at least
the listing's full quantity, in which case doBuyListingQty's normal
"buy everything available" full-fill applies. The returned message
always states the real quantity and price bought.
~~~~


## Filter/order by PER-UNIT price (ask_quantity / quantity), not the

Removed from `internal/bot/handlers/exchange.go:683`.

~~~~text
Filter/order by PER-UNIT price (ask_quantity / quantity), not the
listing's total ask - a large listing whose whole-lot price
exceeds maxDollars can still be an affordable partial fill for
just minQty units, and the old total-ask filter wrongly excluded
those.
~~~~


## HandleBulkBuyMenuCallback shows the "🛒 Buy in Bulk" quick-preset

Removed from `internal/bot/handlers/exchange.go:704`.

~~~~text
HandleBulkBuyMenuCallback shows the "🛒 Buy in Bulk" quick-preset
menu - one tappable button per (resource, preset quantity)
combination, each of which buys as much of that preset as the
player's current cash affords via doBuyMarketItem's cheapest-match
partial-fill search, without needing to type a natural-language "buy
X" command by hand.
~~~~


## HandleBulkBuyCallback executes one preset tapped from

Removed from `internal/bot/handlers/exchange.go:754`.

~~~~text
HandleBulkBuyCallback executes one preset tapped from
HandleBulkBuyMenuCallback - buys up to the preset quantity, spending
no more than the player's current full cash balance (so it always
finds the cheapest matching listing the player can afford, the same
as doBuyMarketItem's normal budget cap, just pre-filled with "my
whole wallet" instead of a typed-in number).
~~~~


## HandleCustomListingInfoCallback answers the "✏️ Custom Listing (Any

Removed from `internal/bot/handlers/exchange.go:792`.

~~~~text
HandleCustomListingInfoCallback answers the "✏️ Custom Listing (Any
Price)" button. There's no separate custom-listing flow to build -
doPostListing (via the natural-language "list X for Y" command,
FEEDBACK_CHANGELOG_NLP_PLAN.md Milestone 3) already accepts any
quantity and any ask (cash at any price, or a barter of any other
tradeable resource) - so this button's whole job is to surface that
capability with a couple of concrete examples for players who
haven't discovered the typed-command flow yet.
~~~~
