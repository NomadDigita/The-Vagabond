# ADR 0088: Decisions from `internal/bot/handlers/merchant_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/merchant_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestDoBuyMerchantOffer_PaysDollarsAndCreditsResource is the direct

Removed from `internal/bot/handlers/merchant_test.go:23`.

~~~~text
TestDoBuyMerchantOffer_PaysDollarsAndCreditsResource is the direct
test for RARE_WORLD_FEATURES_PLAN.md Phase 4.4: buying an offer
deducts exactly its price in dollars and credits exactly its
quantity of the resource (storage-cap clamped), with no seller
counterpart at all.
~~~~


## TestDoBuyMerchantOffer_AlreadyPurchasedFails covers race-safety: a

Removed from `internal/bot/handlers/merchant_test.go:88`.

~~~~text
TestDoBuyMerchantOffer_AlreadyPurchasedFails covers race-safety: a
second purchase attempt against an already-bought offer must fail
cleanly rather than double-selling it.
~~~~


## TestDoBuyMerchantOffer_ExpiredOfferFails confirms an offer past its

Removed from `internal/bot/handlers/merchant_test.go:119`.

~~~~text
TestDoBuyMerchantOffer_ExpiredOfferFails confirms an offer past its
expiry can no longer be purchased, even if it hasn't been swept by
the expiry tick phase yet.
~~~~
