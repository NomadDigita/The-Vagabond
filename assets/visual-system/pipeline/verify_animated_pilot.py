#!/usr/bin/env python3
"""
verify_animated_pilot.py — Upload ONE animated (WEBM/VP9/alpha) custom
emoji into a fresh, separate sticker set and send a live test message,
so we can confirm animated custom emoji actually work end-to-end
before touching the main 10-icon `vagabond_pilot_by_<bot>` set (which
already has some duplicate static entries from an earlier double-run —
see VAGABOND_VISUAL_SYSTEM_LOG.md §5/§10 — no need to fight that mess
just to answer "does animation work at all").

Env vars required (same as telegram_upload.py):
    TG_BOT_TOKEN, TG_OWNER_ID

Usage:
    python3 verify_animated_pilot.py
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
    sys.exit(1)

API = f"https://api.telegram.org/bot{TOKEN}"
HERE = pathlib.Path(__file__).resolve().parent
WEBM_PATH = HERE.parent / "animated" / "electricity" / "electricity.webm"


def call(method, **kwargs):
    files = kwargs.pop("files", None)
    resp = requests.post(f"{API}/{method}", data=kwargs, files=files, timeout=30)
    data = resp.json()
    if not data.get("ok"):
        raise RuntimeError(f"{method} failed: {data}")
    return data["result"]


def utf16_len(s: str) -> int:
    return len(s.encode("utf-16-le")) // 2


def main():
    if not WEBM_PATH.exists():
        print(f"Missing {WEBM_PATH} — run pipeline/animate_electricity.py "
              f"and the ffmpeg encode step first.")
        sys.exit(1)

    me = call("getMe")
    bot_username = me["username"]
    set_name = f"vagabond_animtest_by_{bot_username}"
    print(f"Bot: @{bot_username}  |  Test set: {set_name}")

    print("Uploading electricity.webm as a video sticker...")
    with open(WEBM_PATH, "rb") as f:
        uploaded = call(
            "uploadStickerFile",
            user_id=OWNER_ID,
            sticker_format="video",
            files={"sticker": f},
        )
    file_id = uploaded["file_id"]

    try:
        call(
            "createNewStickerSet",
            user_id=OWNER_ID,
            name=set_name,
            title="Vagabond Animated Test",
            sticker_type="custom_emoji",
            stickers=json.dumps([{
                "sticker": file_id,
                "format": "video",
                "emoji_list": ["\u26A1"],
            }]),
        )
        print(f"Created {set_name}.")
    except RuntimeError as e:
        if "already exists" in str(e) or "occupied" in str(e):
            print(f"{set_name} already exists from a previous run — reusing it.")
        else:
            raise

    full_set = call("getStickerSet", name=set_name)
    cid = full_set["stickers"][-1]["custom_emoji_id"]
    print(f"custom_emoji_id: {cid}")

    piece = "\u26A1"
    text = f"{piece} animated electricity — does this move?\n"
    entities = [{
        "type": "custom_emoji",
        "offset": 0,
        "length": utf16_len(piece),
        "custom_emoji_id": cid,
    }]
    call("sendMessage", chat_id=OWNER_ID, text=text, entities=json.dumps(entities))
    print("Sent. Check your Telegram chat with the bot now — that emoji should")
    print("visibly animate (pulsing glow, traveling light streak, sparkles).")
    print("If it's still static: Telegram client caching can briefly hold the")
    print("old render — try a different chat, or restart the Telegram app.")


if __name__ == "__main__":
    main()
