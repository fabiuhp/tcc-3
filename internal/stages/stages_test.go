package stages_test

import (
	"context"
	"testing"
	"time"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/pipeline"
	"requirement-pipeline/internal/prompts"
	"requirement-pipeline/internal/providers"
	"requirement-pipeline/internal/stages"
)

func TestDefaultStagesProduceExpectedArtifactTypes(t *testing.T) {
	provider := providers.MockAIProvider{TranscriptionText: "transcript", TextPrefix: "generated"}
	defaultPrompts := prompts.Defaults(time.Now().UTC())
	pipelineStages := stages.DefaultStages(provider, provider, prompts.Registry(defaultPrompts), stages.Models{Transcription: "gpt-4o-transcribe", Text: "gpt-5"})
	ctx := pipeline.StageContext{Meeting: domain.Meeting{AudioFile: "meeting.mp3", Language: "pt-BR"}}
	allArtifacts := []domain.Artifact{}
	wantStageOrder := []domain.StageName{
		domain.StageAudioTranscription,
		domain.StageRequirementExtraction,
		domain.StageRequirementReview,
		domain.StageGapAnalysis,
		domain.StageRequirementRefinement,
		domain.StageDiagramGeneration,
		domain.StageArtifactGeneration,
	}
	if len(pipelineStages) != len(wantStageOrder) {
		t.Fatalf("stage count = %d, want %d", len(pipelineStages), len(wantStageOrder))
	}

	for i, stage := range pipelineStages {
		if stage.Name() != wantStageOrder[i] {
			t.Fatalf("stage %d = %s, want %s", i, stage.Name(), wantStageOrder[i])
		}
		ctx.Artifacts = allArtifacts
		result, err := stage.Execute(context.Background(), ctx)
		if err != nil {
			t.Fatalf("stage %s: %v", stage.Name(), err)
		}
		if result.Model == "" {
			t.Fatalf("stage %s did not record model", stage.Name())
		}
		if stage.Name() != domain.StageAudioTranscription && result.PromptVersion != prompts.VersionV1 {
			t.Fatalf("stage %s prompt version = %q", stage.Name(), result.PromptVersion)
		}
		allArtifacts = append(allArtifacts, result.Artifacts...)
	}

	wantTypes := map[domain.ArtifactType]bool{
		domain.ArtifactTranscript:           true,
		domain.ArtifactRequirementDraft:     true,
		domain.ArtifactReviewedRequirements: true,
		domain.ArtifactGapAnalysis:          true,
		domain.ArtifactRefinedRequirements:  true,
		domain.ArtifactBusinessDiagrams:     true,
		domain.ArtifactSRS:                  true,
		domain.ArtifactUserStories:          true,
		domain.ArtifactAcceptanceCriteria:   true,
		domain.ArtifactUseCases:             true,
	}
	for _, artifact := range allArtifacts {
		delete(wantTypes, artifact.Type)
	}
	if len(wantTypes) != 0 {
		t.Fatalf("missing artifact types: %+v", wantTypes)
	}
}
