# ADR 0141: Decisions from `internal/engine/tick/aispawning_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aispawning_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestSpawnNewAIFactionsCreatesAFullyFunctionalEncampment verifies a

Removed from `internal/engine/tick/aispawning_test.go:10`.

~~~~text
TestSpawnNewAIFactionsCreatesAFullyFunctionalEncampment verifies a
spawned faction gets exactly the same shape as a boot-seeded one
(seedAICivilizations in cmd/bot/main.go): a users row, an encampments
row with is_ai_faction = TRUE at the deliberately-weak starting level,
a resources row, and a workshop_inventory row - so growAICivilizations
and decideAIFactionActions need zero special-casing to pick it up.
~~~~


## Force the cadence to have elapsed and confirm a third spawn succeeds.

Removed from `internal/engine/tick/aispawning_test.go:126`.

~~~~text
Force the cadence to have elapsed and confirm a third spawn succeeds.
~~~~


## TestGrowAICivilizationsCanListSurplusOnExchange verifies an AI faction

Removed from `internal/engine/tick/aispawning_test.go:147`.

~~~~text
TestGrowAICivilizationsCanListSurplusOnExchange verifies an AI faction
with a healthy crystal surplus eventually lists some of it on
market_exchange, using the exact same lot size/price a human posting
via /exchange would - project owner direction 2026-08-01 ("they can
literally do every single thing in the game," market listings given as
the explicit example). Runs many ticks since the listing chance is
probabilistic, mirroring TestAIFactionCanRaidAnotherAIFactionRarely's
pattern for a low-probability-per-call behavior.
~~~~


## Replenish crystal so later iterations still have a surplus to

Removed from `internal/engine/tick/aispawning_test.go:178`.

~~~~text
Replenish crystal so later iterations still have a surplus to
list even after an earlier iteration already listed some.
~~~~


## TestGrowAICivilizationsCanBuyAvailableListing verifies an AI faction

Removed from `internal/engine/tick/aispawning_test.go:193`.

~~~~text
TestGrowAICivilizationsCanBuyAvailableListing verifies an AI faction
with enough dollars eventually buys a listing posted by someone else
(a human, in this test), mirroring HandleBuyListingCallback's exact
mechanics - dollars debited from the buyer, credited to the seller
(respecting the seller's storage cap), the item transferred, and the
listing marked sold. Project owner follow-up direction 2026-08-01:
"buy and sell" both, not selling alone.
~~~~


## TestGrowAICivilizationsWontBuyItsOwnListing verifies the seller_id !=

Removed from `internal/engine/tick/aispawning_test.go:251`.

~~~~text
TestGrowAICivilizationsWontBuyItsOwnListing verifies the seller_id !=
faction.id filter: an AI faction's own listing is never a candidate
for its own purchase roll, exactly like HandleBuyListingCallback's
"You can't buy your own listing" guard for humans.
~~~~


## TestGrowAICivilizationsCanFoundOrJoinAClan verifies both clan-related

Removed from `internal/engine/tick/aispawning_test.go:293`.

~~~~text
TestGrowAICivilizationsCanFoundOrJoinAClan verifies both clan-related
paths: an AI faction with no clan eventually either founds its own
(a real clans + user_clans 'Leader' row) or applies to an existing
recruiting human clan (a pending clan_applications row plus a
notification to the human leader - not an automatic join, matching
HandleApplyToClanCallback's behavior for humans). Project owner
follow-up direction 2026-08-01: "create a clan and join one."
~~~~


## A recruiting human clan for the "apply" path to have something to

Removed from `internal/engine/tick/aispawning_test.go:312`.

~~~~text
A recruiting human clan for the "apply" path to have something to
target - without this, the faction can only ever found its own.
~~~~


## Whichever path fired, the faction must not have force-joined the

Removed from `internal/engine/tick/aispawning_test.go:350`.

~~~~text
Whichever path fired, the faction must not have force-joined the
human clan without an application being reviewed - HandleApply-
ToClanCallback never auto-joins, and neither should this.
~~~~
