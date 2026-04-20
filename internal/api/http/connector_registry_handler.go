package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/foundation/secrets"
	"tars/internal/modules/connectors"

	"gopkg.in/yaml.v3"
)

func connectorsListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			items := listConnectors(connectorsSnapshotEntries(deps), connectorsFilter{
				Kind:      strings.TrimSpace(r.URL.Query().Get("kind")),
				Protocol:  strings.TrimSpace(r.URL.Query().Get("protocol")),
				Type:      strings.TrimSpace(r.URL.Query().Get("type")),
				Vendor:    strings.TrimSpace(r.URL.Query().Get("vendor")),
				Enabled:   parseOptionalBool(r.URL.Query().Get("enabled")),
				Query:     query.Query,
				SortBy:    query.SortBy,
				SortOrder: query.SortOrder,
			})
			pageItems, meta := paginateItems(items, query)
			response := dto.ConnectorListResponse{
				Items:    make([]dto.ConnectorManifest, 0, len(pageItems)),
				ListPage: meta,
			}
			for _, item := range pageItems {
				response.Items = append(response.Items, connectorManifestToDTO(item, false, deps.Connectors))
			}

			writeJSON(w, http.StatusOK, response)
		case http.MethodPost:
			connectorCreateHandler(deps)(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func connectorRouterHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/connectors/")
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "not_found", "connector not found")
			return
		}
		if strings.HasSuffix(path, "/export") {
			connectorExportHandler(deps, strings.TrimSuffix(path, "/export"))(w, r)
			return
		}
		if strings.HasSuffix(path, "/enable") {
			connectorEnableHandler(deps, strings.TrimSuffix(path, "/enable"), true)(w, r)
			return
		}
		if strings.HasSuffix(path, "/disable") {
			connectorEnableHandler(deps, strings.TrimSuffix(path, "/disable"), false)(w, r)
			return
		}
		if strings.HasSuffix(path, "/upgrade") {
			connectorUpgradeHandler(deps, strings.TrimSuffix(path, "/upgrade"))(w, r)
			return
		}
		if strings.HasSuffix(path, "/rollback") {
			connectorRollbackHandler(deps, strings.TrimSuffix(path, "/rollback"))(w, r)
			return
		}
		if strings.HasSuffix(path, "/metrics/query") {
			connectorMetricsQueryHandler(deps, strings.TrimSuffix(path, "/metrics/query"))(w, r)
			return
		}
		if strings.HasSuffix(path, "/execution/execute") {
			connectorExecutionHandler(deps, strings.TrimSuffix(path, "/execution/execute"))(w, r)
			return
		}
		if strings.HasSuffix(path, "/templates/apply") {
			connectorTemplateApplyHandler(deps)(w, r)
			return
		}
		if strings.HasSuffix(path, "/health") {
			connectorHealthHandler(deps, strings.TrimSuffix(path, "/health"))(w, r)
			return
		}
		if strings.HasSuffix(path, "/capabilities/invoke") {
			connectorInvokeCapabilityHandler(deps, strings.TrimSuffix(path, "/capabilities/invoke"))(w, r)
			return
		}
		connectorDetailHandler(deps, path)(w, r)
	}
}

func connectorDetailHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		connectorID = strings.TrimSpace(connectorID)
		if connectorID == "" {
			writeError(w, http.StatusNotFound, "not_found", "connector not found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			includeSensitiveConfig := false
			if _, ok := authenticatedPrincipal(deps, r); ok {
				includeSensitiveConfig = true
			}
			for _, entry := range connectorsSnapshotEntries(deps) {
				if entry.Metadata.ID == connectorID {
					writeJSON(w, http.StatusOK, connectorManifestToDTO(entry, includeSensitiveConfig, deps.Connectors))
					return
				}
			}
			writeError(w, http.StatusNotFound, "not_found", "connector not found")
		case http.MethodPut:
			connectorUpdateHandler(deps, connectorID)(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

type connectorsFilter struct {
	Kind      string
	Protocol  string
	Type      string
	Vendor    string
	Enabled   *bool
	Query     string
	SortBy    string
	SortOrder string
}

type connectorToggleRequest struct {
	OperatorReason string `json:"operator_reason"`
}

type connectorUpsertRequest struct {
	Manifest       dto.ConnectorManifest `json:"manifest"`
	OperatorReason string                `json:"operator_reason"`
}

type connectorProbeRequest struct {
	Manifest dto.ConnectorManifest `json:"manifest"`
}

type connectorUpgradeRequest struct {
	Manifest         dto.ConnectorManifest `json:"manifest"`
	OperatorReason   string                `json:"operator_reason"`
	AvailableVersion string                `json:"available_version,omitempty"`
}

type connectorRollbackRequest struct {
	TargetVersion  string `json:"target_version,omitempty"`
	OperatorReason string `json:"operator_reason"`
}

type connectorMetricsQueryRequest struct {
	Service string `json:"service"`
	Host    string `json:"host"`
	Query   string `json:"query,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Start   string `json:"start,omitempty"`
	End     string `json:"end,omitempty"`
	Step    string `json:"step,omitempty"`
	Window  string `json:"window,omitempty"`
}

type connectorExecutionRequest struct {
	SessionID      string `json:"session_id"`
	TargetHost     string `json:"target_host"`
	Command        string `json:"command"`
	Service        string `json:"service,omitempty"`
	OperatorReason string `json:"operator_reason"`
	ExecutionMode  string `json:"execution_mode,omitempty"`
}

func connectorCreateHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Connectors == nil {
			writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
			return
		}

		var req connectorUpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
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
		auditOpsWrite(r.Context(), deps, "connector", manifest.Metadata.ID, "connector_created", map[string]any{"reason": req.OperatorReason})
		entry, err := getConnectorEntry(deps, manifest.Metadata.ID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, connectorManifestToDTO(entry, true, deps.Connectors))
	}
}

func connectorUpdateHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Connectors == nil {
			writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
			return
		}

		var req connectorUpsertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		manifest, err := connectorManifestFromDTO(req.Manifest)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		if manifest.Metadata.ID != strings.TrimSpace(connectorID) {
			writeError(w, http.StatusBadRequest, "validation_failed", "manifest metadata.id must match connector id")
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
		auditOpsWrite(r.Context(), deps, "connector", manifest.Metadata.ID, "connector_updated", map[string]any{"reason": req.OperatorReason})
		entry, err := getConnectorEntry(deps, manifest.Metadata.ID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, connectorManifestToDTO(entry, true, deps.Connectors))
	}
}

func connectorProbeHandler(deps Dependencies) http.HandlerFunc {
	type manifestHealthChecker interface {
		CheckManifestHealth(ctx context.Context, manifest connectors.Manifest) (connectors.LifecycleState, error)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Action == nil {
			writeError(w, http.StatusConflict, "not_configured", "action service is not configured")
			return
		}

		checker, ok := deps.Action.(manifestHealthChecker)
		if !ok {
			writeError(w, http.StatusConflict, "not_configured", "draft connector health probe is not configured")
			return
		}

		var req connectorProbeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		manifest, err := connectorManifestFromDTO(req.Manifest)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}

		state, err := checker.CheckManifestHealth(r.Context(), manifest)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		auditOpsWrite(r.Context(), deps, "connector_runtime", manifest.Metadata.ID, "draft_health_probe", map[string]any{
			"status":     state.Health.Status,
			"summary":    state.Health.Summary,
			"protocol":   manifest.Spec.Protocol,
			"compatible": state.Compatibility.Compatible,
			"draft":      true,
		})
		writeJSON(w, http.StatusOK, lifecycleStateToDTO(state))
	}
}

func connectorsSnapshotEntries(deps Dependencies) []connectors.Manifest {
	if deps.Connectors == nil {
		return nil
	}
	snapshot := deps.Connectors.Snapshot()
	if len(snapshot.Config.Entries) == 0 {
		return nil
	}
	items := make([]connectors.Manifest, len(snapshot.Config.Entries))
	copy(items, snapshot.Config.Entries)
	return items
}

func listConnectors(items []connectors.Manifest, filter connectorsFilter) []connectors.Manifest {
	filtered := make([]connectors.Manifest, 0, len(items))
	queryNeedle := strings.ToLower(strings.TrimSpace(filter.Query))
	kindNeedle := strings.ToLower(filter.Kind)
	protocolNeedle := strings.ToLower(filter.Protocol)
	typeNeedle := strings.ToLower(filter.Type)
	vendorNeedle := strings.ToLower(filter.Vendor)

	for _, item := range items {
		if kindNeedle != "" && strings.ToLower(item.Kind) != kindNeedle {
			continue
		}
		if protocolNeedle != "" && strings.ToLower(item.Spec.Protocol) != protocolNeedle {
			continue
		}
		if typeNeedle != "" && strings.ToLower(item.Spec.Type) != typeNeedle {
			continue
		}
		if vendorNeedle != "" && strings.ToLower(item.Metadata.Vendor) != vendorNeedle {
			continue
		}
		if filter.Enabled != nil && item.Enabled() != *filter.Enabled {
			continue
		}
		if queryNeedle != "" && !connectorMatchesQuery(item, queryNeedle) {
			continue
		}
		filtered = append(filtered, item)
	}

	sortConnectors(filtered, filter.SortBy, filter.SortOrder)
	return filtered
}

func connectorMatchesQuery(item connectors.Manifest, query string) bool {
	haystacks := []string{
		item.APIVersion,
		item.Kind,
		item.Metadata.ID,
		item.Metadata.Name,
		item.Metadata.DisplayName,
		item.Metadata.Vendor,
		item.Metadata.Version,
		item.Metadata.Description,
		item.Spec.Type,
		item.Spec.Protocol,
		item.Marketplace.Category,
		item.Marketplace.Source,
	}
	for _, tag := range item.Marketplace.Tags {
		haystacks = append(haystacks, tag)
	}
	for _, capability := range item.Spec.Capabilities {
		haystacks = append(haystacks,
			capability.ID,
			capability.Action,
			capability.Description,
			strings.Join(capability.Scopes, " "),
		)
	}
	for _, field := range item.Spec.ConnectionForm {
		haystacks = append(haystacks,
			field.Key,
			field.Label,
			field.Type,
			field.Description,
			strings.Join(field.Options, " "),
		)
	}
	for _, haystack := range haystacks {
		if strings.Contains(strings.ToLower(haystack), query) {
			return true
		}
	}
	return false
}

func sortConnectors(items []connectors.Manifest, sortBy string, sortOrder string) {
	desc := strings.EqualFold(sortOrder, "desc")
	keyFor := func(item connectors.Manifest) string {
		switch sortBy {
		case "enabled":
			if item.Enabled() {
				return "1"
			}
			return "0"
		case "id":
			return strings.ToLower(item.Metadata.ID)
		case "name":
			return strings.ToLower(item.Metadata.Name)
		case "display_name":
			return strings.ToLower(item.Metadata.DisplayName)
		case "vendor":
			return strings.ToLower(item.Metadata.Vendor)
		case "kind":
			return strings.ToLower(item.Kind)
		case "version":
			return strings.ToLower(item.Metadata.Version)
		case "protocol":
			return strings.ToLower(item.Spec.Protocol)
		case "type":
			return strings.ToLower(item.Spec.Type)
		default:
			if displayName := strings.ToLower(item.Metadata.DisplayName); displayName != "" {
				return displayName
			}
			return strings.ToLower(item.Metadata.ID)
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		left := keyFor(items[i])
		right := keyFor(items[j])
		if left == right {
			leftID := strings.ToLower(items[i].Metadata.ID)
			rightID := strings.ToLower(items[j].Metadata.ID)
			if desc {
				return leftID > rightID
			}
			return leftID < rightID
		}
		if desc {
			return left > right
		}
		return left < right
	})
}

func connectorExportHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		entry, err := getConnectorEntry(deps, connectorID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		if !entry.Spec.ImportExport.Exportable {
			writeError(w, http.StatusConflict, "invalid_state", "connector export is disabled")
			return
		}

		format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
		if format == "" {
			format = "yaml"
		}

		var (
			payload     []byte
			contentType string
			filename    string
			marshalErr  error
		)
		exportable := connectorPublicManifest(entry)
		switch format {
		case "yaml", "yml":
			payload, marshalErr = yaml.Marshal(exportable)
			contentType = "application/yaml"
			filename = fmt.Sprintf("%s-%s.yaml", sanitizeConnectorFilename(entry.Metadata.ID), fallbackString(entry.Metadata.Version, "latest"))
		case "json":
			payload, marshalErr = json.MarshalIndent(exportable, "", "  ")
			contentType = "application/json"
			filename = fmt.Sprintf("%s-%s.json", sanitizeConnectorFilename(entry.Metadata.ID), fallbackString(entry.Metadata.Version, "latest"))
		default:
			writeValidationError(w, "format must be yaml or json")
			return
		}
		if marshalErr != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", marshalErr.Error())
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}
}

func connectorEnableHandler(deps Dependencies, connectorID string, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Connectors == nil {
			writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
			return
		}

		var req connectorToggleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}

		entry, err := deps.Connectors.SetEnabled(connectorID, enabled)
		if err != nil {
			writeConnectorError(w, err)
			return
		}

		action := "disable"
		if enabled {
			action = "enable"
		}
		auditOpsWrite(r.Context(), deps, "connector", entry.Metadata.ID, action, map[string]any{
			"operator_reason": req.OperatorReason,
			"enabled":         entry.Enabled(),
			"version":         entry.Metadata.Version,
			"type":            entry.Spec.Type,
			"protocol":        entry.Spec.Protocol,
		})
		writeJSON(w, http.StatusOK, connectorManifestToDTO(entry, true, deps.Connectors))
	}
}

func connectorUpgradeHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Connectors == nil {
			writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
			return
		}

		var req connectorUpgradeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		manifest, err := connectorManifestFromDTO(req.Manifest)
		if err != nil {
			writeValidationError(w, err.Error())
			return
		}
		if !connectors.SupportsCurrentTARSMajor(manifest) {
			writeValidationError(w, "connector is not compatible with current TARS major version")
			return
		}
		entry, state, err := deps.Connectors.Upgrade(connectorID, connectors.UpgradeOptions{
			Manifest:  manifest,
			Reason:    req.OperatorReason,
			Available: req.AvailableVersion,
		})
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		auditOpsWrite(r.Context(), deps, "connector", entry.Metadata.ID, "upgrade", map[string]any{
			"operator_reason":   req.OperatorReason,
			"version":           entry.Metadata.Version,
			"available_version": state.AvailableVersion,
		})
		writeJSON(w, http.StatusOK, connectorManifestToDTO(entry, true, deps.Connectors))
	}
}

func connectorRollbackHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Connectors == nil {
			writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
			return
		}

		var req connectorRollbackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		entry, _, err := deps.Connectors.Rollback(connectorID, connectors.RollbackOptions{
			TargetVersion: req.TargetVersion,
			Reason:        req.OperatorReason,
		})
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		auditOpsWrite(r.Context(), deps, "connector", entry.Metadata.ID, "rollback", map[string]any{
			"operator_reason": req.OperatorReason,
			"version":         entry.Metadata.Version,
			"target_version":  req.TargetVersion,
		})
		writeJSON(w, http.StatusOK, connectorManifestToDTO(entry, true, deps.Connectors))
	}
}

func connectorMetricsQueryHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		entry, err := getConnectorEntry(deps, connectorID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		if err := connectors.ValidateRuntimeManifest(entry, "metrics", entry.Spec.Protocol, map[string]struct{}{"prometheus_http": {}, "victoriametrics_http": {}}); err != nil {
			writeConnectorError(w, err)
			return
		}
		if !supportsMetricsRuntime(entry) {
			writeError(w, http.StatusBadRequest, "validation_failed", "connector does not support metrics runtime")
			return
		}

		var req connectorMetricsQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		req.Service = strings.TrimSpace(req.Service)
		req.Host = strings.TrimSpace(req.Host)
		req.Query = strings.TrimSpace(req.Query)
		req.Mode = strings.TrimSpace(req.Mode)
		req.Step = strings.TrimSpace(req.Step)
		req.Window = strings.TrimSpace(req.Window)
		if req.Query == "" && req.Service == "" && req.Host == "" {
			writeValidationError(w, "query, service, or host is required")
			return
		}

		start, err := parseOptionalRFC3339Time(req.Start)
		if err != nil {
			writeValidationError(w, "start must be RFC3339")
			return
		}
		end, err := parseOptionalRFC3339Time(req.End)
		if err != nil {
			writeValidationError(w, "end must be RFC3339")
			return
		}

		entry.Config.Values = resolveConnectorConfigValues(deps.Secrets, entry)
		if deps.Action == nil {
			writeError(w, http.StatusConflict, "not_configured", "action service is not configured")
			return
		}

		result, err := deps.Action.QueryMetrics(r.Context(), contracts.MetricsQuery{
			Service:         req.Service,
			Host:            req.Host,
			Query:           req.Query,
			Mode:            req.Mode,
			Start:           start,
			End:             end,
			Step:            req.Step,
			Window:          req.Window,
			ConnectorID:     entry.Metadata.ID,
			ConnectorType:   entry.Spec.Type,
			ConnectorVendor: entry.Metadata.Vendor,
			Protocol:        entry.Spec.Protocol,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		auditOpsRead(r.Context(), deps, "connector_runtime", entry.Metadata.ID, "metrics_query", map[string]any{
			"service":      req.Service,
			"host":         req.Host,
			"query":        req.Query,
			"mode":         req.Mode,
			"protocol":     entry.Spec.Protocol,
			"series_count": len(result.Series),
		})
		writeJSON(w, http.StatusOK, dto.ConnectorMetricsQueryResponse{
			ConnectorID: entry.Metadata.ID,
			Protocol:    entry.Spec.Protocol,
			Service:     req.Service,
			Host:        req.Host,
			Mode:        req.Mode,
			Query:       req.Query,
			Start:       start,
			End:         end,
			Step:        req.Step,
			Window:      req.Window,
			Series:      result.Series,
			Runtime:     runtimeMetadataDTO(result.Runtime),
		})
	}
}

func parseOptionalRFC3339Time(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, trimmed)
}

func connectorExecutionHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		entry, err := getConnectorEntry(deps, connectorID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		if err := connectors.ValidateRuntimeManifest(entry, "execution", entry.Spec.Protocol, map[string]struct{}{"jumpserver_api": {}, "ssh_native": {}}); err != nil {
			writeConnectorError(w, err)
			return
		}
		if strings.TrimSpace(entry.Spec.Type) != "execution" {
			writeError(w, http.StatusBadRequest, "validation_failed", "connector does not support execution runtime")
			return
		}
		if !requireSSHCredentialUseAccess(deps, w, r, entry.Spec.Protocol) {
			return
		}
		if deps.Action == nil {
			writeError(w, http.StatusConflict, "not_configured", "action service is not configured")
			return
		}

		var req connectorExecutionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		req.TargetHost = strings.TrimSpace(req.TargetHost)
		req.Command = strings.TrimSpace(req.Command)
		req.SessionID = strings.TrimSpace(req.SessionID)
		req.Service = strings.TrimSpace(req.Service)
		if req.TargetHost == "" || req.Command == "" {
			writeValidationError(w, "target_host and command are required")
			return
		}

		executionID := fmt.Sprintf("connector-%d", time.Now().UTC().UnixNano())
		result, err := deps.Action.ExecuteApproved(r.Context(), contracts.ApprovedExecutionRequest{
			ExecutionID:     executionID,
			SessionID:       req.SessionID,
			TargetHost:      req.TargetHost,
			Command:         req.Command,
			Service:         req.Service,
			ConnectorID:     entry.Metadata.ID,
			ConnectorType:   entry.Spec.Type,
			ConnectorVendor: entry.Metadata.Vendor,
			Protocol:        entry.Spec.Protocol,
			ExecutionMode:   fallbackString(req.ExecutionMode, entry.Spec.Protocol),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "connector_runtime", entry.Metadata.ID, "execute", map[string]any{
			"operator_reason": req.OperatorReason,
			"target_host":     req.TargetHost,
			"command":         req.Command,
			"protocol":        entry.Spec.Protocol,
			"execution_id":    result.ExecutionID,
			"status":          result.Status,
		})
		writeJSON(w, http.StatusOK, dto.ConnectorExecutionResponse{
			ConnectorID:     entry.Metadata.ID,
			ExecutionID:     result.ExecutionID,
			SessionID:       result.SessionID,
			Status:          result.Status,
			Protocol:        result.Protocol,
			ExecutionMode:   result.ExecutionMode,
			Runtime:         runtimeMetadataDTO(result.Runtime),
			TargetHost:      req.TargetHost,
			Command:         req.Command,
			ExitCode:        result.ExitCode,
			OutputRef:       result.OutputRef,
			OutputBytes:     result.OutputBytes,
			OutputTruncated: result.OutputTruncated,
			OutputPreview:   result.OutputPreview,
		})
	}
}

func connectorHealthHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		entry, err := getConnectorEntry(deps, connectorID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		if err := connectors.ValidateRuntimeManifest(entry, "", "", nil); err != nil {
			writeConnectorError(w, err)
			return
		}
		if !requireSSHCredentialUseAccess(deps, w, r, entry.Spec.Protocol) {
			return
		}
		if deps.Connectors == nil {
			writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
			return
		}
		if deps.Action == nil {
			writeError(w, http.StatusConflict, "not_configured", "action service is not configured")
			return
		}
		state, err := deps.Action.CheckConnectorHealth(r.Context(), entry.Metadata.ID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		auditOpsWrite(r.Context(), deps, "connector_runtime", entry.Metadata.ID, "health_check", map[string]any{
			"status":     state.Health.Status,
			"summary":    state.Health.Summary,
			"protocol":   entry.Spec.Protocol,
			"compatible": state.Compatibility.Compatible,
		})
		writeJSON(w, http.StatusOK, lifecycleStateToDTO(state))
	}
}

type connectorInvokeCapabilityRequest struct {
	CapabilityID string                 `json:"capability_id"`
	Params       map[string]interface{} `json:"params,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	Caller       string                 `json:"caller,omitempty"`
}

func connectorInvokeCapabilityHandler(deps Dependencies, connectorID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		entry, err := getConnectorEntry(deps, connectorID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		if !entry.Enabled() {
			writeError(w, http.StatusConflict, "invalid_state", "connector is disabled")
			return
		}
		if deps.Action == nil {
			writeError(w, http.StatusConflict, "not_configured", "action service is not configured")
			return
		}

		var req connectorInvokeCapabilityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		req.CapabilityID = strings.TrimSpace(req.CapabilityID)
		if req.CapabilityID == "" {
			writeValidationError(w, "capability_id is required")
			return
		}

		capResult, err := deps.Action.InvokeCapability(r.Context(), contracts.CapabilityRequest{
			ConnectorID:  entry.Metadata.ID,
			CapabilityID: req.CapabilityID,
			Params:       req.Params,
			SessionID:    strings.TrimSpace(req.SessionID),
			Caller:       strings.TrimSpace(fallbackString(req.Caller, "ops_api")),
		})
		if err != nil {
			auditOpsWrite(r.Context(), deps, "connector_runtime", entry.Metadata.ID, "invoke_capability_failed", map[string]any{
				"capability_id": req.CapabilityID,
				"error":         err.Error(),
				"status":        capResult.Status,
			})
			writeError(w, http.StatusInternalServerError, "capability_invocation_failed", err.Error())
			return
		}
		if capResult.Status == "denied" {
			auditOpsWrite(r.Context(), deps, "connector_runtime", entry.Metadata.ID, "invoke_capability_denied", map[string]any{
				"capability_id": req.CapabilityID,
				"protocol":      entry.Spec.Protocol,
				"type":          entry.Spec.Type,
			})
			writeJSON(w, http.StatusForbidden, dto.ConnectorInvokeCapabilityResponse{
				ConnectorID:  entry.Metadata.ID,
				CapabilityID: req.CapabilityID,
				Status:       capResult.Status,
				Output:       capResult.Output,
				Artifacts:    attachmentDTOs(capResult.Artifacts),
				Metadata:     capResult.Metadata,
				Error:        capResult.Error,
				Runtime:      runtimeMetadataDTO(capResult.Runtime),
			})
			return
		}
		if capResult.Status == "pending_approval" {
			auditOpsWrite(r.Context(), deps, "connector_runtime", entry.Metadata.ID, "invoke_capability_pending_approval", map[string]any{
				"capability_id": req.CapabilityID,
				"protocol":      entry.Spec.Protocol,
				"type":          entry.Spec.Type,
			})
			writeJSON(w, http.StatusAccepted, dto.ConnectorInvokeCapabilityResponse{
				ConnectorID:  entry.Metadata.ID,
				CapabilityID: req.CapabilityID,
				Status:       capResult.Status,
				Output:       capResult.Output,
				Artifacts:    attachmentDTOs(capResult.Artifacts),
				Metadata:     capResult.Metadata,
				Error:        capResult.Error,
				Runtime:      runtimeMetadataDTO(capResult.Runtime),
			})
			return
		}

		auditOpsWrite(r.Context(), deps, "connector_runtime", entry.Metadata.ID, "invoke_capability", map[string]any{
			"capability_id": req.CapabilityID,
			"status":        capResult.Status,
			"protocol":      entry.Spec.Protocol,
			"type":          entry.Spec.Type,
		})
		writeJSON(w, http.StatusOK, dto.ConnectorInvokeCapabilityResponse{
			ConnectorID:  entry.Metadata.ID,
			CapabilityID: req.CapabilityID,
			Status:       capResult.Status,
			Output:       capResult.Output,
			Artifacts:    attachmentDTOs(capResult.Artifacts),
			Metadata:     capResult.Metadata,
			Error:        capResult.Error,
			Runtime:      runtimeMetadataDTO(capResult.Runtime),
		})
	}
}

func connectorLifecycleEventsToDTO(items []connectors.LifecycleEvent) []dto.ConnectorLifecycleEvent {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ConnectorLifecycleEvent, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ConnectorLifecycleEvent{
			Type:        item.Type,
			Summary:     item.Summary,
			Version:     item.Version,
			FromVersion: item.FromVersion,
			ToVersion:   item.ToVersion,
			Enabled:     item.Enabled,
			Metadata:    cloneConnectorStringMap(item.Metadata),
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func connectorHealthHistoryToDTO(items []connectors.HealthStatus) []dto.ConnectorHealthStatus {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ConnectorHealthStatus, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ConnectorHealthStatus{
			Status:    item.Status,
			Summary:   item.Summary,
			CheckedAt: item.CheckedAt,
		})
	}
	return out
}

func getConnectorEntry(deps Dependencies, connectorID string) (connectors.Manifest, error) {
	if deps.Connectors == nil {
		return connectors.Manifest{}, connectors.ErrConfigPathNotSet
	}
	entry, ok := deps.Connectors.Get(connectorID)
	if !ok {
		return connectors.Manifest{}, connectors.ErrConnectorNotFound
	}
	return entry, nil
}

func writeConnectorError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, connectors.ErrConnectorNotFound):
		writeError(w, http.StatusNotFound, "not_found", "connector not found")
	case errors.Is(err, connectors.ErrConfigPathNotSet):
		writeError(w, http.StatusConflict, "not_configured", "connectors config path is not configured")
	case errors.Is(err, connectors.ErrConnectorDisabled):
		writeError(w, http.StatusConflict, "invalid_state", "connector is disabled")
	case errors.Is(err, connectors.ErrConnectorIncompatible):
		writeError(w, http.StatusConflict, "invalid_state", err.Error())
	case errors.Is(err, connectors.ErrConnectorRuntimeUnsupported):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func supportsMetricsRuntime(entry connectors.Manifest) bool {
	if strings.TrimSpace(entry.Spec.Type) != "metrics" {
		return false
	}
	switch strings.TrimSpace(entry.Spec.Protocol) {
	case "prometheus_http", "victoriametrics_http":
		return true
	default:
		return false
	}
}

func resolveConnectorConfigValues(store *secrets.Store, entry connectors.Manifest) map[string]string {
	return secrets.ResolveValues(store, entry.Config.Values, entry.Config.SecretRefs)
}

func sanitizeConnectorFilename(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "connector"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, id)
}

func parseOptionalBool(raw string) *bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		value := true
		return &value
	case "false", "0", "no":
		value := false
		return &value
	default:
		return nil
	}
}
