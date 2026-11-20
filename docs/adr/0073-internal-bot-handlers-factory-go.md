# ADR 0073: Decisions from `internal/bot/handlers/factory.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/factory.go` and moved here so the source no longer carries them. Each entry is the original comment.

## renderOrEdit dynamically edits the current message if accessed via callbacks, saving layou

Removed from `internal/bot/handlers/factory.go:25`.

~~~~text
renderOrEdit dynamically edits the current message if accessed via callbacks, saving layout space and preventing desyncs
~~~~


## renderOrEditHTML is identical to renderOrEdit but renders with

Removed from `internal/bot/handlers/factory.go:33`.

~~~~text
renderOrEditHTML is identical to renderOrEdit but renders with
Telegram HTML parse mode - only use at a call site where `text` has
already been built with the htmlBold/htmlItalic/htmlCode/htmlEscape
helpers in render.go.
~~~~


## craftSpec is one buildable item's cost/target-column/labeling, used by

Removed from `internal/bot/handlers/factory.go:262`.

~~~~text
craftSpec is one buildable item's cost/target-column/labeling, used by
the generic multi-quantity crafting flow below. Previously each item
was a fully separate switch case in HandleCraftCallback, hand-coding
the same cost-check/deduct/increment shape 20+ times with quantity
hardcoded to 1 - that made "produce more than one at a time" require
rewriting every case identically (and easy to get inconsistent across
them). This table is the single place quantity, cost, and target
column live now; the actual crafting logic in HandleCraftQuantityCallback
is written once and applies to every item.
~~~~


## resources table column -> cost PER UNIT

Removed from `internal/bot/handlers/factory.go:273`.

~~~~text
resources table column -> cost PER UNIT
~~~~


## true => after crafting, return to the Vehicles panel instead of Recruit

Removed from `internal/bot/handlers/factory.go:277`.

~~~~text
true => after crafting, return to the Vehicles panel instead of Recruit
~~~~


## defaultMaxPerOrder is the "up to 20x at once" ceiling for ordinary

Removed from `internal/bot/handlers/factory.go:280`.

~~~~text
defaultMaxPerOrder is the "up to 20x at once" ceiling for ordinary
units. Units that deliberately get a lower ceiling say so explicitly
in their own craftSpec entry below, with a comment explaining why -
never silently omit maxPerOrder and fall back to this for something
that shouldn't get it.
~~~~


## hardcodedCraftSpecs covers the units that predate the content package's

Removed from `internal/bot/handlers/factory.go:287`.

~~~~text
hardcodedCraftSpecs covers the units that predate the content package's
canonical registry (internal/game/content/units.go's doc comment: "the
SpaceHunt-revival content spine... every new unit added going forward
should be defined here first" - these are the ones from before that
convention started, not yet migrated). Costs here are unchanged from
the original single-unit switch cases - this refactor changes HOW
quantity is handled, not what anything costs.
~~~~


## Deliberately excluded from the 20x default: a single Nuclear

Removed from `internal/bot/handlers/factory.go:310`.

~~~~text
Deliberately excluded from the 20x default: a single Nuclear
Device already carries +1500 Detonation, by far the highest
power-per-unit in the game. Twenty in one order would let a
single craft action reshape the server's balance far more than
any other bulk order - capped low enough that mass-producing
them still takes real, deliberate repetition.
~~~~


## craftSpecFor resolves an item key to its craftSpec, checking the

Removed from `internal/bot/handlers/factory.go:352`.

~~~~text
craftSpecFor resolves an item key to its craftSpec, checking the
pre-content-package units above first, then falling back to
content.FindUnit for everything registered there (scout,
battlecruiser, deathstar, liberator, wraith, observer, guardian,
piercing_missile, cargo_mk1/2/3) - reused live rather than copied, so
this table can never drift out of sync with the canonical registry.
~~~~


## button callback data uses "cargo_jet"; same item, one spec entry

Removed from `internal/bot/handlers/factory.go:360`.

~~~~text
button callback data uses "cargo_jet"; same item, one spec entry
~~~~


## Doomsday Rigs already have their own total-fleet cap tied to

Removed from `internal/bot/handlers/factory.go:371`.

~~~~text
Doomsday Rigs already have their own total-fleet cap tied to
Outpost level (content.MaxDoomsdayRigs, checked separately in
HandleCraftQuantityCallback since it's a cap on units OWNED,
not units per order). Offering a "20x" button that will almost
always fail against a cap that's usually 1-5 total would just
be a confusing, near-always-rejected option - keep the
per-order ceiling low and let the real cap be the binding one.
~~~~


## quantityOptions returns the quantity buttons to offer for a given

Removed from `internal/bot/handlers/factory.go:390`.

~~~~text
quantityOptions returns the quantity buttons to offer for a given
spec's maxPerOrder - always includes 1x, then scales up to (and
including) maxPerOrder without offering an option above it.
~~~~


## HandleCraftCallback ("\fcraft_item") is the first tap on a "Recruit X"/

Removed from `internal/bot/handlers/factory.go:407`.

~~~~text
HandleCraftCallback ("\fcraft_item") is the first tap on a "Recruit X"/
"Forge X" button - it no longer crafts immediately, it opens the
quantity picker (HandleCraftQuantityCallback does the actual crafting).
This two-step shape mirrors how every other consequential action in
this game already confirms before committing resources (e.g. the road-
encounter Attack/Continue buttons) rather than firing on the very
first tap.
~~~~


## HandleCraftQuantityCallback ("\fcraft_qty") performs the actual craft

Removed from `internal/bot/handlers/factory.go:472`.

~~~~text
HandleCraftQuantityCallback ("\fcraft_qty") performs the actual craft
for the quantity chosen on HandleCraftCallback's picker. Every cost
deduction and inventory increment happens ONCE per order, scaled by
quantity (a single `soldiers = soldiers + $1` with quantity as the
parameter) rather than looping the original single-unit logic N times -
both simpler and avoids N separate round-trips for what the player
experiences as one action.
~~~~


## Hangar capacity check, scaled by quantity - see the original

Removed from `internal/bot/handlers/factory.go:530`.

~~~~text
Hangar capacity check, scaled by quantity - see the original
single-unit version of this check (still the same cap formula,
still the same set of columns summed) for why the cap exists.
10x pass: 500 + hangarLvl*200 (was 50 + hangarLvl*20) - keep in
sync with the identical formula in engine/agent/agent.go's
auto-recruit path.
~~~~


## Doomsday Rig's total-fleet cap (tied to Outpost level, not

Removed from `internal/bot/handlers/factory.go:554`.

~~~~text
Doomsday Rig's total-fleet cap (tied to Outpost level, not
per-order) - see craftSpecFor's comment on why this stays a
separate check rather than folding into maxPerOrder.
~~~~


## Engineering Bay refund - same flat-per-craft-ACTION amount as

Removed from `internal/bot/handlers/factory.go:581`.

~~~~text
Engineering Bay refund - same flat-per-craft-ACTION amount as
before (not multiplied by quantity): a 20-unit order is still one
trip through the assembly line, not twenty separate ones.
~~~~
