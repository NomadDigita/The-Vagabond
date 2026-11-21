# ADR 0023: Decisions from `internal/ai/jsonrecovery.go`

Status: Accepted

These decisions were written as comments in `internal/ai/jsonrecovery.go` and moved here so the source no longer carries them. Each entry is the original comment.

## ExtractJSONObject recovers a single JSON object embedded in llmText,

Removed from `internal/ai/jsonrecovery.go:5`.

~~~~text
ExtractJSONObject recovers a single JSON object embedded in llmText,
tolerating three real-world LLM output quirks that survive even an
explicit "respond with a single JSON object and nothing else"
instruction (see every provider's JSONMode handling):

 1. A markdown code fence wrapped around the object
    ("```json\n{...}\n```" or bare "```{...}```").
 2. Leading and/or trailing prose around the object
    ("Here's the analysis:\n{...}" or "{...}\n\nLet me know if you
    want more detail.") — increasingly common with reasoning-style
    models that narrate before/after the structured answer despite
    being told not to.
 3. Braces or quote characters that appear *inside* string values
    (e.g. a summary that itself mentions "{level}") — handled by
    tracking string/escape state while scanning for the matching
    closing brace, so those don't fool the object boundary.

It returns the recovered `{...}` substring and true, or ("", false)
if llmText contains no recognizable JSON object at all — the
signal callers use to fall back to displaying llmText as plain
text (see every package's ParseRecommendation).

This does not itself guarantee the substring is valid JSON — call
json.Unmarshal (optionally after SanitizeJSONControlChars) on the
result and be ready for that to still fail on a genuinely malformed
response.
~~~~


## Ran off the end without the braces balancing — either

Removed from `internal/ai/jsonrecovery.go:69`.

~~~~text
Ran off the end without the braces balancing — either
llmText was truncated mid-object (e.g. hit MaxTokens) or
start pointed at a '{' that was never a real JSON object to
begin with. Either way, there's no complete object to return.
~~~~


## WasTruncated reports whether text contains what looks like a JSON

Removed from `internal/ai/jsonrecovery.go:76`.

~~~~text
WasTruncated reports whether text contains what looks like a JSON
object whose opening brace was found but whose braces never
balanced by the end of the string — the signature of a response cut
off mid-object because the provider hit its MaxTokens limit, as
opposed to a response that never contained a JSON object at all
(e.g. a plain-prose refusal or an off-topic reply). Callers use
this — after ExtractJSONObject has already returned found=false —
to show players a more specific and more actionable message ("the
AI's response got cut off") than the generic parse-failure fallback
text. See ADR-016 in PROJECT_MASTER_PLAN.md.

This intentionally re-walks the same string/escape-aware brace
scan as ExtractJSONObject rather than sharing state with it, since
the two are called independently (only on the already-confirmed
found=false path) and keeping WasTruncated a standalone read-only
scan makes it trivial to reason about and unit test in isolation.
~~~~


## Braces balanced — ExtractJSONObject would have

Removed from `internal/ai/jsonrecovery.go:125`.

~~~~text
Braces balanced — ExtractJSONObject would have
found and returned this object, so whatever made
the caller's parse fail wasn't truncation.
~~~~


## IsTruncatedStopReason reports whether a provider's raw stop/finish

Removed from `internal/ai/jsonrecovery.go:138`.

~~~~text
IsTruncatedStopReason reports whether a provider's raw stop/finish
reason indicates the completion was cut off because it hit its
token budget, as opposed to stopping normally or being blocked for
some other reason. This is a stronger signal than WasTruncated's
brace-balance heuristic: WasTruncated can only detect truncation
inside a JSON object that had already started (it returns false
for prose cut off before any '{', or for a truncated string value
that happens to leave braces balanced by coincidence), while every
provider reports this directly. Callers should check both — see
each package's ParseRecommendation, which now takes stopReason as
well as the raw text — since a provider that omits/misreports its
finish reason still falls back to the brace scan.

Recognized values, one per provider (internal/ai/providers/*):
OpenAI-compatible (Qwen/DeepSeek/Grok/etc.) and Ollama both use
"length"; Anthropic uses "max_tokens"; Gemini uses "MAX_TOKENS".
The mock provider always reports "end_turn" and never matches.
~~~~


## SanitizeJSONControlChars repairs the single most common way a

Removed from `internal/ai/jsonrecovery.go:164`.

~~~~text
SanitizeJSONControlChars repairs the single most common way a
model's otherwise-valid-looking JSON fails Go's strict
encoding/json parser: a raw, unescaped control character (newline,
carriage return, or tab) left inside a string value instead of
being escaped as \n / \r / \t. Most "relaxed" JSON readers accept
this; Go's does not.

Only bytes that are actually inside a string literal (per the same
string/escape tracking as ExtractJSONObject) are touched — control
characters used purely as formatting whitespace between JSON tokens
are left alone.
~~~~
