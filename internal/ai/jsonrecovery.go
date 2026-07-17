package ai

import "strings"

// ExtractJSONObject recovers a single JSON object embedded in llmText,
// tolerating three real-world LLM output quirks that survive even an
// explicit "respond with a single JSON object and nothing else"
// instruction (see every provider's JSONMode handling):
//
//  1. A markdown code fence wrapped around the object
//     ("```json\n{...}\n```" or bare "```{...}```").
//  2. Leading and/or trailing prose around the object
//     ("Here's the analysis:\n{...}" or "{...}\n\nLet me know if you
//     want more detail.") — increasingly common with reasoning-style
//     models that narrate before/after the structured answer despite
//     being told not to.
//  3. Braces or quote characters that appear *inside* string values
//     (e.g. a summary that itself mentions "{level}") — handled by
//     tracking string/escape state while scanning for the matching
//     closing brace, so those don't fool the object boundary.
//
// It returns the recovered `{...}` substring and true, or ("", false)
// if llmText contains no recognizable JSON object at all — the
// signal callers use to fall back to displaying llmText as plain
// text (see every package's ParseRecommendation).
//
// This does not itself guarantee the substring is valid JSON — call
// json.Unmarshal (optionally after SanitizeJSONControlChars) on the
// result and be ready for that to still fail on a genuinely malformed
// response.
func ExtractJSONObject(llmText string) (string, bool) {
	start := strings.IndexByte(llmText, '{')
	if start == -1 {
		return "", false
	}

	inString := false
	escaped := false
	depth := 0

	for i := start; i < len(llmText); i++ {
		c := llmText[i]

		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return llmText[start : i+1], true
			}
		}
	}

	// Ran off the end without the braces balancing — either
	// llmText was truncated mid-object (e.g. hit MaxTokens) or
	// start pointed at a '{' that was never a real JSON object to
	// begin with. Either way, there's no complete object to return.
	return "", false
}

// WasTruncated reports whether text contains what looks like a JSON
// object whose opening brace was found but whose braces never
// balanced by the end of the string — the signature of a response cut
// off mid-object because the provider hit its MaxTokens limit, as
// opposed to a response that never contained a JSON object at all
// (e.g. a plain-prose refusal or an off-topic reply). Callers use
// this — after ExtractJSONObject has already returned found=false —
// to show players a more specific and more actionable message ("the
// AI's response got cut off") than the generic parse-failure fallback
// text. See ADR-016 in PROJECT_MASTER_PLAN.md.
//
// This intentionally re-walks the same string/escape-aware brace
// scan as ExtractJSONObject rather than sharing state with it, since
// the two are called independently (only on the already-confirmed
// found=false path) and keeping WasTruncated a standalone read-only
// scan makes it trivial to reason about and unit test in isolation.
func WasTruncated(text string) bool {
	start := strings.IndexByte(text, '{')
	if start == -1 {
		return false
	}

	inString := false
	escaped := false
	depth := 0

	for i := start; i < len(text); i++ {
		c := text[i]

		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				// Braces balanced — ExtractJSONObject would have
				// found and returned this object, so whatever made
				// the caller's parse fail wasn't truncation.
				return false
			}
		}
	}

	// Ran off the end of the string still inside an unclosed object:
	// truncated mid-object.
	return depth > 0
}

// SanitizeJSONControlChars repairs the single most common way a
// model's otherwise-valid-looking JSON fails Go's strict
// encoding/json parser: a raw, unescaped control character (newline,
// carriage return, or tab) left inside a string value instead of
// being escaped as \n / \r / \t. Most "relaxed" JSON readers accept
// this; Go's does not.
//
// Only bytes that are actually inside a string literal (per the same
// string/escape tracking as ExtractJSONObject) are touched — control
// characters used purely as formatting whitespace between JSON tokens
// are left alone.
func SanitizeJSONControlChars(jsonText string) string {
	var b strings.Builder
	b.Grow(len(jsonText))

	inString := false
	escaped := false

	for i := 0; i < len(jsonText); i++ {
		c := jsonText[i]

		if inString {
			switch {
			case escaped:
				escaped = false
				b.WriteByte(c)
				continue
			case c == '\\':
				escaped = true
				b.WriteByte(c)
				continue
			case c == '"':
				inString = false
				b.WriteByte(c)
				continue
			case c == '\n':
				b.WriteString(`\n`)
				continue
			case c == '\r':
				b.WriteString(`\r`)
				continue
			case c == '\t':
				b.WriteString(`\t`)
				continue
			default:
				b.WriteByte(c)
				continue
			}
		}

		if c == '"' {
			inString = true
		}
		b.WriteByte(c)
	}

	return b.String()
}
