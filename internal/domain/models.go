package domain

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusReady     Status = "ready"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type StageName string

const (
	StageAudioTranscription    StageName = "audio_transcription"
	StageRequirementExtraction StageName = "requirement_extraction"
	StageRequirementReview     StageName = "requirement_review"
	StageGapAnalysis           StageName = "gap_analysis"
	StageRequirementRefinement StageName = "requirement_refinement"
	StageDiagramGeneration     StageName = "diagram_generation"
	StageArtifactGeneration    StageName = "artifact_generation"
)

type ArtifactType string

const (
	ArtifactTranscript           ArtifactType = "transcript"
	ArtifactRequirementDraft     ArtifactType = "requirement_draft"
	ArtifactReviewedRequirements ArtifactType = "reviewed_requirements"
	ArtifactGapAnalysis          ArtifactType = "gap_analysis"
	ArtifactRefinedRequirements  ArtifactType = "refined_requirements"
	ArtifactBusinessDiagrams     ArtifactType = "business_diagrams"
	ArtifactSRS                  ArtifactType = "software_requirement_specification"
	ArtifactUserStories          ArtifactType = "user_stories"
	ArtifactAcceptanceCriteria   ArtifactType = "acceptance_criteria"
	ArtifactUseCases             ArtifactType = "use_cases"
)

type TokenMetrics struct {
	InputTokens  int `bson:"input_tokens" json:"input_tokens"`
	OutputTokens int `bson:"output_tokens" json:"output_tokens"`
	TotalTokens  int `bson:"total_tokens" json:"total_tokens"`
}

type Meeting struct {
	ID        string    `bson:"_id" json:"id"`
	Title     string    `bson:"title" json:"title"`
	AudioFile string    `bson:"audio_file" json:"audio_file"`
	Language  string    `bson:"language" json:"language"`
	Status    Status    `bson:"status" json:"status"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

type PipelineRun struct {
	ID            string     `bson:"_id" json:"id"`
	MeetingID     string     `bson:"meeting_id" json:"meeting_id"`
	StartedAt     time.Time  `bson:"started_at" json:"started_at"`
	FinishedAt    *time.Time `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
	Status        Status     `bson:"status" json:"status"`
	FailedStageID string     `bson:"failed_stage_id,omitempty" json:"failed_stage_id,omitempty"`
	Error         string     `bson:"error,omitempty" json:"error,omitempty"`
}

type StageExecution struct {
	ID                string       `bson:"_id" json:"id"`
	RunID             string       `bson:"run_id" json:"run_id"`
	Name              StageName    `bson:"name" json:"name"`
	Status            Status       `bson:"status" json:"status"`
	StartedAt         time.Time    `bson:"started_at" json:"started_at"`
	FinishedAt        *time.Time   `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
	Model             string       `bson:"model,omitempty" json:"model,omitempty"`
	DurationMillis    int64        `bson:"duration_millis" json:"duration_millis"`
	Tokens            TokenMetrics `bson:"tokens" json:"tokens"`
	InputArtifactIDs  []string     `bson:"input_artifact_ids" json:"input_artifact_ids"`
	OutputArtifactIDs []string     `bson:"output_artifact_ids" json:"output_artifact_ids"`
	Error             string       `bson:"error,omitempty" json:"error,omitempty"`
}

type Artifact struct {
	ID        string       `bson:"_id" json:"id"`
	StageID   string       `bson:"stage_id" json:"stage_id"`
	Type      ArtifactType `bson:"type" json:"type"`
	Content   string       `bson:"content" json:"content"`
	CreatedAt time.Time    `bson:"created_at" json:"created_at"`
}

type Prompt struct {
	ID          string    `bson:"_id" json:"id"`
	StageName   StageName `bson:"stage_name" json:"stage_name"`
	Description string    `bson:"description" json:"description"`
	Template    string    `bson:"template" json:"template"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
}

type AuditRun struct {
	Meeting   Meeting          `json:"meeting"`
	Run       PipelineRun      `json:"run"`
	Stages    []StageExecution `json:"stages"`
	Artifacts []Artifact       `json:"artifacts"`
	Prompts   []Prompt         `json:"prompts"`
}

func NewID() string {
	return uuid.NewString()
}
