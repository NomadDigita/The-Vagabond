# ADR 0014: Decisions from `assets/visual-system/pipeline/telegram_fresh_test_set.py`

Status: Accepted

These decisions were written as comments in `assets/visual-system/pipeline/telegram_fresh_test_set.py` and moved here so the source no longer carries them. Each entry is the original comment.

## Reject local naming mistakes before requesting any credential. This keeps

Removed from `assets/visual-system/pipeline/telegram_fresh_test_set.py:303`.

~~~~text
Reject local naming mistakes before requesting any credential. This keeps
a typo such as an overlong fresh-test slug from causing an unnecessary
hidden-token prompt.
~~~~
