package pipeline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/pipeline"
	"requirement-pipeline/internal/prompts"
	"requirement-pipeline/internal/providers"
	"requirement-pipeline/internal/repository"
	"requirement-pipeline/internal/stages"
)

func TestRunnerCompletesStagesAndPersistsArtifacts(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	defaultPrompts := seedPrompts(t, ctx, store)
	provider := providers.MockAIProvider{TranscriptionText: "meeting transcript", TextPrefix: "generated", Tokens: domain.TokenMetrics{InputTokens: 2, OutputTokens: 3, TotalTokens: 5}}
	runner := pipeline.NewRunner(store, stages.DefaultStages(provider, provider, prompts.Registry(defaultPrompts), stages.Models{Transcription: "gpt-4o-transcribe", Text: "gpt-5"}))

	run, err := runner.Run(ctx, pipeline.RunInput{Title: "Discovery", AudioFile: "meeting.mp3", Language: "pt-BR"})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if run.Status != domain.StatusCompleted {
		t.Fatalf("run status = %s, want completed", run.Status)
	}

	audit, err := store.ReconstructRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("reconstruct run: %v", err)
	}
	if len(audit.Stages) != 7 {
		t.Fatalf("stage count = %d, want 7", len(audit.Stages))
	}
	wantOrder := []domain.StageName{
		domain.StageAudioTranscription,
		domain.StageRequirementExtraction,
		domain.StageRequirementReview,
		domain.StageGapAnalysis,
		domain.StageRequirementRefinement,
		domain.StageDiagramGeneration,
		domain.StageArtifactGeneration,
	}
	for i, want := range wantOrder {
		if audit.Stages[i].Name != want {
			t.Fatalf("stage %d = %s, want %s", i, audit.Stages[i].Name, want)
		}
		if audit.Stages[i].Status != domain.StatusCompleted {
			t.Fatalf("stage %s status = %s", audit.Stages[i].Name, audit.Stages[i].Status)
		}
	}
	if len(audit.Artifacts) != 10 {
		t.Fatalf("artifact count = %d, want 10", len(audit.Artifacts))
	}
	if audit.Stages[1].PromptVersion != prompts.VersionV1 {
		t.Fatalf("prompt version = %q, want %q", audit.Stages[1].PromptVersion, prompts.VersionV1)
	}
	if audit.Stages[1].Model != "gpt-5" {
		t.Fatalf("model = %q, want gpt-5", audit.Stages[1].Model)
	}
	if audit.Stages[1].Tokens.TotalTokens != 5 {
		t.Fatalf("tokens = %+v, want total 5", audit.Stages[1].Tokens)
	}
}

func TestRunnerStopsAfterFailedStage(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	defaultPrompts := seedPrompts(t, ctx, store)
	provider := providers.MockAIProvider{TranscriptionText: "meeting transcript", GenerationErr: errors.New("provider failed")}
	runner := pipeline.NewRunner(store, stages.DefaultStages(provider, provider, prompts.Registry(defaultPrompts), stages.Models{Transcription: "gpt-4o-transcribe", Text: "gpt-5"}))

	run, err := runner.Run(ctx, pipeline.RunInput{Title: "Discovery", AudioFile: "meeting.mp3", Language: "pt-BR"})
	if err == nil {
		t.Fatal("expected failure")
	}
	storedRun, getErr := store.GetPipelineRun(ctx, run.ID)
	if getErr != nil {
		t.Fatalf("get run: %v", getErr)
	}
	if storedRun.Status != domain.StatusFailed {
		t.Fatalf("run status = %s, want failed", storedRun.Status)
	}
	stages, listErr := store.ListStagesByRun(ctx, run.ID)
	if listErr != nil {
		t.Fatalf("list stages: %v", listErr)
	}
	if len(stages) != 2 {
		t.Fatalf("stage count = %d, want 2", len(stages))
	}
	if stages[1].Name != domain.StageRequirementExtraction || stages[1].Status != domain.StatusFailed {
		t.Fatalf("failed stage = %+v", stages[1])
	}
}

func TestRunnerRejectsMissingAudio(t *testing.T) {
	store := repository.NewMemoryStore()
	runner := pipeline.NewRunner(store, nil)
	if _, err := runner.Run(context.Background(), pipeline.RunInput{Title: "No audio"}); err == nil {
		t.Fatal("expected missing audio error")
	}
}

func TestRunnerDefaultsNewMeetingFields(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	runner := pipeline.NewRunner(store, nil)

	run, err := runner.Run(ctx, pipeline.RunInput{AudioFile: "meeting.mp3"})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	meeting, err := store.GetMeeting(ctx, run.MeetingID)
	if err != nil {
		t.Fatalf("get meeting: %v", err)
	}
	if meeting.Title != "Untitled meeting" {
		t.Fatalf("title = %q, want default", meeting.Title)
	}
	if meeting.Language != "pt-BR" {
		t.Fatalf("language = %q, want pt-BR", meeting.Language)
	}
}

func TestRunnerReprocessesWithNewRun(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	defaultPrompts := seedPrompts(t, ctx, store)
	provider := providers.MockAIProvider{TranscriptionText: "meeting transcript"}
	runner := pipeline.NewRunner(store, stages.DefaultStages(provider, provider, prompts.Registry(defaultPrompts), stages.Models{Transcription: "gpt-4o-transcribe", Text: "gpt-5"}))

	first, err := runner.Run(ctx, pipeline.RunInput{Title: "Discovery", AudioFile: "meeting.mp3", Language: "pt-BR"})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	firstAudit, err := store.ReconstructRun(ctx, first.ID)
	if err != nil {
		t.Fatalf("first audit: %v", err)
	}
	second, err := runner.Run(ctx, pipeline.RunInput{MeetingID: firstAudit.Meeting.ID})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("reprocessing reused run id")
	}
	runs, err := store.ListPipelineRunsByMeeting(ctx, firstAudit.Meeting.ID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("run count = %d, want 2", len(runs))
	}
	reloadedFirst, err := store.GetPipelineRun(ctx, first.ID)
	if err != nil {
		t.Fatalf("reload first: %v", err)
	}
	if reloadedFirst.ID != first.ID || reloadedFirst.Status != domain.StatusCompleted {
		t.Fatalf("first run was modified unexpectedly: %+v", reloadedFirst)
	}
}

func seedPrompts(t *testing.T, ctx context.Context, store *repository.MemoryStore) []domain.Prompt {
	t.Helper()
	defaultPrompts := prompts.Defaults(time.Now().UTC())
	for _, prompt := range defaultPrompts {
		if _, err := store.CreatePrompt(ctx, prompt); err != nil {
			t.Fatalf("create prompt: %v", err)
		}
	}
	return defaultPrompts
}
