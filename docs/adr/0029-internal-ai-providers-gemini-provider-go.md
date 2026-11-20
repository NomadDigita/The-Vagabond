# ADR 0029: Decisions from `internal/ai/providers/gemini/provider.go`

Status: Accepted

These decisions were written as comments in `internal/ai/providers/gemini/provider.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package gemini implements ai.Provider against Google's Gemini API

Removed from `internal/ai/providers/gemini/provider.go:1`.

~~~~text
Package gemini implements ai.Provider against Google's Gemini API
(https://generativelanguage.googleapis.com), which uses a distinct
wire format from the OpenAI-Chat-Completions family (see
internal/ai/providers/openaicompat) — different role names ("model"
instead of "assistant"), a top-level systemInstruction field instead
of a system-role message, and a "parts" array per content item.
~~~~


## ModelFallbacks are additional model names tried in order, within

Removed from `internal/ai/providers/gemini/provider.go:28`.

~~~~text
ModelFallbacks are additional model names tried in order, within
this same provider, if DefaultModel (or req.Model) comes back
quota-exhausted or overloaded - see Complete's isRetryableModelError.
Google's free-tier quotas are tracked per model, not per account,
so a different model is frequently still available even when the
configured one is exhausted (confirmed live via Render logs,
2026-08-02: gemini-3.5-flash hit RESOURCE_EXHAUSTED on its 20-
request/day free allotment). This only ever cycles models within
Gemini - if every model here is exhausted too, Complete returns
the last error and internal/ai.Service's own provider-level
fallback (Registry.Ordered) takes over, moving on to Qwen/mock.
~~~~


## BaseURL defaults to Google's real endpoint; overridable so tests

Removed from `internal/ai/providers/gemini/provider.go:41`.

~~~~text
BaseURL defaults to Google's real endpoint; overridable so tests
can point this at a local httptest server instead of requiring
live network access to verify the wire format end-to-end.
~~~~


## New builds a Gemini provider. apiKey may be empty; Available() will

Removed from `internal/ai/providers/gemini/provider.go:47`.

~~~~text
New builds a Gemini provider. apiKey may be empty; Available() will
correctly report false in that case. modelFallbacks may be nil/empty;
see ModelFallbacks' doc comment.

Default model note (confirmed via web search 2026-08-05, superseding
this package's 2026-07-16 note): gemini-3.5-flash (this default)
remains current with no shutdown date announced per Google's own
deprecations page. Google has since shipped Gemini 3.6 Flash and
Gemini 3.5 Flash-Lite to general availability (~2026-07-30) as the
new default fallback models below, replacing the 2.5-generation
names this package shipped with initially - gemini-2.5-flash-lite
was confirmed dead in production via /ai_probe on 2026-08-05
("model gemini-2.5-flash-lite is no longer available to new users"),
and the wider 2.0/2.5 Flash family is being wound down through 2026
per Google's changelog. gemini-3.1-flash-lite (the third fallback)
is confirmed still valid through at least May 2027.
~~~~


## "user" or "model"; omitted for systemInstruction

Removed from `internal/ai/providers/gemini/provider.go:92`.

~~~~text
"user" or "model"; omitted for systemInstruction
~~~~


## toGeminiRole maps internal/ai's Role to Gemini's "user"/"model"

Removed from `internal/ai/providers/gemini/provider.go:138`.

~~~~text
toGeminiRole maps internal/ai's Role to Gemini's "user"/"model"
vocabulary. RoleTool is folded into "user", with the same caveat as
every other provider in this codebase — see ADR-006 in
PROJECT_MASTER_PLAN.md.
~~~~


## isRetryableModelError reports whether an error from one Gemini model

Removed from `internal/ai/providers/gemini/provider.go:149`.

~~~~text
isRetryableModelError reports whether an error from one Gemini model
is worth retrying against a different model in ModelFallbacks, versus
an error that would just as surely fail again with any model (bad
API key, malformed request, network failure) - in which case Complete
returns immediately so internal/ai.Service's provider-level fallback
can move on to the next *provider* without wasting a request per
fallback model first. RESOURCE_EXHAUSTED/429 is Google's quota-hit
status (confirmed live via Render logs, 2026-08-02); UNAVAILABLE/503
is transient overload - both are specific to the model that was
asked for, not the account or the request. NOT_FOUND/404 (confirmed
live via /ai_probe, 2026-08-05: "model gemini-2.5-flash-lite is no
longer available to new users") is the same category for a different
reason - Google periodically retires specific model IDs, and a
retired *fallback* model name is a stale config value, not a request
problem, so the sensible behavior is to skip past it to the next
configured fallback rather than let one dead model name in
ModelFallbacks permanently block every model listed after it. This
only ever cycles within the caller-configured list, so a genuinely
wrong model name still surfaces as an error once every fallback is
exhausted - it just no longer masks working models later in the list.
~~~~


## req.Model (a specific caller-requested model) is honored as the

Removed from `internal/ai/providers/gemini/provider.go:186`.

~~~~text
req.Model (a specific caller-requested model) is honored as the
first attempt, but ModelFallbacks still applies after it - a
caller asking for a specific model doesn't want a totally silent
account-wide outage just because that one model is exhausted.
~~~~


## modelError carries the Gemini-reported status/HTTP code alongside the

Removed from `internal/ai/providers/gemini/provider.go:220`.

~~~~text
modelError carries the Gemini-reported status/HTTP code alongside the
formatted message, so Complete's retry loop can distinguish "try a
different model" from "give up" without re-parsing error strings.
~~~~


## Belt-and-suspenders instruction alongside the native JSON

Removed from `internal/ai/providers/gemini/provider.go:247`.

~~~~text
Belt-and-suspenders instruction alongside the native JSON
mime type, matching every other provider's approach.
~~~~
