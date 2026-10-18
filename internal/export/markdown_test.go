package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/export"
)

const refinedRequirementsJSON = `{"requirements":[{"id":"REQ-0001","type":"functional","statement":"O sistema deve permitir acompanhar o pedido.","status":"confirmed","evidence":[{"artifact_id":"transcript-1","quote":"Eu preciso acompanhar o pedido."}],"related_gap_ids":[]},{"id":"REQ-0002","type":"business_rule","statement":"O prazo da notificação deve ser definido.","status":"pending","evidence":[{"artifact_id":"transcript-1","quote":"O prazo ainda precisa ser decidido."}],"related_gap_ids":["GAP-0001"]}],"open_gaps":[{"id":"GAP-0001","type":"missing_information","status":"pending","description":"O prazo não foi definido.","question":"Qual deve ser o prazo?","related_requirement_ids":["REQ-0002"]}]}`

func TestWriteRequirementsMarkdownIncludesTraceabilityAndMermaid(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião de requisitos", Language: "pt-BR"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{ID: "artifact-1", Type: domain.ArtifactRefinedRequirements, Content: refinedRequirementsJSON},
			{ID: "artifact-2", Type: domain.ArtifactBusinessDiagrams, Content: `{"diagrams":[{"title":"Fluxo do Pedido","type":"business_flow","description":"Acompanhamento do pedido.","mermaid":"flowchart TD\nA[Pedido] --> B[Acompanhamento]"}]}`},
			{ID: "artifact-3", Type: domain.ArtifactSRS, Content: "REQ-0001 - Especificação consolidada."},
		},
	}

	path, err := export.WriteRequirementsMarkdownWithOptions(t.TempDir(), audit, export.MarkdownOptions{GeneratedAt: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	if filepath.Ext(path) != ".md" {
		t.Fatalf("extension = %q, want .md", filepath.Ext(path))
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	for _, want := range []string{
		"# Documento de Requisitos",
		"### REQ-0001",
		"Artefato de origem: `transcript-1`",
		"### GAP-0001",
		"```mermaid",
		"flowchart TD",
		"## Especificação de Requisitos de Software",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("markdown missing %q", want)
		}
	}
}

func TestWriteRequirementsMarkdownIncludesRequirementProblems(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião", Language: "pt-BR"},
		Run:     domain.PipelineRun{ID: "run-1", Status: domain.StatusCompletedWithIssues},
		Artifacts: []domain.Artifact{{
			Type:    domain.ArtifactRefinedRequirements,
			Content: `{"requirements":[{"id":"REQ-0001","type":"functional","statement":"O sistema deve emitir nota fiscal.","status":"pending","evidence":[],"problems":[{"code":"evidence_quote_unverifiable","field":"evidence[1].quote","message":"The source quote does not occur in the transcript and was removed.","value":"O sistema precisa emitir nota fiscal.","action":"removed"}],"related_gap_ids":[]}],"open_gaps":[]}`,
		}},
	}

	path, err := export.WriteRequirementsMarkdown(t.TempDir(), audit)
	if err != nil {
		t.Fatalf("write markdown with requirement problem: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	for _, want := range []string{"Problemas de validação", "evidence_quote_unverifiable", "evidence[1].quote", "O sistema precisa emitir nota fiscal."} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("markdown missing requirement problem %q", want)
		}
	}
}

func TestWriteRequirementsMarkdownFallsBackForInvalidStructuredRequirements(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{Type: domain.ArtifactRefinedRequirements, Content: `{"requirements":[]}`},
		},
	}

	path, err := export.WriteRequirementsMarkdown(t.TempDir(), audit)
	if err != nil {
		t.Fatalf("write fallback markdown: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	for _, want := range []string{"## Diagnóstico do processamento", "open_gaps", "saída parcial"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("markdown missing fallback detail %q", want)
		}
	}
}

func TestWriteRequirementsMarkdownExportsTranscriptWhenPipelineFails(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião", Language: "pt-BR"},
		Run: domain.PipelineRun{
			ID:     "run-1",
			Status: domain.StatusFailed,
			Error:  "stage requirement_extraction: provider failed",
		},
		Stages: []domain.StageExecution{{
			Name:   domain.StageRequirementExtraction,
			Status: domain.StatusFailed,
			Error:  "provider failed",
		}},
		Artifacts: []domain.Artifact{{
			Type:    domain.ArtifactTranscript,
			Content: "Cliente: preciso acompanhar o pedido.",
		}},
	}

	path, err := export.WriteRequirementsMarkdown(t.TempDir(), audit)
	if err != nil {
		t.Fatalf("write transcript fallback markdown: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	for _, want := range []string{"stage requirement_extraction: provider failed", "## Transcrição disponível", "preciso acompanhar o pedido"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("markdown missing failure fallback detail %q", want)
		}
	}
}

func TestWriteRequirementsMarkdownFallsBackForInvalidDiagrams(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{Type: domain.ArtifactRefinedRequirements, Content: refinedRequirementsJSON},
			{Type: domain.ArtifactBusinessDiagrams, Content: "not json"},
		},
	}

	path, err := export.WriteRequirementsMarkdown(t.TempDir(), audit)
	if err != nil {
		t.Fatalf("write markdown with diagram fallback: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(content), "não pôde ser interpretado") {
		t.Fatal("markdown did not include invalid diagram fallback")
	}
}
