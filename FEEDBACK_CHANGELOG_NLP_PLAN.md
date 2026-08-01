# Feedback, Changelog & Natural-Language Command Plan

Status: written before any of it exists - this is the design. Read this
whole file before writing code. Three features, requested together,
grouped in one document because the third (natural-language commands)
is a genuinely new category of AI usage for this codebase and needs its
own careful reasoning about safety and scope; the first two are small
and self-contained by comparison. Each is buildable and testable
independently - see "Suggested build order" at the end.

This assumes familiarity with two already-established patterns this
plan leans on heavily:
1. **The pending-input chain** (admin.go's `HandleAdminPendingInput`,
   onboarding.go's `HandleOnboardingPendingInput`, both wired ahead of
   `nlp.HandleTextMessage` in main.go's `telebot.OnText` handler): a
   per-user in-memory map tracks "this user is mid-flow on an action
   that needs their next free-text message," checked in order before
   falling through to normal text routing. Milestone 1 adds a third
   link in this chain.
2. **The AI-agent pattern** (`internal/game/researchplanner` is the
   clearest example): a `Planner`/`Advisor`-style struct holding
   `DB *sql.DB` and `AI *ai.Service`, a `SystemPrompt` constant, a
   `BuildUserPrompt` function, and a `Parse*` function turning
   `resp.Text` into a typed Go struct (tolerant of malformed JSON via
   `internal/ai/jsonrecovery.go`). Every existing AI feature built this
   way is **advisory-only** - it recommends, a human taps a button to
   actually act. Milestone 3 breaks that precedent on purpose (see its
   "Why this is different" section) and is designed specifically to
   keep that safe.

---

## Milestone 1: Feedback button + immediate admin delivery

### What exists today
`/feedback [message]` (profile.go's `HandleFeedback`) already inserts
into `feedback_submissions` - but that's it. No admin is ever notified;
someone has to think to query the table. Also slash-command-only, no
button, no free-text flow (type `/feedback` with no arguments and you
get a usage message, not a prompt).

### What this milestone adds
1. **A real entry point.** A "💬 Send Feedback" button (added to
   `MainNavigation` or `ProfileNavigation`, wherever collides least -
   check via the existing `TestNoButtonTextCollisionsAcrossKeyboards`
   guard) that starts a pending-input flow: tap it, bot asks "What's on
   your mind?", the player's next message is captured as the feedback
   text. `/feedback [message]` keeps working exactly as today for
   players who prefer typing it in one shot.
2. **Immediate delivery to admins.** On submit, in addition to the
   existing `feedback_submissions` insert, queue a `notifications.Queue`
   call to every ID in `AdminIDs` with the player's name/username and
   message. Category: `general` (non-mutable - an admin muting routine
   pings should never lose actual player feedback). This is the literal
   "they receive it immediately" ask.
3. **A minimal admin-side list**, `/feedback_inbox` (admin-gated same
   way `/admin` already is), showing the most recent N submissions -
   mainly so a submission isn't only ever visible in a chat log if an
   admin was offline when it arrived.

### Explicitly out of scope for milestone 1
Two-way admin reply threading (admin taps a button, types a reply, it
reaches the original player). Real, but a distinct feature with its own
UX questions (does the player know it's the admin vs. a bot message?
what if multiple admins reply?) - flagged as a natural follow-up, not
bundled in here.

### Schema
No new table - `feedback_submissions` already has everything needed.
Nothing to migrate.

---

## Milestone 2: Changelog home

### Data model
New `changelog_entries` table:

```sql
CREATE TABLE changelog_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category VARCHAR(20) NOT NULL,   -- 'feature' | 'fix' | 'balance'
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    published_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE changelog_reads (
    user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
    entry_id UUID NOT NULL REFERENCES changelog_entries(id) ON DELETE CASCADE,
    read_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, entry_id)
);
```

`changelog_reads` is what makes "at least 5 oldest" meaningful: a brand
new player (or one who's been away) has a backlog of entries they've
never seen. Showing oldest-unread-first means they catch up in
chronological order instead of landing in the middle of an ongoing
story. Once a player is caught up, "oldest 5" naturally becomes "the 5
oldest of whatever's left," which for an active player is just recent
history.

### Publishing
`/publish_changelog` (admin-only, same `AdminIDs` gate as everything
else admin-facing): a guided-input flow (category, title, body) that
inserts a row and immediately broadcasts it to every real player via
`notifications.QueueToAllPlayers` (already built - see
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.2) using category
`general`. This is the "dispatch it to users" half of the ask.

### Player-facing browsing
`/changelog` (and a button, same navigation-collision check as
milestone 1): shows the oldest 5 entries this player hasn't read yet
(per `changelog_reads`), marks them read on view, with a "Show 5 more"
button if more remain unread. If fully caught up, shows the 5 most
recent entries instead (nothing to "catch up" on, so most-recent is the
useful default) - stated explicitly since the doc's own instruction was
literally "oldest," and this is the one place this plan deviates from
that instruction on purpose, flagged here rather than silently decided.

---

## Milestone 3: Natural-language command execution

### The literal ask vs. what's buildable safely
"Understand every single text related to the game" is the goal to work
toward, not a milestone-one deliverable - realistically no system
understands *every* phrasing of *every* possible action on day one, and
claiming otherwise would set up a bad experience the first time someone
phrases something unexpectedly. What's genuinely buildable now: an
AI-powered intent parser wired as the **final fallback** in
`nlp.HandleTextMessage`'s existing chain (after every hardcoded
shortcut already there fails to match, exactly where a "list 300k
scraps in the market" message ends up today with no handler at all) -
covering a defined, growing set of actions, expanded milestone by
milestone rather than attempted all at once.

### Why this is different from every other AI feature in the game
Every existing AI consumer (researchplanner, econadvisor, etc.) is
advisory-only by explicit design - it never spends a resource or
mutates state itself, a human taps a button to actually act. A command
interpreter's entire point is to *act* on the player's behalf, which
breaks that precedent on purpose. The safety design has to compensate:

1. **The AI never touches the database.** It has exactly one job: turn
   free text into a structured `ParsedCommand{Action string, Args
   map[string]any}` via a tool-calling completion request (see
   `internal/ai/types.go`'s `ToolDefinition`/`ToolCall` - already
   built, unused so far since every existing feature is pure
   text-completion). Execution is a hard boundary: a fixed Go
   `switch action` dispatches to the **exact same core functions** the
   button-driven UI already calls (`doDispatchScoutMission`,
   `doGiftPremium`-style `doX` functions per this session's established
   convention) - the same validation, the same resource checks, the
   same everything. The AI can request "list_market_item, scrap,
   300000" but it's `HandlePostListingCallback`'s own logic that
   decides whether that's actually valid, exactly as if the player had
   used the button flow.
2. **Every parsed command gets a confirmation step**, not silent
   execution, for anything that spends resources or commits forces -
   matching this game's own established convention that "Confirm" is
   always its own explicit button (see the raid launch flow, Ghost
   Protocol, and now the styled-button confirm/cancel pairs from the
   previous session). "List 300k scrap for sale" becomes a rendered
   confirmation card ("List 300,000 Scrap for sale - confirm?") with
   Confirm/Cancel styled buttons (danger=cancel, success=confirm,
   reusing keyboards/styled.go), not an instant, irreversible action
   fired from a single ambiguous sentence. Pure read requests ("what's
   my scrap balance") skip confirmation and answer directly - nothing
   to confirm when nothing is being spent.
3. **A fixed action allow-list**, not an open-ended "do anything"
   tool. The tool schema offered to the model lists only the specific
   actions this milestone actually implements (starting set below);
   anything outside it, the model is instructed to respond with a
   plain-text clarifying question instead of inventing an action name
   that has no matching case in the dispatch switch.
4. **Cost-bounded like every other AI feature.** New
   `FeatureCommandInterpreter` entry in `internal/ai/config.go`'s
   `AllFeatures()`, subject to the exact same per-user/global daily
   budget enforcement (`Service.checkBudget`) already protecting every
   other feature - a burst of natural-language messages can't run up
   an unbounded bill.

### Starting action set (deliberately small)
Chosen for being unambiguous, already having a battle-tested `doX` core
to dispatch to, and covering the example in the request:

- `list_market_item` (resource, quantity, price) → `HandlePostListingCallback`'s core
- `check_resources` (read-only, no confirmation needed)
- `dispatch_scout_mission` (count) → `doDispatchScoutMission` (already
  extracted as a testable core - see scoutmissions.go)
- `check_scout_status` (read-only)

Each additional action is its own small addition to the tool schema +
one new `case` in the dispatch switch + its own confirmation card
template - designed to grow this way rather than needing a rewrite to
add the 5th, 20th, or 100th action.

### What "300k" needs to parse to
Quantity shorthand (`300k` → 300000, `1.5m` → 1500000) is exactly the
kind of thing a tool-calling model handles naturally as part of
structured extraction (the tool's `quantity` parameter is typed as a
number, not a string, so the model resolves the shorthand itself as
part of emitting the tool call) - no separate regex parser needed.

---

## Open questions this session is deciding rather than blocking on
(per standing authorization - other developers are active on this
project and everything below is documented either way)

1. **Changelog "oldest" vs "most recent" once caught up** - decided
   above (oldest-unread while there's a backlog, most-recent once
   caught up).
2. **Confirmation card styling** - reusing `keyboards/styled.go`'s
   danger/success pattern from the previous session rather than
   inventing a new one.
3. **Where the Feedback button lives** - decided at build time by
   whichever existing keyboard has the least collision risk per
   `TestNoButtonTextCollisionsAcrossKeyboards`, not pre-committed here.
4. **Command interpreter action allow-list starting set** - the four
   listed above; expanding it is explicitly designed to be incremental,
   not a decision that needs to be exhaustive up front.

## Suggested build order
1. **Feedback button + admin delivery** (milestone 1) - smallest,
   self-contained, immediately valuable, no schema risk (table already
   exists).
2. **Changelog** (milestone 2) - self-contained, depends only on
   already-built `QueueToAllPlayers`.
3. **Natural-language command execution** (milestone 3) - depends on
   nothing from 1/2, but saved for last because it's the largest and
   most novel piece, and the tool-calling code path
   (`ai.ToolDefinition`/`ToolCall`) has never been exercised by any
   existing feature in this codebase - worth having the smaller wins
   banked first.
