package requirements

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"requirement-pipeline/internal/domain"
)

type RequirementType string

const (
	TypeFunctional    RequirementType = "functional"
	TypeNonFunctional RequirementType = "non_functional"
	TypeBusinessRule  RequirementType = "business_rule"
	TypeConstraint    RequirementType = "constraint"
)

type RequirementStatus string

const (
	StatusConfirmed  RequirementStatus = "confirmed"
	StatusAssumption RequirementStatus = "assumption"
	StatusPending    RequirementStatus = "pending"
)

type Evidence struct {
	ArtifactID string `json:"artifact_id"`
	Quote      string `json:"quote"`
}

type Requirement struct {
	ID        string            `json:"id"`
	Type      RequirementType   `json:"type"`
	Statement string            `json:"statement"`
	Status    RequirementStatus `json:"status"`
	Evidence  []Evidence        `json:"evidence"`
}

type DraftDocument struct {
	Requirements []Requirement `json:"requirements"`
}

type draftResponseEvidence struct {
	Quote string `json:"quote"`
}

type draftResponseRequirement struct {
	Type      RequirementType         `json:"type"`
	Statement string                  `json:"statement"`
	Status    RequirementStatus       `json:"status"`
	Evidence  []draftResponseEvidence `json:"evidence"`
}

type draftResponse struct {
	Requirements []draftResponseRequirement `json:"requirements"`
}

type ReviewOutcome string

const (
	ReviewApproved           ReviewOutcome = "approved"
	ReviewRevised            ReviewOutcome = "revised"
	ReviewNeedsClarification ReviewOutcome = "needs_clarification"
)

type Review struct {
	Outcome  ReviewOutcome `json:"outcome"`
	Findings []string      `json:"findings"`
}

type ReviewedRequirement struct {
	Requirement
	Review Review `json:"review"`
}

type ReviewedDocument struct {
	Requirements []ReviewedRequirement `json:"requirements"`
}

type reviewResponseRequirement struct {
	ID        string            `json:"id"`
	Type      RequirementType   `json:"type"`
	Statement string            `json:"statement"`
	Status    RequirementStatus `json:"status"`
	Review    Review            `json:"review"`
}

type reviewResponse struct {
	Requirements []reviewResponseRequirement `json:"requirements"`
}

type GapType string

const (
	GapMissingInformation GapType = "missing_information"
	GapAmbiguity          GapType = "ambiguity"
	GapConflict           GapType = "conflict"
	GapIncompleteRule     GapType = "incomplete_rule"
)

type Gap struct {
	ID                    string            `json:"id"`
	Type                  GapType           `json:"type"`
	Status                RequirementStatus `json:"status"`
	Description           string            `json:"description"`
	Question              string            `json:"question"`
	RelatedRequirementIDs []string          `json:"related_requirement_ids"`
}

type GapDocument struct {
	Gaps []Gap `json:"gaps"`
}

type gapResponseItem struct {
	Type                  GapType           `json:"type"`
	Status                RequirementStatus `json:"status"`
	Description           string            `json:"description"`
	Question              string            `json:"question"`
	RelatedRequirementIDs []string          `json:"related_requirement_ids"`
}

type gapResponse struct {
	Gaps []gapResponseItem `json:"gaps"`
}

type RefinedRequirement struct {
	Requirement
	RelatedGapIDs []string `json:"related_gap_ids"`
}

type RefinedDocument struct {
	Requirements []RefinedRequirement `json:"requirements"`
	OpenGaps     []Gap                `json:"open_gaps"`
}

type refinementResponse struct {
	Requirements []refinementResponseRequirement `json:"requirements"`
}

type refinementResponseRequirement struct {
	ID        string            `json:"id"`
	Statement string            `json:"statement"`
	Status    RequirementStatus `json:"status"`
}

func NormalizeDraft(response string, transcript domain.Artifact) (string, error) {
	var responseDocument draftResponse
	if err := decodeStrict(response, &responseDocument); err != nil {
		return "", fmt.Errorf("parse requirement draft: %w", err)
	}
	if responseDocument.Requirements == nil {
		return "", fmt.Errorf("requirement draft requirements must be an array")
	}
	document := DraftDocument{Requirements: make([]Requirement, len(responseDocument.Requirements))}
	for i, responseRequirement := range responseDocument.Requirements {
		evidence := make([]Evidence, len(responseRequirement.Evidence))
		for j, responseEvidence := range responseRequirement.Evidence {
			evidence[j] = Evidence{Quote: responseEvidence.Quote}
		}
		document.Requirements[i] = Requirement{
			ID:        fmt.Sprintf("REQ-%04d", i+1),
			Type:      responseRequirement.Type,
			Statement: responseRequirement.Statement,
			Status:    responseRequirement.Status,
			Evidence:  evidence,
		}
		if err := normalizeRequirement(&document.Requirements[i], true, &transcript); err != nil {
			return "", fmt.Errorf("requirement draft requirement %d: %w", i+1, err)
		}
	}
	return encodeCanonical(document)
}

func NormalizeReview(response, draftContent string) (string, error) {
	draft, err := ParseDraft(draftContent)
	if err != nil {
		return "", fmt.Errorf("parse review input: %w", err)
	}

	var responseDocument reviewResponse
	if err := decodeStrict(response, &responseDocument); err != nil {
		return "", fmt.Errorf("parse reviewed requirements: %w", err)
	}
	if responseDocument.Requirements == nil {
		return "", fmt.Errorf("reviewed requirements must be an array")
	}

	reviewedByID := make(map[string]reviewResponseRequirement, len(responseDocument.Requirements))
	for i := range responseDocument.Requirements {
		requirement := &responseDocument.Requirements[i]
		requirement.ID = strings.TrimSpace(requirement.ID)
		requirement.Type = RequirementType(strings.TrimSpace(string(requirement.Type)))
		requirement.Statement = strings.TrimSpace(requirement.Statement)
		requirement.Status = RequirementStatus(strings.TrimSpace(string(requirement.Status)))
		if err := validateRequirementFields(requirement.ID, requirement.Type, requirement.Statement, requirement.Status, true); err != nil {
			return "", fmt.Errorf("reviewed requirement %d: %w", i+1, err)
		}
		if err := normalizeReview(&requirement.Review); err != nil {
			return "", fmt.Errorf("reviewed requirement %s: %w", requirement.ID, err)
		}
		if _, exists := reviewedByID[requirement.ID]; exists {
			return "", fmt.Errorf("reviewed requirements contain duplicate id %q", requirement.ID)
		}
		reviewedByID[requirement.ID] = *requirement
	}

	ordered := make([]ReviewedRequirement, 0, len(draft.Requirements))
	for _, original := range draft.Requirements {
		responseRequirement, ok := reviewedByID[original.ID]
		if !ok {
			return "", fmt.Errorf("reviewed requirements omitted id %q", original.ID)
		}
		if promotesToConfirmed(original.Status, responseRequirement.Status) {
			return "", fmt.Errorf("reviewed requirement %q promoted %s to confirmed without new evidence", original.ID, original.Status)
		}
		ordered = append(ordered, ReviewedRequirement{
			Requirement: Requirement{
				ID:        responseRequirement.ID,
				Type:      responseRequirement.Type,
				Statement: responseRequirement.Statement,
				Status:    responseRequirement.Status,
				Evidence:  original.Evidence,
			},
			Review: responseRequirement.Review,
		})
		delete(reviewedByID, original.ID)
	}
	if len(reviewedByID) != 0 {
		return "", fmt.Errorf("reviewed requirements introduced unknown ids: %s", joinMapKeys(reviewedByID))
	}
	document := ReviewedDocument{Requirements: ordered}
	return encodeCanonical(document)
}

func NormalizeGapAnalysis(response, reviewedContent string) (string, error) {
	reviewed, err := ParseReviewed(reviewedContent)
	if err != nil {
		return "", fmt.Errorf("parse gap analysis input: %w", err)
	}
	validRequirementIDs := make(map[string]bool, len(reviewed.Requirements))
	for _, requirement := range reviewed.Requirements {
		validRequirementIDs[requirement.ID] = true
	}

	var responseDocument gapResponse
	if err := decodeStrict(response, &responseDocument); err != nil {
		return "", fmt.Errorf("parse gap analysis: %w", err)
	}
	if responseDocument.Gaps == nil {
		return "", fmt.Errorf("gap analysis gaps must be an array")
	}
	document := GapDocument{Gaps: make([]Gap, len(responseDocument.Gaps))}
	for i, responseGap := range responseDocument.Gaps {
		document.Gaps[i] = Gap{
			ID:                    fmt.Sprintf("GAP-%04d", i+1),
			Type:                  responseGap.Type,
			Status:                responseGap.Status,
			Description:           responseGap.Description,
			Question:              responseGap.Question,
			RelatedRequirementIDs: responseGap.RelatedRequirementIDs,
		}
		if err := normalizeGap(&document.Gaps[i], true, validRequirementIDs); err != nil {
			return "", fmt.Errorf("gap analysis item %d: %w", i+1, err)
		}
	}
	return encodeCanonical(document)
}

func NormalizeRefined(response, reviewedContent, gapContent string) (string, error) {
	reviewed, err := ParseReviewed(reviewedContent)
	if err != nil {
		return "", fmt.Errorf("parse refinement requirements input: %w", err)
	}
	gaps, err := ParseGapAnalysis(gapContent)
	if err != nil {
		return "", fmt.Errorf("parse refinement gap input: %w", err)
	}

	var responseDocument refinementResponse
	if err := decodeStrict(response, &responseDocument); err != nil {
		return "", fmt.Errorf("parse refined requirements: %w", err)
	}
	if responseDocument.Requirements == nil {
		return "", fmt.Errorf("refined requirements must be an array")
	}

	validRequirementIDs := make(map[string]bool, len(reviewed.Requirements))
	relatedGapIDs := make(map[string][]string, len(reviewed.Requirements))
	for _, requirement := range reviewed.Requirements {
		validRequirementIDs[requirement.ID] = true
		relatedGapIDs[requirement.ID] = []string{}
	}
	for _, gap := range gaps.Gaps {
		if err := validateReferences(gap.RelatedRequirementIDs, validRequirementIDs, "requirement"); err != nil {
			return "", fmt.Errorf("gap %q: %w", gap.ID, err)
		}
		for _, requirementID := range gap.RelatedRequirementIDs {
			relatedGapIDs[requirementID] = append(relatedGapIDs[requirementID], gap.ID)
		}
	}
	refinedByID := make(map[string]refinementResponseRequirement, len(responseDocument.Requirements))
	for i := range responseDocument.Requirements {
		requirement := &responseDocument.Requirements[i]
		requirement.ID = strings.TrimSpace(requirement.ID)
		requirement.Statement = strings.TrimSpace(requirement.Statement)
		requirement.Status = RequirementStatus(strings.TrimSpace(string(requirement.Status)))
		if err := validateRefinementFields(requirement.ID, requirement.Statement, requirement.Status); err != nil {
			return "", fmt.Errorf("refined requirement %d: %w", i+1, err)
		}
		if _, exists := refinedByID[requirement.ID]; exists {
			return "", fmt.Errorf("refined requirements contain duplicate id %q", requirement.ID)
		}
		refinedByID[requirement.ID] = *requirement
	}

	ordered := make([]RefinedRequirement, 0, len(reviewed.Requirements))
	for _, original := range reviewed.Requirements {
		responseRequirement, ok := refinedByID[original.ID]
		if !ok {
			return "", fmt.Errorf("refined requirements omitted id %q", original.ID)
		}
		if promotesToConfirmed(original.Status, responseRequirement.Status) {
			return "", fmt.Errorf("refined requirement %q promoted %s to confirmed without new evidence", original.ID, original.Status)
		}
		ordered = append(ordered, RefinedRequirement{
			Requirement: Requirement{
				ID:        responseRequirement.ID,
				Type:      original.Type,
				Statement: responseRequirement.Statement,
				Status:    responseRequirement.Status,
				Evidence:  original.Evidence,
			},
			RelatedGapIDs: relatedGapIDs[responseRequirement.ID],
		})
		delete(refinedByID, original.ID)
	}
	if len(refinedByID) != 0 {
		return "", fmt.Errorf("refined requirements introduced unknown ids: %s", joinMapKeys(refinedByID))
	}
	if err := validateGapCoverage(ordered, gaps.Gaps); err != nil {
		return "", err
	}

	document := RefinedDocument{Requirements: ordered, OpenGaps: gaps.Gaps}
	return encodeCanonical(document)
}

func ParseDraft(content string) (DraftDocument, error) {
	var document DraftDocument
	if err := decodeStrict(content, &document); err != nil {
		return DraftDocument{}, err
	}
	if document.Requirements == nil {
		return DraftDocument{}, fmt.Errorf("requirements must be an array")
	}
	if err := validateRequirements(document.Requirements); err != nil {
		return DraftDocument{}, err
	}
	return document, nil
}

func ParseReviewed(content string) (ReviewedDocument, error) {
	var document ReviewedDocument
	if err := decodeStrict(content, &document); err != nil {
		return ReviewedDocument{}, err
	}
	if document.Requirements == nil {
		return ReviewedDocument{}, fmt.Errorf("requirements must be an array")
	}
	seen := make(map[string]bool, len(document.Requirements))
	for i := range document.Requirements {
		if err := normalizeRequirement(&document.Requirements[i].Requirement, true, nil); err != nil {
			return ReviewedDocument{}, fmt.Errorf("requirement %d: %w", i+1, err)
		}
		if seen[document.Requirements[i].ID] {
			return ReviewedDocument{}, fmt.Errorf("duplicate requirement id %q", document.Requirements[i].ID)
		}
		seen[document.Requirements[i].ID] = true
		if err := normalizeReview(&document.Requirements[i].Review); err != nil {
			return ReviewedDocument{}, fmt.Errorf("requirement %s: %w", document.Requirements[i].ID, err)
		}
	}
	return document, nil
}

func ParseGapAnalysis(content string) (GapDocument, error) {
	var document GapDocument
	if err := decodeStrict(content, &document); err != nil {
		return GapDocument{}, err
	}
	if document.Gaps == nil {
		return GapDocument{}, fmt.Errorf("gaps must be an array")
	}
	seen := make(map[string]bool, len(document.Gaps))
	for i := range document.Gaps {
		if err := normalizeGap(&document.Gaps[i], true, nil); err != nil {
			return GapDocument{}, fmt.Errorf("gap %d: %w", i+1, err)
		}
		if seen[document.Gaps[i].ID] {
			return GapDocument{}, fmt.Errorf("duplicate gap id %q", document.Gaps[i].ID)
		}
		seen[document.Gaps[i].ID] = true
	}
	return document, nil
}

func ParseRefined(content string) (RefinedDocument, error) {
	var document RefinedDocument
	if err := decodeStrict(content, &document); err != nil {
		return RefinedDocument{}, err
	}
	if document.Requirements == nil {
		return RefinedDocument{}, fmt.Errorf("requirements must be an array")
	}
	if document.OpenGaps == nil {
		return RefinedDocument{}, fmt.Errorf("open_gaps must be an array")
	}

	validGapIDs := make(map[string]bool, len(document.OpenGaps))
	for i := range document.OpenGaps {
		if err := normalizeGap(&document.OpenGaps[i], true, nil); err != nil {
			return RefinedDocument{}, fmt.Errorf("open gap %d: %w", i+1, err)
		}
		if validGapIDs[document.OpenGaps[i].ID] {
			return RefinedDocument{}, fmt.Errorf("duplicate gap id %q", document.OpenGaps[i].ID)
		}
		validGapIDs[document.OpenGaps[i].ID] = true
	}

	seen := make(map[string]bool, len(document.Requirements))
	for i := range document.Requirements {
		requirement := &document.Requirements[i]
		if err := normalizeRequirement(&requirement.Requirement, true, nil); err != nil {
			return RefinedDocument{}, fmt.Errorf("requirement %d: %w", i+1, err)
		}
		if seen[requirement.ID] {
			return RefinedDocument{}, fmt.Errorf("duplicate requirement id %q", requirement.ID)
		}
		seen[requirement.ID] = true
		if requirement.RelatedGapIDs == nil {
			return RefinedDocument{}, fmt.Errorf("requirement %q related_gap_ids must be an array", requirement.ID)
		}
		if err := validateReferences(requirement.RelatedGapIDs, validGapIDs, "gap"); err != nil {
			return RefinedDocument{}, fmt.Errorf("requirement %q: %w", requirement.ID, err)
		}
	}
	if err := validateGapCoverage(document.Requirements, document.OpenGaps); err != nil {
		return RefinedDocument{}, err
	}
	return document, nil
}

func validateRequirements(requirements []Requirement) error {
	seen := make(map[string]bool, len(requirements))
	for i := range requirements {
		if err := normalizeRequirement(&requirements[i], true, nil); err != nil {
			return fmt.Errorf("requirement %d: %w", i+1, err)
		}
		if seen[requirements[i].ID] {
			return fmt.Errorf("duplicate requirement id %q", requirements[i].ID)
		}
		seen[requirements[i].ID] = true
	}
	return nil
}

func normalizeRequirement(requirement *Requirement, requireID bool, transcript *domain.Artifact) error {
	requirement.ID = strings.TrimSpace(requirement.ID)
	requirement.Type = RequirementType(strings.TrimSpace(string(requirement.Type)))
	requirement.Statement = strings.TrimSpace(requirement.Statement)
	requirement.Status = RequirementStatus(strings.TrimSpace(string(requirement.Status)))
	if err := validateRequirementFields(requirement.ID, requirement.Type, requirement.Statement, requirement.Status, requireID); err != nil {
		return err
	}
	if requirement.Evidence == nil || len(requirement.Evidence) == 0 {
		return fmt.Errorf("evidence must contain at least one source quote")
	}
	seenQuotes := make(map[string]bool, len(requirement.Evidence))
	validatedEvidence := make([]Evidence, 0, len(requirement.Evidence))
	for i := range requirement.Evidence {
		evidence := &requirement.Evidence[i]
		evidence.ArtifactID = strings.TrimSpace(evidence.ArtifactID)
		evidence.Quote = strings.TrimSpace(evidence.Quote)
		if evidence.Quote == "" {
			if transcript != nil {
				continue
			}
			return fmt.Errorf("evidence %d quote is required", i+1)
		}
		normalizedQuote := normalizeText(evidence.Quote)
		if normalizedQuote == "" {
			if transcript != nil {
				continue
			}
			return fmt.Errorf("evidence %d quote has no searchable text", i+1)
		}
		if transcript != nil {
			if !containsNormalized(transcript.Content, evidence.Quote) {
				continue
			}
			evidence.ArtifactID = transcript.ID
		} else if evidence.ArtifactID == "" {
			return fmt.Errorf("evidence %d artifact_id is required", i+1)
		}
		if seenQuotes[normalizedQuote] {
			if transcript != nil {
				continue
			}
			return fmt.Errorf("evidence contains duplicate quote %q", evidence.Quote)
		}
		seenQuotes[normalizedQuote] = true
		validatedEvidence = append(validatedEvidence, *evidence)
	}
	if len(validatedEvidence) == 0 {
		return fmt.Errorf("evidence has no quote that can be verified in the transcript")
	}
	requirement.Evidence = validatedEvidence
	return nil
}

func validateRequirementFields(id string, requirementType RequirementType, statement string, status RequirementStatus, requireID bool) error {
	if requireID && id == "" {
		return fmt.Errorf("id is required")
	}
	if !validRequirementType(requirementType) {
		return fmt.Errorf("type %q is invalid", requirementType)
	}
	if statement == "" {
		return fmt.Errorf("statement is required")
	}
	if !validRequirementStatus(status) {
		return fmt.Errorf("status %q is invalid", status)
	}
	return nil
}

func validateRefinementFields(id, statement string, status RequirementStatus) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if statement == "" {
		return fmt.Errorf("statement is required")
	}
	if !validRequirementStatus(status) {
		return fmt.Errorf("status %q is invalid", status)
	}
	return nil
}

func normalizeReview(review *Review) error {
	review.Outcome = ReviewOutcome(strings.TrimSpace(string(review.Outcome)))
	if review.Outcome != ReviewApproved && review.Outcome != ReviewRevised && review.Outcome != ReviewNeedsClarification {
		return fmt.Errorf("review outcome %q is invalid", review.Outcome)
	}
	if review.Findings == nil {
		return fmt.Errorf("review findings must be an array")
	}
	for i := range review.Findings {
		review.Findings[i] = strings.TrimSpace(review.Findings[i])
		if review.Findings[i] == "" {
			return fmt.Errorf("review finding %d is empty", i+1)
		}
	}
	if review.Outcome != ReviewApproved && len(review.Findings) == 0 {
		return fmt.Errorf("review outcome %q requires at least one finding", review.Outcome)
	}
	return nil
}

func normalizeGap(gap *Gap, requireID bool, validRequirementIDs map[string]bool) error {
	gap.ID = strings.TrimSpace(gap.ID)
	gap.Type = GapType(strings.TrimSpace(string(gap.Type)))
	gap.Status = RequirementStatus(strings.TrimSpace(string(gap.Status)))
	gap.Description = strings.TrimSpace(gap.Description)
	gap.Question = strings.TrimSpace(gap.Question)
	if requireID && gap.ID == "" {
		return fmt.Errorf("id is required")
	}
	if gap.Type != GapMissingInformation && gap.Type != GapAmbiguity && gap.Type != GapConflict && gap.Type != GapIncompleteRule {
		return fmt.Errorf("type %q is invalid", gap.Type)
	}
	if gap.Status != StatusPending {
		return fmt.Errorf("status = %q, want %q", gap.Status, StatusPending)
	}
	if gap.Description == "" {
		return fmt.Errorf("description is required")
	}
	if gap.Question == "" {
		return fmt.Errorf("question is required")
	}
	if gap.RelatedRequirementIDs == nil || len(gap.RelatedRequirementIDs) == 0 {
		return fmt.Errorf("related_requirement_ids must contain at least one requirement")
	}
	if validRequirementIDs != nil {
		if err := validateReferences(gap.RelatedRequirementIDs, validRequirementIDs, "requirement"); err != nil {
			return err
		}
	} else if err := validateUniqueNonEmpty(gap.RelatedRequirementIDs, "requirement"); err != nil {
		return err
	}
	return nil
}

func validateReferences(ids []string, valid map[string]bool, kind string) error {
	if err := validateUniqueNonEmpty(ids, kind); err != nil {
		return err
	}
	for _, id := range ids {
		if !valid[id] {
			return fmt.Errorf("unknown %s id %q", kind, id)
		}
	}
	return nil
}

func validateUniqueNonEmpty(ids []string, kind string) error {
	seen := make(map[string]bool, len(ids))
	for i := range ids {
		ids[i] = strings.TrimSpace(ids[i])
		if ids[i] == "" {
			return fmt.Errorf("%s id is empty", kind)
		}
		if seen[ids[i]] {
			return fmt.Errorf("duplicate %s id %q", kind, ids[i])
		}
		seen[ids[i]] = true
	}
	return nil
}

func validateGapCoverage(requirements []RefinedRequirement, gaps []Gap) error {
	relatedGapsByRequirement := make(map[string]map[string]bool, len(requirements))
	for _, requirement := range requirements {
		related := make(map[string]bool, len(requirement.RelatedGapIDs))
		for _, gapID := range requirement.RelatedGapIDs {
			related[gapID] = true
		}
		relatedGapsByRequirement[requirement.ID] = related
	}
	for _, gap := range gaps {
		for _, requirementID := range gap.RelatedRequirementIDs {
			if !relatedGapsByRequirement[requirementID][gap.ID] {
				return fmt.Errorf("refined requirement %q does not reference related gap %q", requirementID, gap.ID)
			}
		}
	}
	return nil
}

func validRequirementType(value RequirementType) bool {
	return value == TypeFunctional || value == TypeNonFunctional || value == TypeBusinessRule || value == TypeConstraint
}

func validRequirementStatus(value RequirementStatus) bool {
	return value == StatusConfirmed || value == StatusAssumption || value == StatusPending
}

func promotesToConfirmed(from, to RequirementStatus) bool {
	return from != StatusConfirmed && to == StatusConfirmed
}

func containsNormalized(content, quote string) bool {
	return strings.Contains(normalizeText(content), normalizeText(quote))
}

func normalizeText(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func joinMapKeys[T any](values map[string]T) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return strings.Join(keys, ", ")
}

func encodeCanonical(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func decodeStrict(content string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(cleanJSON(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected data after JSON document")
		}
		return err
	}
	return nil
}

func cleanJSON(content string) string {
	cleaned := strings.TrimSpace(content)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}
