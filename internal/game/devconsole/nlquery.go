// This file implements natural-language admin queries: an admin types
// a free-text question, and gets a grounded answer. See queries.go's
// doc comment for the safety model this depends on — the model never
// writes or executes SQL; it only ever picks a name from a fixed
// whitelist plus two bounded integers.
//
// This is a two-call flow, unlike every other Phase B-J feature's
// single call:
//  1. Classify: given the admin's question and the whitelist
//     (IntentDescriptions), the model returns {"intent": "...",
//     "days": N, "limit": N}. Validated against the whitelist and
//     clamped to safe bounds before anything runs.
//  2. Answer: given the admin's original question and the actual
//     query result text (never anything the model wrote), the model
//     produces a final natural-language answer grounded in those real
//     numbers.
//
// Two calls cost more than one, but there's no safe way to compress
// this to one call without either letting the model's raw output
// drive execution (unsafe) or answering before the real numbers exist
// (ungrounded/hallucination-prone).
package devconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// NLQueryMemoryScope is used for both calls in the natural-language
// query flow, distinct from MemoryScope (the weekly-report scope).
const NLQueryMemoryScope = "dev_console_nlquery"

const (
	minDays, maxDays   = 1, 90
	minLimit, maxLimit = 1, 25
)

func ClampDays(n int) int {
	if n < minDays {
		return 7
	}
	if n > maxDays {
		return maxDays
	}
	return n
}

func ClampLimit(n int) int {
	if n < minLimit {
		return 5
	}
	if n > maxLimit {
		return maxLimit
	}
	return n
}

// IntentClassification is the model's raw choice — validated by
// ClassifyIntent before ever being used to run anything.
type IntentClassification struct {
	Intent string `json:"intent"`
	Days   int    `json:"days"`
	Limit  int    `json:"limit"`

	FellBackToRawText bool
	Truncated         bool
}

// ClassificationSystemPrompt is the fixed instruction for the
// intent-classification call.
const ClassificationSystemPrompt = `You are a query classifier for The Vagabond's admin console. Given an admin's free-text question, pick EXACTLY ONE intent from this fixed list that would answer it — never invent a new intent name:

` + "REPLACED_AT_INIT" + `

Respond with a single JSON object: {"intent": "<one of the names above>", "days": <integer, only relevant for time-windowed intents, otherwise 0>, "limit": <integer, only relevant for intents returning a list, otherwise 0>}. If the question doesn't clearly match any intent, pick the closest reasonable one — never leave intent blank.`

func ClassificationSystemPromptText() string {
	return strings.Replace(ClassificationSystemPrompt, "REPLACED_AT_INIT", IntentDescriptions(), 1)
}

func BuildClassificationUserPrompt(question string) string {
	return fmt.Sprintf("Admin's question: %q", question)
}

// parseClassification decodes the classifier's response text,
// tolerating a markdown code fence the same way every other Phase B-J
// package does.
func ParseClassification(text string) *IntentClassification {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &IntentClassification{FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
	}
	var ic IntentClassification
	if err := json.Unmarshal([]byte(candidate), &ic); err == nil && ic.Intent != "" {
		return &ic
	}
	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &ic); err == nil && ic.Intent != "" {
		return &ic
	}
	return &IntentClassification{FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
}

// AnswerRecommendation is the final, grounded natural-language answer.
type AnswerRecommendation struct {
	Answer  string `json:"answer"`
	Caveats string `json:"caveats"`

	FellBackToRawText bool
	Truncated         bool
}

// AnswerSystemPrompt is the fixed instruction for the second
// (grounded-answer) call.
const AnswerSystemPrompt = `You are the AI Developer Console for The Vagabond, answering an admin's question using ONLY the real data provided below — never add numbers, names, or facts that aren't in it. You NEVER take any action yourself — you only report.

Rules:
- If the data doesn't fully answer the question, say what it does show and what it doesn't, rather than guessing at the rest.
- caveats should note anything about the data worth flagging (e.g. a capped list, a small sample, or a data-collection limit like "this game doesn't track real-world location").
- Keep the answer conversational and direct — this is a chat reply to a specific question, not a report with headers.`

func BuildAnswerUserPrompt(question, dataBlock string) string {
	return fmt.Sprintf("Admin's question: %q\n\nREAL DATA:\n%s\n\nRespond with a single JSON object matching this shape exactly:\n"+
		`{"answer": "...", "caveats": "..."}`, question, dataBlock)
}

func ParseAnswer(text string) *AnswerRecommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &AnswerRecommendation{Answer: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
	}
	var rec AnswerRecommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Answer != "" {
		return &rec
	}
	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Answer != "" {
		return &rec
	}
	return &AnswerRecommendation{Answer: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
}

// FormatAnswerForTelegram renders an AnswerRecommendation as a
// plain-text Telegram reply.
func FormatAnswerForTelegram(rec *AnswerRecommendation) string {
	var b strings.Builder
	b.WriteString("🖥️ AI DEVELOPER CONSOLE\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ The AI's response got cut off before it finished — showing the partial reply below:\n\n")
		} else {
			b.WriteString("⚠️ Couldn't parse the AI's structured response — showing its raw reply below:\n\n")
		}
		fmt.Fprintf(&b, "```\n%s\n```", rec.Answer)
		return b.String()
	}

	b.WriteString(rec.Answer)
	if rec.Caveats != "" {
		fmt.Fprintf(&b, "\n\n📝 %s", rec.Caveats)
	}
	return b.String()
}

// Ask runs the full classify → execute → answer flow for one
// free-text admin question. It stores all turns in ai_memory under
// NLQueryMemoryScope.
//
// Read-only end to end: every intent in queryIntents is a SELECT (see
// queries.go), and nothing the model outputs is ever used as
// executable code — only as a validated choice from a fixed list plus
// two clamped integers.
func (co *Console) Ask(ctx context.Context, callerUserID int64, question string) (*AnswerRecommendation, error) {
	classifyPrompt := BuildClassificationUserPrompt(question)

	classifyResp, err := co.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureDevConsole),
		UserID:      callerUserID,
		System:      ClassificationSystemPromptText(),
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: classifyPrompt}},
		MaxTokens:   256,
		Temperature: 0.1,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("devconsole: classification call failed: %w", err)
	}

	classification := ParseClassification(classifyResp.Text)
	if classification.FellBackToRawText || !IsKnownIntent(classification.Intent) {
		return &AnswerRecommendation{
			Answer: "I couldn't match that question to anything I know how to look up. Try asking about new players, top players, active users, totals, the economy, combat stats, clans, world state, or recent news.",
		}, nil
	}

	days := ClampDays(classification.Days)
	limit := ClampLimit(classification.Limit)

	dataBlock, err := co.RunIntent(ctx, classification.Intent, days, limit)
	if err != nil {
		return nil, fmt.Errorf("devconsole: running intent %q: %w", classification.Intent, err)
	}

	answerPrompt := BuildAnswerUserPrompt(question, dataBlock)

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, NLQueryMemoryScope, ai.Message{Role: ai.RoleUser, Content: question})
	}

	answerResp, err := co.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureDevConsole),
		UserID:      callerUserID,
		System:      AnswerSystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: answerPrompt}},
		MaxTokens:   1024,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("devconsole: answer call failed: %w", err)
	}

	rec := ParseAnswer(answerResp.Text)

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, NLQueryMemoryScope, ai.Message{Role: ai.RoleAssistant, Content: answerResp.Text})
	}

	return rec, nil
}
