package ai_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

func TestHTMLQuote(t *testing.T) {
	got := ai.HTMLQuote("hello")
	want := "<blockquote>hello</blockquote>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLExpandableQuote(t *testing.T) {
	got := ai.HTMLExpandableQuote("a long narrative")
	want := "<blockquote expandable>a long narrative</blockquote>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLStrike(t *testing.T) {
	got := ai.HTMLStrike("50")
	want := "<s>50</s>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLTable_BasicAlignment(t *testing.T) {
	out := ai.HTMLTable(
		[]string{"Unit", "Win%"},
		[][]string{
			{"soldiers", "50"},
			{"bombers", "100"},
		},
	)
	if !strings.HasPrefix(out, "<pre>") || !strings.HasSuffix(out, "</pre>") {
		t.Fatalf("expected table wrapped in <pre>, got: %q", out)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(out, "<pre>"), "</pre>")
	lines := strings.Split(inner, "\n")
	if len(lines) != 4 { // header, separator, 2 rows
		t.Fatalf("expected 4 lines (header+sep+2 rows), got %d: %q", len(lines), inner)
	}
	// Every line should be the same rendered width, i.e. columns line up.
	width := len([]rune(lines[0]))
	for i, l := range lines {
		if n := len([]rune(l)); n != width {
			t.Errorf("line %d (%q) has width %d, want %d - columns don't line up", i, l, n, width)
		}
	}
}

func TestHTMLTable_MalformedRowSkipped(t *testing.T) {
	out := ai.HTMLTable(
		[]string{"Unit", "Win%"},
		[][]string{
			{"soldiers", "50"},
			{"bad_row_only_one_col"}, // wrong length - must be dropped, not misalign the table
			{"bombers", "100"},
		},
	)
	if strings.Count(out, "\n") != 3 { // header + separator + 2 valid rows = 3 newlines
		t.Errorf("expected malformed row to be dropped, got: %q", out)
	}
	if strings.Contains(out, "bad_row_only_one_col") {
		t.Errorf("malformed row should not appear in output: %q", out)
	}
}

func TestHTMLTable_LongCellClipped(t *testing.T) {
	longVal := strings.Repeat("x", 40)
	out := ai.HTMLTable([]string{"Name"}, [][]string{{longVal}})
	if strings.Contains(out, longVal) {
		t.Errorf("expected long cell to be clipped to stay mobile-readable, got: %q", out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("expected an ellipsis marker on the clipped cell, got: %q", out)
	}
}
