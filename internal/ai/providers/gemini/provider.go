// Package gemini implements ai.Provider against Google's Gemini API
// (https://generativelanguage.googleapis.com), which uses a distinct
// wire format from the OpenAI-Chat-Completions family (see
// internal/ai/providers/openaicompat) — different role names ("model"
// instead of "assistant"), a top-level systemInstruction field instead
// of a system-role message, and a "parts" array per content item.
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

// Provider implements ai.Provider against the Gemini generateContent API.
type Provider struct {
	APIKey       string
	DefaultModel string
	HTTPClient   *http.Client
	// BaseURL defaults to Google's real endpoint; overridable so tests
	// can point this at a local httptest server instead of requiring
	// live network access to verify the wire format end-to-end.
	BaseURL string
}

// New builds a Gemini provider. apiKey may be empty; Available() will
// correctly report false in that case.
//
// Default model note (confirmed via web search 2026-07-16, superseding
// this package's earlier 2026-07-15 note): Google shipped an entire
// new generation since gemini-2.5-flash was last verified as current —
// Gemini 3.5 launched at I/O on 2026-05-19, with gemini-3.5-flash
// reaching general availability the same day ($1.50 input / $9.00
// output per million tokens, confirmed against a dedicated pricing
// tracker dated 2026-05-22). gemini-2.5-flash likely still works (no
// deprecation was found for it specifically) but is no longer the
// current generation. Gemini 3.5 Pro is NOT yet generally available
// as of this check — it remains in limited enterprise preview, with a
// rumored (not Google-confirmed) July 17, 2026 GA date circulating in
// the press. Do not configure GEMINI_MODEL=gemini-3.5-pro until that
// is independently confirmed generally available.
func New(apiKey, defaultModel string) *Provider {
	if defaultModel == "" {
		defaultModel = "gemini-3.5-flash"
	}
	return &Provider{
		APIKey:       apiKey,
		DefaultModel: defaultModel,
		HTTPClient:   &http.Client{Timeout: 60 * time.Second},
		BaseURL:      defaultBaseURL,
	}
}

func (p *Provider) Name() string    { return "gemini" }
func (p *Provider) Available() bool { return p.APIKey != "" }

// wire types mirror the Gemini generateContent request/response shape.

type wirePart struct {
	Text         string        `json:"text,omitempty"`
	FunctionCall *wireFuncCall `json:"functionCall,omitempty"`
}

type wireFuncCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type wireContent struct {
	Role  string     `json:"role,omitempty"` // "user" or "model"; omitted for systemInstruction
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

// toGeminiRole maps internal/ai's Role to Gemini's "user"/"model"
// vocabulary. RoleTool is folded into "user", with the same caveat as
// every other provider in this codebase — see ADR-006 in
// PROJECT_MASTER_PLAN.md.
func toGeminiRole(r ai.Role) string {
	if r == ai.RoleAssistant {
		return "model"
	}
	return "user"
}

func (p *Provider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	if !p.Available() {
		return nil, fmt.Errorf("gemini: no API key configured")
	}

	model := req.Model
	if model == "" {
		model = p.DefaultModel
	}
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
		// Belt-and-suspenders instruction alongside the native JSON
		// mime type, matching every other provider's approach.
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
		return nil, fmt.Errorf("gemini: api error (%s): %s", wr2.Error.Status, wr2.Error.Message)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini: unexpected status %d: %s", httpResp.StatusCode, string(raw))
	}
	if len(wr2.Candidates) == 0 {
		return nil, fmt.Errorf("gemini: response contained no candidates")
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
				// Gemini's functionCall has no per-call ID field the
				// way OpenAI/Anthropic do; leave ToolCall.ID empty.
				// Flagged as a minor cross-provider inconsistency —
				// fine for now since no tool-execution loop exists yet
				// (ADR-006), revisit if/when one is built.
			})
		}
	}

	return resp, nil
}
