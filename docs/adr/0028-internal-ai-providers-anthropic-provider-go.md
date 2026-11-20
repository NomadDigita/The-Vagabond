# ADR 0028: Decisions from `internal/ai/providers/anthropic/provider.go`

Status: Accepted

These decisions were written as comments in `internal/ai/providers/anthropic/provider.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package anthropic implements ai.Provider against Anthropic's public

Removed from `internal/ai/providers/anthropic/provider.go:1`.

~~~~text
Package anthropic implements ai.Provider against Anthropic's public
Messages API (https://api.anthropic.com/v1/messages). It has zero
dependency on internal/bot or internal/engine.
~~~~


## New builds an Anthropic provider. apiKey may be empty (e.g. read

Removed from `internal/ai/providers/anthropic/provider.go:28`.

~~~~text
New builds an Anthropic provider. apiKey may be empty (e.g. read
from an unset environment variable); Available() will correctly
report false in that case rather than the caller needing to check.
~~~~


## Anthropic has no bare "tool" role at this layer; encode

Removed from `internal/ai/providers/anthropic/provider.go:117`.

~~~~text
Anthropic has no bare "tool" role at this layer; encode
tool results as a user turn labelled by name so context
is preserved without requiring full tool_result blocks
for this foundational version.
~~~~


## system is sent via the top-level System field

Removed from `internal/ai/providers/anthropic/provider.go:124`.

~~~~text
system is sent via the top-level System field
~~~~
