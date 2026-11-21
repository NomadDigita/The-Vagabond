# ADR 0062: Decisions from `internal/bot/handlers/crystal_exchange.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/crystal_exchange.go` and moved here so the source no longer carries them. Each entry is the original comment.

## crystalConversionRate is how much of a resource 1 Crystal converts into.

Removed from `internal/bot/handlers/crystal_exchange.go:13`.

~~~~text
crystalConversionRate is how much of a resource 1 Crystal converts into.
Crystal is the rarest and most valuable resource in the game (see
MMO_WORLD_EVOLUTION_PLAN.md), so every rate here is deliberately large -
this is meant to be a genuine "cash out my rarest resource for a big
stockpile of whatever I'm actually short on" lever, not a rounding-error
trade. Rates scale with how commonly needed the target resource is:
resources every player burns constantly (Scrap, Metal, Rations) convert
generously; resources that are already scarce or high-value in their own
right (Neuro Cores, Ether) convert conservatively, since they don't need
the help and a generous rate there would just let Crystal launder into
a second rare currency for free.
~~~~


## HandleCrystalExchangePanel shows the player's Crystal balance and one

Removed from `internal/bot/handlers/crystal_exchange.go:42`.

~~~~text
HandleCrystalExchangePanel shows the player's Crystal balance and one
button per convertible resource, offering fixed 1/5/25-Crystal presets so
the conversion amount never needs free-text input.
~~~~


## resourceKey is only ever one of the fixed literals in

Removed from `internal/bot/handlers/crystal_exchange.go:132`.

~~~~text
resourceKey is only ever one of the fixed literals in
crystalConversionTable above (never taken verbatim from outside
that table), so it is safe to interpolate into the column list.
~~~~
