# ADR 0019: Decisions from `cmd/bot/main.go`

Status: Accepted

These decisions were written as comments in `cmd/bot/main.go` and moved here so the source no longer carries them. Each entry is the original comment.

## handleRecoveredPanic is wired into middleware.Recover (see main()) to

Removed from `cmd/bot/main.go:42`.

The original comment points at main_test.go, which is not in this repository. The decision to recover panics in a named function still matches handleRecoveredPanic.

~~~~text
handleRecoveredPanic is wired into middleware.Recover (see main()) to
stop a single panicking handler from crashing the entire bot process
for every connected player. A named function instead of an inline
closure specifically so it's unit-testable (see main_test.go) -
nothing here should ever itself be able to panic, since a panic inside
the panic recoverer has nothing left to catch it.
~~~~


## aiFactionSeed describes one persistent AI civilization to seed at

Removed from `cmd/bot/main.go:149`.

~~~~text
aiFactionSeed describes one persistent AI civilization to seed at
startup (Phase 6, MMO_WORLD_EVOLUTION_PLAN.md). Deliberately modeled as
a real encampments row (see migrations/032_mmo_ai_civilizations.sql for
why) rather than a special-cased entity, so the entire existing
discovery/targeting/raiding/looting pipeline handles it with zero
changes.
~~~~


## seedAICivilizations creates the persistent AI factions described in

Removed from `cmd/bot/main.go:163`.

~~~~text
seedAICivilizations creates the persistent AI factions described in
MMO_WORLD_EVOLUTION_PLAN.md's Phase 6 if they don't already exist
(idempotent via ai_faction_key). Two per continent, at distinct starting
levels, so the world has real discoverable, raidable, growing AI-run
outposts instead of the single difficulty-matched "Rogue Drone Nest"
dummy target Phase 0-5 relied on. The Rogue Drone Nest itself is left
completely untouched as a fallback for a continent with no other
outpost at all (see resolveExplorationDiscovery in internal/engine/tick
/engine.go).
~~~~


## Registered once at startup rather than via lib/pq's package-level

Removed from `cmd/bot/main.go:298`.

~~~~text
Registered once at startup rather than via lib/pq's package-level
init(), because it wraps lib/pq's own "postgres" driver — see
internal/dbdriver for why (Supabase's PgBouncer transaction-mode
pooler occasionally hands the second half of a parameterized
query to a different backend than the one that parsed it; this
driver retries transparently instead of surfacing that as a
player-facing "temporarily unavailable" error).
~~~~


## Global panic recovery. Without this, any single unhandled panic in

Removed from `cmd/bot/main.go:337`.

~~~~text
Global panic recovery. Without this, any single unhandled panic in
any handler - an index-out-of-range on c.Args() (many call sites
across this codebase index into it without a preceding length
check, relying on the button that triggers them always supplying
enough args - true for normal play, but not a guarantee telebot's
dispatch loop itself enforces), a nil pointer dereference, etc. -
crashes the ENTIRE bot process, disconnecting every player, not
just failing the one interaction that triggered it. telebot has no
built-in recovery of its own; this wraps every handler invocation.
The custom callback both logs with real context (not just the bare
error middleware.Recover's default OnError would produce) and
tries to leave the player with a clear response instead of a
button that spins forever or a message that silently never arrives.
~~~~


## --- AI Foundation wiring (Phase A, independent AI roadmap branch) ---

Removed from `cmd/bot/main.go:397`.

~~~~text
--- AI Foundation wiring (Phase A, independent AI roadmap branch) ---
Provider-agnostic by design: register additional providers here
(OpenAI, Gemini, Qwen, Grok, DeepSeek, Ollama, ...) without
touching internal/ai itself. Mock is always registered last so
the bot degrades gracefully instead of failing when no real
provider key is configured.
~~~~


## --- AI Developer Console wiring (Phase J, independent AI roadmap branch) ---

Removed from `cmd/bot/main.go:454`.

~~~~text
--- AI Developer Console wiring (Phase J, independent AI roadmap branch) ---
Reuses admin.AdminIDs (already parsed above) rather than re-parsing
ADMIN_IDS a second time.
~~~~


## --- AI Command Interpreter wiring (Milestone 3, FEEDBACK_CHANGELOG_NLP_PLAN.md) ---

Removed from `cmd/bot/main.go:460`.

~~~~text
--- AI Command Interpreter wiring (Milestone 3, FEEDBACK_CHANGELOG_NLP_PLAN.md) ---
Constructed after aiService (unlike every handler above, nlp
used to be built before the AI Foundation section existed) so
it can be handed a real interpreter. See internal/game/nlpcommand
for the safety design - this is the only AI feature in the
codebase whose parsed output can directly trigger a game action.
~~~~


## Jobs section buttons (mother/child keyboard pair, see

Removed from `cmd/bot/main.go:580`.

~~~~text
Jobs section buttons (mother/child keyboard pair, see
internal/bot/keyboards/navigation.go's JobsNavigation doc comment -
this is purely additive discoverability on top of the slash
commands above, which keep working unchanged).
~~~~


## AI Advisors section buttons (mother/child keyboard pair, matching

Removed from `cmd/bot/main.go:594`.

~~~~text
AI Advisors section buttons (mother/child keyboard pair, matching
the Jobs pattern above) - purely additive discoverability on top of
the slash commands already registered elsewhere for these same
handlers, which keep working unchanged.
~~~~


## Player Profile section buttons (mother/child keyboard pair) -

Removed from `cmd/bot/main.go:608`.

~~~~text
Player Profile section buttons (mother/child keyboard pair) -
notably gives /settings its first button access ever, despite it
being where the route_status notification mute toggle actually lives.
~~~~


## Phase 7 (item 13): admin panel consolidation's guided-input flow.

Removed from `cmd/bot/main.go:718`.

~~~~text
Phase 7 (item 13): admin panel consolidation's guided-input flow.
If the sender is an admin mid-flow on a button that needs a
free-text argument (e.g. "Gift Premium"), consume this message as
that argument instead of falling through to normal NLP parsing.
Everyone else (and any admin with no pending action) is completely
unaffected - HandleAdminPendingInput returns handled=false
immediately for them.
~~~~


## New-survivor welcome message quick-action buttons. view_warehouse

Removed from `cmd/bot/main.go:745`.

~~~~text
New-survivor welcome message quick-action buttons. view_warehouse
and view_manual are already registered further down (pre-existing
dashboard buttons); only open_agent/open_refer are new here.
~~~~


## BUGFIX: these two dashboard buttons (onboarding.go's returning-user

Removed from `cmd/bot/main.go:760`.

~~~~text
BUGFIX: these two dashboard buttons (onboarding.go's returning-user
panel) had no registered handler at all - tapping them did
nothing, since telebot silently drops a callback with no matching
bot.Handle. Reusing the existing HandleWarehouseReserves/HandleHelp
panels, same "registered as both a command and a callback, no
c.Respond needed" pattern browse_clans above already uses.
~~~~
