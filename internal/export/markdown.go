package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/requirements"
)

type MarkdownOptions struct {
	GeneratedAt time.Time
}

func WriteRequirementsMarkdown(outputDir string, audit domain.AuditRun) (string, error) {
	return WriteRequirementsMarkdownWithOptions(outputDir, audit, MarkdownOptions{})
}

func WriteRequirementsMarkdownWithOptions(outputDir string, audit domain.AuditRun, options MarkdownOptions) (string, error) {
	document, source, fallbackReasons := requirementsForMarkdown(audit)
	if options.GeneratedAt.IsZero() {
		options.GeneratedAt = time.Now().UTC()
	}

	content := buildRequirementsMarkdown(audit, document, source, fallbackReasons, options.GeneratedAt)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outputDir, fmt.Sprintf("requirements_%s_%s.md", audit.Run.ID, audit.Meeting.ID))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func buildRequirementsMarkdown(audit domain.AuditRun, document requirements.RefinedDocument, source string, fallbackReasons []string, generatedAt time.Time) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# Documento de Requisitos\n\n")
	fmt.Fprintf(&output, "- **Reunião:** %s\n", markdownInline(audit.Meeting.Title))
	fmt.Fprintf(&output, "- **Idioma:** %s\n", markdownInline(audit.Meeting.Language))
	if audit.Meeting.TranscriptFile != "" {
		fmt.Fprintf(&output, "- **Origem:** transcrição importada de %s (sem transcrição de áudio).\n", markdownInline(audit.Meeting.TranscriptFile))
	}
	fmt.Fprintf(&output, "- **Meeting ID:** `%s`\n", audit.Meeting.ID)
	fmt.Fprintf(&output, "- **Run ID:** `%s`\n", audit.Run.ID)
	fmt.Fprintf(&output, "- **Gerado em:** %s\n\n", generatedAt.Format(time.RFC3339))

	confirmed, assumptions, pending := countRequirementStatuses(document.Requirements)
	fmt.Fprintf(&output, "## Resumo\n\n")
	fmt.Fprintf(&output, "- Requisitos confirmados: **%d**\n", confirmed)
	fmt.Fprintf(&output, "- Suposições: **%d**\n", assumptions)
	fmt.Fprintf(&output, "- Requisitos pendentes: **%d**\n", pending)
	fmt.Fprintf(&output, "- Lacunas abertas: **%d**\n\n", len(document.OpenGaps))
	fmt.Fprintf(&output, "> Somente requisitos com status `confirmed` devem ser tratados como escopo confirmado. Itens `assumption`, `pending` e lacunas exigem validação dos stakeholders.\n\n")

	writeProcessingDiagnostics(&output, audit, source, fallbackReasons)
	writeBusinessDiagramsMarkdown(&output, audit.Artifacts)

	requirementsHeading := "Requisitos Refinados"
	if source != "refined_requirements" {
		requirementsHeading = "Requisitos Disponíveis (saída parcial)"
	}
	fmt.Fprintf(&output, "## %s\n\n", requirementsHeading)
	for _, requirement := range document.Requirements {
		fmt.Fprintf(&output, "### %s\n\n", requirement.ID)
		fmt.Fprintf(&output, "- **Tipo:** %s\n", requirementTypeLabel(requirement.Type))
		fmt.Fprintf(&output, "- **Status:** `%s`\n", requirement.Status)
		if len(requirement.RelatedGapIDs) > 0 {
			fmt.Fprintf(&output, "- **Lacunas relacionadas:** `%s`\n", strings.Join(requirement.RelatedGapIDs, "`, `"))
		}
		fmt.Fprintf(&output, "\n%s\n\n", requirement.Statement)
		fmt.Fprintf(&output, "**Evidências**\n\n")
		for _, evidence := range requirement.Evidence {
			fmt.Fprintf(&output, "> %s\n>\n> Artefato de origem: `%s`\n\n", strings.ReplaceAll(evidence.Quote, "\n", " "), evidence.ArtifactID)
		}
		writeRequirementProblems(&output, requirement.Problems)
	}
	if source == "transcript" {
		if transcript, ok := findArtifact(audit.Artifacts, domain.ArtifactTranscript); ok && strings.TrimSpace(transcript.Content) != "" {
			fmt.Fprintf(&output, "## Transcrição disponível\n\n%s\n\n", strings.TrimSpace(transcript.Content))
		}
	}

	if len(document.OpenGaps) > 0 {
		fmt.Fprintf(&output, "## Lacunas e Perguntas Pendentes\n\n")
		for _, gap := range document.OpenGaps {
			fmt.Fprintf(&output, "### %s\n\n", gap.ID)
			fmt.Fprintf(&output, "- **Tipo:** %s\n", gapTypeLabel(gap.Type))
			fmt.Fprintf(&output, "- **Status:** `%s`\n", gap.Status)
			fmt.Fprintf(&output, "- **Requisitos relacionados:** `%s`\n\n", strings.Join(gap.RelatedRequirementIDs, "`, `"))
			fmt.Fprintf(&output, "%s\n\n", gap.Description)
			fmt.Fprintf(&output, "**Pergunta para validação:** %s\n\n", gap.Question)
		}
	}

	writeGeneratedArtifact(&output, audit.Artifacts, domain.ArtifactSRS, "Especificação de Requisitos de Software")
	writeGeneratedArtifact(&output, audit.Artifacts, domain.ArtifactUserStories, "Histórias de Usuário")
	writeGeneratedArtifact(&output, audit.Artifacts, domain.ArtifactAcceptanceCriteria, "Critérios de Aceitação")
	writeGeneratedArtifact(&output, audit.Artifacts, domain.ArtifactUseCases, "Casos de Uso")
	return output.String()
}

func requirementsForMarkdown(audit domain.AuditRun) (requirements.RefinedDocument, string, []string) {
	fallbackReasons := []string{}
	if artifact, ok := findArtifact(audit.Artifacts, domain.ArtifactRefinedRequirements); ok {
		document, err := requirements.ParseRefined(artifact.Content)
		if err == nil {
			return document, "refined_requirements", fallbackReasons
		}
		fallbackReasons = append(fallbackReasons, fmt.Sprintf("O artefato de requisitos refinados não pôde ser interpretado: %v", err))
	}
	if artifact, ok := findArtifact(audit.Artifacts, domain.ArtifactReviewedRequirements); ok {
		document, err := requirements.ParseReviewed(artifact.Content)
		if err == nil {
			return refinedFromReviewed(document), "reviewed_requirements", fallbackReasons
		}
		fallbackReasons = append(fallbackReasons, fmt.Sprintf("O artefato de requisitos revisados não pôde ser interpretado: %v", err))
	}
	if artifact, ok := findArtifact(audit.Artifacts, domain.ArtifactRequirementDraft); ok {
		document, err := requirements.ParseDraft(artifact.Content)
		if err == nil {
			return refinedFromDraft(document), "requirement_draft", fallbackReasons
		}
		fallbackReasons = append(fallbackReasons, fmt.Sprintf("O artefato de requisitos preliminares não pôde ser interpretado: %v", err))
	}
	return requirements.RefinedDocument{
		Requirements: []requirements.RefinedRequirement{},
		OpenGaps:     []requirements.Gap{},
	}, "transcript", fallbackReasons
}

func refinedFromReviewed(document requirements.ReviewedDocument) requirements.RefinedDocument {
	values := make([]requirements.RefinedRequirement, 0, len(document.Requirements))
	for _, requirement := range document.Requirements {
		values = append(values, requirements.RefinedRequirement{Requirement: requirement.Requirement, RelatedGapIDs: []string{}})
	}
	return requirements.RefinedDocument{Requirements: values, OpenGaps: []requirements.Gap{}}
}

func refinedFromDraft(document requirements.DraftDocument) requirements.RefinedDocument {
	values := make([]requirements.RefinedRequirement, 0, len(document.Requirements))
	for _, requirement := range document.Requirements {
		values = append(values, requirements.RefinedRequirement{Requirement: requirement, RelatedGapIDs: []string{}})
	}
	return requirements.RefinedDocument{Requirements: values, OpenGaps: []requirements.Gap{}}
}

func writeProcessingDiagnostics(output *strings.Builder, audit domain.AuditRun, source string, fallbackReasons []string) {
	issues := audit.Run.ValidationIssues
	if len(issues) == 0 {
		for _, stage := range audit.Stages {
			issues = append(issues, stage.ValidationIssues...)
		}
	}
	hasFailedStage := false
	for _, stage := range audit.Stages {
		if stage.Status == domain.StatusFailed {
			hasFailedStage = true
			break
		}
	}
	if !hasFailedStage && audit.Run.Status != domain.StatusCompletedWithIssues && len(issues) == 0 && len(fallbackReasons) == 0 {
		return
	}

	fmt.Fprintf(output, "## Diagnóstico do processamento\n\n")
	if source != "refined_requirements" {
		fmt.Fprintf(output, "- **Fonte do documento:** saída parcial baseada em `%s`.\n", source)
	}
	if audit.Run.Status != "" {
		fmt.Fprintf(output, "- **Status da execução:** `%s`\n", audit.Run.Status)
	}
	if audit.Run.Error != "" {
		fmt.Fprintf(output, "- **Resumo:** %s\n", markdownInline(audit.Run.Error))
	}
	for _, stage := range audit.Stages {
		if stage.Status == domain.StatusFailed {
			fmt.Fprintf(output, "- **Etapa `%s`:** %s\n", stage.Name, markdownInline(stage.Error))
		}
	}
	for _, issue := range issues {
		locationParts := []string{}
		if issue.Stage != "" {
			locationParts = append(locationParts, string(issue.Stage))
		}
		if issue.Item != "" {
			locationParts = append(locationParts, issue.Item)
		}
		if issue.Field != "" {
			locationParts = append(locationParts, issue.Field)
		}
		location := strings.Join(locationParts, " / ")
		if location == "" {
			location = issue.Code
		}
		message := issue.Message
		if issue.Value != "" {
			message = fmt.Sprintf("%s Valor rejeitado: `%s`.", message, markdownInline(issue.Value))
		}
		if issue.Action != "" {
			message = fmt.Sprintf("%s Ação: `%s`.", message, issue.Action)
		}
		fmt.Fprintf(output, "- **Validação (%s):** %s\n", location, markdownInline(message))
	}
	for _, reason := range fallbackReasons {
		fmt.Fprintf(output, "- **Fallback de exportação:** %s\n", markdownInline(reason))
	}
	fmt.Fprintf(output, "\n")
}

func writeRequirementProblems(output *strings.Builder, problems []requirements.RequirementProblem) {
	if len(problems) == 0 {
		return
	}
	fmt.Fprintf(output, "**Problemas de validação**\n\n")
	for _, problem := range problems {
		fmt.Fprintf(output, "- `%s`", problem.Code)
		if problem.Field != "" {
			fmt.Fprintf(output, " em `%s`", problem.Field)
		}
		fmt.Fprintf(output, ": %s", markdownInline(problem.Message))
		if problem.Value != "" {
			fmt.Fprintf(output, " Valor rejeitado: `%s`.", markdownInline(problem.Value))
		}
		if problem.Action != "" {
			fmt.Fprintf(output, " Ação tomada: `%s`.", problem.Action)
		}
		fmt.Fprintf(output, "\n")
	}
	fmt.Fprintf(output, "\n")
}

func writeBusinessDiagramsMarkdown(output *strings.Builder, artifacts []domain.Artifact) {
	fmt.Fprintf(output, "## Diagramas de Negócio\n\n")
	artifact, ok := findArtifact(artifacts, domain.ArtifactBusinessDiagrams)
	if !ok {
		fmt.Fprintf(output, "> Nenhum diagrama de negócio foi gerado.\n\n")
		return
	}
	document, err := ParseBusinessDiagrams(artifact.Content)
	if err != nil {
		fmt.Fprintf(output, "> O artefato de diagramas não pôde ser interpretado: %s\n\n", err)
		return
	}
	for _, diagram := range document.Diagrams {
		fmt.Fprintf(output, "### %s\n\n", diagram.Title)
		if strings.TrimSpace(diagram.Description) != "" {
			fmt.Fprintf(output, "%s\n\n", diagram.Description)
		}
		fmt.Fprintf(output, "```mermaid\n%s\n```\n\n", strings.TrimSpace(diagram.Mermaid))
	}
}

func writeGeneratedArtifact(output *strings.Builder, artifacts []domain.Artifact, artifactType domain.ArtifactType, title string) {
	artifact, ok := findArtifact(artifacts, artifactType)
	if !ok || strings.TrimSpace(artifact.Content) == "" {
		return
	}
	fmt.Fprintf(output, "## %s\n\n%s\n\n", title, strings.TrimSpace(artifact.Content))
}

func countRequirementStatuses(values []requirements.RefinedRequirement) (confirmed, assumptions, pending int) {
	for _, requirement := range values {
		switch requirement.Status {
		case requirements.StatusConfirmed:
			confirmed++
		case requirements.StatusAssumption:
			assumptions++
		case requirements.StatusPending:
			pending++
		}
	}
	return confirmed, assumptions, pending
}

func requirementTypeLabel(value requirements.RequirementType) string {
	switch value {
	case requirements.TypeFunctional:
		return "Funcional"
	case requirements.TypeNonFunctional:
		return "Não funcional"
	case requirements.TypeBusinessRule:
		return "Regra de negócio"
	case requirements.TypeConstraint:
		return "Restrição"
	default:
		return string(value)
	}
}

func gapTypeLabel(value requirements.GapType) string {
	switch value {
	case requirements.GapMissingInformation:
		return "Informação ausente"
	case requirements.GapAmbiguity:
		return "Ambiguidade"
	case requirements.GapConflict:
		return "Conflito"
	case requirements.GapIncompleteRule:
		return "Regra incompleta"
	default:
		return string(value)
	}
}

func markdownInline(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func findArtifact(artifacts []domain.Artifact, artifactType domain.ArtifactType) (domain.Artifact, bool) {
	for i := len(artifacts) - 1; i >= 0; i-- {
		if artifacts[i].Type == artifactType {
			return artifacts[i], true
		}
	}
	return domain.Artifact{}, false
}
