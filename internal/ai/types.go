package ai

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role
	Content    string
	ToolCallID string
	Name       string
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

type Usage struct {
	InputTokens  int
	OutputTokens int
}

type CompletionRequest struct {
	Model string

	System      string
	Messages    []Message
	Tools       []ToolDefinition
	MaxTokens   int
	Temperature float64

	JSONMode bool

	Feature string

	UserID int64
}

type CompletionResponse struct {
	Text       string
	ToolCalls  []ToolCall
	Usage      Usage
	Provider   string
	Model      string
	StopReason string
	CostUSD    float64
	Cached     bool
}

type Provider interface {
	Name() string
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	Available() bool
}
