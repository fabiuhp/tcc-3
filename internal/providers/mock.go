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
	GenerationText    string
	GenerationFunc    func(ports.TextGenerationRequest) (ports.TextGenerationResponse, error)
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
	if m.GenerationFunc != nil {
		return m.GenerationFunc(req)
	}
	if m.GenerationText != "" {
		return ports.TextGenerationResponse{Text: m.GenerationText, Model: req.Model, Tokens: m.Tokens}, nil
	}
	prefix := m.TextPrefix
	if prefix == "" {
		prefix = "mock output"
	}
	return ports.TextGenerationResponse{Text: fmt.Sprintf("%s: %s", prefix, req.Prompt), Model: req.Model, Tokens: m.Tokens}, nil
}
