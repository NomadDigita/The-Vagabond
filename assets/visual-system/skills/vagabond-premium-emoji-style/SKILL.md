---
name: vagabond-premium-emoji-style
description: Art direction, technique, and process rules for building The Vagabond's custom Telegram emoji, stickers, logo, and social media assets. Use this whenever creating or editing anything under assets/visual-system/ — new icons, animation, color choices, or file-format decisions — not just when explicitly asked for "style guidance." Also use before running or writing any telegram_upload.py-style script, since the process rules here (dry-run defaults, video compression limits, incremental logging) were each learned from a real mistake in this project's history.
---

# The Vagabond — Premium Emoji & Brand Asset Style

## Read this first

This skill is **technique and process**, not history or current status.
For "what's been built, what's confirmed working, what's still open" —
read `VAGABOND_VISUAL_SYSTEM_LOG.md` at the repo root FIRST, in full,
before touching anything. That log is ground truth. This file exists so
the *how* survives even in a session that hasn't read the whole log yet,
and so lessons already paid for in mistakes don't get re-learned.

If anything in this skill contradicts the log, **the log wins** — update
this skill to match it, not the other way around, and note the
correction in the log's change history.

## What this project actually is

Not just 10 icons. The full Vagabond visual identity system:
~173 custom Telegram emoji (replacing hardcoded Unicode emoji in the Go
bot), animated custom-emoji sticker packs, an ecosystem logo, and a
social media asset kit (banners, profile art, announcement templates).
Build it one deliverable at a time — trying to advance all of it in one
sitting produces worse art, not more of it — but don't treat any single
piece as the whole scope.

## Reference material — watch it, don't just read this summary

Three videos live in `assets/visual-system/reference/`. Watch the actual
video before making art decisions from it; a paraphrase (including this
one) loses the parts that matter for judging quality:

1. **`telegram-premium-emoji-reference.mp4`** — Telegram's own Premium
   animated emoji (crystal ball, etc.). Reference for *technique only*:
   glossy render, soft top light, colored glow, subtle motion (rotation,
   drift, pulse). Do not copy the actual character designs shown.
2. **`premium-emoji-reference-2-full-range.mp4`** — wider sweep of the
   Premium emoji picker: illustrated character stickers (boxer
   mid-punch), large expressive symbol emoji (giant eyes), a ghost. Shows
   the range of what "premium" covers — not just objects, full
   characters too.
3. **`premium-emoji-reference-3-glossy-3d-gifts.mp4`** — mostly
   Telegram's paid 3D collectible "Gift" figures (e.g. a boxed
   vinyl-figure "Pavel Durov" toy) plus glossy large-emoji closeups
   (fork/knife/plate, a virus). **Important distinction:** the
   collectible figures are a different Telegram product tier — real 3D
   modeling or hand-illustrated character art with pose/likeness design,
   not something a parameterized-gradient SVG pipeline replicates. Don't
   promise literal 3D-collectible fidelity. DO take from it: the glassy
   material language (strong specular highlight, soft reflection, clean
   transparent background) and apply that to *our* icon vocabulary.

If a future reference video is sent and is too large to commit (GitHub's
hard limit is 100MB per file), compress before committing — this has
already broken a push once:

```bash
ffmpeg -i input.mp4 -vf "scale=480:-2" -c:v libx264 -crf 28 -preset slow \
  -c:a aac -b:a 96k output_compressed.mp4
```

480px wide / crf 28 is plenty for reviewing UI screen recordings and
typically gets a ~140MB phone capture under 10MB with no meaningful
quality loss for reference purposes.

## Current construction system (as of v6/v8 — verify against the log for anything newer)

All icons share one `<defs>` block (gradients, filters) defined in
`pipeline/build_icons.py` and reused by every animation script. Never
invent a new hex color inline — add it to the shared `<defs>` and to the
palette table below so the next icon can reuse it too.

Shared gradients: `gunmetal`, `steel`, `rust`, `cyan`, `radyellow`,
`danger` (red), `gold`, `bolt` (rivet material), plus `glassDome` (a
radial highlight for the glossy top-light look) and shadow/blur filters.

Construction rules, unchanged since v3:
- 100×100 viewBox, exported as 100/128/256/512px PNG for static, WEBM for
  animated.
- Every icon gets: a soft drop shadow, a glass-dome top highlight
  (`glassDome`), a couple of subtle scratch/wear lines (reads as
  "object," not "sticker"), and small rivet details where the subject
  supports it.
- No in-icon text. A 100×100 canvas renders ~18–20px inline in Telegram —
  below any legible glyph size. The message text carries the label
  instead (e.g. "⚡ electricity").

## The v8 rule: never repeat a color pairing

Every icon's *primary* body-material gradient plus its *accent* gradient
(the thing that animates/glows) must be a combination no other icon in
the set uses. This is a bookkeeping discipline, not a rendering
technique — keep a running table and check it before starting a new
icon. As of v8:

| Icon | Primary material | Accent |
|---|---|---|
| electricity | danger (red) | gold |
| warning | radyellow | danger (red) |
| failure | gunmetal | gold + danger |
| shield | steel | cyan + gold |
| transport | rust | cyan + gold |
| ai_mech | gunmetal | cyan + gold |
| gear | steel | rust + gold |
| satellite | gunmetal | cyan + gold |
| combat | steel + gunmetal | danger + gold |
| scrap | steel | rust + gold |
| oracle (11th) | violet | cyan + magenta (chromatic) |

The body-material table above still has real repeats (`cyan + gold`
appears 3-4 times as the *primary accent*) — that's still an open todo.
But as of v9, every icon additionally carries its own chromatic-rim
color pair (a separate, finer-grained layer — see below), and none of
those 9 pairs repeat: `(amber,red)` warning, `(gold,red)` failure,
`(cyan,violet)` shield, `(amber,gold)` transport, `(cyan,teal)` ai_mech,
`(gold,white)` gear, `(cyan,magenta)` satellite, `(red,white)` combat,
`(rust,gold)` scrap, `(cyan,magenta)` oracle's chromatic dispersion.
Note oracle's chromatic pair duplicates satellite's — that's the next
thing to fix if a 12th icon gets added.

## The v8 direction: push toward "liquid glass," not flat gradient

Feedback from the project owner (v8): icons should read as closer to a
polished glass/liquid material, not a flat two-stop gradient circle.
Concrete, achievable moves in that direction (all still pure SVG, no
raster/3D-engine dependency):

1. **Layered specular highlights, not one.** A single big soft highlight
   reads as "glossy plastic." Two or three highlights at different sizes
   and opacities — one large soft one (existing `glassDome`), one small
   sharp one near the true light-source corner, and a thin curved
   highlight *band* following the silhouette's edge — reads as glass or
   liquid because real transparent/reflective materials show multiple
   coincident reflections, not one.
2. **A thin bright rim-light stroke** just inside the outer silhouette
   edge (a duplicate path, stroke-only, `stroke-width` 1–1.5, near-white,
   low opacity, on the *inner* side of the main outline) — this is what
   sells "light passing through an edge" rather than "flat cutout."
3. **Refraction-band accents.** For the animated glow/accent layer,
   consider a thin curved highlight that shifts position or bends
   slightly across the loop (not just opacity pulsing) — this reads as
   light moving through a curved glass surface rather than a lamp
   blinking behind a flat icon.
4. **Keep the background transparent.** Full alpha, not white or dark —
   the "clear background" note from the project owner is about the
   PNG/WEBM alpha channel, and the existing pipeline already does this
   correctly (`yuva420p`, `output_width`/`output_height` with
   transparency) — don't regress it while changing the material look.

None of this is a rewrite of the shared `<defs>` — it's 2–3 additional
highlight/rim-light shapes layered on top of the existing gradient body,
prototyped on ONE icon first (same discipline as v5's electricity
redesign) before touching all 10.

## v9: pushing the glass technique further (still honest about the ceiling)

The project owner asked for the technique pushed closer to the
reference's crystal-ball/collectible-figure fidelity. Restating the v8
honesty note because it matters even more here: that reference is
professional 3D-modeled or hand-animated Lottie/After-Effects work.
Layered SVG gradients through `cairosvg` cannot literally equal it —
say so plainly rather than implying otherwise. What DID move the
needle, prototyped on an original 11th icon (`oracle`, a glass-orb
scanner — see the log's v8 checkpoint 5 for the full writeup):

- **Chromatic dispersion rim** — two colored ring/edge strokes (e.g.
  cyan + magenta) offset in opposite phase around a beat, so they
  visibly separate and re-converge. Simulates the color-fringing real
  refractive materials show at their edges; a single-color rim stroke
  never gets this even at high opacity.
- **Depth-parallax particles** — give each sparkle/mote its own `depth`
  value (0=far, 1=near) and derive its orbital speed, size, and blur
  radius from that value. Nearer particles: bigger, sharper, faster.
  Farther ones: smaller, softer, slower. Randomly-placed same-size
  sparkles read as flat confetti; depth-linked ones read as floating
  inside a real volume.
- **Multiple independent refraction bands** at different angles and
  speeds (not just one) sweeping across the same clipped silhouette.
- **A genuinely distinct hero motif per icon**, not a shared
  gradient-body-plus-recolored-silhouette. Directly answers "everything
  should read different" — the point isn't just varying the color
  table (that's the v8 rule, still separate and still required), it's
  making sure each icon's *central shape/concept* is unique too, not
  just its palette.

**A real bug worth remembering if something silently fails to render
with no error:** `clip-path` and `transform` on the *same* SVG element
interact badly — the transform establishes a new coordinate system that
the clip path's own coordinates then get evaluated inside, so a clip
circle meant to line up with the content can end up transformed far
away from it, silently clipping everything inside that group to
nothing. If a layer vanishes with no error message, check for this
combination before anything else.

## Format spec (unchanged, confirm against the log before assuming)

- Static: SVG source + PNG at 100/128/256/512px.
- Animated: WEBM, VP9, alpha channel (`yuva420p`), 100×100, ≤3 seconds,
  looping, no audio. This is Telegram's best custom-emoji format for real
  motion — confirmed working end-to-end as of v8.
- TGS/Lottie was considered and explicitly not chosen — see the log's
  ADR on this if picking it back up.

## Process rules (each one learned from an actual mistake — don't re-learn them)

- **"Pushed to git" and "verified in Telegram" are different claims.**
  Say both explicitly, every time. A prior session conflated them and
  had to walk it back after the project owner caught it directly.
- **Telegram-API scripts (`telegram_upload.py`,
  `delete_stale_stickers.py`) cannot run inside an AI sandbox** — no
  route to `api.telegram.org`. Write them correctly, hand them to the
  project owner with exact env-var setup instructions, and don't imply
  they've been run when they haven't.
- **Destructive scripts default to dry-run.** `delete_stale_stickers.py`
  requires an explicit `--confirm` flag and refuses to act without a
  complete `mapping.json` as ground truth. Any new script that deletes
  or replaces live Telegram assets should follow the same pattern.
- **Check `.gitignore` after editing it.** A previous session
  accidentally concatenated two ignore patterns onto one line with no
  newline, silently disabling both. `git status --short` after any
  `.gitignore` edit, before trusting what gets staged.
- **Don't let intermediate frame PNGs into git.** Animation scripts write
  per-frame PNGs to `animated/<name>/frames/` before encoding to WEBM —
  these are gitignored on purpose (5,000+ files across the full 173-icon
  set would balloon the repo). Only the final `.webm` is tracked.
- **Compress reference videos before committing** (see the reference
  section above) — GitHub's 100MB per-file limit is real and a push will
  simply fail past it.
- **Log every real step, not just the final result.** If a session might
  run out of budget mid-task, commit and push after each meaningful
  checkpoint (a new script, a new asset batch, a log update) rather than
  batching everything into one commit at the end — that way the last
  completed checkpoint is never lost.

## Workflow for a new icon or asset

1. Read the log's current status table and this skill in full.
2. Check the color-pairing table above; pick an unused primary+accent
   combination.
3. Build static SVG first, reusing shared `<defs>` (add new gradients to
   the shared block, not inline, if genuinely needed).
4. If animating: give it a motion tied to what the subject actually does
   — not a generic pulse copy-pasted from another icon (see §10/§6 v6 in
   the log for nine worked examples of subject-specific motion).
5. Render a composited preview (icon(s) tiled or looped on a dark
   background) and actually look at it before pushing.
6. Update `VAGABOND_VISUAL_SYSTEM_LOG.md` — status table, change log,
   asset inventory as relevant.
7. Commit and push. If the piece of work is large, split it into the
   checkpoints described above rather than one giant commit.
8. Tell the project owner exactly what's confirmed vs. still needs their
   verification (network restriction) — never round up.
