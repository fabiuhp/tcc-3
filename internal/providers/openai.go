package providers

import (
	"context"
	"errors"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/ports"
)

type OpenAIProvider struct {
	client openai.Client
}

func NewOpenAIProvider(apiKey string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY is required")
	}
	return &OpenAIProvider{client: openai.NewClient(option.WithAPIKey(apiKey))}, nil
}

func NewOpenAIProviderFromEnv() (*OpenAIProvider, error) {
	return NewOpenAIProvider(os.Getenv("OPENAI_API_KEY"))
}

func (p *OpenAIProvider) Transcribe(ctx context.Context, req ports.TranscriptionRequest) (ports.TranscriptionResponse, error) {
	file, err := os.Open(req.AudioFile)
	if err != nil {
		return ports.TranscriptionResponse{}, err
	}
	defer file.Close()

	transcription, err := p.client.Audio.Transcriptions.New(ctx, openai.AudioTranscriptionNewParams{
		File:  file,
		Model: req.Model,
	})
	if err != nil {
		return ports.TranscriptionResponse{}, err
	}
	return ports.TranscriptionResponse{
		Text:   transcription.Text,
		Model:  req.Model,
		Tokens: transcriptionTokenMetrics(transcription.Usage),
	}, nil
}

func (p *OpenAIProvider) Generate(ctx context.Context, req ports.TextGenerationRequest) (ports.TextGenerationResponse, error) {
	response, err := p.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: req.Model,
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(req.Prompt)},
	})
	if err != nil {
		return ports.TextGenerationResponse{}, err
	}
	return ports.TextGenerationResponse{
		Text:   response.OutputText(),
		Model:  req.Model,
		Tokens: domain.TokenMetrics{InputTokens: int(response.Usage.InputTokens), OutputTokens: int(response.Usage.OutputTokens), TotalTokens: int(response.Usage.TotalTokens)},
	}, nil
}

func transcriptionTokenMetrics(usage openai.TranscriptionUsageUnion) domain.TokenMetrics {
	if usage.Type != "tokens" {
		return domain.TokenMetrics{}
	}
	return domain.TokenMetrics{
		InputTokens:  int(usage.InputTokens),
		OutputTokens: int(usage.OutputTokens),
		TotalTokens:  int(usage.TotalTokens),
	}
}
