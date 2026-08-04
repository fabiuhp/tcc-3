package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fabiuhp/tcc-3/internal/domain"
	artifactexport "github.com/fabiuhp/tcc-3/internal/export"
	"github.com/fabiuhp/tcc-3/internal/pipeline"
	"github.com/fabiuhp/tcc-3/internal/ports"
	"github.com/fabiuhp/tcc-3/internal/prompts"
	"github.com/fabiuhp/tcc-3/internal/requirements"
)

type Models struct {
	Transcription string
	Text          string
}

func DefaultStages(speech ports.SpeechToTextProvider, text ports.TextGenerationProvider, registry map[domain.StageName]domain.Prompt, models Models) []pipeline.Stage {
	return []pipeline.Stage{
		TranscriptionStage{provider: speech, model: models.Transcription},
		TextStage{name: domain.StageRequirementExtraction, outputType: domain.ArtifactRequirementDraft, provider: text, prompt: registry[domain.StageRequirementExtraction], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactTranscript}, normalize: normalizeRequirementDraft},
		TextStage{name: domain.StageRequirementReview, outputType: domain.ArtifactReviewedRequirements, provider: text, prompt: registry[domain.StageRequirementReview], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactRequirementDraft}, normalize: normalizeReviewedRequirements},
		TextStage{name: domain.StageGapAnalysis, outputType: domain.ArtifactGapAnalysis, provider: text, prompt: registry[domain.StageGapAnalysis], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactReviewedRequirements}, normalize: normalizeGapAnalysis},
		TextStage{name: domain.StageRequirementRefinement, outputType: domain.ArtifactRefinedRequirements, provider: text, prompt: registry[domain.StageRequirementRefinement], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactReviewedRequirements, domain.ArtifactGapAnalysis}, normalize: normalizeRefinedRequirements},
		TextStage{name: domain.StageDiagramGeneration, outputType: domain.ArtifactBusinessDiagrams, provider: text, prompt: registry[domain.StageDiagramGeneration], model: models.Text, inputs: []domain.ArtifactType{domain.ArtifactRefinedRequirements}, normalize: normalizeBusinessDiagrams},
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
	normalize  textResponseNormalizer
}

type textResponseNormalizer func(string, map[domain.ArtifactType]domain.Artifact) (string, error)

func (s TextStage) Name() domain.StageName { return s.name }

func (s TextStage) Execute(ctx context.Context, stageCtx pipeline.StageContext) (pipeline.StageResult, error) {
	values := map[string]string{"language": stageCtx.Meeting.Language}
	inputs := make(map[domain.ArtifactType]domain.Artifact, len(s.inputs))
	for _, artifactType := range s.inputs {
		artifact, err := pipeline.MustFindArtifact(stageCtx.Artifacts, artifactType)
		if err != nil {
			return pipeline.StageResult{Model: s.model}, err
		}
		values[string(artifactType)] = artifact.Content
		values["input"] = artifact.Content
		inputs[artifactType] = artifact
	}
	prompt := prompts.Render(s.prompt.Template, values)
	response, err := s.provider.Generate(ctx, ports.TextGenerationRequest{Prompt: prompt, Model: s.model})
	if err != nil {
		return pipeline.StageResult{Model: s.model}, err
	}
	content := response.Text
	if s.normalize != nil {
		content, err = s.normalize(response.Text, inputs)
		if err != nil {
			return pipeline.StageResult{Model: response.Model, Tokens: response.Tokens}, err
		}
	}
	return pipeline.StageResult{
		Artifacts: []domain.Artifact{{ID: domain.NewID(), Type: s.outputType, Content: content}},
		Model:     response.Model,
		Tokens:    response.Tokens,
	}, nil
}

func normalizeRequirementDraft(response string, inputs map[domain.ArtifactType]domain.Artifact) (string, error) {
	return requirements.NormalizeDraft(response, inputs[domain.ArtifactTranscript])
}

func normalizeReviewedRequirements(response string, inputs map[domain.ArtifactType]domain.Artifact) (string, error) {
	return requirements.NormalizeReview(response, inputs[domain.ArtifactRequirementDraft].Content)
}

func normalizeGapAnalysis(response string, inputs map[domain.ArtifactType]domain.Artifact) (string, error) {
	return requirements.NormalizeGapAnalysis(response, inputs[domain.ArtifactReviewedRequirements].Content)
}

func normalizeRefinedRequirements(response string, inputs map[domain.ArtifactType]domain.Artifact) (string, error) {
	return requirements.NormalizeRefined(response, inputs[domain.ArtifactReviewedRequirements].Content, inputs[domain.ArtifactGapAnalysis].Content)
}

func normalizeBusinessDiagrams(response string, _ map[domain.ArtifactType]domain.Artifact) (string, error) {
	return artifactexport.NormalizeBusinessDiagrams(response)
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
		return pipeline.StageResult{Model: s.model}, err
	}
	prompt := prompts.Render(s.prompt.Template, map[string]string{"input": refined.Content, "language": stageCtx.Meeting.Language})
	response, err := s.provider.Generate(ctx, ports.TextGenerationRequest{Prompt: prompt, Model: s.model})
	if err != nil {
		return pipeline.StageResult{Model: s.model}, err
	}
	documentation, err := parseFinalDocumentation(response.Text)
	if err != nil {
		return pipeline.StageResult{Model: response.Model, Tokens: response.Tokens}, err
	}
	return pipeline.StageResult{
		Artifacts: []domain.Artifact{
			{ID: domain.NewID(), Type: domain.ArtifactSRS, Content: documentation.SRS},
			{ID: domain.NewID(), Type: domain.ArtifactUserStories, Content: documentation.UserStories},
			{ID: domain.NewID(), Type: domain.ArtifactAcceptanceCriteria, Content: documentation.AcceptanceCriteria},
			{ID: domain.NewID(), Type: domain.ArtifactUseCases, Content: documentation.UseCases},
		},
		Model:  response.Model,
		Tokens: response.Tokens,
	}, nil
}

type finalDocumentation struct {
	SRS                string `json:"software_requirement_specification"`
	UserStories        string `json:"user_stories"`
	AcceptanceCriteria string `json:"acceptance_criteria"`
	UseCases           string `json:"use_cases"`
}

func parseFinalDocumentation(text string) (finalDocumentation, error) {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var documentation finalDocumentation
	decoder := json.NewDecoder(strings.NewReader(cleaned))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&documentation); err != nil {
		return finalDocumentation{}, fmt.Errorf("parse final documentation: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return finalDocumentation{}, fmt.Errorf("parse final documentation: unexpected data after JSON document")
	}

	fields := []struct {
		name  string
		value *string
	}{
		{name: "software_requirement_specification", value: &documentation.SRS},
		{name: "user_stories", value: &documentation.UserStories},
		{name: "acceptance_criteria", value: &documentation.AcceptanceCriteria},
		{name: "use_cases", value: &documentation.UseCases},
	}
	for _, field := range fields {
		*field.value = strings.TrimSpace(*field.value)
		if *field.value == "" {
			return finalDocumentation{}, fmt.Errorf("parse final documentation: field %q is required", field.name)
		}
	}
	return documentation, nil
}
