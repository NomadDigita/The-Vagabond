# ADR 0104: Decisions from `internal/bot/handlers/scoutmissions.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/scoutmissions.go` and moved here so the source no longer carries them. Each entry is the original comment.

## scoutQuickDispatchCounts are the one-tap quantity options shown on the

Removed from `internal/bot/handlers/scoutmissions.go:25`.

~~~~text
scoutQuickDispatchCounts are the one-tap quantity options shown on the
panel's inline keyboard - common round numbers rather than an exhaustive
picker, matching the "a handful of good defaults, not a slider" spirit
of every other quantity choice in this game (see e.g. the Workshop's
batch-production buttons).
~~~~


## maxConcurrentScoutMissions caps how many independent scout_missions rows

Removed from `internal/bot/handlers/scoutmissions.go:32`.

~~~~text
maxConcurrentScoutMissions caps how many independent scout_missions rows
an encampment can have active (phase 'searching' or 'returning') at
once. Originally 1 (a hard EXISTS gate); raised to 3 per player request
and AI_AND_SCOUTING_EXPANSION_PLAN.md item 3 - each mission still finds
its own target and returns on its own timeline independently, since the
tick-side processing in internal/engine/tick/scoutmissions.go already
operates per-row (queries every mission in a phase, not "the"
encampment's mission), so raising this cap needed no tick-engine
changes, only the dispatch gate and the status rendering below.
~~~~


## doDispatchScoutMission is the testable core of HandleDispatchScoutMission

Removed from `internal/bot/handlers/scoutmissions.go:43`.

~~~~text
doDispatchScoutMission is the testable core of HandleDispatchScoutMission
and HandleScoutDispatchCallback - no telebot.Context dependency, so both
the slash command and the inline quantity buttons share one code path
(and one set of tests), matching admin.go's doSetTaxRate / jobs.go's
doGhostProtocol convention. Returns the HTML-formatted message to show
and, on failure, a non-nil error alongside a plain-text failure notice.
~~~~


## HandleDispatchScoutMission (/scoutmission <count>) is the slash-command

Removed from `internal/bot/handlers/scoutmissions.go:96`.

~~~~text
HandleDispatchScoutMission (/scoutmission <count>) is the slash-command
entry point - kept for players who prefer typing, alongside the wired
panel below.
~~~~


## HandleScoutDispatchCallback fires when a player taps one of the panel's

Removed from `internal/bot/handlers/scoutmissions.go:123`.

~~~~text
HandleScoutDispatchCallback fires when a player taps one of the panel's
quantity buttons. Mirrors HandleDispatchExpeditionCallback's
respond-then-redraw pattern in exploration.go: a short toast
acknowledgment, then the panel itself resent with the now-updated
mission state.
~~~~


## HandleScoutPanel (button-driven, and also useful as a direct command)

Removed from `internal/bot/handlers/scoutmissions.go:160`.

~~~~text
HandleScoutPanel (button-driven, and also useful as a direct command)
is the wired "mother" screen for long-range scouting - the entry point
missing before this pass: dispatching or checking a mission previously
required already knowing the exact /scoutmission or /scoutstatus slash
syntax, with no button path and no formatting at all.

Shows the in-flight mission status (if any) with no action buttons -
nothing to tap mid-mission, matching HandleExplorePanel's "expedition
en route" read-only state - or, if the outpost is free, an info panel
plus one-tap quantity buttons that dispatch directly.
~~~~


## Only the mute toggle made it in - no scouts to dispatch.

Removed from `internal/bot/handlers/scoutmissions.go:225`.

~~~~text
Only the mute toggle made it in - no scouts to dispatch.
~~~~


## scoutMuteToggleButton renders the Scout panel's one-tap notification

Removed from `internal/bot/handlers/scoutmissions.go:232`.

~~~~text
scoutMuteToggleButton renders the Scout panel's one-tap notification
toggle - 2026-08-06 direct request for a way to mute/unmute long-range
scouting pings "directly without any other things", i.e. right here on
the panel a player is already looking at, not buried three taps deep in
/settings (that toggle still exists too, for anyone who prefers it -
see HandleSettings - both read/write the same mute_scout_status column).
~~~~


## HandleScoutMuteToggleCallback flips mute_scout_status for the tapping

Removed from `internal/bot/handlers/scoutmissions.go:246`.

~~~~text
HandleScoutMuteToggleCallback flips mute_scout_status for the tapping
player and re-renders the Scout panel in place so the button label
(🔔 Unmute / 🔕 Mute) immediately reflects the new state.
~~~~


## renderScoutStatus builds the beautified status block shared by

Removed from `internal/bot/handlers/scoutmissions.go:267`.

~~~~text
renderScoutStatus builds the beautified status block shared by
HandleScoutStatus and HandleScoutPanel, listing every one of this
encampment's currently active missions (up to maxConcurrentScoutMissions)
rather than just the most recent one, and reports how many are active so
callers can decide whether there's still a free dispatch slot.
~~~~


## HandleScoutStatus (/scoutstatus) is the slash-command equivalent of the

Removed from `internal/bot/handlers/scoutmissions.go:330`.

~~~~text
HandleScoutStatus (/scoutstatus) is the slash-command equivalent of the
panel's read-only status view, kept for players who prefer typing.
~~~~
