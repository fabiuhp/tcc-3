package providers

import (
	"testing"

	"github.com/openai/openai-go"

	"github.com/fabiuhp/tcc-3/internal/domain"
)

func TestTranscriptionTokenMetrics(t *testing.T) {
	usage := openai.TranscriptionUsageUnion{
		Type:         "tokens",
		InputTokens:  11,
		OutputTokens: 7,
		TotalTokens:  18,
	}

	if got, want := transcriptionTokenMetrics(usage), (domain.TokenMetrics{InputTokens: 11, OutputTokens: 7, TotalTokens: 18}); got != want {
		t.Fatalf("metrics = %+v, want %+v", got, want)
	}
}

func TestTranscriptionTokenMetricsIgnoresDurationUsage(t *testing.T) {
	usage := openai.TranscriptionUsageUnion{Type: "duration", Seconds: 42}

	if got := transcriptionTokenMetrics(usage); got != (domain.TokenMetrics{}) {
		t.Fatalf("metrics = %+v, want zero metrics", got)
	}
}
