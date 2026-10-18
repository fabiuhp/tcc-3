package pipeline

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/ports"
	"github.com/fabiuhp/tcc-3/internal/transcripts"
)

type Stage interface {
	Name() domain.StageName
	Execute(context.Context, StageContext) (StageResult, error)
}

type StageError struct {
	Stage domain.StageName
	Err   error
}

func (e StageError) Error() string {
	return fmt.Sprintf("stage %s: %v", e.Stage, e.Err)
}

func (e StageError) Unwrap() error {
	return e.Err
}

type StageContext struct {
	Meeting   domain.Meeting
	Run       domain.PipelineRun
	StageID   string
	Artifacts []domain.Artifact
}

type StageResult struct {
	Artifacts        []domain.Artifact
	Model            string
	Tokens           domain.TokenMetrics
	ValidationIssues []domain.ValidationIssue
}

type Runner struct {
	store  ports.Store
	stages []Stage
	now    func() time.Time
}

type RunInput struct {
	MeetingID      string
	Title          string
	AudioFile      string
	Transcript     string
	TranscriptFile string
	Language       string
}

func NewRunner(store ports.Store, stages []Stage) *Runner {
	return &Runner{store: store, stages: stages, now: func() time.Time { return time.Now().UTC() }}
}

func (r *Runner) Run(ctx context.Context, input RunInput) (domain.PipelineRun, error) {
	if input.MeetingID != "" && (input.AudioFile != "" || input.Transcript != "" || input.TranscriptFile != "") {
		return domain.PipelineRun{}, errors.New("meeting id cannot be combined with audio or transcript input")
	}
	if input.AudioFile != "" && (input.Transcript != "" || input.TranscriptFile != "") {
		return domain.PipelineRun{}, errors.New("provide either audio or transcript, not both")
	}
	if input.Transcript != "" || input.TranscriptFile != "" {
		if err := transcripts.Validate(input.Transcript); err != nil {
			return domain.PipelineRun{}, err
		}
	}
	if input.AudioFile == "" && input.Transcript == "" && input.MeetingID == "" {
		return domain.PipelineRun{}, errors.New("audio file path or transcript is required")
	}

	meeting, err := r.resolveMeeting(ctx, input)
	if err != nil {
		return domain.PipelineRun{}, err
	}
	if meeting.Transcript != "" {
		if err := transcripts.Validate(meeting.Transcript); err != nil {
			return domain.PipelineRun{}, err
		}
	}

	run, err := r.store.CreatePipelineRun(ctx, domain.PipelineRun{
		ID:        domain.NewID(),
		MeetingID: meeting.ID,
		StartedAt: r.now(),
		Status:    domain.StatusRunning,
	})
	if err != nil {
		return domain.PipelineRun{}, err
	}

	artifacts := []domain.Artifact{}
	runStages := r.stages
	if meeting.Transcript != "" {
		runStages = []Stage{transcriptImportStage{}}
		for _, stage := range r.stages {
			if stage.Name() != domain.StageAudioTranscription {
				runStages = append(runStages, stage)
			}
		}
	}
	for _, pipelineStage := range runStages {
		stageStart := r.now()
		stage, err := r.store.CreateStage(ctx, domain.StageExecution{
			ID:               domain.NewID(),
			RunID:            run.ID,
			Name:             pipelineStage.Name(),
			Status:           domain.StatusRunning,
			StartedAt:        stageStart,
			InputArtifactIDs: artifactIDs(artifacts),
		})
		if err != nil {
			return domain.PipelineRun{}, err
		}

		result, execErr := pipelineStage.Execute(ctx, StageContext{Meeting: meeting, Run: run, StageID: stage.ID, Artifacts: artifacts})
		finished := r.now()
		stage.FinishedAt = &finished
		stage.DurationMillis = finished.Sub(stageStart).Milliseconds()
		stage.Model = result.Model
		stage.Tokens = result.Tokens
		stage.ValidationIssues = issuesForStage(result.ValidationIssues, pipelineStage.Name())
		run.ValidationIssues = append(run.ValidationIssues, stage.ValidationIssues...)

		if execErr != nil {
			stage.Status = domain.StatusFailed
			stage.Error = execErr.Error()
			_ = r.store.UpdateStage(ctx, stage)
			run.Status = domain.StatusFailed
			run.FinishedAt = &finished
			run.FailedStageID = stage.ID
			failure := StageError{Stage: pipelineStage.Name(), Err: execErr}
			run.Error = failure.Error()
			_ = r.store.UpdatePipelineRun(ctx, run)
			return run, failure
		}

		for i := range result.Artifacts {
			result.Artifacts[i].StageID = stage.ID
			if result.Artifacts[i].CreatedAt.IsZero() {
				result.Artifacts[i].CreatedAt = finished
			}
		}
		created, err := r.store.CreateArtifacts(ctx, result.Artifacts)
		if err != nil {
			return domain.PipelineRun{}, err
		}
		stage.Status = domain.StatusCompleted
		if len(stage.ValidationIssues) > 0 {
			stage.Status = domain.StatusCompletedWithIssues
		}
		stage.OutputArtifactIDs = artifactIDs(created)
		if err := r.store.UpdateStage(ctx, stage); err != nil {
			return domain.PipelineRun{}, err
		}
		artifacts = append(artifacts, created...)
	}

	finished := r.now()
	run.Status = domain.StatusCompleted
	if len(run.ValidationIssues) > 0 {
		run.Status = domain.StatusCompletedWithIssues
		run.Error = fmt.Sprintf("pipeline completed with %d validation issue(s)", len(run.ValidationIssues))
	}
	run.FinishedAt = &finished
	if err := r.store.UpdatePipelineRun(ctx, run); err != nil {
		return domain.PipelineRun{}, err
	}
	return run, nil
}

func (r *Runner) resolveMeeting(ctx context.Context, input RunInput) (domain.Meeting, error) {
	if input.MeetingID != "" {
		meeting, err := r.store.GetMeeting(ctx, input.MeetingID)
		if err != nil {
			return domain.Meeting{}, err
		}
		return meeting, nil
	}
	if input.AudioFile == "" && input.Transcript == "" {
		return domain.Meeting{}, errors.New("audio file path or transcript is required")
	}
	return r.store.CreateMeeting(ctx, domain.Meeting{
		ID:             domain.NewID(),
		Title:          cmp.Or(input.Title, "Untitled meeting"),
		AudioFile:      input.AudioFile,
		Transcript:     input.Transcript,
		TranscriptFile: input.TranscriptFile,
		Language:       cmp.Or(input.Language, "pt-BR"),
		Status:         domain.StatusReady,
		CreatedAt:      r.now(),
	})
}

type transcriptImportStage struct{}

func (transcriptImportStage) Name() domain.StageName { return domain.StageTranscriptImport }

func (transcriptImportStage) Execute(_ context.Context, ctx StageContext) (StageResult, error) {
	return StageResult{Artifacts: []domain.Artifact{{
		ID: domain.NewID(), Type: domain.ArtifactTranscript, Content: ctx.Meeting.Transcript,
	}}}, nil
}

func MustFindArtifact(artifacts []domain.Artifact, artifactType domain.ArtifactType) (domain.Artifact, error) {
	for i := len(artifacts) - 1; i >= 0; i-- {
		if artifacts[i].Type == artifactType {
			return artifacts[i], nil
		}
	}
	return domain.Artifact{}, fmt.Errorf("artifact %s not found", artifactType)
}

func artifactIDs(artifacts []domain.Artifact) []string {
	ids := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		ids = append(ids, artifact.ID)
	}
	return ids
}

func issuesForStage(issues []domain.ValidationIssue, stage domain.StageName) []domain.ValidationIssue {
	if len(issues) == 0 {
		return nil
	}
	assigned := make([]domain.ValidationIssue, len(issues))
	copy(assigned, issues)
	for i := range assigned {
		if assigned[i].Stage == "" {
			assigned[i].Stage = stage
		}
	}
	return assigned
}
