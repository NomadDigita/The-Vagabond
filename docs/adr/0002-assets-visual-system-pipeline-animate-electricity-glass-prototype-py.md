# ADR 0002: Decisions from `assets/visual-system/pipeline/animate_electricity_glass_prototype.py`

Status: Accepted

These decisions were written as comments in `assets/visual-system/pipeline/animate_electricity_glass_prototype.py` and moved here so the source no longer carries them. Each entry is the original comment.

## Refraction band: sweeps diagonally across the bolt once per loop.

Removed from `assets/visual-system/pipeline/animate_electricity_glass_prototype.py:125`.

~~~~text
Refraction band: sweeps diagonally across the bolt once per loop.
Travels well outside the 0-100 viewbox on either side so it enters/
exits cleanly rather than popping in/out abruptly.
~~~~
