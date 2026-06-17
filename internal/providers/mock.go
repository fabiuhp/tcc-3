package providers

import (
	"context"
	"fmt"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/ports"
)

type MockAIProvider struct {
	TranscriptionText string
	TextPrefix        string
	Tokens            domain.TokenMetrics
	TranscriptionErr  error
	GenerationErr     error
}

func (m MockAIProvider) Transcribe(_ context.Context, req ports.TranscriptionRequest) (ports.TranscriptionResponse, error) {
	if m.TranscriptionErr != nil {
		return ports.TranscriptionResponse{}, m.TranscriptionErr
	}
	text := m.TranscriptionText
	if text == "" {
		text = "mock transcript"
	}
	return ports.TranscriptionResponse{Text: text, Model: req.Model, Tokens: m.Tokens}, nil
}

func (m MockAIProvider) Generate(_ context.Context, req ports.TextGenerationRequest) (ports.TextGenerationResponse, error) {
	if m.GenerationErr != nil {
		return ports.TextGenerationResponse{}, m.GenerationErr
	}
	prefix := m.TextPrefix
	if prefix == "" {
		prefix = "mock output"
	}
	return ports.TextGenerationResponse{Text: fmt.Sprintf("%s: %s", prefix, req.Prompt), Model: req.Model, Tokens: m.Tokens}, nil
}
