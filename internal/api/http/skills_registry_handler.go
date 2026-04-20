package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/modules/skills"

	"gopkg.in/yaml.v3"
)

func skillsListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			items := listSkills(skillsSnapshotEntries(deps), skillsFilter{
				Status:    strings.TrimSpace(r.URL.Query().Get("status")),
				Source:    strings.TrimSpace(r.URL.Query().Get("source")),
				Enabled:   parseOptionalBool(r.URL.Query().Get("enabled")),
				Query:     query.Query,
				SortBy:    query.SortBy,
				SortOrder: query.SortOrder,
			}, deps)
			pageItems, meta := paginateItems(items, query)
			response := dto.SkillListResponse{Items: make([]dto.SkillManifest, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				response.Items = append(response.Items, skillManifestToDTO(item, deps))
			}
			auditOpsRead(r.Context(), deps, "skill_registry", "", "skills_listed", map[string]any{"count": len(response.Items)})
			writeJSON(w, http.StatusOK, response)
		case http.MethodPost:
			var req struct {
				Manifest       dto.SkillManifest `json:"manifest"`
				OperatorReason string            `json:"operator_reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			manifest, err := skillManifestFromDTO(req.Manifest)
			if err != nil {
				writeValidationError(w, err.Error())
				return
			}
			updated, state, err := deps.Skills.Upsert(skills.UpsertOptions{Manifest: manifest, Reason: req.OperatorReason, Action: "skill_created", Source: manifest.Metadata.Source, Status: "draft"})
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_skill", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "skill", updated.Metadata.ID, "skill_created", map[string]any{"reason": req.OperatorReason})
			writeJSON(w, http.StatusCreated, dto.SkillImportResponse{Manifest: skillManifestToDTO(updated, deps), State: skillLifecycleDTO(state)})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func skillRouterHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/skills/")
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "not_found", "skill not found")
			return
		}
		switch {
		case strings.HasSuffix(path, "/enable"):
			skillEnableHandler(deps, strings.TrimSuffix(path, "/enable"), true)(w, r)
		case strings.HasSuffix(path, "/disable"):
			skillEnableHandler(deps, strings.TrimSuffix(path, "/disable"), false)(w, r)
		case strings.HasSuffix(path, "/promote"):
			skillPromoteHandler(deps, strings.TrimSuffix(path, "/promote"))(w, r)
		case strings.HasSuffix(path, "/rollback"):
			skillRollbackHandler(deps, strings.TrimSuffix(path, "/rollback"))(w, r)
		case strings.HasSuffix(path, "/export"):
			skillExportHandler(deps, strings.TrimSuffix(path, "/export"))(w, r)
		default:
			skillDetailHandler(deps, path)(w, r)
		}
	}
}

func skillDetailHandler(deps Dependencies, skillID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		skillID = strings.TrimSpace(skillID)
		if skillID == "" {
			writeError(w, http.StatusNotFound, "not_found", "skill not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			entry, ok := deps.Skills.Get(skillID)
			if !ok {
				writeError(w, http.StatusNotFound, "not_found", "skill not found")
				return
			}
			auditOpsRead(r.Context(), deps, "skill", skillID, "skill_viewed", nil)
			writeJSON(w, http.StatusOK, skillManifestToDTO(entry, deps))
		case http.MethodPut:
			var req struct {
				Manifest       dto.SkillManifest `json:"manifest"`
				OperatorReason string            `json:"operator_reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			manifest, err := skillManifestFromDTO(req.Manifest)
			if err != nil {
				writeValidationError(w, err.Error())
				return
			}
			if manifest.Metadata.ID != skillID {
				writeValidationError(w, "manifest metadata.id must match skill id")
				return
			}
			updated, _, err := deps.Skills.Upsert(skills.UpsertOptions{Manifest: manifest, Reason: req.OperatorReason, Action: "skill_updated", Source: manifest.Metadata.Source})
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_skill", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "skill", skillID, "skill_updated", map[string]any{"reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, skillManifestToDTO(updated, deps))
		case http.MethodDelete:
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
			if err := deps.Skills.Delete(skillID, req.OperatorReason); err != nil {
				if err == skills.ErrSkillNotFound {
					writeError(w, http.StatusNotFound, "not_found", "skill not found")
				} else {
					writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
				}
				return
			}
			auditOpsWrite(r.Context(), deps, "skill", skillID, "skill_deleted", map[string]any{"reason": req.OperatorReason})
			writeJSON(w, http.StatusAccepted, dto.AcceptedResponse{Accepted: true, Message: "skill deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func skillEnableHandler(deps Dependencies, skillID string, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		updated, state, err := deps.Skills.SetEnabled(strings.TrimSpace(skillID), enabled, req.OperatorReason)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "skill", skillID, map[bool]string{true: "skill_enabled", false: "skill_disabled"}[enabled], map[string]any{"reason": req.OperatorReason, "status": state.Status})
		writeJSON(w, http.StatusOK, skillManifestToDTO(updated, deps))
	}
}

func skillPromoteHandler(deps Dependencies, skillID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			OperatorReason string `json:"operator_reason"`
			ReviewState    string `json:"review_state"`
			RuntimeMode    string `json:"runtime_mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		updated, state, err := deps.Skills.Promote(strings.TrimSpace(skillID), skills.PromoteOptions{OperatorReason: req.OperatorReason, ReviewState: req.ReviewState, RuntimeMode: req.RuntimeMode})
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "skill", skillID, "skill_promoted", map[string]any{"reason": req.OperatorReason, "review_state": state.ReviewState, "runtime_mode": state.RuntimeMode})
		writeJSON(w, http.StatusOK, skillManifestToDTO(updated, deps))
	}
}

func skillRollbackHandler(deps Dependencies, skillID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
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
		updated, _, err := deps.Skills.Rollback(strings.TrimSpace(skillID), skills.RollbackOptions{OperatorReason: req.OperatorReason})
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "skill", skillID, "skill_rolled_back", map[string]any{"reason": req.OperatorReason})
		writeJSON(w, http.StatusOK, skillManifestToDTO(updated, deps))
	}
}

func skillExportHandler(deps Dependencies, skillID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		entry, ok := deps.Skills.Get(strings.TrimSpace(skillID))
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "skill not found")
			return
		}
		format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
		if format == "" {
			format = "zip"
		}
		var (
			payload     []byte
			err         error
			contentType string
			filename    string
		)
		switch format {
		case "zip":
			buf := new(bytes.Buffer)
			zw := zip.NewWriter(buf)
			id := strings.TrimSpace(entry.Metadata.ID)
			name := firstNonEmpty(entry.Metadata.DisplayName, entry.Metadata.Name, id)
			// Format SKILL.md per user request
			var md strings.Builder
			md.WriteString("---\n")
			md.WriteString(fmt.Sprintf("name: %s\n", name))
			md.WriteString(fmt.Sprintf("description: %s\n", entry.Metadata.Description))
			md.WriteString("---\n\n")
			md.WriteString(entry.Metadata.Content)
			f, err := zw.Create(fmt.Sprintf("%s/SKILL.md", id))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "zip_error", err.Error())
				return
			}
			_, _ = f.Write([]byte(md.String()))
			_ = zw.Close()
			payload = buf.Bytes()
			contentType = "application/zip"
			filename = fmt.Sprintf("%s.zip", sanitizeConnectorFilename(id))
		case "md":
			id := strings.TrimSpace(entry.Metadata.ID)
			name := firstNonEmpty(entry.Metadata.DisplayName, entry.Metadata.Name, id)
			var md strings.Builder
			md.WriteString("---\n")
			md.WriteString(fmt.Sprintf("name: %s\n", name))
			md.WriteString(fmt.Sprintf("description: %s\n", entry.Metadata.Description))
			md.WriteString("---\n\n")
			md.WriteString(entry.Metadata.Content)
			payload = []byte(md.String())
			contentType = "text/markdown"
			filename = "SKILL.md"
		case "yaml", "yml":
			public := skillManifestToDTO(entry, deps)
			payload, err = yaml.Marshal(public)
			contentType = "application/yaml"
			filename = fmt.Sprintf("%s.yaml", sanitizeConnectorFilename(entry.Metadata.ID))
		case "json":
			public := skillManifestToDTO(entry, deps)
			payload, err = json.MarshalIndent(public, "", "  ")
			contentType = "application/json"
			filename = fmt.Sprintf("%s.json", sanitizeConnectorFilename(entry.Metadata.ID))
		default:
			writeValidationError(w, "format must be zip, md, yaml or json")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "skill", skillID, "skill_exported", map[string]any{"format": format})
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}
}

func skillsImportHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req struct {
			Manifest       dto.SkillManifest `json:"manifest"`
			OperatorReason string            `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		manifest, err := skillManifestFromDTO(req.Manifest)
		if err != nil {
			writeValidationError(w, err.Error())
			return
		}
		updated, state, err := deps.Skills.Upsert(skills.UpsertOptions{Manifest: manifest, Reason: req.OperatorReason, Action: "skill_imported", Source: firstNonEmpty(manifest.Metadata.Source, "imported"), Status: "draft"})
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_skill", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "skill", updated.Metadata.ID, "skill_imported", map[string]any{"reason": req.OperatorReason})
		writeJSON(w, http.StatusOK, dto.SkillImportResponse{Manifest: skillManifestToDTO(updated, deps), State: skillLifecycleDTO(state)})
	}
}

type skillsFilter struct {
	Status    string
	Source    string
	Enabled   *bool
	Query     string
	SortBy    string
	SortOrder string
}

func skillsSnapshotEntries(deps Dependencies) []skills.Manifest {
	if deps.Skills == nil {
		return nil
	}
	snapshot := deps.Skills.Snapshot()
	if len(snapshot.Config.Entries) == 0 {
		return nil
	}
	items := make([]skills.Manifest, len(snapshot.Config.Entries))
	copy(items, snapshot.Config.Entries)
	return items
}

func listSkills(items []skills.Manifest, filter skillsFilter, deps Dependencies) []skills.Manifest {
	filtered := make([]skills.Manifest, 0, len(items))
	queryNeedle := strings.ToLower(strings.TrimSpace(filter.Query))
	statusNeedle := strings.ToLower(strings.TrimSpace(filter.Status))
	sourceNeedle := strings.ToLower(strings.TrimSpace(filter.Source))
	snapshot := deps.Skills.Snapshot()
	for _, item := range items {
		state := snapshot.Lifecycle[item.Metadata.ID]
		if statusNeedle != "" && strings.ToLower(state.Status) != statusNeedle {
			continue
		}
		if sourceNeedle != "" && strings.ToLower(firstNonEmpty(state.Source, item.Metadata.Source, item.Marketplace.Source)) != sourceNeedle {
			continue
		}
		if filter.Enabled != nil && item.Enabled() != *filter.Enabled {
			continue
		}
		if queryNeedle != "" && !skillMatchesQuery(item, queryNeedle) {
			continue
		}
		filtered = append(filtered, item)
	}
	sortSkills(filtered, snapshot.Lifecycle, filter.SortBy, filter.SortOrder)
	return filtered
}

func skillMatchesQuery(item skills.Manifest, query string) bool {
	haystacks := []string{
		strings.ToLower(item.Metadata.ID),
		strings.ToLower(item.Metadata.Name),
		strings.ToLower(item.Metadata.DisplayName),
		strings.ToLower(item.Metadata.Description),
	}
	haystacks = append(haystacks, item.Metadata.Tags...)
	for _, haystack := range haystacks {
		if strings.Contains(haystack, query) {
			return true
		}
	}
	return false
}

func sortSkills(items []skills.Manifest, states map[string]skills.LifecycleState, sortBy string, sortOrder string) {
	desc := strings.EqualFold(sortOrder, "desc")
	keyFor := func(item skills.Manifest) string {
		state := states[item.Metadata.ID]
		switch sortBy {
		case "id":
			return strings.ToLower(item.Metadata.ID)
		case "status":
			return strings.ToLower(state.Status)
		case "source":
			return strings.ToLower(firstNonEmpty(state.Source, item.Metadata.Source))
		default:
			return strings.ToLower(firstNonEmpty(item.Metadata.DisplayName, item.Metadata.Name, item.Metadata.ID))
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := keyFor(items[i])
		right := keyFor(items[j])
		if left == right {
			if desc {
				return items[i].Metadata.ID > items[j].Metadata.ID
			}
			return items[i].Metadata.ID < items[j].Metadata.ID
		}
		if desc {
			return left > right
		}
		return left < right
	})
}

func skillManifestToDTO(entry skills.Manifest, deps Dependencies) dto.SkillManifest {
	var lifecycle *dto.SkillLifecycle
	if deps.Skills != nil {
		if state, ok := deps.Skills.GetLifecycle(entry.Metadata.ID); ok {
			converted := skillLifecycleDTO(state)
			lifecycle = &converted
		}
	}
	return dto.SkillManifest{
		APIVersion: entry.APIVersion,
		Kind:       entry.Kind,
		Enabled:    entry.Enabled(),
		Metadata: dto.SkillMetadata{
			ID:          entry.Metadata.ID,
			Name:        entry.Metadata.Name,
			DisplayName: entry.Metadata.DisplayName,
			Version:     entry.Metadata.Version,
			Tags:        append([]string(nil), entry.Metadata.Tags...),
			Vendor:      entry.Metadata.Vendor,
			Description: entry.Metadata.Description,
			Source:      entry.Metadata.Source,
			Content:     entry.Metadata.Content,
			OrgID:       ownershipValue(entry.Metadata.OrgID, defaultAffiliation(deps.Org).OrgID),
			TenantID:    ownershipValue(entry.Metadata.TenantID, defaultAffiliation(deps.Org).TenantID),
			WorkspaceID: ownershipValue(entry.Metadata.WorkspaceID, defaultAffiliation(deps.Org).WorkspaceID),
		},
		Spec: dto.SkillSpec{
			Governance: dto.SkillGovernance{
				ExecutionPolicy: entry.Spec.Governance.ExecutionPolicy,
				ReadOnlyFirst:   entry.Spec.Governance.ReadOnlyFirst,
				ConnectorPreference: dto.SkillConnectorPreference{
					Metrics:       append([]string(nil), entry.Spec.Governance.ConnectorPreference.Metrics...),
					Execution:     append([]string(nil), entry.Spec.Governance.ConnectorPreference.Execution...),
					Observability: append([]string(nil), entry.Spec.Governance.ConnectorPreference.Observability...),
					Delivery:      append([]string(nil), entry.Spec.Governance.ConnectorPreference.Delivery...),
				},
			},
		},
		Compatibility: map[string]interface{}{
			"compatible":         entry.Compatibility.Compatible,
			"current_tars_major": entry.Compatibility.CurrentTARSMajor,
			"reasons":            entry.Compatibility.Reasons,
			"checked_at":         entry.Compatibility.CheckedAt,
		},
		Lifecycle: lifecycle,
	}
}

func skillLifecycleDTO(state skills.LifecycleState) dto.SkillLifecycle {
	out := dto.SkillLifecycle{
		SkillID:     state.SkillID,
		DisplayName: state.DisplayName,
		Source:      state.Source,
		Status:      state.Status,
		ReviewState: state.ReviewState,
		RuntimeMode: state.RuntimeMode,
		Enabled:     state.Enabled,
		InstalledAt: state.InstalledAt,
		UpdatedAt:   state.UpdatedAt,
		PublishedAt: state.PublishedAt,
		History:     make([]dto.SkillLifecycleEvent, 0, len(state.History)),
		Revisions:   make([]dto.SkillRevision, 0, len(state.Revisions)),
	}
	for _, item := range state.History {
		out.History = append(out.History, dto.SkillLifecycleEvent{Type: item.Type, Summary: item.Summary, Metadata: item.Metadata, CreatedAt: item.CreatedAt})
	}
	for _, item := range state.Revisions {
		out.Revisions = append(out.Revisions, dto.SkillRevision{CreatedAt: item.CreatedAt, Reason: item.Reason, Action: item.Action})
	}
	return out
}

func skillManifestFromDTO(input dto.SkillManifest) (skills.Manifest, error) {
	manifest := skills.Manifest{
		APIVersion: input.APIVersion,
		Kind:       input.Kind,
		Disabled:   !input.Enabled,
		Metadata: skills.Metadata{
			ID:          input.Metadata.ID,
			Name:        input.Metadata.Name,
			DisplayName: input.Metadata.DisplayName,
			Version:     input.Metadata.Version,
			Tags:        append([]string(nil), input.Metadata.Tags...),
			Vendor:      input.Metadata.Vendor,
			Description: input.Metadata.Description,
			Source:      input.Metadata.Source,
			Content:     input.Metadata.Content,
			OrgID:       input.Metadata.OrgID,
			TenantID:    input.Metadata.TenantID,
			WorkspaceID: input.Metadata.WorkspaceID,
		},
		Spec: skills.Spec{
			Governance: skills.Governance{
				ExecutionPolicy: input.Spec.Governance.ExecutionPolicy,
				ReadOnlyFirst:   input.Spec.Governance.ReadOnlyFirst,
				ConnectorPreference: skills.ConnectorPreference{
					Metrics:       append([]string(nil), input.Spec.Governance.ConnectorPreference.Metrics...),
					Execution:     append([]string(nil), input.Spec.Governance.ConnectorPreference.Execution...),
					Observability: append([]string(nil), input.Spec.Governance.ConnectorPreference.Observability...),
					Delivery:      append([]string(nil), input.Spec.Governance.ConnectorPreference.Delivery...),
				},
			},
		},
	}
	return manifest, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
