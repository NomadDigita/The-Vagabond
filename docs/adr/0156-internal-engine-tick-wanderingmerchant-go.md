# ADR 0156: Decisions from `internal/engine/tick/wanderingmerchant.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/wanderingmerchant.go` and moved here so the source no longer carries them. Each entry is the original comment.

## RARE_WORLD_FEATURES_PLAN.md Phase 4: The Wandering Merchant.

Removed from `internal/engine/tick/wanderingmerchant.go:12`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 4: The Wandering Merchant.

A rare, time-limited NPC vendor - not tied to any player's own
listing, not competing with the player market exchange - offering a
short rotating batch of steeply discounted resource lots. Gone after
being bought out or after its window closes. See the plan doc's
section 4.1 for why this is its own dedicated table rather than a
synthetic market_exchange seller.
~~~~


## merchantSpawnChance is the per-tick probability of the Wandering

Removed from `internal/engine/tick/wanderingmerchant.go:21`.

~~~~text
merchantSpawnChance is the per-tick probability of the Wandering
Merchant arriving with a fresh batch of offers, checked only when
there are currently no active (unpurchased, unexpired) offers - a
fresh visit is a clean batch, not an ever-growing backlog. Rarer
than a normal weather roll, commoner than a relic convoy - averaging
roughly one visit every ~4 hours of continuous uptime at the tick
engine's ~60s cadence.

A var, not a const, so wanderingmerchant_test.go can deterministically
force a spawn instead of looping thousands of real-Postgres round
trips waiting for a legitimate 0.4% roll to land.
~~~~


## merchantVisitWindowMinutes is how long an unpurchased offer batch

Removed from `internal/engine/tick/wanderingmerchant.go:34`.

~~~~text
merchantVisitWindowMinutes is how long an unpurchased offer batch
stays live - shorter than a relic convoy's window since this is a
recurring, lighter event rather than a once-in-many-hours moment.
~~~~


## merchantDiscountFactor is what fraction of a "typical" market rate

Removed from `internal/engine/tick/wanderingmerchant.go:46`.

~~~~text
merchantDiscountFactor is what fraction of a "typical" market rate
the Merchant charges - 0.6 means roughly 40% cheaper than a normal
player listing, a genuine bargain rather than a reskinned market
post.
~~~~


## merchantOfferResources are the resources the Merchant can offer -

Removed from `internal/engine/tick/wanderingmerchant.go:52`.

~~~~text
merchantOfferResources are the resources the Merchant can offer -
the same three tradeable resources the player market exchange
supports (see exchange.go's validMarketResources).
~~~~


## merchantBaseUnitPrice is a rough "typical market rate" per unit for

Removed from `internal/engine/tick/wanderingmerchant.go:57`.

~~~~text
merchantBaseUnitPrice is a rough "typical market rate" per unit for
each resource, used only to derive a plausible discounted Merchant
price - not read from live market_exchange listings (which may be
sparse or absent early in a world's life), so the Merchant always
has a sensible price to offer regardless of current player market
activity. Deliberately in the same ballpark as the exchange panel's
own quick-list presets (e.g. "List 50 Metal ($150)" implies $3/unit).
~~~~


## wanderingMerchantSpawn is the "wandering_merchant_spawn" tick phase -

Removed from `internal/engine/tick/wanderingmerchant.go:76`.

~~~~text
wanderingMerchantSpawn is the "wandering_merchant_spawn" tick phase -
see engine.go's ProcessTick phase list. Rolls a fresh batch of offers
into existence only when none are currently active.
~~~~


## wanderingMerchantExpire is the "wandering_merchant_expire" tick

Removed from `internal/engine/tick/wanderingmerchant.go:116`.

~~~~text
wanderingMerchantExpire is the "wandering_merchant_expire" tick
phase - clears offers nobody bought in time, with a "moved on"
broadcast, same pattern as Phase 1/2's expiry precedent. Only
broadcasts once per expiry sweep (not once per leftover offer) since
a whole batch expiring together is one event, not several.
~~~~
