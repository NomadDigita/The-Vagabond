# PROJECT_MASTER_PLAN.md

**Read this file before doing anything else.** It is the single source
of truth for the AI-systems branch of The Vagabond. If you are a new
AI agent or human developer picking this up cold, this document alone
should tell you what exists, what's next, and why decisions were made.

---

## 0. Two Roadmaps, One Repository

There are two independent workstreams in this repo:

| Roadmap | Scope | Status | Owner |
|---|---|---|---|
| **SpaceHunt Roadmap** (Phases 1–6) | Player identity/social, guild improvements, economy, gameplay jobs, buildings, combat | In progress elsewhere | A separate AI workspace/session |
| **AI Systems Roadmap** (Phases A–J) | Provider-agnostic AI infrastructure and every AI-driven gameplay feature built on top of it | **Phase A shipped**, B–J not started | This document's lineage |

**Rule: this branch never touches SpaceHunt phase code or tables unless
fixing a hard blocking bug.** Everything AI-related lives under
`internal/ai/` (pure infrastructure, zero game dependencies) plus a
thin, clearly-marked integration seam in `cmd/bot/main.go` and one new
handler file `internal/bot/handlers/ai_status.go`. New command names
only (`/ai_status`, `/ai_status_toggle`, `/ai_settings`) — nothing that
could collide with a SpaceHunt command.

Assume the SpaceHunt branch will eventually be merged into this one (or
vice versa). That is why `internal/ai` was built with **zero imports
from `internal/bot`, `internal/engine`, or `internal/game`** — it only
depends on `database/sql` and the stdlib. Any Phase B–J feature that
needs game state (fleets, planets, resources) should read that state
through the existing handlers/DB layer and pass it into `internal/ai`
as plain data, never the reverse.

---

## 1. Current Status (as of this session)

**Phase A — AI Foundation: COMPLETE.**

Everything below phases B–J is **not started**. Do not assume any
Governor/Fleet Commander/etc. logic exists yet — Phase A only built the
plumbing all of those will eventually sit on.

### What was built

```
internal/ai/
├── types.go                     Provider-agnostic Message, ToolDefinition,
│                                 ToolCall, CompletionRequest/Response, Usage
├── config.go                    Env-driven Config + Feature flag constants
├── registry.go                  Provider registry with fallback ordering
├── cache.go                     Cache interface + in-memory TTL implementation
├── cost.go                      CostTracker interface + Postgres implementation,
│                                 static per-provider/model price table,
│                                 EstimateCostUSD()
├── permission.go                PermissionManager: global feature flags +
│                                 per-user opt-out, both DB-backed
├── memory.go                    MemoryStore interface + Postgres implementation
│                                 (scoped conversational history)
├── service.go                   Service: the ONE entry point Phase B-J code
│                                 should call. Wires kill-switch → permissions
│                                 → cache → budget → provider fallback → cost
│                                 recording, in that order.
├── service_test.go               6 passing unit tests using fakes (no network,
│                                 no DB) covering happy path, fallback,
│                                 unavailable-provider skipping, caching,
│                                 kill switch, and input validation.
└── providers/
    ├── mock/provider.go         Zero-dependency deterministic provider.
    │                             Always Available(). Always last in the
    │                             fallback chain so the bot never hard-fails
    │                             when no real API key is configured.
    └── anthropic/provider.go    Real implementation against
                                  https://api.anthropic.com/v1/messages
                                  (text + tool_use blocks parsed; JSON mode
                                  via system-prompt instruction since
                                  Anthropic has no native json_mode field).
```

**Database (migrations):**
- `migrations/020_vagabond_ai_foundation.sql` — annotated standalone
  copy of the schema (for readers), **but the schema that actually runs
  on boot is the copy inlined into the `migrations` slice inside
  `cmd/bot/main.go`'s `executeStartupMigrations()`**. This repo does
  **not** execute `.sql` files from disk — confirm this hasn't changed
  before assuming otherwise. Tables: `ai_feature_flags`,
  `ai_permissions`, `ai_memory`, `ai_cost_log`. All prefixed `ai_`, all
  additive, none touch SpaceHunt tables.

**Bot integration (`cmd/bot/main.go`):**
- Imports `internal/ai` + the two Phase A providers.
- Builds one `*ai.Service` at boot (`aiService`), registers Anthropic
  then Mock into the registry (mock is the safety net).
- New handler `AIStatusHandler` wired to three commands:
  - `/ai_status` — admin-only. Shows provider availability, global
    feature-flag states, today's global spend vs. budget.
  - `/ai_status_toggle <feature> <on|off>` — admin-only. Flips a
    global feature flag live, no redeploy.
  - `/ai_settings [feature] [on|off]` — any player. Lets a player opt
    themselves out of a specific AI feature (their choice is always
    respected on top of the global flag).

**Verification performed this session:**
- `go build ./internal/ai/...` — clean (this subtree has zero external
  module deps, so it builds even in network-restricted sandboxes).
- `go test ./internal/ai/... -v` — 6/6 tests pass.
- `gofmt -d` on every new file — zero diffs (all already
  gofmt-clean); `cmd/bot/main.go`'s only gofmt complaint is a
  pre-existing missing trailing newline, unrelated to this session's edits.
- **Not verified:** a full `go build ./...` of the whole binary,
  because this sandbox's network egress allowlist does not include
  `proxy.golang.org` or `gopkg.in` (needed to fetch `telebot.v3`,
  `lib/pq`, `godotenv`). **The very first thing the next session should
  do is run `go build ./... && go vet ./...` in an environment with
  normal Go module proxy access**, since the `cmd/bot/main.go` edits
  were hand-verified but never compiled end-to-end against the real
  `telebot.v3` API.

---

## 2. Architecture Decision Records (ADRs)

**ADR-001: `internal/ai` has zero dependency on game packages.**
Rationale: it must be safely mergeable regardless of what the parallel
SpaceHunt branch does to `internal/bot`/`internal/engine`/`internal/game`.
Consequence: Phase B–J features live in *new* files (likely a new
`internal/game/aigov`, `internal/game/aifleet`, etc., or as new handler
files) that import `internal/ai` and the existing game DB layer, never
the other way around.

**ADR-002: The DB schema actually executed lives in `cmd/bot/main.go`,
not in the `migrations/*.sql` files.** Discovered this session — the
`.sql` files appear to be historical/documentation only; `main.go`'s
`executeStartupMigrations()` embeds every `CREATE TABLE IF NOT EXISTS`
as a Go string literal and that's what runs on boot. Phase A's schema
was added to **both** places (the standalone `.sql` for readability,
the inline copy because it's the one that matters) to keep the
convention. **Future sessions: verify this is still true before adding
a migration** — if a real migration runner gets introduced later, this
whole section is stale and the inline copy becomes redundant/wrong.

**ADR-003: Mock provider is always registered and always last.**
Rationale: "fallback logic" was an explicit requirement, and a game
server must never hard-fail an AI-touching command just because no
API key is configured (e.g. local dev, a fresh deploy before secrets
are set). The mock provider returns a clearly-labeled placeholder
response and still exercises the full cost/cache/permission pipeline.

**ADR-004: Cost tracking uses a static, hardcoded price table
(`pricePerMillionTokens` in `cost.go`), not a live pricing API.**
Rationale: simplicity and zero extra network dependency for Phase A.
Consequence: **prices will drift from reality over time** — flagged as
technical debt below.

**ADR-005: JSON mode for Anthropic is implemented via a system-prompt
instruction, not a native API parameter.** Anthropic's Messages API has
no `response_format: json` field (unlike OpenAI). Consequence: for
Phase J (Developer Console) and Phase F (Battle Analyst), which need
strict structured output, consider layering a JSON-Schema-validating
retry loop on top of `Service.Complete` rather than trusting the model
to always comply — this was **not** built in Phase A and should be a
first task of whichever phase needs it.

**ADR-006: `internal/ai` tool-calling is defined (`ToolDefinition`,
`ToolCall` types) but no tool-execution loop exists yet.** The
Anthropic provider will happily return `tool_use` blocks in
`CompletionResponse.ToolCalls`, but nothing in Phase A calls tools,
feeds results back, and loops. That orchestration (an "agent loop")
is deliberately left for whichever Phase B–J feature needs it first,
since different features will want different loop-termination
policies (e.g. Fleet Commander should never loop more than once
without human approval; Developer Console might loop several times).

---

## 3. Full AI Systems Roadmap (Phases A–J)

| Phase | Name | Status | Depends on |
|---|---|---|---|
| A | AI Foundation | Done | — |
| B | AI Planet Governor | Not started | A |
| C | AI Fleet Commander | Not started | A |
| D | AI Economy Advisor | Not started | A |
| E | AI Research Planner | Not started | A |
| F | AI Battle Analyst | Not started | A |
| G | AI Guild Assistant | Not started | A |
| H | AI Dynamic Galaxy | Not started | A |
| I | AI NPC Intelligence | Not started | A, ideally after G |
| J | AI Developer Console | Not started | A |

**Progress by subsystem:** Foundation 100%. All gameplay-facing AI
phases (B–J): 0%.

### Recommended next task

**Phase B — AI Planet Governor**, because:
1. It's the simplest Phase B–J feature to scope (single-planet
   optimization, no multi-agent coordination, no combat math).
2. It will be the first real consumer of `ai.Service`, `MemoryStore`,
   and `PermissionManager` together, and will surface any Phase A
   rough edges before more complex phases (Fleet Commander, Battle
   Analyst) build on top of the same foundation.
3. It naturally needs a "player approval" UX pattern (recommend now,
   act only if the player confirms or has enabled autopilot) that
   every later phase (C, D, G) will reuse — worth getting right once.

**Before writing Phase B code:** read `internal/game/content` and the
`camp.go` / `agent.go` handlers to understand the existing building
and automation-agent data model. Phase B should almost certainly reuse
the existing `agent_tasks` table's on/off pattern rather than
inventing a parallel automation toggle.

---

## 4. Known Issues / Technical Debt

- **Full binary was never compiled this session** (see §1 — sandbox
  network couldn't reach `proxy.golang.org`/`gopkg.in`). The
  `cmd/bot/main.go` wiring edits are believed correct (mirrors the
  exact pattern every other handler uses) but must be confirmed with a
  real `go build ./...` before deploying.
- **Static cost table will go stale.** `internal/ai/cost.go`'s
  `pricePerMillionTokens` map needs periodic manual updates as
  Anthropic (and future providers) change pricing. Consider moving
  this to an env-var-configurable table if it becomes painful.
- **No tool-execution loop.** See ADR-006. Any phase that wants the
  model to actually call a tool and see the result needs to build that
  loop; `Service.Complete` only returns the requested tool calls, it
  does not execute them.
- **No JSON-Schema validation/retry for structured output.** See
  ADR-005. Needed before Phase F/J can be trusted for anything
  machine-consumed downstream.
- **`InMemoryCache` is process-local.** If the bot ever runs as more
  than one instance/process, cache hits won't be shared across
  instances (harmless — just a missed optimization, not a
  correctness bug, since cache is purely an optimization layer that
  Service treats as best-effort).
- **Anthropic provider maps `RoleTool` messages to a plain `user` turn**
  rather than Anthropic's native `tool_result` content-block format
  (see the `Complete` method in `providers/anthropic/provider.go`).
  This is a placeholder simplification — fine for Phase A (which has no
  tool-execution loop yet, per ADR-006), but **must be fixed to use
  proper `tool_result` blocks before Phase B or later builds an actual
  tool-calling agent loop**, or multi-turn tool conversations will
  silently lose structure.

## 5. Risks / Blockers

- **No `ANTHROPIC_API_KEY` is configured anywhere in this environment.**
  Every Phase A code path was designed and tested to work with this
  (mock fallback), but nobody has yet exercised the real Anthropic
  provider against a live API key. First Phase B session should set
  one (even a personal/test key) and manually sanity-check
  `/ai_status` shows `anthropic` in the provider list.
- **This session could not push to GitHub.** The repo owner offered a
  Personal Access Token directly in chat; per this assistant's fixed
  operating rules, credentials pasted into a conversation are never
  used to authenticate, regardless of how the account is described.
  **All work in this section exists only as local commits in this
  session's sandbox / a git bundle handed to the user** — it has not
  been pushed to `https://github.com/NomadDigita/The-Vagabond` and
  will not exist there until the repo owner pushes it themselves (see
  the handoff instructions given in-chat). If you're a future session
  reading this file *from the actual GitHub repo*, that means the push
  succeeded — if you're reading it any other way, it may still be
  sitting unpushed.

## 6. Change Log

- **This session:** Phase A (AI Foundation) implemented in full:
  provider abstraction, Anthropic + mock providers, config/feature
  flags, in-memory cache, Postgres-backed cost tracking + budgets,
  Postgres-backed permission system, Postgres-backed memory store,
  unifying `Service`, 3 new bot commands, DB schema, 6 passing unit
  tests. This file created.

## 7. Future Ideas (unscoped, not committed to any phase)

- A `SummarizingMemoryStore` decorator (per the note in `memory.go`)
  that periodically compacts old `ai_memory` rows via an LLM call, for
  Phases G/I which will want longer-lived context than raw
  transcript replay.
- Structured-output validation helper (`ai.CompleteJSON[T any](...)`
  using generics) that marshals into a caller-supplied struct and
  retries once on invalid JSON — would remove the ADR-005 debt cleanly
  for whichever phase needs it first.
- A real tool-execution loop type (`ai.AgentLoop`) built on top of
  `Service`, generic enough for Fleet Commander, Guild Assistant, and
  Developer Console to each supply their own tool set and stopping
  condition.

## 8. Integration Notes for the Parallel (SpaceHunt) Branch

- This branch added exactly one contiguous, clearly-commented block to
  the shared `migrations` slice in `cmd/bot/main.go`, plus one
  contiguous block of `bot.Handle(...)` registrations. Both are easy
  to keep or drop in a merge — they don't interleave with SpaceHunt's
  edits to the same file beyond needing a straightforward textual
  merge if SpaceHunt also touches `main.go`.
- No SpaceHunt table was renamed, altered, or dropped.
- No SpaceHunt command name was reused.
- If SpaceHunt's roadmap ever wants AI assistance for guild
  leadership (their Phase 2) or economy (their Phase 3), that's Phase
  G (AI Guild Assistant) and Phase D (AI Economy Advisor) respectively
  — coordinate before either side builds it twice.

---

## 9. How to Resume Work (for the next session, AI or human)

1. Read this whole file first.
2. Run `go build ./... && go test ./... && go vet ./...` with normal
   network access — confirm Phase A still compiles end-to-end (this
   session could only verify the `internal/ai` subtree in isolation;
   see §1 and §4).
3. Pick up the "Recommended next task" in §3 (Phase B) unless the
   project owner has redirected you.
4. Before writing code for any phase: inspect the relevant existing
   handler/engine code first (see the phase-specific note in §3),
   confirm extension points, and only then implement — incrementally,
   not as a rewrite.
5. **After finishing any task, update this file**: move it from
   "Not started" to "In progress"/"Done" in §3's table, add an ADR if
   you made a non-obvious design call, add to the Change Log in §6,
   and update "Recommended next task." This file is only useful if
   every session leaves it accurate for the next one — treat updating
   it as part of the task, not an optional afterthought.
