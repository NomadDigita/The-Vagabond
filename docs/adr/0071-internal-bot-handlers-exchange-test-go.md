# ADR 0071: Decisions from `internal/bot/handlers/exchange_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/exchange_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## 800 metal listed for $1600 total -> $2/unit.

Removed from `internal/bot/handlers/exchange_test.go:234`.

~~~~text
800 metal listed for $1600 total -> $2/unit.
~~~~


## TestDoBuyListingQty_FullFillStillMarksSold covers the existing

Removed from `internal/bot/handlers/exchange_test.go:275`.

~~~~text
TestDoBuyListingQty_FullFillStillMarksSold covers the existing
whole-listing behavior: requesting at least the full quantity (or 0,
via doBuyListing) still buys everything and marks the listing sold,
exactly as before partial fills were introduced.
~~~~


## Requesting more than what's listed (300 > 200) clamps to a full buy.

Removed from `internal/bot/handlers/exchange_test.go:298`.

~~~~text
Requesting more than what's listed (300 > 200) clamps to a full buy.
~~~~


## TestDoBuyMarketItem_PartialFillFromLargerListing is doBuyMarketItem's

Removed from `internal/bot/handlers/exchange_test.go:323`.

~~~~text
TestDoBuyMarketItem_PartialFillFromLargerListing is doBuyMarketItem's
(the natural-language "buy X" search path) counterpart to
TestDoBuyListingQty_PartialFillLeavesRemainder: wanting 500 metal
should succeed and only take 500 even when the only matching listing
has 800, and the per-unit price filter should admit a listing whose
FULL price exceeds the budget but whose price for just the requested
quantity does not.
~~~~


## 800 metal for $2400 total ($3/unit) - the whole lot costs more

Removed from `internal/bot/handlers/exchange_test.go:341`.

~~~~text
800 metal for $2400 total ($3/unit) - the whole lot costs more
than the buyer's $2000 budget, but 500 units at $3/unit ($1500)
fits comfortably.
~~~~
