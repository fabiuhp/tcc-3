package web

import (
	"context"

	"requirement-pipeline/internal/domain"
	"requirement-pipeline/internal/export"
)

type AuditStore interface {
	ReconstructRun(context.Context, string) (domain.AuditRun, error)
}

type FileExporter struct {
	Store           AuditStore
	OutputDir       string
	MermaidRenderer export.MermaidRenderer
}

func (e FileExporter) Export(ctx context.Context, runID string) (string, error) {
	audit, err := e.Store.ReconstructRun(ctx, runID)
	if err != nil {
		return "", err
	}
	return export.WriteRefinedRequirementsPDFWithOptions(e.OutputDir, audit, export.PDFOptions{MermaidRenderer: e.MermaidRenderer})
}
