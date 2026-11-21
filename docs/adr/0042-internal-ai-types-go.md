# ADR 0042: Decisions from `internal/ai/types.go`

Status: Accepted

These decisions were written as comments in `internal/ai/types.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package ai is the provider-agnostic AI Foundation for The Vagabond.

Removed from `internal/ai/types.go:1`.

~~~~text
Package ai is the provider-agnostic AI Foundation for The Vagabond.

This package (Phase A of the AI Systems Roadmap) never talks to game
tables directly and never imports internal/bot or internal/engine.
Every future AI capability (Planet Governor, Fleet Commander, Economy
Advisor, Research Planner, Battle Analyst, Guild Assistant, Dynamic
Galaxy, NPC Intelligence, Developer Console) is expected to depend on
this package, never the other way around. See PROJECT_MASTER_PLAN.md
for the full roadmap and integration notes.
~~~~


## Content is plain text. Structured tool results should be

Removed from `internal/ai/types.go:27`.

~~~~text
Content is plain text. Structured tool results should be
JSON-encoded into Content by the caller.
~~~~


## ToolCallID links a RoleTool message back to the ToolCall.ID

Removed from `internal/ai/types.go:30`.

~~~~text
ToolCallID links a RoleTool message back to the ToolCall.ID
that produced it. Empty for non-tool messages.
~~~~


## ToolDefinition describes a callable tool in JSON-Schema form, mirroring

Removed from `internal/ai/types.go:38`.

~~~~text
ToolDefinition describes a callable tool in JSON-Schema form, mirroring
the shape used by every major provider (Anthropic, OpenAI, Gemini).
~~~~


## CompletionRequest is the provider-agnostic request shape. Every

Removed from `internal/ai/types.go:60`.

~~~~text
CompletionRequest is the provider-agnostic request shape. Every
Provider implementation is responsible for translating this into its
own wire format.
~~~~


## Model is a provider-specific model identifier. Callers should

Removed from `internal/ai/types.go:64`.

~~~~text
Model is a provider-specific model identifier. Callers should
generally leave this empty and let the Service fill in the
configured default for the routed provider.
~~~~


## JSONMode asks the provider to constrain output to a single

Removed from `internal/ai/types.go:75`.

~~~~text
JSONMode asks the provider to constrain output to a single
JSON object (used by structured-response callers such as the
Developer Console and Battle Analyst).
~~~~


## Feature identifies which game subsystem is calling (e.g.

Removed from `internal/ai/types.go:80`.

~~~~text
Feature identifies which game subsystem is calling (e.g.
"planet_governor", "fleet_commander"). Used for permissioning,
cost attribution, caching, and observability. Required.
~~~~


## UserID is the Telegram ID of the player this request is made

Removed from `internal/ai/types.go:85`.

~~~~text
UserID is the Telegram ID of the player this request is made
on behalf of. Zero means a system/background request (e.g. the
Dynamic Galaxy director), which bypasses per-user budgets but
is still subject to the global daily budget.
~~~~


## CostUSD is filled in by the Service after pricing, not by the

Removed from `internal/ai/types.go:100`.

~~~~text
CostUSD is filled in by the Service after pricing, not by the
Provider itself.
~~~~


## Provider is implemented by every AI backend (Anthropic, OpenAI,

Removed from `internal/ai/types.go:108`.

~~~~text
Provider is implemented by every AI backend (Anthropic, OpenAI,
Gemini, Qwen, Grok, DeepSeek, Ollama, a local mock, ...). Providers
must be safe for concurrent use.
~~~~


## Available reports whether the provider is currently usable

Removed from `internal/ai/types.go:116`.

~~~~text
Available reports whether the provider is currently usable
(e.g. has credentials configured). The registry skips
unavailable providers during fallback.
~~~~
