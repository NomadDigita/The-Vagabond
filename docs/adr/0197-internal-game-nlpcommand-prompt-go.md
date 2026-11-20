# ADR 0197: Decisions from `internal/game/nlpcommand/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/nlpcommand/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package nlpcommand implements Milestone 3 of

Removed from `internal/game/nlpcommand/prompt.go:1`.

~~~~text
Package nlpcommand implements Milestone 3 of
FEEDBACK_CHANGELOG_NLP_PLAN.md: natural-language command execution,
wired as the final fallback in internal/bot/handlers/nlp.go's
HandleTextMessage chain.

This is deliberately different from every other AI package in this
codebase (governor, fleetcommander, econadvisor, researchplanner,
...): those are advisory-only by design - they recommend, a human
taps a button to act. This package's whole point is to let the AI's
parsed intent result in an actual game action. The safety design
that makes that acceptable:

 1. The AI never touches the database. It has exactly one job: turn
    free text into a ParsedCommand (Action + Args) via a
    tool-calling completion request. This file (prompt.go) has zero
    I/O so it's independently unit-testable, matching every sibling
    AI package's planner.go/prompt.go split. All actual game-state
    mutation happens in internal/bot/handlers, calling the exact
    same doX core functions the button-driven UI already calls
    (doDispatchScoutMission, doPostListing) - same validation, same
    resource checks, same everything.
 2. A fixed action allow-list, not an open-ended tool. See Action
    and ToolDefinitions below - only what's listed there can ever
    be returned as Matched.
 3. Anything that spends resources or commits forces requires an
    explicit Confirm/Cancel step before executing - see
    Action.RequiresConfirmation. Pure reads skip confirmation since
    there's nothing to confirm.
~~~~


## Action is one of the fixed, allow-listed actions this milestone

Removed from `internal/game/nlpcommand/prompt.go:37`.

~~~~text
Action is one of the fixed, allow-listed actions this milestone
implements. Expanding this list is designed to be incremental (add
a tool definition + one dispatch case + one confirmation template)
rather than needing a rewrite - see the plan doc's "Starting action
set" section.
~~~~


## ActionListMarketItem posts a quantity of a tradeable resource

Removed from `internal/game/nlpcommand/prompt.go:45`.

~~~~text
ActionListMarketItem posts a quantity of a tradeable resource
for sale on the player auction market exchange. Mutating -
requires confirmation.
~~~~


## ActionBuyMarketItem searches existing market listings for a

Removed from `internal/game/nlpcommand/prompt.go:49`.

~~~~text
ActionBuyMarketItem searches existing market listings for a
resource and buys the best-matching one against a cash budget.
2026-08-05 addition, following this file's own "add a tool
definition + one dispatch case + one confirmation template"
design for incremental growth - see doBuyMarketItem in
exchange.go for the match/purchase logic and why it's a new
core rather than a reuse of HandleBuyListingCallback (which
buys by a specific already-known listing ID, not by searching).
Mutating - requires confirmation.
~~~~


## ActionCheckResources looks up the player's current resource

Removed from `internal/game/nlpcommand/prompt.go:59`.

~~~~text
ActionCheckResources looks up the player's current resource
stockpile. Read-only - no confirmation needed.
~~~~


## ActionDispatchScoutMission commits scout walkers to a

Removed from `internal/game/nlpcommand/prompt.go:62`.

~~~~text
ActionDispatchScoutMission commits scout walkers to a
long-range search mission. Mutating - requires confirmation.
~~~~


## ActionCheckScoutStatus reports on the player's currently

Removed from `internal/game/nlpcommand/prompt.go:65`.

~~~~text
ActionCheckScoutStatus reports on the player's currently
active scout missions. Read-only - no confirmation needed.
~~~~


## ActionLaunchRaid opens the existing draft/launch-style picker

Removed from `internal/game/nlpcommand/prompt.go:68`.

~~~~text
ActionLaunchRaid opens the existing draft/launch-style picker
for a named, already-discovered target. 2026-08-05 addition -
deliberately does NOT itself commit any forces or spend
anything: it only jumps the player to the same
composition-and-launch-style screen a manual tap on the /raid
target matrix opens, matching that flow's own UX exactly (no
confirm card there either, since the real commitment point is
the player choosing a composition and tapping one of the three
Launch buttons themselves - see
CombatHandler.startRaidDraft's doc comment). Not a "call an
AI to send my army" shortcut - it's a "skip searching the
target matrix by hand" shortcut, and the actual send-forces
decision stays exactly where it already was.
~~~~


## Valid reports whether a is one of the six actions this milestone

Removed from `internal/game/nlpcommand/prompt.go:84`.

~~~~text
Valid reports whether a is one of the six actions this milestone
actually implements - anything else (e.g. a hallucinated tool name)
is rejected rather than dispatched.
~~~~


## RequiresConfirmation reports whether a spends a resource or commits

Removed from `internal/game/nlpcommand/prompt.go:96`.

~~~~text
RequiresConfirmation reports whether a spends a resource or commits
forces and therefore must be shown as a Confirm/Cancel card rather
than executed immediately - see the plan doc's safety rule #2.
ActionLaunchRaid is deliberately NOT here - see its doc comment for
why opening the draft/launch-style picker is not itself a commitment
of anything.
~~~~


## ArgFloat reads a numeric argument. Tool-call args decode as

Removed from `internal/game/nlpcommand/prompt.go:128`.

~~~~text
ArgFloat reads a numeric argument. Tool-call args decode as
float64 in the common case (encoding/json's default for JSON
numbers); json.Number is handled too since some providers may
round-trip through a decoder configured with UseNumber.
~~~~


## ArgInt reads a numeric argument truncated to an int - used for

Removed from `internal/game/nlpcommand/prompt.go:149`.

~~~~text
ArgInt reads a numeric argument truncated to an int - used for
counts/quantities, which are always whole units in this game.
~~~~


## Result is what Interpret returns to the caller: either exactly one

Removed from `internal/game/nlpcommand/prompt.go:155`.

~~~~text
Result is what Interpret returns to the caller: either exactly one
matched, allow-listed ParsedCommand, or a plain-text reply from the
model (a clarifying question, per the system prompt's instruction)
to show the player instead of a raw "not recognized" message.
~~~~


## SystemPrompt instructs the model to act purely as an intent

Removed from `internal/game/nlpcommand/prompt.go:165`.

~~~~text
SystemPrompt instructs the model to act purely as an intent
classifier over the fixed action allow-list - never to answer from
its own knowledge, never to invent an action outside the tool
schema, and to ask a clarifying question in plain text when nothing
in the allow-list genuinely fits (per Milestone 3's "fixed action
allow-list, not an open-ended tool" safety rule).
~~~~


## ParseResponse turns a raw ai.CompletionResponse into a Result. A

Removed from `internal/game/nlpcommand/prompt.go:286`.

~~~~text
ParseResponse turns a raw ai.CompletionResponse into a Result. A
truncated response (see ai.IsTruncatedStopReason) is treated as
unmatched with no clarify text rather than acted on, since a cut-off
tool call's arguments can't be trusted - matching ADR-025's "never
act on a truncated response" discipline elsewhere in this codebase.
~~~~
