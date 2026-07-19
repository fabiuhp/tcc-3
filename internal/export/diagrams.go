package export

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	mismatchedDecisionNodePattern  = regexp.MustCompile(`(\b[A-Za-z][A-Za-z0-9_]*\{[^\r\n{}\[\]]*)\]`)
	mismatchedRectangleNodePattern = regexp.MustCompile(`(\b[A-Za-z][A-Za-z0-9_]*\[[^\r\n{}\[\]]*)\}`)
)

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
	decoder := json.NewDecoder(strings.NewReader(cleaned))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return BusinessDiagramDocument{}, fmt.Errorf("parse business diagrams: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return BusinessDiagramDocument{}, fmt.Errorf("parse business diagrams: unexpected data after JSON document")
	}
	if len(document.Diagrams) == 0 {
		return BusinessDiagramDocument{}, fmt.Errorf("business diagrams artifact has no diagrams")
	}
	if len(document.Diagrams) > 3 {
		return BusinessDiagramDocument{}, fmt.Errorf("business diagrams artifact has %d diagrams, maximum is 3", len(document.Diagrams))
	}
	for i := range document.Diagrams {
		document.Diagrams[i].Mermaid = normalizeMermaidSource(document.Diagrams[i].Mermaid)
		if err := ValidateBusinessDiagram(document.Diagrams[i]); err != nil {
			return BusinessDiagramDocument{}, fmt.Errorf("diagram %d: %w", i+1, err)
		}
	}
	return document, nil
}

func NormalizeBusinessDiagrams(content string) (string, error) {
	document, err := ParseBusinessDiagrams(content)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func ValidateBusinessDiagram(diagram BusinessDiagram) error {
	if strings.TrimSpace(diagram.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if diagram.Type != "business_flow" && diagram.Type != "user_journey" && diagram.Type != "state_flow" && diagram.Type != "decision_flow" {
		return fmt.Errorf("type %q is invalid", diagram.Type)
	}
	if strings.TrimSpace(diagram.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if strings.TrimSpace(diagram.Mermaid) == "" {
		return fmt.Errorf("mermaid source is required")
	}
	return nil
}

func normalizeMermaidSource(source string) string {
	source = mismatchedDecisionNodePattern.ReplaceAllString(source, `${1}}`)
	source = mismatchedRectangleNodePattern.ReplaceAllString(source, `${1}]`)
	return strings.TrimSpace(source)
}
