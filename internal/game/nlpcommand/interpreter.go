package nlpcommand

import (
	"context"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// Interpreter is the Milestone 3 entry point, matching the
// Planner/Advisor-style struct convention every sibling AI package
// (researchplanner.Planner, econadvisor.Advisor, ...) uses. It
// deliberately holds no *sql.DB, unlike every sibling package - see
// the package doc comment: this interpreter never reads or writes
// game state, only classifies intent.
type Interpreter struct {
	AI *ai.Service
}

// New builds an Interpreter around an already-wired ai.Service.
func New(service *ai.Service) *Interpreter {
	return &Interpreter{AI: service}
}

// Interpret sends one player message to the model as a tool-calling
// completion request and returns the classified Result. It is the
// final fallback in internal/bot/handlers/nlp.go's HandleTextMessage
// chain, called only after every hardcoded shortcut and lexical
// pattern has already failed to match.
func (in *Interpreter) Interpret(ctx context.Context, userID int64, text string) (Result, error) {
	if in == nil || in.AI == nil {
		return Result{}, fmt.Errorf("nlpcommand: interpreter not configured")
	}

	resp, err := in.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureCommandInterpreter),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: text}},
		Tools:       ToolDefinitions(),
		MaxTokens:   1024,
		Temperature: 0,
	})
	if err != nil {
		return Result{}, fmt.Errorf("nlpcommand: ai completion failed: %w", err)
	}

	// Every other AI feature in this codebase (governor, fleetcommander,
	// econadvisor, ...) intentionally shows the mock provider's
	// placeholder text, because the player explicitly tapped a
	// dedicated "AI Advisor" button and "no live AI configured yet" is
	// a meaningful answer to that. This interpreter is different: it's
	// the last link in an ordinary free-text chain, so a player who
	// just mistyped something never explicitly asked for an AI
	// feature. Surfacing mock's raw echo/placeholder text there would
	// read as a confusing implementation leak rather than a helpful
	// reply, so treat a mock response as "no match" and let the caller
	// fall through to its pre-Milestone-3 behavior instead.
	if resp.Provider == "mock" {
		return Result{}, nil
	}

	return ParseResponse(resp), nil
}
