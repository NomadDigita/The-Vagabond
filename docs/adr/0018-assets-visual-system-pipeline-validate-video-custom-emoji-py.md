# ADR 0018: Decisions from `assets/visual-system/pipeline/validate_video_custom_emoji.py`

Status: Accepted

These decisions were written as comments in `assets/visual-system/pipeline/validate_video_custom_emoji.py` and moved here so the source no longer carries them. Each entry is the original comment.

## Telegram's public video-emoji page states 256KiB, but the live Bot API

Removed from `assets/visual-system/pipeline/validate_video_custom_emoji.py:24`.

~~~~text
Telegram's public video-emoji page states 256KiB, but the live Bot API
rejected the 71.7KiB v10 Oracle as "file is too big" during custom-emoji set
creation on 2026-07-19. Keep a conservative 64KiB delivery gate until the
discrepancy is resolved upstream; callers can explicitly override it for
diagnostic work.
~~~~


## ffprobe may describe VP9-alpha as yuv420p while exposing alpha_mode=1 in

Removed from `assets/visual-system/pipeline/validate_video_custom_emoji.py:81`.

~~~~text
ffprobe may describe VP9-alpha as yuv420p while exposing alpha_mode=1 in
stream tags. Decode an RGBA frame instead of trusting either label.
~~~~
