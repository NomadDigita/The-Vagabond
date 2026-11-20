# ADR 0015: Decisions from `assets/visual-system/pipeline/telegram_upload.py`

Status: Accepted

These decisions were written as comments in `assets/visual-system/pipeline/telegram_upload.py` and moved here so the source no longer carries them. Each entry is the original comment.

## icon_name -> one or more standard emoji Telegram uses for search/suggestion

Removed from `assets/visual-system/pipeline/telegram_upload.py:85`.

~~~~text
icon_name -> one or more standard emoji Telegram uses for search/suggestion
(required by the API even for a fully custom set — pick the closest
real emoji so the icon still surfaces when someone searches for it)
~~~~


## ⚠

Removed from `assets/visual-system/pipeline/telegram_upload.py:89`.

~~~~text
⚠
❌
🛡
🚚
🤖
⚙
🛰
⚔
⚡
🔩
🔮 (original design, not a copy — see animate_oracle.py's header)
~~~~


## Icons that have an animated WEBM (VP9, alpha, 100x100, <3s, looped) in

Removed from `assets/visual-system/pipeline/telegram_upload.py:102`.

~~~~text
Icons that have an animated WEBM (VP9, alpha, 100x100, <3s, looped) in
assets/visual-system/animated/<name>/<name>.webm. Everything else in
ICONS still falls back to the static PNG in png/. See
VAGABOND_VISUAL_SYSTEM_LOG.md §10 for why WEBM was chosen over TGS/Lottie,
and for the plan to animate the rest.
~~~~


## Must end in _by_<bot_username> per Telegram's rules. Fetched dynamically

Removed from `assets/visual-system/pipeline/telegram_upload.py:113`.

~~~~text
Must end in _by_<bot_username> per Telegram's rules. Fetched dynamically
below via getMe so this script doesn't need the bot username hardcoded.
~~~~


## Placeholder text under a custom_emoji entity should be the

Removed from `assets/visual-system/pipeline/telegram_upload.py:142`.

~~~~text
Placeholder text under a custom_emoji entity should be the
underlying standard emoji it's replacing (matches Telegram's
own convention, and keeps this readable as plain text as a
fallback on any client that doesn't render custom emoji).
fallback: question mark
~~~~


## Does the set already exist? If so, fetch it so we can replace

Removed from `assets/visual-system/pipeline/telegram_upload.py:202`.

~~~~text
Does the set already exist? If so, fetch it so we can replace
(not duplicate) any icon that's already there — this is the fix
for the "known mess" in VAGABOND_VISUAL_SYSTEM_LOG.md §10: an
earlier double-run used addStickerToSet, which doesn't reject
repeats the way createNewStickerSet rejects a repeat set name.
~~~~


## Telegram's custom-emoji stickers don't carry our icon name back,

Removed from `assets/visual-system/pipeline/telegram_upload.py:209`.

~~~~text
Telegram's custom-emoji stickers don't carry our icon name back,
so we rely on upload order matching ICONS order — same
assumption mapping.json already makes. If mapping.json is
stale/missing, fall back to treating the set as empty (adds
only; won't dedupe, but also won't crash).
~~~~
