# ADR 0012: Decisions from `assets/visual-system/pipeline/render_oracle_3d_v10.py`

Status: Accepted

These decisions were written as comments in `assets/visual-system/pipeline/render_oracle_3d_v10.py` and moved here so the source no longer carries them. Each entry is the original comment.

## Dithered blending adds visible grain at a 100px canvas. Blended

Removed from `assets/visual-system/pipeline/render_oracle_3d_v10.py:100`.

~~~~text
Dithered blending adds visible grain at a 100px canvas. Blended
transmission preserves the intentionally smooth liquid-glass read.
~~~~


## EEVEE's sample setting moved between Blender releases; use the current

Removed from `assets/visual-system/pipeline/render_oracle_3d_v10.py:226`.

~~~~text
EEVEE's sample setting moved between Blender releases; use the current
property when it exists and remain compatible with older LTS builds.
~~~~


## Glare affects colour but must never turn the transparent film into an

Removed from `assets/visual-system/pipeline/render_oracle_3d_v10.py:249`.

~~~~text
Glare affects colour but must never turn the transparent film into an
opaque black square when the PNG sequence is encoded to VP9 alpha.
~~~~


## A unique Vagabond object: a mechanical survey core, not an eye, crystal

Removed from `assets/visual-system/pipeline/render_oracle_3d_v10.py:269`.

~~~~text
A unique Vagabond object: a mechanical survey core, not an eye, crystal
ball, or copy of a Telegram asset. The iris is an aperture scanner.
~~~~


## Purposeful debris: depth and parallax, not generic sparkle confetti.

Removed from `assets/visual-system/pipeline/render_oracle_3d_v10.py:326`.

~~~~text
Purposeful debris: depth and parallax, not generic sparkle confetti.
~~~~


## A custom emoji is read at roughly 20px. A full turn makes this narrow

Removed from `assets/visual-system/pipeline/render_oracle_3d_v10.py:334`.

~~~~text
A custom emoji is read at roughly 20px. A full turn makes this narrow
silhouette disappear edge-on, so the heavy command core has a deliberate
glassy yaw/bob while the aperture performs the visibly full rotation.
The matching start/end pose makes the two-second loop seamless.
~~~~
