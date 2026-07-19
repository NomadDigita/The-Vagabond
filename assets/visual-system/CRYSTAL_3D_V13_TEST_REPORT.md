# Crystal 3D v13 - Motion Rebuild and Test Record

## Why v13 exists

The owner created and reviewed the isolated `crystal_3d_v12` Telegram set.
Telegram accepted it and assigned a custom emoji ID, proving the upload path.
At actual client size, however, v12's whole-object drift softened the image and
did not create the promised living-crystal effect. It is not the approved
motion standard.

## Rebuild

V13 keeps the recognizable violet glass sphere and gold Vagabond pedestal but
uses separate motion layers:

- Stable glass sphere and pedestal preserve the silhouette.
- The faceted heart completes a visible edge-on turn once per two-second loop.
- Heart luminance changes with rotation, so it reads as a refractive gemstone.
- Three star glints pulse independently; no generic global fade is used.
- A faint glass reflection veil is composited over the heart to place it inside
  the sphere rather than on its surface.

The sources are produced through Codex's built-in image tool on a dedicated
chroma background and keyed locally. `render_crystal_3d_v13.py` is a local
Pillow plus FFmpeg compositor; it has no API call, token, or Blender dependency.

## Local QA

- Python syntax: PASS.
- Front-facing and edge-on frames reviewed at final 100x100 size.
- Animated output: `animated/crystal_3d_v13/crystal_3d_v13.webm`.
- Validator: PASS - VP9, decoded alpha, 100x100, silent, 2.000 seconds.
- Delivered size: 9.0 KiB, under the project's conservative 64 KiB gate.
- Telegram: not uploaded yet. Use a brand-new fresh test slug; do not alter the
  v12 set until this version is seen in the client.

## Upload

```powershell
$repo = 'C:\Users\PC\Documents\Codex\2026-07-18\nomaddigita-the-vagabond-https-github-com\work\The-Vagabond'
Set-Location $repo
python assets\visual-system\pipeline\telegram_fresh_test_set.py `
  --apply --prompt-token --owner-id 6582793388 `
  --set-slug vagabond_crystal_v13_motion_test `
  --asset "$repo\assets\visual-system\animated\crystal_3d_v13\crystal_3d_v13.webm" `
  --asset-key crystal_3d_v13 `
  --emoji 🔮 `
  --title "The Vagabond Crystal v13 Motion Test"
```
