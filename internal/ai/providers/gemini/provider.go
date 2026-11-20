package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

type Provider struct {
	APIKey         string
	DefaultModel   string
	ModelFallbacks []string
	HTTPClient     *http.Client
	BaseURL        string
}

func New(apiKey, defaultModel string, modelFallbacks []string) *Provider {
	if defaultModel == "" {
		defaultModel = "gemini-3.5-flash"
	}
	return &Provider{
		APIKey:         apiKey,
		DefaultModel:   defaultModel,
		ModelFallbacks: modelFallbacks,
		HTTPClient:     &http.Client{Timeout: 60 * time.Second},
		BaseURL:        defaultBaseURL,
	}
}

func (p *Provider) Name() string    { return "gemini" }
func (p *Provider) Available() bool { return p.APIKey != "" }

type wirePart struct {
	Text         string        `json:"text,omitempty"`
	FunctionCall *wireFuncCall `json:"functionCall,omitempty"`
}

type wireFuncCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type wireContent struct {
	Role  string     `json:"role,omitempty"`
	Parts []wirePart `json:"parts"`
}

type wireFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type wireTool struct {
	FunctionDeclarations []wireFunctionDeclaration `json:"functionDeclarations"`
}

type wireGenerationConfig struct {
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type wireRequest struct {
	SystemInstruction *wireContent          `json:"systemInstruction,omitempty"`
	Contents          []wireContent         `json:"contents"`
	Tools             []wireTool            `json:"tools,omitempty"`
	GenerationConfig  *wireGenerationConfig `json:"generationConfig,omitempty"`
}

type wireCandidate struct {
	Content      wireContent `json:"content"`
	FinishReason string      `json:"finishReason"`
}

type wireUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

type wireResponse struct {
	Candidates    []wireCandidate   `json:"candidates"`
	UsageMetadata wireUsageMetadata `json:"usageMetadata"`
	Error         *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func toGeminiRole(r ai.Role) string {
	if r == ai.RoleAssistant {
		return "model"
	}
	return "user"
}

func isRetryableModelError(status string, httpStatusCode int) bool {
	switch status {
	case "RESOURCE_EXHAUSTED", "UNAVAILABLE", "NOT_FOUND":
		return true
	}
	return httpStatusCode == http.StatusTooManyRequests || httpStatusCode == http.StatusServiceUnavailable || httpStatusCode == http.StatusNotFound
}

func (p *Provider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	if !p.Available() {
		return nil, fmt.Errorf("gemini: no API key configured")
	}

	primary := req.Model
	if primary == "" {
		primary = p.DefaultModel
	}
	models := []string{primary}
	for _, m := range p.ModelFallbacks {
		if m != primary {
			models = append(models, m)
		}
	}

	var lastErr error
	for i, model := range models {
		resp, err := p.completeWithModel(ctx, req, model)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		var status string
		var httpStatusCode int
		if me, ok := err.(*modelError); ok {
			status, httpStatusCode = me.status, me.httpStatusCode
		}
		if !isRetryableModelError(status, httpStatusCode) {
			return nil, err
		}
		if i < len(models)-1 {
			continue
		}
	}
	return nil, lastErr
}

type modelError struct {
	status         string
	httpStatusCode int
	msg            string
}

func (e *modelError) Error() string { return e.msg }

func (p *Provider) completeWithModel(ctx context.Context, req ai.CompletionRequest, model string) (*ai.CompletionResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	wr := wireRequest{
		GenerationConfig: &wireGenerationConfig{
			MaxOutputTokens: maxTokens,
			Temperature:     req.Temperature,
		},
	}

	system := req.System
	if req.JSONMode {
		wr.GenerationConfig.ResponseMimeType = "application/json"
		system = system + "\n\nRespond with a single valid JSON object and nothing else — no prose, no markdown fences."
	}
	if system != "" {
		wr.SystemInstruction = &wireContent{Parts: []wirePart{{Text: system}}}
	}

	for _, m := range req.Messages {
		wr.Contents = append(wr.Contents, wireContent{Role: toGeminiRole(m.Role), Parts: []wirePart{{Text: m.Content}}})
	}

	for _, t := range req.Tools {
		params := t.InputSchema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		wr.Tools = append(wr.Tools, wireTool{
			FunctionDeclarations: []wireFunctionDeclaration{{Name: t.Name, Description: t.Description, Parameters: params}},
		})
	}

	body, err := json.Marshal(wr)
	if err != nil {
		return nil, fmt.Errorf("gemini: marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s:generateContent?key=%s", p.BaseURL, url.PathEscape(model), url.QueryEscape(p.APIKey))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gemini: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: request failed: %w", err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	var wr2 wireResponse
	if err := json.Unmarshal(raw, &wr2); err != nil {
		return nil, fmt.Errorf("gemini: decode response (status %d): %w", httpResp.StatusCode, err)
	}
	if wr2.Error != nil {
		return nil, &modelError{
			status:         wr2.Error.Status,
			httpStatusCode: httpResp.StatusCode,
			msg:            fmt.Sprintf("gemini: api error (%s) on model %s: %s", wr2.Error.Status, model, wr2.Error.Message),
		}
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, &modelError{
			httpStatusCode: httpResp.StatusCode,
			msg:            fmt.Sprintf("gemini: unexpected status %d on model %s: %s", httpResp.StatusCode, model, string(raw)),
		}
	}
	if len(wr2.Candidates) == 0 {
		return nil, fmt.Errorf("gemini: response contained no candidates (model %s)", model)
	}

	candidate := wr2.Candidates[0]
	resp := &ai.CompletionResponse{
		Model:      model,
		StopReason: candidate.FinishReason,
		Usage: ai.Usage{
			InputTokens:  wr2.UsageMetadata.PromptTokenCount,
			OutputTokens: wr2.UsageMetadata.CandidatesTokenCount,
		},
	}
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			resp.Text += part.Text
		}
		if part.FunctionCall != nil {
			resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		}
	}

	return resp, nil
}
