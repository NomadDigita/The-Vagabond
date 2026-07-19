#!/usr/bin/env python3
"""Regression tests for safe Telegram fresh-set response handling."""

from __future__ import annotations

import importlib.util
import traceback
import unittest
from pathlib import Path
from unittest.mock import patch

import requests


SCRIPT = Path(__file__).with_name("telegram_fresh_test_set.py")
SPEC = importlib.util.spec_from_file_location("telegram_fresh_test_set", SCRIPT)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


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


class TelegramFreshTestSetResponseTests(unittest.TestCase):
    def test_missing_set_is_a_safe_absence(self) -> None:
        response = FakeResponse(400, {"ok": False, "error_code": 400, "description": "Bad Request: STICKERSET_INVALID"})
        with patch.object(MODULE.requests, "post", return_value=response):
            self.assertIsNone(MODULE.get_set_if_missing_only("https://api.telegram.org/botREDACTED", "new_set"))

    def test_transport_failure_does_not_echo_token(self) -> None:
        secret_url = "https://api.telegram.org/botTOKEN_MUST_NOT_APPEAR/getStickerSet"
        with patch.object(MODULE.requests, "post", side_effect=requests.ConnectionError(secret_url)):
            with self.assertRaisesRegex(RuntimeError, "getStickerSet transport failure") as raised:
                MODULE.call("https://api.telegram.org/botTOKEN_MUST_NOT_APPEAR", "getStickerSet", name="new_set")
        self.assertNotIn("TOKEN_MUST_NOT_APPEAR", str(raised.exception))
        rendered_traceback = "".join(traceback.format_exception(raised.exception))
        self.assertNotIn("TOKEN_MUST_NOT_APPEAR", rendered_traceback)

    def test_non_json_response_is_safe_and_actionable(self) -> None:
        response = FakeResponse(502, ValueError("gateway HTML containing https://api.telegram.org/botTOKEN_MUST_NOT_APPEAR"))
        with patch.object(MODULE.requests, "post", return_value=response):
            with self.assertRaisesRegex(RuntimeError, r"non-JSON response \(HTTP 502\)") as raised:
                MODULE.call("https://api.telegram.org/botTOKEN_MUST_NOT_APPEAR", "getStickerSet", name="new_set")
        self.assertNotIn("TOKEN_MUST_NOT_APPEAR", str(raised.exception))

    def test_invalid_sticker_set_slug_is_rejected_before_network_access(self) -> None:
        with self.assertRaisesRegex(SystemExit, "consecutive underscores"):
            MODULE.validate_slug("valid__but_rejected")
        with self.assertRaisesRegex(SystemExit, "end in an underscore"):
            MODULE.validate_slug("trailing_")


if __name__ == "__main__":
    unittest.main()
