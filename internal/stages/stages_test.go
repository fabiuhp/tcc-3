package stages_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/pipeline"
	"github.com/fabiuhp/tcc-3/internal/ports"
	"github.com/fabiuhp/tcc-3/internal/prompts"
	"github.com/fabiuhp/tcc-3/internal/providers"
	"github.com/fabiuhp/tcc-3/internal/stages"
)

func TestDefaultStagesProduceExpectedArtifactTypes(t *testing.T) {
	provider := providers.MockAIProvider{TranscriptionText: "transcript", GenerationFunc: structuredGeneration}
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

func TestArtifactGenerationUsesSimulatedRequirementsWithoutTranscription(t *testing.T) {
	simulatedRequirements := refinedDocumentJSON
	response := `{"software_requirement_specification":"# SRS\n\nREQ-0001","user_stories":"REQ-0001: Como cliente, eu quero acompanhar meu pedido para saber quando ele chegará.","acceptance_criteria":"REQ-0001: Dado um pedido enviado, quando o status mudar, então o cliente deve ser notificado.","use_cases":"REQ-0001: Ator: Cliente. Fluxo: consultar pedido e receber atualização."}`
	provider := &recordingTextProvider{response: response}
	defaultPrompts := prompts.Defaults(time.Now().UTC())
	pipelineStages := stages.DefaultStages(nil, provider, prompts.Registry(defaultPrompts), stages.Models{Text: "gpt-5"})
	artifactStage := pipelineStages[len(pipelineStages)-1]

	result, err := artifactStage.Execute(context.Background(), pipeline.StageContext{
		Meeting: domain.Meeting{Language: "pt-BR"},
		Artifacts: []domain.Artifact{{
			ID:      domain.NewID(),
			Type:    domain.ArtifactRefinedRequirements,
			Content: simulatedRequirements,
		}},
	})
	if err != nil {
		t.Fatalf("generate artifacts: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("generation calls = %d, want 1", provider.calls)
	}
	if !strings.Contains(provider.request.Prompt, simulatedRequirements) {
		t.Fatal("generation prompt does not contain the simulated requirements")
	}
	wantContents := map[domain.ArtifactType]string{
		domain.ArtifactSRS:                "# SRS\n\nREQ-0001",
		domain.ArtifactUserStories:        "REQ-0001: Como cliente, eu quero acompanhar meu pedido para saber quando ele chegará.",
		domain.ArtifactAcceptanceCriteria: "REQ-0001: Dado um pedido enviado, quando o status mudar, então o cliente deve ser notificado.",
		domain.ArtifactUseCases:           "REQ-0001: Ator: Cliente. Fluxo: consultar pedido e receber atualização.",
	}
	if len(result.Artifacts) != len(wantContents) {
		t.Fatalf("artifact count = %d, want %d", len(result.Artifacts), len(wantContents))
	}
	for _, artifact := range result.Artifacts {
		want, ok := wantContents[artifact.Type]
		if !ok {
			t.Fatalf("unexpected artifact type %q", artifact.Type)
		}
		if artifact.Content != want {
			t.Fatalf("artifact %s content = %q, want %q", artifact.Type, artifact.Content, want)
		}
		delete(wantContents, artifact.Type)
	}
	if len(wantContents) != 0 {
		t.Fatalf("missing artifact contents: %+v", wantContents)
	}
}

func TestArtifactGenerationRejectsIncompleteResponse(t *testing.T) {
	provider := &recordingTextProvider{response: `{"software_requirement_specification":"SRS","user_stories":"Histórias","acceptance_criteria":"Critérios"}`}
	defaultPrompts := prompts.Defaults(time.Now().UTC())
	pipelineStages := stages.DefaultStages(nil, provider, prompts.Registry(defaultPrompts), stages.Models{Text: "gpt-5"})
	artifactStage := pipelineStages[len(pipelineStages)-1]

	_, err := artifactStage.Execute(context.Background(), pipeline.StageContext{
		Meeting:   domain.Meeting{Language: "pt-BR"},
		Artifacts: []domain.Artifact{{Type: domain.ArtifactRefinedRequirements, Content: refinedDocumentJSON}},
	})
	if err == nil || !strings.Contains(err.Error(), "use_cases") {
		t.Fatalf("error = %v, want missing use_cases error", err)
	}
}

func TestArtifactGenerationRejectsUnknownField(t *testing.T) {
	provider := &recordingTextProvider{response: `{"software_requirement_specification":"SRS","user_stories":"Histórias","acceptance_criteria":"Critérios","use_cases":"Casos","unexpected":"value"}`}
	defaultPrompts := prompts.Defaults(time.Now().UTC())
	pipelineStages := stages.DefaultStages(nil, provider, prompts.Registry(defaultPrompts), stages.Models{Text: "gpt-5"})
	artifactStage := pipelineStages[len(pipelineStages)-1]

	_, err := artifactStage.Execute(context.Background(), pipeline.StageContext{
		Meeting:   domain.Meeting{Language: "pt-BR"},
		Artifacts: []domain.Artifact{{Type: domain.ArtifactRefinedRequirements, Content: refinedDocumentJSON}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown field error", err)
	}
}

type recordingTextProvider struct {
	response string
	request  ports.TextGenerationRequest
	calls    int
}

func (p *recordingTextProvider) Generate(_ context.Context, request ports.TextGenerationRequest) (ports.TextGenerationResponse, error) {
	p.calls++
	p.request = request
	return ports.TextGenerationResponse{Text: p.response, Model: request.Model}, nil
}

func structuredGeneration(request ports.TextGenerationRequest) (ports.TextGenerationResponse, error) {
	var text string
	switch {
	case strings.Contains(request.Prompt, `"software_requirement_specification"`):
		text = finalDocumentationJSON
	case strings.Contains(request.Prompt, `"diagrams"`):
		text = diagramsJSON
	case strings.Contains(request.Prompt, "Refine a redação"):
		text = refinedResponseJSON
	case strings.Contains(request.Prompt, `"question":"pergunta objetiva para o stakeholder"`):
		text = gapResponseJSON
	case strings.Contains(request.Prompt, `"review":{"outcome"`):
		text = reviewResponseJSON
	default:
		text = draftResponseJSON
	}
	return ports.TextGenerationResponse{Text: text, Model: request.Model}, nil
}

const (
	draftResponseJSON      = `{"requirements":[{"type":"functional","statement":"O sistema deve processar a transcrição.","status":"confirmed","evidence":[{"quote":"transcript"}]}]}`
	reviewResponseJSON     = `{"requirements":[{"id":"REQ-0001","type":"functional","statement":"O sistema deve processar a transcrição.","status":"confirmed","review":{"outcome":"approved","findings":[]}}]}`
	gapResponseJSON        = `{"gaps":[]}`
	refinedResponseJSON    = `{"requirements":[{"id":"REQ-0001","statement":"O sistema deve processar a transcrição.","status":"confirmed"}]}`
	refinedDocumentJSON    = `{"requirements":[{"id":"REQ-0001","type":"functional","statement":"O sistema deve processar a transcrição.","status":"confirmed","evidence":[{"artifact_id":"transcript-1","quote":"transcript"}],"related_gap_ids":[]}],"open_gaps":[]}`
	diagramsJSON           = `{"diagrams":[{"title":"Fluxo","type":"business_flow","description":"Fluxo principal.","mermaid":"flowchart TD\nA[Início] --> B[Fim]"}]}`
	finalDocumentationJSON = `{"software_requirement_specification":"SRS simulada para REQ-0001","user_stories":"Histórias simuladas para REQ-0001","acceptance_criteria":"Critérios simulados para REQ-0001","use_cases":"Casos simulados para REQ-0001"}`
)
