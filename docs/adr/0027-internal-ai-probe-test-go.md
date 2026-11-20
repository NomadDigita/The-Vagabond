# ADR 0027: Decisions from `internal/ai/probe_test.go`

Status: Accepted

These decisions were written as comments in `internal/ai/probe_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestProbeAllProviders_ReportsRealErrorText is the core regression

Removed from `internal/ai/probe_test.go:8`.

~~~~text
TestProbeAllProviders_ReportsRealErrorText is the core regression
this file exists to guard: a provider that IS configured
(Available()==true) but fails on the actual call must surface its
exact error text through ProbeResult, not just "unavailable" — this
is precisely the distinction /ai_status's old Providers list could
never make, which is why an account-side quota/access error looked
identical to a code bug across several past investigation sessions.
~~~~


## TestProbeAllProviders_UnavailableProviderNotCalled confirms a

Removed from `internal/ai/probe_test.go:47`.

~~~~text
TestProbeAllProviders_UnavailableProviderNotCalled confirms a
provider with no key configured is reported as "not configured"
without ever invoking Complete (which would likely panic/error on
most real provider implementations given no credentials).
~~~~


## TestProbeAllProviders_RecordsCostOnSuccess confirms a successful

Removed from `internal/ai/probe_test.go:78`.

~~~~text
TestProbeAllProviders_RecordsCostOnSuccess confirms a successful
probe still records its (tiny) cost against the global budget, so
repeated /ai_probe runs can't bypass daily spend tracking.
~~~~


## TestProbeAllProviders_SkipsWhenGlobalBudgetExhausted confirms the

Removed from `internal/ai/probe_test.go:96`.

~~~~text
TestProbeAllProviders_SkipsWhenGlobalBudgetExhausted confirms the
probe respects the same daily budget Service.Complete does, rather
than being a way to spend past the cap.
~~~~
