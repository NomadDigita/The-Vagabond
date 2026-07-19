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

1. The initial EEVEE renderer was isolated as the failure point: it stalls in
   this headless Windows Blender build even for a 100px opaque sphere.
2. The Blender scene now uses Cycles CPU, which rendered a transparent
   100x100 probe successfully at eight samples (2.30 seconds render time).
3. The approved crystal design was generated through Codex's built-in image
   tool on a flat chroma-key background, then locally extracted to an RGBA PNG.
   Visual inspection confirms transparent surroundings and no dark square.
4. `animate_transparent_still.py` creates the final WebM with a seamless,
   reversible three-degree crystal drift. It uses local FFmpeg only.
5. `animated/crystal_3d_v12/crystal_3d_v12.webm`: **PASS** local validation -
   VP9, decoded alpha, 100x100, silent, 2.000 seconds, 7.7 KiB, under the
   conservative 64 KiB delivery gate.
6. Telegram fresh-set creation and actual client review: **PASS for delivery,
   not approved for visual quality.** Telegram created the isolated v12 set and
   assigned a custom emoji ID; the owner reported that the whole-object drift
   was too soft and lacked independent heart/star motion. V13 supersedes v12
   for visual testing.

## Re-run command

Use the repository's portable Blender and ffmpeg paths. Cycles CPU is the
reliable local 3D renderer; do not switch this asset back to EEVEE.

```powershell
$repo = 'C:\Users\PC\Documents\Codex\2026-07-18\nomaddigita-the-vagabond-https-github-com\work\The-Vagabond'
$blender = 'C:\Users\PC\Documents\Codex\2026-07-18\nomaddigita-the-vagabond-https-github-com\work\toolcache\blender-4.5.3\blender.exe'
$ffmpeg = 'C:\Users\PC\Documents\Codex\2026-07-18\nomaddigita-the-vagabond-https-github-com\work\toolcache\ffmpeg\bin\ffmpeg.exe'

& $blender --factory-startup -b -P "$repo\assets\visual-system\pipeline\render_crystal_3d_v12.py" -- `
  --frames 1 --samples 8 --no-encode --no-stills

python "$repo\assets\visual-system\pipeline\animate_transparent_still.py" `
  "$repo\assets\visual-system\source\crystal_3d_v12_alpha.png" `
  --output "$repo\assets\visual-system\animated\crystal_3d_v12\crystal_3d_v12.webm" `
  --ffmpeg $ffmpeg
```

Then validate the generated WebM and use the isolated fresh-set uploader;
never use the legacy uploader or an existing production set.
