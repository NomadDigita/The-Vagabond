package guildassistant_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/guildassistant"
)

func sampleSnapshot() guildassistant.Snapshot {
	return guildassistant.Snapshot{
		ClanID:          "clan-1",
		Name:            "Wasteland Raiders",
		Recruiting:      true,
		MemberCount:     9,
		MemberCap:       15,
		CombinedLevel:   72,
		MilitaryPower:   4200,
		InactiveMembers: 2,
		PendingApplicants: []guildassistant.Applicant{
			{Username: "newbie99", Level: 2},
			{Username: "vetplayer", Level: 15},
		},
		War: guildassistant.WarRecord{
			InActiveWar:   true,
			OpponentName:  "Iron Vultures",
			OurScore:      3200,
			TheirScore:    2800,
			CompletedWars: 4,
			Wins:          3,
			Losses:        1,
		},
	}
}

func TestBuildUserPrompt_IsDeterministic(t *testing.T) {
	s := sampleSnapshot()
	p1 := guildassistant.BuildUserPrompt(s)
	p2 := guildassistant.BuildUserPrompt(s)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input")
	}
}

func TestBuildUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := guildassistant.BuildUserPrompt(sampleSnapshot())
	for _, want := range []string{
		"Wasteland Raiders", "9 / 15 members",
		"@newbie99 — Level 2", "@vetplayer — Level 15",
		"Iron Vultures", "3200", "2800",
		"4 completed wars: 3 wins, 1 losses",
		"recruitment_calls",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_EmptyStatesPaths(t *testing.T) {
	s := guildassistant.Snapshot{ClanID: "x", Name: "Bare Clan", MemberCap: 15}
	p := guildassistant.BuildUserPrompt(s)
	for _, want := range []string{"None.", "Not currently at war.", "No completed wars yet."} {
		if !strings.Contains(p, want) {
			t.Errorf("expected empty-state message %q, got:\n%s", want, p)
		}
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Solid roster, one weak applicant.", "recruitment_calls": [{"username": "newbie99", "recommendation": "accept", "reason": "low level but room to grow"}], "war_insight": "Leading by 400 points.", "recommended_focus": "keep pressing the war lead", "notes": "2 inactive members worth checking on"}`
	rec := guildassistant.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if len(rec.RecruitmentCalls) != 1 || rec.RecruitmentCalls[0].Recommendation != "accept" {
		t.Errorf("unexpected recruitment calls: %+v", rec.RecruitmentCalls)
	}
	if rec.WarInsight != "Leading by 400 points." {
		t.Errorf("unexpected war insight: %q", rec.WarInsight)
	}
}

func TestParseRecommendation_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n" + `{"summary": "ok", "recruitment_calls": [], "war_insight": "", "recommended_focus": "", "notes": ""}` + "\n```"
	rec := guildassistant.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected fence to be stripped and JSON parsed, got fallback")
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "yeah accept everyone I guess"
	rec := guildassistant.ParseRecommendation(raw)
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
	raw := `{"summary": "Roster looks healthy.", "recruitment_calls": []}` + "\n\nLet me know if you want more detail!"
	rec := guildassistant.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected trailing prose to be discarded, not trigger fallback")
	}
}

func TestParseRecommendation_RawNewlineInsideStringValue(t *testing.T) {
	raw := "{\"summary\": \"Line one\nline two\", \"recruitment_calls\": []}"
	rec := guildassistant.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected sanitized control chars to allow parsing, got fallback")
	}
}

// See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
// is distinguished from one that never contained JSON at all.
func TestParseRecommendation_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"summary": "This clan's roster is strong but the war record shows a concerning trend where losses have been mounting against oppon`
	rec := guildassistant.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !rec.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestFormatForTelegram_FallbackPath(t *testing.T) {
	rec := &guildassistant.Recommendation{Summary: "raw text", FellBackToRawText: true}
	out := guildassistant.FormatForTelegram(rec)
	if !strings.Contains(out, "Couldn't parse") || !strings.Contains(out, "raw text") {
		t.Errorf("expected fallback notice and raw text, got: %s", out)
	}
}

func TestFormatForTelegram_TruncatedPath(t *testing.T) {
	rec := &guildassistant.Recommendation{
		Summary:           `{"summary": "cut off mid`,
		FellBackToRawText: true,
		Truncated:         true,
	}
	out := guildassistant.FormatForTelegram(rec)
	if !strings.Contains(out, "cut off before it finished") {
		t.Errorf("expected truncation-specific message, got: %s", out)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := &guildassistant.Recommendation{
		Summary: "Solid roster, one weak applicant.",
		RecruitmentCalls: []guildassistant.RecruitmentCall{
			{Username: "newbie99", Recommendation: "accept", Reason: "low level but room to grow"},
		},
		WarInsight:       "Leading by 400 points.",
		RecommendedFocus: "keep pressing the war lead",
		Notes:            "2 inactive members worth checking on",
	}
	out := guildassistant.FormatForTelegram(rec)
	for _, want := range []string{
		"AI GUILD ASSISTANT", "Solid roster, one weak applicant.",
		"@newbie99 — accept", "low level but room to grow",
		"Leading by 400 points.", "keep pressing the war lead",
		"2 inactive members worth checking on",
		"no applicant has been accepted/rejected and no war has been declared automatically",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
