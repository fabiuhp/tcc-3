package export_test

import (
	"strings"
	"testing"

	"requirement-pipeline/internal/export"
)

func TestParseBusinessDiagrams(t *testing.T) {
	content := `{"diagrams":[{"title":"Fluxo de Matrícula","type":"business_flow","description":"Do interesse ao acesso.","mermaid":"flowchart TD\nA[Aluno interessado] --> B[Escolhe plano]\nB --> C[Pagamento]\nC --> D[Acesso liberado]"}]}`

	document, err := export.ParseBusinessDiagrams(content)
	if err != nil {
		t.Fatalf("parse diagrams: %v", err)
	}
	if len(document.Diagrams) != 1 {
		t.Fatalf("diagram count = %d, want 1", len(document.Diagrams))
	}
	if document.Diagrams[0].Title != "Fluxo de Matrícula" {
		t.Fatalf("title = %q", document.Diagrams[0].Title)
	}
}

func TestParseBusinessDiagramsRejectsMalformedContent(t *testing.T) {
	if _, err := export.ParseBusinessDiagrams("not json"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidateBusinessDiagramRejectsMissingMermaid(t *testing.T) {
	err := export.ValidateBusinessDiagram(export.BusinessDiagram{Title: "Fluxo"})
	if err == nil || !strings.Contains(err.Error(), "mermaid") {
		t.Fatalf("error = %v, want mermaid validation", err)
	}
}

func TestValidateBusinessDiagramRejectsTooManyNodes(t *testing.T) {
	mermaid := "flowchart TD\nA[A] --> B[B]\nB --> C[C]\nC --> D[D]\nD --> E[E]\nE --> F[F]\nF --> G[G]\nG --> H[H]\nH --> I[I]\nI --> J[J]\nJ --> K[K]\nK --> L[L]\nL --> M[M]\nM --> N[N]\nN --> O[O]\nO --> P[P]\nP --> Q[Q]"
	err := export.ValidateBusinessDiagram(export.BusinessDiagram{Title: "Fluxo", Mermaid: mermaid})
	if err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("error = %v, want maximum validation", err)
	}
}
