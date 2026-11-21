# ADR 0021: Decisions from `internal/ai/config.go`

Status: Accepted

These decisions were written as comments in `internal/ai/config.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Feature is a stable identifier for an AI-driven subsystem. Every

Removed from `internal/ai/config.go:9`.

~~~~text
Feature is a stable identifier for an AI-driven subsystem. Every
Phase B-J capability registers under one of these so permissions,
cost attribution, and feature flags all key off the same string.
~~~~


## FeatureCommandInterpreter is Milestone 3 of

Removed from `internal/ai/config.go:25`.

~~~~text
FeatureCommandInterpreter is Milestone 3 of
FEEDBACK_CHANGELOG_NLP_PLAN.md: the natural-language command
interpreter wired as nlp.HandleTextMessage's final fallback.
Unlike every feature above (advisory-only), this one's parsed
intent can result in state-mutating game actions - always via
the same doX core functions the button UI uses, and always
behind an explicit Confirm/Cancel step for anything that spends
resources. See the plan doc's "Why this is different" section.
~~~~


## Config holds process-wide AI settings, loaded from environment

Removed from `internal/ai/config.go:52`.

~~~~text
Config holds process-wide AI settings, loaded from environment
variables at startup. Per-feature and per-user overrides live in the
database (see ai_feature_flags / ai_permissions in
migrations/020_vagabond_ai_foundation.sql) and are consulted by
PermissionManager on top of this static config.
~~~~


## Enabled is a master kill switch. When false, Service.Complete

Removed from `internal/ai/config.go:58`.

~~~~text
Enabled is a master kill switch. When false, Service.Complete
returns ErrAIDisabled immediately for every request.
~~~~


## AnthropicAPIKey / AnthropicModel configure the built-in

Removed from `internal/ai/config.go:68`.

~~~~text
AnthropicAPIKey / AnthropicModel configure the built-in
Anthropic provider. Empty key means the provider reports
itself Available() == false and is skipped.
~~~~


## DeepSeekAPIKey / DeepSeekModel configure the DeepSeek provider

Removed from `internal/ai/config.go:79`.

~~~~text
DeepSeekAPIKey / DeepSeekModel configure the DeepSeek provider
(openaicompat, registered as "deepseek"). Note: as of 2026-07-15
the legacy "deepseek-chat" model alias is scheduled for
deprecation by DeepSeek on 2026-07-24 — the default below uses
"deepseek-v4-flash" instead.
~~~~


## QwenAPIKey / QwenModel / QwenBaseURL configure the Qwen provider

Removed from `internal/ai/config.go:87`.

~~~~text
QwenAPIKey / QwenModel / QwenBaseURL configure the Qwen provider
(openaicompat, registered as "qwen") via Alibaba DashScope's
OpenAI-compatible endpoint. QwenBaseURL defaults to the
international (Singapore) endpoint; mainland China deployments
should set it to the dashscope.aliyuncs.com equivalent.
~~~~


## GrokAPIKey / GrokModel configure the Grok/xAI provider

Removed from `internal/ai/config.go:96`.

~~~~text
GrokAPIKey / GrokModel configure the Grok/xAI provider
(openaicompat, registered as "grok"). xAI's model lineup and
naming have shifted repeatedly through 2026 — verify GrokModel
against https://docs.x.ai/docs/models if recommendations seem
to be failing with a model-not-found error.
~~~~


## GeminiAPIKey / GeminiModel configure the Gemini provider

Removed from `internal/ai/config.go:104`.

~~~~text
GeminiAPIKey / GeminiModel configure the Gemini provider
(internal/ai/providers/gemini, registered as "gemini").
GeminiModelFallbacks are additional models the provider itself
cycles through before giving up (see gemini.Provider.Complete) -
Google tracks free-tier quota per model, not per account, so a
different model is frequently still usable even when the
configured one is exhausted. Defaults to a hand-picked list of
free-tier-eligible models (see LoadConfig below); set
GEMINI_MODEL_FALLBACKS="" explicitly to disable and fail straight
to the next provider instead.
~~~~


## OllamaBaseURL / OllamaModel configure the Ollama provider

Removed from `internal/ai/config.go:118`.

~~~~text
OllamaBaseURL / OllamaModel configure the Ollama provider
(internal/ai/providers/ollama, registered as "ollama") for
self-hosted open-weight models. No API key is used. Empty
OllamaBaseURL means the provider reports itself unavailable —
it is never enabled by default.
~~~~


## MaxUserCostPerDayUSD caps spend attributable to a single

Removed from `internal/ai/config.go:126`.

~~~~text
MaxUserCostPerDayUSD caps spend attributable to a single
Telegram user across all features, per UTC day. Zero disables
the per-user cap (not recommended in production).
~~~~


## CacheTTLSeconds controls how long identical requests (same

Removed from `internal/ai/config.go:134`.

~~~~text
CacheTTLSeconds controls how long identical requests (same
feature + user + prompt hash) are served from cache instead of
hitting a provider. Zero disables caching.
~~~~


## Default model note (confirmed via web search 2026-07-16):

Removed from `internal/ai/config.go:250`.

~~~~text
Default model note (confirmed via web search 2026-07-16):
gpt-4o-mini is an older OpenAI generation. OpenAI shipped
GPT-5.6 to general availability on 2026-07-09; Luna is its
cheapest tier ($1.00/$6.00 per million tokens, confirmed
against OpenAI's own pricing page). gpt-4o-mini likely still
works but is no longer current.
~~~~
