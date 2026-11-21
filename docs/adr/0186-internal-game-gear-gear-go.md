# ADR 0186: Decisions from `internal/game/gear/gear.go`

Status: Accepted

These decisions were written as comments in `internal/game/gear/gear.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package gear is NEXT_GENERATION_FEATURES_PLAN.md Phase 3: Crafting &

Removed from `internal/game/gear/gear.go:1`.

~~~~text
Package gear is NEXT_GENERATION_FEATURES_PLAN.md Phase 3: Crafting &
Gear. Definitions (which gear exists, its slot/rarity/cost/bonus)
are static Go data, matching the same convention every other
definition catalog in this codebase uses (progression.Achievements,
weather.go's eventPool, relicconvoys.go's relicNames) - only
per-player ownership and the crafting queue are persisted.

Scope note on gear's effect (see NEXT_GENERATION_FEATURES_PLAN.md's
own Phase 3 status line for the full writeup): equipped gear grants
a passive resource-generation bonus, applied at the exact same
call site internal/engine/resource/resource.go already uses for
Seasonal Wasteland Blooms and every other generation multiplier -
deliberately NOT wired into internal/engine/tick's raid-combat power
calculation. That calculation is large, deeply interconnected (hero
superpowers, capital-unit tech multipliers, road-combat power) and
this project has an established precedent of not touching it
directly for additive features (see factiondefeat.go's own
documented rationale for staying out of resolveRaidCombats). A
resource-generation bonus is a fully real, felt stat modifier -
just a safer one to integrate than a raw combat-power edit.
~~~~


## Querier is satisfied by both *sql.DB and *sql.Tx, matching the same

Removed from `internal/game/gear/gear.go:32`.

~~~~text
Querier is satisfied by both *sql.DB and *sql.Tx, matching the same
convention progression.Querier/notifications.querier/storagecap.Queryer
already establish.
~~~~


## Definition is one craftable or drop-only item. ResourceBonusPct is

Removed from `internal/game/gear/gear.go:58`.

~~~~text
Definition is one craftable or drop-only item. ResourceBonusPct is
the passive resource-generation bonus (e.g. 3.0 = +3%) it grants
while equipped - see the package doc comment above for why this,
not combat power, is what "stat modifier" means for gear in this
build. Craftable gear (Common/Rare) has a real CraftCost/CraftMinutes;
drop-only gear (Epic/Legendary) has both left at their zero values
and is never offered in the Craft sub-panel.
~~~~


## resource column -> amount

Removed from `internal/game/gear/gear.go:73`.

~~~~text
resource column -> amount
~~~~


## Catalog is the full launch gear set - one Common, one Rare (both

Removed from `internal/game/gear/gear.go:77`.

~~~~text
Catalog is the full launch gear set - one Common, one Rare (both
craftable), one Epic, and one Legendary (both drop-only, see
GrantRandomDrop) per slot. A future phase can grow this catalog as
pure data with no code changes, the same property
progression.Achievements/Quests were built around.
~~~~


## ByKey looks up a catalog Definition, or (zero-value, false) if

Removed from `internal/game/gear/gear.go:102`.

~~~~text
ByKey looks up a catalog Definition, or (zero-value, false) if
gearKey doesn't match anything - a deliberately safe result, since
gearKey most often arrives as caller-controlled text (a callback
arg, a player_gear row that could in theory outlive a catalog
change).
~~~~


## StartCraft begins crafting one unit of gearKey for campID: validates

Removed from `internal/game/gear/gear.go:128`.

~~~~text
StartCraft begins crafting one unit of gearKey for campID: validates
it's a real, Craftable definition, deducts CraftCost (failing
cleanly on insufficient resources, same "check then deduct in one
transaction" pattern doPostListing/doBuyListingQty already
establish), and inserts a crafting_queue row due
CraftMinutes from now. ResolveCraftingQueue (the tick phase) credits
the finished item later - crafting is never instant, matching the
plan doc's explicit "spend resources up front, wait, tick phase
resolves completion" design.
~~~~


## ResolveCraftingQueue is the "crafting_resolve" tick phase - credits

Removed from `internal/game/gear/gear.go:168`.

~~~~text
ResolveCraftingQueue is the "crafting_resolve" tick phase - credits
every crafting_queue row whose completes_at has passed and isn't yet
marked complete, upserting the finished item into player_gear and
notifying the owner.
~~~~


## Equip sets gearKey as the equipped item in its slot for campID,

Removed from `internal/game/gear/gear.go:214`.

~~~~text
Equip sets gearKey as the equipped item in its slot for campID,
unequipping whatever else was in that slot - a player owns at most
one equipped item per slot at a time (Weapon/Armor/Utility), same
three-slot model the plan doc's own design describes. Fails if the
player doesn't own at least one unit of gearKey.
~~~~


## Unequip anything else currently in this slot - one query using

Removed from `internal/game/gear/gear.go:229`.

~~~~text
Unequip anything else currently in this slot - one query using
the catalog's own slot membership rather than a slot column on
player_gear, so the catalog stays the single source of truth for
which items belong to which slot.
~~~~


## GrantRandomDrop credits one random Definition of the given rarity to

Removed from `internal/game/gear/gear.go:249`.

~~~~text
GrantRandomDrop credits one random Definition of the given rarity to
campID (used for Epic/Legendary World Boss and Relic Convoy drops -
see engine.go's payoutWorldBossLoot and relicconvoy.go's
doClaimRelicConvoy) and notifies the owner. Returns the granted
Definition so the caller can fold its name into its own reward
message if it wants to.
~~~~


## ResourceBonusPctFor sums the ResourceBonusPct of every currently

Removed from `internal/game/gear/gear.go:279`.

~~~~text
ResourceBonusPctFor sums the ResourceBonusPct of every currently
equipped item for campID - the value
internal/engine/resource/resource.go reads at the same call site
Phase 2's Bloom multiplier already uses.
~~~~


## AffordCraft reports whether campID currently has enough resources to

Removed from `internal/game/gear/gear.go:301`.

~~~~text
AffordCraft reports whether campID currently has enough resources to
start crafting def - used by the /gear panel to show affordability
inline, matching the exchange panel's own established
affordability-preview pattern, without actually spending anything.
~~~~


## pqStringArray formats a Go string slice as a Postgres text[] array

Removed from `internal/game/gear/gear.go:315`.

~~~~text
pqStringArray formats a Go string slice as a Postgres text[] array
literal for use with = ANY($n) - a small local helper rather than
pulling in the lib/pq Array helper type just for this one call site.
~~~~
