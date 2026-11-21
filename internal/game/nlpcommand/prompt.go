package nlpcommand

import (
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type Action string

const (
	ActionListMarketItem       Action = "list_market_item"
	ActionBuyMarketItem        Action = "buy_market_item"
	ActionCheckResources       Action = "check_resources"
	ActionDispatchScoutMission Action = "dispatch_scout_mission"
	ActionCheckScoutStatus     Action = "check_scout_status"
	ActionLaunchRaid           Action = "launch_raid"
)

func (a Action) Valid() bool {
	switch a {
	case ActionListMarketItem, ActionBuyMarketItem, ActionCheckResources, ActionDispatchScoutMission, ActionCheckScoutStatus, ActionLaunchRaid:
		return true
	default:
		return false
	}
}

func (a Action) RequiresConfirmation() bool {
	switch a {
	case ActionListMarketItem, ActionBuyMarketItem, ActionDispatchScoutMission:
		return true
	default:
		return false
	}
}

type ParsedCommand struct {
	Action Action
	Args   map[string]any
}

func (p ParsedCommand) ArgString(key string) string {
	if v, ok := p.Args[key].(string); ok {
		return v
	}
	return ""
}

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

func (p ParsedCommand) ArgInt(key string) int {
	return int(p.ArgFloat(key))
}

type Result struct {
	Matched     bool
	Command     ParsedCommand
	ClarifyText string
}

const SystemPrompt = `You are the command interpreter for The Vagabond, a post-apocalyptic survival MMO played through a Telegram bot.

Your ONLY job is to read one free-text message from a player and, if it clearly matches one of the tools available to you, call that tool with the correct arguments extracted from the message. You never answer questions yourself and you never touch any game state directly - a separate system executes the actual action after you classify it.

Rules:
- Only call a tool if the player's intent is reasonably clear. Do not guess wildly.
- Never invent an action, field, or tool that isn't offered to you.
- Quantity shorthand: "300k" means 300000, "1.5m" means 1500000, "2k" means 2000. Always resolve shorthand to a plain number in the tool's numeric arguments.
- Market listings can ask for cash OR another resource in return (barter) - "list 300k scrap for sale" or "sell 50 metal for $200" means ask_type is 'dollars', while "list 200k scrap for 40k metal" or "trade 50 crystal for scrap" means ask_type is the named resource. Never assume a dollar price when the player named a resource instead.
- If the message doesn't clearly match any available tool, do NOT call a tool. Instead reply with a short, friendly plain-text clarifying question (one sentence) asking what the player meant, OR a brief direct answer if the message is really just a greeting or small talk unrelated to any action.
- Never fabricate game data (resource amounts, prices, mission outcomes) - you have no access to the player's actual state. Only the tool call itself carries information forward; any numbers in your plain-text replies must come only from what the player themselves said.`

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
			Name:        string(ActionBuyMarketItem),
			Description: "Search the player auction market exchange for an existing listing of a resource and buy it, paying cash. Only call this when the player clearly wants to buy something right now from the market (e.g. 'buy 200 metal for 500 dollars', 'buy any crystal listing under $1000'). Buys exactly the requested quantity - if the best matching listing has more than that available (e.g. wanting 500 metal from an 800-metal listing), only the requested amount is bought and the seller keeps the remainder listed; the executor will show the real quantity and price before anything is charged.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resource": map[string]any{
						"type":        "string",
						"enum":        []string{"metal", "crystal", "scrap"},
						"description": "Which resource to buy.",
					},
					"min_quantity": map[string]any{
						"type":        "number",
						"description": "The minimum quantity the player wants - only listings with at least this much of the resource are considered. Resolve shorthand like '300k' to 300000.",
					},
					"max_dollars": map[string]any{
						"type":        "number",
						"description": "The most the player is willing to pay in total, in dollars. Only listings asking this much or less are considered. Resolve shorthand like '1.5m' to 1500000.",
					},
				},
				"required": []string{"resource", "min_quantity", "max_dollars"},
			},
		},
		{
			Name:        string(ActionLaunchRaid),
			Description: "Open the raid draft/launch screen for a specific, named target the player has already discovered or scouted (e.g. 'raid Lotus Dominion', 'attack Bob's base', 'launch on the outpost near Wrenfield'). This does NOT itself send any forces or spend anything - it only opens the same composition-and-launch-style picker a manual tap on the target matrix opens; the player still chooses their force composition and taps Launch themselves. Only call this when the player names a SPECIFIC target - if they ask to raid 'someone' or 'anyone' with no name given, do not call this tool; ask them to name a target instead.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target_name": map[string]any{
						"type":        "string",
						"description": "The outpost name or commander's name the player wants to raid, exactly as they referred to it.",
					},
				},
				"required": []string{"target_name"},
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
