# VAGABOND_VISUAL_SYSTEM_LOG.md

**Read this before touching anything under `assets/visual-system/`.** This
is the working log for the **Visual Identity workstream** — replacing
Unicode emoji with an original custom-emoji set, and (longer-term) the
game's full brand system: logos, social assets, sticker packs, and
animated variants. It is maintained by a Claude session working
directly with the project owner (Asiwaju) in chat, iterating on art
direction turn by turn.

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
| Art direction / style guide | **Done, v3** — see §2. Locked in after 3 iterations with the project owner. |
| Pilot icon batch (10 of 173) | **Done, v3.** `warning`, `failure`, `shield`, `transport`, `ai_mech`, `gear`, `satellite`, `combat`, `electricity`, `scrap`. SVG + PNG (100/128/256/512px) in `assets/visual-system/`. |
| Remaining ~163 icons | **Not started.** Blocked on project owner sign-off on v3 style (this session ends with that question open). |
| Telegram custom-emoji sticker-set upload script | **Not started.** |
| Go source: swap literal emoji → `custom_emoji_id` entities | **Not started.** Needs a mapping table (old emoji → new custom_emoji_id) and a helper in `internal/bot` — proposed but not built, see §7. |
| Ecosystem logo | **Not started.** |
| Social media asset kit | **Not started.** |
| Sticker pack / Lottie / TGS animation | **Not started.** |

**Recommended next task:** get explicit sign-off on the v3 pilot
(waiting on project owner reply as of this log entry), then either (a)
extend the same style to the full 173-icon set, or (b) start the
ecosystem logo — whichever the project owner prioritizes. Don't start
both in parallel; the style guide in §2 needs to survive contact with
a second, very different asset type (a logo) before it's safe to call
"locked."

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
├── svg/                  10 source SVGs, v3 style
├── png/                  10 icons × 4 sizes (100/128/256/512px)
├── pipeline/build_icons.py   regenerates svg/ + png/ from scratch
├── previews/preview_sheet_v3.png   contact sheet, dark bg, for review
└── reference/
    ├── telegram-premium-emoji-reference.mp4   the actual clip the
    │   project owner sent to explain the target rendering quality
    │   (Telegram's own Premium animated emoji, incl. the crystal
    │   ball). Kept as the raw video, not just a written description
    │   — a future session should watch it, not take my paraphrase of
    │   it on faith. It is a reference for *technique* (glossy render,
    │   soft top light, colored glow) only — see §2 and the explicit
    │   "rejected directions" note about not copying the actual
    │   character designs shown in it.
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

## 9. How to Resume Work (for the next session, AI or human)

1. Read this whole file first, then look at
   `assets/visual-system/previews/preview_sheet_v3.png` before opening
   any SVG.
2. Check with the project owner whether v3 was approved. If a v4
   exists, this file's §2 should already reflect it — if it doesn't,
   the previous session forgot to update this log; fix that first.
3. If extending the icon set: follow §2's construction rules exactly,
   reuse the shared `<defs>`, don't invent a new gradient without
   adding it to §2's table.
4. If starting the logo or another asset type: don't assume the icon
   style guide transfers 1:1 (see §5's last bullet) — validate it
   against the new context before committing to it.
5. **After finishing any task, update this file**: move items in §1's
   table, add an ADR if you made a non-obvious call, add to the Change
   Log in §6, update "Recommended next task." Same rule as the other
   two logs in this repo — this file is only useful if it stays
   accurate.
