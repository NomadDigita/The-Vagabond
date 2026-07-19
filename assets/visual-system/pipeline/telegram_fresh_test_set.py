#!/usr/bin/env python3
"""Create or safely recover an isolated Telegram test set for any owned asset.

The normal apply path performs one direct multipart create request. It never
uses add, replace, or delete operations, and it never probes an existing set
before creation. A set-name collision is rejected by Telegram without changing
the existing set. If the create response is lost, the script records an intent
locally and offers a read-only recovery command instead of retrying creation.
"""

from __future__ import annotations

import argparse
import getpass
import hashlib
import json
import os
import re
import time
from datetime import UTC, datetime
from pathlib import Path

import requests


HERE = Path(__file__).resolve().parent
ASSET_ROOT = HERE.parent
MANIFEST_ROOT = ASSET_ROOT / "test-results"
HTTP = requests.Session()
READ_ONLY_METHODS = frozenset({"getMe", "getStickerSet"})
READ_RETRY_ATTEMPTS = 4
ASSET = {
    "key": "oracle_3d_v10",
    "emoji": "\U0001F52E",
    "format": "video",
    "path": ASSET_ROOT / "animated" / "oracle_3d_v10" / "oracle_3d_v10.webm",
    "title": "The Vagabond \u2014 v10 Oracle 3D Test",
}
TOKEN_PATTERN = re.compile(r"\b\d{6,}:[A-Za-z0-9_-]{20,}\b")


def redact_tokens(value: object) -> str:
    return TOKEN_PATTERN.sub("<redacted-bot-token>", str(value))


class TelegramAPIError(RuntimeError):
    def __init__(self, method: str, payload: dict):
        self.error_code = payload.get("error_code")
        self.description = redact_tokens(payload.get("description", "Telegram returned an unspecified API error."))
        super().__init__(f"{method} failed (HTTP {self.error_code}): {self.description}")


class TelegramTransportError(RuntimeError):
    """A token-safe failure with no usable Bot API response."""

    def __init__(self, method: str, failure_kind: str, attempts: int):
        super().__init__(f"{method} transport failure ({failure_kind}) after {attempts} attempt(s).")
        self.method = method
        self.failure_kind = failure_kind
        self.attempts = attempts


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="Perform Telegram API calls; default is dry-run.")
    parser.add_argument("--set-slug", default="vagabond_v10_oracle_test", help="Fresh set prefix; bot suffix is appended automatically.")
    parser.add_argument("--prompt-token", action="store_true", help="Prompt for the bot token without echoing or storing it in an environment variable.")
    parser.add_argument("--owner-id", help="Numeric Telegram owner/test-account ID; overrides TG_OWNER_ID for this run only.")
    parser.add_argument("--recover-set", help="Read, verify, and message an already-created test set without creating or changing any set.")
    parser.add_argument("--asset", type=Path, help="Owned 100x100 VP9-alpha WebM to upload; defaults to the local Oracle proof asset.")
    parser.add_argument("--asset-key", help="Readable manifest key for --asset, for example vagabond_crystal_v1.")
    parser.add_argument("--emoji", help="Unicode fallback emoji associated with the uploaded custom emoji; defaults to the Oracle fallback.")
    parser.add_argument("--title", help="Fresh test-set title; defaults to the Oracle proof title.")
    return parser.parse_args()


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def configure_asset(args: argparse.Namespace) -> None:
    path_argument = getattr(args, "asset", None)
    if path_argument:
        source = Path(path_argument).expanduser().resolve()
        if source.suffix.lower() != ".webm":
            raise SystemExit("--asset must be a .webm file. Convert your owned source to Telegram's VP9-alpha WebM contract before uploading.")
        if not source.is_file():
            raise SystemExit(f"Missing --asset: {source}")
        ASSET["path"] = source
    key = getattr(args, "asset_key", None)
    if key:
        if not re.fullmatch(r"[A-Za-z][A-Za-z0-9_-]{1,63}", key):
            raise SystemExit("--asset-key must start with a letter and contain only letters, digits, underscores, or hyphens.")
        ASSET["key"] = key
    emoji = getattr(args, "emoji", None)
    if emoji:
        if not emoji.strip() or utf16_len(emoji) > 20:
            raise SystemExit("--emoji must be a non-empty short fallback emoji string.")
        ASSET["emoji"] = emoji
    title = getattr(args, "title", None)
    if title:
        if len(title) > 64:
            raise SystemExit("--title must be 64 characters or fewer.")
        ASSET["title"] = title


def asset_source_label() -> str:
    try:
        return str(ASSET["path"].relative_to(ASSET_ROOT.parent.parent))
    except ValueError:
        return str(ASSET["path"])


def utf16_len(text: str) -> int:
    return len(text.encode("utf-16-le")) // 2


def validate_slug(slug: str) -> str:
    if not re.fullmatch(r"[A-Za-z][A-Za-z0-9_]{1,30}", slug):
        raise SystemExit("--set-slug must start with a letter and contain only letters, digits, and underscores (2-31 characters).")
    slug = slug.lower()
    if "__" in slug or slug.endswith("_"):
        raise SystemExit("--set-slug cannot contain consecutive underscores or end in an underscore, because Telegram appends `_by_<bot_username>`.")
    return slug


def validate_set_name(name: str) -> str:
    if not re.fullmatch(r"[A-Za-z][A-Za-z0-9_]{0,63}", name):
        raise SystemExit("--recover-set must be a Telegram sticker-set name of 1-64 letters, digits, or underscores.")
    name = name.lower()
    if "__" in name or name.endswith("_"):
        raise SystemExit("--recover-set has an invalid Telegram sticker-set name.")
    return name


def retry_delay(attempt: int) -> float:
    return 0.5 * (2 ** (attempt - 1))


def call(api: str, method: str, **kwargs):
    """Call Telegram without ever putting the credential-bearing URL in errors."""
    files = kwargs.pop("files", None)
    read_only = files is None and method in READ_ONLY_METHODS
    attempts = READ_RETRY_ATTEMPTS if read_only else 1
    for attempt in range(1, attempts + 1):
        try:
            if read_only:
                response = HTTP.get(f"{api}/{method}", params=kwargs, timeout=(8, 12))
            else:
                response = HTTP.post(f"{api}/{method}", data=kwargs, files=files, timeout=(8, 60))
        except requests.RequestException as error:
            failure_kind = type(error).__name__
            if attempt < attempts:
                print(f"{method}: {failure_kind}; retrying safe read ({attempt}/{attempts - 1}).")
                time.sleep(retry_delay(attempt))
                continue
            raise TelegramTransportError(method, failure_kind, attempt) from None
        try:
            payload = response.json()
        except ValueError:
            failure_kind = "NonJSONResponse"
            if attempt < attempts:
                print(f"{method}: {failure_kind}; retrying safe read ({attempt}/{attempts - 1}).")
                time.sleep(retry_delay(attempt))
                continue
            raise TelegramTransportError(method, failure_kind, attempt) from None
        if not isinstance(payload, dict):
            raise RuntimeError(f"{method} returned a non-object JSON response (HTTP {response.status_code}).")
        if not payload.get("ok"):
            raise TelegramAPIError(method, payload)
        if not response.ok:
            raise RuntimeError(f"{method} returned HTTP {response.status_code} with an invalid success payload.")
        return payload["result"]
    raise AssertionError("unreachable")


def sticker_input(sticker_reference: str) -> dict:
    return {"sticker": sticker_reference, "format": ASSET["format"], "emoji_list": [ASSET["emoji"]]}


def create_set_directly(api: str, owner_id: str, set_name: str) -> None:
    """Create a new set in one multipart request; never append or replace."""
    with ASSET["path"].open("rb") as source:
        created = call(
            api,
            "createNewStickerSet",
            user_id=owner_id,
            name=set_name,
            title=ASSET["title"],
            sticker_type="custom_emoji",
            stickers=json.dumps([sticker_input("attach://oracle_video")]),
            files={"oracle_video": (ASSET["path"].name, source, "video/webm")},
        )
    if created is not True:
        raise RuntimeError("createNewStickerSet returned an unexpected success payload.")


def send_verification_message(api: str, owner_id: str, custom_emoji_id: str) -> None:
    text = f"{ASSET['emoji']} {ASSET['key']} \u2014 inspect at inline size and long-press scale.\n"
    entity = {"type": "custom_emoji", "offset": 0, "length": utf16_len(ASSET["emoji"]), "custom_emoji_id": custom_emoji_id}
    call(api, "sendMessage", chat_id=owner_id, text=text, entities=json.dumps([entity]))


def write_journal(set_name: str, status: str, detail: str | None = None) -> Path:
    MANIFEST_ROOT.mkdir(parents=True, exist_ok=True)
    journal_path = MANIFEST_ROOT / f"{set_name}.intent.json"
    journal = {
        "schema_version": 1,
        "purpose": "local Telegram fresh-test intent and recovery record; never contains a bot token",
        "updated_at": datetime.now(UTC).isoformat(),
        "status": status,
        "sticker_set": set_name,
        "asset": {
            "key": ASSET["key"],
            "source": asset_source_label(),
            "sha256": sha256(ASSET["path"]),
        },
    }
    if detail:
        journal["detail"] = redact_tokens(detail)
    journal_path.write_text(json.dumps(journal, indent=2) + "\n", encoding="utf-8")
    return journal_path


def write_manifest(set_name: str, sticker: dict) -> Path:
    MANIFEST_ROOT.mkdir(parents=True, exist_ok=True)
    manifest_path = MANIFEST_ROOT / f"{set_name}.json"
    manifest = {
        "schema_version": 2,
        "purpose": "fresh isolated Telegram visual test; not production mapping",
        "created_at": datetime.now(UTC).isoformat(),
        "sticker_set": set_name,
        "sticker_set_url": f"https://t.me/addemoji/{set_name}",
        "create_mode": "direct_multipart_createNewStickerSet",
        "asset": {
            "key": ASSET["key"],
            "source": asset_source_label(),
            "sha256": sha256(ASSET["path"]),
            "custom_emoji_id": sticker["custom_emoji_id"],
        },
        "verification": {"local_media_contract": "pass", "telegram_client": "PENDING_OWNER_REVIEW"},
    }
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    return manifest_path


def finalize_verified_set(api: str, owner_id: str, set_name: str, sticker_set: dict) -> None:
    if sticker_set.get("sticker_type") != "custom_emoji":
        raise RuntimeError("Fresh set assertion failed: Telegram did not return a custom-emoji set.")
    stickers = sticker_set.get("stickers", [])
    if len(stickers) != 1:
        raise RuntimeError(f"Fresh set assertion failed: expected one sticker, found {len(stickers)}. No mapping was written.")
    sticker = stickers[0]
    if sticker.get("is_video") is not True:
        raise RuntimeError("Fresh set assertion failed: Telegram did not return a video custom emoji.")
    custom_emoji_id = sticker.get("custom_emoji_id")
    if not isinstance(custom_emoji_id, str) or not custom_emoji_id:
        raise RuntimeError("Fresh set assertion failed: Telegram returned no custom_emoji_id. No mapping was written.")
    manifest = write_manifest(set_name, sticker)
    try:
        send_verification_message(api, owner_id, custom_emoji_id)
    except (TelegramAPIError, TelegramTransportError) as error:
        journal = write_journal(set_name, "VERIFIED_MESSAGE_PENDING", str(error))
        print("Set creation and readback passed, but the bot could not send its verification message.")
        print("Open the pack link below manually, confirm the bot DM was started, then use --recover-set to retry only the message/readback.")
        print(f"Recovery journal: {journal}")
    else:
        write_journal(set_name, "VERIFIED_MESSAGE_SENT")
    print(f"Verified fresh test set: {set_name}")
    print(f"Pack link: https://t.me/addemoji/{set_name}")
    print(f"custom_emoji_id: {custom_emoji_id}")
    print(f"Wrote deterministic test manifest: {manifest}")
    print("Telegram client review is now required; this is not production approval.")


def credentials(args: argparse.Namespace) -> tuple[str, str]:
    token = getpass.getpass("Paste BotFather token locally (input hidden): ") if args.prompt_token else os.environ.get("TG_BOT_TOKEN", "")
    token = token.strip()
    owner_id = (args.owner_id or os.environ.get("TG_OWNER_ID", "")).strip()
    if not token or not owner_id:
        raise SystemExit("--apply requires --prompt-token (recommended) or TG_BOT_TOKEN, plus --owner-id or TG_OWNER_ID.")
    if not re.fullmatch(r"\d{6,}:[A-Za-z0-9_-]{20,}", token):
        raise SystemExit("The bot token has an invalid shape. Generate a fresh token in @BotFather and enter it only at the hidden prompt.")
    if not owner_id.isdecimal():
        raise SystemExit("--owner-id / TG_OWNER_ID must be numeric.")
    return token, owner_id


def main() -> None:
    args = parse_args()
    configure_asset(args)
    source = ASSET["path"]
    if not source.is_file():
        raise SystemExit(f"Missing source asset: {source}")
    print(f"Asset: {ASSET['key']} ({ASSET['format']}, {source.name}, {source.stat().st_size / 1024:.1f}KiB)")
    print(f"SHA-256: {sha256(source)}")
    if not args.apply:
        print("DRY RUN: no Telegram request was made.")
        if args.recover_set:
            print(f"Would only read and verify existing set `{validate_set_name(args.recover_set)}`; no set mutation would occur.")
        else:
            slug = validate_slug(args.set_slug)
            print(f"Would create only a NEW set named `{slug}_by_<bot_username>` in one multipart request.")
            print("Telegram rejects an existing name; this workflow never appends to, replaces, or deletes a set.")
        return

    token, owner_id = credentials(args)
    api = f"https://api.telegram.org/bot{token}"
    if args.recover_set:
        set_name = validate_set_name(args.recover_set)
        print(f"RECOVERY: reading only `{set_name}`; no set creation or modification will occur.")
        try:
            recovered = call(api, "getStickerSet", name=set_name)
        except TelegramTransportError as error:
            raise SystemExit(f"Recovery could not reach Telegram safely: {error}. No set was changed.")
        except TelegramAPIError as error:
            raise SystemExit(f"Recovery could not read `{set_name}`: {error.description}. No set was changed.")
        finalize_verified_set(api, owner_id, set_name, recovered)
        return

    slug = validate_slug(args.set_slug)
    try:
        bot = call(api, "getMe")
    except TelegramTransportError as error:
        raise SystemExit(f"Could not identify the bot: {error}. No set was changed.")
    except TelegramAPIError as error:
        raise SystemExit(f"Telegram rejected the bot token: {error.description}. No set was changed.")
    bot_username = bot.get("username")
    if not isinstance(bot_username, str) or not bot_username:
        raise RuntimeError("getMe returned a bot without a username. Set a username in @BotFather before creating a sticker set.")
    set_name = f"{slug}_by_{bot_username}"
    if len(set_name) > 64:
        raise SystemExit(f"Fresh set name is too long ({len(set_name)} > 64): {set_name}")

    journal = write_journal(set_name, "CREATE_PENDING")
    print(f"Creating a fresh set in one direct multipart request: {set_name}")
    try:
        create_set_directly(api, owner_id, set_name)
    except TelegramTransportError as error:
        write_journal(set_name, "CREATE_UNKNOWN", str(error))
        try:
            reconciled = call(api, "getStickerSet", name=set_name)
        except (TelegramTransportError, TelegramAPIError):
            raise SystemExit(
                f"The create request has an unknown outcome ({error}); it was not retried.\n"
                f"Journal: {journal}\n"
                f"Do not reuse this slug. To inspect later without mutation, run --recover-set {set_name}."
            )
        print("The create response was interrupted, but safe readback found the new set.")
        finalize_verified_set(api, owner_id, set_name, reconciled)
        return
    except TelegramAPIError as error:
        write_journal(set_name, "CREATE_REJECTED", error.description)
        raise SystemExit(
            f"Telegram rejected the fresh-set creation: {error.description}. No existing set was changed.\n"
            f"Journal: {journal}\n"
            "Choose a new --set-slug only if you want a separate test."
        )

    write_journal(set_name, "CREATED")
    try:
        created = call(api, "getStickerSet", name=set_name)
    except (TelegramTransportError, TelegramAPIError) as error:
        write_journal(set_name, "CREATED_READBACK_PENDING", str(error))
        raise SystemExit(
            f"The fresh set was created, but readback did not complete: {error}.\n"
            f"Pack link: https://t.me/addemoji/{set_name}\n"
            f"Do not recreate it. Recover safely later with --recover-set {set_name}."
        )
    finalize_verified_set(api, owner_id, set_name, created)


if __name__ == "__main__":
    main()
