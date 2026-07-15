// Package openaicompat implements ai.Provider once against the
// OpenAI Chat Completions wire format
// (POST {base_url}/chat/completions), which is also exposed, with
// only cosmetic differences, by:
//   - OpenAI itself            (https://api.openai.com/v1)
//   - DeepSeek                 (https://api.deepseek.com/v1)
//   - Alibaba Qwen / DashScope compatible mode
//     (https://dashscope-intl.aliyuncs.com/compatible-mode/v1, or the
//     mainland-China endpoint dashscope.aliyuncs.com/compatible-mode/v1)
//   - xAI Grok                 (https://api.x.ai/v1)
//   - A locally-run Ollama instance's OpenAI-compatible endpoint
//     (http://<host>:11434/v1) — though internal/ai/providers/ollama
//     has its own dedicated implementation using Ollama's native API,
//     which supports a few Ollama-specific features this generic
//     client does not.
//
// One implementation, four (or more) providers — each is just this
// struct constructed with a different BaseURL/APIKey/DefaultModel/Name.
// This directly satisfies the roadmap's "Support future providers like
// OpenAI, ..., Qwen, Grok, DeepSeek, ..." requirement without writing
// near-duplicate HTTP clients four times.
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

// Provider implements ai.Provider against any OpenAI-Chat-Completions-
// compatible HTTP API.
type Provider struct {
	// ProviderName is what Name() returns — e.g. "openai", "deepseek",
	// "qwen", "grok". Distinct from the underlying model name.
	ProviderName string
	// BaseURL has no trailing slash, e.g. "https://api.openai.com/v1".
	BaseURL      string
	APIKey       string
	DefaultModel string
	// SupportsJSONResponseFormat controls whether {"response_format":
	// {"type":"json_object"}} is sent when CompletionRequest.JSONMode
	// is true. All four target APIs above support it as of this
	// writing, but the field exists so a future 5th OpenAI-compatible
	// provider that doesn't can be registered without a code change —
	// it will fall back to the same system-prompt-instruction approach
	// internal/ai/providers/anthropic uses.
	SupportsJSONResponseFormat bool

	HTTPClient *http.Client
}

// New builds an OpenAI-compatible provider. apiKey may be empty;
// Available() will correctly report false in that case.
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

// wire types mirror the OpenAI Chat Completions request/response shape.

type wireFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type wireTool struct {
	Type     string       `json:"type"` // always "function"
	Function wireFunction `json:"function"`
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON-encoded string, per OpenAI's format
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

	system := req.System
	if req.JSONMode {
		if p.SupportsJSONResponseFormat {
			wr.ResponseFormat = &wireResponseFormat{Type: "json_object"}
		}
		// Belt-and-suspenders regardless of native support, exactly
		// like the Anthropic provider does — costs nothing and
		// tolerates a misconfigured SupportsJSONResponseFormat flag.
		// Appended even when System was empty, so JSON mode is never
		// silently dropped just because the caller had no system
		// prompt of its own (a real bug caught by this package's own
		// tests — see provider_test.go).
		system = system + "\n\nRespond with a single valid JSON object and nothing else — no prose, no markdown fences."
	}
	if system != "" {
		wr.Messages = append(wr.Messages, wireMessage{Role: "system", Content: system})
	}
	for _, m := range req.Messages {
		role := string(m.Role)
		if role == string(ai.RoleTool) {
			// This generic client treats tool results as a labelled
			// user turn for now, matching the same simplification (and
			// same caveat — see ADR-006 in PROJECT_MASTER_PLAN.md) as
			// internal/ai/providers/anthropic. A dedicated tool-result
			// message type should be added to all providers together
			// when a real tool-execution loop is built.
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
			// Tool-call arguments arrive as a JSON-encoded string per
			// the OpenAI wire format, not a nested object — must be
			// unmarshaled a second time. A malformed/partial arguments
			// string (which happens if the model's output was
			// truncated) is not treated as a fatal error: the tool
			// call is still surfaced with an empty Input map so the
			// caller can decide how to handle it, rather than losing
			// the whole completion over one bad tool call.
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{ID: tc.ID, Name: tc.Function.Name, Input: args})
	}

	return resp, nil
}
