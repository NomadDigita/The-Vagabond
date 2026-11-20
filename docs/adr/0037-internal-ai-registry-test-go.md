# ADR 0037: Decisions from `internal/ai/registry_test.go`

Status: Accepted

These decisions were written as comments in `internal/ai/registry_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## fakeNamedProvider is a minimal Provider for registry-ordering tests

Removed from `internal/ai/registry_test.go:10`.

~~~~text
fakeNamedProvider is a minimal Provider for registry-ordering tests
(fakeProvider in service_test.go also exists, but that one is
scoped to Service-level fallback behavior; this one is deliberately
separate and simpler, scoped only to Registry.Ordered()).
~~~~


## TestRegistry_MockIsAlwaysLast is a direct regression test for a

Removed from `internal/ai/registry_test.go:25`.

~~~~text
TestRegistry_MockIsAlwaysLast is a direct regression test for a
confirmed real bug (2026-07-16): Config's defaults
(AI_DEFAULT_PROVIDER="mock", AI_FALLBACK_PROVIDERS="mock") placed
"mock" first in the configured order. Because mock is always
Available() and its Complete never errors, Service.Complete's
fallback loop returned immediately after mock every time — real,
available providers (confirmed live: Qwen and Gemini, both with
valid API keys configured) were never reached at all, despite
/ai_status correctly reporting them as available.

This test constructs the exact failure condition (mock listed
first in order, alongside other available providers) and asserts
mock is NOT first in the result — it must be moved to the end
regardless of its position in the configured order.
~~~~


## Reproduce the exact real-world default: mock listed first.

Removed from `internal/ai/registry_test.go:48`.

~~~~text
Reproduce the exact real-world default: mock listed first.
~~~~


## TestRegistry_UnlistedProviderStillReachable confirms a provider

Removed from `internal/ai/registry_test.go:80`.

~~~~text
TestRegistry_UnlistedProviderStillReachable confirms a provider
registered but not mentioned in SetOrder is still returned (just
after the explicitly-ordered ones and before mock).
~~~~
