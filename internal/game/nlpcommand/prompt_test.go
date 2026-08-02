package nlpcommand

import (
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

func TestToolDefinitions_FourActionsMatchAllowList(t *testing.T) {
	defs := ToolDefinitions()
	if len(defs) != 4 {
		t.Fatalf("expected 4 tool definitions, got %d", len(defs))
	}
	for _, d := range defs {
		if !Action(d.Name).Valid() {
			t.Errorf("tool definition %q is not a Valid() Action", d.Name)
		}
	}
}

func TestActionValid(t *testing.T) {
	valid := []Action{ActionListMarketItem, ActionCheckResources, ActionDispatchScoutMission, ActionCheckScoutStatus}
	for _, a := range valid {
		if !a.Valid() {
			t.Errorf("expected %q to be Valid()", a)
		}
	}
	if Action("delete_all_resources").Valid() {
		t.Error("expected an unlisted action to be invalid")
	}
	if Action("").Valid() {
		t.Error("expected empty action to be invalid")
	}
}

func TestActionRequiresConfirmation(t *testing.T) {
	mustConfirm := []Action{ActionListMarketItem, ActionDispatchScoutMission}
	for _, a := range mustConfirm {
		if !a.RequiresConfirmation() {
			t.Errorf("expected %q to require confirmation", a)
		}
	}
	readOnly := []Action{ActionCheckResources, ActionCheckScoutStatus}
	for _, a := range readOnly {
		if a.RequiresConfirmation() {
			t.Errorf("expected %q to NOT require confirmation", a)
		}
	}
}

func TestParseResponse_MatchesKnownToolCall(t *testing.T) {
	resp := &ai.CompletionResponse{
		ToolCalls: []ai.ToolCall{
			{Name: "list_market_item", Input: map[string]any{"resource": "scrap", "quantity": 300000.0, "price": 500.0}},
		},
	}
	result := ParseResponse(resp)
	if !result.Matched {
		t.Fatal("expected Matched=true")
	}
	if result.Command.Action != ActionListMarketItem {
		t.Errorf("expected action %q, got %q", ActionListMarketItem, result.Command.Action)
	}
	if got := result.Command.ArgString("resource"); got != "scrap" {
		t.Errorf("expected resource %q, got %q", "scrap", got)
	}
	if got := result.Command.ArgInt("quantity"); got != 300000 {
		t.Errorf("expected quantity 300000, got %d", got)
	}
	if got := result.Command.ArgFloat("price"); got != 500.0 {
		t.Errorf("expected price 500.0, got %v", got)
	}
}

// TestParseResponse_BarterListing covers the market's extension beyond
// cash-only listings: "list 200k scrap for 40k metal" should parse to
// ask_type="metal", not force a dollar price.
func TestParseResponse_BarterListing(t *testing.T) {
	resp := &ai.CompletionResponse{
		ToolCalls: []ai.ToolCall{
			{Name: "list_market_item", Input: map[string]any{
				"resource": "scrap", "quantity": 200000.0,
				"ask_type": "metal", "ask_quantity": 40000.0,
			}},
		},
	}
	result := ParseResponse(resp)
	if !result.Matched {
		t.Fatal("expected Matched=true")
	}
	if got := result.Command.ArgString("ask_type"); got != "metal" {
		t.Errorf("expected ask_type %q, got %q", "metal", got)
	}
	if got := result.Command.ArgFloat("ask_quantity"); got != 40000.0 {
		t.Errorf("expected ask_quantity 40000, got %v", got)
	}
}

func TestToolDefinitions_ListMarketItemRequiresAskTypeAndQuantity(t *testing.T) {
	for _, d := range ToolDefinitions() {
		if d.Name != string(ActionListMarketItem) {
			continue
		}
		schema, ok := d.InputSchema["required"].([]string)
		if !ok {
			t.Fatalf("expected list_market_item's required field to be a []string")
		}
		want := map[string]bool{"resource": false, "quantity": false, "ask_type": false, "ask_quantity": false}
		for _, r := range schema {
			if _, ok := want[r]; ok {
				want[r] = true
			}
		}
		for field, found := range want {
			if !found {
				t.Errorf("expected list_market_item to require %q", field)
			}
		}
		// The old dollars-only field name should be gone entirely -
		// this tool now expresses cash via ask_type="dollars" instead
		// of a dedicated price field.
		if _, ok := d.InputSchema["properties"].(map[string]any)["price"]; ok {
			t.Error("expected the old 'price' field to be removed in favor of ask_type/ask_quantity")
		}
	}
}

func TestParseResponse_IgnoresUnknownToolName(t *testing.T) {
	resp := &ai.CompletionResponse{
		ToolCalls: []ai.ToolCall{
			{Name: "delete_all_resources", Input: map[string]any{}},
		},
		Text: "",
	}
	result := ParseResponse(resp)
	if result.Matched {
		t.Fatal("expected an unlisted tool name to never be Matched")
	}
}

func TestParseResponse_FallsBackToClarifyText(t *testing.T) {
	resp := &ai.CompletionResponse{
		Text: "  Did you want to check your resources or dispatch scouts?  ",
	}
	result := ParseResponse(resp)
	if result.Matched {
		t.Fatal("expected Matched=false when no tool was called")
	}
	if result.ClarifyText != "Did you want to check your resources or dispatch scouts?" {
		t.Errorf("expected trimmed clarify text, got %q", result.ClarifyText)
	}
}

func TestParseResponse_NilResponse(t *testing.T) {
	result := ParseResponse(nil)
	if result.Matched || result.ClarifyText != "" {
		t.Errorf("expected zero-value Result for nil response, got %+v", result)
	}
}

func TestParseResponse_TruncatedResponseNeverActedOn(t *testing.T) {
	resp := &ai.CompletionResponse{
		StopReason: "max_tokens",
		ToolCalls: []ai.ToolCall{
			{Name: "dispatch_scout_mission", Input: map[string]any{"count": 5.0}},
		},
		Text: "some partial thought that got cut off",
	}
	result := ParseResponse(resp)
	if result.Matched {
		t.Fatal("expected a truncated response's tool call to never be acted on")
	}
	if result.ClarifyText != "" {
		t.Error("expected a truncated response to also suppress clarify text, not just the tool call")
	}
}

func TestParsedCommand_ArgIntTruncatesFloat(t *testing.T) {
	cmd := ParsedCommand{Args: map[string]any{"count": 4.9}}
	if got := cmd.ArgInt("count"); got != 4 {
		t.Errorf("expected 4.9 to truncate to 4, got %d", got)
	}
}

func TestParsedCommand_MissingArgsReturnZeroValues(t *testing.T) {
	cmd := ParsedCommand{Args: map[string]any{}}
	if got := cmd.ArgString("resource"); got != "" {
		t.Errorf("expected empty string for missing arg, got %q", got)
	}
	if got := cmd.ArgFloat("price"); got != 0 {
		t.Errorf("expected 0 for missing arg, got %v", got)
	}
}
