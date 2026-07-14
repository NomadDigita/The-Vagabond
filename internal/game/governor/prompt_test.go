package governor_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/governor"
)

func sampleSnapshot() governor.Snapshot {
	return governor.Snapshot{
		EncampmentID: "abc-123",
		Name:         "Fort Wasteland",
		Level:        4,
		Scrap:        1200,
		Rations:      340,
		Electricity:  80,
		Metal:        50,
		Crystal:      5,
		Hydrogen:     0,
		Dollars:      2000,
		Modules: []governor.ModuleState{
			{Type: "generator", Level: 3},
			{Type: "tent", Level: 2},
			{Type: "warehouse", Level: 1},
		},
		Soldiers:          12,
		Buggies:           2,
		Ships:             0,
		DefenseTechLvl:    2,
		ProductionTechLvl: 3,
	}
}

func TestBuildUserPrompt_IsDeterministic(t *testing.T) {
	s := sampleSnapshot()
	// Shuffle module order to confirm the prompt sorts them, which
	// matters for internal/ai's cache key hashing (identical logical
	// state must produce an identical prompt string).
	shuffled := governor.Snapshot{
		EncampmentID: s.EncampmentID, Name: s.Name, Level: s.Level,
		Scrap: s.Scrap, Rations: s.Rations, Electricity: s.Electricity,
		Metal: s.Metal, Crystal: s.Crystal, Hydrogen: s.Hydrogen, Dollars: s.Dollars,
		Modules:  []governor.ModuleState{{Type: "warehouse", Level: 1}, {Type: "generator", Level: 3}, {Type: "tent", Level: 2}},
		Soldiers: s.Soldiers, Buggies: s.Buggies, Ships: s.Ships,
		DefenseTechLvl: s.DefenseTechLvl, ProductionTechLvl: s.ProductionTechLvl,
	}

	p1 := governor.BuildUserPrompt(s)
	p2 := governor.BuildUserPrompt(shuffled)
	if p1 != p2 {
		t.Fatalf("expected identical prompts regardless of module input order for cache stability, got:\n---p1---\n%s\n---p2---\n%s", p1, p2)
	}
}

func TestBuildUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := governor.BuildUserPrompt(sampleSnapshot())
	for _, want := range []string{"Fort Wasteland", "generator: level 3", "12 soldiers", "priority_actions"} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Upgrade generator next.", "priority_actions": [{"action": "upgrade", "target": "generator", "reason": "electricity bottleneck"}], "storage_warning": "none", "expected_impact": "+20% electricity"}`
	rec := governor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if rec.Summary != "Upgrade generator next." {
		t.Errorf("unexpected summary: %q", rec.Summary)
	}
	if len(rec.PriorityActions) != 1 || rec.PriorityActions[0].Target != "generator" {
		t.Errorf("unexpected priority actions: %+v", rec.PriorityActions)
	}
}

func TestParseRecommendation_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n{\"summary\": \"ok\", \"priority_actions\": []}\n```"
	rec := governor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected fence-wrapped JSON to still parse, got fallback")
	}
	if rec.Summary != "ok" {
		t.Errorf("unexpected summary: %q", rec.Summary)
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "I think you should build more tents, honestly."
	rec := governor.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
	if rec.Summary != raw {
		t.Errorf("expected raw text preserved in Summary, got %q", rec.Summary)
	}
}

func TestFormatForTelegram_FallbackPath(t *testing.T) {
	rec := governor.ParseRecommendation("plain text advice")
	out := governor.FormatForTelegram(rec)
	if !strings.Contains(out, "plain text advice") {
		t.Errorf("expected fallback text to be included verbatim, got: %s", out)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := governor.ParseRecommendation(`{"summary": "Focus on defense.", "priority_actions": [{"action": "upgrade", "target": "shield", "reason": "low defense tech"}], "storage_warning": "scrap near cap", "expected_impact": "safer base"}`)
	out := governor.FormatForTelegram(rec)
	for _, want := range []string{"Focus on defense.", "upgrade", "shield", "scrap near cap", "safer base", "recommendation only"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected formatted output to contain %q, got:\n%s", want, out)
		}
	}
}
