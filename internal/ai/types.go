// Package ai is the provider-agnostic AI Foundation for The Vagabond.
//
// This package (Phase A of the AI Systems Roadmap) never talks to game
// tables directly and never imports internal/bot or internal/engine.
// Every future AI capability (Planet Governor, Fleet Commander, Economy
// Advisor, Research Planner, Battle Analyst, Guild Assistant, Dynamic
// Galaxy, NPC Intelligence, Developer Console) is expected to depend on
// this package, never the other way around. See PROJECT_MASTER_PLAN.md
// for the full roadmap and integration notes.
package ai

import "context"

// Role identifies who authored a Message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in a provider-agnostic conversation.
type Message struct {
	Role Role
	// Content is plain text. Structured tool results should be
	// JSON-encoded into Content by the caller.
	Content string
	// ToolCallID links a RoleTool message back to the ToolCall.ID
	// that produced it. Empty for non-tool messages.
	ToolCallID string
	// Name optionally identifies the tool or sub-agent that authored
	// this message (useful for multi-agent transcripts / logging).
	Name string
}

// ToolDefinition describes a callable tool in JSON-Schema form, mirroring
// the shape used by every major provider (Anthropic, OpenAI, Gemini).
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolCall is a request from the model to invoke a tool.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// Usage reports token accounting for a single completion, used by the
// cost-control subsystem to price the call.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// CompletionRequest is the provider-agnostic request shape. Every
// Provider implementation is responsible for translating this into its
// own wire format.
type CompletionRequest struct {
	// Model is a provider-specific model identifier. Callers should
	// generally leave this empty and let the Service fill in the
	// configured default for the routed provider.
	Model string

	System      string
	Messages    []Message
	Tools       []ToolDefinition
	MaxTokens   int
	Temperature float64

	// JSONMode asks the provider to constrain output to a single
	// JSON object (used by structured-response callers such as the
	// Developer Console and Battle Analyst).
	JSONMode bool

	// Feature identifies which game subsystem is calling (e.g.
	// "planet_governor", "fleet_commander"). Used for permissioning,
	// cost attribution, caching, and observability. Required.
	Feature string

	// UserID is the Telegram ID of the player this request is made
	// on behalf of. Zero means a system/background request (e.g. the
	// Dynamic Galaxy director), which bypasses per-user budgets but
	// is still subject to the global daily budget.
	UserID int64
}

// CompletionResponse is the provider-agnostic result shape.
type CompletionResponse struct {
	Text       string
	ToolCalls  []ToolCall
	Usage      Usage
	Provider   string
	Model      string
	StopReason string
	// CostUSD is filled in by the Service after pricing, not by the
	// Provider itself.
	CostUSD float64
	// Cached reports whether this response was served from the cache
	// layer rather than a live provider call.
	Cached bool
}

// Provider is implemented by every AI backend (Anthropic, OpenAI,
// Gemini, Qwen, Grok, DeepSeek, Ollama, a local mock, ...). Providers
// must be safe for concurrent use.
type Provider interface {
	// Name is a short stable identifier, e.g. "anthropic", "mock".
	Name() string
	// Complete performs one round of completion.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	// Available reports whether the provider is currently usable
	// (e.g. has credentials configured). The registry skips
	// unavailable providers during fallback.
	Available() bool
}
