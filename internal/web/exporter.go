package web

import (
	"context"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/export"
)

type AuditStore interface {
	ReconstructRun(context.Context, string) (domain.AuditRun, error)
}

type FileExporter struct {
	Store     AuditStore
	OutputDir string
}

func (e FileExporter) Export(ctx context.Context, runID string) (string, error) {
	audit, err := e.Store.ReconstructRun(ctx, runID)
	if err != nil {
		return "", err
	}
	return export.WriteRequirementsMarkdown(e.OutputDir, audit)
}
