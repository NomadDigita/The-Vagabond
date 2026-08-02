// Package nlpcommand implements Milestone 3 of
// FEEDBACK_CHANGELOG_NLP_PLAN.md: natural-language command execution,
// wired as the final fallback in internal/bot/handlers/nlp.go's
// HandleTextMessage chain.
//
// This is deliberately different from every other AI package in this
// codebase (governor, fleetcommander, econadvisor, researchplanner,
// ...): those are advisory-only by design - they recommend, a human
// taps a button to act. This package's whole point is to let the AI's
// parsed intent result in an actual game action. The safety design
// that makes that acceptable:
//
//  1. The AI never touches the database. It has exactly one job: turn
//     free text into a ParsedCommand (Action + Args) via a
//     tool-calling completion request. This file (prompt.go) has zero
//     I/O so it's independently unit-testable, matching every sibling
//     AI package's planner.go/prompt.go split. All actual game-state
//     mutation happens in internal/bot/handlers, calling the exact
//     same doX core functions the button-driven UI already calls
//     (doDispatchScoutMission, doPostListing) - same validation, same
//     resource checks, same everything.
//  2. A fixed action allow-list, not an open-ended tool. See Action
//     and ToolDefinitions below - only what's listed there can ever
//     be returned as Matched.
//  3. Anything that spends resources or commits forces requires an
//     explicit Confirm/Cancel step before executing - see
//     Action.RequiresConfirmation. Pure reads skip confirmation since
//     there's nothing to confirm.
package nlpcommand

import (
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// Action is one of the fixed, allow-listed actions this milestone
// implements. Expanding this list is designed to be incremental (add
// a tool definition + one dispatch case + one confirmation template)
// rather than needing a rewrite - see the plan doc's "Starting action
// set" section.
type Action string

const (
	// ActionListMarketItem posts a quantity of a tradeable resource
	// for sale on the player auction market exchange. Mutating -
	// requires confirmation.
	ActionListMarketItem Action = "list_market_item"
	// ActionCheckResources looks up the player's current resource
	// stockpile. Read-only - no confirmation needed.
	ActionCheckResources Action = "check_resources"
	// ActionDispatchScoutMission commits scout walkers to a
	// long-range search mission. Mutating - requires confirmation.
	ActionDispatchScoutMission Action = "dispatch_scout_mission"
	// ActionCheckScoutStatus reports on the player's currently
	// active scout missions. Read-only - no confirmation needed.
	ActionCheckScoutStatus Action = "check_scout_status"
)

// Valid reports whether a is one of the four actions this milestone
// actually implements - anything else (e.g. a hallucinated tool name)
// is rejected rather than dispatched.
func (a Action) Valid() bool {
	switch a {
	case ActionListMarketItem, ActionCheckResources, ActionDispatchScoutMission, ActionCheckScoutStatus:
		return true
	default:
		return false
	}
}

// RequiresConfirmation reports whether a spends a resource or commits
// forces and therefore must be shown as a Confirm/Cancel card rather
// than executed immediately - see the plan doc's safety rule #2.
func (a Action) RequiresConfirmation() bool {
	switch a {
	case ActionListMarketItem, ActionDispatchScoutMission:
		return true
	default:
		return false
	}
}

// ParsedCommand is the structured result of turning a player's free
// text into an intent - the plan doc's literal
// "ParsedCommand{Action string, Args map[string]any}" shape.
type ParsedCommand struct {
	Action Action
	Args   map[string]any
}

// ArgString reads a string argument, defaulting to "" if absent or a
// different type.
func (p ParsedCommand) ArgString(key string) string {
	if v, ok := p.Args[key].(string); ok {
		return v
	}
	return ""
}

// ArgFloat reads a numeric argument. Tool-call args decode as
// float64 in the common case (encoding/json's default for JSON
// numbers); json.Number is handled too since some providers may
// round-trip through a decoder configured with UseNumber.
func (p ParsedCommand) ArgFloat(key string) float64 {
	switch v := p.Args[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case interface{ Float64() (float64, error) }:
		f, err := v.Float64()
		if err == nil {
			return f
		}
	}
	return 0
}

// ArgInt reads a numeric argument truncated to an int - used for
// counts/quantities, which are always whole units in this game.
func (p ParsedCommand) ArgInt(key string) int {
	return int(p.ArgFloat(key))
}

// Result is what Interpret returns to the caller: either exactly one
// matched, allow-listed ParsedCommand, or a plain-text reply from the
// model (a clarifying question, per the system prompt's instruction)
// to show the player instead of a raw "not recognized" message.
type Result struct {
	Matched     bool
	Command     ParsedCommand
	ClarifyText string
}

// SystemPrompt instructs the model to act purely as an intent
// classifier over the fixed action allow-list - never to answer from
// its own knowledge, never to invent an action outside the tool
// schema, and to ask a clarifying question in plain text when nothing
// in the allow-list genuinely fits (per Milestone 3's "fixed action
// allow-list, not an open-ended tool" safety rule).
const SystemPrompt = `You are the command interpreter for The Vagabond, a post-apocalyptic survival MMO played through a Telegram bot.

Your ONLY job is to read one free-text message from a player and, if it clearly matches one of the tools available to you, call that tool with the correct arguments extracted from the message. You never answer questions yourself and you never touch any game state directly - a separate system executes the actual action after you classify it.

Rules:
- Only call a tool if the player's intent is reasonably clear. Do not guess wildly.
- Never invent an action, field, or tool that isn't offered to you.
- Quantity shorthand: "300k" means 300000, "1.5m" means 1500000, "2k" means 2000. Always resolve shorthand to a plain number in the tool's numeric arguments.
- Market listings can ask for cash OR another resource in return (barter) - "list 300k scrap for sale" or "sell 50 metal for $200" means ask_type is 'dollars', while "list 200k scrap for 40k metal" or "trade 50 crystal for scrap" means ask_type is the named resource. Never assume a dollar price when the player named a resource instead.
- If the message doesn't clearly match any available tool, do NOT call a tool. Instead reply with a short, friendly plain-text clarifying question (one sentence) asking what the player meant, OR a brief direct answer if the message is really just a greeting or small talk unrelated to any action.
- Never fabricate game data (resource amounts, prices, mission outcomes) - you have no access to the player's actual state. Only the tool call itself carries information forward; any numbers in your plain-text replies must come only from what the player themselves said.`

// ToolDefinitions is the fixed tool schema offered to the model -
// exactly the four actions this milestone implements, per the plan
// doc's "Starting action set (deliberately small)".
func ToolDefinitions() []ai.ToolDefinition {
	return []ai.ToolDefinition{
		{
			Name:        string(ActionListMarketItem),
			Description: "List a quantity of a tradeable resource for sale on the player auction market exchange, asking for either a cash price OR another resource in return (barter). Only call this when the player clearly wants to post a new sale listing (e.g. 'list 300k scrap for sale', 'sell 50 metal for $200', 'list 200k scrap for 40k metal').",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resource": map[string]any{
						"type":        "string",
						"enum":        []string{"metal", "crystal", "scrap"},
						"description": "Which resource to list.",
					},
					"quantity": map[string]any{
						"type":        "number",
						"description": "How many units to list. Resolve shorthand like '300k' to 300000.",
					},
					"ask_type": map[string]any{
						"type":        "string",
						"enum":        []string{"dollars", "metal", "crystal", "scrap"},
						"description": "What the player wants in return: 'dollars' for a cash asking price, or a resource name to barter for that resource instead. Must be different from 'resource' - a resource can't be bartered for itself. If the player didn't say what they want in return at all, do not call this tool yet - ask them.",
					},
					"ask_quantity": map[string]any{
						"type":        "number",
						"description": "How much of ask_type is being asked for - a dollar amount if ask_type is 'dollars', otherwise a quantity of that resource. Resolve shorthand like '40k' to 40000.",
					},
				},
				"required": []string{"resource", "quantity", "ask_type", "ask_quantity"},
			},
		},
		{
			Name:        string(ActionCheckResources),
			Description: "Look up the player's current resource stockpile (Metal, Crystal, Scrap, Cash, and so on). Call this for any question about what the player currently has or their balance (e.g. 'what's my scrap balance', 'how much metal do I have').",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        string(ActionDispatchScoutMission),
			Description: "Send scout walkers out to search the wasteland for other outposts. Only call this when the player clearly wants to send scouts right now (e.g. 'send 5 scouts', 'dispatch all my scouts').",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"count": map[string]any{
						"type":        "number",
						"description": "How many scout walkers to commit. If the player said 'all', use a very large number (e.g. 999999) and let the executor clamp it to what's actually available.",
					},
				},
				"required": []string{"count"},
			},
		},
		{
			Name:        string(ActionCheckScoutStatus),
			Description: "Check on the status of the player's currently active scout missions (e.g. 'where are my scouts', 'are my scouts back yet').",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// ParseResponse turns a raw ai.CompletionResponse into a Result. A
// truncated response (see ai.IsTruncatedStopReason) is treated as
// unmatched with no clarify text rather than acted on, since a cut-off
// tool call's arguments can't be trusted - matching ADR-025's "never
// act on a truncated response" discipline elsewhere in this codebase.
func ParseResponse(resp *ai.CompletionResponse) Result {
	if resp == nil {
		return Result{}
	}
	if ai.IsTruncatedStopReason(resp.StopReason) {
		return Result{}
	}
	for _, tc := range resp.ToolCalls {
		action := Action(tc.Name)
		if !action.Valid() {
			continue
		}
		return Result{Matched: true, Command: ParsedCommand{Action: action, Args: tc.Input}}
	}
	return Result{ClarifyText: strings.TrimSpace(resp.Text)}
}
