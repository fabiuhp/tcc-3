package ports

import (
	"context"

	"requirement-pipeline/internal/domain"
)

type MeetingRepository interface {
	CreateMeeting(context.Context, domain.Meeting) (domain.Meeting, error)
	GetMeeting(context.Context, string) (domain.Meeting, error)
}

type PipelineRunRepository interface {
	CreatePipelineRun(context.Context, domain.PipelineRun) (domain.PipelineRun, error)
	UpdatePipelineRun(context.Context, domain.PipelineRun) error
	GetPipelineRun(context.Context, string) (domain.PipelineRun, error)
	ListPipelineRunsByMeeting(context.Context, string) ([]domain.PipelineRun, error)
}

type StageRepository interface {
	CreateStage(context.Context, domain.StageExecution) (domain.StageExecution, error)
	UpdateStage(context.Context, domain.StageExecution) error
	ListStagesByRun(context.Context, string) ([]domain.StageExecution, error)
}

type ArtifactRepository interface {
	CreateArtifact(context.Context, domain.Artifact) (domain.Artifact, error)
	CreateArtifacts(context.Context, []domain.Artifact) ([]domain.Artifact, error)
	GetArtifact(context.Context, string) (domain.Artifact, error)
	ListArtifactsByStage(context.Context, string) ([]domain.Artifact, error)
}

type PromptRepository interface {
	CreatePrompt(context.Context, domain.Prompt) (domain.Prompt, error)
	GetLatestPromptByStage(context.Context, domain.StageName) (domain.Prompt, error)
	ListPrompts(context.Context) ([]domain.Prompt, error)
}

type AuditRepository interface {
	ReconstructRun(context.Context, string) (domain.AuditRun, error)
}

type Store interface {
	MeetingRepository
	PipelineRunRepository
	StageRepository
	ArtifactRepository
	PromptRepository
	AuditRepository
}

type TranscriptionRequest struct {
	AudioFile string
	Language  string
	Model     string
}

type TranscriptionResponse struct {
	Text   string
	Model  string
	Tokens domain.TokenMetrics
}

type SpeechToTextProvider interface {
	Transcribe(context.Context, TranscriptionRequest) (TranscriptionResponse, error)
}

type TextGenerationRequest struct {
	Prompt string
	Model  string
}

type TextGenerationResponse struct {
	Text   string
	Model  string
	Tokens domain.TokenMetrics
}

type TextGenerationProvider interface {
	Generate(context.Context, TextGenerationRequest) (TextGenerationResponse, error)
}
