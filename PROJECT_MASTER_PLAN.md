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

## 1.5 Production Hardening + Multi-Provider Ecosystem (this session)

This session did not add a new lettered phase. It (a) fixed a real bug
found live in production, (b) addressed UX gaps the project owner
flagged directly from the live bot, and (c) built out the
provider-agnostic ecosystem Phase A's design always intended, so real
(non-mock) output is one API key away for six different providers.

### Production bugfix: `/economy_advisor` crash

**Confirmed root cause**, not guessed: `cmd/bot/main.go`'s
`CREATE TABLE IF NOT EXISTS resources` statement lists `ether` and
`neuro_cores` in its column list, but on any database where the
`resources` table already existed *before* those two columns were
added to that list (i.e. the live production database), `CREATE TABLE
IF NOT EXISTS` is a no-op — so those columns were silently never
created there. Nothing in the migration list had a corresponding
`ALTER TABLE ADD COLUMN`. `internal/game/econadvisor` reads `ether`,
which crashed with `column "ether" does not exist` — confirmed via the
project owner's own screenshot of the live error. `internal/game/governor`
reads `neuro_cores`, which had the identical latent gap but hadn't
been hit yet.

**Fix:** two new, idempotent, defensively-placed statements —
`ALTER TABLE resources ADD COLUMN IF NOT EXISTS ether ...` and the
same for `neuro_cores` — added to the embedded migration list right
after the existing resources rename/drop block. Safe to run against
any database state (brand new or years old).

### Mock provider output quality

**Confirmed root cause** (via the project owner's screenshots): every
Phase B/C/D handler requests `JSONMode: true` and parses the response
through `ParseRecommendation`, which falls back to dumping raw text
verbatim if JSON parsing fails. The mock provider (correctly, per
ADR-003) never made a real API call, but it also never returned valid
JSON — so every mock response fell through to the raw-text path,
producing the "sooo worst" (project owner's words) output visible in
the screenshots: a literal untruncated dump of the prompt data.

**Fix:** `internal/ai/providers/mock/provider.go` now returns a valid,
clearly-labeled placeholder JSON object per feature (governor,
fleet_commander, economy_advisor), matching each phase's real
`json:"..."` tag shape. This is verified, not assumed —
`internal/ai/providers/mock/placeholder_test.go` calls each phase's
real `ParseRecommendation` against the mock's output and asserts it
parses cleanly (not a fallback), so the formatted, non-raw template
renders even with zero API keys configured. If a phase's JSON shape
changes in the future, this test will fail loudly rather than silently
regressing back to raw-text mock output.

### Inline keyboards

The project owner flagged that `/governor`, `/fleet_commander`, and
`/economy_advisor` had no inline keyboard, unlike the rest of the bot's
UI. Fixed:
- `/governor` — 🔄 Refresh Analysis + 🛠️ Autopilot toggle (label
  reflects current stored preference; toggling still does not cause
  any autonomous action, per ADR-007 — the button and the text command
  both say so explicitly).
- `/fleet_commander` — 🔄 Refresh Analysis.
- `/economy_advisor` — 🔄 Refresh Analysis.

Each refresh button triggers a genuine new `ai.Service.Complete` call
(subject to the same cost/cache/budget rules as any other call), not a
replay of the same cached text. Callback unique names
(`gov_refresh`, `gov_toggle_autopilot`, `fleet_refresh`, `econ_refresh`)
follow the existing `\f`-prefixed convention already used by
`combat.go`'s inline buttons.

### Multi-provider ecosystem

Per the project owner's explicit request for "as many LLMs as
possible, each with as many models as possible," four new providers
were added on top of the existing Anthropic + mock:

- **`internal/ai/providers/openaicompat`** — one implementation
  covering **OpenAI**, **DeepSeek**, **Qwen** (Alibaba DashScope,
  OpenAI-compatible mode), and **Grok** (xAI), since all four expose
  the same Chat Completions wire format. Verified end-to-end
  (`provider_test.go`, 4 tests) against a local `httptest` server —
  this caught and fixed a real bug: the JSON-mode instruction was
  silently dropped whenever a caller passed no `System` prompt (every
  current Phase B/C/D caller always sets one, so this hadn't bitten
  yet, but would have the moment a caller didn't).
- **`internal/ai/providers/gemini`** — Google's native
  `generateContent` API (distinct wire format: `"model"` role instead
  of `"assistant"`, top-level `systemInstruction`, `responseMimeType`
  for JSON mode). Verified end-to-end (`provider_test.go`, 5 tests)
  against a local server, including the confirmed-necessary role
  mapping. **Caught a real bug before it shipped**: the initial default
  model was `gemini-2.0-flash`, which Google shut down 2026-06-01 —
  confirmed via web search, not assumed; fixed to default to
  `gemini-2.5-flash`.
- **`internal/ai/providers/ollama`** — self-hosted open-weight models
  via Ollama's native `/api/chat` endpoint. No API key required
  (`Available()` only checks a base URL is set). Verified end-to-end
  (`provider_test.go`, 5 tests) including an unreachable-host case.
  **This is the concrete path toward the project owner's "supermini
  LLM, thinks itself, less dependency on outside LLMs" ask** — but see
  the honest caveat in §5: capable open-weight inference needs real
  compute (RAM/GPU), which this provider does not manufacture out of
  nothing.

**Cost table (`internal/ai/cost.go`)** was rebuilt with prices verified
by live web search on 2026-07-15 (not estimated) for every provider:
Anthropic Sonnet 4.6 ($3.00/$15.00 — unchanged, confirmed still
accurate), OpenAI gpt-4o-mini ($0.15/$0.60), DeepSeek deepseek-v4-flash
($0.14/$0.28 — **note:** the legacy `deepseek-chat` alias retires
2026-07-24, do not configure that model name), Qwen qwen-plus
(~$0.40/$1.20, flagged as approximate — DashScope pricing has
request-length tiering), Grok grok-4-fast (~$0.20/$0.50, flagged as
the least certain row — xAI's model naming has shifted repeatedly
through 2026), Gemini gemini-2.5-flash ($0.30/$2.50), Ollama (genuinely
$0/$0 — compute cost instead of API cost).

**`internal/ai/config.go`** gained `OPENAI_API_KEY`/`OPENAI_MODEL`,
`DEEPSEEK_API_KEY`/`DEEPSEEK_MODEL`, `QWEN_API_KEY`/`QWEN_MODEL`/
`QWEN_BASE_URL`, `GROK_API_KEY`/`GROK_MODEL`, `GEMINI_API_KEY`/
`GEMINI_MODEL`, `OLLAMA_BASE_URL`/`OLLAMA_MODEL` — every one optional,
every provider activates the moment its own key (or, for Ollama, base
URL) is set, with zero other code change needed. `cmd/bot/main.go`
registers all seven providers (five new + Anthropic + mock) into the
registry at boot; `Registry.Ordered()`'s existing "unlisted-but-
available providers are appended last" behavior (see `registry.go`)
means a freshly-configured provider is reachable even if
`AI_FALLBACK_PROVIDERS` isn't updated to mention it explicitly.

**Verification performed this session:** `gofmt`, `go build`, `go vet`,
and `go test` all clean across every touched package — 43 tests total
(was 26 at end of Phase D), zero external module dependencies added to
any of the new provider packages (all use only `net/http`,
`encoding/json`, and stdlib). Full `./...` build against the real
`telebot.v3`/`lib/pq` dependency tree still not verified in this
sandbox — same standing blocker noted in every phase above.

---

## 1.6 Deep Schema-Drift Audit (following session, prompted by project owner)

The project owner reported (via screenshots) that `/economy_advisor`
was still crashing and mock output was still showing on the live bot,
and asked for a genuinely deep review rather than another one-off
patch. Two things were true at once, and it's worth being precise
about which was which:

**Not new breakage:** both symptoms in the screenshots were already
fixed by the §1.5 commit — they just hadn't been pushed/merged/
deployed yet. Confirmed via `git fetch origin main`: that commit was
still local-only in the working sandbox.

**A real, separate discovery:** fetching the true current `origin/main`
revealed it had moved forward substantially since this project's
Phase A–D work was merged (PR #16) — the parallel SpaceHunt workspace
had since merged **Phase 6** (six new turret types, new ships:
Liberator/Wraith/Observer/Guardian, Piercing Missile, three Cargo Ship
tiers) and an unplanned **Phase 7** ("Rogue AI scaling via real player
subsystems" — the PvE rogue nest now has a full per-turret Defense
Grid, Guardian/Observer garrison, research level, shields, and an
optional hero-equivalent superpower, not just a flat bonus).

This was exactly the right thing to check for, because Phase C (Fleet
Commander) depends on both `workshop_inventory`'s column set and
`internal/game/content.RogueNestComposition`'s output shape — both of
which Phase 6/7 changed.

### Audit method (confirmed via git history, not guessed)

For every table `internal/game/governor`, `internal/game/fleetcommander`,
and `internal/game/econadvisor` read from, the definitive current
column set was extracted programmatically from `cmd/bot/main.go` (union
of `CREATE TABLE` columns and every `ALTER TABLE ... ADD COLUMN`), then
cross-referenced against `git log` to find each table's *original*
column set at first creation. A column present now but absent at
creation, with no `ALTER TABLE ADD COLUMN` guard, is exactly the bug
class that caused the `ether`/`neuro_cores` crash.

**Result — one crash-class bug found (already fixed in §1.5), zero new
ones:** `resources.ether`/`resources.neuro_cores` remain the only
columns anywhere in the schema added to a `CREATE TABLE IF NOT EXISTS`
column list without a corresponding `ALTER TABLE ADD COLUMN` guard.
Every other newer column across `encampments`, `raids`,
`research_states`, `bank_accounts`, and `workshop_inventory` — all
tables this AI roadmap's code reads from — was confirmed to have a
proper `ALTER TABLE ADD COLUMN IF NOT EXISTS` guard. `modules` and
`market_exchange` were confirmed to have had every one of their current
columns present since their very first commit (`d38ca0f`) — genuinely
zero risk, not merely unchecked.

**Result — two confirmed analysis-quality gaps found and fixed (not
crashes, but real correctness gaps):**

1. **`fleetcommander.unitColumns` was missing 10 real
   `workshop_inventory` columns** added by Phase 6/7 (`liberators`,
   `wraiths`, `observers`, `guardians`, `piercing_missiles`,
   `cargo_mk1`/`2`/`3`, `garrisoned_soldiers`, `garrisoned_mechs`) —
   confirmed by extracting the full 28-column set and diffing against
   the hardcoded 18-column list. This never crashed (all 10 columns
   are properly `ALTER`-guarded), but Fleet Commander was silently
   blind to a meaningful slice of a player's actual fleet strength —
   including several capital-ship-tier units. **Fixed:** the list now
   includes all 28 confirmed columns, with a comment documenting how
   to re-verify it's still complete after a future SpaceHunt combat
   merge.
2. **`fleetcommander.TargetProfile`/`BuildRogueNestTarget` only used
   the legacy flat `TurretBonus` field**, ignoring the new
   per-turret Defense Grid, Guardian/Observer garrison, Integrity Tech
   level, shields, and hero-superpower data Phase 7 added to
   `content.RogueNestForce`. Confirmed by reading the actual Phase 7
   diff. This directly undermined Phase C's core purpose — accurately
   assessing target strength. **Fixed:** `TargetProfile` gained
   `TurretGrid`, `IntegrityTechLvl`, `Shields`, `HeroSuperpower` fields,
   all populated in `BuildRogueNestTarget` and rendered in
   `BuildUserPrompt`; the system prompt now explicitly calls out a
   hero superpower as a risk factor. Verified with a new test
   (`TestBuildRogueNestTarget_IncludesEnrichedDefenseData`) that
   exercises every enrichment threshold at level 20 and asserts the
   data reaches the actual rendered prompt, not just the struct.

### Branch history note

The `ai-foundation-phase-a` branch (this session's local work) was
rebased onto the true current `origin/main` (picking up SpaceHunt
Phase 6/7) with **zero merge conflicts** — a good sign the file
boundaries between the two roadmaps (per §0's isolation rule) held up
under real divergence, not just in theory.

**Verification performed:** `gofmt`/`go build`/`go vet`/`go test` all
clean; 44 tests total (was 43). The audit script/method itself is
reproducible — re-run the same `CREATE TABLE` ∪ `ALTER TABLE ADD
COLUMN` cross-reference against `git log` for any table before trusting
new AI-roadmap code against it.

---

## 1.7 Critical Fix: Mock Provider Always Won + Model-Generation Drift (following session)

The project owner deployed the §1.5/§1.6 work, set `GEMINI_API_KEY`
and `QWEN_API_KEY` on Render, and confirmed via `/ai_status` that both
providers reported `available ✅`. But `/governor` still returned mock
placeholder output. This was a real, separate bug — not a deployment
issue, not a re-run of the `ether` bug.

### Confirmed root cause

`ai.Registry.Ordered()` returned providers in the order given by
`Config.DefaultProvider` + `Config.FallbackOrder`, which **default to
`"mock"`** (`AI_DEFAULT_PROVIDER=mock`, `AI_FALLBACK_PROVIDERS=mock`)
when unset — and nothing in this project's setup instructions ever
told the project owner they needed to set those two variables once a
real provider key was added. Because the mock provider is always
`Available()` and its `Complete` never returns an error,
`Service.Complete`'s fallback loop returned immediately after mock on
every single call — Qwen and Gemini, despite being registered,
available, and correctly configured, were **structurally unreachable**
regardless of any key being set. ADR-003 always documented the
*intent* that "mock is always last," but no code enforced that
invariant — it only happened to be true by accident of config.

### Fix

`Registry.Ordered()` now enforces "the provider named `mock` is always
placed last" unconditionally, regardless of its position in the
configured order — moving the invariant from a documentation comment
(ADR-003) into actual, tested code. Three new regression tests in
`internal/ai/registry_test.go` directly reproduce the exact failure
condition (mock listed first, alongside other available providers) and
assert mock is never reachable before a real available provider.
**This is the fix that makes a configured API key work immediately,
with zero additional `AI_DEFAULT_PROVIDER`/`AI_FALLBACK_PROVIDERS`
configuration ever required** — closing the gap that caused the
project owner's confusion.

### Confirmed model-generation drift (a second, related finding)

Prompted by the project owner asking about "Gemini 3.5 Pro," a fresh
round of web-search verification (2026-07-16) found the AI model
landscape had moved meaningfully since the §1.5 pricing table was
built just one day earlier (2026-07-15) — this space moves fast enough
that even a one-day-old default can already be the wrong generation:

- **Gemini**: Google shipped an entirely new 3.5 generation at I/O on
  2026-05-19. `gemini-2.5-flash` (this project's prior default) is
  superseded by `gemini-3.5-flash`, now GA. **Gemini 3.5 Pro — what the
  project owner asked about by name — is confirmed NOT yet generally
  available**, still in limited enterprise preview, with an unconfirmed
  rumored July 17, 2026 GA date circulating in the press. Fixed:
  default updated to `gemini-3.5-flash`; do not configure
  `GEMINI_MODEL=gemini-3.5-pro` until independently reconfirmed GA.
- **OpenAI**: GPT-5.6 reached general availability 2026-07-09,
  superseding the `gpt-4o` family this project defaulted to. Fixed:
  default updated to `gpt-5.6-luna` ($1.00/$6.00 per million, the
  cheapest current-generation tier); `gpt-4o-mini`/`gpt-4o` pricing
  rows kept in the cost table since they likely still work if
  explicitly configured.
- **Grok**: re-confirmed the existing `$0.20/$0.50` cheap-tier pricing
  still holds (cited as "Grok 4.1 Fast" across independent trackers),
  though xAI's flagship has moved to Grok 4.5 ($2.00/$6.00, launched
  2026-07-08). No default change needed here, but the exact model-ID
  string remains the least certain part of this codebase's provider
  configuration — confirm at https://docs.x.ai/docs/models before
  trusting `GROK_MODEL` in production.
- **Anthropic** and **DeepSeek** defaults were re-checked and remain
  accurate as of 2026-07-16 — no change needed.

**Verification:** `gofmt`/`go build`/`go vet`/`go test` all clean; 47
tests total (was 44), all passing. The 3 new registry tests are a
direct, permanent regression guard for the mock-priority bug
specifically — not just a passive count increase.

---

## 1.8 Critical Fix: Real Provider JSON Responses Silently Fell Back to Raw Text (following session)

The project owner deployed Phase E (this branch) and, before testing
it, sent a live screenshot of `/governor`, `/economy_advisor`, and
`/fleet_commander` output from the production bot. All three showed a
literal, unformatted JSON blob (`{ "summary": "..." }`) instead of the
emoji-formatted report every one of those commands is supposed to
produce — flagged, correctly, as looking "generic" and unfinished.

### Confirmed root cause

Every one of `governor.ParseRecommendation`,
`fleetcommander.ParseRecommendation`, `econadvisor.ParseRecommendation`
(and this session's own new `researchplanner.ParseRecommendation`)
used the same brittle two-step cleanup: strip a leading/trailing
` ```json ` fence, then call `json.Unmarshal` on the *entire* remaining
string. That only works if the model's response is **exactly** one
JSON object and nothing else. In production, real providers (Qwen via
DashScope, Gemini) — despite every provider's JSONMode already
appending an explicit "respond with a single valid JSON object and
nothing else" instruction — sometimes:

1. add a sentence of prose before and/or after the object anyway
   (`"Here's the analysis: {...}"` or `"{...}\n\nLet me know if you'd
   like more detail!"`), which makes `json.Unmarshal` reject the whole
   string as invalid (trailing/leading data); or
2. leave a raw, unescaped newline or tab character inside a string
   value (most often a multi-sentence `"summary"`) instead of encoding
   it as `\n`/`\t` — valid enough for a "relaxed" JSON reader, but
   rejected outright by Go's strict `encoding/json`.

Either failure tripped the existing `err != nil` fallback, which
displayed the raw model text completely unformatted — exactly the
"generic," unbeautified output in the screenshot. This was worse than
the risk ADR-005 already flagged (that risk was scoped to *future*
Phase F/J structured-output needs) — it was live today in three
already-shipped phases.

### Fix

Added `internal/ai/jsonrecovery.go`: `ExtractJSONObject` (scans for the
first balanced top-level `{...}` object, tracking string/escape state
so quotes and braces *inside* string values don't confuse the
boundary — this is what discards any leading/trailing prose) and
`SanitizeJSONControlChars` (re-escapes a raw newline/tab/carriage
return found while inside a string literal, leaving formatting
whitespace *between* JSON tokens untouched). This is shared
`internal/ai` infrastructure, not duplicated per package — see
ADR-015 for why that's the right call here specifically, in contrast
to this project's usual per-package type-duplication pattern.

All four `ParseRecommendation` functions
(governor/fleetcommander/econadvisor/researchplanner) now: extract the
JSON object first, attempt to unmarshal it, and if that fails, retry
once against the control-char-sanitized version, before finally
falling back to raw text. **The raw-text fallback path itself was also
improved** in all four packages' `FormatForTelegram`: instead of
dumping the model's raw text bare, it's now prefixed with a clear
"⚠️ Couldn't parse..." notice and wrapped in a Telegram monospace code
block — so even the genuine last-resort case (a model producing
actually-wrong-shaped output, not just mis-escaped valid output) no
longer looks like a broken/half-finished feature.

**Verification:** 14 new tests in `internal/ai/jsonrecovery_test.go`
(bare JSON, fence-wrapped, leading prose, trailing prose, both,
brace-inside-string-value, escaped-quote-inside-string, nested object,
no-JSON-at-all, truncated-never-closes, raw-newline-escaped,
formatting-whitespace-left-alone, tab/CR handled,
already-escaped-not-double-escaped) plus 2 new regression tests per
feature package (8 total) directly reproducing the exact
trailing-prose and raw-newline failure modes this session found live.
87 tests total (was 65), all passing. Full `go build ./... && go vet
./... && go test ./...` confirmed clean for the whole repo (same
sandbox-only, reverted-before-commit `go.mod` replace workaround as
Phase E — see that phase's note for why).

---

## 1.9 Critical Fix: Truncated Responses Were Indistinguishable From Genuinely Unparseable Ones (following session)

ADR-015 closed the "prose-wrapped or mis-escaped but otherwise valid
JSON" failure mode, but it left a related, narrower case unaddressed:
when a real provider's response was cut off **mid-object** — because
it hit the `MaxTokens` cap before finishing — `ExtractJSONObject`
correctly reports `found=false` (there's no balanced object to
extract), and every package's `ParseRecommendation` falls back to raw
text exactly as it would for a response that never contained JSON at
all (a refusal, an off-topic reply, etc.). Both cases produced the
same generic "⚠️ Couldn't parse the AI's structured response" message,
even though a truncated response is a distinguishable, more actionable
situation — the model was on the right track, it simply ran out of
room.

### Fix

Two parts, both scoped to the same root cause:

1. **Raise `MaxTokens` from 1024 to 2048** in all four packages
   (`governor.go`, `commander.go`, `advisor.go`, `planner.go`) —
   reduces how often the model hits the cap before finishing a
   normal-length recommendation in the first place.
2. **Distinguish truncation from other unparseable responses.** Added
   `ai.WasTruncated(text)` to `internal/ai/jsonrecovery.go`: it re-runs
   the same string/escape-aware brace scan as `ExtractJSONObject`, and
   reports `true` only when an opening `{` was found but the braces
   never balanced by the end of the text — the structural signature of
   a `MaxTokens` cutoff, as opposed to prose with no JSON object at
   all. All four `ParseRecommendation` functions now set a new
   `Truncated bool` field on their fallback `Recommendation` using
   this helper, and all four `FormatForTelegram` functions show a more
   specific "⚠️ The AI's response got cut off before it finished —
   showing the partial reply below" message when `Truncated` is true,
   instead of the generic parse-failure notice.

This is deliberately *not* a fix that tries to salvage a partial
object (e.g. auto-closing dangling braces and guessing at missing
fields) — a truncated response is definitionally missing information
the player needs (e.g. `priority_actions` cut off halfway through), so
showing the raw partial text with an honest "cut off" label is more
trustworthy than fabricating a complete-looking but silently
incomplete result.

**Verification:** 5 new tests in `internal/ai/jsonrecovery_test.go`
(unclosed object, unclosed nested object, valid complete object
correctly reported as not truncated, plain prose with no `{` at all
correctly reported as not truncated, object left open via an
unterminated string) plus 2 new regression tests per feature package
(8 total) confirming both the `Truncated` flag itself and the new
Telegram message. 100 tests total (was 87), all passing. Full `go
build ./... && go vet ./... && go test ./...` confirmed clean for the
whole repo (same sandbox-only, reverted-before-commit `go.mod` replace
workaround as Phases E and the §1.8 fix — see those sessions' notes
for why).

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

**ADR-007: Phase B stores an `autopilot_enabled` preference but does
not act on it.** The roadmap implies an eventual autonomous-execution
mode ("automation is explicitly enabled"). Rather than wire a
half-tested "AI upgrades your buildings for you" path into the same
session that built the recommend flow, the data model and player-facing
toggle were built now (so nothing later needs a schema migration to add
them), while actual execution — which needs its own safety design
(rate limits, dry-run/rollback, abuse prevention, cost implications of
running unattended) — is explicitly deferred. See §4/§1 Phase B notes.

**ADR-008: Fleet Commander's combat-history "win rate" is a heuristic,
not authoritative.** The `raids` table has no explicit outcome column;
`fleetcommander.BuildCombatHistory` counts a completed raid as an
"apparent win" if any resources were stolen. This is a reasonable
proxy but can misclassify edge cases (e.g. a costly Pyrrhic raid that
still stole a small amount of loot). Flagged as technical debt in §4 —
replace with an authoritative outcome column if/when the SpaceHunt
combat branch adds one.

**ADR-009: Fleet Commander analyzes only the PvE rogue-nest target in
this version, not arbitrary rival players.** Looking up a real
player's live `workshop_inventory` from a raw chat command would
either (a) require the AI Fleet Commander to expose fresh military
intelligence the game's actual `/scout` mechanic is supposed to gate,
undercutting that gameplay loop, or (b) need its own
scouted-data-only lookup layer. Neither was designed carefully enough
in this session to ship responsibly, so PvP targeting is deferred —
see §4 and the Phase C notes in §1.

**ADR-010: One `openaicompat` implementation serves four providers
(OpenAI, DeepSeek, Qwen, Grok) rather than four near-duplicate HTTP
clients.** Confirmed (not assumed) that all four expose the same Chat
Completions wire shape via each provider's own published API docs.
Trade-off: if one of the four ever diverges from the OpenAI shape in a
provider-specific way, this shared implementation would need either a
per-provider flag (the pattern `SupportsJSONResponseFormat` already
establishes) or a fork into its own package. Not a problem yet — flag
it if a fourth wire-format quirk shows up.

**ADR-011: Gemini and Ollama each get their own provider package
instead of being forced into `openaicompat`.** Confirmed both have
genuinely different wire formats (Gemini: `"model"` role, top-level
`systemInstruction`, `responseMimeType`; Ollama: native `/api/chat`,
object-shaped tool-call arguments instead of a JSON string). Forcing
either into the OpenAI shape would require lossy translation; a
dedicated package per genuinely-different wire format was judged
clearer than one "OpenAI-compatible-ish" package with special cases.

**ADR-012: Provider cost prices are hardcoded from a verified,
timestamped web search, not fetched live.** Same trade-off as ADR-004,
extended to five more providers. Every price in
`internal/ai/cost.go`'s table was individually confirmed via web search
on 2026-07-15, with the least-certain entries (Qwen, Grok) explicitly
flagged as approximate in code comments — prices for these two
providers have moved multiple times within 2026 across independent
sources. Re-verify before trusting this table for a real production
budget more than a few months out.

**ADR-013: Schema-drift audits against the parallel branch are now a
standing practice, not a one-off.** The `ether`/`neuro_cores` bug and
the Fleet Commander gaps found in §1.6 are the same underlying risk:
this AI roadmap's code reads tables and content owned by the parallel
SpaceHunt roadmap, which evolves independently. The audit method in
§1.6 (cross-reference `CREATE TABLE` ∪ `ALTER TABLE ADD COLUMN` against
git history for column-level drift; diff `internal/game/content` for
shape-level drift) is cheap enough to re-run after every SpaceHunt
merge and should be, rather than waiting for another live crash report
to prompt it.

**ADR-014: "Mock is always last" is now enforced in `Registry.Ordered()`
code, not left as a documentation-only intent.** Confirmed real bug
(2026-07-16): the previous implementation only achieved ADR-003's
stated invariant by coincidence of `Config`'s default values. Any
future provider named `"mock"` — or any test double sharing that name
— is now unconditionally sorted last by the registry itself, closing
the gap between documented intent and actual behavior permanently
rather than trusting every future config change to preserve it by
accident.

**ADR-015: JSON recovery from real providers is a shared
`internal/ai` helper (`ExtractJSONObject` +
`SanitizeJSONControlChars`), not duplicated per-feature-package.**
Confirmed real bug (2026-07-17, reported by project owner with a live
screenshot — see §1.8): despite every provider's JSONMode appending an
explicit "respond with a single valid JSON object and nothing else"
instruction (see ADR-005's related caveat about Anthropic specifically
lacking a native `response_format` field), real Qwen/Gemini responses
observed in production sometimes wrap the object in a sentence of
prose, or leave a raw unescaped newline inside a string value — both
of which make Go's strict `encoding/json.Unmarshal` reject an object a
human would read as obviously-intended-to-be-valid JSON. This is
**not** domain logic the way the tech-tree/module/unit type
duplication in governor/fleetcommander/econadvisor/researchplanner is
(see each package's doc comment for why *that* duplication is a
deliberate isolation trade-off) — it's generic text-recovery
infrastructure with zero game knowledge, so it lives once in
`internal/ai` (which every one of those packages already imports for
`ai.Service`/`ai.CompletionRequest`, so this adds no new dependency
edge) and all four `ParseRecommendation` functions call it identically.
This does not replace the ADR-005/§4 tech-debt item asking for a real
JSON-Schema-validating retry loop before Phase F/J — it closes today's
observed failure mode; a model that returns genuinely-wrong-shaped
JSON (right syntax, wrong fields) still needs that future work.

**ADR-016: Truncation detection (`ai.WasTruncated`) is a separate
helper from `ExtractJSONObject`, not a third return value bolted onto
it.** `ExtractJSONObject` is called on every `ParseRecommendation`
attempt, including the common, fully-successful case, so its signature
and hot-path logic are kept minimal and unchanged. `WasTruncated` is
only ever called after `ExtractJSONObject` has already returned
`found=false` — a comparatively rare path — so paying the small cost
of a second, independent brace-scan there (rather than threading extra
state through the first scan) keeps `ExtractJSONObject` itself easier
to reason about and leaves every existing call site untouched. The
trade-off: if a future caller needed truncation info on the *success*
path too, this would need revisiting — not a real need today.

**ADR-017: Battle Analyst (Phase F) covers raids and arena battles
only, not World Bosses — confirmed by re-auditing the schema, not
inherited from an earlier session's claim.** A schema audit done
before writing any Phase F code (grepping every `DELETE
FROM`/`ON CONFLICT ... DO UPDATE` touching the relevant tables in
`internal/engine/tick/engine.go`) found `raids` (`state = 'completed'`
rows) and `arena_battles` are genuinely durable, but
`world_boss_attacks` rows are deleted once survivors return home and
`world_boss_contributions` rows are deleted the moment a boss is
defeated — neither retains cross-engagement history today. Building
Battle Analyst's `Snapshot` around World Boss data that doesn't
durably exist would have meant either silently analyzing only
whatever boss is currently mid-fight (misleading — that's not
"history") or fabricating a shape around data that isn't there. Scope
was narrowed instead, and the gap is documented in §4 as real schema
work for a future session, not hidden or worked around with
placeholder data. A related, smaller decision made the same session:
Battle Analyst's defender-side "apparent win" heuristic (attacker
stole nothing) is a natural inverse of Fleet Commander's existing
attacker-side heuristic (some resources stolen) rather than a new,
unrelated proxy — keeping the two packages' win/loss judgment
philosophically consistent even though they're independently
implemented (per the same package-isolation trade-off as every other
Phase B+ package).

**ADR-018: `coordinates.danger_level` is excluded from Galaxy
Advisor's `Snapshot` — it's stored but mechanically dead, and building
advice around dead data would be misleading.** Grepping every
reference before writing Phase H's `Snapshot` found `danger_level` is
only ever written (randomized 1-5) when a new coordinate is inserted
(`internal/bot/handlers/onboarding.go`, `jobs.go`) and never read by
any tick-engine or handler logic — no combat, resource, or event
calculation consults it. This is the same category of state
`world_state.active_weather` was in before SpaceHunt Phase 7 wired up
`world_events` (stored, plausible-looking, but disconnected from
anything that actually happens in-game). Feeding it to the model would
produce advice that sounds grounded in a real risk signal but isn't —
worse than simply not mentioning it. If a future session makes
`danger_level` mechanically meaningful (e.g. affecting raid outcomes
near a coordinate), Galaxy Advisor should pick it up then, not before.

## 3. Full AI Systems Roadmap (Phases A–J)

| Phase | Name | Status | Depends on |
|---|---|---|---|
| A | AI Foundation | Done | — |
| B | AI Planet Governor | Done (advisory-only; autopilot execution deferred, see §4) | A |
| C | AI Fleet Commander | Done (PvE rogue-nest target only; PvP target lookup deferred, see §4) | A |
| D | AI Economy Advisor | Done | A |
| E | AI Research Planner | Done | A |
| F | AI Battle Analyst | Done (raids + arena only; World Boss history excluded, see ADR-017) | A |
| G | AI Guild Assistant | Done (Leader-only; see §3 "What Phase G built") | A |
| H | AI Dynamic Galaxy | Done (see §3 "What Phase H built") | A |
| I | AI NPC Intelligence | Done (see §3 "What Phase I built") | A, ideally after G |
| J | AI Developer Console | Done — scoped to weekly activity reporting per project owner's explicit brief (see §3 "What Phase J built") | A |

**Progress by subsystem:** Foundation 100%. Planet Governor: recommend
flow 100%, autopilot execution 0% (intentionally deferred). Fleet
Commander: PvE recommend flow 100%, PvP target lookup 0% (deferred).
Economy Advisor: 100%. Research Planner: 100%. Battle Analyst: 100%
(raids + arena; World Boss excluded, see ADR-017). Guild Assistant:
100% (Leader-only). Dynamic Galaxy: 100%. NPC Intelligence: 100%
(tactical composition read only — deliberately not a second
attack/no-attack call). Developer Console: 100% of its scoped brief
(weekly activity reporting) — NOT a general natural-language admin
query tool, content/balance engine, or anything else "developer
console" might otherwise suggest; see ADR-019 and §4 for what remains
genuinely unbuilt if a future session is asked for it. **All ten
phases (A–J) on the original roadmap are now done.**

### What Phase B built

```
internal/game/governor/
├── prompt.go        Pure logic: Snapshot type, SystemPrompt, BuildUserPrompt
│                     (deterministic — sorts modules so cache keys are stable),
│                     ParseRecommendation (JSON with markdown-fence tolerance
│                     and raw-text fallback), FormatForTelegram.
├── prompt_test.go    7 passing unit tests, zero DB/network dependency.
└── governor.go       Governor: BuildSnapshot (reads encampments/resources/
                       modules/workshop_inventory/research_states — mirrors
                       internal/engine/resource's COALESCE defaults so the
                       Governor's view of "not built yet" agrees with the
                       tick engine's), Recommend (calls ai.Service.Complete,
                       stores both turns in ai_memory under scope
                       "planet_governor"), AutopilotSetting/SetAutopilot
                       (preference storage only — see ADR-007).
```

New table: `governor_settings` (encampment_id PK, autopilot_enabled).
New commands: `/governor` (any player — read-only recommendation),
`/governor_autopilot [on|off]` (any player — preference only, inert;
see ADR-007).

### What Phase C built

```
internal/game/fleetcommander/
├── prompt.go         Pure logic: FleetComposition (generic unit-name→count
│                      map, not hardcoded per-unit fields, so new
│                      workshop_inventory columns never require a code
│                      change here), TargetProfile, CombatHistorySummary,
│                      SystemPrompt, deterministic BuildUserPrompt,
│                      ParseRecommendation (same markdown-fence tolerance +
│                      raw-text fallback pattern as governor), FormatForTelegram.
├── prompt_test.go     7 passing unit tests, zero DB/network dependency.
└── commander.go       Commander: BuildOwnFleet (explicit workshop_inventory
                        column list, not SELECT *, so schema changes are a
                        deliberate one-line addition), BuildRogueNestTarget
                        (reuses internal/game/content.RogueNestComposition —
                        the exact same PvE data already shown via the
                        existing static /recon-style report, so the two
                        never disagree), BuildCombatHistory (heuristic
                        win/loss proxy off the raids table — see ADR-008),
                        Recommend (ties it together via ai.Service.Complete,
                        persists both turns to ai_memory under scope
                        "fleet_commander").
```

New command: `/fleet_commander` (any player — read-only recommendation:
attack / retreat / reinforce / scout / wait / split_fleet, with
reasoning). No new DB table — Phase C only reads existing data
(encampments, workshop_inventory, raids).

**Deliberately PvE-only in this version.** The roadmap asks for
analysis of "enemy strength" generally, which could mean a rival
player's base. Resolving a safe, abuse-resistant way to look up an
arbitrary rival's `workshop_inventory` (a real player's live military
strength) from a raw chat command needs its own design pass — should
it require the player to have scouted them first? Should it use the
same stale-snapshot data /scout already produces, to avoid this
becoming a fresher, cheaper recon tool than the game's real recon
mechanic? Shipping that carelessly could quietly undercut the existing
scouting gameplay loop, so it's deferred rather than rushed. See §4.

### What Phase D built

```
internal/game/econadvisor/
├── prompt.go         Pure logic: Snapshot (generic Resources map like
│                      fleetcommander's FleetComposition, plus Modules,
│                      bank balances/debt, own market listings, market-wide
│                      stats per item type), SystemPrompt (requires every
│                      action include a quantitative expected_gain per the
│                      roadmap spec), deterministic BuildUserPrompt,
│                      ParseRecommendation (fence tolerance + fallback),
│                      FormatForTelegram.
├── prompt_test.go     6 passing unit tests, zero DB/network dependency.
└── advisor.go         Advisor: BuildSnapshot (resources table via explicit
                        column list, modules, bank_accounts, own active
                        market_exchange listings, market-wide stats grouped
                        by item_type), Recommend (ties it together via
                        ai.Service.Complete, persists to ai_memory scope
                        "economy_advisor").
```

New command: `/economy_advisor` (any player — read-only recommendation
with quantitative ROI estimates, bottleneck warnings, market timing,
trading advice). No new DB table — reads existing resources, modules,
bank_accounts, market_exchange.

Note: deliberately kept independent of `governor` and `fleetcommander`
packages (no shared import) even though all three read overlapping
data (resources, modules) — see the package doc comment in
`econadvisor/prompt.go` for the reasoning (isolation over reuse, at
this small a duplication cost).

### What Phase E built

```
internal/game/researchplanner/
├── prompt.go         Pure logic: Goal type (raiding/defense/economy/
│                      balanced) with inference from the current tech
│                      spread when the player doesn't pick one, TechNode/
│                      Snapshot types (a small deliberate duplicate of
│                      handlers.researchTree's 7-node shape — see the
│                      package doc comment for why), SystemPrompt,
│                      deterministic BuildUserPrompt, ParseRecommendation
│                      (fence tolerance + fallback), FormatForTelegram.
├── prompt_test.go     17 passing unit tests, zero DB/network dependency
│                      (tech node cost formula, goal inference in both
│                      directions, prompt content, JSON parse/fallback,
│                      Telegram formatting).
└── planner.go         Planner: BuildSnapshot (reads encampments/
                        research_states/resources.neuro_cores — the
                        fetchResearchLevels query mirrors
                        handlers.ResearchHandler's exactly, including the
                        same lazy-row-creation default, so the two never
                        disagree about a new player's starting state),
                        Recommend (calls ai.Service.Complete, persists to
                        ai_memory scope "research_planner").
```

New command: `/research_planner [goal]` (any player — read-only
recommendation with a concrete research order, core costs, and
expected gains; `goal` is optional and one of `raiding`/`defense`/
`economy`/`balanced` — omitted or invalid input falls back to
inference). Inline keyboard: a refresh button plus one button per
goal, so a player can steer future recommendations without typing a
command. No new DB table — reads existing `research_states` and
`resources`. Never spends a Neuro Core or upgrades a tech node itself;
the existing `/research` panel remains the only place that actually
happens.

Mock provider updated with a matching placeholder JSON case for
`ai_research_planner`, verified by a new
`TestMockPlaceholder_ParsesForResearchPlanner` case in
`placeholder_test.go` (mirroring the existing per-feature verification
pattern). Combined with `researchplanner`'s own 17 tests, this phase
adds 18 new tests (47 → 65 total, all passing — see the verification
note below).

**Verification this session:** `go build ./...`, `go vet ./...`, and
`go test ./...` were all run clean, full-repo, in this sandbox — not
just the `internal/ai` subtree — by temporarily adding a local
`replace gopkg.in/telebot.v3 => github.com/tucnak/telebot/v3 v3.3.8`
directive to `go.mod` (this sandbox's network allowlist doesn't
include `proxy.golang.org`, only `github.com`, and the module was
already present in the local module cache under that path), then
reverting `go.mod` before committing. That replace directive is
**not** part of this phase's actual diff — it never touched the
committed `go.mod`. This resolves the §4 tech-debt bullet about an
unconfirmed full build for this session's own changes; the underlying
`proxy.golang.org` network-access gap for *future* sessions'
first-time dependency downloads is unchanged and still worth noting to
whoever runs the real deploy (they should have normal internet access,
so it's expected to be a non-issue there).

### What Phase F built

```
internal/game/battleanalyst/
├── prompt.go         Pure logic: RaidStats/ArenaStats/Snapshot types,
│                      Pattern/Recommendation types, SystemPrompt,
│                      deterministic BuildUserPrompt, ParseRecommendation
│                      (fence tolerance + fallback, Truncated flag baked
│                      in from the start per ADR-016), FormatForTelegram.
├── prompt_test.go     13 passing unit tests, zero DB/network dependency
│                      (prompt determinism/content, empty-history paths,
│                      attacker-vs-defender stolen-value asymmetry, JSON
│                      parse/fallback/truncation, Telegram formatting).
└── analyst.go         Analyst: BuildSnapshot (reads completed raids from
                        both the attacker and defender side, plus
                        arena_battles matched by current username),
                        Recommend (calls ai.Service.Complete, persists to
                        ai_memory scope "battle_analyst").
```

New command: `/battle_analyst` (any player — read-only analysis of
their accumulated raid and arena combat record; no goal/argument, since
this looks backward at everything that already happened rather than
steering toward one forward-looking goal). Inline keyboard: a single
refresh button (no per-goal buttons, unlike Research Planner — see
ADR-017). No new DB table — reads existing `raids` and `arena_battles`.
Never changes any raid, arena battle, or unit itself.

**Scope decided by re-auditing the actual schema, not by trusting an
earlier session's notes.** Before writing any code, this session
re-confirmed which tables are genuinely durable history by grepping
every `DELETE FROM`/`ON CONFLICT ... DO UPDATE` touching `raids`,
`world_boss_attacks`, `world_boss_contributions`, and `arena_battles`
in `internal/engine/tick/engine.go`. Result: `raids` rows with
`state = 'completed'` persist (matching what `fleetcommander` already
relies on); `arena_battles` rows are never deleted; but
`world_boss_attacks` rows are deleted once survivors return home, and
`world_boss_contributions` rows are deleted the moment a boss is
defeated — neither retains any history across completed engagements.
See ADR-017 for why this means Battle Analyst covers raids (both
sides) and arena only, not World Bosses, at least until/unless a
future session adds a durable World Boss history table.

Mock provider updated with a matching placeholder JSON case for
`ai_battle_analyst`, verified by a new
`TestMockPlaceholder_ParsesForBattleAnalyst` case in
`placeholder_test.go`. Combined with `battleanalyst`'s own 13 tests,
this phase adds 14 new tests (100 → 114 total, all passing).

**Verification this session:** `go build ./...`, `go vet ./...`, and
`go test ./...` all run clean, full-repo, using the same sandbox-only,
reverted-before-commit `go.mod` replace workaround as every prior
session since Phase E (see that phase's note for why it's needed and
why it's safe to revert).

### What Phase G built

```
internal/game/guildassistant/
├── prompt.go          Pure logic: Applicant/WarRecord/Snapshot types,
│                       RecruitmentCall/Recommendation types,
│                       SystemPrompt, deterministic BuildUserPrompt,
│                       ParseRecommendation (fence tolerance + fallback,
│                       Truncated flag baked in per ADR-016),
│                       FormatForTelegram.
├── prompt_test.go      13 passing unit tests, zero DB/network
│                       dependency (prompt determinism/content,
│                       empty-state paths, JSON parse/fallback/
│                       truncation, Telegram formatting).
└── assistant.go        Assistant: BuildSnapshot (reads the caller's
                         clan roster/combined level/military power via
                         the same soldiers*10+mechs*150 formula
                         HandleAllianceStatsCallback already uses, plus
                         pending clan_applications enriched with
                         applicant level, plus durable clan_wars
                         history), Recommend (calls
                         ai.Service.Complete, persists to ai_memory
                         scope "guild_assistant").
```

New command: `/guild_assistant` — **Leader-only**, unlike every prior
Phase B-F command. Every other clan-leadership decision in
`internal/bot/handlers/clan.go` (accepting/rejecting applicants,
declaring war, kicking/promoting members) is already gated to
`role = "Leader"`; recruitment and war strategy fall in that same
bucket, so `Assistant.BuildSnapshot` returns a distinct `ErrNotLeader`
(as well as `ErrNoClan` for players not in a clan at all) rather than
silently serving a regular member clan-wide leadership information
that isn't theirs to act on. Inline keyboard: a single refresh button
(no per-goal buttons, matching Battle Analyst's reasoning — this looks
at whatever the clan's current state is, not one steerable goal). No
new DB table — reads existing `clans`, `user_clans`, `clan_applications`,
`clan_wars`, `encampments`, `workshop_inventory`. Never accepts/rejects
an applicant, declares a war, or changes membership itself.

**Scope decided by reading the real "clans" system before writing any
code**, the same discipline Phase F used: `migrations/017_spacehunt_
phase2_guild_extras.sql` confirmed the guild feature (tables `clans`,
`user_clans`, `clan_applications`, `clan_wars`) already exists on
`main` from the parallel SpaceHunt branch, with a hard 15-member cap
and a "Leader"/"Soldier" role model (no separate Officer tier).
Grepping every `DELETE FROM` touching those tables confirmed
`clan_wars` rows are never deleted (durable win/loss history, unlike
Phase F's World Boss tables), while `clan_applications` rows are
deleted on accept/reject (current pending state only, which is exactly
what's needed here — no history claim made about past decisions).

Mock provider updated with a matching placeholder JSON case for
`ai_guild_assistant`, verified by a new
`TestMockPlaceholder_ParsesForGuildAssistant` case. Combined with
`guildassistant`'s own 13 tests, this phase adds 14 new tests (114 →
128 total, all passing).

**Verification this session:** `go build ./...`, `go vet ./...`, and
`go test ./...` all run clean, full-repo. Also rebased both the
still-open `critical-fix-truncation-flag` and `phase-f-battle-analyst`
branches onto `main` first this session, since `main` had moved
forward (SpaceHunt visual-system commits) since they were last pushed
— confirmed both rebase cleanly and still build/test clean before
starting Phase G on top of them.

### What Phase H built

```
internal/game/galaxyadvisor/
├── prompt.go          Pure logic: ContinentStatus/Snapshot types,
│                       Action/Recommendation types, SystemPrompt,
│                       deterministic BuildUserPrompt (including its
│                       own eventEffect mechanical-effect-text mapping
│                       for all 7 event types + nominal), ParseRecommendation
│                       (fence tolerance + fallback, Truncated flag baked
│                       in per ADR-016), FormatForTelegram.
├── prompt_test.go      13 passing unit tests, zero DB/network
│                       dependency (prompt determinism/content, no-news
│                       and unrecognized-event-type paths, JSON parse/
│                       fallback/truncation, Telegram formatting).
└── advisor.go          Advisor: BuildSnapshot (reads the caller's home
                         continent via encampments→coordinates.region,
                         every continent's active event via the shared
                         internal/engine/world.ActiveEventsByContinent/
                         Continents helpers the tick engine and world-
                         feed panel already use, and the same 5 most
                         recent world_news headlines
                         HandleWorldFeed's own panel shows), Recommend
                         (calls ai.Service.Complete, persists to
                         ai_memory scope "galaxy_advisor").
```

New command: `/galaxy_advisor` (any player — read-only briefing on
their home continent's current world event plus the wider galaxy's
environmental state across all four continents; no goal/argument,
since — like Battle Analyst and Guild Assistant — this reasons about
whatever the current state is, not one steerable goal). Inline
keyboard: a single refresh button. No new DB table — reads existing
`world_events`/`world_news`/`coordinates`/`encampments` through the
already-shared `internal/engine/world` helpers rather than
reimplementing that lookup. Never changes any world event, marches any
fleet, or queues any construction itself.

**Genuinely cross-cutting, matching how §3 already flagged this
phase.** Every prior package (B-G) reasons about one player's base,
fleet, one raid target, economy, research, combat record, or clan.
This one is the first to reason about *shared* state — the same
per-continent world events every player on that continent sees — and
explicitly looks across all four continents together (`galaxy_outlook`
in the JSON shape), not just the player's own, so a player can also
reason about whether another continent is a better target right now.

**Schema audit done before writing any code**, the same discipline as
Phases F and G: read `internal/engine/world/events.go` and
`weather.go` (added by the parallel SpaceHunt branch's Phase 7 item 12,
`migrations/025`) to confirm world events are already resolved
independently per continent with existing exported helpers this phase
could reuse directly, rather than re-querying `world_events` by hand.
Also checked whether `coordinates.danger_level` was worth including as
a signal — it isn't: grepping every reference confirmed it's written
(randomized 1-5) on new-coordinate insert but never read by any game
mechanic, the same "stored but mechanically dead" state
`world_state.active_weather` was in before Phase 7 wired up
`world_events`. Excluded from `Snapshot` as a result; see ADR-018.

Mock provider updated with a matching placeholder JSON case for
`ai_dynamic_galaxy`, verified by a new
`TestMockPlaceholder_ParsesForGalaxyAdvisor` case. Combined with
`galaxyadvisor`'s own 13 tests, this phase adds 14 new tests. A fresh
full-suite count this session verified 141 tests total, all passing
(see Change Log for the exact before/after — a minor discrepancy was
found in Phase G's own recorded count while verifying this, tracked in
§4 rather than silently corrected).

**Verification this session:** `go build ./...`, `go vet ./...`, and
`go test ./...` all run clean, full-repo. Rebased all three still-open
branches (`critical-fix-truncation-flag`, `phase-f-battle-analyst`,
`phase-g-guild-assistant`) onto `main` first, since `main` had again
moved forward (SpaceHunt Phase 7 regional world events landed) since
they were last pushed — all three rebase cleanly and still build/test
clean before starting Phase H on top of them.

### What Phase I built

```
internal/game/npcintel/
├── prompt.go          Pure logic: NestProfile/FleetProfile/Snapshot
│                       types, UnitVerdict/Recommendation types,
│                       SystemPrompt (encodes the combat engine's real
│                       hard-counter rules explicitly), deterministic
│                       BuildUserPrompt, ParseRecommendation (fence
│                       tolerance + fallback, Truncated flag baked in
│                       per ADR-016), FormatForTelegram.
├── prompt_test.go      14 passing unit tests, zero DB/network
│                       dependency (prompt determinism/content,
│                       Warlord-present and no-fleet paths, JSON
│                       parse/fallback/truncation, Telegram formatting).
└── intel.go            Intel: BuildSnapshot (scales the Rogue Drone
                         Nest via the same content.RogueNestComposition
                         Fleet Commander and /recon_ai already use, and
                         reads the player's own mobile fleet from
                         workshop_inventory), Recommend (calls
                         ai.Service.Complete, persists to ai_memory
                         scope "npc_intel").
```

New command: `/npc_intel` (any player — a composition-specific
tactical read of the Rogue Drone Nest against their own current mobile
fleet). Inline keyboard: a single refresh button. No new DB tables or
game content — reuses `content.RogueNestComposition`/`ThreatTier`
directly and reads existing `encampments`/`workshop_inventory`. Never
launches a raid or moves any unit.

**What this phase actually is, and isn't.** The Vagabond has exactly
one NPC/hostile-AI entity — the Rogue Drone Nest
(`internal/game/content.RogueNestComposition`) — not a roster of
distinct NPCs, so "AI NPC Intelligence" could easily have collapsed
into a second copy of Fleet Commander's existing attack/no-attack call.
It deliberately doesn't: Fleet Commander (Phase C) already answers
"should I attack this Nest at all"; this phase answers a different,
previously-unaddressed question — "given I might attack, which of my
own units help or hurt against *this specific* Nest's composition." That
distinction is only meaningful because the combat engine has real
hard-counter mechanics (Destroyer/Wraith vs. the Nest's drones+jets,
Bomber vs. a turreted Defense Grid, the Nest's own Guardians countering
Bombers right back, and three of its five turret types scaling against
specific attacker unit types) that the static `/recon_ai` report
already shows as raw numbers but never interprets. Those mechanics are
listed explicitly in `SystemPrompt` — read directly from
`internal/engine/tick/engine.go`'s combat resolution comments before
writing a line of this package's code — rather than left for the model
to guess at or invent.

Mock provider updated with a matching placeholder JSON case for
`ai_npc_intelligence`, verified by a new
`TestMockPlaceholder_ParsesForNPCIntel` case. Combined with
`npcintel`'s own 14 tests, this phase adds 14 new tests (141 → 155
total, all passing — recounted fresh this session, continuing the
practice started in Phase H's write-up rather than trusting a prior
session's arithmetic).

**Process note:** this session initially wrote all of Phase I's code
directly on `main` by mistake (forgot to check out a feature branch
first) before any of it was committed. Caught before anything was
committed or pushed — the new files were moved onto a proper branch
stacked on `phase-h-dynamic-galaxy` before continuing, and `main` was
left untouched. Also rebased all four still-open branches
(`critical-fix-truncation-flag` through `phase-h-dynamic-galaxy`) onto
`main` again this session, since `main` had moved forward again
(SpaceHunt Phase 7 items 10/11: World Exploration + Clan Diplomacy)
since they were last pushed — all four rebase cleanly.

**Verification this session:** `go build ./...`, `go vet ./...`, and
`go test ./...` all run clean, full-repo.

### What Phase J built

```
internal/game/devconsole/
├── prompt.go          Pure logic: NewPlayer/TopPlayer/Snapshot types,
│                       Recommendation type, SystemPrompt, deterministic
│                       BuildUserPrompt (capped-list-notice, empty-state,
│                       and no-outpost-yet paths), ParseRecommendation
│                       (fence tolerance + fallback, Truncated flag
│                       baked in per ADR-016), FormatForTelegram.
├── prompt_test.go      13 passing unit tests, zero DB/network
│                       dependency.
└── console.go           Console: BuildSnapshot (new-player signups via
                          users.registered_at, top players via the
                          same scoring.ScoreExpr the player-facing
                          Global Ranking panel already uses, active-user
                          count via users.last_active, recent
                          world_news — all windowed to the last N
                          days), Recommend (calls ai.Service.Complete,
                          persists to ai_memory scope "dev_console").
```

New command: `/weekly_report [days]` — **admin-only**, using the exact
same `AdminIDs`/`IsAdmin` gate every other admin-only action in
`internal/bot/handlers/admin.go` already uses (reused via
`admin.AdminIDs`, not re-parsed). Defaults to a 7-day window; an
optional numeric argument overrides it. Inline keyboard: a single
refresh button that preserves the original window. No new DB tables —
reads existing `users`/`encampments`/`coordinates`/`world_news`. Never
changes any player, setting, or game data.

**ADR-019: scope was set explicitly by the project owner, not
guessed at — and deliberately narrower than "AI Developer Console"
could otherwise imply.** Every prior phase (F-I) started from an
already-built player-facing system to ground its scope in (raids,
clans, world events, the Rogue Nest). Phase J's name had no such
anchor, and the session's own prior write-up flagged this explicitly
rather than guessing. Asked directly, the project owner specified a
concrete brief: an admin-only AI summary of recent game activity — new
signups (name, username, join time, in-game home region), top players,
and similar, over up to a week. This package implements exactly that
brief and nothing broader. It is explicitly NOT a general
natural-language admin query tool, an AI-assisted content/balance
suggestion engine, or any other capability "developer console" might
otherwise suggest — those remain unbuilt, and would need their own
explicit scoping conversation if ever requested, per the same
discipline used here.

**Honest data-availability boundary, stated to the project owner
before building:** the game collects no real-world IP or geolocation
data for any user — there is no such column on `users`. "Where a new
player is from" is reported as their in-game home continent (via
`encampments` → `coordinates.region`, the same continent scheme every
world-aware phase since H already uses), not a real-world location.
This was surfaced as a constraint rather than silently substituted —
building a report that implied real-world geolocation the game doesn't
actually have would have been misleading in exactly the way ADR-017's
and ADR-018's "don't build on data that isn't really there" precedent
warns against.

Mock provider updated with a matching placeholder JSON case for
`ai_developer_console`, verified by a new
`TestMockPlaceholder_ParsesForDevConsole` case. Combined with
`devconsole`'s own 13 tests, this phase adds 14 new tests (155 → 169
total, all passing).

**Verification this session:** `go build ./...`, `go vet ./...`, and
`go test ./...` all run clean, full-repo. Branched directly off `main`
this time (all of Phases F–I had already been reviewed and merged via
PR by the project owner since the last session, confirmed via `git
log` before starting — no rebasing of stacked feature branches was
needed this session).

### Phase J expansion: natural-language admin queries + balance-suggestion tooling (following session)

The project owner explicitly asked for the two capabilities ADR-019
had deliberately left out of the first Phase J pass. Built as an
addition to the same `devconsole` package (three new files:
`queries.go`, `nlquery.go`, `balance.go`), not a new phase — same
Feature (`ai_developer_console`), same admin-only gate.

**ADR-020: the model never writes or executes SQL — it only ever
picks a name from a fixed whitelist plus two bounded integers.** This
is the load-bearing safety decision for natural-language admin
queries. `queries.go` defines `queryIntents`, a fixed map of 9
read-only intent names (`new_players`, `top_players`, `active_users`,
`totals`, `economy_snapshot`, `combat_stats`, `clan_stats`,
`world_state`, `recent_news`) to already-written, parameterized
`SELECT` queries. The flow (`nlquery.go`'s `Ask`) is two separate AI
calls, not one:
1. **Classify** — given the admin's free-text question and the
   whitelist description, the model returns `{"intent": "...", "days":
   N, "limit": N}`.
2. **Execute** — `IsKnownIntent` rejects anything not in the literal
   whitelist outright (verified by
   `TestParseClassification_UnknownIntentStillRejectedByIsKnownIntent`,
   which feeds a SQL-injection-shaped intent value through the real
   parser and confirms it's still refused); `days`/`limit` are clamped
   to `[1,90]`/`[1,25]` (`ClampDays`/`ClampLimit`) regardless of what
   the model returned. `RunIntent` then runs exactly one predetermined
   query — the model's output is never concatenated into SQL at any
   point.
3. **Answer** — a second call receives the admin's original question
   plus the real query-result text (never anything the model wrote)
   and produces the final grounded natural-language answer.

Two calls cost more than every other phase's single call, but there's
no safe single-call design here: compressing to one call would mean
either letting the model's raw output drive execution (unsafe) or
answering before the real numbers exist (ungrounded). New command:
`/admin_ask <question>` (admin-only).

**Balance-suggestion tooling (`balance.go`)** computes real
usage/outcome statistics per mobilizable unit type
(`soldiers_mobilized` through `wraiths_mobilized` on `raid_forces`)
across completed raids: usage rate (% of raids that unit type appears
in) and apparent win rate when used (reusing the same stolen-resources
heuristic as `battleanalyst`/`fleetcommander`). **This is
correlational, not causal, and the system prompt says so explicitly**
— a unit's high apparent win rate could mean it's strong, or could
simply mean better players favor it; `BalanceSystemPrompt` forbids the
model from stating a unit is "overpowered"/"underpowered" as fact, and
`TestBalanceSystemPrompt_WarnsAgainstCausalClaims` checks that
constraint is actually present in the prompt text, not just intended.
New command: `/balance_report [days]` (admin-only).

**Verification:** 34 new tests — 22 in `nlquery_test.go` (classifier/
answer/clamp/whitelist coverage, including the adversarial
SQL-injection-shaped-intent test above) plus 12 in `balance_test.go`.
169 → 203 total, all passing, confirmed by a fresh full-suite run this
session. Full `go build ./... && go vet ./... && go test ./...` clean
for the whole repo. Branched as `phase-j2-admin-console-tools`,
stacked on the already-pushed `phase-j-dev-console` branch (not yet
merged at the start of this session).

### Roadmap status: all ten phases (A–J) complete

Every phase on the original AI Systems Roadmap (§3's table) is now
Done. This does not mean every AI idea for the game is exhausted — §4
lists real, intentionally-deferred gaps (Governor autopilot execution,
Fleet Commander PvP target lookup, World Boss history, and Phase J's
deliberately narrow scope among them) and §7 lists unscoped future
ideas — but there is no more "next phase" to default to. Future AI work
on this game should start from an explicit brief from the project
owner (what should it do, for whom, grounded in which real system/data)
rather than this document picking the next roadmap letter, the same
way Phase J itself was scoped.

---

## 4. Known Issues / Technical Debt

- **World Boss engagement history is not durable in the current
  schema** (see ADR-017 and the "What Phase F built" note in §3): `world_boss_attacks` rows are
  deleted once survivors return home, and `world_boss_contributions`
  rows are deleted the moment a boss is defeated. Battle Analyst
  (Phase F) therefore only covers raids and arena battles. If a future
  session wants World Boss pattern analysis, it needs a new durable
  history table (e.g. an append-only `world_boss_attack_log`) added
  first — this is real schema work, not something Phase F itself could
  work around.
- **Arena/raid win-loss stats rely on heuristics, not authoritative
  outcome columns.** Battle Analyst's defender-side "apparent win" (no
  resources stolen) and Fleet Commander's existing attacker-side
  "apparent win" (some resources stolen) are both proxies for a real
  outcome column `raids` doesn't have — see ADR-009's original note on
  this for Fleet Commander, now shared by Battle Analyst too. Also,
  Battle Analyst's arena stats match by the player's *current*
  Telegram username (since `arena_battles` stores usernames as plain
  strings, not a `user_id` foreign key) — a player who changed their
  username after a past arena battle will have that battle silently
  excluded. Both are acceptable proxies today; replace if/when the
  SpaceHunt combat branch adds authoritative columns.
- **`coordinates.danger_level` is stored but mechanically dead** (see
  ADR-018): randomized on insert, never read by any game logic. Galaxy
  Advisor (Phase H) deliberately excludes it rather than presenting it
  as a real signal. If it's ever wired up mechanically, revisit.
- **Minor test-count discrepancy found while verifying Phase H.** Phase
  G's own write-up claimed 128 tests total after that phase; a fresh
  count this session (before adding any Phase H tests) found 127. The
  likely cause: `guildassistant`'s package has 12 tests, not the 13
  originally claimed in that session's notes — an off-by-one in
  counting, not a missing test or a functional gap (every behavior
  that write-up described is still covered; re-reading
  `prompt_test.go` confirms 12 distinct `func Test...` cases). Left as
  a recorded discrepancy rather than silently edited into Phase G's
  already-committed history; Phase H's own numbers in this document
  are based on a fresh, verified count (127 → 141) rather than trusting
  the prior session's arithmetic.
- **Phase J (AI Developer Console) is deliberately narrower than its
  name implies** (see ADR-019): scoped to exactly one admin-only
  capability (weekly-style game activity reporting) per the project
  owner's explicit brief. A general natural-language admin query tool,
  AI-assisted content/balance suggestions, or other "developer
  console" capabilities are NOT built and would need their own
  explicit scoping conversation if ever wanted — don't assume Phase J
  being "Done" means the full space implied by its name is covered.
- **`/weekly_report`'s "new player location" is in-game continent, not
  real-world geolocation** (see Phase J's ADR-019 write-up in §3): the
  game collects no real IP/location data for any user. Don't extend
  this report to imply real-world geolocation without adding real data
  collection for it first — and consider the privacy implications of
  doing so before building it.
- **`/admin_ask`'s query whitelist is intentionally small (9 intents)**
  (see ADR-020 in §3). A question that doesn't map to any of them gets
  a "couldn't match that" reply, not a best-effort guess — by design,
  since guessing would mean either inventing a query or letting the
  model's output drive execution more loosely. Adding a new intent
  means adding both a `queryIntents` entry and a `RunIntent` case
  together — never let the classification step alone "unlock" a query
  that doesn't already exist in `RunIntent`.
- **`/balance_report`'s unit stats are correlational, not causal** (see
  ADR-020 / `balance.go`'s doc comment in §3): a unit's apparent win
  rate when used doesn't isolate the unit's own strength from who
  tends to use it. Treat every output as "worth a human balance
  designer looking into," never as a verdict — the system prompt
  enforces this on the model's side, but a human reader should apply
  the same skepticism.
- **Static cost table will go stale.** `internal/ai/cost.go`'s
  `pricePerMillionTokens` map needs periodic manual updates as
  Anthropic (and future providers) change pricing. Consider moving
  this to an env-var-configurable table if it becomes painful.
- **No tool-execution loop.** See ADR-006. Any phase that wants the
  model to actually call a tool and see the result needs to build that
  loop; `Service.Complete` only returns the requested tool calls, it
  does not execute them.
- **No JSON-Schema validation/retry for structured output.** See
  ADR-005. ADR-015 closed the specific failure mode observed live
  (prose-wrapped/malformed-whitespace JSON getting rejected outright)
  with a text-recovery layer, but a model returning genuinely
  wrong-shaped JSON (valid syntax, missing/renamed fields) still isn't
  caught — needed before Phase F/J can be trusted for anything
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
- **Fleet Commander's win/loss history is a heuristic** (any stolen
  resources = "apparent win"). See ADR-008.
- **Fleet Commander has no PvP target support.** See ADR-009. A player
  can currently only get a recommendation against the scaled PvE rogue
  nest, not a specific rival.
- **Grok/Qwen cost estimates are the least certain entries in the cost
  table.** See ADR-012. xAI in particular has renamed and repriced its
  model lineup multiple times within 2026 across independent sources —
  confirm `GROK_MODEL` still resolves to a real model ID at
  https://docs.x.ai/docs/models before relying on it in production.
- **`openaicompat`, `gemini`, and `ollama` all share the same
  `RoleTool`-folded-into-`user` simplification as the Anthropic
  provider** (see the existing tech-debt bullet above and ADR-006) —
  fix all four together when a real tool-execution loop is built,
  not one at a time.
- **Ollama's realistic performance on Render's standard (CPU-only)
  plans is unconfirmed.** The provider itself is complete and tested,
  but nobody has verified it against an actual Render deployment. A
  capable open-weight model (7B+ parameters) run on CPU is typically
  much slower per reply than a hosted API, and standard Render web
  service plans may not have enough RAM for a useful model size. This
  is a known general constraint of CPU inference, not a guess specific
  to this project — but the project owner's specific Render tier
  hasn't been confirmed, so no claim is made here about whether Ollama
  is currently practical on their infrastructure.

## 5. Risks / Blockers

- **No API key is configured for any of the six real providers
  (Anthropic, OpenAI, DeepSeek, Qwen, Grok, Gemini) as of this
  session, and Ollama has no base URL set.** Every provider's code
  path was designed and tested (via mocked HTTP servers, not live
  calls — see §1.5) to work correctly once a key is added, but zero
  have been exercised against a real, live API in this project. The
  project owner has been asked directly which provider(s) they have
  keys for; this is not yet confirmed.
- **Whether Ollama is practical on the project owner's actual Render
  plan is unconfirmed** — see the tech-debt bullet in §4. Do not
  assume Ollama is viable there without confirming the plan's RAM/CPU
  and testing actual response latency for a real model.
- **Push status, corrected again:** as of this write-up, the
  `ai-foundation-phase-a` branch has been rebased onto the true current
  `origin/main` (confirmed via `git fetch`, which showed `main` had
  advanced to include SpaceHunt Phase 6/7 since our Phase D merge —
  see §1.6). This rebase was clean, zero conflicts. **The §1.5 and §1.6
  work (bugfix, mock JSON fix, keyboards, 4 new providers, Fleet
  Commander schema-drift fixes) is still not yet pushed to GitHub** as
  of this write-up — it exists only as local commits in this session's
  sandbox / a new git bundle handed to the project owner. If a future
  session finds this file already reflecting these changes on GitHub,
  the push succeeded; otherwise it's still pending.
- **On credentials:** a GitHub Personal Access Token was offered
  directly in chat more than once during this project. Per this
  assistant's fixed operating rules, credentials pasted into a
  conversation are never used to authenticate, regardless of how the
  account is described or how much authority is attached to the token.
  This is not a preference that changes with re-asking — future
  sessions should expect the same response and should not spend
  turns re-litigating it. The working handoff process (git bundle +
  the project owner pushing themselves, from Termux or otherwise) is
  proven to work and should simply be repeated.

## 6. Change Log

- **This session:** Phase A (AI Foundation) implemented in full:
  provider abstraction, Anthropic + mock providers, config/feature
  flags, in-memory cache, Postgres-backed cost tracking + budgets,
  Postgres-backed permission system, Postgres-backed memory store,
  unifying `Service`, 3 new bot commands, DB schema, 6 passing unit
  tests. This file created.
- **This session (continued):** Phase B (AI Planet Governor)
  implemented: pure prompt-building/parsing logic (`prompt.go`, 7
  passing unit tests), DB-backed orchestration (`governor.go`), 2 new
  bot commands (`/governor`, `/governor_autopilot`), 1 new table
  (`governor_settings`). Autopilot execution explicitly deferred —
  see ADR-007. Recommended next task updated to Phase C.
- **This session (continued):** Phase C (AI Fleet Commander)
  implemented: pure prompt/parsing logic (`prompt.go`, 7 passing unit
  tests), DB-backed orchestration (`commander.go`) reusing
  `internal/game/content`'s existing rogue-nest PvE data, 1 new bot
  command (`/fleet_commander`), no new tables. PvP target lookup and
  authoritative win/loss tracking explicitly deferred — see ADR-008,
  ADR-009. Recommended next task updated to Phase D.
- **This session (continued):** Phase D (AI Economy Advisor)
  implemented: pure prompt/parsing logic (`prompt.go`, 6 passing unit
  tests) requiring quantitative expected-gain estimates per the
  roadmap spec, DB-backed orchestration (`advisor.go`) reading
  resources/modules/bank_accounts/market_exchange, 1 new bot command
  (`/economy_advisor`), no new tables. Recommended next task updated
  to Phase E.
- **Following session:** production hardening + multi-provider
  ecosystem (see §1.5 for full detail). Fixed a confirmed live crash
  (`ether`/`neuro_cores` columns missing from `resources` on the
  production database). Fixed the mock provider's raw-text output
  quality by making it JSON-aware per feature (verified via
  `placeholder_test.go`, not assumed). Added inline keyboards
  (refresh + autopilot toggle) to `/governor`, `/fleet_commander`,
  `/economy_advisor`. Built and end-to-end tested (via local
  `httptest` servers, not assumed) four new providers — `openaicompat`
  (OpenAI/DeepSeek/Qwen/Grok), `gemini`, `ollama` — catching and fixing
  two real bugs in the process (a silently-dropped JSON-mode
  instruction in `openaicompat`, and a default Gemini model that
  Google had already shut down). Cost table rebuilt with prices
  verified by web search on 2026-07-15. Added ADR-010 through ADR-012.
  17 new tests this session (26 → 43 total).
- **Following session (same day):** deep schema-drift audit prompted
  by the project owner (see §1.6). Rebased onto the true current
  `origin/main` (zero conflicts), discovering the parallel SpaceHunt
  branch had merged Phase 6 and an unplanned Phase 7 since our Phase D
  merge. Confirmed via git-history cross-reference that the
  `ether`/`neuro_cores` bug was the only crash-class schema-drift issue
  anywhere in the tables this roadmap's code reads. Found and fixed two
  real (non-crash) analysis-quality gaps in Fleet Commander: a stale
  10-column-short unit list, and unused richer Phase 7 rogue-nest
  defense data. Added ADR-013 (schema-drift audits as standing
  practice). 1 new test (43 → 44 total).
- **Following session (same day):** critical fix — `Registry.Ordered()`
  was structurally routing every request to the mock provider
  regardless of real, available, correctly-configured providers (see
  §1.7). Root cause confirmed via reproduction, not guessed: mock's
  default position (first) plus its guaranteed non-error `Complete`
  meant real providers were unreachable. Fixed by making "mock always
  last" a code-enforced invariant (`registry.go`), with 3 new
  regression tests reproducing the exact failure condition. Also
  fixed confirmed model-generation drift found the same session:
  Gemini default updated 2.5→3.5-flash (Google's 3.5 generation
  reached GA 2026-05-19; 3.5 Pro — what the project owner asked about
  — confirmed not yet GA), OpenAI default updated 4o-mini→5.6-luna
  (GPT-5.6 reached GA 2026-07-09). Added ADR-014. 3 new tests (44 → 47
  total).
- **Following session:** Phase E (AI Research Planner) implemented:
  pure prompt/parsing logic (`prompt.go`, 17 passing unit tests
  covering cost formula, goal inference in both directions, prompt
  content, JSON parse/fallback, and Telegram formatting), DB-backed
  orchestration (`planner.go`) reusing the exact tech-tree read
  pattern from `internal/bot/handlers/research.go` (read first, per
  §3's own instruction, before writing any Phase E code), 1 new bot
  command (`/research_planner [goal]`) with a refresh button plus one
  inline button per goal (raiding/defense/economy/balanced), no new
  tables. Mock provider given a matching placeholder JSON case,
  verified by 1 new test in `placeholder_test.go`. 18 new tests total
  (47 → 65, all passing). Full `go build ./... && go vet ./... && go
  test ./...` confirmed clean for the whole repo this session (see the
  verification note under "What Phase E built" for how the sandbox's
  `proxy.golang.org` gap was worked around locally without touching
  the committed `go.mod`). Recommended next task updated to Phase F,
  with an explicit note to check the parallel SpaceHunt branch for
  real battle data before designing it.
- **Following session:** Critical fix (§1.8, ADR-015): confirmed real
  production bug where Governor/Fleet Commander/Economy Advisor (and
  latently, Research Planner) fell back to displaying raw unformatted
  JSON whenever a real provider's response had prose wrapped around
  the JSON object or a raw newline inside a string value — reported by
  the project owner via a live screenshot. Added shared
  `internal/ai/jsonrecovery.go` (`ExtractJSONObject` +
  `SanitizeJSONControlChars`) and wired it into all 4 packages'
  `ParseRecommendation`; also improved all 4 packages' raw-text
  fallback rendering (clear notice + code block) for the genuine
  last-resort case. 22 new tests (14 in `internal/ai`, 2 regression
  tests each in governor/fleetcommander/econadvisor/researchplanner).
  65 → 87 total, all passing.
- **Following session:** Critical fix (§1.9, ADR-016): distinguished
  responses truncated mid-object (hit `MaxTokens`) from genuinely
  unparseable ones, which previously showed the player the same
  generic "couldn't parse" message. Added `ai.WasTruncated`; wired a
  new `Truncated bool` field through all four packages'
  `ParseRecommendation`/`FormatForTelegram`; raised `MaxTokens`
  1024→2048 in all four packages to reduce how often truncation
  happens at all. 13 new tests (5 in `internal/ai`, 2 regression tests
  each in governor/fleetcommander/econadvisor/researchplanner). 87 →
  100 total, all passing. Recommended next task remains Phase F.
- **Following session:** Phase F (AI Battle Analyst) implemented: pure
  prompt/parsing logic (`prompt.go`, 13 passing unit tests, `Truncated`
  handling baked in from the start per ADR-016), DB-backed
  orchestration (`analyst.go`) reading completed raids from both the
  attacker and defender side plus `arena_battles` matched by current
  username, 1 new bot command (`/battle_analyst`, single refresh
  button, no goal selection — this phase looks backward at the whole
  combat record rather than steering toward one forward-looking goal),
  no new tables. A schema audit done before writing any code (per §3's
  own instruction to check real data first) found World Boss
  engagement history is not durable in the current schema — see
  ADR-017 for why this narrowed Battle Analyst's scope to raids and
  arena only, and §4 for the follow-up work that would fix that. Mock
  provider given a matching placeholder JSON case, verified by 1 new
  test in `placeholder_test.go`. 14 new tests total (100 → 114, all
  passing). Full `go build ./... && go vet ./... && go test ./...`
  confirmed clean for the whole repo this session (same sandbox-only
  go.mod workaround as every session since Phase E). Recommended next
  task updated to Phase G.
- **Following session:** Phase G (AI Guild Assistant) implemented:
  pure prompt/parsing logic (`prompt.go`, 13 passing unit tests,
  `Truncated` handling baked in per ADR-016), DB-backed orchestration
  (`assistant.go`) reading the caller's clan roster/military
  power/pending applicants (enriched with applicant level) plus
  durable `clan_wars` history, 1 new bot command (`/guild_assistant`,
  Leader-only — unlike every prior Phase B-F command, since
  recruitment/war strategy are already Leader-gated decisions
  elsewhere in `clan.go`). Confirmed the guild system
  (`clans`/`user_clans`/`clan_applications`/`clan_wars`) already exists
  on `main` from the parallel SpaceHunt branch by reading
  `migrations/017_spacehunt_phase2_guild_extras.sql` and `clan.go`
  before writing any Phase G code, then confirmed via `DELETE FROM`
  grepping that `clan_wars` (unlike Phase F's World Boss tables) is
  genuinely durable history. Also rebased the still-open
  `critical-fix-truncation-flag` and `phase-f-battle-analyst` branches
  onto `main` (which had moved forward with SpaceHunt visual-system
  commits) before starting Phase G on top of them — both rebased
  cleanly. Mock provider given a matching placeholder JSON case,
  verified by 1 new test. 14 new tests total (114 → 128, all passing).
  Full `go build ./... && go vet ./... && go test ./...` confirmed
  clean for the whole repo this session. Recommended next task updated
  to Phase H.
- **Following session:** Phase H (AI Dynamic Galaxy) implemented: pure
  prompt/parsing logic (`prompt.go`, 13 passing unit tests, `Truncated`
  handling baked in per ADR-016, plus its own event→mechanical-effect
  text mapping for all 7 event types), DB-backed orchestration
  (`advisor.go`) reusing the existing shared
  `internal/engine/world.ActiveEventsByContinent`/`Continents` helpers
  and the same 5-headline `world_news` query
  `HandleWorldFeed`'s panel already uses, 1 new bot command
  (`/galaxy_advisor`, single refresh button). This is the first phase
  to reason about shared per-continent state rather than one player's
  own base/fleet/economy/research/combat/clan — matches how §3 already
  flagged it as more cross-cutting. Read
  `internal/engine/world/events.go` and `weather.go` (added by the
  parallel SpaceHunt branch's Phase 7 item 12) before writing any code,
  confirming reusable exported helpers already existed rather than
  re-querying `world_events` by hand. Checked `coordinates.danger_level`
  as a candidate signal and found it mechanically dead (written, never
  read) — excluded per ADR-018. While verifying test counts, found a
  minor discrepancy in Phase G's own recorded total (claimed 128;
  fresh count found 127 before this session's additions — see §4);
  Phase H's own numbers use the freshly-verified count. Mock provider
  given a matching placeholder JSON case, verified by 1 new test. 14
  new tests total (127 → 141, all passing). Full `go build ./... && go
  vet ./... && go test ./...` confirmed clean for the whole repo. Also
  rebased all three still-open branches
  (`critical-fix-truncation-flag`, `phase-f-battle-analyst`,
  `phase-g-guild-assistant`) onto `main` again this session (SpaceHunt
  Phase 7 regional world events had landed) before starting Phase H on
  top of them — all three rebase cleanly. Recommended next task updated
  to Phase I.
- **Following session:** Phase I (AI NPC Intelligence) implemented:
  pure prompt/parsing logic (`prompt.go`, 14 passing unit tests,
  `Truncated` handling baked in per ADR-016), DB-backed orchestration
  (`intel.go`) scaling the Rogue Drone Nest via the existing
  `content.RogueNestComposition`/`ThreatTier` helpers Fleet Commander
  and `/recon_ai` already use and reading the player's own mobile
  fleet from `workshop_inventory`, 1 new bot command (`/npc_intel`,
  single refresh button). Deliberately scoped to NOT duplicate Fleet
  Commander's existing attack/no-attack call: this phase instead reads
  the combat engine's real hard-counter mechanics (Destroyer/Wraith
  vs. drones+jets, Bomber vs. turreted Defense Grids countered by
  Guardians, per-turret-type scaling against specific attacker unit
  types) straight from `internal/engine/tick/engine.go`'s combat
  resolution comments and gives a composition-specific tactical read
  the static `/recon_ai` report never provided. Mock provider given a
  matching placeholder JSON case, verified by 1 new test. 14 new tests
  total (141 → 155, all passing). Full `go build ./... && go vet
  ./... && go test ./...` confirmed clean for the whole repo.
  Mid-session process note: all of this phase's code was initially
  written directly on `main` by mistake (forgot to check out a feature
  branch); caught before anything was committed, moved onto a proper
  branch stacked on `phase-h-dynamic-galaxy`, `main` left untouched.
  Also rebased all four still-open branches onto `main` again
  (SpaceHunt Phase 7 items 10/11: World Exploration + Clan Diplomacy
  had landed) before starting Phase I on top of them — all four
  rebase cleanly. Recommended next task updated to Phase J, flagged as
  needing explicit scope discussion with the project owner before
  building (vaguest phase name on the roadmap, least grounded in an
  existing player-facing system).
- **Following session:** Phase J (AI Developer Console) implemented,
  after asking the project owner directly for scope rather than
  guessing. Brief given: an admin-only AI summary of recent game
  activity — new signups (name, username, join time, in-game home
  region), top players, and similar, over up to a week. Implemented
  exactly that: pure prompt/parsing logic (`prompt.go`, 13 passing
  unit tests, `Truncated` handling baked in per ADR-016), DB-backed
  orchestration (`console.go`) reading `users.registered_at`/
  `last_active`, the same `scoring.ScoreExpr` the player-facing Global
  Ranking panel already uses for top players, and `world_news`, all
  windowed to the last N days (default 7). New admin-only command
  `/weekly_report [days]`, gated with the exact same `AdminIDs`/
  `IsAdmin` pattern `admin.go` already uses. Documented as ADR-019 that
  this phase is deliberately narrower than "AI Developer Console"
  could imply — a general natural-language admin query tool or
  content/balance suggestion engine were NOT built and weren't asked
  for. Also surfaced explicitly (to the project owner, and now in this
  document) that the game collects no real IP/geolocation data — "new
  player location" here means in-game home continent, not real-world
  location. Mock provider given a matching placeholder JSON case,
  verified by 1 new test. 14 new tests total (155 → 169, all passing).
  Full `go build ./... && go vet ./... && go test ./...` confirmed
  clean for the whole repo. Branched directly off `main` (Phases F–I
  had already been reviewed and merged via PR by the project owner
  since the last session). **With Phase J done, all ten phases (A–J)
  on the original AI Systems Roadmap are complete** — see §3's new
  "Roadmap status" note for what that does and doesn't mean going
  forward.
- **Following session:** Phase J expanded with the two capabilities
  ADR-019 had deliberately deferred — the project owner asked for them
  directly. Natural-language admin queries (`/admin_ask <question>`):
  built as a two-call classify-then-answer flow rather than one call,
  specifically so the model's output is NEVER SQL or anything
  concatenated into a query — it only ever picks a name from a 9-entry
  fixed whitelist (`queries.go`) plus two integers clamped to safe
  bounds, documented as ADR-020. Tested adversarially: a
  classification response containing a SQL-injection-shaped intent
  value is still rejected by `IsKnownIntent` regardless of how
  "validly" it parsed as JSON. Balance-suggestion tooling
  (`/balance_report [days]`): real per-unit usage/win-rate correlations
  from completed raids' `raid_forces` data, with the system prompt
  explicitly forbidding the model from stating a unit is
  over/underpowered as fact from what is admittedly correlational, not
  causal, data — checked by a test that greps the actual prompt text
  for that constraint rather than just trusting it was written in.
  34 new tests (169 → 203, all passing). Full `go build ./... && go
  vet ./... && go test ./...` confirmed clean for the whole repo.
  Branched as `phase-j2-admin-console-tools`, stacked on the
  already-pushed (not yet merged) `phase-j-dev-console` branch.

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
3. Pick up the "Recommended next task" in §3 unless the project owner
   has redirected you.
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
