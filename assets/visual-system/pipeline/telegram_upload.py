#!/usr/bin/env python3
"""
telegram_upload.py — Upload the pilot icon batch as a real Telegram
custom-emoji sticker set, and send yourself a live test message so you
can SEE how they actually render, at real inline size, on a real
client. This is the step that turns "art-complete" into "verified"
(see VAGABOND_VISUAL_SYSTEM_LOG.md §8).

WHY THIS CAN'T RUN INSIDE THE AI SANDBOX
-----------------------------------------
The session that wrote this script has an outbound network allow-list
that does not include api.telegram.org. This is a hard restriction,
not a permissions issue that a token can fix. Run this script from a
machine that CAN reach the real internet (your own laptop, a VPS,
GitHub Actions, etc).

WHAT YOU NEED BEFORE RUNNING THIS
-----------------------------------------
1. A bot token (BotFather -> your bot -> API Token). Treat it like a
   password: don't commit it, don't paste it in chat with anyone
   (including an AI) except to a throwaway/test bot you're prepared to
   revoke immediately after. Export it as an env var, never hardcode:

       export TG_BOT_TOKEN="123456:AA...."

2. Your own numeric Telegram user ID. Custom emoji sticker sets are
   owned by a *user*, not a bot, and that user must have started the
   bot at least once (open a DM with your bot and hit Start if you
   haven't). Get your numeric ID from @userinfobot (message it "/start"
   and it replies with your ID). Export it:

       export TG_OWNER_ID="123456789"

3. The 10 pilot PNGs already exported at 100x100 by build_icons.py —
   this script reads them straight from ../png/*_100.png relative to
   this file, so just run it from anywhere, it resolves its own path.

WHAT THIS SCRIPT DOES, IN ORDER
-----------------------------------------
1. Uploads each icon's 100x100 PNG via uploadStickerFile.
2. Creates ONE custom-emoji sticker set (createNewStickerSet,
   sticker_type="custom_emoji") containing all 10, owned by
   TG_OWNER_ID.
3. Writes assets/visual-system/mapping.json: {icon_name: custom_emoji_id}
   — this is the file internal/bot/emoji.go (not yet built, see log
   §5/§7) will eventually read from to swap literal Unicode emoji for
   these.
4. Sends TG_OWNER_ID a real message using those custom_emoji_id
   entities, one per line with the icon's name next to it, so you can
   visually confirm every icon at true inline size in your own
   Telegram client.
5. Prints a plain-language pass/fail summary. Does NOT touch any game
   code, table, or existing sticker set.

Re-running: safe. If the set already exists (same short_name), this
script adds any missing stickers to it rather than failing outright,
but will not delete or recreate what's already there.
"""

import os
import sys
import json
import pathlib

import requests

TOKEN = os.environ.get("TG_BOT_TOKEN")
OWNER_ID = os.environ.get("TG_OWNER_ID")

if not TOKEN or not OWNER_ID:
    print("Missing TG_BOT_TOKEN and/or TG_OWNER_ID environment variables.")
    print("See the module docstring at the top of this file for setup steps.")
    sys.exit(1)

API = f"https://api.telegram.org/bot{TOKEN}"
HERE = pathlib.Path(__file__).resolve().parent
PNG_DIR = HERE.parent / "png"
MAPPING_PATH = HERE.parent / "mapping.json"

# icon_name -> one or more standard emoji Telegram uses for search/suggestion
# (required by the API even for a fully custom set — pick the closest
# real emoji so the icon still surfaces when someone searches for it)
ICONS = {
    "warning":     "\u26A0",   # ⚠
    "failure":     "\u274C",   # ❌
    "shield":      "\U0001F6E1",  # 🛡
    "transport":   "\U0001F69A",  # 🚚
    "ai_mech":     "\U0001F916",  # 🤖
    "gear":        "\u2699",   # ⚙
    "satellite":   "\U0001F6F0",  # 🛰
    "combat":      "\u2694",   # ⚔
    "electricity": "\u26A1",   # ⚡
    "scrap":       "\U0001F529",  # 🔩
}

# Icons that have an animated WEBM (VP9, alpha, 100x100, <3s, looped) in
# assets/visual-system/animated/<name>/<name>.webm. Everything else in
# ICONS still falls back to the static PNG in png/. See
# VAGABOND_VISUAL_SYSTEM_LOG.md §10 for why WEBM was chosen over TGS/Lottie,
# and for the plan to animate the rest.
ANIMATED_DIR = HERE.parent / "animated"
ANIMATED_ICONS = {
    name for name in ICONS
    if (ANIMATED_DIR / name / f"{name}.webm").exists()
}

# Must end in _by_<bot_username> per Telegram's rules. Fetched dynamically
# below via getMe so this script doesn't need the bot username hardcoded.


def call(method, **kwargs):
    files = kwargs.pop("files", None)
    resp = requests.post(f"{API}/{method}", data=kwargs, files=files, timeout=30)
    data = resp.json()
    if not data.get("ok"):
        raise RuntimeError(f"{method} failed: {data}")
    return data["result"]


def utf16_len(s: str) -> int:
    """Telegram entity offset/length are counted in UTF-16 code units,
    not Python codepoints. Any character outside the Basic Multilingual
    Plane (most of our placeholder emoji, e.g. U+1F916) is 2 UTF-16
    units even though Python's len() counts it as 1 — that mismatch is
    exactly what caused the original 'ends in the middle of a UTF-16
    symbol' error. Always compute lengths through this helper when
    building message entities."""
    return len(s.encode("utf-16-le")) // 2


def send_verification_message(mapping):
    entities = []
    text_parts = []
    cursor = 0
    for name, cid in mapping.items():
        # Placeholder text under a custom_emoji entity should be the
        # underlying standard emoji it's replacing (matches Telegram's
        # own convention, and keeps this readable as plain text as a
        # fallback on any client that doesn't render custom emoji).
        piece = ICONS.get(name, "\u2753")  # fallback: question mark
        line = f"{piece} {name}\n"
        entities.append({
            "type": "custom_emoji",
            "offset": cursor,
            "length": utf16_len(piece),
            "custom_emoji_id": cid,
        })
        text_parts.append(line)
        cursor += utf16_len(line)

    text = "".join(text_parts)
    call("sendMessage", chat_id=OWNER_ID, text=text, entities=json.dumps(entities))
    print(f"Sent a live test message to Telegram user {OWNER_ID}.")
    print("Open that chat now and check every line at real size, in real Telegram.")
    print("If any icon looks wrong, fix the SVG, re-run build_icons.py, and re-run")
    print("this script — addStickerToSet will fail for duplicates, so bump the")
    print("icon's PNG content or delete it from the set first via deleteStickerFromSet.")


def main():
    # If a previous run already got through the upload/create-set phase
    # (mapping.json exists with all 10 icons), don't redo that — Telegram
    # will just bounce duplicate-sticker errors. Go straight to sending
    # the verification message so a fix to *that* step alone is a fast
    # re-run, not a full re-upload.
    if MAPPING_PATH.exists():
        with open(MAPPING_PATH) as f:
            existing = json.load(f)
        if set(existing.keys()) == set(ICONS.keys()):
            print(f"Found complete {MAPPING_PATH.name} from a previous run — "
                  f"skipping upload/create, sending verification message only.")
            send_verification_message(existing)
            return

    me = call("getMe")
    bot_username = me["username"]
    set_name = f"vagabond_pilot_by_{bot_username}"
    print(f"Bot: @{bot_username}  |  Sticker set short_name: {set_name}")

    mapping = {}
    stickers_payload = []

    for name, emoji in ICONS.items():
        png_path = PNG_DIR / f"{name}_100.png"
        if not png_path.exists():
            print(f"  SKIP {name}: {png_path} not found (run build_icons.py first)")
            continue
        print(f"  Uploading {name} ({png_path.name})...")
        with open(png_path, "rb") as f:
            uploaded = call(
                "uploadStickerFile",
                user_id=OWNER_ID,
                sticker_format="static",
                files={"sticker": f},
            )
        file_id = uploaded["file_id"]
        stickers_payload.append({"name": name, "file_id": file_id, "emoji": emoji})

    if not stickers_payload:
        print("Nothing uploaded — nothing to create a set from. Aborting.")
        sys.exit(1)

    # Try creating the set with the first sticker; add the rest after.
    first = stickers_payload[0]
    try:
        call(
            "createNewStickerSet",
            user_id=OWNER_ID,
            name=set_name,
            title="The Vagabond (Pilot)",
            sticker_type="custom_emoji",
            stickers=json.dumps([{
                "sticker": first["file_id"],
                "format": "static",
                "emoji_list": [first["emoji"]],
            }]),
        )
        print(f"Created sticker set {set_name} with {first['name']}.")
        remaining = stickers_payload[1:]
    except RuntimeError as e:
        if "already exists" in str(e) or "name is already occupied" in str(e):
            print(f"Set {set_name} already exists — will add any missing stickers to it.")
            remaining = stickers_payload
        else:
            raise

    for item in remaining:
        try:
            call(
                "addStickerToSet",
                user_id=OWNER_ID,
                name=set_name,
                sticker=json.dumps({
                    "sticker": item["file_id"],
                    "format": "static",
                    "emoji_list": [item["emoji"]],
                }),
            )
            print(f"  Added {item['name']} to set.")
        except RuntimeError as e:
            print(f"  WARNING: could not add {item['name']}: {e}")

    # Pull the finished set back to read the real custom_emoji_id per sticker.
    full_set = call("getStickerSet", name=set_name)
    # Telegram returns stickers in upload order; zip against our name list.
    for meta, sticker in zip(stickers_payload, full_set["stickers"]):
        mapping[meta["name"]] = sticker["custom_emoji_id"]

    with open(MAPPING_PATH, "w") as f:
        json.dump(mapping, f, indent=2, sort_keys=True)
    print(f"Wrote {MAPPING_PATH} ({len(mapping)} icons).")

    send_verification_message(mapping)


if __name__ == "__main__":
    main()
