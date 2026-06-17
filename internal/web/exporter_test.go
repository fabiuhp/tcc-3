package web

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/export"
)

func TestFileExporterExportsEnhancedPDFWithBusinessDiagrams(t *testing.T) {
	outputDir := t.TempDir()
	exporter := FileExporter{
		Store:           staticAuditStore{audit: auditWithBusinessDiagrams(`{"diagrams":[{"title":"Fluxo de Cadastro","type":"business_flow","description":"Cadastro com liberação.","mermaid":"flowchart TD\nA[Cliente] --> B[Cadastro]\nB --> C[Acesso]"}]}`)},
		OutputDir:       outputDir,
		MermaidRenderer: &testMermaidRenderer{outputDir: outputDir, width: 640, height: 360},
	}

	path, err := exporter.Export(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("pdf is empty")
	}
}

func TestFileExporterFallsBackWhenBusinessDiagramsAreInvalid(t *testing.T) {
	exporter := FileExporter{
		Store:           staticAuditStore{audit: auditWithBusinessDiagrams("not json")},
		OutputDir:       t.TempDir(),
		MermaidRenderer: &testMermaidRenderer{},
	}

	path, err := exporter.Export(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("export fallback: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("pdf is empty")
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
			{ID: "artifact-1", Type: domain.ArtifactRefinedRequirements, Content: "O sistema deve gerar requisitos.", CreatedAt: now},
			{ID: "artifact-2", Type: domain.ArtifactBusinessDiagrams, Content: content, CreatedAt: now},
		},
	}
}

type testMermaidRenderer struct {
	outputDir string
	width     int
	height    int
}

func (r *testMermaidRenderer) RenderMermaid(source string) (export.RenderedDiagram, error) {
	if r.outputDir == "" {
		return export.RenderedDiagram{}, os.ErrInvalid
	}
	path := filepath.Join(r.outputDir, "diagram.png")
	img := image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			img.Set(x, y, color.White)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return export.RenderedDiagram{}, err
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return export.RenderedDiagram{}, err
	}
	return export.RenderedDiagram{Path: path, WidthPx: r.width, HeightPx: r.height}, nil
}
