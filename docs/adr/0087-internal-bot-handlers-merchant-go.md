# ADR 0087: Decisions from `internal/bot/handlers/merchant.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/merchant.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MerchantHandler is the panel/purchase side of

Removed from `internal/bot/handlers/merchant.go:14`.

~~~~text
MerchantHandler is the panel/purchase side of
RARE_WORLD_FEATURES_PLAN.md Phase 4 - the tick.Engine side
(spawn/expire) lives in internal/engine/tick/wanderingmerchant.go.
~~~~


## doBuyMerchantOffer is the testable core behind HandleMerchantBuyCallback.

Removed from `internal/bot/handlers/merchant.go:81`.

~~~~text
doBuyMerchantOffer is the testable core behind HandleMerchantBuyCallback.
Transactional, FOR UPDATE-locked exactly like doBuyListingQty in
exchange.go: checks purchased_by IS NULL AND not expired, checks/
deducts the buyer's dollars, credits the resource (storage-cap
clamped), marks purchased_by/purchased_at. No counterparty payout -
there's no seller, so unlike the player market this only ever
touches the buyer's own resources.
~~~~
