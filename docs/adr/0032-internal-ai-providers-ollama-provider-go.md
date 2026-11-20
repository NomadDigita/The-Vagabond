# ADR 0032: Decisions from `internal/ai/providers/ollama/provider.go`

Status: Accepted

These decisions were written as comments in `internal/ai/providers/ollama/provider.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package ollama implements ai.Provider against a self-hosted Ollama

Removed from `internal/ai/providers/ollama/provider.go:1`.

~~~~text
Package ollama implements ai.Provider against a self-hosted Ollama
instance's native API (POST {base_url}/api/chat). This is the path
toward running open-weight models under your own infrastructure
instead of paying a per-token API — see PROJECT_MASTER_PLAN.md for
an honest note on the compute requirements that implies (a capable
model needs real RAM/GPU; this provider does not change that).

Unlike every other provider in this codebase, Ollama needs no API
key — Available() only checks that a base URL is configured. Set
OLLAMA_BASE_URL (e.g. "http://localhost:11434" or the address of a
dedicated inference host) to enable it.
~~~~


## no trailing slash, e.g. "http://localhost:11434"

Removed from `internal/ai/providers/ollama/provider.go:29`.

~~~~text
no trailing slash, e.g. "http://localhost:11434"
~~~~


## New builds an Ollama provider. baseURL may be empty; Available()

Removed from `internal/ai/providers/ollama/provider.go:34`.

~~~~text
New builds an Ollama provider. baseURL may be empty; Available()
will correctly report false in that case (no key is ever required).
~~~~


## Local/self-hosted inference, especially on CPU, can be far

Removed from `internal/ai/providers/ollama/provider.go:43`.

~~~~text
Local/self-hosted inference, especially on CPU, can be far
slower than a hosted API — a longer timeout avoids treating
a slow-but-working reply as a hard failure.
~~~~


## object, not a JSON string (unlike OpenAI)

Removed from `internal/ai/providers/ollama/provider.go:67`.

~~~~text
object, not a JSON string (unlike OpenAI)
~~~~


## Ollama's equivalent of max_tokens

Removed from `internal/ai/providers/ollama/provider.go:79`.

~~~~text
Ollama's equivalent of max_tokens
~~~~


## same simplification as every other provider — see ADR-006

Removed from `internal/ai/providers/ollama/provider.go:131`.

~~~~text
same simplification as every other provider — see ADR-006
~~~~
