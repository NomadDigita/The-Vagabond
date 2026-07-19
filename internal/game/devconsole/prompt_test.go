package devconsole_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/devconsole"
)

func sampleSnapshot() devconsole.Snapshot {
	return devconsole.Snapshot{
		WindowDays: 7,
		NewPlayers: []devconsole.NewPlayer{
			{Username: "newbie99", FirstName: "Alex", JoinedAt: "2026-07-15 14:32 UTC", HomeContinent: "Asia"},
			{Username: "", FirstName: "Sam", JoinedAt: "2026-07-16 09:10 UTC", HomeContinent: ""},
		},
		NewPlayerCount:    2,
		TotalUsersAllTime: 340,
		TopPlayers: []devconsole.TopPlayer{
			{Name: "Fort Wasteland", Score: 88000, Level: 22},
			{Name: "Iron Vultures HQ", Score: 76000, Level: 19},
		},
		ActiveUserCount: 120,
		RecentWorldNews: []string{
			"⚡ SOLAR FLARE DETECTED: Intense electromagnetic wave spikes registered over Asia.",
		},
	}
}

func TestBuildUserPrompt_IsDeterministic(t *testing.T) {
	s := sampleSnapshot()
	p1 := devconsole.BuildUserPrompt(s)
	p2 := devconsole.BuildUserPrompt(s)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input")
	}
}

func TestBuildUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := devconsole.BuildUserPrompt(sampleSnapshot())
	for _, want := range []string{
		"last 7 day(s)",
		"Total users all-time: 340", "Active in window (last_active): 120",
		"NEW PLAYERS THIS WINDOW: 2 total",
		"Alex (@newbie99)", "home: Asia",
		"Sam, joined 2026-07-16 09:10 UTC, home: no outpost yet",
		"Fort Wasteland", "88000", "Iron Vultures HQ",
		"SOLAR FLARE DETECTED",
		"recommendations_for_admins",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_EmptyStatesPaths(t *testing.T) {
	s := devconsole.Snapshot{WindowDays: 7}
	p := devconsole.BuildUserPrompt(s)
	for _, want := range []string{"NEW PLAYERS THIS WINDOW: 0 total", "None."} {
		if !strings.Contains(p, want) {
			t.Errorf("expected empty-state message %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_CappedListNotice(t *testing.T) {
	s := devconsole.Snapshot{
		WindowDays:     7,
		NewPlayerCount: 50,
		NewPlayers:     []devconsole.NewPlayer{{FirstName: "Only One Shown", JoinedAt: "x", HomeContinent: "Africa"}},
	}
	p := devconsole.BuildUserPrompt(s)
	if !strings.Contains(p, "showing 1 most recent of 50 total") {
		t.Errorf("expected capped-list notice, got:\n%s", p)
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Steady week, healthy growth.", "highlights": ["2 new players", "no incidents"], "new_player_narrative": "Both joined mid-week.", "top_performer_narrative": "Fort Wasteland leads by a wide margin.", "recommendations_for_admins": "Nothing urgent.", "notes": "small sample"}`
	rec := devconsole.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if len(rec.Highlights) != 2 {
		t.Errorf("unexpected highlights: %+v", rec.Highlights)
	}
	if rec.RecommendationsForAdmins != "Nothing urgent." {
		t.Errorf("unexpected admin recommendation: %q", rec.RecommendationsForAdmins)
	}
}

func TestParseRecommendation_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n" + `{"summary": "ok", "highlights": [], "new_player_narrative": "", "top_performer_narrative": "", "recommendations_for_admins": "", "notes": ""}` + "\n```"
	rec := devconsole.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected fence to be stripped and JSON parsed, got fallback")
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "seems fine this week"
	rec := devconsole.ParseRecommendation(raw)
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
	raw := `{"summary": "Quiet week.", "highlights": []}` + "\n\nLet me know if you want more detail."
	rec := devconsole.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected trailing prose to be discarded, not trigger fallback")
	}
}

func TestParseRecommendation_RawNewlineInsideStringValue(t *testing.T) {
	raw := "{\"summary\": \"Line one\nline two\", \"highlights\": []}"
	rec := devconsole.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected sanitized control chars to allow parsing, got fallback")
	}
}

// See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
// is distinguished from one that never contained JSON at all.
func TestParseRecommendation_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"summary": "This week saw a healthy increase in new signups, largely concentrated in the Asia contin`
	rec := devconsole.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !rec.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestFormatForTelegram_FallbackPath(t *testing.T) {
	rec := &devconsole.Recommendation{Summary: "raw text", FellBackToRawText: true}
	out := devconsole.FormatForTelegram(rec)
	if !strings.Contains(out, "Couldn't parse") || !strings.Contains(out, "raw text") {
		t.Errorf("expected fallback notice and raw text, got: %s", out)
	}
}

func TestFormatForTelegram_TruncatedPath(t *testing.T) {
	rec := &devconsole.Recommendation{
		Summary:           `{"summary": "cut off mid`,
		FellBackToRawText: true,
		Truncated:         true,
	}
	out := devconsole.FormatForTelegram(rec)
	if !strings.Contains(out, "cut off before it finished") {
		t.Errorf("expected truncation-specific message, got: %s", out)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := &devconsole.Recommendation{
		Summary:                  "Steady week, healthy growth.",
		Highlights:               []string{"2 new players", "no incidents"},
		NewPlayerNarrative:       "Both joined mid-week.",
		TopPerformerNarrative:    "Fort Wasteland leads by a wide margin.",
		RecommendationsForAdmins: "Nothing urgent.",
		Notes:                    "small sample",
	}
	out := devconsole.FormatForTelegram(rec)
	for _, want := range []string{
		"AI DEVELOPER CONSOLE", "Steady week, healthy growth.",
		"2 new players", "no incidents",
		"Both joined mid-week.", "Fort Wasteland leads by a wide margin.",
		"Nothing urgent.", "small sample",
		"no player, setting, or game data has been changed automatically",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
