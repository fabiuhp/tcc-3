package stages

import (
	"context"
	"strings"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/pipeline"
	"requirement-pipeline/internal/ports"
	"requirement-pipeline/internal/prompts"
)

type Models struct {
	Transcription string
	Text          string
}

func DefaultStages(speech ports.SpeechToTextProvider, text ports.TextGenerationProvider, registry map[domain.StageName]domain.Prompt, models Models) []pipeline.Stage {
	return []pipeline.Stage{
		TranscriptionStage{provider: speech, model: models.Transcription},
		TextStage{name: domain.StageRequirementExtraction, outputType: domain.ArtifactRequirementDraft, provider: text, prompt: registry[domain.StageRequirementExtraction], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactTranscript}},
		TextStage{name: domain.StageRequirementReview, outputType: domain.ArtifactReviewedRequirements, provider: text, prompt: registry[domain.StageRequirementReview], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactRequirementDraft}},
		TextStage{name: domain.StageGapAnalysis, outputType: domain.ArtifactGapAnalysis, provider: text, prompt: registry[domain.StageGapAnalysis], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactReviewedRequirements}},
		TextStage{name: domain.StageRequirementRefinement, outputType: domain.ArtifactRefinedRequirements, provider: text, prompt: registry[domain.StageRequirementRefinement], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactReviewedRequirements, domain.ArtifactGapAnalysis}},
		TextStage{name: domain.StageDiagramGeneration, outputType: domain.ArtifactBusinessDiagrams, provider: text, prompt: registry[domain.StageDiagramGeneration], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactRefinedRequirements}},
		ArtifactGenerationStage{provider: text, prompt: registry[domain.StageArtifactGeneration], model: models.Text},
	}
}

type TranscriptionStage struct {
	provider ports.SpeechToTextProvider
	model    string
}

func (s TranscriptionStage) Name() domain.StageName { return domain.StageAudioTranscription }

func (s TranscriptionStage) Execute(ctx context.Context, stageCtx pipeline.StageContext) (pipeline.StageResult, error) {
	response, err := s.provider.Transcribe(ctx, ports.TranscriptionRequest{AudioFile: stageCtx.Meeting.AudioFile, Language: stageCtx.Meeting.Language, Model: s.model})
	if err != nil {
		return pipeline.StageResult{Model: s.model}, err
	}
	return pipeline.StageResult{
		Artifacts: []domain.Artifact{{ID: domain.NewID(), Type: domain.ArtifactTranscript, Content: response.Text}},
		Model:     response.Model,
		Tokens:    response.Tokens,
	}, nil
}

type TextStage struct {
	name       domain.StageName
	outputType domain.ArtifactType
	provider   ports.TextGenerationProvider
	prompt     domain.Prompt
	model      string
	inputs     []domain.ArtifactType
}

func (s TextStage) Name() domain.StageName { return s.name }

func (s TextStage) Execute(ctx context.Context, stageCtx pipeline.StageContext) (pipeline.StageResult, error) {
	values := map[string]string{"language": stageCtx.Meeting.Language}
	for _, artifactType := range s.inputs {
		artifact, err := pipeline.MustFindArtifact(stageCtx.Artifacts, artifactType)
		if err != nil {
			return pipeline.StageResult{Model: s.model, PromptVersion: s.prompt.Version}, err
		}
		values[string(artifactType)] = artifact.Content
		values["input"] = artifact.Content
	}
	prompt := prompts.Render(s.prompt.Template, values)
	response, err := s.provider.Generate(ctx, ports.TextGenerationRequest{Prompt: prompt, Model: s.model})
	if err != nil {
		return pipeline.StageResult{Model: s.model, PromptVersion: s.prompt.Version}, err
	}
	return pipeline.StageResult{
		Artifacts:     []domain.Artifact{{ID: domain.NewID(), Type: s.outputType, Content: response.Text}},
		Model:         response.Model,
		PromptVersion: s.prompt.Version,
		Tokens:        response.Tokens,
	}, nil
}

type ArtifactGenerationStage struct {
	provider ports.TextGenerationProvider
	prompt   domain.Prompt
	model    string
}

func (s ArtifactGenerationStage) Name() domain.StageName { return domain.StageArtifactGeneration }

func (s ArtifactGenerationStage) Execute(ctx context.Context, stageCtx pipeline.StageContext) (pipeline.StageResult, error) {
	refined, err := pipeline.MustFindArtifact(stageCtx.Artifacts, domain.ArtifactRefinedRequirements)
	if err != nil {
		return pipeline.StageResult{Model: s.model, PromptVersion: s.prompt.Version}, err
	}
	prompt := prompts.Render(s.prompt.Template, map[string]string{"input": refined.Content, "language": stageCtx.Meeting.Language})
	response, err := s.provider.Generate(ctx, ports.TextGenerationRequest{Prompt: prompt, Model: s.model})
	if err != nil {
		return pipeline.StageResult{Model: s.model, PromptVersion: s.prompt.Version}, err
	}
	sections := splitFinalDocumentation(response.Text)
	return pipeline.StageResult{
		Artifacts: []domain.Artifact{
			{ID: domain.NewID(), Type: domain.ArtifactSRS, Content: sections[0]},
			{ID: domain.NewID(), Type: domain.ArtifactUserStories, Content: sections[1]},
			{ID: domain.NewID(), Type: domain.ArtifactAcceptanceCriteria, Content: sections[2]},
			{ID: domain.NewID(), Type: domain.ArtifactUseCases, Content: sections[3]},
		},
		Model:         response.Model,
		PromptVersion: s.prompt.Version,
		Tokens:        response.Tokens,
	}, nil
}

func splitFinalDocumentation(text string) []string {
	labels := []string{"Especificação de Requisitos de Software", "Histórias de Usuário", "Critérios de Aceitação", "Casos de Uso"}
	sections := make([]string, len(labels))
	for i, label := range labels {
		sections[i] = "# " + label + "\n\n" + strings.TrimSpace(text)
	}
	return sections
}
