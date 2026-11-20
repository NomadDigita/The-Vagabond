# ADR 0030: Decisions from `internal/ai/providers/gemini/provider_test.go`

Status: Accepted

These decisions were written as comments in `internal/ai/providers/gemini/provider_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestProvider_Complete_HappyPath exercises the real Gemini wire

Removed from `internal/ai/providers/gemini/provider_test.go:29`.

~~~~text
TestProvider_Complete_HappyPath exercises the real Gemini wire
format end-to-end against a local server standing in for
generativelanguage.googleapis.com, using the provider's overridable
BaseURL field.
~~~~


## Verify Gemini-specific role mapping: RoleAssistant must become

Removed from `internal/ai/providers/gemini/provider_test.go:74`.

~~~~text
Verify Gemini-specific role mapping: RoleAssistant must become
"model", not "assistant" — this is the whole reason Gemini has
its own provider instead of reusing openaicompat.
~~~~


## Verify systemInstruction was set as its own top-level field

Removed from `internal/ai/providers/gemini/provider_test.go:86`.

~~~~text
Verify systemInstruction was set as its own top-level field
(not folded into contents), per Gemini's actual API shape.
~~~~


## TestProvider_Complete_RetriesOnQuotaExhaustedToNextModel confirms the

Removed from `internal/ai/providers/gemini/provider_test.go:159`.

~~~~text
TestProvider_Complete_RetriesOnQuotaExhaustedToNextModel confirms the
2026-08-02 fix: a RESOURCE_EXHAUSTED response for the primary model
(Google's quota-hit status, confirmed live via Render logs) should
make Complete retry against the next entry in ModelFallbacks instead
of failing the whole provider outright - free-tier quota is tracked
per model, not per account, so a different model is frequently still
usable.
~~~~


## Path shape is "/{model}:generateContent" per completeWithModel's

Removed from `internal/ai/providers/gemini/provider_test.go:170`.

~~~~text
Path shape is "/{model}:generateContent" per completeWithModel's
endpoint format.
~~~~


## TestProvider_Complete_NonRetryableErrorSkipsFallbackModels confirms

Removed from `internal/ai/providers/gemini/provider_test.go:213`.

~~~~text
TestProvider_Complete_NonRetryableErrorSkipsFallbackModels confirms
the other half of the same fix: an error that isn't quota/overload
specific (e.g. a malformed request, or a bad API key) should fail
immediately rather than burning a request against every fallback
model too - that error will just as surely repeat on every model, so
internal/ai.Service's provider-level fallback (moving on to the next
*provider* entirely) should get the chance sooner, not later.
~~~~


## TestProvider_Complete_RetriesOnModelNotFoundToNextModel is a direct

Removed from `internal/ai/providers/gemini/provider_test.go:245`.

~~~~text
TestProvider_Complete_RetriesOnModelNotFoundToNextModel is a direct
regression test for the confirmed live bug (2026-08-05, via
/ai_probe): gemini-2.5-flash-lite, a previous default fallback
model, was retired by Google and started returning NOT_FOUND ("model
... is no longer available to new users"). Before this fix, a
retired model anywhere in ModelFallbacks permanently blocked every
model listed after it, since NOT_FOUND wasn't treated as
retryable-to-next-model. A stale/retired fallback entry should be
skipped over, not treated the same as a genuine bad-key/malformed-
request failure.
~~~~
