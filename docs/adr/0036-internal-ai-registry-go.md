# ADR 0036: Decisions from `internal/ai/registry.go`

Status: Accepted

These decisions were written as comments in `internal/ai/registry.go` and moved here so the source no longer carries them. Each entry is the original comment.

## SetOrder fixes the fallback order explicitly. Names not registered

Removed from `internal/ai/registry.go:35`.

~~~~text
SetOrder fixes the fallback order explicitly. Names not registered
are ignored at resolution time rather than erroring, so config typos
degrade gracefully instead of crashing the game server.
~~~~


## mockProviderName must match mock.Provider.Name() in

Removed from `internal/ai/registry.go:44`.

~~~~text
mockProviderName must match mock.Provider.Name() in
internal/ai/providers/mock. Duplicated here as a plain string
(rather than importing that package, which internal/ai itself must
not depend on per this package's layering) so Ordered can enforce
ADR-003's invariant in code.
~~~~


## AllNames returns every registered provider's name, regardless of

Removed from `internal/ai/registry.go:51`.

~~~~text
AllNames returns every registered provider's name, regardless of
Available(), in no particular order. Unlike Ordered() (which is
deliberately scoped to "what Service.Complete would actually try
right now"), this exists for diagnostics that need to see and probe
a provider even when it's reporting itself unavailable — see
Service.ProbeAllProviders.
~~~~


## Ordered returns the currently-available providers in fallback order,

Removed from `internal/ai/registry.go:67`.

~~~~text
Ordered returns the currently-available providers in fallback order,
with one hard rule enforced regardless of configured order: the
provider named "mock" is always placed last.

Confirmed bug (2026-07-16), not hypothetical: Config's defaults are
AI_DEFAULT_PROVIDER="mock" and AI_FALLBACK_PROVIDERS="mock", so
mock previously ended up first in r.order. Since mock.Available()
is always true and mock.Complete() never returns an error, the
Service.Complete fallback loop returned immediately after mock every
time — real providers registered afterward (confirmed live: Qwen
and Gemini, both reporting Available()==true with valid API keys
configured) were never reached at all. ADR-003 always documented
"mock is always last" as the intent, but nothing in this function
previously enforced it — it only happened to be true when whoever
deployed the service also happened to list real providers before
"mock" in AI_FALLBACK_PROVIDERS, which nobody had been told to do.
This function now makes that invariant true unconditionally, so a
correctly-configured API key works the moment it's set, with zero
additional AI_DEFAULT_PROVIDER/AI_FALLBACK_PROVIDERS configuration
required.
~~~~


## Any registered provider not mentioned in order is appended next,

Removed from `internal/ai/registry.go:114`.

~~~~text
Any registered provider not mentioned in order is appended next,
so a newly-registered provider is never silently unreachable.
~~~~
