package openaicompat

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

type Provider struct {
	ProviderName               string
	BaseURL                    string
	APIKey                     string
	DefaultModel               string
	SupportsJSONResponseFormat bool

	HTTPClient *http.Client
}

func New(providerName, baseURL, apiKey, defaultModel string, supportsJSONResponseFormat bool) *Provider {
	return &Provider{
		ProviderName:               providerName,
		BaseURL:                    baseURL,
		APIKey:                     apiKey,
		DefaultModel:               defaultModel,
		SupportsJSONResponseFormat: supportsJSONResponseFormat,
		HTTPClient:                 &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *Provider) Name() string    { return p.ProviderName }
func (p *Provider) Available() bool { return p.APIKey != "" && p.BaseURL != "" }

type wireFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []wireToolCall `json:"tool_calls,omitempty"`
}

type wireResponseFormat struct {
	Type string `json:"type"`
}

type wireRequest struct {
	Model          string              `json:"model"`
	Messages       []wireMessage       `json:"messages"`
	Tools          []wireTool          `json:"tools,omitempty"`
	MaxTokens      int                 `json:"max_tokens,omitempty"`
	Temperature    float64             `json:"temperature,omitempty"`
	ResponseFormat *wireResponseFormat `json:"response_format,omitempty"`
	EnableThinking *bool               `json:"enable_thinking,omitempty"`
}

type wireChoice struct {
	Message      wireMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type wireResponse struct {
	Choices []wireChoice `json:"choices"`
	Model   string       `json:"model"`
	Usage   wireUsage    `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *Provider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	if !p.Available() {
		return nil, fmt.Errorf("%s: no API key or base URL configured", p.ProviderName)
	}

	model := req.Model
	if model == "" {
		model = p.DefaultModel
	}

	wr := wireRequest{
		Model:       model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	if p.ProviderName == "qwen" {
		noThinking := false
		wr.EnableThinking = &noThinking
	}

	system := req.System
	if req.JSONMode {
		if p.SupportsJSONResponseFormat {
			wr.ResponseFormat = &wireResponseFormat{Type: "json_object"}
		}
		system = system + "\n\nRespond with a single valid JSON object and nothing else — no prose, no markdown fences."
	}
	if system != "" {
		wr.Messages = append(wr.Messages, wireMessage{Role: "system", Content: system})
	}
	for _, m := range req.Messages {
		role := string(m.Role)
		if role == string(ai.RoleTool) {
			role = "user"
		}
		wr.Messages = append(wr.Messages, wireMessage{Role: role, Content: m.Content})
	}

	for _, t := range req.Tools {
		params := t.InputSchema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		wr.Tools = append(wr.Tools, wireTool{
			Type:     "function",
			Function: wireFunction{Name: t.Name, Description: t.Description, Parameters: params},
		})
	}

	body, err := json.Marshal(wr)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", p.ProviderName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%s: build request: %w", p.ProviderName, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	httpResp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s: request failed: %w", p.ProviderName, err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read response: %w", p.ProviderName, err)
	}

	var wr2 wireResponse
	if err := json.Unmarshal(raw, &wr2); err != nil {
		return nil, fmt.Errorf("%s: decode response (status %d): %w", p.ProviderName, httpResp.StatusCode, err)
	}
	if wr2.Error != nil {
		return nil, fmt.Errorf("%s: api error (%s): %s", p.ProviderName, wr2.Error.Type, wr2.Error.Message)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected status %d: %s", p.ProviderName, httpResp.StatusCode, string(raw))
	}
	if len(wr2.Choices) == 0 {
		return nil, fmt.Errorf("%s: response contained no choices", p.ProviderName)
	}

	choice := wr2.Choices[0]
	resp := &ai.CompletionResponse{
		Text:       choice.Message.Content,
		Model:      wr2.Model,
		StopReason: choice.FinishReason,
		Usage: ai.Usage{
			InputTokens:  wr2.Usage.PromptTokens,
			OutputTokens: wr2.Usage.CompletionTokens,
		},
	}
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: args})
	}

	return resp, nil
}
