# Onboarding Audit Findings (2026-08-13)

Found while auditing `internal/bot/handlers/onboarding.go` and the
first-session flow for `NEXT_GENERATION_FEATURES_PLAN.md` Phase 17
(Onboarding Polish). Same format as `BUGS_AND_INCONSISTENCIES.md`'s
"UI Polish Wave 1" doc - a log of what was found, grouped by severity,
not a rewrite of onboarding itself (per that phase's own explicit
scope: this is a targeted audit-and-fix pass, not a rewrite).

## Real gaps (things a new player would actually miss)

1. **The first-10-minutes quickstart (`gettingStartedGuideText`) never
   mentioned Clans or the Market Exchange at all** - two of the
   biggest social/economic systems in the game, with zero pointer
   toward either in the one piece of text every brand-new survivor is
   guaranteed to see. **Fixed in this pass**: added steps for `/clan`
   and the Market Exchange to the guide.
2. **No structured, rewarded "first actions" checklist existed at
   all** - a new player had no tracked, payoff-bearing path through
   their first raid/research upgrade/clan join/market purchase, only
   prose telling them what to try. **Fixed in this pass**: a
   `PeriodOnce` "🎯 Getting Started" quest chain (4 steps, $500 each)
   added to `internal/game/progression`, reusing Phase 2's
   quest-progress machinery rather than building a third tracking
   system - see `NEXT_GENERATION_FEATURES_PLAN.md` Phase 17's own
   design section for the schema/integration details.
3. **The quickstart guide predates Phase 1/2 (Achievements & Quests)
   entirely** - shipped after the guide text was last written, so
   neither system was mentioned anywhere a new player would see it.
   **Fixed in this pass**: the guide now points to `/quests` and
   `/achievements` directly.

## Notable but out of scope for this pass

4. **The returning-player dashboard (`HandleStart`'s main branch) is
   plain `fmt.Sprintf` text with box-drawing characters, not a Rich
   Message `<table>`** - unlike most panels touched since the "UI
   Polish Wave" and Rich Message work. This is the single
   highest-traffic panel in the entire game (every `/start` from an
   existing player hits it), so a formatting pass here would be high-
   value, but it's a presentation change orthogonal to the "does a new
   player understand what to do" question this phase is actually
   scoped to. Flagged for a future pass, not touched here.
5. **`renderFactionChoice`'s welcome message uses `c.Send` directly**
   instead of `sendPanelWithNavHTML`. This is correct as-is, not a
   bug - there's no account yet at this point in the flow, so there's
   no nav keyboard to attach (`keyboards.MainNavigation()` assumes an
   existing player). Noted so a future reader doesn't "fix" this into
   a bug.
6. **The account-creation/faction-choice flow itself** (Metal Vanguard
   vs. Rust Nomads, the starting-resource baseline, the referral
   system) was walked end-to-end during this audit and found sound -
   no changes made, per Phase 17's explicit "not in scope unless
   specifically found broken" boundary.

## What shipped from this audit

- `gettingStartedGuideText` (`onboarding.go`): added Clan/Market
  Exchange steps, added a step pointing to the new `/quests` "Getting
  Started" checklist and `/achievements`.
- `internal/game/progression/progression.go`: new `TriggerResearchUpgrade`
  trigger (wired into `research.go`'s `HandleUpgradeTechCallback`),
  new `PeriodOnce` quest period (a permanent, never-resetting
  `player_quest_progress` row - see that file's own `periodStart`
  comment), and 4 new `onboarding_*` quest definitions.
- `quests.go`: `/quests` panel gains a third "🎯 Getting Started
  (one-time)" section alongside the existing daily/weekly sections.
