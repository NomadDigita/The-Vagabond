# ADR 0005: Decisions from `assets/visual-system/pipeline/animate_transparent_still.py`

Status: Accepted

These decisions were written as comments in `assets/visual-system/pipeline/animate_transparent_still.py` and moved here so the source no longer carries them. Each entry is the original comment.

## The 108px work canvas prevents transparent corners from being cropped

Removed from `assets/visual-system/pipeline/animate_transparent_still.py:58`.

~~~~text
The 108px work canvas prevents transparent corners from being cropped
while the source gently rocks.  `black@0` is deliberate: it preserves
alpha outside the original still instead of baking a dark background.
sin() starts and ends at zero over the two-second default, so Telegram's
loop point has no rotational jump.
+/- 3 degrees
~~~~
