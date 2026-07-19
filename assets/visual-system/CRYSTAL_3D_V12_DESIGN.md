# Crystal 3D v12 - Design and Render Record

## Intent

`crystal_3d_v12` replaces the rejected Oracle art direction as the next
standalone visual-system experiment. It is deliberately an unmistakable
crystal ball at Telegram inline scale, not a scanner, eye, gear, or generic
purple orb.

The project owner supplied an owned crystal reference. This asset uses its
high-level material and silhouette cues only, rebuilt as a fresh Vagabond
scene: violet liquid glass, a compact warm-gold plinth, an internal faceted
light core, and three moving four-point glints. It does not reuse the
reference recording as an upload asset.

## Motion language

- The whole ball completes one very slow, seamless turn over two seconds.
- Internal facets counter-rotate, giving the sphere a refractive depth cue.
- Star glints sway through a small loop instead of simply flashing on/off.
- The gold pedestal remains stable so the moving sphere reads as an object,
  not a spinning flat sticker.

## Delivery contract

- 100 x 100 transparent PNG frames.
- Two-second, 24fps, silent VP9-alpha WebM on successful render.
- Conservative target under 64 KiB before a Telegram fresh-set test. The
  public Telegram ceiling is higher, but this project has observed a stricter
  live rejection and therefore validates to the lower working budget.

## Files

- `pipeline/render_crystal_3d_v12.py` - deterministic Blender scene and VP9
  encode command; it never contacts Telegram.
- `source/crystal_3d_v12.blend` - editable scene source generated before the
  first render attempt.
- `animated/crystal_3d_v12/crystal_3d_v12.webm` - intended final output;
  absent until the renderer completes.

## Verification record - 2026-07-19

1. Python syntax for the renderer script: **PASS**.
2. Blender scene/source generation: **PASS** (`crystal_3d_v12.blend`, 918,379
   bytes).
3. First-frame EEVEE render: **NOT COMPLETE**. A one-frame, one-sample,
   100px probe consumed CPU for more than a minute without writing a PNG, so
   it was stopped. The same behavior occurred for an isolated minimal scene;
   this is a current local Blender-renderer failure, not a claim that the
   v12 art is rendered or Telegram-ready.
4. Video validation, Telegram fresh-set creation, and client review: **not
   run** because no WebM exists yet.

## Re-run command

Use the repository's portable Blender and ffmpeg paths. Begin with the
one-frame probe, then run the final command only after it writes a PNG.

```powershell
$repo = 'C:\Users\PC\Documents\Codex\2026-07-18\nomaddigita-the-vagabond-https-github-com\work\The-Vagabond'
$blender = 'C:\Users\PC\Documents\Codex\2026-07-18\nomaddigita-the-vagabond-https-github-com\work\toolcache\blender-4.5.3\blender.exe'
$ffmpeg = 'C:\Users\PC\Documents\Codex\2026-07-18\nomaddigita-the-vagabond-https-github-com\work\toolcache\ffmpeg\bin\ffmpeg.exe'

& $blender --factory-startup -b -P "$repo\assets\visual-system\pipeline\render_crystal_3d_v12.py" -- `
  --frames 1 --samples 1 --no-encode --no-stills

& $blender --factory-startup -b -P "$repo\assets\visual-system\pipeline\render_crystal_3d_v12.py" -- `
  --frames 48 --samples 32 --ffmpeg $ffmpeg
```

Then validate the generated WebM and use the isolated fresh-set uploader;
never use the legacy uploader or an existing production set.
