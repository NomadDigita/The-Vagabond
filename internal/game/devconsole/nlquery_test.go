package devconsole_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/devconsole"
)

func TestIsKnownIntent_AcceptsWhitelisted(t *testing.T) {
	for _, name := range []string{"new_players", "top_players", "active_users", "totals", "economy_snapshot", "combat_stats", "clan_stats", "world_state", "recent_news"} {
		if !devconsole.IsKnownIntent(name) {
			t.Errorf("expected %q to be a known intent", name)
		}
	}
}

func TestIsKnownIntent_RejectsUnknown(t *testing.T) {
	for _, name := range []string{"", "drop_table_users", "DELETE FROM users", "SELECT * FROM users", "arbitrary_sql", "shutdown_server"} {
		if devconsole.IsKnownIntent(name) {
			t.Errorf("expected %q to be rejected as an unknown intent", name)
		}
	}
}

func TestIntentDescriptions_ListsAllWhitelistedIntents(t *testing.T) {
	desc := devconsole.IntentDescriptions()
	for _, name := range []string{"new_players", "top_players", "active_users", "totals", "economy_snapshot", "combat_stats", "clan_stats", "world_state", "recent_news"} {
		if !strings.Contains(desc, name) {
			t.Errorf("expected intent descriptions to mention %q, got:\n%s", name, desc)
		}
	}
}

func TestFormatAnswerForTelegram_FallbackPath(t *testing.T) {
	rec := &devconsole.AnswerRecommendation{Answer: "raw text", FellBackToRawText: true}
	out := devconsole.FormatAnswerForTelegram(rec)
	if !strings.Contains(out, "Couldn't parse") || !strings.Contains(out, "raw text") {
		t.Errorf("expected fallback notice and raw text, got: %s", out)
	}
}

func TestFormatAnswerForTelegram_TruncatedPath(t *testing.T) {
	rec := &devconsole.AnswerRecommendation{
		Answer:            `{"answer": "cut off mid`,
		FellBackToRawText: true,
		Truncated:         true,
	}
	out := devconsole.FormatAnswerForTelegram(rec)
	if !strings.Contains(out, "cut off before it finished") {
		t.Errorf("expected truncation-specific message, got: %s", out)
	}
}

func TestFormatAnswerForTelegram_StructuredPath(t *testing.T) {
	rec := &devconsole.AnswerRecommendation{
		Answer:  "You had 12 new players this week, mostly in Asia.",
		Caveats: "This only counts in-game home continent, not real-world location.",
	}
	out := devconsole.FormatAnswerForTelegram(rec)
	for _, want := range []string{
		"AI DEVELOPER CONSOLE",
		"You had 12 new players this week, mostly in Asia.",
		"This only counts in-game home continent, not real-world location.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestClampDays_WithinBoundsUnchanged(t *testing.T) {
	if got := devconsole.ClampDays(14); got != 14 {
		t.Errorf("expected 14 unchanged, got %d", got)
	}
}

func TestClampDays_TooLowDefaultsToSeven(t *testing.T) {
	if got := devconsole.ClampDays(0); got != 7 {
		t.Errorf("expected 0 to default to 7, got %d", got)
	}
	if got := devconsole.ClampDays(-5); got != 7 {
		t.Errorf("expected -5 to default to 7, got %d", got)
	}
}

func TestClampDays_TooHighClampedToMax(t *testing.T) {
	if got := devconsole.ClampDays(99999); got != 90 {
		t.Errorf("expected an absurd value to clamp to 90, got %d", got)
	}
}

func TestClampLimit_WithinBoundsUnchanged(t *testing.T) {
	if got := devconsole.ClampLimit(10); got != 10 {
		t.Errorf("expected 10 unchanged, got %d", got)
	}
}

func TestClampLimit_TooLowDefaultsToFive(t *testing.T) {
	if got := devconsole.ClampLimit(0); got != 5 {
		t.Errorf("expected 0 to default to 5, got %d", got)
	}
}

func TestClampLimit_TooHighClampedToMax(t *testing.T) {
	if got := devconsole.ClampLimit(99999); got != 25 {
		t.Errorf("expected an absurd value to clamp to 25, got %d", got)
	}
}

func TestParseClassification_ValidJSON(t *testing.T) {
	raw := `{"intent": "top_players", "days": 0, "limit": 10}`
	ic := devconsole.ParseClassification(raw)
	if ic.FellBackToRawText {
		t.Fatalf("expected clean parse, got fallback")
	}
	if ic.Intent != "top_players" || ic.Limit != 10 {
		t.Errorf("unexpected classification: %+v", ic)
	}
}

func TestParseClassification_FallsBackOnGarbage(t *testing.T) {
	ic := devconsole.ParseClassification("I don't know what you mean")
	if !ic.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
}

// Critical safety test: even a maliciously-crafted "intent" value that
// looks like SQL or a destructive instruction must never be treated as
// valid — IsKnownIntent must reject it regardless of what
// ParseClassification returns, since parsing JSON successfully is not
// the same as the intent being safe to run.
func TestParseClassification_UnknownIntentStillRejectedByIsKnownIntent(t *testing.T) {
	raw := `{"intent": "DROP TABLE users; --", "days": 7, "limit": 5}`
	ic := devconsole.ParseClassification(raw)
	if ic.FellBackToRawText {
		t.Fatalf("expected this to parse as valid JSON (the danger is in the intent value, not the JSON syntax)")
	}
	if devconsole.IsKnownIntent(ic.Intent) {
		t.Fatalf("expected a SQL-injection-shaped intent value to be rejected by IsKnownIntent")
	}
}

func TestParseClassification_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n" + `{"intent": "totals", "days": 0, "limit": 0}` + "\n```"
	ic := devconsole.ParseClassification(raw)
	if ic.FellBackToRawText {
		t.Fatalf("expected fence to be stripped and JSON parsed, got fallback")
	}
}

// See ADR-016: a response cut off mid-object is distinguished from one
// that never contained JSON at all.
func TestParseClassification_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"intent": "top_players", "days": 7, "lim`
	ic := devconsole.ParseClassification(raw)
	if !ic.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !ic.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestParseAnswer_ValidJSON(t *testing.T) {
	raw := `{"answer": "12 new players joined this week.", "caveats": "small sample"}`
	rec := devconsole.ParseAnswer(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean parse, got fallback")
	}
	if rec.Answer != "12 new players joined this week." {
		t.Errorf("unexpected answer: %q", rec.Answer)
	}
}

func TestParseAnswer_FallsBackOnGarbage(t *testing.T) {
	raw := "there were some new players I think"
	rec := devconsole.ParseAnswer(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
	if rec.Answer != raw {
		t.Errorf("expected raw text preserved, got %q", rec.Answer)
	}
	if rec.Truncated {
		t.Errorf("expected Truncated=false for prose with no JSON object at all")
	}
}

func TestParseAnswer_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"answer": "There were 12 new players this week, most of whom joined from the Asia contin`
	rec := devconsole.ParseAnswer(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !rec.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestBuildClassificationUserPrompt_ContainsQuestion(t *testing.T) {
	p := devconsole.BuildClassificationUserPrompt("how many new players this week?")
	if !strings.Contains(p, "how many new players this week?") {
		t.Errorf("expected prompt to contain the admin's question, got:\n%s", p)
	}
}

func TestClassificationSystemPromptText_ListsWhitelist(t *testing.T) {
	p := devconsole.ClassificationSystemPromptText()
	for _, name := range []string{"new_players", "top_players", "world_state"} {
		if !strings.Contains(p, name) {
			t.Errorf("expected classification system prompt to mention %q, got:\n%s", name, p)
		}
	}
}

func TestBuildAnswerUserPrompt_ContainsQuestionAndData(t *testing.T) {
	p := devconsole.BuildAnswerUserPrompt("how many new players?", "New players in the last 7 day(s): 12 total")
	if !strings.Contains(p, "how many new players?") || !strings.Contains(p, "12 total") {
		t.Errorf("expected prompt to contain both question and data, got:\n%s", p)
	}
}
