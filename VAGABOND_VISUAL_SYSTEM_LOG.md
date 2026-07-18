# VAGABOND_VISUAL_SYSTEM_LOG.md

**Read this before touching anything under `assets/visual-system/`.** This
is the working log for the **Visual Identity workstream** — replacing
Unicode emoji with an original custom-emoji set, and the game's full
brand system: logos, social assets, sticker packs, and animated
variants. As of v7 (§6), the project owner made explicit that this is
the actual roadmap, not a someday-maybe extension — see §7 and the v7
change-log entry before assuming "pilot icons only" is still the
scope. It is maintained by a Claude session working directly with the
project owner (Asiwaju) in chat, iterating on art direction turn by
turn.

This file is **not** `PROJECT_MASTER_PLAN.md` or
`SPACEHUNT_PHASE7_LOG.md`. Those are gameplay/engine workstreams. This
one is pure asset design and, eventually, a thin integration seam into
the bot. If you're an AI or human picking this up cold:

- Emoji/icon/logo/brand asset work → this file.
- Gameplay/combat/economy/hero/world/UX → `SPACEHUNT_PHASE7_LOG.md`.
- `internal/ai/*`, Governor, Fleet Commander, Economy Advisor →
  `PROJECT_MASTER_PLAN.md`.
- Nothing in this workstream has touched Go code yet. No handler,
  command, or table has been added or modified. This log covers art
  assets and the script that generates them only, until §5 says
  otherwise.

## v10 ground truth — 2026-07-18

This correction supersedes conflicting older statements in §§1, 5, 8, 10,
and 11. It exists because the preceding work log contained stale claims about
the upload path and could otherwise cause a destructive rerun.

- The 11 legacy SVG/WebM pilot assets remain unchanged. Their current live
  state is not inferred here; any claim of Telegram verification must identify
  the exact asset version and client test.
- A separate, original **Blender-rendered v10 Oracle** now exists at
  `assets/visual-system/animated/oracle_3d_v10/oracle_3d_v10.webm`, with
  matching 100/128/256/512px transparent PNGs and an editable `.blend` source.
  It is locally render-verified but **has not been uploaded to Telegram**.
  See `assets/visual-system/V10_ORACLE_3D_TEST_REPORT.md` for the complete
  source, toolchain, visual-QA, and media-contract record.
- `pipeline/telegram_upload.py` and `verify_animated_pilot.py` must not be
  used for production or test uploads from the known duplicate-filled pilot
  set. Their positional mapping assumptions can associate an incorrect
  `custom_emoji_id` with an icon or create duplicates. The legacy uploader now
  refuses to run.
- The approved test path is `pipeline/telegram_fresh_test_set.py`. It is
  dry-run by default, creates exactly one fresh isolated v10 Oracle set only
  with `--apply`, and refuses to touch an existing set. Its owner-side sequence
  is documented in `assets/visual-system/TELEGRAM_TEST_RUNBOOK.md`.
- `mapping.json` remains intentionally absent. There is no Go integration and
  no claim that v10 is a production replacement until the fresh client test is
  approved and a separate deterministic production-pack mapping design exists.

---

## 0. Source brief

Requested directly by the project owner, in chat, across several
turns:

1. Replace the ~173 unique Unicode emoji currently hardcoded across 46
   Go files (`grep`-counted, see §1) with an original custom emoji set
   — Telegram supports bot-owned "custom emoji" sticker sets
   (`createNewStickerSet` with `sticker_type: "custom_emoji"`), which
   lets a bot ship emoji nobody else can use.
2. Original art only — explicitly told not to copy Apple/Google/
   Telegram/Microsoft emoji designs. Reference supplied (video of
   Telegram's own Premium animated emoji, e.g. the crystal ball) was
   for *rendering quality* (glossy, lit, soft highlight, colored glow
   halo) — not for subject matter or shape language, which stays
   Vagabond's own military/post-apocalyptic/industrial vocabulary.
3. Longer-term ambition, not yet started: the same visual language
   extended to an ecosystem logo, social media assets, and every other
   game asset (see §7 — deliberately unscoped for now, see note
   below).
4. This log, kept current enough that a cold read tells the next
   session everything it needs.

**Scope note for whoever picks this up:** the brief as given is
effectively unbounded ("everything designing," logos + social + every
asset). That's being treated as a backlog, not a single task — see §7.
Each session should pick one concrete deliverable, ship it well, log
it, and let the project owner redirect from there. Don't try to boil
the ocean in one sitting; it produces worse art, not more of it.

---

## 1. Current Status (as of this session)

| Item | Status |
|---|---|
| Emoji usage audit (`internal/`, `*.go`, `*.md`, `*.sql`) | **Done** — 173 unique emoji, 1,962 occurrences, 46 files. Script not yet committed (was a one-off `python3` scan; worth productionizing if the full 173 gets built out — see §7). |
| Art direction / style guide | **Done, v5.** Multi-tone (2, occasionally 3, palette colors per icon) locked in — see §2. |
| Pilot icon batch (10 of 173), static | **Done, v3.** SVG + PNG (100/128/256/512px) in `assets/visual-system/`. `oracle` (11th) is animated-only, no static variant. |
| Pilot icon batch, animated | **11 icons now (10 + `oracle`). Confirmed live in Telegram as of v6/v8: the original 10.  NOT yet verified: the v9 "liquid glass" upgrade (chromatic rim + depth particles) applied to all 9 non-electricity icons, and `oracle` as a new 11th icon — both pushed to the repo but unrun through `telegram_upload.py` yet.** See v8 checkpoint 6 in §6. Live set also has ~10 leftover static duplicates from earlier test runs — not yet cleaned up, see `delete_stale_stickers.py` (§10/v8). |
| Remaining ~163 icons | **Not started.** Next candidate once the animated pilot batch is confirmed in real Telegram (§8). |
| Telegram custom-emoji sticker-set upload script | **Written, updated for animated icons, still unrun** (§8/§10 — this sandbox cannot reach `api.telegram.org`). `telegram_upload.py` now uploads video-format for any icon with a WEBM and falls back to static PNG otherwise, and replaces (not duplicates) anything already in the set. |
| Go source: swap literal emoji → `custom_emoji_id` entities | **Not started.** Needs `mapping.json` populated by an actual `telegram_upload.py` run (owner-side) and a helper in `internal/bot` — proposed but not built, see §7. |
| Ecosystem logo | **Not started.** |
| Social media asset kit | **Not started.** |
| Sticker pack / Lottie / TGS animation | **Not started** (WEBM/VP9 chosen over TGS for the animated pilot — see §10). |

**Recommended next task:** get the project owner to run `telegram_upload.py`
(now animated-aware) from a machine with normal internet access, and
confirm all 10 icons render and animate correctly at true inline size
in a real Telegram client — this is the one thing this workstream
still cannot self-verify (§8). Once confirmed: (a) clean up the known
duplicate-sticker mess in `vagabond_pilot_by_<bot>` (§10), then (b)
decide whether to extend the animated multi-tone treatment to the
remaining 163 icons, or pivot to the ecosystem logo. Don't start both
in parallel — see the original scope note above.

---

## 2. Style Guide (v3 — current)

**Palette** (semantic, not decorative — pick by meaning, not
preference):

| Role | Color | Hex (mid-tone) |
|---|---|---|
| Structure / neutral hardware | Gunmetal | `#2e3238` |
| Structure / light hardware | Steel | `#96a2ad` |
| Wear, hazard-adjacent, cargo | Rust orange | `#c2591c` |
| Tech, energy, AI, comms | Electric cyan | `#39d6ec` |
| Hazard / caution | Radiation yellow | `#dcbe28` |
| Combat, failure, danger | Danger red | `#c72a1e` |
| Premium accent / high-energy | Gold | `#e0a824` |

**Amendment (v5):** earlier guidance implied roughly one accent color
per icon. Project owner pushed back — icons that pair two colors
deliberately (not randomly) read as noticeably more premium than a
single-hue icon, even with identical shading technique. New rule:
**icons may combine 2, occasionally 3, palette colors** when there's a
real reason for the pairing (a "core + arc" relationship, a
"structure + energy" relationship, etc) — every color still has to
come from this table, never a one-off hex invented for a single icon.
Reference case: `electricity` v2 — danger-red main bolt as the "core
discharge," gold/yellow secondary arc-bolts crackling around it as the
"overflow energy." Cohesion still comes from the shared gradient
defs and construction rules below, not from a one-color-per-icon
restriction — that restriction was overcautious, not what was making
things feel unified.

**Construction rules** (every icon, no exceptions):

1. 100×100 viewBox. Silhouette fills roughly the 8–92 safe zone —
   Telegram custom emoji render as small as ~18px, so no detail finer
   than ~4 units wide survives.
2. Radial gradient, not flat fill, on every major shape — light source
   fixed at upper-left (`cx≈30%, cy≈24%`), 4 stops (bright highlight →
   midtone → shadow → near-black edge). This is what gives the
   "domed metal" read instead of flat vector icons.
3. A soft white `glassDome` radial-gradient ellipse over the upper
   ~40% of the primary shape — this is the "premium glossy" cue from
   the reference video, reinterpreted in our own materials instead of
   copying Telegram's actual character art.
4. A colored, heavily blurred glow halo behind the whole icon
   (`blur3`, ~40 unit radius, 30% opacity), color matched to the
   icon's dominant accent. This is what makes it read as "premium"
   sitting on a dark chat background instead of "generic app icon."
5. A dark, offset duplicate of the primary shape behind it (drop
   shadow baked into the shape, not just the SVG-level filter) plus
   the `softshadow` filter on the whole group — two shadow layers,
   not one, or icons look like flat stickers.
6. Rivets/bolts use the shared `bolt` radial gradient (bright fleck →
   steel → dark edge), never a flat circle — this is the recurring
   "industrial hardware" motif tying every icon to the same universe.
7. 1–3 short, low-opacity scratch lines per icon (grime/wear), color
   picked from the shape's own shadow tone, not pure black.
8. Every color reference goes through the shared `<defs>` gradients
   (`gunmetal`, `steel`, `rust`, `cyan`, `radyellow`, `danger`, `bolt`,
   `glassDome`) — never a one-off hex fill on a major shape. This is
   the single biggest lever for keeping 173 icons feeling like one
   family instead of 173 individual efforts.

**Explicitly rejected directions** (so nobody re-litigates these
without a reason):

- Flat single-tone fills (v1) — read as generic flat-icon-pack, not
  "premium."
- Linear gradients only, no dome highlight (v2) — better, still read
  as matte/plastic rather than glossy.
- Literal reproduction of Telegram/Apple/Google emoji shapes or
  color palettes — against the explicit brief; also a copyright/ToS
  concern independent of the brief.

---

## 3. Asset Inventory (this session)

```
assets/visual-system/
├── svg/                  10 source SVGs, v3 style (static)
├── png/                  10 icons × 4 sizes (100/128/256/512px)
├── animated/             11 icons, WEBM/VP9/alpha, 100×100, ≤3s loop
│   └── <name>/<name>.webm   one per pilot icon (v4: electricity only;
│       v6: 10 total; v8/v9: `oracle` added as the 11th). `frames/`
│       subdirs are gitignored build scratch, regenerate via the
│       pipeline scripts below, not committed.
├── animated/electricity_glass_prototype/   v8 "liquid glass" material
│   prototype, NOT the live confirmed electricity.webm — separate path
│   on purpose, awaiting owner sign-off (§6 v8 checkpoint 4)
├── pipeline/
│   ├── build_icons.py          regenerates svg/ + png/ from scratch
│   ├── animate_electricity.py  electricity's animation (v5, red/gold)
│   ├── animate_pilot_batch.py  the other 9 icons' animations (v6)
│   ├── animate_electricity_glass_prototype.py  v8 liquid-glass
│   │   technique prototype, renders to a separate path, does not
│   │   touch the live electricity.webm
│   ├── telegram_upload.py      real upload/verify script (confirmed
│   │   working, v8 — all 10 icons live)
│   ├── delete_stale_stickers.py  dry-run-by-default cleanup for the
│   │   duplicate-sticker mess (v8, §10); needs --confirm to delete
│   ├── verify_animated_pilot.py  narrow "does animation work at all"
│       smoke test against an isolated test set (§10)
├── skills/
│   └── vagabond-premium-emoji-style/SKILL.md   portable style/process
│       skill for future sessions (v8) — technique + hard-won process
│       rules; defers to THIS log for status/history, doesn't duplicate it
├── previews/
│   ├── preview_sheet_v3.png     static contact sheet, dark bg
│   ├── all10_preview.mp4        all 10 animated icons tiled + looping,
│   │   dark bg, for reviewing the full set together (v6)
│   ├── all11_v9_preview.mp4     all 11 (10 + oracle) with the v9 glass
│   │   upgrade applied, 4x3 grid, dark bg (v8 checkpoint 6)
│   └── electricity_glass_prototype_vs_original.mp4   literal
│       side-by-side of the v8 liquid-glass prototype vs. the live
│       confirmed version, for owner sign-off before scaling to all 10
├── mapping.json          NOT YET CREATED — written by telegram_upload.py
│   once the project owner actually runs it (§8)
└── reference/
    ├── telegram-premium-emoji-reference.mp4   the first clip the
    │   project owner sent to explain the target rendering quality
    │   (Telegram's own Premium animated emoji, incl. the crystal
    │   ball). Kept as the raw video, not just a written description
    │   — a future session should watch it, not take my paraphrase of
    │   it on faith. It is a reference for *technique* (glossy render,
    │   soft top light, colored glow) only — see §2 and the explicit
    │   "rejected directions" note about not copying the actual
    │   character designs shown in it.
    ├── premium-emoji-reference-2-full-range.mp4   second clip (v7),
    │   a wider sweep of Telegram's Premium emoji picker — full
    │   illustrated character stickers (a boxer mid-punch), large
    │   expressive symbol emoji (giant eyes), a ghost, more. Raised a
    │   real gap: this is more illustrated/character-driven than the
    │   current geometric icon set. See v7 in §6 before assuming the
    │   current pipeline can close that gap by just adding color.
    └── stills/
        ├── crystal-ball-emoji-picker.png   frame ~15s, the icon that
        │   prompted this whole v3 pass — study this one first.
        └── emoji-topics-panel.png   frame ~30s, shows the Telegram
            "Topics" emoji-removal panel the project owner screenshotted
            earlier in the conversation — useful context for what a
            typical bot's current stock-emoji footprint looks like.
```

Icons done: `warning`, `failure`, `shield`, `transport`, `ai_mech`,
`gear`, `satellite`, `combat`, `electricity`, `scrap`. These were
picked as the 10 highest-frequency emoji in the current codebase (⚠❌
🛡🚚🤖⚙🛰⚔⚡🔩), not arbitrarily — see §1's audit.

**To regenerate:** `pip install cairosvg --break-system-packages && \
python3 assets/visual-system/pipeline/build_icons.py`. Writes into
`assets/visual-system/svg/` and `assets/visual-system/png/` in place.
Edit the `icons[...]` dict in that file per-icon; edit the shared
`<defs>` block to change the style guide globally (§2's whole point is
that a palette/gradient change there propagates everywhere).

---

## 4. Architecture Decision Records (ADRs)

**ADR-V1: Custom emoji over static image replacement.**
Considered just swapping images in bot message text (e.g. sending
these as inline images). Rejected — Telegram custom emoji
(`custom_emoji_id` message entities via a bot-owned sticker set) are
the only mechanism that lets these render inline in text exactly where
a Unicode emoji does today, at the size the game's dense HUD-style
messages (see the screenshots the project owner shared) depend on.
Image attachments would break every existing message layout.

**ADR-V2: One shared `<defs>` block, referenced by ID, not per-icon
colors.**
Every gradient is defined once and referenced by `url(#id)`. This was
a deliberate constraint from v1 onward so that a future global palette
change (e.g. project owner wants "more red" across danger icons) is a
one-line edit, not a 173-file find-and-replace.

**ADR-V3: Pilot batch = top 10 by usage frequency, not first 10
alphabetically or by category.**
Maximizes how much of the 1,962 total emoji occurrences get upgraded
per icon designed — `warning` alone is 250 occurrences (12.7% of all
emoji use in the codebase).

**ADR-V4: Store the raw reference video, not just a written
description of it.**
Could have just expanded §2's prose instead. Rejected — a design
brief this subjective (visual "premium" feel) is a source that should
be revisitable directly. A written description of a highlight/glow
technique is a lossy compression of the actual frame; if v4/v5 needs
to re-derive intent from scratch, or if the project owner disputes
that a future batch matches the original ask, the raw clip is the
ground truth, not this log's interpretation of it. 25MB video +
2 stills is a trivial repo-size cost for that.

---

## 5. Known Issues / Not Yet Done

- No Telegram sticker-set upload script exists yet. Needs: a bot
  token with sticker-set permissions, `uploadStickerFile` →
  `createNewStickerSet(sticker_type="custom_emoji")` → record the
  returned `custom_emoji_id` per icon into a mapping file (proposed:
  `assets/visual-system/mapping.json`, `{"warning": "5XXXXXXXXXXXXXXXXXXX", ...}`).
- No Go-side integration. The 1,962 call sites still use literal
  Unicode emoji. Swapping them needs a small helper (proposed:
  `internal/bot/emoji.go` with a `Custom(key string) string` or
  message-entity-builder function) plus a mechanical pass over the 46
  files — deliberately not started until the art + upload pipeline is
  proven end-to-end on the pilot 10, so we don't do that pass twice.
- Style guide has only been stress-tested on small hardware/UI-style
  icons. Not yet validated against a logo (different silhouette
  constraints — needs to work as a large mark, not just a 20px inline
  glyph) or a social-media asset (different aspect ratios, needs to
  work without the dark-chat-background assumption baked into every
  icon's glow halo right now).

---

## 6. Change Log

- **v1 (this session, iteration 1):** First 10 icons. Flat single
  linear-gradient fills, basic rivets, drop-shadow filter. Project
  owner feedback: "too flat/simple, want more depth."
- **v2 (this session, iteration 2):** Switched to radial gradients
  (lit upper-left), added specular highlight ellipses, ambient
  occlusion at shape junctions, scratch texture, gradient-based rivets
  with highlight flecks. Not yet reviewed by project owner when v3
  work started (project owner instead supplied the Telegram Premium
  emoji reference video before responding to v2).
- **v3 (this session, iteration 3):** Added the `glassDome`
  broad soft highlight and per-icon colored glow halo to match the
  glossy/premium *rendering quality* of the reference video, richer
  4-stop gradients, without adopting Telegram's actual character
  designs (kept Vagabond's own hardware/military subject matter).
  Assets + this log committed to `assets/visual-system/` and repo
  root respectively.
- **v3.1 (this session, iteration 4):** Project owner correctly pointed
  out that a static contact sheet doesn't prove anything about how
  these actually render as Telegram custom emoji at real inline size —
  see §8, that verification hasn't happened yet. Archived the raw
  reference video + two key stills into `assets/visual-system/reference/`
  per project owner request, so future sessions have the original
  source instead of relying on this log's paraphrase of it (ADR-V4).
- **v3.2 (this session, iteration 5):** Wrote
  `assets/visual-system/pipeline/telegram_upload.py` — the real
  Telegram upload/verification script (uploadStickerFile →
  createNewStickerSet → mapping.json → live test message). Confirmed
  this sandbox cannot reach `api.telegram.org` (network allow-list),
  so the script is written and syntax-checked but **not run**.
  Project owner has the credentials to run it themselves; see §8.
  **Still awaiting: (a) in-Telegram render verification, (b) sign-off
  before scaling to the remaining 163 icons.**
- **v4 (this session, iteration 6):** Project owner sent two videos of
  the actual live static pilot next to Telegram's animated Premium
  emoji reference and correctly rejected it — static was never going
  to match. Root-caused to a file-format gap, not an art-direction
  gap (see new §10). Built the first animated icon (`electricity`,
  WEBM/VP9/alpha, ~13KB, 1.25s loop) and a standalone verification
  script (`verify_animated_pilot.py`) using an isolated test sticker
  set so it doesn't collide with the main set's existing duplicate
  mess (also documented, see §10's last paragraph). **Confirmed
  working** — project owner verified the animated custom emoji
  actually animates in a real Telegram client.
- **v5 (this session, iteration 7):** Project owner asked for richer,
  more "sweet"/premium color combinations per icon rather than one
  accent color each — gave `electricity` in red-with-gold as the
  concrete example. Rebuilt it: danger-red main bolt, two golden
  secondary arc-bolts crackling around it with independent flicker
  timing, dual-tone (red+gold) glow halo, gold-tinted sparkle motes.
  Added the Gold entry and the multi-color amendment to §2. Fixed
  `verify_animated_pilot.py` to replace (`replaceStickerInSet`) the
  existing test sticker instead of skipping when the test set already
  exists, so re-running it after a design change actually shows the
  new version instead of the stale one. **Confirmed by project owner**
  on the red/gold redesign specifically.
- **v6 (this session, iteration 8):** Project owner confirmed
  `electricity` v5 and said to proceed with full authority: same
  two-tone-with-a-reason animated treatment on the other 9 pilot
  icons, WEBM as the standing format going forward (VP9, alpha,
  100×100, ≤3s loop — Telegram's best custom-emoji format for real
  motion; confirmed as the right call, see §10), and no in-icon text
  (a 100×100 canvas renders ~18–20px inline, well below any legible
  glyph size — the message text already carries the label, e.g. "⚡
  electricity", and that's what actually works instead of baking words
  into the icon itself). Built `pipeline/animate_pilot_batch.py`, one
  bespoke motion per icon rather than a generic pulse applied
  uniformly:
  - `warning` — red rim-flash (double pulse/loop) + exclamation glow
  - `failure` — gold spark-burst radiating from the X on a beat
  - `shield` — cyan core breathing (radius+opacity) + a gold ring
    chasing the shield's rim via animated `stroke-dashoffset`
  - `transport` — wheel spokes actually rotate (`transform=rotate`
    driven by `t`), headlight opacity pulses, three gold dust
    particles drift/fade behind the cab on staggered phases
  - `ai_mech` — a bright scan-line sweeps down the visor rect (clipped
    to the visor shape), a gold ping ring pulses on the antenna tip
  - `gear` — the whole icon rotates continuously (`transform=rotate`
    on the outer group), one gold spark flick timed to the loop seam
    so it reads as "once per turn" rather than once per tooth
  - `satellite` — three gold ping rings expand from the dish on
    staggered phases, both solar panels flicker independently
  - `combat` — a bright glint slides down each blade's long axis
    (independent phase per blade), the red core "beats" (two-lobe
    pulse, not a plain sine, to read as a heartbeat)
  - `scrap` — gold weld-spark streaks burst off the center bolt at
    randomized angles (seeded, so it's reproducible) on a beat
  All colors still route through the shared palette (§2) — every
  "gold" or "red" accent above is `url(#gold)` / `url(#danger)` from
  the same `<defs>`, never a one-off hex. File sizes: `ai_mech` 20KB,
  `combat` 12KB, `failure` 12KB, `gear` 48KB (was 66KB at the default
  36-frame/crf-30 encode — dropped to 24 frames + crf 40 specifically
  for this icon, since a continuously-rotating full-frame silhouette
  is much higher-entropy per frame than a mostly-static icon with a
  small moving accent; still the heaviest of the ten but well under
  Telegram's ceiling), `satellite` 32KB, `scrap` 16KB, `shield` 12KB,
  `transport` 12KB, `warning` 20KB. All 10 (including `electricity`,
  32KB) total ~216KB — trivial. Built a composited preview
  (`previews/all10_preview.mp4`, all 10 tiled on a dark background,
  looped) to sanity-check the full set together before pushing.
  Updated `telegram_upload.py` to actually use the `ANIMATED_ICONS`
  detection it already had but never acted on: icons with a WEBM now
  upload as `format="video"` instead of always falling back to the
  static PNG, and any icon already present in the live set gets
  `replaceStickerInSet`'d instead of duplicated — directly fixes the
  "replace, not just add" gap the previous session flagged. Still
  unrun (§8's network limitation is unchanged) — this is the next
  concrete action for the project owner. **No in-Telegram verification
  yet for the 9 new animated icons** (only `electricity` has been
  confirmed live, per v4). The known duplicate-sticker mess in the
  live `vagabond_pilot_by_<bot>` set (§10) is still there — the
  updated script now detects and reports it but deliberately never
  deletes anything on its own.

- **v7 (this session, iteration 9) — honesty correction + scope
  expansion, no art changes yet.** Project owner pushed back hard on
  how prior sessions' summaries read — "pushed," "done," "confirmed
  working" language sat next to icons that were, in fact, still static
  or still unverified in real Telegram. That critique is fair and
  worth stating plainly rather than re-explaining away: **as of this
  entry, only `electricity` has ever been confirmed animating in a
  real Telegram client (v4). The other 9 icons animated in v6 are
  pushed to the repo but have never been uploaded or seen in Telegram
  — "pushed to git" and "verified" are not the same claim, and future
  entries in this log should not conflate them.**
  Project owner also sent a second, wider-ranging capture of
  Telegram's own Premium emoji picker (archived as
  `reference/premium-emoji-reference-2-full-range.mp4`) — not just the
  crystal ball this time but a sweep across many Premium emoji:
  full illustrated character stickers (e.g. a boxer mid-punch),
  large expressive symbol emoji (giant googly eyes), objects, a ghost.
  Worth naming the honest gap this exposes: those are richer,
  more illustrated, more *character*-driven pieces of art than
  anything in the current icon set, which is deliberately
  geometric/hardware-vocabulary (§2's construction rules — gradients,
  rivets, glass-dome highlight). The current pipeline is good at what
  it does (clean, cohesive, genuinely premium-reading *industrial*
  icons) but hasn't been asked to produce character illustration, and
  a fully procedural shared-`<defs>` SVG system has real limits there
  — a boxer mid-punch needs actual figure/pose design, not a
  parameterized gradient shape. This should be treated as a real
  constraint to design around (richer per-icon illustration, possibly
  less sharing from the common `<defs>` for anything figure-based),
  not glossed over the way the "static vs animated" gap almost was
  in v3.1–v4.

  **Scope, explicitly widened by the project owner:** this workstream
  is no longer "10 pilot icons, maybe eventually more." It's the whole
  Vagabond visual identity system — full 173-icon custom-emoji set,
  Telegram sticker packs, an ecosystem logo, and a social media asset
  kit (banners, profile art, announcement templates) — all built to
  the same premium bar as the reference material. §7's "unscoped
  backlog, one deliverable per session" framing still holds as the
  *execution* discipline (doing all of it simultaneously produces
  worse art, not more of it — that observation hasn't changed), but
  §7 should no longer be read as "maybe someday." It's the actual
  roadmap now; treat it as ordered work, not speculative ideas.
  Immediate next concrete steps, in order: (1) project owner runs the
  updated `telegram_upload.py` and actually confirms all 10 animated
  icons in a real client — the log should not claim v6 is "done" until
  that happens; (2) once confirmed, raise the icon art quality itself
  per this session's feedback (richer detail, not just the multi-tone
  color rule from v5/v6, which technically already answers "add gold
  lightning to electricity" but hasn't been judged against the new,
  higher character-art reference yet); (3) only then decide whether to
- **v8 (this session, iteration 10) — checkpoint 1 of several: new
  reference material, watched and logged before any art changes.**
  Project owner confirmed all 10 v6 animated icons now render and
  animate correctly in real Telegram (§1's "unverified" flag from v7
  is resolved — see the updated table). Also flagged that the live
  `vagabond_pilot_by_<bot>` set now has the 10 working animated icons
  *plus* roughly 10 leftover static duplicates from earlier test runs
  (§10's "known mess," still not cleaned up — see the new cleanup
  script noted below).

  Sent a third reference video, archived as
  `reference/premium-emoji-reference-3-glossy-3d-gifts.mp4`
  (compressed from a 141MB raw capture to 6.4MB via
  `scale=480:-2, crf 28` — the original exceeded GitHub's 100MB
  per-file limit and would have failed to push; resolution/bitrate
  drop only, no cropped content). **What it actually shows, watched
  frame-by-frame, not assumed:** mostly Telegram's paid 3D collectible
  "Gift" figures (e.g. a boxed vinyl-figure-style "Pavel Durov" toy,
  rendered with real box reflections and a glossy plastic-figure
  material), plus a couple of large emoji-preview closeups — a
  fork/knife/plate emoji with a genuinely glassy, reflective metal and
  porcelain finish, and a spiky virus emoji with strong specular
  highlight and soft ambient occlusion.

  **Important distinction to log plainly, not blur:** the collectible
  "Gift" figures are a different Telegram product tier from custom
  emoji — those are (almost certainly) professionally modeled 3D
  assets or hand-illustrated character art with real pose/likeness
  design, not something a parameterized-gradient SVG pipeline
  produces. This reference is genuinely useful for two narrower,
  achievable things: (1) the *material* language — glossy highlight,
  soft reflection, clean transparent background, "liquid glass"
  surface read — which the existing SVG-gradient pipeline can push
  much further than it currently does; and (2) the standing rule that
  no two icons should reuse the same color pairing, which is a
  bookkeeping/design-discipline fix, not a rendering-technology one.
  Treating this video as "go build literal 3D collectible figures"
  would be overpromising against what this pipeline can actually
  deliver; treating it as "push the glass/glossy material further and
  never repeat a palette" is honest and actionable. Proceeding on that
  reading — see the next checkpoint entries for what was actually
  built against it.

  **Checkpoint 2: `pipeline/delete_stale_stickers.py`.** Same network
  restriction as `telegram_upload.py` (§8, unchanged) — written here,
  must be run by the project owner. Dry-run by default (prints what it
  would delete, changes nothing); needs an explicit `--confirm` flag
  to actually call `deleteStickerFromSet`. Decides what's "stale" by
  diffing the live set against `mapping.json`'s known-good
  `custom_emoji_id` values — anything in the set that doesn't match a
  tracked icon is a leftover from the pre-replace-logic test runs.
  Refuses to delete anything if `mapping.json` is missing or has fewer
  than 10 entries, specifically so it can't be run against stale
  ground truth and accidentally delete a real icon.

  **Checkpoint 3: `skills/vagabond-premium-emoji-style/SKILL.md`.**
  Project owner asked for a proper, portable style/process document —
  written as an actual Claude Skill (YAML frontmatter + Markdown body,
  matching the format `skill-creator` documents) so it can be copied
  into `/mnt/skills/user/` and auto-load in future sessions, or just
  read directly from the repo. Deliberately scoped as *technique and
  process*, not history — it defers to this log for "what's actually
  been built and confirmed," to avoid the two documents drifting out
  of sync with each other. Contents: an index of all 3 reference
  videos with an honest read of what each does and doesn't license;
  the current shared-`<defs>` construction system; a color-pairing
  table so no two icons repeat the same primary+accent combination
  (and an explicit note that the current table already has 3 `cyan +
  gold` repeats — a todo, not a pass); a concrete, still-pure-SVG
  "liquid glass" technique upgrade (layered specular highlights, a
  thin inner rim-light stroke, refraction-style accent bands) as the
  actionable response to the v8 reference video, instead of promising
  literal 3D-collectible fidelity; and a "process rules learned from
  an actual mistake" section listing the gitignore concatenation bug,
  the oversized-video push failure, the pushed-vs-verified conflation,
  and the dry-run-by-default pattern — so each of those gets learned
  once, not re-learned by a future session hitting the same wall.

  **Checkpoint 4: `pipeline/animate_electricity_glass_prototype.py`**
  — a working prototype of the SKILL.md's "liquid glass" technique
  (three additions: a second sharp specular highlight near the true
  light-source corner, a thin bright rim-light stroke just inside each
  bolt's outer edge, and a diagonal refraction band clipped to the
  main bolt that sweeps across it once per loop), applied to
  `electricity` only — same discipline as v5's color redesign: prove
  it on the one already-confirmed icon before touching the other 9.
  Renders to `animated/electricity_glass_prototype/` — a separate
  path, so the live, confirmed, working `electricity.webm` is
  completely untouched. Built
  `previews/electricity_glass_prototype_vs_original.mp4`, a literal
  side-by-side of the two so the difference can be judged directly
  rather than described. **Awaiting project owner sign-off before this
  becomes the new standard and gets applied to the other 9** — do not
  skip that step even if the prototype looks obviously better in
  isolation; v3→v4's static-vs-animated gap and v5's color redesign
  were both real, owner-confirmed steps, not assumed ones, and this
  should follow the same pattern.

  One bug worth logging so it isn't hit again: an XML comment inside
  the generated SVG contained a literal `--` in the middle of the
  comment text ("corner -- real glass..."), which is invalid anywhere
  inside an XML comment (not just at the start/end) and broke
  `cairosvg`'s parser with an opaque "not well-formed" error. Fixed by
  rewording the comment; worth remembering that any inline SVG
  comments in these scripts can't contain em-dash-style `--` at all.

  **Checkpoint 5: `oracle` — an 11th icon, testing how far the glass
  technique can go.** Project owner pushed back on checkpoint 4:
  the glass prototype was real progress but nowhere near the
  reference's crystal-ball/Durov-figure fidelity, and asked for an
  original "Vagabond" crystal/orb icon built to the highest version of
  this technique, explicitly calling out that the collectible-figure
  reference reads as "future tech," not 2025-era flat premium emoji.

  **Restated plainly, because it matters more here than anywhere else
  in this log:** the reference material is professional 3D-modeled or
  hand-animated Lottie/After-Effects work. This pipeline is layered SVG
  gradients rendered through `cairosvg`. It cannot literally equal that
  fidelity, and the honest move is to say so directly rather than nod
  and quietly under-deliver again — that exact pattern (claiming more
  than was true) is what v7 already had to walk back once. What this
  checkpoint IS: every SVG-technique lever pushed further than any of
  the first 10 icons went, judged as "how far can this specific medium
  go," not as parity with a professional 3D render.

  Built `pipeline/animate_oracle_prototype.py` — an **original** design
  (not a copy of Telegram's crystal-ball-with-painted-eye; copying it
  would violate this project's own "original art only" rule from §0),
  named `oracle`: a glass sphere on a gold ring stand (the
  object-on-a-pedestal *composition* is a fair technique reference; the
  specific character/glyph design is not) housing a floating mechanical
  iris/aperture as its hero motif — ties thematically to `ai_mech`'s
  visor without reusing it, and reads as a sci-fi scanner core rather
  than a mystical eye. New techniques over checkpoint 4:
  - **Chromatic dispersion rim** — two colored ring strokes (cyan,
    magenta) offset in opposite phase around a beat, visibly separating
    and re-converging, simulating the color-fringing real refractive
    materials show.
  - **Depth-parallax particles** — 6 sparkle motes each carry their own
    orbital radius/speed/blur-radius tied to a `depth` value, so nearer
    ones are sharper/bigger/faster and farther ones are softer/smaller/
    slower — a real depth cue, not just particles scattered at random.
  - **Dual independent refraction bands** at different angles/speeds
    (checkpoint 4 had one).
  - **A genuinely distinct hero motif** rather than the shared
    gradient-body-plus-recolored-silhouette approach every icon before
    this used — directly answering the "everything should read
    different" note.
  Introduces one new shared-palette color, `violet` (deep
  indigo-to-violet glass gradient) — if kept, this needs promoting into
  `build_icons.py`'s shared `<defs>` so future icons reuse it instead of
  redefining it locally, per the skill's "never invent inline" rule.
  Renders to a separate `animated/oracle_prototype/` path (11th icon,
  for testing — not yet added to `telegram_upload.py`'s `ICONS` dict).

  **Real bug hit and fixed, worth remembering:** the iris motif
  silently failed to render at all on the first attempt — a `<g>` had
  both `clip-path="url(#orbClip)"` and `transform="translate(...)
  rotate(...)"`. When both are present on the same element, the
  transform establishes a new coordinate system that the `userSpaceOnUse`
  clip path's own coordinates get evaluated in *after* — so the clip
  circle's `(50,44)` center, which was meant to line up with the
  content, ended up transformed miles away from it, clipping the whole
  motif to nothing. No error was thrown; it just silently vanished.
  Fixed by removing the (unnecessary — the motif stays within the
  orb's radius naturally) clip-path from that group. Worth checking for
  this specific combination (`clip-path` + `transform` on the same
  element) any time something silently fails to render rather than
  erroring.

  Checked at true 100×100 render (not just the zoomed preview) — the
  orb-on-a-stand silhouette and the glass depth cues hold up clearly;
  the iris's fine detail will simplify to a soft glowing core at actual
  Telegram inline size (~18-20px) but the icon still reads as distinct
  from all 10 existing ones at that size. **Awaiting project owner
  reaction to this specific checkpoint before doing anything with the
  other 9** — see `previews/oracle_and_glass_v2_preview.mp4` for both
  prototypes side by side.

  **Checkpoint 6 — rollout to all 9, plus `oracle` promoted to a real
  11th icon.** Project owner didn't object to the `oracle`/glass
  direction and said to continue (also noted GitHub's inline video
  preview looks worse than actual playback — correct, and consistent
  with why Telegram itself, not the GitHub embed, has been treated as
  ground truth throughout this log). Read as approval to scale the v9
  technique to the other 9 icons, per the standing rule of proving one
  before scaling.

  Rather than redesigning each icon's core motion (already
  subject-specific and confirmed live, see v6), layered the v9 glass
  additions on top via a `GLASS_UPGRADE` config + wrapper in
  `pipeline/animate_pilot_batch.py`: each of the 9 gets its own
  chromatic-dispersion rim-light pair and 3 depth-parallax particles,
  with **every icon's rim color pair distinct from every other's** —
  directly extending the color-pairing discipline from a body-material
  question (v6/v8) to this new decorative layer too. Concretely: no
  reused pair among `(amber,red)`, `(gold,red)`, `(cyan,violet)`,
  `(amber,gold)`, `(cyan,teal)`, `(gold,white)`, `(cyan,magenta)`,
  `(red,white)`, `(rust,gold)` across warning/failure/shield/transport/
  ai_mech/gear/satellite/combat/scrap respectively. File sizes grew
  modestly (extra geometry per frame): totals now `warning` 27KB,
  `failure` 14KB, `shield` 18KB, `transport` 27KB, `ai_mech` 26KB,
  `gear` 55KB, `satellite` 47KB, `combat` 32KB, `scrap` 33KB — all
  still trivial for Telegram's limits.

  Checked actual renders at true 100×100, not just assumed the
  technique would look the same as the `electricity`/`oracle`
  prototypes: it does — chromatic fringing is clearly visible on
  `shield` and `combat` especially. `satellite` shows a green color
  blotch at small size; checked and confirmed this is the **pre-existing
  chroma-subsampling artifact** already noted when the original v6 grid
  preview was reviewed (yuva420p at 100×100 has real color-bleed limits
  at this resolution) — not a new bug introduced by this checkpoint.
  Built `previews/all11_v9_preview.mp4` (4×3 grid, all 11 icons
  including `electricity` and `oracle`) to review the whole set
  together.

  **`oracle` promoted from prototype to the real 11th icon:** moved
  `animate_oracle_prototype.py` → `animate_oracle.py`, output path
  `animated/oracle_prototype/` → `animated/oracle/oracle.webm` (the
  old prototype path removed — git history still has it if needed),
  registered in `telegram_upload.py`'s `ICONS` dict (emoji: 🔮, the
  closest existing Unicode emoji this is meant to eventually replace in
  bot text — the *design* is original, see `animate_oracle.py`'s
  header). `delete_stale_stickers.py`'s `EXPECTED_ICON_COUNT` bumped
  10→11 to match. `violet` (introduced for `oracle`) and `gold`
  (introduced back in v5 for `electricity` but never promoted) are now
  both in `build_icons.py`'s shared static-build `<defs>`, per the
  skill's "never invent inline, add to the shared block" rule — so any
  future static icon can reuse either without redefining them.

  **Still unrun in real Telegram** (§8's restriction, unchanged): none
  of this checkpoint's output — the 9 upgraded icons or `oracle` as a
  registered 11th — has been uploaded or seen live yet. Don't read
  "rolled out" as "verified"; that's the exact distinction v7 already
  had to correct once.

---

## 7. Future Ideas (unscoped, not committed to any session)

- Full 173-icon set, using the v3 (or whatever's approved next) style.
- `mapping.json` + Telegram sticker-set upload script (see §5).
- `internal/bot/emoji.go` integration pass (see §5).
- Ecosystem logo — needs its own design pass; the icon style guide is
  a starting point, not a guarantee it'll work at logo scale/context.
- Social media asset kit (profile images, banners, announcement
  templates) — once a logo exists to anchor it.
- Sticker pack (regular Telegram stickers, larger/more expressive than
  inline emoji) and animated (TGS/Lottie) variants of the icon set.
- Productionize the emoji-usage audit script (currently a throwaway
  `python3 -c "..."` one-liner, not committed) so future sessions can
  re-run it instead of re-deriving the count.

---

## 8. Verification Status — IMPORTANT, READ BEFORE CLAIMING THIS IS DONE

**Nothing in this workstream has been confirmed to render correctly
inside actual Telegram yet.** Everything in §1/§3 is PNG/SVG viewed as
static files on a simulated dark background. That is not the same
thing as a Telegram custom emoji rendered inline in a real message at
real size, and the project owner was right to flag that gap.

What "actually verified" requires, that hasn't been done:

1. A Telegram bot with a user who has previously started it (custom
   emoji sticker sets are created against a `user_id`, not just a bot
   token — see the Bot API docs for `createNewStickerSet`).
2. `uploadStickerFile` for each of the 10 pilot PNGs (must be exactly
   100×100 per Telegram's spec — the pipeline already exports this
   size, see §3).
3. `createNewStickerSet(sticker_type="custom_emoji")` to actually
   create the set.
4. Sending a real test message containing those `custom_emoji_id`
   values, on an actual phone/desktop client, at the small inline size
   they'll really be used at (not the 512px preview sheet).
5. Only after that: a real answer to "does this look premium at 18px
   in a chat," not a simulated one.

None of steps 1–4 have been run. This needs either a bot token with
appropriate permissions from the project owner, or the project owner
running the upload themselves with a script this workstream provides.
**Whoever picks this up next should treat the pilot batch as
"art-complete, render-unverified" — not "done."**

**Update, same session:** `assets/visual-system/pipeline/telegram_upload.py`
now exists and does steps 2–4 end-to-end (upload → create custom-emoji
set → write `mapping.json` → send a real test message to the owner's
own Telegram account so they can eyeball every icon at true inline
size). **It has not been run.** The AI sandbox that wrote it has an
outbound network allow-list that does not include `api.telegram.org` —
confirmed via a direct request (`host_not_allowed`) — so this is a
hard environment limitation, not a missing-credential problem. The
project owner needs to run this script from a machine with normal
internet access (see the script's own docstring for the two required
env vars and how to get them). A test bot token was shared in this
chat for this purpose — **it should be revoked from BotFather once
testing is done**, same as the GitHub token earlier in this
workstream's history; pasting live credentials into a chat transcript
means treating them as burned after use, even for a throwaway test
account.

---

## 10. Static vs. Animated — the gap that made the pilot look wrong

**What happened:** the pilot batch (§1–§3) was static PNG/SVG only.
When actually sent as Telegram custom emoji, they sat still next to
Telegram's own Premium animated emoji (project owner's reference
video, e.g. the crystal ball), which visibly pulse/rotate/sparkle.
Project owner correctly called this out — a beautifully shaded static
icon still reads as cheap next to something that moves.

**Root cause, precisely:** this was never a rendering-quality problem
(§2's gradients/highlights/glow are fine). It was a *file format*
problem — Telegram custom emoji come in three formats
(`core.telegram.org/stickers`, `core.telegram.org/stickers/webm-vp9-encoding`):

| Format | What it is | Spec |
|---|---|---|
| `static` | PNG/WebP | exactly 100×100px |
| `animated` | TGS = gzipped Lottie/Bodymovin JSON | 512×512 canvas, ≤64KB, ≤3s, loops, restricted After-Effects feature subset (no images/masks/expressions/3D layers) |
| `video` | WEBM, VP9 codec, alpha channel | 100×100px for emoji specifically, no audio, ≤3s, loops, keep well under 64–256KB (sources vary; aim low) |

§1–§3's pipeline only ever produced the first kind. Nothing else was
wrong.

**Also worth correcting directly:** the reference isn't rendering true
3D geometry either — nobody ray-traces a scene for a 20px emoji. It's
a 2D vector illustration that reads as dimensional because of
*motion* (rotation, drifting sparkle, pulsing light) layered on top of
the same kind of shading §2 already does statically. "Make it 3D" and
"make it move" turned out to be the same request.

**Why WEBM was picked over TGS for the fix:** TGS requires a valid
gzipped Lottie/Bodymovin JSON, hand-authoring which is possible (it's
just shape layers + keyframed transforms) but has a strict validator
on Telegram's side and a much larger space to get subtly wrong with no
local way to preview it before upload. WEBM/VP9 reuses tooling already
in this pipeline (`cairosvg` for frames, `ffmpeg` — confirmed
`libvpx-vp9` available in this environment — for encoding), is
trivially previewable locally before ever touching the Telegram API,
and meets the same visual goal. TGS may be worth revisiting later
purely for its much smaller file-size ceiling if that ever matters at
scale (173 icons × a few KB adds up either way — 173 WEBMs at ~13KB
each is still only ~2.2MB total, not a real constraint yet).

**Pilot animated icon:** `electricity` — done, unverified in real
Telegram as of this entry. Built by
`assets/visual-system/pipeline/animate_electricity.py`, self-contained
(renders 30 frames at 24fps = 1.25s loop, then shells out to `ffmpeg`
to encode straight to `assets/visual-system/animated/electricity/electricity.webm`,
~13KB). Motion: pulsing colored glow halo (matches the icon's existing
glow-halo motif from §2 rule 4, just breathing now instead of static),
a bright point of light traveling along the bolt's centerline once per
loop, and three independently-phased sparkle motes fading in and out —
deliberately the same twinkle motif as the reference crystal ball, but
built from scratch as our own shape (4-point star, not a copy of
Telegram's).

**Verification-in-progress:** `assets/visual-system/pipeline/verify_animated_pilot.py`
uploads just this one animated icon into an isolated test sticker set
(`vagabond_animtest_by_<bot>`), deliberately separate from the main
`vagabond_pilot_by_<bot>` set — that set already has ~20 stickers
(duplicates from an earlier double-run before the UTF-16 fix, see §5's
addendum in this section below) and untangling it isn't worth doing
before answering the more basic question "does an animated custom
emoji actually animate for us at all." Once that's confirmed, the
plan is: replace (not duplicate) each of the 10 pilot entries in the
real set with an animated version, one at a time, via `replaceStickerInSet`.

**Known mess to clean up later, not blocking:** the main
`vagabond_pilot_by_<bot>` set currently has roughly double the
expected stickers because an early test run of `telegram_upload.py`
(before the UTF-16 fix in commit `d786834`) was run twice against an
already-created set, and `addStickerToSet` didn't reject the repeat
the way `createNewStickerSet` rejected the set-level duplicate.
Cosmetic, not correctness-affecting for `mapping.json` (positional
zip against upload order still lines up), but should be tidied with
`deleteStickerFromSet` once we're doing the full animated pass anyway.

**v6 — full 10-icon animated batch:** with `electricity` confirmed
working (v4) and its multi-tone redesign confirmed (v5), the project
owner gave the go-ahead for the same treatment on the remaining 9
pilot icons plus a standing rule: WEBM/VP9/alpha is the format for
*all* future animated custom emoji (not just electricity — this was
worth confirming explicitly since v4's investigation into WEBM vs TGS
was scoped to the one pilot icon). One correction made mid-session:
the project owner asked whether words could be baked into the 100×100
icon itself. Flagged that this won't read — Telegram custom emoji
render as small as ~18–20px inline, and even a single bold glyph turns
to mush below that; the existing convention of pairing the icon with
its name in the message text (e.g. "⚡ electricity") is what actually
carries a label. Built for that, not for text-in-icon.

`pipeline/animate_pilot_batch.py` holds all 9 new animations, each
with a motion chosen for what that icon's subject actually does (full
per-icon breakdown in §6's v6 entry) — a rotating gear actually
rotates, a shield's rim gets a chasing charge-indicator light, a truck
kicks up dust, etc. Deliberately not a single "breathing glow" effect
reused 9 times; that would have been faster but reads as generic
rather than considered. Shared machinery (glow halos, spark streaks,
star-shaped sparkle motes, single/double-pulse timing helpers) is
factored into reusable functions in that file so a future icon can
reuse the vocabulary without copy-pasting a whole animation from
scratch. `gear` needed its own compression pass (24 frames instead of
36, crf 40 instead of 30) since a full-frame continuous rotation is
much higher-entropy per frame than a static icon with a small moving
accent — even trimmed it's the heaviest of the ten at 48KB, still
trivially under Telegram's limit.

`telegram_upload.py` was updated (not rewritten from scratch) to
actually branch on `ANIMATED_ICONS` — it already computed that set but
never used it to pick static vs. video format, so every icon was still
uploading as a static PNG regardless of whether a WEBM existed. Also
added `replaceStickerInSet` support: if `mapping.json` says an icon is
already in the live set, re-running the script now swaps it in place
instead of calling `addStickerToSet` and creating a second copy — this
is the concrete fix for exactly the kind of drift that caused the
duplicate mess described above. The script still cannot run inside
this sandbox (§8's network restriction is environmental, unchanged
since v3.2) and still needs the project owner to run it from a machine
with real internet access before any of the 9 new animations are
verified in an actual Telegram client. `verify_animated_pilot.py` was
left as-is — it's still a useful narrow smoke test for "does animation
work at all" independent of the main set, and doesn't need to know
about all 10 icons to do that job.

---

## 11. How to Resume Work (for the next session, AI or human)

1. Read this whole file first, then watch
   `assets/visual-system/previews/all10_preview.mp4` (all 10 animated
   icons together) before opening any SVG or animation script.
2. Check with the project owner whether `telegram_upload.py` has been
   run yet and what it showed. As of this entry it hasn't — that's the
   single open blocker (§8/§10 v6). If it has, this file's §1/§8/§10
   should already reflect the result; if they don't, the previous
   session forgot to update this log — fix that first.
3. If extending the icon set (static or animated): follow §2's
   construction rules exactly, reuse the shared `<defs>`, don't invent
   a new gradient without adding it to §2's table. For animation
   specifically, give each icon its own motion tied to what the
   subject does (§10 v6) — don't reuse one generic effect across many
   icons.
4. If starting the logo or another asset type: don't assume the icon
   style guide transfers 1:1 (see §5's last bullet) — validate it
   against the new context before committing to it.
5. **After finishing any task, update this file**: move items in §1's
   table, add an ADR if you made a non-obvious call, add to the Change
   Log in §6, update "Recommended next task." Same rule as the other
   two logs in this repo — this file is only useful if it stays
   accurate.
