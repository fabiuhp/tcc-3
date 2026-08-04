package requirements_test

import (
	"strings"
	"testing"

	"github.com/fabiuhp/tcc-3/internal/domain"
	"github.com/fabiuhp/tcc-3/internal/requirements"
)

const (
	transcriptText  = "Cliente: Eu preciso acompanhar o pedido. Gestor: O prazo da notificação ainda precisa ser decidido."
	draftResponse   = `{"requirements":[{"type":"functional","statement":"O cliente deve acompanhar o pedido.","status":"confirmed","evidence":[{"quote":"Eu preciso acompanhar o pedido."}]},{"type":"business_rule","statement":"O prazo da notificação deve ser definido.","status":"pending","evidence":[{"quote":"O prazo da notificação ainda precisa ser decidido."}]}]}`
	reviewResponse  = `{"requirements":[{"id":"REQ-0001","type":"functional","statement":"O sistema deve permitir que o cliente acompanhe o pedido.","status":"confirmed","review":{"outcome":"revised","findings":["Redação ajustada para o formato imperativo."]}},{"id":"REQ-0002","type":"business_rule","statement":"O prazo da notificação deve ser definido pelos stakeholders.","status":"pending","review":{"outcome":"needs_clarification","findings":["O prazo não foi informado."]}}]}`
	gapResponse     = `{"gaps":[{"type":"missing_information","status":"pending","description":"O prazo da notificação não foi definido.","question":"Qual deve ser o prazo da notificação?","related_requirement_ids":["REQ-0002"]}]}`
	refinedResponse = `{"requirements":[{"id":"REQ-0001","statement":"O sistema deve permitir que o cliente acompanhe o pedido.","status":"confirmed"},{"id":"REQ-0002","statement":"O prazo da notificação deve ser definido pelos stakeholders.","status":"pending"}]}`
)

func TestNormalizeRequirementDocumentsPreservesTraceability(t *testing.T) {
	transcript := domain.Artifact{ID: "transcript-1", Type: domain.ArtifactTranscript, Content: transcriptText}

	draftContent, err := requirements.NormalizeDraft(draftResponse, transcript)
	if err != nil {
		t.Fatalf("normalize draft: %v", err)
	}
	draft, err := requirements.ParseDraft(draftContent)
	if err != nil {
		t.Fatalf("parse draft: %v", err)
	}
	if draft.Requirements[0].ID != "REQ-0001" || draft.Requirements[1].ID != "REQ-0002" {
		t.Fatalf("generated requirement ids = %q, %q", draft.Requirements[0].ID, draft.Requirements[1].ID)
	}
	if draft.Requirements[0].Evidence[0].ArtifactID != transcript.ID {
		t.Fatalf("evidence artifact id = %q, want %q", draft.Requirements[0].Evidence[0].ArtifactID, transcript.ID)
	}

	reviewedContent, err := requirements.NormalizeReview(reviewResponse, draftContent)
	if err != nil {
		t.Fatalf("normalize review: %v", err)
	}
	reviewed, err := requirements.ParseReviewed(reviewedContent)
	if err != nil {
		t.Fatalf("parse reviewed requirements: %v", err)
	}
	if reviewed.Requirements[0].Evidence[0] != draft.Requirements[0].Evidence[0] {
		t.Fatal("review normalization did not preserve the original evidence")
	}

	gapContent, err := requirements.NormalizeGapAnalysis(gapResponse, reviewedContent)
	if err != nil {
		t.Fatalf("normalize gap analysis: %v", err)
	}
	gaps, err := requirements.ParseGapAnalysis(gapContent)
	if err != nil {
		t.Fatalf("parse gap analysis: %v", err)
	}
	if gaps.Gaps[0].ID != "GAP-0001" {
		t.Fatalf("generated gap id = %q, want GAP-0001", gaps.Gaps[0].ID)
	}

	refinedContent, err := requirements.NormalizeRefined(refinedResponse, reviewedContent, gapContent)
	if err != nil {
		t.Fatalf("normalize refined requirements: %v", err)
	}
	refined, err := requirements.ParseRefined(refinedContent)
	if err != nil {
		t.Fatalf("parse refined requirements: %v", err)
	}
	if len(refined.Requirements) != 2 || len(refined.OpenGaps) != 1 {
		t.Fatalf("refined document = %+v", refined)
	}
	if refined.Requirements[1].Status != requirements.StatusPending {
		t.Fatalf("pending requirement status = %q", refined.Requirements[1].Status)
	}
	if len(refined.Requirements[1].RelatedGapIDs) != 1 || refined.Requirements[1].RelatedGapIDs[0] != "GAP-0001" {
		t.Fatalf("derived gap ids = %v, want GAP-0001", refined.Requirements[1].RelatedGapIDs)
	}
	if refined.Requirements[1].Type != reviewed.Requirements[1].Type || refined.Requirements[1].Evidence[0] != reviewed.Requirements[1].Evidence[0] {
		t.Fatal("refinement normalization did not reattach deterministic fields")
	}
}

func TestNormalizeDraftRejectsEvidenceOutsideTranscript(t *testing.T) {
	response := `{"requirements":[{"type":"functional","statement":"O sistema deve emitir nota fiscal.","status":"confirmed","evidence":[{"quote":"O sistema precisa emitir nota fiscal."}]}]}`
	_, err := requirements.NormalizeDraft(response, domain.Artifact{ID: "transcript-1", Content: transcriptText})
	if err == nil || !strings.Contains(err.Error(), "no quote that can be verified") {
		t.Fatalf("error = %v, want evidence validation error", err)
	}
}

func TestNormalizeDraftDiscardsUnverifiableAndDuplicateEvidence(t *testing.T) {
	response := `{"requirements":[{"type":"functional","statement":"O sistema deve permitir acompanhar o pedido.","status":"confirmed","evidence":[{"quote":"Eu preciso acompanhar o pedido!"},{"quote":"Eu preciso acompanhar o pedido."},{"quote":"O sistema também deve emitir nota fiscal."}]}]}`
	content, err := requirements.NormalizeDraft(response, domain.Artifact{ID: "transcript-1", Content: transcriptText})
	if err != nil {
		t.Fatalf("normalize draft: %v", err)
	}
	document, err := requirements.ParseDraft(content)
	if err != nil {
		t.Fatalf("parse draft: %v", err)
	}
	if len(document.Requirements[0].Evidence) != 1 {
		t.Fatalf("validated evidence count = %d, want 1", len(document.Requirements[0].Evidence))
	}
	if document.Requirements[0].Evidence[0].ArtifactID != "transcript-1" {
		t.Fatalf("artifact id = %q, want transcript-1", document.Requirements[0].Evidence[0].ArtifactID)
	}
}

func TestNormalizeDraftRejectsApplicationOwnedID(t *testing.T) {
	response := `{"requirements":[{"id":"REQ-9999","type":"functional","statement":"O sistema deve acompanhar o pedido.","status":"confirmed","evidence":[{"quote":"Eu preciso acompanhar o pedido."}]}]}`
	_, err := requirements.NormalizeDraft(response, domain.Artifact{ID: "transcript-1", Content: transcriptText})
	if err == nil || !strings.Contains(err.Error(), "unknown field \"id\"") {
		t.Fatalf("error = %v, want application-owned id rejection", err)
	}
}

func TestNormalizeReviewRejectsOmittedRequirement(t *testing.T) {
	draftContent, err := requirements.NormalizeDraft(draftResponse, domain.Artifact{ID: "transcript-1", Content: transcriptText})
	if err != nil {
		t.Fatalf("normalize draft: %v", err)
	}
	response := `{"requirements":[{"id":"REQ-0001","type":"functional","statement":"O sistema deve permitir acompanhamento.","status":"confirmed","review":{"outcome":"revised","findings":["Redação revisada."]}}]}`
	_, err = requirements.NormalizeReview(response, draftContent)
	if err == nil || !strings.Contains(err.Error(), "omitted id \"REQ-0002\"") {
		t.Fatalf("error = %v, want omitted requirement error", err)
	}
}

func TestNormalizeGapAnalysisRejectsUnknownRequirement(t *testing.T) {
	draftContent, reviewedContent := normalizedReviewInputs(t)
	_ = draftContent
	response := `{"gaps":[{"type":"ambiguity","status":"pending","description":"Referência inválida.","question":"Qual requisito?","related_requirement_ids":["REQ-9999"]}]}`
	_, err := requirements.NormalizeGapAnalysis(response, reviewedContent)
	if err == nil || !strings.Contains(err.Error(), "unknown requirement id") {
		t.Fatalf("error = %v, want unknown requirement error", err)
	}
}

func TestNormalizeGapAnalysisRejectsApplicationOwnedID(t *testing.T) {
	_, reviewedContent := normalizedReviewInputs(t)
	response := `{"gaps":[{"id":"GAP-9999","type":"ambiguity","status":"pending","description":"Prazo ambíguo.","question":"Qual prazo?","related_requirement_ids":["REQ-0002"]}]}`
	_, err := requirements.NormalizeGapAnalysis(response, reviewedContent)
	if err == nil || !strings.Contains(err.Error(), "unknown field \"id\"") {
		t.Fatalf("error = %v, want application-owned id rejection", err)
	}
}

func TestNormalizeRefinedRejectsPromotionToConfirmed(t *testing.T) {
	_, reviewedContent := normalizedReviewInputs(t)
	gapContent, err := requirements.NormalizeGapAnalysis(gapResponse, reviewedContent)
	if err != nil {
		t.Fatalf("normalize gaps: %v", err)
	}
	response := `{"requirements":[{"id":"REQ-0001","statement":"O sistema deve permitir acompanhamento.","status":"confirmed"},{"id":"REQ-0002","statement":"O prazo será de um dia.","status":"confirmed"}]}`
	_, err = requirements.NormalizeRefined(response, reviewedContent, gapContent)
	if err == nil || !strings.Contains(err.Error(), "promoted pending to confirmed") {
		t.Fatalf("error = %v, want status promotion error", err)
	}
}

func normalizedReviewInputs(t *testing.T) (string, string) {
	t.Helper()
	draftContent, err := requirements.NormalizeDraft(draftResponse, domain.Artifact{ID: "transcript-1", Content: transcriptText})
	if err != nil {
		t.Fatalf("normalize draft: %v", err)
	}
	reviewedContent, err := requirements.NormalizeReview(reviewResponse, draftContent)
	if err != nil {
		t.Fatalf("normalize review: %v", err)
	}
	return draftContent, reviewedContent
}
