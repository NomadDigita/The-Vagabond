package researchplanner

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleLevels() map[string]int {
	return map[string]int{
		"econ": 3, "production": 3, "integrity": 1,
		"defense": 1, "intel": 2, "speed": 2, "military": 1,
	}
}

func TestBuildTechNodes_SevenNodesInOrder(t *testing.T) {
	nodes := BuildTechNodes(sampleLevels())
	if len(nodes) != 7 {
		t.Fatalf("expected 7 tech nodes, got %d", len(nodes))
	}
	wantOrder := []string{"econ", "production", "integrity", "defense", "intel", "speed", "military"}
	for i, key := range wantOrder {
		if nodes[i].Key != key {
			t.Errorf("node %d: expected key %q, got %q", i, key, nodes[i].Key)
		}
	}
}

func TestBuildTechNodes_CostFormula(t *testing.T) {
	nodes := BuildTechNodes(map[string]int{"econ": 5})
	for _, n := range nodes {
		if n.Key != "econ" {
			continue
		}
		if n.Cost != 40 {
			t.Errorf("expected cost 5*8=40 at level 5, got %d", n.Cost)
		}
		if n.Maxed {
			t.Errorf("level 5 should not be maxed")
		}
	}
}

func TestBuildTechNodes_MaxedNodeHasZeroCost(t *testing.T) {
	nodes := BuildTechNodes(map[string]int{"military": MaxResearchLevel})
	for _, n := range nodes {
		if n.Key != "military" {
			continue
		}
		if !n.Maxed {
			t.Errorf("expected military at level %d to be maxed", MaxResearchLevel)
		}
		if n.Cost != 0 {
			t.Errorf("expected 0 cost for maxed node, got %d", n.Cost)
		}
	}
}

func TestTechNodeCost_LinearFormula(t *testing.T) {
	cases := map[int]int{0: 0, 1: 8, 5: 40, 19: 152}
	for lvl, want := range cases {
		if got := TechNodeCost(lvl); got != want {
			t.Errorf("TechNodeCost(%d) = %d, want %d", lvl, got, want)
		}
	}
}

func TestIsValidGoal(t *testing.T) {
	for _, g := range ValidGoals() {
		if !IsValidGoal(string(g)) {
			t.Errorf("expected %q to be a valid goal", g)
		}
	}
	if IsValidGoal("nonsense") {
		t.Errorf("expected 'nonsense' to be invalid")
	}
	if IsValidGoal("") {
		t.Errorf("expected empty string to be invalid (means: infer)")
	}
}

func TestInferredGoal_ExplicitRequestWins(t *testing.T) {
	s := Snapshot{RequestedGoal: GoalRaiding, Nodes: BuildTechNodes(sampleLevels())}
	if got := s.InferredGoal(); got != GoalRaiding {
		t.Errorf("expected explicit request to win, got %q", got)
	}
}

func TestInferredGoal_CombatAheadInfersEconomy(t *testing.T) {
	// Combat nodes (integrity/defense/military) far ahead of
	// economy nodes (econ/production) → infer "economy" catch-up.
	s := Snapshot{Nodes: BuildTechNodes(map[string]int{
		"integrity": 10, "defense": 10, "military": 10,
		"econ": 1, "production": 1,
		"intel": 1, "speed": 1,
	})}
	if got := s.InferredGoal(); got != GoalEconomy {
		t.Errorf("expected inferred goal 'economy', got %q", got)
	}
}

func TestInferredGoal_EconomyAheadInfersDefense(t *testing.T) {
	s := Snapshot{Nodes: BuildTechNodes(map[string]int{
		"econ": 10, "production": 10,
		"integrity": 1, "defense": 1, "military": 1,
		"intel": 1, "speed": 1,
	})}
	if got := s.InferredGoal(); got != GoalDefense {
		t.Errorf("expected inferred goal 'defense', got %q", got)
	}
}

func TestInferredGoal_EvenSpreadIsBalanced(t *testing.T) {
	s := Snapshot{Nodes: BuildTechNodes(map[string]int{
		"econ": 2, "production": 2,
		"integrity": 2, "defense": 2, "military": 2,
		"intel": 2, "speed": 2,
	})}
	if got := s.InferredGoal(); got != GoalBalanced {
		t.Errorf("expected inferred goal 'balanced' for even spread, got %q", got)
	}
}

func TestBuildUserPrompt_IncludesGoalAndCores(t *testing.T) {
	s := Snapshot{
		EncampmentID:  "camp-1",
		Name:          "Rustbucket",
		Level:         4,
		NeuroCores:    120,
		Nodes:         BuildTechNodes(sampleLevels()),
		RequestedGoal: GoalRaiding,
	}
	prompt := BuildUserPrompt(s)
	if !strings.Contains(prompt, "GOAL: raiding") {
		t.Errorf("expected prompt to include requested goal, got: %s", prompt)
	}
	if !strings.Contains(prompt, "NEURO CORES AVAILABLE: 120") {
		t.Errorf("expected prompt to include neuro core count, got: %s", prompt)
	}
	if !strings.Contains(prompt, "military") {
		t.Errorf("expected prompt to list all 7 tech nodes, got: %s", prompt)
	}
}

func TestBuildUserPrompt_MaxedNodeNotedAsUnavailable(t *testing.T) {
	s := Snapshot{Nodes: BuildTechNodes(map[string]int{"military": MaxResearchLevel})}
	prompt := BuildUserPrompt(s)
	if !strings.Contains(prompt, "MAX, cannot be upgraded further") {
		t.Errorf("expected prompt to flag maxed node, got: %s", prompt)
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Focus raiding.", "goal_used": "raiding", "recommended_order": [{"node": "speed", "target_level": 3, "reason": "faster raids", "core_cost": 16, "expected_gain": "-8% travel time"}], "cores_needed": 16, "cores_available": 120, "notes": ""}`
	rec := ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected valid JSON to parse, got fallback")
	}
	if rec.Summary != "Focus raiding." {
		t.Errorf("unexpected summary: %s", rec.Summary)
	}
	if len(rec.RecommendedOrder) != 1 || rec.RecommendedOrder[0].Node != "speed" {
		t.Errorf("unexpected recommended_order: %+v", rec.RecommendedOrder)
	}
}

func TestParseRecommendation_FencedJSON(t *testing.T) {
	raw := "```json\n{\"summary\": \"ok\"}\n```"
	rec := ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected fenced JSON to parse, got fallback")
	}
}

func TestParseRecommendation_InvalidFallsBackToRawText(t *testing.T) {
	raw := "not json at all"
	rec := ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected invalid text to fall back")
	}
	if rec.Summary != raw {
		t.Errorf("expected raw text preserved in Summary, got: %s", rec.Summary)
	}
}

func TestFormatForTelegram_FallbackShowsRawText(t *testing.T) {
	rec := &Recommendation{Summary: "raw dump", FellBackToRawText: true}
	out := FormatForTelegram(rec)
	if !strings.Contains(out, "raw dump") {
		t.Errorf("expected fallback text in output, got: %s", out)
	}
}

func TestFormatForTelegram_StructuredIncludesOrderAndCores(t *testing.T) {
	rec := &Recommendation{
		Summary:  "Go raiding.",
		GoalUsed: "raiding",
		RecommendedOrder: []PrioritizedNode{
			{Node: "speed", TargetLevel: 3, Reason: "faster raids", CoreCost: 16, ExpectedGain: "-8% travel time"},
		},
		CoresNeeded:    16,
		CoresAvailable: 120,
	}
	out := FormatForTelegram(rec)
	for _, want := range []string{"raiding", "Go raiding.", "speed", "16 cores", "120", "nothing has been researched automatically"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got: %s", want, out)
		}
	}
}

func TestRecommendation_JSONRoundTrip(t *testing.T) {
	rec := Recommendation{
		Summary:  "test",
		GoalUsed: "balanced",
		RecommendedOrder: []PrioritizedNode{
			{Node: "econ", TargetLevel: 2, Reason: "r", CoreCost: 8, ExpectedGain: "g"},
		},
		CoresNeeded:    8,
		CoresAvailable: 50,
		Notes:          "n",
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var rec2 Recommendation
	if err := json.Unmarshal(data, &rec2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if rec2.Summary != rec.Summary || rec2.CoresNeeded != rec.CoresNeeded {
		t.Errorf("round-trip mismatch: %+v vs %+v", rec, rec2)
	}
}
