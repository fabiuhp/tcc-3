package export

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const maxBusinessDiagramNodes = 16

var mermaidNodeIDPattern = regexp.MustCompile(`^[\s\-\.>]*([A-Za-z][A-Za-z0-9_]*)`)

type BusinessDiagramDocument struct {
	Diagrams []BusinessDiagram `json:"diagrams"`
}

type BusinessDiagram struct {
	Title       string `json:"title"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Mermaid     string `json:"mermaid"`
}

func ParseBusinessDiagrams(content string) (BusinessDiagramDocument, error) {
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var document BusinessDiagramDocument
	if err := json.Unmarshal([]byte(cleaned), &document); err != nil {
		return BusinessDiagramDocument{}, fmt.Errorf("parse business diagrams: %w", err)
	}
	if len(document.Diagrams) == 0 {
		return BusinessDiagramDocument{}, fmt.Errorf("business diagrams artifact has no diagrams")
	}
	for i, diagram := range document.Diagrams {
		if err := ValidateBusinessDiagram(diagram); err != nil {
			return BusinessDiagramDocument{}, fmt.Errorf("diagram %d: %w", i+1, err)
		}
	}
	return document, nil
}

func ValidateBusinessDiagram(diagram BusinessDiagram) error {
	if strings.TrimSpace(diagram.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(diagram.Mermaid) == "" {
		return fmt.Errorf("mermaid source is required")
	}
	if nodes := CountMermaidNodes(diagram.Mermaid); nodes > maxBusinessDiagramNodes {
		return fmt.Errorf("diagram has %d nodes, maximum is %d", nodes, maxBusinessDiagramNodes)
	}
	return nil
}

func CountMermaidNodes(source string) int {
	nodes := map[string]bool{}
	for line := range strings.SplitSeq(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "flowchart ") || strings.HasPrefix(line, "graph ") || strings.HasPrefix(line, "%%") {
			continue
		}
		parts := strings.FieldsFunc(line, func(r rune) bool {
			return r == '-' || r == '>' || r == '|'
		})
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" || strings.HasPrefix(part, "style ") || strings.HasPrefix(part, "classDef ") {
				continue
			}
			match := mermaidNodeIDPattern.FindStringSubmatch(part)
			if len(match) == 2 {
				nodes[match[1]] = true
			}
		}
	}
	return len(nodes)
}
