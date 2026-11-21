package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type Provider struct {
	BaseURL      string
	DefaultModel string
	HTTPClient   *http.Client
}

func New(baseURL, defaultModel string) *Provider {
	if defaultModel == "" {
		defaultModel = "llama3.1"
	}
	return &Provider{
		BaseURL:      strings.TrimSuffix(baseURL, "/"),
		DefaultModel: defaultModel,
		HTTPClient:   &http.Client{Timeout: 180 * time.Second},
	}
}

func (p *Provider) Name() string    { return "ollama" }
func (p *Provider) Available() bool { return p.BaseURL != "" }

type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description,omitempty"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type wireToolCall struct {
	Function struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	} `json:"function"`
}

type wireMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []wireToolCall `json:"tool_calls,omitempty"`
}

type wireOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type wireRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Tools    []wireTool    `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format,omitempty"`
	Options  *wireOptions  `json:"options,omitempty"`
}

type wireResponse struct {
	Model           string      `json:"model"`
	Message         wireMessage `json:"message"`
	Done            bool        `json:"done"`
	DoneReason      string      `json:"done_reason"`
	PromptEvalCount int         `json:"prompt_eval_count"`
	EvalCount       int         `json:"eval_count"`
	Error           string      `json:"error"`
}

func (p *Provider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	if !p.Available() {
		return nil, fmt.Errorf("ollama: no base URL configured")
	}

	model := req.Model
	if model == "" {
		model = p.DefaultModel
	}

	wr := wireRequest{
		Model:  model,
		Stream: false,
		Options: &wireOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	system := req.System
	if req.JSONMode {
		wr.Format = "json"
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
		wt := wireTool{Type: "function"}
		wt.Function.Name = t.Name
		wt.Function.Description = t.Description
		wt.Function.Parameters = params
		wr.Tools = append(wr.Tools, wt)
	}

	body, err := json.Marshal(wr)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed (is the Ollama server reachable at %s?): %w", p.BaseURL, err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama: read response: %w", err)
	}

	var wr2 wireResponse
	if err := json.Unmarshal(raw, &wr2); err != nil {
		return nil, fmt.Errorf("ollama: decode response (status %d): %w", httpResp.StatusCode, err)
	}
	if wr2.Error != "" {
		return nil, fmt.Errorf("ollama: api error: %s", wr2.Error)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: unexpected status %d: %s", httpResp.StatusCode, string(raw))
	}

	resp := &ai.CompletionResponse{
		Text:       wr2.Message.Content,
		Model:      wr2.Model,
		StopReason: wr2.DoneReason,
		Usage: ai.Usage{
			InputTokens:  wr2.PromptEvalCount,
			OutputTokens: wr2.EvalCount,
		},
	}
	for _, tc := range wr2.Message.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{Name: tc.Function.Name, Input: tc.Function.Arguments})
	}

	return resp, nil
}
