package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/modules/extensions"
	"tars/internal/modules/skills"
)

func extensionsListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			items := listExtensions(deps.Extensions.List(), query)
			pageItems, meta := paginateItems(items, query)
			response := dto.ExtensionListResponse{Items: make([]dto.ExtensionCandidate, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				response.Items = append(response.Items, extensionCandidateToDTO(item, deps))
			}
			auditOpsRead(r.Context(), deps, "extension_candidate", "", "extensions_listed", map[string]any{"count": len(response.Items)})
			writeJSON(w, http.StatusOK, response)
		case http.MethodPost:
			var req struct {
				Bundle         dto.ExtensionBundle `json:"bundle"`
				OperatorReason string              `json:"operator_reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			bundle, err := extensionBundleFromDTO(req.Bundle)
			if err != nil {
				writeValidationError(w, err.Error())
				return
			}
			candidate, err := deps.Extensions.Generate(extensions.GenerateOptions{Bundle: bundle, OperatorReason: req.OperatorReason})
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "extension_candidate", candidate.ID, "extension_generated", map[string]any{"reason": req.OperatorReason, "skill_id": candidate.Bundle.Skill.Metadata.ID})
			writeJSON(w, http.StatusCreated, extensionCandidateToDTO(candidate, deps))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func extensionRouterHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/extensions/")
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "not_found", "extension candidate not found")
			return
		}
		switch {
		case path == "validate":
			extensionValidateHandler(deps)(w, r)
		case path == "import":
			extensionImportHandler(deps)(w, r)
		case path == "review":
			extensionReviewHandler(deps)(w, r)
		case strings.HasSuffix(path, "/validate"):
			extensionValidateCandidateHandler(deps, strings.TrimSuffix(path, "/validate"))(w, r)
		case strings.HasSuffix(path, "/import"):
			extensionImportCandidateHandler(deps, strings.TrimSuffix(path, "/import"))(w, r)
		case strings.HasSuffix(path, "/review"):
			extensionReviewCandidateHandler(deps, strings.TrimSuffix(path, "/review"))(w, r)
		default:
			extensionDetailHandler(deps, path)(w, r)
		}
	}
}

func extensionDetailHandler(deps Dependencies, candidateID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		candidate, ok := deps.Extensions.Get(strings.TrimSpace(candidateID))
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "extension candidate not found")
			return
		}
		auditOpsRead(r.Context(), deps, "extension_candidate", candidate.ID, "extension_viewed", nil)
		writeJSON(w, http.StatusOK, extensionCandidateToDTO(candidate, deps))
	}
}

func extensionValidateHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			Bundle dto.ExtensionBundle `json:"bundle"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		bundle, err := extensionBundleFromDTO(req.Bundle)
		if err != nil {
			writeValidationError(w, err.Error())
			return
		}
		normalized, validation, preview, err := deps.Extensions.ValidateBundle(bundle)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, dto.ExtensionValidationResponse{Bundle: extensionBundleToDTO(normalized, deps), Validation: extensionValidationDTO(validation), Preview: extensionPreviewDTO(preview)})
	}
}

func extensionValidateCandidateHandler(deps Dependencies, candidateID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		candidate, err := deps.Extensions.ValidateCandidate(strings.TrimSpace(candidateID))
		if err != nil {
			if err == extensions.ErrCandidateNotFound {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "extension_candidate", candidate.ID, "extension_validated", map[string]any{"valid": candidate.Validation.Valid})
		writeJSON(w, http.StatusOK, extensionCandidateToDTO(candidate, deps))
	}
}

func extensionImportHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			Bundle         dto.ExtensionBundle `json:"bundle"`
			OperatorReason string              `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		bundle, err := extensionBundleFromDTO(req.Bundle)
		if err != nil {
			writeValidationError(w, err.Error())
			return
		}
		candidate, err := deps.Extensions.Generate(extensions.GenerateOptions{Bundle: bundle, OperatorReason: req.OperatorReason})
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		candidate, err = deps.Extensions.ReviewCandidate(candidate.ID, extensions.ReviewOptions{State: extensions.ReviewApproved, OperatorReason: req.OperatorReason})
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		result, err := deps.Extensions.ImportCandidate(candidate.ID, req.OperatorReason)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "extension_candidate", result.Candidate.ID, "extension_imported", map[string]any{"skill_id": result.Manifest.Metadata.ID, "reason": req.OperatorReason})
		writeJSON(w, http.StatusOK, extensionImportResultToDTO(result, deps))
	}
}

func extensionReviewHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			Bundle         dto.ExtensionBundle `json:"bundle"`
			ReviewState    string              `json:"review_state"`
			OperatorReason string              `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		if strings.TrimSpace(req.ReviewState) == "" {
			writeValidationError(w, "review_state is required")
			return
		}
		if strings.TrimSpace(req.ReviewState) == extensions.ReviewImported {
			writeValidationError(w, "review_state cannot be imported")
			return
		}
		bundle, err := extensionBundleFromDTO(req.Bundle)
		if err != nil {
			writeValidationError(w, err.Error())
			return
		}
		candidate, err := deps.Extensions.Generate(extensions.GenerateOptions{Bundle: bundle, OperatorReason: req.OperatorReason})
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		reviewed, err := deps.Extensions.ReviewCandidate(candidate.ID, extensions.ReviewOptions{State: req.ReviewState, OperatorReason: req.OperatorReason})
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "extension_candidate", reviewed.ID, "extension_reviewed", map[string]any{"review_state": reviewed.ReviewState, "reason": req.OperatorReason})
		writeJSON(w, http.StatusOK, extensionCandidateToDTO(reviewed, deps))
	}
}

func extensionReviewCandidateHandler(deps Dependencies, candidateID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			ReviewState    string `json:"review_state"`
			OperatorReason string `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		if strings.TrimSpace(req.ReviewState) == "" {
			writeValidationError(w, "review_state is required")
			return
		}
		if strings.TrimSpace(req.ReviewState) == extensions.ReviewImported {
			writeValidationError(w, "review_state cannot be imported")
			return
		}
		reviewed, err := deps.Extensions.ReviewCandidate(strings.TrimSpace(candidateID), extensions.ReviewOptions{State: req.ReviewState, OperatorReason: req.OperatorReason})
		if err != nil {
			if err == extensions.ErrCandidateNotFound {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "extension_candidate", reviewed.ID, "extension_reviewed", map[string]any{"review_state": reviewed.ReviewState, "reason": req.OperatorReason})
		writeJSON(w, http.StatusOK, extensionCandidateToDTO(reviewed, deps))
	}
}

func extensionImportCandidateHandler(deps Dependencies, candidateID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Extensions == nil {
			writeError(w, http.StatusConflict, "not_configured", "extension manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			OperatorReason string `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		result, err := deps.Extensions.ImportCandidate(strings.TrimSpace(candidateID), req.OperatorReason)
		if err != nil {
			if err == extensions.ErrCandidateNotFound {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "extension_candidate", result.Candidate.ID, "extension_imported", map[string]any{"skill_id": result.Manifest.Metadata.ID, "reason": req.OperatorReason})
		writeJSON(w, http.StatusOK, extensionImportResultToDTO(result, deps))
	}
}

func listExtensions(items []extensions.Candidate, query listQuery) []extensions.Candidate {
	filtered := make([]extensions.Candidate, 0, len(items))
	needle := strings.ToLower(strings.TrimSpace(query.Query))
	for _, item := range items {
		if needle != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.ID,
				item.Bundle.Metadata.DisplayName,
				item.Bundle.Metadata.ID,
				item.Bundle.Skill.Metadata.ID,
				item.Bundle.Skill.Metadata.DisplayName,
				item.Status,
				item.ReviewState,
			}, " "))
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]
		sortBy := strings.ToLower(strings.TrimSpace(query.SortBy))
		desc := strings.ToLower(strings.TrimSpace(query.SortOrder)) != "asc"
		compare := func(a, b string) bool {
			if desc {
				return a > b
			}
			return a < b
		}
		switch sortBy {
		case "status":
			if left.Status == right.Status {
				return compare(left.ID, right.ID)
			}
			return compare(left.Status, right.Status)
		case "skill_id":
			if left.Bundle.Skill.Metadata.ID == right.Bundle.Skill.Metadata.ID {
				return compare(left.ID, right.ID)
			}
			return compare(left.Bundle.Skill.Metadata.ID, right.Bundle.Skill.Metadata.ID)
		default:
			if left.UpdatedAt.Equal(right.UpdatedAt) {
				return compare(left.ID, right.ID)
			}
			if desc {
				return left.UpdatedAt.After(right.UpdatedAt)
			}
			return left.UpdatedAt.Before(right.UpdatedAt)
		}
	})
	return filtered
}

func extensionCandidateToDTO(item extensions.Candidate, deps Dependencies) dto.ExtensionCandidate {
	return dto.ExtensionCandidate{
		ID:              item.ID,
		Status:          item.Status,
		ReviewState:     item.ReviewState,
		ReviewHistory:   extensionReviewHistoryToDTO(item.ReviewHistory),
		ImportedSkillID: item.ImportedSkillID,
		ImportedAt:      item.ImportedAt,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		Bundle:          extensionBundleToDTO(item.Bundle, deps),
		Validation:      extensionValidationDTO(item.Validation),
		Preview:         extensionPreviewDTO(item.Preview),
	}
}

func extensionReviewHistoryToDTO(items []extensions.ReviewEvent) []dto.ExtensionReviewEvent {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ExtensionReviewEvent, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ExtensionReviewEvent{State: item.State, Reason: item.Reason, CreatedAt: item.CreatedAt, ImportedBy: item.ImportedBy})
	}
	return out
}

func extensionBundleToDTO(item extensions.Bundle, deps Dependencies) dto.ExtensionBundle {
	return dto.ExtensionBundle{
		APIVersion: item.APIVersion,
		Kind:       item.Kind,
		Metadata: dto.ExtensionBundleMetadata{
			ID:          item.Metadata.ID,
			DisplayName: item.Metadata.DisplayName,
			Summary:     item.Metadata.Summary,
			Source:      item.Metadata.Source,
			GeneratedBy: item.Metadata.GeneratedBy,
			CreatedAt:   item.Metadata.CreatedAt,
		},
		Skill: extensionSkillManifestToDTO(item.Skill, deps),
		Docs:  extensionDocsToDTO(item.Docs),
		Tests: extensionTestsToDTO(item.Tests),
		Compatibility: map[string]interface{}{
			"compatible":         item.Compatibility.Compatible,
			"current_tars_major": item.Compatibility.CurrentTARSMajor,
			"reasons":            item.Compatibility.Reasons,
			"checked_at":         item.Compatibility.CheckedAt,
		},
	}
}

func extensionSkillManifestToDTO(entry skills.Manifest, deps Dependencies) dto.SkillManifest {
	return skillManifestToDTO(entry, deps)
}

func extensionDocsToDTO(items []extensions.DocsAsset) []dto.ExtensionDocAsset {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ExtensionDocAsset, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ExtensionDocAsset{ID: item.ID, Slug: item.Slug, Title: item.Title, Format: item.Format, Summary: item.Summary, Content: item.Content})
	}
	return out
}

func extensionTestsToDTO(items []extensions.TestSpec) []dto.ExtensionTestSpec {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ExtensionTestSpec, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ExtensionTestSpec{ID: item.ID, Name: item.Name, Kind: item.Kind, Command: item.Command})
	}
	return out
}

func extensionValidationDTO(item extensions.ValidationReport) dto.ExtensionValidationReport {
	return dto.ExtensionValidationReport{Valid: item.Valid, Errors: append([]string(nil), item.Errors...), Warnings: append([]string(nil), item.Warnings...), CheckedAt: item.CheckedAt}
}

func extensionPreviewDTO(item extensions.PreviewSummary) dto.ExtensionPreviewSummary {
	return dto.ExtensionPreviewSummary{ChangeType: item.ChangeType, Summary: append([]string(nil), item.Summary...)}
}

func extensionImportResultToDTO(result extensions.ImportResult, deps Dependencies) dto.ExtensionImportResponse {
	return dto.ExtensionImportResponse{Candidate: extensionCandidateToDTO(result.Candidate, deps), Manifest: skillManifestToDTO(result.Manifest, deps), State: skillLifecycleDTO(result.State)}
}

func extensionBundleFromDTO(input dto.ExtensionBundle) (extensions.Bundle, error) {
	skillManifest, err := skillManifestFromDTO(input.Skill)
	if err != nil {
		return extensions.Bundle{}, err
	}
	return extensions.Bundle{
		APIVersion: input.APIVersion,
		Kind:       input.Kind,
		Metadata: extensions.BundleMetadata{
			ID:          input.Metadata.ID,
			DisplayName: input.Metadata.DisplayName,
			Summary:     input.Metadata.Summary,
			Source:      input.Metadata.Source,
			GeneratedBy: input.Metadata.GeneratedBy,
			CreatedAt:   input.Metadata.CreatedAt,
		},
		Skill: skillManifest,
		Docs:  extensionDocsFromDTO(input.Docs),
		Tests: extensionTestsFromDTO(input.Tests),
	}, nil
}

func extensionDocsFromDTO(items []dto.ExtensionDocAsset) []extensions.DocsAsset {
	if len(items) == 0 {
		return nil
	}
	out := make([]extensions.DocsAsset, 0, len(items))
	for _, item := range items {
		out = append(out, extensions.DocsAsset{ID: item.ID, Slug: item.Slug, Title: item.Title, Format: item.Format, Summary: item.Summary, Content: item.Content})
	}
	return out
}

func extensionTestsFromDTO(items []dto.ExtensionTestSpec) []extensions.TestSpec {
	if len(items) == 0 {
		return nil
	}
	out := make([]extensions.TestSpec, 0, len(items))
	for _, item := range items {
		out = append(out, extensions.TestSpec{ID: item.ID, Name: item.Name, Kind: item.Kind, Command: item.Command})
	}
	return out
}
