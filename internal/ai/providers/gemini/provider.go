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
	// ModelFallbacks are additional model names tried in order, within
	// this same provider, if DefaultModel (or req.Model) comes back
	// quota-exhausted or overloaded - see Complete's isRetryableModelError.
	// Google's free-tier quotas are tracked per model, not per account,
	// so a different model is frequently still available even when the
	// configured one is exhausted (confirmed live via Render logs,
	// 2026-08-02: gemini-3.5-flash hit RESOURCE_EXHAUSTED on its 20-
	// request/day free allotment). This only ever cycles models within
	// Gemini - if every model here is exhausted too, Complete returns
	// the last error and internal/ai.Service's own provider-level
	// fallback (Registry.Ordered) takes over, moving on to Qwen/mock.
	ModelFallbacks []string
	HTTPClient     *http.Client
	// BaseURL defaults to Google's real endpoint; overridable so tests
	// can point this at a local httptest server instead of requiring
	// live network access to verify the wire format end-to-end.
	BaseURL string
}

// New builds a Gemini provider. apiKey may be empty; Available() will
// correctly report false in that case. modelFallbacks may be nil/empty;
// see ModelFallbacks' doc comment.
//
// Default model note (confirmed via web search 2026-08-05, superseding
// this package's 2026-07-16 note): gemini-3.5-flash (this default)
// remains current with no shutdown date announced per Google's own
// deprecations page. Google has since shipped Gemini 3.6 Flash and
// Gemini 3.5 Flash-Lite to general availability (~2026-07-30) as the
// new default fallback models below, replacing the 2.5-generation
// names this package shipped with initially - gemini-2.5-flash-lite
// was confirmed dead in production via /ai_probe on 2026-08-05
// ("model gemini-2.5-flash-lite is no longer available to new users"),
// and the wider 2.0/2.5 Flash family is being wound down through 2026
// per Google's changelog. gemini-3.1-flash-lite (the third fallback)
// is confirmed still valid through at least May 2027.
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

// isRetryableModelError reports whether an error from one Gemini model
// is worth retrying against a different model in ModelFallbacks, versus
// an error that would just as surely fail again with any model (bad
// API key, malformed request, network failure) - in which case Complete
// returns immediately so internal/ai.Service's provider-level fallback
// can move on to the next *provider* without wasting a request per
// fallback model first. RESOURCE_EXHAUSTED/429 is Google's quota-hit
// status (confirmed live via Render logs, 2026-08-02); UNAVAILABLE/503
// is transient overload - both are specific to the model that was
// asked for, not the account or the request. NOT_FOUND/404 (confirmed
// live via /ai_probe, 2026-08-05: "model gemini-2.5-flash-lite is no
// longer available to new users") is the same category for a different
// reason - Google periodically retires specific model IDs, and a
// retired *fallback* model name is a stale config value, not a request
// problem, so the sensible behavior is to skip past it to the next
// configured fallback rather than let one dead model name in
// ModelFallbacks permanently block every model listed after it. This
// only ever cycles within the caller-configured list, so a genuinely
// wrong model name still surfaces as an error once every fallback is
// exhausted - it just no longer masks working models later in the list.
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
	// req.Model (a specific caller-requested model) is honored as the
	// first attempt, but ModelFallbacks still applies after it - a
	// caller asking for a specific model doesn't want a totally silent
	// account-wide outage just because that one model is exhausted.
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
			continue // this model is exhausted/overloaded specifically - try the next one
		}
	}
	return nil, lastErr
}

// modelError carries the Gemini-reported status/HTTP code alongside the
// formatted message, so Complete's retry loop can distinguish "try a
// different model" from "give up" without re-parsing error strings.
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
