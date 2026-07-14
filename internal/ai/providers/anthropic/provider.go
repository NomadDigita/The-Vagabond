// Package anthropic implements ai.Provider against Anthropic's public
// Messages API (https://api.anthropic.com/v1/messages). It has zero
// dependency on internal/bot or internal/engine.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

const apiURL = "https://api.anthropic.com/v1/messages"
const apiVersion = "2023-06-01"

// Provider implements ai.Provider against Anthropic's Messages API.
type Provider struct {
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
}

// New builds an Anthropic provider. apiKey may be empty (e.g. read
// from an unset environment variable); Available() will correctly
// report false in that case rather than the caller needing to check.
func New(apiKey, defaultModel string) *Provider {
	if defaultModel == "" {
		defaultModel = "claude-sonnet-4-6"
	}
	return &Provider{
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *Provider) Name() string    { return "anthropic" }
func (p *Provider) Available() bool { return p.APIKey != "" }

// wire types mirror the Anthropic Messages API request/response shape.

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type wireTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type wireRequest struct {
	Model       string        `json:"model"`
	System      string        `json:"system,omitempty"`
	Messages    []wireMessage `json:"messages"`
	Tools       []wireTool    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature,omitempty"`
}

type wireContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type wireResponse struct {
	Content    []wireContentBlock `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *Provider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	if !p.Available() {
		return nil, fmt.Errorf("anthropic: no API key configured")
	}

	model := req.Model
	if model == "" {
		model = p.DefaultModel
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	system := req.System
	if req.JSONMode {
		system = system + "\n\nRespond with a single valid JSON object and nothing else — no prose, no markdown fences."
	}

	wr := wireRequest{
		Model:       model,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	}
	for _, m := range req.Messages {
		role := string(m.Role)
		if role == string(ai.RoleTool) {
			// Anthropic has no bare "tool" role at this layer; encode
			// tool results as a user turn labelled by name so context
			// is preserved without requiring full tool_result blocks
			// for this foundational version.
			role = "user"
		}
		if role != "user" && role != "assistant" {
			continue // system is sent via the top-level System field
		}
		wr.Messages = append(wr.Messages, wireMessage{Role: role, Content: m.Content})
	}
	for _, t := range req.Tools {
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		wr.Tools = append(wr.Tools, wireTool{Name: t.Name, Description: t.Description, InputSchema: schema})
	}

	body, err := json.Marshal(wr)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	httpResp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: read response: %w", err)
	}

	var wr2 wireResponse
	if err := json.Unmarshal(raw, &wr2); err != nil {
		return nil, fmt.Errorf("anthropic: decode response (status %d): %w", httpResp.StatusCode, err)
	}
	if wr2.Error != nil {
		return nil, fmt.Errorf("anthropic: api error (%s): %s", wr2.Error.Type, wr2.Error.Message)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic: unexpected status %d: %s", httpResp.StatusCode, string(raw))
	}

	resp := &ai.CompletionResponse{
		Model:      wr2.Model,
		StopReason: wr2.StopReason,
		Usage: ai.Usage{
			InputTokens:  wr2.Usage.InputTokens,
			OutputTokens: wr2.Usage.OutputTokens,
		},
	}
	for _, block := range wr2.Content {
		switch block.Type {
		case "text":
			resp.Text += block.Text
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{ID: block.ID, Name: block.Name, Input: block.Input})
		}
	}
	return resp, nil
}
