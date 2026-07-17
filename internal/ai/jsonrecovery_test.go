package ai

import "testing"

func TestExtractJSONObject_BareValidJSON(t *testing.T) {
	in := `{"summary": "ok"}`
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != in {
		t.Errorf("expected unchanged passthrough, got: %s", got)
	}
}

func TestExtractJSONObject_MarkdownFence(t *testing.T) {
	in := "```json\n{\"summary\": \"ok\"}\n```"
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != `{"summary": "ok"}` {
		t.Errorf("expected fence stripped, got: %q", got)
	}
}

func TestExtractJSONObject_LeadingProse(t *testing.T) {
	in := `Here is the analysis you asked for:` + "\n" + `{"summary": "ok"}`
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != `{"summary": "ok"}` {
		t.Errorf("expected leading prose stripped, got: %q", got)
	}
}

func TestExtractJSONObject_TrailingProse(t *testing.T) {
	in := `{"summary": "ok"}` + "\n\n" + `Let me know if you'd like more detail!`
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != `{"summary": "ok"}` {
		t.Errorf("expected trailing prose stripped, got: %q", got)
	}
}

func TestExtractJSONObject_LeadingAndTrailingProse(t *testing.T) {
	in := `Sure! ` + `{"summary": "ok"}` + ` Hope that helps.`
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != `{"summary": "ok"}` {
		t.Errorf("expected both sides stripped, got: %q", got)
	}
}

func TestExtractJSONObject_BraceInsideStringValue(t *testing.T) {
	in := `{"summary": "reaches level {8} soon"}`
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != in {
		t.Errorf("expected brace-in-string not to confuse depth tracking, got: %q", got)
	}
}

func TestExtractJSONObject_EscapedQuoteInsideString(t *testing.T) {
	in := `{"summary": "she said \"go\" and left"}`
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != in {
		t.Errorf("expected escaped quote handled correctly, got: %q", got)
	}
}

func TestExtractJSONObject_NestedObject(t *testing.T) {
	in := `{"a": {"b": 1}, "c": 2}`
	got, found := ExtractJSONObject(in)
	if !found {
		t.Fatal("expected object to be found")
	}
	if got != in {
		t.Errorf("expected full nested object returned, got: %q", got)
	}
}

func TestExtractJSONObject_NoJSONAtAll(t *testing.T) {
	_, found := ExtractJSONObject("I'm sorry, I can't help with that right now.")
	if found {
		t.Fatal("expected no object to be found in plain prose")
	}
}

func TestExtractJSONObject_TruncatedNeverCloses(t *testing.T) {
	// Simulates a response cut off by MaxTokens mid-object.
	_, found := ExtractJSONObject(`{"summary": "partial output that just stops`)
	if found {
		t.Fatal("expected truncated/never-closed object to report not found")
	}
}

func TestSanitizeJSONControlChars_EscapesRawNewlineInString(t *testing.T) {
	in := "{\"summary\": \"line one\nline two\"}"
	out := SanitizeJSONControlChars(in)
	want := `{"summary": "line one\nline two"}`
	if out != want {
		t.Errorf("expected raw newline escaped, got: %q want: %q", out, want)
	}
}

func TestSanitizeJSONControlChars_LeavesFormattingWhitespaceAlone(t *testing.T) {
	in := "{\n  \"summary\": \"ok\"\n}"
	out := SanitizeJSONControlChars(in)
	if out != in {
		t.Errorf("expected whitespace between tokens untouched, got: %q", out)
	}
}

func TestSanitizeJSONControlChars_HandlesTabAndCarriageReturn(t *testing.T) {
	in := "{\"summary\": \"a\tb\rc\"}"
	out := SanitizeJSONControlChars(in)
	want := `{"summary": "a\tb\rc"}`
	if out != want {
		t.Errorf("got: %q want: %q", out, want)
	}
}

func TestSanitizeJSONControlChars_DoesNotDoubleEscapeAlreadyEscaped(t *testing.T) {
	in := `{"summary": "already has \n an escaped newline"}`
	out := SanitizeJSONControlChars(in)
	if out != in {
		t.Errorf("expected already-escaped sequence left alone, got: %q", out)
	}
}
