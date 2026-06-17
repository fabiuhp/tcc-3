package repository_test

import (
	"context"
	"testing"
	"time"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/repository"
)

func TestMemoryStorePersistsAndReconstructsRun(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	now := time.Now().UTC()

	meeting, err := store.CreateMeeting(ctx, domain.Meeting{Title: "Discovery", AudioFile: "meeting.mp3", Language: "pt-BR", Status: domain.StatusReady, CreatedAt: now})
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	run, err := store.CreatePipelineRun(ctx, domain.PipelineRun{MeetingID: meeting.ID, StartedAt: now, Status: domain.StatusRunning})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	stage, err := store.CreateStage(ctx, domain.StageExecution{RunID: run.ID, Name: domain.StageAudioTranscription, StartedAt: now, Status: domain.StatusRunning})
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}
	artifact, err := store.CreateArtifact(ctx, domain.Artifact{StageID: stage.ID, Type: domain.ArtifactTranscript, Content: "text", CreatedAt: now})
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	prompt, err := store.CreatePrompt(ctx, domain.Prompt{StageName: domain.StageRequirementExtraction, Version: "v1", Template: "{{input}}", CreatedAt: now})
	if err != nil {
		t.Fatalf("create prompt: %v", err)
	}

	audit, err := store.ReconstructRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if audit.Meeting.ID != meeting.ID || audit.Run.ID != run.ID {
		t.Fatalf("unexpected audit root: %+v", audit)
	}
	if len(audit.Stages) != 1 || audit.Stages[0].ID != stage.ID {
		t.Fatalf("unexpected stages: %+v", audit.Stages)
	}
	if len(audit.Artifacts) != 1 || audit.Artifacts[0].ID != artifact.ID {
		t.Fatalf("unexpected artifacts: %+v", audit.Artifacts)
	}
	if len(audit.Prompts) != 1 || audit.Prompts[0].ID != prompt.ID {
		t.Fatalf("unexpected prompts: %+v", audit.Prompts)
	}
}

func TestMemoryStoreLatestPromptByStage(t *testing.T) {
	ctx := context.Background()
	store := repository.NewMemoryStore()
	now := time.Now().UTC()
	_, _ = store.CreatePrompt(ctx, domain.Prompt{StageName: domain.StageRequirementExtraction, Version: "v1", CreatedAt: now})
	_, _ = store.CreatePrompt(ctx, domain.Prompt{StageName: domain.StageRequirementExtraction, Version: "v2", CreatedAt: now.Add(time.Second)})

	prompt, err := store.GetLatestPromptByStage(ctx, domain.StageRequirementExtraction)
	if err != nil {
		t.Fatalf("latest prompt: %v", err)
	}
	if prompt.Version != "v2" {
		t.Fatalf("version = %s, want v2", prompt.Version)
	}
}
