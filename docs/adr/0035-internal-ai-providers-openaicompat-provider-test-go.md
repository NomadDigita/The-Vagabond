# ADR 0035: Decisions from `internal/ai/providers/openaicompat/provider_test.go`

Status: Accepted

These decisions were written as comments in `internal/ai/providers/openaicompat/provider_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestProvider_Complete_HappyPath spins up a local HTTP server that

Removed from `internal/ai/providers/openaicompat/provider_test.go:25`.

~~~~text
TestProvider_Complete_HappyPath spins up a local HTTP server that
mimics the OpenAI Chat Completions response shape and verifies the
provider both sends a well-formed request and correctly parses the
response — this is a real end-to-end exercise of the wire format,
not an assumption that the JSON shapes line up.
~~~~


## TestProvider_Complete_JSONModeWithoutSystemPrompt verifies the

Removed from `internal/ai/providers/openaicompat/provider_test.go:81`.

~~~~text
TestProvider_Complete_JSONModeWithoutSystemPrompt verifies the
JSON-mode instruction still gets attached even when the caller
passed no System prompt at all — this exercises a real edge case
(not just the common path where System is always set).
~~~~


## With no System prompt supplied, the JSON-mode instruction should

Removed from `internal/ai/providers/openaicompat/provider_test.go:110`.

~~~~text
With no System prompt supplied, the JSON-mode instruction should
still have been added as its own system message rather than
silently dropped.
~~~~


## TestProvider_Complete_QwenDisablesThinking guards against the bug

Removed from `internal/ai/providers/openaicompat/provider_test.go:118`.

~~~~text
TestProvider_Complete_QwenDisablesThinking guards against the bug
where DashScope/Qwen3's default enable_thinking=true silently ate
into MaxTokens with hidden reasoning tokens, truncating visible
JSON-mode output (e.g. Dev Console balance/weekly reports) well
before MaxTokens should have been exhausted. Qwen must always send
enable_thinking:false; every other OpenAI-compatible provider must
never send the field at all, since they don't recognize it.
~~~~
