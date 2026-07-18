# Blender 3D Emoji Pipeline

## Why this exists

The original animated pilot uses layered SVG geometry. It is an intentional,
valid fallback for compact Telegram media, but it cannot produce the
camera-relative reflections, occlusion, glass transmission, or real geometric
rotation seen in high-end 3D emoji references. This pipeline adds a separate
Blender-rendered asset tier without rewriting or replacing the proven pilot.

The v10 `oracle_3d_v10` is the first quality gate. It is an original Vagabond
survey-command core: liquid-glass containment, a rotating mechanical aperture,
gunmetal cradle, gold signal hardware, cyan/magenta dispersion rings, and
depth-separated sensor motes. It deliberately does not reproduce a Telegram
character, symbol, or collectible.

## Local tool contract

- Blender 4.5 LTS or newer: procedural mesh construction and RGBA frame render.
- FFmpeg with `libvpx-vp9`: silent VP9/alpha WebM encoding.
- Python 3.10+: offline file-contract validation.

No tool in this pipeline needs a bot token or calls Telegram. The upload step
remains separate and owner-controlled.

## Render

From the repository root, set these values to the local Blender and FFmpeg
executables, then run:

```powershell
$env:VAGABOND_BLENDER = 'C:\path\to\blender.exe'
$env:VAGABOND_FFMPEG = 'C:\path\to\ffmpeg.exe'
& $env:VAGABOND_BLENDER -b -P assets/visual-system/pipeline/render_oracle_3d_v10.py -- --ffmpeg $env:VAGABOND_FFMPEG
```

For an inexpensive lighting test, use `--frames 12 --no-encode`. The final
render has 48 frames at 24fps (2 seconds) and writes only these paths:

```text
assets/visual-system/source/oracle_3d_v10.blend
assets/visual-system/animated/oracle_3d_v10/frames/frame_*.png  (scratch)
assets/visual-system/animated/oracle_3d_v10/oracle_3d_v10.webm
assets/visual-system/renders/oracle_3d_v10/oracle_3d_v10_{100,128,256,512}.png
```

The `frames/` directory is scratch output and must stay ignored. Re-running the
renderer deletes only its own `frame_*.png` files.

## Validate before any upload

```powershell
python assets/visual-system/pipeline/validate_video_custom_emoji.py `
  assets/visual-system/animated/oracle_3d_v10/oracle_3d_v10.webm `
  --ffprobe C:\path\to\ffprobe.exe `
  --ffmpeg C:\path\to\ffmpeg.exe
```

The offline gate fails unless the asset is silent VP9, has a transparently
decoded alpha plane, is 100x100 pixels, is 3 seconds or less, and is 256KiB or
less. Passing that gate only means the file is technically ready for the
repository's Telegram upload path.
It does not claim that the asset was uploaded or viewed in a Telegram client.

## Art acceptance gates

1. At 512px: clear glass volume, metal occlusion, and intentional lighting.
2. At 100px: silhouette remains legible without in-icon text.
3. In motion: object rotation, aperture motion, and parallax are visibly tied
   to real geometry rather than a global opacity pulse.
4. On transparent pixels: no black matte or rectangular background.
5. In Telegram: owner verifies true inline rendering after a controlled upload.
