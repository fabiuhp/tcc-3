package export

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type MermaidRenderer interface {
	RenderMermaid(source string) (RenderedDiagram, error)
}

type RenderedDiagram struct {
	Path     string
	WidthPx  int
	HeightPx int
}

type MermaidCLIRenderer struct{}

func (MermaidCLIRenderer) RenderMermaid(source string) (RenderedDiagram, error) {
	if _, err := exec.LookPath("npx"); err != nil {
		return RenderedDiagram{}, fmt.Errorf("npx not found: %w", err)
	}

	dir, err := os.MkdirTemp("", "requirement-pipeline-mermaid-*")
	if err != nil {
		return RenderedDiagram{}, err
	}
	inputPath := filepath.Join(dir, "diagram.mmd")
	outputPath := filepath.Join(dir, "diagram.png")
	if err := os.WriteFile(inputPath, []byte(source), 0o644); err != nil {
		return RenderedDiagram{}, err
	}

	cmd := exec.Command("npx", "-y", "@mermaid-js/mermaid-cli", "-i", inputPath, "-o", outputPath, "-b", "white")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return RenderedDiagram{}, fmt.Errorf("render mermaid: %w: %s", err, strings.TrimSpace(string(output)))
	}

	file, err := os.Open(outputPath)
	if err != nil {
		return RenderedDiagram{}, err
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return RenderedDiagram{}, fmt.Errorf("read rendered diagram: %w", err)
	}
	return RenderedDiagram{Path: outputPath, WidthPx: config.Width, HeightPx: config.Height}, nil
}
