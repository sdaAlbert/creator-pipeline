package video

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	creator "creator-pipeline/internal/eino"
)

type GenerationRequest struct {
	TaskID  string               `json:"task_id"`
	Prompt  string               `json:"prompt"`
	Plan    creator.CreationPlan `json:"plan"`
	Attempt int                  `json:"attempt"`
}

type GenerationResult struct {
	Payload []byte
}

type VideoGenerator interface {
	Generate(ctx context.Context, req GenerationRequest) (GenerationResult, error)
}

type MockGenerator struct {
	DefaultDelay time.Duration
	SlowDelay    time.Duration
}

func NewMockGenerator() *MockGenerator {
	return &MockGenerator{
		DefaultDelay: 500 * time.Millisecond,
		SlowDelay:    5 * time.Second,
	}
}

func (g *MockGenerator) Generate(ctx context.Context, req GenerationRequest) (GenerationResult, error) {
	lower := strings.ToLower(req.Prompt)
	delay := g.DefaultDelay
	if delay <= 0 {
		delay = 500 * time.Millisecond
	}
	if strings.Contains(lower, "slow") {
		delay = g.SlowDelay
		if delay <= 0 {
			delay = 5 * time.Second
		}
	}

	select {
	case <-ctx.Done():
		return GenerationResult{}, ctx.Err()
	case <-time.After(delay):
	}

	if strings.Contains(lower, "fail") {
		return GenerationResult{}, errors.New("mock video generator returned generation error")
	}

	payload, err := json.Marshal(map[string]any{
		"task_id": req.TaskID,
		"prompt":  req.Prompt,
		"plan":    req.Plan,
	})
	if err != nil {
		return GenerationResult{}, err
	}
	return GenerationResult{Payload: payload}, nil
}
