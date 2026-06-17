package export_test

import (
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

func TestWriteRefinedRequirementsPDF(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião de requisitos", Language: "pt-BR"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{ID: "artifact-1", Type: domain.ArtifactRefinedRequirements, Content: "Requisito refinado\n- O sistema deve gerar PDF.", CreatedAt: time.Now().UTC()},
		},
	}

	path, err := export.WriteRefinedRequirementsPDF(t.TempDir(), audit)
	if err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("pdf is empty")
	}
}

func TestWriteRefinedRequirementsPDFWithBusinessDiagrams(t *testing.T) {
	outputDir := t.TempDir()
	renderer := &fakeMermaidRenderer{outputDir: outputDir, width: 800, height: 500}
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião de requisitos", Language: "pt-BR"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{ID: "artifact-1", Type: domain.ArtifactRefinedRequirements, Content: "O sistema deve liberar acesso após pagamento.", CreatedAt: time.Now().UTC()},
			{ID: "artifact-2", Type: domain.ArtifactBusinessDiagrams, Content: `{"diagrams":[{"title":"Fluxo de Matrícula","type":"business_flow","description":"Mostra a liberação de acesso.","mermaid":"flowchart TD\nA[Aluno] --> B[Pagamento]\nB --> C[Acesso]"}]}`, CreatedAt: time.Now().UTC()},
		},
	}

	path, err := export.WriteRefinedRequirementsPDFWithOptions(outputDir, audit, export.PDFOptions{MermaidRenderer: renderer})
	if err != nil {
		t.Fatalf("write pdf with diagrams: %v", err)
	}
	if renderer.calls != 1 {
		t.Fatalf("renderer calls = %d, want 1", renderer.calls)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("pdf is empty")
	}
}

func TestWriteRefinedRequirementsPDFFallsBackForInvalidBusinessDiagrams(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião de requisitos", Language: "pt-BR"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{ID: "artifact-1", Type: domain.ArtifactRefinedRequirements, Content: "O sistema deve gerar PDF.", CreatedAt: time.Now().UTC()},
			{ID: "artifact-2", Type: domain.ArtifactBusinessDiagrams, Content: "not json", CreatedAt: time.Now().UTC()},
		},
	}

	path, err := export.WriteRefinedRequirementsPDFWithOptions(t.TempDir(), audit, export.PDFOptions{MermaidRenderer: &failingMermaidRenderer{}})
	if err != nil {
		t.Fatalf("write pdf fallback: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("pdf is empty")
	}
}

type fakeMermaidRenderer struct {
	outputDir string
	width     int
	height    int
	calls     int
}

func (f *fakeMermaidRenderer) RenderMermaid(source string) (export.RenderedDiagram, error) {
	f.calls++
	path := filepath.Join(f.outputDir, "diagram.png")
	img := image.NewRGBA(image.Rect(0, 0, f.width, f.height))
	for y := 0; y < f.height; y++ {
		for x := 0; x < f.width; x++ {
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
	return export.RenderedDiagram{Path: path, WidthPx: f.width, HeightPx: f.height}, nil
}

type failingMermaidRenderer struct{}

func (f *failingMermaidRenderer) RenderMermaid(source string) (export.RenderedDiagram, error) {
	return export.RenderedDiagram{}, os.ErrNotExist
}

func TestWriteRefinedRequirementsPDFWithOptions(t *testing.T) {
	audit := domain.AuditRun{
		Meeting: domain.Meeting{ID: "meeting-1", Title: "Reunião de requisitos", Language: "pt-BR"},
		Run:     domain.PipelineRun{ID: "run-1"},
		Artifacts: []domain.Artifact{
			{ID: "artifact-1", Type: domain.ArtifactRefinedRequirements, Content: "O sistema deve incluir uma visão geral visual.", CreatedAt: time.Now().UTC()},
		},
	}

	path, err := export.WriteRefinedRequirementsPDFWithOptions(t.TempDir(), audit, export.PDFOptions{GeneratedAt: time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("write pdf with options: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("pdf is empty")
	}
}
