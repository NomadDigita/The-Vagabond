package nlpcommand

import (
	"context"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type Interpreter struct {
	AI *ai.Service
}

func New(service *ai.Service) *Interpreter {
	return &Interpreter{AI: service}
}

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

	if resp.Provider == "mock" {
		return Result{}, nil
	}

	return ParseResponse(resp), nil
}
