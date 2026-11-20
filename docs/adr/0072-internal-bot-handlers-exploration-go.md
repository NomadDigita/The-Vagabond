# ADR 0072: Decisions from `internal/bot/handlers/exploration.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/exploration.go` and moved here so the source no longer carries them. Each entry is the original comment.

## explorationSiteTemplate is one entry in the reward pool a personal

Removed from `internal/bot/handlers/exploration.go:59`.

~~~~text
explorationSiteTemplate is one entry in the reward pool a personal
exploration expedition rolls from at dispatch time - a flavor name/type
paired with a reward currency, amount range, and a relative weight.

Rebuilt 2026-07-26 to fix a real scarcity bug: exploration used to be a
SINGLE shared, contested site per continent (one player claims it, every
other concurrent player in that continent gets nothing until the next
site randomly spawns - which itself only had a 15%-per-tick chance).
With many concurrent players sharing one continent, most players could
go a very long time without ever successfully claiming a site at all -
which meant they never even reached the discovery roll below, no matter
how many times they tried to dispatch. Exploration is now personal and
always available (subject only to one-dispatch-at-a-time per outpost,
same as before): every dispatch generates its own site, so there's
nothing to race for and nobody else's activity can starve you out.

Reward variety was also expanded from 4 currencies (metal/crystal/ether/
dollars) to all 9 real resources, weighted so everyday materiel is
common and Crystal/Ether stay rare finds - not the ONLY things
exploration ever turns up.
~~~~


## Show any expedition this outpost already has in flight first -

Removed from `internal/bot/handlers/exploration.go:135`.

~~~~text
Show any expedition this outpost already has in flight first -
only one at a time, matching the one-dispatch-per-outpost-at-once
design (keeps this from being spammable).
~~~~


## 2026-08-06: the reward line used to spell out the exact

Removed from `internal/bot/handlers/exploration.go:151`.

~~~~text
2026-08-06: the reward line used to spell out the exact
amount and resource in plain text the instant a player
dispatched - which flatly contradicted the panel's own
pitch two screens back ("has a real chance of making first
contact... every expedition returns with a resource haul")
by resolving all the suspense before the expedition had even
left. Now hidden behind Telegram's tap-to-reveal spoiler
(htmlSpoiler, render.go) while the expedition's still en
route - the actual haul lands, unhidden, in the completion
notification once it's genuinely earned (see engine.go's
resolveExplorationDispatches). A curious player can still
tap to peek early; this isn't a hard lock, just a real
choice between spoiling it themselves or waiting for the
reveal - which is what a spoiler tag is actually for.
~~~~


## HandleDispatchExpeditionCallback fires when a player taps "Launch

Removed from `internal/bot/handlers/exploration.go:199`.

~~~~text
HandleDispatchExpeditionCallback fires when a player taps "Launch
Expedition". Generates a personal exploration site on the spot (see the
explorationSiteTemplate doc comment above for why this is no longer a
shared, contested resource) and immediately dispatches to it - nobody
else's activity can block or race this outpost's own expedition.
~~~~


## Generate this outpost's own personal site - nothing shared, nothing

Removed from `internal/bot/handlers/exploration.go:235`.

~~~~text
Generate this outpost's own personal site - nothing shared, nothing
to race another player for. See the explorationSiteTemplate doc
comment for why this replaced the old shared-site-pool design.
~~~~


## generous window; this site only ever belongs to this dispatch

Removed from `internal/bot/handlers/exploration.go:242`.

~~~~text
generous window; this site only ever belongs to this dispatch
~~~~
