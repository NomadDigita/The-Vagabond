# ADR 0034: Decisions from `internal/ai/providers/openaicompat/provider.go`

Status: Accepted

These decisions were written as comments in `internal/ai/providers/openaicompat/provider.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package openaicompat implements ai.Provider once against the

Removed from `internal/ai/providers/openaicompat/provider.go:1`.

~~~~text
Package openaicompat implements ai.Provider once against the
OpenAI Chat Completions wire format
(POST {base_url}/chat/completions), which is also exposed, with
only cosmetic differences, by:
  - OpenAI itself            (https://api.openai.com/v1)
  - DeepSeek                 (https://api.deepseek.com/v1)
  - Alibaba Qwen / DashScope compatible mode
    (https://dashscope-intl.aliyuncs.com/compatible-mode/v1, or the
    mainland-China endpoint dashscope.aliyuncs.com/compatible-mode/v1)
  - xAI Grok                 (https://api.x.ai/v1)
  - A locally-run Ollama instance's OpenAI-compatible endpoint
    (http://<host>:11434/v1) — though internal/ai/providers/ollama
    has its own dedicated implementation using Ollama's native API,
    which supports a few Ollama-specific features this generic
    client does not.

One implementation, four (or more) providers — each is just this
struct constructed with a different BaseURL/APIKey/DefaultModel/Name.
This directly satisfies the roadmap's "Support future providers like
OpenAI, ..., Qwen, Grok, DeepSeek, ..." requirement without writing
near-duplicate HTTP clients four times.
~~~~


## ProviderName is what Name() returns — e.g. "openai", "deepseek",

Removed from `internal/ai/providers/openaicompat/provider.go:39`.

~~~~text
ProviderName is what Name() returns — e.g. "openai", "deepseek",
"qwen", "grok". Distinct from the underlying model name.
~~~~


## BaseURL has no trailing slash, e.g. "https://api.openai.com/v1".

Removed from `internal/ai/providers/openaicompat/provider.go:42`.

~~~~text
BaseURL has no trailing slash, e.g. "https://api.openai.com/v1".
~~~~


## SupportsJSONResponseFormat controls whether {"response_format":

Removed from `internal/ai/providers/openaicompat/provider.go:46`.

~~~~text
SupportsJSONResponseFormat controls whether {"response_format":
{"type":"json_object"}} is sent when CompletionRequest.JSONMode
is true. All four target APIs above support it as of this
writing, but the field exists so a future 5th OpenAI-compatible
provider that doesn't can be registered without a code change —
it will fall back to the same system-prompt-instruction approach
internal/ai/providers/anthropic uses.
~~~~


## New builds an OpenAI-compatible provider. apiKey may be empty;

Removed from `internal/ai/providers/openaicompat/provider.go:58`.

~~~~text
New builds an OpenAI-compatible provider. apiKey may be empty;
Available() will correctly report false in that case.
~~~~


## JSON-encoded string, per OpenAI's format

Removed from `internal/ai/providers/openaicompat/provider.go:92`.

~~~~text
JSON-encoded string, per OpenAI's format
~~~~


## EnableThinking is DashScope/Qwen-specific. Left as a pointer so

Removed from `internal/ai/providers/openaicompat/provider.go:113`.

PROJECT_MASTER_PLAN.md records the Qwen enable_thinking choice as ADR-023. The comment cited ADR-016 and ADR-017, which are different decisions. The wire-format reason in the comment still matches the provider.

~~~~text
EnableThinking is DashScope/Qwen-specific. Left as a pointer so
it's omitted entirely for OpenAI/DeepSeek/Grok, which don't
recognize the field. Qwen3 models default this to true, and
the resulting reasoning tokens are drawn from the same
max_tokens budget as the visible completion — this silently
starves JSON-mode callers (Dev Console balance/weekly reports,
advisors) of their MaxTokens before any visible text is
written, producing output that looks truncated even though
nothing is wrong with MaxTokens itself. See ADR-016/017.
~~~~


## DashScope's compatible-mode endpoint defaults enable_thinking

Removed from `internal/ai/providers/openaicompat/provider.go:162`.

~~~~text
DashScope's compatible-mode endpoint defaults enable_thinking
to true for Qwen3 models. Force it off so reasoning tokens
don't eat into MaxTokens before the visible JSON/text is
written (see wireRequest.EnableThinking doc comment above).
~~~~


## Belt-and-suspenders regardless of native support, exactly

Removed from `internal/ai/providers/openaicompat/provider.go:175`.

~~~~text
Belt-and-suspenders regardless of native support, exactly
like the Anthropic provider does — costs nothing and
tolerates a misconfigured SupportsJSONResponseFormat flag.
Appended even when System was empty, so JSON mode is never
silently dropped just because the caller had no system
prompt of its own (a real bug caught by this package's own
tests — see provider_test.go).
~~~~


## Tool-call arguments arrive as a JSON-encoded string per

Removed from `internal/ai/providers/openaicompat/provider.go:262`.

~~~~text
Tool-call arguments arrive as a JSON-encoded string per
the OpenAI wire format, not a nested object — must be
unmarshaled a second time. A malformed/partial arguments
string (which happens if the model's output was
truncated) is not treated as a fatal error: the tool
call is still surfaced with an empty Input map so the
caller can decide how to handle it, rather than losing
the whole completion over one bad tool call.
~~~~
