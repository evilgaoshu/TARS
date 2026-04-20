package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/modules/reasoning"
)

func providersRegistryHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Providers == nil {
			writeError(w, http.StatusConflict, "not_configured", "providers manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "providers.read", "providers.write"))
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			snapshot := deps.Providers.Snapshot()
			items := filterProviders(snapshot.Config.Entries, query.Query)
			pageItems, meta := paginateItems(items, query)
			resp := dto.ProviderRegistryListResponse{Items: make([]dto.ProviderRegistryEntry, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				resp.Items = append(resp.Items, providerRegistryEntryToDTO(item, snapshot.Config, deps.Org))
			}
			auditOpsRead(r.Context(), deps, "providers", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			var req struct {
				Provider       dto.ProviderRegistryEntry `json:"provider"`
				OperatorReason string                    `json:"operator_reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " created provider"
			}
			updated, err := upsertProviderEntry(deps.Providers, providerRegistryEntryFromDTO(req.Provider, deps.Org))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "provider", updated.ID, "provider_created", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusCreated, providerRegistryEntryToDTO(updated, deps.Providers.Snapshot().Config, deps.Org))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func providerDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Providers == nil {
			writeError(w, http.StatusConflict, "not_configured", "providers manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "providers.read", "providers.write"))
		if !ok {
			return
		}
		providerID, action := nestedResourcePath(r.URL.Path, "/api/v1/providers/")
		if providerID == "" {
			writeError(w, http.StatusNotFound, "not_found", "provider not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			entry, cfg, found := getProviderByID(deps.Providers, providerID)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "provider not found")
				return
			}
			auditOpsRead(r.Context(), deps, "provider", providerID, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, providerRegistryEntryToDTO(entry, cfg, deps.Org))
		case r.Method == http.MethodPut && action == "":
			var req struct {
				Provider       dto.ProviderRegistryEntry `json:"provider"`
				OperatorReason string                    `json:"operator_reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " updated provider"
			}
			entry := providerRegistryEntryFromDTO(req.Provider, deps.Org)
			entry.ID = providerID
			updated, err := upsertProviderEntry(deps.Providers, entry)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "provider", providerID, "provider_updated", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, providerRegistryEntryToDTO(updated, deps.Providers.Snapshot().Config, deps.Org))
		case r.Method == http.MethodPost && (action == "enable" || action == "disable"):
			var req operatorReasonRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " " + action + "d provider"
			}
			entry, _, found := getProviderByID(deps.Providers, providerID)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "provider not found")
				return
			}
			entry.Enabled = action == "enable"
			updated, err := upsertProviderEntry(deps.Providers, entry)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "provider", providerID, "provider_"+action+"d", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, providerRegistryEntryToDTO(updated, deps.Providers.Snapshot().Config, deps.Org))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func providerBindingsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Providers == nil {
			writeError(w, http.StatusConflict, "not_configured", "providers manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "providers.read", "providers.write"))
		if !ok {
			return
		}
		snapshot := deps.Providers.Snapshot()
		switch r.Method {
		case http.MethodGet:
			auditOpsRead(r.Context(), deps, "provider_bindings", "", "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, providerBindingsResponse(snapshot))
		case http.MethodPut:
			var req providerBindingsUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if req.Bindings == nil {
				writeValidationError(w, "bindings is required")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " updated provider bindings"
			}
			next := snapshot.Config
			next.Primary = reasoning.ProviderBinding{ProviderID: strings.TrimSpace(req.Bindings.Primary.ProviderID), Model: strings.TrimSpace(req.Bindings.Primary.Model)}
			next.Assist = reasoning.ProviderBinding{ProviderID: strings.TrimSpace(req.Bindings.Assist.ProviderID), Model: strings.TrimSpace(req.Bindings.Assist.Model)}
			if err := deps.Providers.SaveConfig(next); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			updated := deps.Providers.Snapshot()
			auditOpsWrite(r.Context(), deps, "provider_bindings", "", "provider_bindings_updated", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, providerBindingsResponse(updated))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}
