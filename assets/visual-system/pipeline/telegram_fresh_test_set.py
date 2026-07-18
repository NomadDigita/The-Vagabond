#!/usr/bin/env python3
"""Create a fresh, deterministic Telegram test set for the Blender v10 asset.

This is dry-run by default and will never replace, append to, or infer
identities in an existing set. Legacy upload tooling used positional matching
against a duplicate-filled sticker set, which cannot safely recover an icon
name -> custom_emoji_id mapping.

Required only with --apply:
    TG_BOT_TOKEN  BotFather token, set locally and never committed.
    TG_OWNER_ID   Numeric Telegram ID of the Premium owner/test account.

Examples:
    python assets/visual-system/pipeline/telegram_fresh_test_set.py
    python assets/visual-system/pipeline/telegram_fresh_test_set.py \
      --apply --set-slug vagabond_v10_oracle_test
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
from datetime import UTC, datetime
from pathlib import Path

import requests


HERE = Path(__file__).resolve().parent
ASSET_ROOT = HERE.parent
MANIFEST_ROOT = ASSET_ROOT / "test-results"
ASSET = {
    "key": "oracle_3d_v10",
    "emoji": "🔮",
    "format": "video",
    "path": ASSET_ROOT / "animated" / "oracle_3d_v10" / "oracle_3d_v10.webm",
}


class TelegramAPIError(RuntimeError):
    def __init__(self, method: str, payload: dict):
        super().__init__(f"{method} failed: {payload}")
        self.error_code = payload.get("error_code")
        self.description = payload.get("description", "")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="Perform Telegram API calls; default is dry-run.")
    parser.add_argument("--set-slug", default="vagabond_v10_oracle_test", help="Fresh set prefix; bot suffix is appended automatically.")
    return parser.parse_args()


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def utf16_len(text: str) -> int:
    return len(text.encode("utf-16-le")) // 2


def validate_slug(slug: str) -> str:
    if not re.fullmatch(r"[A-Za-z][A-Za-z0-9_]{1,30}", slug):
        raise SystemExit("--set-slug must start with a letter and contain only letters, digits, and underscores (2-31 characters).")
    return slug.lower()


def call(api: str, method: str, **kwargs):
    files = kwargs.pop("files", None)
    try:
        response = requests.post(f"{api}/{method}", data=kwargs, files=files, timeout=45)
        response.raise_for_status()
        payload = response.json()
    except (requests.RequestException, ValueError) as error:
        raise RuntimeError(f"{method} network/response failure: {error}") from error
    if not payload.get("ok"):
        raise TelegramAPIError(method, payload)
    return payload["result"]


def get_set_if_missing_only(api: str, name: str):
    try:
        return call(api, "getStickerSet", name=name)
    except TelegramAPIError as error:
        description = error.description.upper()
        if error.error_code == 400 and ("STICKERSET_INVALID" in description or "NOT FOUND" in description):
            return None
        # Network/auth/API errors must never be treated as a missing set.
        raise


def upload_file(api: str, owner_id: str) -> str:
    with ASSET["path"].open("rb") as source:
        uploaded = call(api, "uploadStickerFile", user_id=owner_id, sticker_format=ASSET["format"], files={"sticker": source})
    return uploaded["file_id"]


def sticker_input(file_id: str) -> dict:
    return {"sticker": file_id, "format": ASSET["format"], "emoji_list": [ASSET["emoji"]]}


def send_verification_message(api: str, owner_id: str, custom_emoji_id: str) -> None:
    text = f"{ASSET['emoji']} {ASSET['key']} — inspect at inline size and long-press scale.\n"
    entity = {"type": "custom_emoji", "offset": 0, "length": utf16_len(ASSET["emoji"]), "custom_emoji_id": custom_emoji_id}
    call(api, "sendMessage", chat_id=owner_id, text=text, entities=json.dumps([entity]))


def write_manifest(set_name: str, sticker: dict, upload_file_id: str) -> Path:
    MANIFEST_ROOT.mkdir(parents=True, exist_ok=True)
    manifest_path = MANIFEST_ROOT / f"{set_name}.json"
    manifest = {
        "schema_version": 1,
        "purpose": "fresh isolated Telegram visual test; not production mapping",
        "created_at": datetime.now(UTC).isoformat(),
        "sticker_set": set_name,
        "asset": {
            "key": ASSET["key"],
            "source": str(ASSET["path"].relative_to(ASSET_ROOT.parent.parent)),
            "sha256": sha256(ASSET["path"]),
            "telegram_upload_file_id": upload_file_id,
            "custom_emoji_id": sticker["custom_emoji_id"],
        },
        "verification": {"local_media_contract": "pass", "telegram_client": "PENDING_OWNER_REVIEW"},
    }
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    return manifest_path


def main() -> None:
    args = parse_args()
    slug = validate_slug(args.set_slug)
    source = ASSET["path"]
    if not source.is_file():
        raise SystemExit(f"Missing source asset: {source}")
    print(f"Asset: {ASSET['key']} ({ASSET['format']}, {source.name}, {source.stat().st_size / 1024:.1f}KiB)")
    print(f"SHA-256: {sha256(source)}")
    if not args.apply:
        print("DRY RUN: no Telegram request was made.")
        print(f"Would create only a NEW set named `{slug}_by_<bot_username>`.")
        print("The apply run aborts if the exact set already exists; it never replaces or appends to a set.")
        return

    token = os.environ.get("TG_BOT_TOKEN")
    owner_id = os.environ.get("TG_OWNER_ID")
    if not token or not owner_id:
        raise SystemExit("--apply requires TG_BOT_TOKEN and TG_OWNER_ID to be set locally.")
    api = f"https://api.telegram.org/bot{token}"
    bot = call(api, "getMe")
    set_name = f"{slug}_by_{bot['username']}"
    if len(set_name) > 64:
        raise SystemExit(f"Fresh set name is too long ({len(set_name)} > 64): {set_name}")
    if get_set_if_missing_only(api, set_name) is not None:
        raise SystemExit(f"Refusing to touch existing set `{set_name}`. Choose a new --set-slug.")

    file_id = upload_file(api, owner_id)
    call(api, "createNewStickerSet", user_id=owner_id, name=set_name, title="The Vagabond — v10 Oracle 3D Test", sticker_type="custom_emoji", stickers=json.dumps([sticker_input(file_id)]))
    created = call(api, "getStickerSet", name=set_name)
    stickers = created.get("stickers", [])
    if len(stickers) != 1:
        raise RuntimeError(f"Fresh set assertion failed: expected one sticker, found {len(stickers)}. No mapping was written.")
    sticker = stickers[0]
    manifest = write_manifest(set_name, sticker, file_id)
    send_verification_message(api, owner_id, sticker["custom_emoji_id"])
    print(f"Created fresh test set: {set_name}")
    print(f"custom_emoji_id: {sticker['custom_emoji_id']}")
    print(f"Wrote deterministic test manifest: {manifest}")
    print("Telegram client review is now required; this is not production approval.")


if __name__ == "__main__":
    main()
