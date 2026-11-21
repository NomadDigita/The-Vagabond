# ADR 0026: Decisions from `internal/ai/probe.go`

Status: Accepted

These decisions were written as comments in `internal/ai/probe.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Err is the exact error returned by the provider on failure

Removed from `internal/ai/probe.go:14`.

~~~~text
Err is the exact error returned by the provider on failure
(e.g. "gemini: api error (RESOURCE_EXHAUSTED) on model
gemini-3.5-flash: ...", or "openaicompat: unexpected status 403:
...AccessDenied.Unpurchased..."). Empty on success. Deliberately
NOT truncated or reworded here — the caller (a Telegram handler)
decides display length; this is a diagnostic surface, and a
clipped or paraphrased error is exactly what has made this class
of bug take multiple sessions to root-cause in the past, each
one only able to see "available: true/false", never the real
account/quota/config reason a provider fell through to mock.
~~~~


## ProbeAllProviders issues one minimal real completion request against

Removed from `internal/ai/probe.go:28`.

~~~~text
ProbeAllProviders issues one minimal real completion request against
every registered, non-mock provider — including ones reporting
Available()==false, so a missing key shows up the same way a quota
error does, in one place — bypassing Service.Complete's cache and
permission layers entirely (this is an admin diagnostic, not a game
feature a player can trigger). The daily global budget is still
respected: if it's already exhausted, every provider is reported
skipped rather than silently spending past the cap.

This closes the actual gap behind "AI console still shows
PLACEHOLDER despite the key being set" being independently
re-investigated across half a dozen sessions (see
BUGS_AND_INCONSISTENCIES.md, 2026-08-02 entries): Registry.Ordered()
mock-always-last was fixed long before those sessions and was never
the problem again — the real, repeated problem was that nothing in
Telegram could distinguish "provider unavailable", "provider
available but erroring (bad key / quota / unpurchased model)", and
"correctly falling through to mock" from each other. All three
looked identical in the old /ai_status output. Every prior fix
session had to get live Render log access to find the real error
text; this makes that same error text visible from inside Telegram
on demand, going forward, without needing dashboard access at all.
~~~~
