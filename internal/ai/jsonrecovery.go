package ai

import "strings"

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

	return "", false
}

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
				return false
			}
		}
	}

	return depth > 0
}

func IsTruncatedStopReason(stopReason string) bool {
	switch strings.ToLower(stopReason) {
	case "length", "max_tokens", "max_tokens_exceeded":
		return true
	default:
		return false
	}
}

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
