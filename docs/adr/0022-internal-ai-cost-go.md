# ADR 0022: Decisions from `internal/ai/cost.go`

Status: Accepted

These decisions were written as comments in `internal/ai/cost.go` and moved here so the source no longer carries them. Each entry is the original comment.

## pricePerMillionTokens is used only for budget accounting (not

Removed from `internal/ai/cost.go:14`.

~~~~text
pricePerMillionTokens is used only for budget accounting (not
billing) — it estimates spend against the daily budget caps in
Config, it is not what any provider actually charges you.

Every entry below was verified by web search on 2026-07-15 against
each provider's own pricing documentation (not guessed). Prices
change — re-verify before trusting this table for a real budget
months from now, especially the fast-moving providers (DeepSeek,
Grok/xAI, Qwen) called out individually below.

Unknown provider/model pairs fall back to a deliberately
conservative "default" entry so cost tracking never silently
under-counts an unrecognized model.
~~~~


## Anthropic — confirmed at $3.00/$15.00 (Sonnet 4.6) as of 2026-07-15.

Removed from `internal/ai/cost.go:28`.

~~~~text
Anthropic — confirmed at $3.00/$15.00 (Sonnet 4.6) as of 2026-07-15.
~~~~


## OpenAI — GPT-5.6 reached general availability 2026-07-09,

Removed from `internal/ai/cost.go:31`.

~~~~text
OpenAI — GPT-5.6 reached general availability 2026-07-09,
superseding the gpt-4o family. gpt-5.6-luna (the cheapest tier)
confirmed at $1.00/$6.00 as of 2026-07-16 against OpenAI's own
pricing page. gpt-4o-mini ($0.15/$0.60) and gpt-4o ($2.50/$10.00)
are kept below since they likely still work if explicitly
configured, but are no longer the current generation.
~~~~


## DeepSeek — confirmed at $0.14/$0.28 for deepseek-v4-flash as of

Removed from `internal/ai/cost.go:41`.

~~~~text
DeepSeek — confirmed at $0.14/$0.28 for deepseek-v4-flash as of
2026-07-15. IMPORTANT: the legacy "deepseek-chat" model alias is
scheduled for deprecation by DeepSeek on 2026-07-24 — do not
configure DEEPSEEK_MODEL=deepseek-chat going forward, use
deepseek-v4-flash directly.
~~~~


## Qwen (Alibaba DashScope) — qwen-plus commonly cited around

Removed from `internal/ai/cost.go:48`.

~~~~text
Qwen (Alibaba DashScope) — qwen-plus commonly cited around
$0.40/$1.20 per million tokens as of mid-2026, though DashScope
pricing has tiered-by-request-length quirks and sources vary
(some cite $0.26/$0.78) — treat this row as approximate and
verify against the Alibaba Cloud Model Studio pricing page for
a real budget.
~~~~


## Grok (xAI) — re-confirmed 2026-07-16: xAI's flagship moved to

Removed from `internal/ai/cost.go:56`.

~~~~text
Grok (xAI) — re-confirmed 2026-07-16: xAI's flagship moved to
Grok 4.5 ($2.00/$6.00, launched 2026-07-08), but the cheap
"fast"/volume tier this codebase targets is still confirmed at
$0.20/$0.50 per million tokens (cited as "Grok 4.1 Fast" across
multiple independent pricing trackers). The exact API model
string is the least certain part of this row — sources show
both dash and dot conventions across xAI's model lineup
(grok-4.5, grok-4.3, grok-4.1-fast). Confirm the exact string at
https://docs.x.ai/docs/models before relying on GROK_MODEL in
production; if requests fail with a model-not-found error, that
is almost certainly why.
~~~~


## Gemini — gemini-3.5-flash confirmed at $1.50/$9.00 as of

Removed from `internal/ai/cost.go:69`.

~~~~text
Gemini — gemini-3.5-flash confirmed at $1.50/$9.00 as of
2026-07-16 (Google shipped the 3.5 generation 2026-05-19,
superseding 2.5; gemini-2.0-flash was separately shut down
2026-06-01 and must not be used). gemini-3.5-pro is not yet
generally available — do not add a pricing row for it until
independently confirmed GA.
~~~~


## Ollama (self-hosted) — genuinely zero marginal API cost; you pay

Removed from `internal/ai/cost.go:77`.

~~~~text
Ollama (self-hosted) — genuinely zero marginal API cost; you pay
for the compute instead. Kept at 0/0 rather than omitted so a
lookup for "ollama:<any model>" resolves to a real, correct
answer instead of falling through to "default" below.
~~~~


## CostTracker records spend and answers budget questions. The

Removed from `internal/ai/cost.go:99`.

~~~~text
CostTracker records spend and answers budget questions. The
Postgres-backed implementation is the source of truth across process
restarts and horizontal scaling; callers needing zero-dependency unit
tests can implement the interface with an in-memory fake.
~~~~


## nullableUserID maps the "system/background request" sentinel (0) to

Removed from `internal/ai/cost.go:150`.

~~~~text
nullableUserID maps the "system/background request" sentinel (0) to
NULL so ai_cost_log.user_id's FK to users(telegram_id) never breaks
on background jobs like the Dynamic Galaxy director.
~~~~
