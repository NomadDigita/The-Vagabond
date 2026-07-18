#!/usr/bin/env python3
"""
delete_stale_stickers.py — Clean up the duplicate-sticker mess in the
live vagabond_pilot_by_<bot> custom-emoji set (see
VAGABOND_VISUAL_SYSTEM_LOG.md §10 and the v8 checkpoint in §6).

WHY THIS EXISTS
-----------------------------------------
Early test runs of telegram_upload.py (before it supported
replaceStickerInSet) called addStickerToSet repeatedly, which doesn't
reject a re-add the way createNewStickerSet rejects a duplicate set
name. The live set has ended up with the 10 real, current, working
icons PLUS a batch of leftover static entries from those earlier
runs. This script finds and removes only the leftovers.

SAME NETWORK RESTRICTION AS telegram_upload.py
-----------------------------------------
This can't run inside the AI sandbox (no route to api.telegram.org).
Run it yourself, same env vars as telegram_upload.py:

    export TG_BOT_TOKEN="123456:AA...."
    export TG_OWNER_ID="123456789"

HOW IT DECIDES WHAT'S "STALE"
-----------------------------------------
It reads mapping.json (written by the last successful telegram_upload.py
run) to get the 10 known-good custom_emoji_id values — the ones that
are actually correct right now. Anything in the live set whose
custom_emoji_id is NOT in that list is a leftover and becomes a
deletion candidate. If mapping.json is missing, empty, or doesn't have
all 10 tracked icons, this script refuses to delete anything and tells
you to run telegram_upload.py first — deleting against incomplete
ground truth is exactly how you'd lose a real icon by mistake.

SAFE BY DEFAULT
-----------------------------------------
Running with no arguments is a DRY RUN: it prints exactly what it
would delete and does nothing else. Nothing is ever removed without
the explicit --confirm flag:

    python3 delete_stale_stickers.py            # dry run, always safe
    python3 delete_stale_stickers.py --confirm   # actually deletes

Uses deleteStickerFromSet (takes the sticker's file_id, not its
custom_emoji_id — Telegram's own inconsistency, handled here).
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
MAPPING_PATH = HERE.parent / "mapping.json"

EXPECTED_ICON_COUNT = 10  # the current pilot batch; bump this if/when it grows


def call(method, **kwargs):
    resp = requests.post(f"{API}/{method}", data=kwargs, timeout=30)
    data = resp.json()
    if not data.get("ok"):
        raise RuntimeError(f"{method} failed: {data}")
    return data["result"]


def main():
    confirm = "--confirm" in sys.argv

    if not MAPPING_PATH.exists():
        print(f"{MAPPING_PATH} doesn't exist — run telegram_upload.py first. "
              f"Refusing to guess at what's safe to delete.")
        sys.exit(1)

    with open(MAPPING_PATH) as f:
        mapping = json.load(f)

    if len(mapping) < EXPECTED_ICON_COUNT:
        print(f"mapping.json only has {len(mapping)} icons, expected "
              f"{EXPECTED_ICON_COUNT}. Refusing to delete against incomplete "
              f"ground truth — run telegram_upload.py first so mapping.json "
              f"is current, then re-run this script.")
        sys.exit(1)

    known_good_ids = set(mapping.values())
    print(f"Loaded {len(known_good_ids)} known-good icons from {MAPPING_PATH.name}:")
    for name, cid in mapping.items():
        print(f"  {name}: {cid}")

    me = call("getMe")
    set_name = f"vagabond_pilot_by_{me['username']}"
    sticker_set = call("getStickerSet", name=set_name)
    stickers = sticker_set["stickers"]
    print(f"\nLive set '{set_name}' has {len(stickers)} stickers total.")

    stale = [s for s in stickers if s.get("custom_emoji_id") not in known_good_ids]
    keep = [s for s in stickers if s.get("custom_emoji_id") in known_good_ids]

    print(f"  {len(keep)} match a known-good icon — keeping.")
    print(f"  {len(stale)} do NOT match any known-good icon — stale, "
          f"{'deleting now' if confirm else 'would delete'}:")
    for s in stale:
        fmt = s.get("format", "?")
        print(f"    file_id={s['file_id']}  format={fmt}  "
              f"custom_emoji_id={s.get('custom_emoji_id')}")

    if not stale:
        print("\nNothing to clean up — the live set already matches "
              "mapping.json exactly.")
        return

    if not confirm:
        print(f"\nDry run only — no changes made. Re-run with --confirm to "
              f"actually delete these {len(stale)} stale stickers.")
        return

    deleted, failed = 0, 0
    for s in stale:
        try:
            call("deleteStickerFromSet", sticker=s["file_id"])
            deleted += 1
        except RuntimeError as e:
            print(f"  FAILED to delete {s['file_id']}: {e}")
            failed += 1

    print(f"\nDeleted {deleted}/{len(stale)} stale stickers"
          f"{f', {failed} failed' if failed else ''}.")

    final_set = call("getStickerSet", name=set_name)
    print(f"Live set now has {len(final_set['stickers'])} stickers "
          f"(expected {EXPECTED_ICON_COUNT}).")


if __name__ == "__main__":
    main()
