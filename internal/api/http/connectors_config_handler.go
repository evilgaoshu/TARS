package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/modules/connectors"
)

type connectorsConfigUpdateRequest struct {
	Content        string                `json:"content"`
	Config         *dto.ConnectorsConfig `json:"config"`
	OperatorReason string                `json:"operator_reason"`
}

type connectorsImportRequest struct {
	Manifest       dto.ConnectorManifest `json:"manifest"`
	OperatorReason string                `json:"operator_reason"`
}

func connectorsConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleConnectorsConfigGet(w, r, deps)
		case http.MethodPut:
			handleConnectorsConfigPut(w, r, deps)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func connectorsImportHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		handleConnectorsImportPost(w, r, deps)
	}
}

func handleConnectorsConfigGet(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Connectors == nil {
		writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
		return
	}
	snapshot := deps.Connectors.Snapshot()
	auditOpsRead(r.Context(), deps, "connectors_config", fallbackString(snapshot.Path, "runtime"), "get", map[string]any{
		"configured": strings.TrimSpace(snapshot.Path) != "",
		"loaded":     snapshot.Loaded,
		"entries":    len(snapshot.Config.Entries),
	})
	writeJSON(w, http.StatusOK, connectorsConfigResponse(snapshot))
}

func handleConnectorsConfigPut(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Connectors == nil {
		writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
		return
	}
	var req connectorsConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
		return
	}
	switch {
	case strings.TrimSpace(req.Content) != "":
		if err := deps.Connectors.Save(req.Content); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	case req.Config != nil:
		cfg, err := connectorsConfigFromDTO(*req.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		if err := deps.Connectors.SaveConfig(cfg); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", "content or config is required")
		return
	}
	snapshot := deps.Connectors.Snapshot()
	metadata := map[string]any{
		"loaded":  snapshot.Loaded,
		"entries": len(snapshot.Config.Entries),
	}
	if req.Config != nil {
		metadata["connector_ids"] = connectorIDsForAudit(req.Config.Entries)
		metadata["secret_refs"] = connectorSecretRefsForAudit(req.Config.Entries)
	}
	logConfigUpdate(r, deps, "connectors_config", fallbackString(snapshot.Path, "runtime"), req.OperatorReason, metadata)
	writeJSON(w, http.StatusOK, connectorsConfigResponse(snapshot))
}

func handleConnectorsImportPost(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Connectors == nil {
		writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
		return
	}
	var req connectorsImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
		return
	}
	manifest, err := connectorManifestFromDTO(req.Manifest)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if !connectors.SupportsCurrentTARSMajor(manifest) {
		writeError(w, http.StatusBadRequest, "validation_failed", "connector is not compatible with current TARS major version")
		return
	}
	if err := deps.Connectors.Upsert(manifest); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	snapshot := deps.Connectors.Snapshot()
	logConfigUpdate(r, deps, "connectors_config", manifest.Metadata.ID, req.OperatorReason, map[string]any{
		"operation": "import",
		"entries":   len(snapshot.Config.Entries),
		"type":      manifest.Spec.Type,
		"protocol":  manifest.Spec.Protocol,
	})
	writeJSON(w, http.StatusOK, connectorsConfigResponse(snapshot))
}

func connectorsConfigResponse(snapshot connectors.Snapshot) dto.ConnectorsConfigResponse {
	return dto.ConnectorsConfigResponse{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
		Content:    snapshot.Content,
		Config:     connectorsConfigToDTO(snapshot.Config, snapshot.Lifecycle),
	}
}

func connectorsConfigToDTO(cfg connectors.Config, lifecycle map[string]connectors.LifecycleState) dto.ConnectorsConfig {
	out := dto.ConnectorsConfig{
		Entries: make([]dto.ConnectorManifest, 0, len(cfg.Entries)),
	}
	for _, entry := range cfg.Entries {
		out.Entries = append(out.Entries, connectorManifestToDTOWithLifecycle(entry, true, lifecycle[entry.Metadata.ID]))
	}
	return out
}

func connectorsConfigFromDTO(cfg dto.ConnectorsConfig) (connectors.Config, error) {
	out := connectors.Config{
		Entries: make([]connectors.Manifest, 0, len(cfg.Entries)),
	}
	for _, entry := range cfg.Entries {
		manifest, err := connectorManifestFromDTO(entry)
		if err != nil {
			return connectors.Config{}, err
		}
		out.Entries = append(out.Entries, manifest)
	}
	return out, nil
}

func connectorManifestToDTO(entry connectors.Manifest, includeSensitiveConfig bool, manager *connectors.Manager) dto.ConnectorManifest {
	enabled := entry.Enabled()
	lifecycle := connectorLifecycleToDTO(entry)
	if manager != nil {
		if state, ok := manager.GetLifecycle(entry.Metadata.ID); ok {
			lifecycle = &dto.ConnectorLifecycle{
				ConnectorID:      state.ConnectorID,
				DisplayName:      state.DisplayName,
				CurrentVersion:   state.CurrentVersion,
				AvailableVersion: state.AvailableVersion,
				Enabled:          state.Enabled,
				InstalledAt:      state.InstalledAt,
				UpdatedAt:        state.UpdatedAt,
				Runtime: dto.ConnectorRuntimeMetadata{
					Type:     state.Runtime.Type,
					Protocol: state.Runtime.Protocol,
					Vendor:   state.Runtime.Vendor,
					Mode:     state.Runtime.Mode,
					State:    connectorRuntimeState(state.Runtime.Protocol),
				},
				Compatibility: dto.ConnectorCompatibilityReport{
					Compatible:       state.Compatibility.Compatible,
					CurrentTARSMajor: state.Compatibility.CurrentTARSMajor,
					Reasons:          cloneStrings(state.Compatibility.Reasons),
					CheckedAt:        state.Compatibility.CheckedAt,
				},
				Health: dto.ConnectorHealthStatus{
					Status:           state.Health.Status,
					CredentialStatus: state.Health.CredentialStatus,
					Summary:          state.Health.Summary,
					CheckedAt:        state.Health.CheckedAt,
					RuntimeState:     connectorRuntimeState(state.Runtime.Protocol),
				},
				History:       connectorLifecycleEventsToDTO(state.History),
				HealthHistory: connectorHealthHistoryToDTO(state.HealthHistory),
				Revisions:     connectorRevisionsToDTO(state.Revisions),
			}
		}
	}
	return dto.ConnectorManifest{
		APIVersion: entry.APIVersion,
		Kind:       entry.Kind,
		Enabled:    &enabled,
		Metadata: dto.ConnectorMetadata{
			ID:          entry.Metadata.ID,
			Name:        entry.Metadata.Name,
			DisplayName: entry.Metadata.DisplayName,
			Vendor:      entry.Metadata.Vendor,
			Version:     entry.Metadata.Version,
			Description: entry.Metadata.Description,
			OrgID:       entry.Metadata.OrgID,
			TenantID:    entry.Metadata.TenantID,
			WorkspaceID: entry.Metadata.WorkspaceID,
		},
		Spec: dto.ConnectorSpec{
			Type:     entry.Spec.Type,
			Protocol: entry.Spec.Protocol,
			Capabilities: func() []dto.ConnectorCapability {
				items := make([]dto.ConnectorCapability, 0, len(entry.Spec.Capabilities))
				for _, capability := range entry.Spec.Capabilities {
					items = append(items, dto.ConnectorCapability{
						ID:          capability.ID,
						Action:      capability.Action,
						ReadOnly:    capability.ReadOnly,
						Invocable:   capability.Invocable,
						Scopes:      cloneStrings(capability.Scopes),
						Description: capability.Description,
					})
				}
				return items
			}(),
			ConnectionForm: func() []dto.ConnectorField {
				items := make([]dto.ConnectorField, 0, len(entry.Spec.ConnectionForm))
				for _, field := range entry.Spec.ConnectionForm {
					items = append(items, dto.ConnectorField{
						Key:         field.Key,
						Label:       field.Label,
						Type:        field.Type,
						Required:    field.Required,
						Secret:      field.Secret,
						Default:     field.Default,
						Options:     cloneStrings(field.Options),
						Description: field.Description,
					})
				}
				return items
			}(),
			ImportExport: dto.ConnectorImportExport{
				Exportable: entry.Spec.ImportExport.Exportable,
				Importable: entry.Spec.ImportExport.Importable,
				Formats:    cloneStrings(entry.Spec.ImportExport.Formats),
			},
		},
		Config: dto.ConnectorRuntimeConfig{
			Values:     connectorConfigValuesForDTO(entry, includeSensitiveConfig),
			SecretRefs: cloneConnectorStringMap(entry.Config.SecretRefs),
		},
		Compatibility: dto.ConnectorCompatibility{
			TARSMajorVersions:     cloneStrings(entry.Compatibility.TARSMajorVersions),
			UpstreamMajorVersions: cloneStrings(entry.Compatibility.UpstreamMajorVersions),
			Modes:                 cloneStrings(entry.Compatibility.Modes),
		},
		Marketplace: dto.ConnectorMarketplace{
			Category: entry.Marketplace.Category,
			Tags:     cloneStrings(entry.Marketplace.Tags),
			Source:   entry.Marketplace.Source,
		},
		Lifecycle: lifecycle,
	}
}

func connectorRuntimeState(protocol string) string {
	if strings.EqualFold(strings.TrimSpace(protocol), "stub") {
		return "stub"
	}
	if strings.TrimSpace(protocol) == "" {
		return ""
	}
	return "real"
}

func connectorManifestToDTOWithLifecycle(entry connectors.Manifest, includeSensitiveConfig bool, state connectors.LifecycleState) dto.ConnectorManifest {
	enabled := entry.Enabled()
	lifecycle := connectorLifecycleToDTO(entry)
	if strings.TrimSpace(state.ConnectorID) != "" {
		lifecycle = lifecycleStateToDTO(state)
	}
	return dto.ConnectorManifest{
		APIVersion: entry.APIVersion,
		Kind:       entry.Kind,
		Enabled:    &enabled,
		Metadata: dto.ConnectorMetadata{
			ID:          entry.Metadata.ID,
			Name:        entry.Metadata.Name,
			DisplayName: entry.Metadata.DisplayName,
			Vendor:      entry.Metadata.Vendor,
			Version:     entry.Metadata.Version,
			Description: entry.Metadata.Description,
			OrgID:       entry.Metadata.OrgID,
			TenantID:    entry.Metadata.TenantID,
			WorkspaceID: entry.Metadata.WorkspaceID,
		},
		Spec: dto.ConnectorSpec{
			Type:     entry.Spec.Type,
			Protocol: entry.Spec.Protocol,
			Capabilities: func() []dto.ConnectorCapability {
				items := make([]dto.ConnectorCapability, 0, len(entry.Spec.Capabilities))
				for _, capability := range entry.Spec.Capabilities {
					items = append(items, dto.ConnectorCapability{
						ID:          capability.ID,
						Action:      capability.Action,
						ReadOnly:    capability.ReadOnly,
						Invocable:   capability.Invocable,
						Scopes:      cloneStrings(capability.Scopes),
						Description: capability.Description,
					})
				}
				return items
			}(),
			ConnectionForm: func() []dto.ConnectorField {
				items := make([]dto.ConnectorField, 0, len(entry.Spec.ConnectionForm))
				for _, field := range entry.Spec.ConnectionForm {
					items = append(items, dto.ConnectorField{
						Key:         field.Key,
						Label:       field.Label,
						Type:        field.Type,
						Required:    field.Required,
						Secret:      field.Secret,
						Default:     field.Default,
						Options:     cloneStrings(field.Options),
						Description: field.Description,
					})
				}
				return items
			}(),
			ImportExport: dto.ConnectorImportExport{
				Exportable: entry.Spec.ImportExport.Exportable,
				Importable: entry.Spec.ImportExport.Importable,
				Formats:    cloneStrings(entry.Spec.ImportExport.Formats),
			},
		},
		Config: dto.ConnectorRuntimeConfig{
			Values:     connectorConfigValuesForDTO(entry, includeSensitiveConfig),
			SecretRefs: cloneConnectorStringMap(entry.Config.SecretRefs),
		},
		Compatibility: dto.ConnectorCompatibility{
			TARSMajorVersions:     cloneStrings(entry.Compatibility.TARSMajorVersions),
			UpstreamMajorVersions: cloneStrings(entry.Compatibility.UpstreamMajorVersions),
			Modes:                 cloneStrings(entry.Compatibility.Modes),
		},
		Marketplace: dto.ConnectorMarketplace{
			Category: entry.Marketplace.Category,
			Tags:     cloneStrings(entry.Marketplace.Tags),
			Source:   entry.Marketplace.Source,
		},
		Lifecycle: lifecycle,
	}
}

func lifecycleStateToDTO(state connectors.LifecycleState) *dto.ConnectorLifecycle {
	return &dto.ConnectorLifecycle{
		ConnectorID:      state.ConnectorID,
		DisplayName:      state.DisplayName,
		CurrentVersion:   state.CurrentVersion,
		AvailableVersion: state.AvailableVersion,
		Enabled:          state.Enabled,
		InstalledAt:      state.InstalledAt,
		UpdatedAt:        state.UpdatedAt,
		Runtime: dto.ConnectorRuntimeMetadata{
			Type:     state.Runtime.Type,
			Protocol: state.Runtime.Protocol,
			Vendor:   state.Runtime.Vendor,
			Mode:     state.Runtime.Mode,
			State:    connectorRuntimeState(state.Runtime.Protocol),
		},
		Compatibility: dto.ConnectorCompatibilityReport{
			Compatible:       state.Compatibility.Compatible,
			CurrentTARSMajor: state.Compatibility.CurrentTARSMajor,
			Reasons:          cloneStrings(state.Compatibility.Reasons),
			CheckedAt:        state.Compatibility.CheckedAt,
		},
		Health: dto.ConnectorHealthStatus{
			Status:           state.Health.Status,
			CredentialStatus: state.Health.CredentialStatus,
			Summary:          state.Health.Summary,
			CheckedAt:        state.Health.CheckedAt,
			RuntimeState:     connectorRuntimeState(state.Runtime.Protocol),
		},
		History:       connectorLifecycleEventsToDTO(state.History),
		HealthHistory: connectorHealthHistoryToDTO(state.HealthHistory),
		Revisions:     connectorRevisionsToDTO(state.Revisions),
	}
}

func connectorManifestFromDTO(entry dto.ConnectorManifest) (connectors.Manifest, error) {
	manifest := connectors.Manifest{
		APIVersion: entry.APIVersion,
		Kind:       entry.Kind,
		Disabled: func() bool {
			if entry.Enabled == nil {
				return false
			}
			return !*entry.Enabled
		}(),
		Metadata: connectors.Metadata{
			ID:          strings.TrimSpace(entry.Metadata.ID),
			Name:        strings.TrimSpace(entry.Metadata.Name),
			DisplayName: strings.TrimSpace(entry.Metadata.DisplayName),
			Vendor:      strings.TrimSpace(entry.Metadata.Vendor),
			Version:     strings.TrimSpace(entry.Metadata.Version),
			Description: strings.TrimSpace(entry.Metadata.Description),
			OrgID:       strings.TrimSpace(entry.Metadata.OrgID),
			TenantID:    strings.TrimSpace(entry.Metadata.TenantID),
			WorkspaceID: strings.TrimSpace(entry.Metadata.WorkspaceID),
		},
		Spec: connectors.Spec{
			Type:     strings.TrimSpace(entry.Spec.Type),
			Protocol: strings.TrimSpace(entry.Spec.Protocol),
			Capabilities: func() []connectors.Capability {
				items := make([]connectors.Capability, 0, len(entry.Spec.Capabilities))
				for _, capability := range entry.Spec.Capabilities {
					items = append(items, connectors.Capability{
						ID:          strings.TrimSpace(capability.ID),
						Action:      strings.TrimSpace(capability.Action),
						ReadOnly:    capability.ReadOnly,
						Invocable:   capability.Invocable,
						Scopes:      cloneStrings(capability.Scopes),
						Description: strings.TrimSpace(capability.Description),
					})
				}
				return items
			}(),
			ConnectionForm: func() []connectors.Field {
				items := make([]connectors.Field, 0, len(entry.Spec.ConnectionForm))
				for _, field := range entry.Spec.ConnectionForm {
					items = append(items, connectors.Field{
						Key:         strings.TrimSpace(field.Key),
						Label:       strings.TrimSpace(field.Label),
						Type:        strings.TrimSpace(field.Type),
						Required:    field.Required,
						Secret:      field.Secret,
						Default:     strings.TrimSpace(field.Default),
						Options:     cloneStrings(field.Options),
						Description: strings.TrimSpace(field.Description),
					})
				}
				return items
			}(),
			ImportExport: connectors.ImportExport{
				Exportable: entry.Spec.ImportExport.Exportable,
				Importable: entry.Spec.ImportExport.Importable,
				Formats:    cloneStrings(entry.Spec.ImportExport.Formats),
			},
		},
		Config: connectors.RuntimeConfig{
			Values:     cloneConnectorStringMap(entry.Config.Values),
			SecretRefs: cloneConnectorStringMap(entry.Config.SecretRefs),
		},
		Compatibility: connectors.Compatibility{
			TARSMajorVersions:     cloneStrings(entry.Compatibility.TARSMajorVersions),
			UpstreamMajorVersions: cloneStrings(entry.Compatibility.UpstreamMajorVersions),
			Modes:                 cloneStrings(entry.Compatibility.Modes),
		},
		Marketplace: connectors.MarketplaceMetadata{
			Category: strings.TrimSpace(entry.Marketplace.Category),
			Tags:     cloneStrings(entry.Marketplace.Tags),
			Source:   strings.TrimSpace(entry.Marketplace.Source),
		},
	}
	if err := connectors.ValidateManifest(manifest); err != nil {
		return connectors.Manifest{}, err
	}
	return manifest, nil
}

func connectorConfigValuesForDTO(entry connectors.Manifest, includeSensitive bool) map[string]string {
	if !includeSensitive {
		return nil
	}
	if len(entry.Config.Values) == 0 {
		return nil
	}
	values := make(map[string]string, 0)
	for key, value := range entry.Config.Values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		values[trimmedKey] = strings.TrimSpace(value)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func connectorPublicManifest(entry connectors.Manifest) connectors.Manifest {
	cloned := entry
	cloned.Config.Values = nil
	return cloned
}

func connectorLifecycleToDTO(entry connectors.Manifest) *dto.ConnectorLifecycle {
	compatibility := connectors.CompatibilityReportForManifest(entry)
	compatibility.CheckedAt = time.Now().UTC()
	health := connectors.HealthStatusForManifest(entry, compatibility, compatibility.CheckedAt)
	return &dto.ConnectorLifecycle{
		ConnectorID:      entry.Metadata.ID,
		DisplayName:      fallbackString(entry.Metadata.DisplayName, fallbackString(entry.Metadata.Name, entry.Metadata.ID)),
		CurrentVersion:   entry.Metadata.Version,
		AvailableVersion: entry.Metadata.Version,
		Enabled:          entry.Enabled(),
		UpdatedAt:        compatibility.CheckedAt,
		Runtime: dto.ConnectorRuntimeMetadata{
			Type:     entry.Spec.Type,
			Protocol: entry.Spec.Protocol,
			Vendor:   entry.Metadata.Vendor,
			Mode:     fallbackString(connectors.RuntimeMetadataForManifest(entry).Mode, "managed"),
			State:    connectorRuntimeState(entry.Spec.Protocol),
		},
		Compatibility: dto.ConnectorCompatibilityReport{
			Compatible:       compatibility.Compatible,
			CurrentTARSMajor: compatibility.CurrentTARSMajor,
			Reasons:          cloneStrings(compatibility.Reasons),
			CheckedAt:        compatibility.CheckedAt,
		},
		Health: dto.ConnectorHealthStatus{
			Status:           health.Status,
			CredentialStatus: health.CredentialStatus,
			Summary:          health.Summary,
			CheckedAt:        health.CheckedAt,
			RuntimeState:     connectorRuntimeState(entry.Spec.Protocol),
		},
		Revisions: []dto.ConnectorRevision{{
			Version:   entry.Metadata.Version,
			CreatedAt: compatibility.CheckedAt,
			Reason:    "current",
		}},
	}
}

func connectorRevisionsToDTO(items []connectors.RevisionSnapshot) []dto.ConnectorRevision {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ConnectorRevision, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ConnectorRevision{
			Version:   item.Version,
			CreatedAt: item.CreatedAt,
			Reason:    item.Reason,
		})
	}
	return out
}

func cloneConnectorStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		out[trimmedKey] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func connectorIDsForAudit(items []dto.ConnectorManifest) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.Metadata.ID); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func connectorSecretRefsForAudit(items []dto.ConnectorManifest) []string {
	refs := make([]string, 0)
	for _, item := range items {
		for _, ref := range item.Config.SecretRefs {
			if trimmed := strings.TrimSpace(ref); trimmed != "" {
				refs = append(refs, trimmed)
			}
		}
	}
	sort.Strings(refs)
	return refs
}
