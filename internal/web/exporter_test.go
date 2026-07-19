package web

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"requirement-pipeline/internal/domain"
)

const webRefinedRequirementsJSON = `{"requirements":[{"id":"REQ-0001","type":"functional","statement":"O sistema deve gerar requisitos.","status":"confirmed","evidence":[{"artifact_id":"transcript-1","quote":"O sistema deve gerar requisitos."}],"related_gap_ids":[]}],"open_gaps":[]}`

func TestFileExporterExportsMarkdownWithBusinessDiagrams(t *testing.T) {
	outputDir := t.TempDir()
	exporter := FileExporter{
		Store:     staticAuditStore{audit: auditWithBusinessDiagrams(`{"diagrams":[{"title":"Fluxo de Cadastro","type":"business_flow","description":"Cadastro com liberação.","mermaid":"flowchart TD\nA[Cliente] --> B[Cadastro]\nB --> C[Acesso]"}]}`)},
		OutputDir: outputDir,
	}

	path, err := exporter.Export(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if filepath.Ext(path) != ".md" {
		t.Fatalf("extension = %q, want .md", filepath.Ext(path))
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(content), "```mermaid") {
		t.Fatal("markdown does not contain Mermaid diagram")
	}
}

func TestFileExporterFallsBackWhenBusinessDiagramsAreInvalid(t *testing.T) {
	exporter := FileExporter{
		Store:     staticAuditStore{audit: auditWithBusinessDiagrams("not json")},
		OutputDir: t.TempDir(),
	}

	path, err := exporter.Export(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("export fallback: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if !strings.Contains(string(content), "não pôde ser interpretado") {
		t.Fatal("markdown does not contain diagram fallback")
	}
}

type staticAuditStore struct {
	audit domain.AuditRun
}

func (s staticAuditStore) ReconstructRun(_ context.Context, _ string) (domain.AuditRun, error) {
	return s.audit, nil
}

func auditWithBusinessDiagrams(content string) domain.AuditRun {
	now := time.Now().UTC()
	return domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião", Language: "pt-BR"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{ID: "artifact-1", Type: domain.ArtifactRefinedRequirements, Content: webRefinedRequirementsJSON, CreatedAt: now},
			{ID: "artifact-2", Type: domain.ArtifactBusinessDiagrams, Content: content, CreatedAt: now},
		},
	}
}
