# Telegram v10 Fresh-Test Runbook

Use this only after reviewing `V10_ORACLE_3D_TEST_REPORT.md`. This validates
the Oracle in a real Telegram client without touching the duplicate-filled
legacy pilot set.

## Preconditions

1. The test owner has Telegram Premium and has started the bot in a direct
   message.
2. The owner has a newly issued throwaway test-bot token and their numeric
   Telegram ID. The command below asks for the token locally with hidden input;
   never paste it into chat, a source file, a command line, or a commit.
3. The local media validator has passed.

Telegram’s current video custom-emoji requirements are a 100x100 VP9 WebM,
silent, at most 30fps, at most three seconds, and at most 256KiB. See the
[official VP9 encoding guide](https://core.telegram.org/stickers/webm-vp9-encoding)
and [Bot API sticker methods](https://core.telegram.org/bots/api#uploadstickerfile).

## Local commands

From the repository root, first re-run the offline validator with the local
FFmpeg executables:

```powershell
python assets/visual-system/pipeline/validate_video_custom_emoji.py `
  assets/visual-system/animated/oracle_3d_v10/oracle_3d_v10.webm `
  --ffprobe C:\path\to\ffprobe.exe `
  --ffmpeg C:\path\to\ffmpeg.exe
```

Inspect the no-network plan first:

```powershell
python assets/visual-system/pipeline/telegram_fresh_test_set.py
```

When the printed source filename, SHA-256, and new test-set name look right,
apply it once with a unique slug. The token is requested through Python's
hidden local prompt and is never written to a file or environment variable:

```powershell
python assets/visual-system/pipeline/telegram_fresh_test_set.py `
  --apply --prompt-token --owner-id YOUR_NUMERIC_TELEGRAM_ID `
  --set-slug vagabond_v10_oracle_test
```

The script aborts if that set name exists. Pick a new slug for a new test;
never reuse it to replace or append stickers. On success it writes a
`test-results/<set-name>.json` manifest containing the exact assigned
`custom_emoji_id` and source hash.

## Owner visual review

Check the same asset in all of these places before approving it:

1. Sticker picker at normal size.
2. A bot DM at true inline message size.
3. Long-press / expanded preview.
4. Dark chat, light chat, and a typical Telegram wallpaper.
5. Android and Desktop at minimum; iOS if available.

Reject the test if there is any black rectangle, magenta/cyan VP9 bleed,
flicker at the loop seam, unreadable inline silhouette, or a visually static
appearance. Record the client and result in the generated manifest before
considering production promotion.

## Explicitly out of scope

This test does not update `mapping.json`, does not touch Go code, and does not
modify the old `vagabond_pilot` sticker set. Production mapping can begin only
after the fresh test is approved and a deterministic pack-reconciliation design
has separately been reviewed.
