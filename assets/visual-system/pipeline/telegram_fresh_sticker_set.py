#!/usr/bin/env python3
"""Create a new isolated Telegram *regular sticker* test pack safely.

Default mode only prints the intended operation.  --apply creates exactly one
new pack in one request; it never appends, replaces, or deletes a sticker.
"""

from __future__ import annotations

import argparse
import getpass
import hashlib
import json
import os
import re
from datetime import UTC, datetime
from pathlib import Path

import requests

from validate_telegram_regular_sticker import validate_static, validate_tgs, validate_video

HERE = Path(__file__).resolve().parent
ASSET_ROOT = HERE.parent
RESULTS = ASSET_ROOT / "test-results"
TOKEN = re.compile(r"\b\d{6,}:[A-Za-z0-9_-]{20,}\b")


def redact(value: object) -> str:
    return TOKEN.sub("<redacted-bot-token>", str(value))


def digest(path: Path) -> str:
    hasher = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def check_slug(value: str) -> str:
    if not re.fullmatch(r"[A-Za-z][A-Za-z0-9_]{1,30}", value) or "__" in value or value.endswith("_"):
        raise SystemExit("--set-slug must start with a letter, be 2-31 letters/digits/underscores, and not end in or repeat underscores.")
    return value.lower()


def asset_format(path: Path) -> tuple[str, str]:
    suffix = path.suffix.lower()
    formats = {".tgs": ("animated", "application/x-tgsticker"), ".webm": ("video", "video/webm"), ".webp": ("static", "image/webp"), ".png": ("static", "image/png")}
    if suffix not in formats:
        raise SystemExit("--asset must be .tgs, .webm, .webp, or .png.")
    return formats[suffix]


def validate_local_asset(path: Path, format_name: str) -> None:
    """Reject malformed media before credentials or network access."""
    if format_name == "animated":
        validate_tgs(path)
    elif format_name == "static":
        validate_static(path)
    else:
        validate_video(path, os.environ.get("FFPROBE", "ffprobe"))


def call(api: str, method: str, *, data: dict | None = None, files: dict | None = None) -> dict:
    try:
        response = requests.post(f"{api}/{method}", data=data, files=files, timeout=(8, 60))
        payload = response.json()
    except (requests.RequestException, ValueError) as error:
        raise RuntimeError(f"{method} transport failure: {type(error).__name__}.") from None
    if not isinstance(payload, dict) or not payload.get("ok"):
        description = redact(payload.get("description", "unspecified Telegram error")) if isinstance(payload, dict) else "non-object response"
        raise RuntimeError(f"{method} rejected by Telegram: {description}")
    return payload["result"]


def journal(name: str, asset: Path, status: str) -> Path:
    RESULTS.mkdir(parents=True, exist_ok=True)
    output = RESULTS / f"{name}.regular-sticker.intent.json"
    output.write_text(json.dumps({"schema_version": 1, "purpose": "isolated regular-sticker test; token-free", "updated_at": datetime.now(UTC).isoformat(), "status": status, "sticker_set": name, "asset": {"source": str(asset), "sha256": digest(asset)}}, indent=2) + "\\n", encoding="utf-8")
    return output


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--asset", type=Path, required=True, help="Owned local sticker asset (.tgs, .webm, .webp, or .png).")
    parser.add_argument("--set-slug", required=True, help="Fresh set prefix; _by_<bot_username> is added automatically.")
    parser.add_argument("--title", required=True, help="Regular sticker-pack title (1-64 characters).")
    parser.add_argument("--emoji", default="\u2728", help="One ordinary emoji associated with this sticker.")
    parser.add_argument("--owner-id", help="Numeric owner/test-account ID; overrides TG_OWNER_ID.")
    parser.add_argument("--prompt-token", action="store_true", help="Prompt for the BotFather token without echoing it.")
    parser.add_argument("--apply", action="store_true", help="Create the new pack. Default is safe plan mode.")
    args = parser.parse_args()
    asset = args.asset.expanduser().resolve()
    if not asset.is_file():
        raise SystemExit(f"Missing --asset: {asset}")
    slug = check_slug(args.set_slug)
    if not args.title.strip() or len(args.title) > 64:
        raise SystemExit("--title must contain 1-64 characters.")
    if not args.emoji.strip() or len(args.emoji.encode("utf-16-le")) // 2 > 20:
        raise SystemExit("--emoji must be a non-empty short ordinary emoji string.")
    format_name, mime = asset_format(asset)
    validate_local_asset(asset, format_name)
    print(f"Asset: {asset.name} ({format_name}, {asset.stat().st_size / 1024:.1f}KiB)")
    print(f"SHA-256: {digest(asset)}")
    if not args.apply:
        print("DRY RUN: no Telegram request was made.")
        print(f"Would create only a NEW regular sticker set named `{slug}_by_<bot_username>`.")
        print("Telegram rejects an existing name; this tool never appends, replaces, or deletes a set.")
        return
    token = getpass.getpass("Paste BotFather token locally (input hidden): " ) if args.prompt_token else ""
    owner = (args.owner_id or os.environ.get("TG_OWNER_ID", "")).strip()
    if not token.strip() or not re.fullmatch(r"\d{6,}:[A-Za-z0-9_-]{20,}", token.strip()):
        raise SystemExit("--apply requires a valid token through --prompt-token; tokens are never stored.")
    if not owner.isdecimal():
        raise SystemExit("--apply requires numeric --owner-id.")
    api = f"https://api.telegram.org/bot{token.strip()}"
    bot = call(api, "getMe")
    username = bot.get("username")
    if not isinstance(username, str) or not username:
        raise SystemExit("The bot needs a BotFather username before it can create a sticker pack.")
    name = f"{slug}_by_{username}"
    if len(name) > 64:
        raise SystemExit(f"Generated set name is too long: {name}")
    record = journal(name, asset, "CREATE_PENDING")
    sticker = {"sticker": "attach://sticker_file", "format": format_name, "emoji_list": [args.emoji]}
    print(f"Creating fresh regular sticker set: {name}")
    try:
        with asset.open("rb") as source:
            call(api, "createNewStickerSet", data={"user_id": owner, "name": name, "title": args.title, "sticker_type": "regular", "stickers": json.dumps([sticker])}, files={"sticker_file": (asset.name, source, mime)})
    except RuntimeError as error:
        journal(name, asset, "CREATE_REJECTED_OR_UNKNOWN")
        raise SystemExit(f"Fresh-set creation did not complete: {error}\\nJournal: {record}\\nDo not reuse this slug unless you independently confirm no set was created.")
    journal(name, asset, "CREATED")
    sticker_set = call(api, "getStickerSet", data={"name": name})
    stickers = sticker_set.get("stickers", [])
    if sticker_set.get("sticker_type") != "regular" or len(stickers) != 1:
        raise SystemExit("Fresh pack was created but readback did not match the expected one-sticker regular set. Do not recreate it.")
    RESULTS.mkdir(parents=True, exist_ok=True)
    manifest = RESULTS / f"{name}.regular-sticker.json"
    manifest.write_text(json.dumps({"schema_version": 1, "purpose": "fresh regular-sticker test; not production approval", "sticker_set": name, "sticker_set_url": f"https://t.me/addstickers/{name}", "asset": {"source": str(asset), "sha256": digest(asset), "file_id": stickers[0].get("file_id")}, "verification": {"telegram_readback": "pass", "client_review": "PENDING_OWNER_REVIEW"}}, indent=2) + "\\n", encoding="utf-8")
    print(f"Verified fresh regular sticker set: {name}")
    print(f"Pack link: https://t.me/addstickers/{name}")
    print(f"Manifest: {manifest}")


if __name__ == "__main__":
    main()
