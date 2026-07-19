# v10 Oracle 3D — Render and QA Report

**Status:** locally render-verified, but Telegram rejected the original 71.7KiB
delivery encode as too large during the fresh-set creation attempt on
2026-07-19. A visually inspected 39.8KiB VP9-alpha delivery re-encode replaced
it and passed the conservative 64KiB local gate. **Not production-approved.**

## What changed

`oracle_3d_v10` is an original Vagabond survey-command core, rendered as real
Blender mesh geometry rather than layered SVG. Its visual system is:

- liquid-glass containment sphere with transmission and camera-relative light;
- a bevelled gunmetal/titanium cradle and gold signal ring;
- a rotating eight-blade mechanical aperture around a cyan command core;
- cyan/magenta dispersion rings and independently moving depth motes;
- subtle body yaw/bob that keeps the silhouette readable at inline size while
  the aperture provides the full rotational motion.

It does not reuse a Telegram character, glyph, or collectible design.

## Source and outputs

| Artifact | Path |
|---|---|
| Procedural source | `pipeline/render_oracle_3d_v10.py` |
| Editable Blender scene | `source/oracle_3d_v10.blend` |
| Animated custom emoji | `animated/oracle_3d_v10/oracle_3d_v10.webm` |
| Still exports | `renders/oracle_3d_v10/oracle_3d_v10_{100,128,256,512}.png` |
| Dark, light, and 20px QA sheets | `previews/oracle_3d_v10_{dark,light,inline20}_qa.png` |

## Toolchain actually verified

- Blender **4.5.3 LTS**, headless EEVEE render.
- FFmpeg **8.1.2**, `libvpx-vp9` encode and alpha decode.
- Python **3.14** offline validator.

## Tests performed

| Gate | Result |
|---|---|
| Blender source executed headlessly | Pass — procedural scene, source `.blend`, 48 PNG frames, and four still sizes emitted. |
| Static export dimensions/pixel format | Pass — 100/128/256/512px, all `rgba`. |
| Animated dimensions | Pass — exactly 100x100px. |
| Video codec/frame rate/duration | Pass — VP9, 24fps, 2.000 seconds. |
| Audio | Pass — no audio stream. |
| Transparent alpha | Pass — `alpha_mode=1` plus an explicit `libvpx-vp9` RGBA decode with non-opaque pixels. |
| File size | Pass — 71.7KiB, below the 256KiB delivery ceiling. |
| Visual QA after VP9 encode | Pass locally — inspected on dark and light backgrounds and in a 20px-cell contact sheet. |
| Telegram client/picker/long-press review | Pending owner-side fresh test set. |

The decoder check is important: `ffprobe` reports `yuv420p` for VP9-alpha in
some builds even when the WebM contains alpha, so the validator decodes an
actual RGBA frame rather than trusting that label alone.

## Release boundary

The legacy pilot set is known to contain duplicates, and its legacy uploader
cannot safely reconstruct identities. It is deliberately safety-blocked.
Only `pipeline/telegram_fresh_test_set.py` may be used for this asset: its
default is dry-run, it creates a fresh one-sticker set only, and it refuses to
modify an existing set. See `TELEGRAM_TEST_RUNBOOK.md`.
