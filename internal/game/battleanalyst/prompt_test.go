package battleanalyst_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/battleanalyst"
)

func sampleSnapshot() battleanalyst.Snapshot {
	return battleanalyst.Snapshot{
		EncampmentID: "abc-123",
		Name:         "Fort Wasteland",
		Level:        6,
		AsAttacker: battleanalyst.RaidStats{
			TotalRaids: 10, ApparentWins: 7, TotalLosses: 40, AverageLosses: 4.0, TotalStolenValue: 12000,
		},
		AsDefender: battleanalyst.RaidStats{
			TotalRaids: 5, ApparentWins: 1, TotalLosses: 60, AverageLosses: 12.0,
		},
		Arena: battleanalyst.ArenaStats{Wins: 3, Losses: 9},
	}
}

func TestBuildUserPrompt_IsDeterministic(t *testing.T) {
	s := sampleSnapshot()
	p1 := battleanalyst.BuildUserPrompt(s)
	p2 := battleanalyst.BuildUserPrompt(s)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input")
	}
}

func TestBuildUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := battleanalyst.BuildUserPrompt(sampleSnapshot())
	for _, want := range []string{
		"Fort Wasteland", "10 raids, 7 apparent wins",
		"5 raids, 1 apparent wins", "3 wins, 9 losses",
		"World Boss engagement history is not available",
		"key_patterns",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_EmptyHistoryPaths(t *testing.T) {
	s := battleanalyst.Snapshot{EncampmentID: "x", Name: "Bare Camp"}
	p := battleanalyst.BuildUserPrompt(s)
	for _, want := range []string{"No recorded raids.", "No recorded arena battles."} {
		if !strings.Contains(p, want) {
			t.Errorf("expected empty-history message %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_DefenderStatsOmitStolenValueLine(t *testing.T) {
	// Defender-side stats have no meaningful "stolen value" concept
	// (that's an attacker-only figure) — confirm the prompt doesn't
	// fabricate one for the defender section.
	s := sampleSnapshot()
	p := battleanalyst.BuildUserPrompt(s)
	attackerIdx := strings.Index(p, "RAIDS AS ATTACKER")
	defenderIdx := strings.Index(p, "RAIDS AS DEFENDER")
	arenaIdx := strings.Index(p, "ARENA BATTLES")
	defenderSection := p[defenderIdx:arenaIdx]
	if !strings.Contains(p[attackerIdx:defenderIdx], "Total resources stolen") {
		t.Errorf("expected attacker section to include stolen-value line")
	}
	if strings.Contains(defenderSection, "Total resources stolen") {
		t.Errorf("expected defender section to omit stolen-value line, got:\n%s", defenderSection)
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Strong attacker, weak defender.", "key_patterns": [{"observation": "defense losses high", "evidence": "12 avg losses/raid", "suggestion": "reinforce garrison"}], "recommended_focus": "shore up defense", "notes": "small arena sample"}`
	rec := battleanalyst.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if len(rec.KeyPatterns) != 1 || rec.KeyPatterns[0].Suggestion != "reinforce garrison" {
		t.Errorf("unexpected patterns: %+v", rec.KeyPatterns)
	}
	if rec.RecommendedFocus != "shore up defense" {
		t.Errorf("unexpected focus: %q", rec.RecommendedFocus)
	}
}

func TestParseRecommendation_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n" + `{"summary": "ok", "key_patterns": [], "recommended_focus": "", "notes": ""}` + "\n```"
	rec := battleanalyst.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected fence to be stripped and JSON parsed, got fallback")
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "you lose a lot honestly"
	rec := battleanalyst.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
	if rec.Summary != raw {
		t.Errorf("expected raw text preserved in Summary, got %q", rec.Summary)
	}
	if rec.Truncated {
		t.Errorf("expected Truncated=false for prose with no JSON object at all")
	}
}

func TestParseRecommendation_TrailingProseAroundJSON(t *testing.T) {
	raw := `{"summary": "Strong attacker.", "key_patterns": []}` +
		"\n\nHope that helps!"
	rec := battleanalyst.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected trailing prose to be discarded, not trigger fallback")
	}
}

func TestParseRecommendation_RawNewlineInsideStringValue(t *testing.T) {
	raw := "{\"summary\": \"Line one\nline two\", \"key_patterns\": []}"
	rec := battleanalyst.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected sanitized control chars to allow parsing, got fallback")
	}
}

// See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
// is distinguished from one that never contained JSON at all.
func TestParseRecommendation_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"summary": "This player's defense is weak because their garrison`
	rec := battleanalyst.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !rec.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestFormatForTelegram_FallbackPath(t *testing.T) {
	rec := &battleanalyst.Recommendation{Summary: "raw text", FellBackToRawText: true}
	out := battleanalyst.FormatForTelegram(rec)
	if !strings.Contains(out, "Couldn't parse") || !strings.Contains(out, "raw text") {
		t.Errorf("expected fallback notice and raw text, got: %s", out)
	}
}

func TestFormatForTelegram_TruncatedPath(t *testing.T) {
	rec := &battleanalyst.Recommendation{
		Summary:           `{"summary": "cut off mid`,
		FellBackToRawText: true,
		Truncated:         true,
	}
	out := battleanalyst.FormatForTelegram(rec)
	if !strings.Contains(out, "cut off before it finished") {
		t.Errorf("expected truncation-specific message, got: %s", out)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := &battleanalyst.Recommendation{
		Summary: "Strong attacker, weak defender.",
		KeyPatterns: []battleanalyst.Pattern{
			{Observation: "defense losses high", Evidence: "12 avg losses/raid", Suggestion: "reinforce garrison"},
		},
		RecommendedFocus: "shore up defense",
		Notes:            "small arena sample",
	}
	out := battleanalyst.FormatForTelegram(rec)
	for _, want := range []string{
		"AI BATTLE ANALYST", "Strong attacker, weak defender.",
		"defense losses high", "reinforce garrison",
		"shore up defense", "small arena sample",
		"nothing has been changed automatically",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
