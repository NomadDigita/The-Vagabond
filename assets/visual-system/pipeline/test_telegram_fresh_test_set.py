#!/usr/bin/env python3
"""Offline regression tests for the safe Telegram fresh-set workflow."""

from __future__ import annotations

import argparse
import importlib.util
import io
import json
import os
import tempfile
import traceback
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch

import requests


SCRIPT = Path(__file__).with_name("telegram_fresh_test_set.py")
SPEC = importlib.util.spec_from_file_location("telegram_fresh_test_set", SCRIPT)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)

SAFE_TOKEN = "123456:abcdefghijklmnopqrst"
SET_RESULT = {
    "name": "fresh_test_by_testbot",
    "sticker_type": "custom_emoji",
    "stickers": [{"is_video": True, "custom_emoji_id": "987654321"}],
}


class FakeResponse:
    def __init__(self, status_code: int, payload: object | Exception):
        self.status_code = status_code
        self._payload = payload

    @property
    def ok(self) -> bool:
        return 200 <= self.status_code < 300

    def json(self):
        if isinstance(self._payload, Exception):
            raise self._payload
        return self._payload


class RecordingHTTP:
    def __init__(self, get_responses: list[FakeResponse], post_responses: list[FakeResponse]):
        self.get_responses = list(get_responses)
        self.post_responses = list(post_responses)
        self.calls: list[tuple[str, str, dict]] = []

    def get(self, url: str, **kwargs):
        self.calls.append(("GET", url, kwargs))
        return self.get_responses.pop(0)

    def post(self, url: str, **kwargs):
        self.calls.append(("POST", url, kwargs))
        return self.post_responses.pop(0)


class TelegramFreshTestSetResponseTests(unittest.TestCase):
    def test_safe_reads_retry_and_never_echo_token(self) -> None:
        secret_url = "https://api.telegram.org/botTOKEN_MUST_NOT_APPEAR/getStickerSet"
        http = RecordingHTTP([], [])
        with patch.object(http, "get", side_effect=requests.ConnectionError(secret_url)) as get_request, patch.object(MODULE, "HTTP", http), patch.object(MODULE.time, "sleep") as sleep:
            with self.assertRaises(MODULE.TelegramTransportError) as raised:
                MODULE.call(secret_url.rsplit("/", 1)[0], "getStickerSet", name="fresh_test")
        self.assertEqual(get_request.call_count, 4)
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [0.5, 1.0, 2.0])
        rendered_traceback = "".join(traceback.format_exception(raised.exception))
        self.assertNotIn("TOKEN_MUST_NOT_APPEAR", str(raised.exception))
        self.assertNotIn("TOKEN_MUST_NOT_APPEAR", rendered_traceback)

    def test_non_json_safe_read_retries(self) -> None:
        http = RecordingHTTP([FakeResponse(502, ValueError("gateway HTML"))] * 4, [])
        with patch.object(MODULE, "HTTP", http), patch.object(MODULE.time, "sleep") as sleep:
            with self.assertRaisesRegex(MODULE.TelegramTransportError, "NonJSONResponse"):
                MODULE.call("https://api.telegram.org/botREDACTED", "getStickerSet", name="fresh_test")
        self.assertEqual(len(http.calls), 4)
        self.assertEqual(sleep.call_count, 3)

    def test_direct_create_posts_one_attached_video(self) -> None:
        http = RecordingHTTP([], [FakeResponse(200, {"ok": True, "result": True})])
        with patch.object(MODULE, "HTTP", http):
            MODULE.create_set_directly("https://api.telegram.org/botREDACTED", "6582793388", "fresh_test_by_testbot")
        self.assertEqual(len(http.calls), 1)
        method, url, kwargs = http.calls[0]
        self.assertEqual((method, url.rsplit("/", 1)[-1]), ("POST", "createNewStickerSet"))
        payload = json.loads(kwargs["data"]["stickers"])
        self.assertEqual(payload, [{"sticker": "attach://oracle_video", "format": "video", "emoji_list": [MODULE.ASSET["emoji"]]}])
        self.assertEqual(set(kwargs["files"]), {"oracle_video"})
        self.assertEqual(kwargs["files"]["oracle_video"][0], MODULE.ASSET["path"].name)
        self.assertEqual(kwargs["files"]["oracle_video"][2], "video/webm")

    def test_main_creates_without_a_preflight_set_lookup(self) -> None:
        http = RecordingHTTP(
            [FakeResponse(200, {"ok": True, "result": {"username": "testbot"}}), FakeResponse(200, {"ok": True, "result": SET_RESULT})],
            [FakeResponse(200, {"ok": True, "result": True}), FakeResponse(200, {"ok": True, "result": {"message_id": 1}})],
        )
        args = argparse.Namespace(apply=True, prompt_token=False, owner_id="6582793388", set_slug="fresh_test", recover_set=None)
        with tempfile.TemporaryDirectory() as temporary, patch.object(MODULE, "HTTP", http), patch.object(MODULE, "MANIFEST_ROOT", Path(temporary)), patch.object(MODULE, "parse_args", return_value=args), patch.dict(os.environ, {"TG_BOT_TOKEN": SAFE_TOKEN}, clear=False), redirect_stdout(io.StringIO()):
            MODULE.main()
        call_names = [(method, url.rsplit("/", 1)[-1]) for method, url, _ in http.calls]
        self.assertEqual(call_names, [("GET", "getMe"), ("POST", "createNewStickerSet"), ("GET", "getStickerSet"), ("POST", "sendMessage")])

    def test_recovery_never_creates_or_modifies_a_set(self) -> None:
        http = RecordingHTTP([FakeResponse(200, {"ok": True, "result": SET_RESULT})], [FakeResponse(200, {"ok": True, "result": {"message_id": 1}})])
        args = argparse.Namespace(apply=True, prompt_token=False, owner_id="6582793388", set_slug="ignored_slug", recover_set="fresh_test_by_testbot")
        with tempfile.TemporaryDirectory() as temporary, patch.object(MODULE, "HTTP", http), patch.object(MODULE, "MANIFEST_ROOT", Path(temporary)), patch.object(MODULE, "parse_args", return_value=args), patch.dict(os.environ, {"TG_BOT_TOKEN": SAFE_TOKEN}, clear=False), redirect_stdout(io.StringIO()):
            MODULE.main()
        call_names = [(method, url.rsplit("/", 1)[-1]) for method, url, _ in http.calls]
        self.assertEqual(call_names, [("GET", "getStickerSet"), ("POST", "sendMessage")])

    def test_mutating_create_is_never_retried(self) -> None:
        secret_url = "https://api.telegram.org/botTOKEN_MUST_NOT_APPEAR/createNewStickerSet"
        http = RecordingHTTP([], [])
        with patch.object(http, "post", side_effect=requests.ConnectionError(secret_url)) as post_request, patch.object(MODULE, "HTTP", http):
            with self.assertRaises(MODULE.TelegramTransportError) as raised:
                MODULE.create_set_directly(secret_url.rsplit("/", 1)[0], "6582793388", "fresh_test_by_testbot")
        self.assertEqual(post_request.call_count, 1)
        self.assertNotIn("TOKEN_MUST_NOT_APPEAR", "".join(traceback.format_exception(raised.exception)))

    def test_api_error_redacts_token_like_text(self) -> None:
        error = MODULE.TelegramAPIError("getStickerSet", {"error_code": 400, "description": f"Bad request {SAFE_TOKEN}"})
        self.assertNotIn(SAFE_TOKEN, str(error))
        self.assertIn("<redacted-bot-token>", str(error))

    def test_invalid_sticker_set_names_are_rejected(self) -> None:
        with self.assertRaisesRegex(SystemExit, "consecutive underscores"):
            MODULE.validate_slug("valid__but_rejected")
        with self.assertRaisesRegex(SystemExit, "invalid Telegram"):
            MODULE.validate_set_name("trailing_")


if __name__ == "__main__":
    unittest.main()
