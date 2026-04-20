package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tars/internal/api/dto"
	"tars/internal/foundation/secrets"
	"tars/internal/foundation/tracing"
	"tars/internal/modules/connectors"
	"tars/internal/modules/reasoning"
)

var processStartedAt = time.Now().UTC()

type secretUpdateRequest struct {
	Upserts        []dto.SecretValueInput `json:"upserts"`
	Deletes        []string               `json:"deletes"`
	OperatorReason string                 `json:"operator_reason"`
}

type connectorTemplateApplyRequest struct {
	TemplateID     string `json:"template_id"`
	OperatorReason string `json:"operator_reason"`
}

func secretsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleSecretsGet(w, r, deps)
		case http.MethodPut:
			handleSecretsPut(w, r, deps)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func connectorTemplatesHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		items := connectorTemplatesFromSnapshot(deps)
		auditOpsRead(r.Context(), deps, "connector_template", "*", "list", map[string]any{"count": len(items)})
		writeJSON(w, http.StatusOK, dto.ConnectorTemplateListResponse{Items: items})
	}
}

func connectorTemplateApplyHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		connectorID := strings.TrimPrefix(r.URL.Path, "/api/v1/connectors/")
		connectorID = strings.TrimSuffix(connectorID, "/templates/apply")
		connectorID = strings.TrimSuffix(connectorID, "/")
		entry, err := getConnectorEntry(deps, connectorID)
		if err != nil {
			writeConnectorError(w, err)
			return
		}
		if deps.Connectors == nil {
			writeError(w, http.StatusConflict, "not_configured", "connectors manager is not configured")
			return
		}
		var req connectorTemplateApplyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" || strings.TrimSpace(req.TemplateID) == "" {
			writeValidationError(w, "template_id and operator_reason are required")
			return
		}
		beforeValues := cloneConnectorStringMap(entry.Config.Values)
		template, ok := findConnectorTemplate(entry, req.TemplateID)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "template not found")
			return
		}
		for key, value := range template.Values {
			if entry.Config.Values == nil {
				entry.Config.Values = map[string]string{}
			}
			if _, exists := entry.Config.Values[key]; !exists || strings.TrimSpace(entry.Config.Values[key]) == "" {
				entry.Config.Values[key] = value
			}
		}
		if err := deps.Connectors.Upsert(entry); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "connector_template", connectorID, "apply", map[string]any{
			"template_id":     template.ID,
			"template_name":   template.Name,
			"operator_reason": req.OperatorReason,
			"before_keys":     sortedMapKeys(beforeValues),
			"after_keys":      sortedMapKeys(entry.Config.Values),
		})
		updated, _ := deps.Connectors.Get(connectorID)
		writeJSON(w, http.StatusOK, connectorManifestToDTO(updated, true, deps.Connectors))
	}
}

func dashboardHealthHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		response := buildDashboardHealthResponse(deps)
		auditOpsRead(r.Context(), deps, "ops_dashboard", "health", "get", map[string]any{
			"connector_count": len(response.Connectors),
			"secret_count":    len(response.Secrets),
		})
		writeJSON(w, http.StatusOK, response)
	}
}

func handleSecretsGet(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	storeSnapshot := secrets.Snapshot{}
	if deps.Secrets != nil {
		storeSnapshot = deps.Secrets.Snapshot()
	}
	response := dto.SecretsInventoryResponse{
		Configured: strings.TrimSpace(storeSnapshot.Path) != "",
		Loaded:     storeSnapshot.Loaded,
		Path:       storeSnapshot.Path,
		UpdatedAt:  storeSnapshot.UpdatedAt,
		Items:      listSecretDescriptors(deps, storeSnapshot),
	}
	auditOpsRead(r.Context(), deps, "secret_inventory", "runtime", "get", map[string]any{"count": len(response.Items)})
	writeJSON(w, http.StatusOK, response)
}

func handleSecretsPut(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Secrets == nil {
		writeError(w, http.StatusConflict, "not_configured", "secret store is not configured")
		return
	}
	var req secretUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeValidationError(w, "operator_reason is required")
		return
	}
	upserts := make(map[string]string, len(req.Upserts))
	for _, item := range req.Upserts {
		ref := strings.TrimSpace(item.Ref)
		if ref == "" {
			writeValidationError(w, "secret ref is required")
			return
		}
		upserts[ref] = item.Value
	}
	_, err := deps.Secrets.Apply(upserts, req.Deletes, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	for _, item := range req.Upserts {
		meta, _ := deps.Secrets.Metadata(strings.TrimSpace(item.Ref))
		if deps.Metrics != nil {
			deps.Metrics.RecordComponentResult("secret_store", "success", "secret updated: "+strings.TrimSpace(item.Ref))
		}
		auditOpsWrite(r.Context(), deps, "secret_ref", strings.TrimSpace(item.Ref), "upsert", map[string]any{
			"operator_reason": req.OperatorReason,
			"ref":             strings.TrimSpace(item.Ref),
			"previously_set":  meta.Set,
		})
	}
	for _, ref := range req.Deletes {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			continue
		}
		auditOpsWrite(r.Context(), deps, "secret_ref", trimmed, "delete", map[string]any{
			"operator_reason": req.OperatorReason,
			"ref":             trimmed,
		})
	}
	auditOpsWrite(r.Context(), deps, "secret_inventory", "runtime", "update", map[string]any{
		"operator_reason": req.OperatorReason,
		"upserts":         secretRefsForAudit(req.Upserts),
		"deletes":         cloneStrings(req.Deletes),
	})
	handleSecretsGet(w, r, deps)
}

func listSecretDescriptors(deps Dependencies, snapshot secrets.Snapshot) []dto.SecretDescriptor {
	items := make([]dto.SecretDescriptor, 0)
	seen := make(map[string]struct{})
	for _, item := range connectorSecretDescriptors(deps.Connectors, snapshot) {
		if _, ok := seen[item.Ref+":"+item.OwnerType+":"+item.OwnerID+":"+item.Key]; ok {
			continue
		}
		seen[item.Ref+":"+item.OwnerType+":"+item.OwnerID+":"+item.Key] = struct{}{}
		items = append(items, item)
	}
	for _, item := range providerSecretDescriptors(deps.Providers, snapshot) {
		if _, ok := seen[item.Ref+":"+item.OwnerType+":"+item.OwnerID+":"+item.Key]; ok {
			continue
		}
		seen[item.Ref+":"+item.OwnerType+":"+item.OwnerID+":"+item.Key] = struct{}{}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].OwnerType == items[j].OwnerType {
			if items[i].OwnerID == items[j].OwnerID {
				return items[i].Key < items[j].Key
			}
			return items[i].OwnerID < items[j].OwnerID
		}
		return items[i].OwnerType < items[j].OwnerType
	})
	return items
}

func connectorSecretDescriptors(manager *connectors.Manager, snapshot secrets.Snapshot) []dto.SecretDescriptor {
	if manager == nil {
		return nil
	}
	entries := manager.Snapshot().Config.Entries
	out := make([]dto.SecretDescriptor, 0)
	for _, entry := range entries {
		for key, ref := range entry.Config.SecretRefs {
			meta, _ := snapshot.Entries[ref]
			out = append(out, dto.SecretDescriptor{
				Ref:       ref,
				OwnerType: "connector",
				OwnerID:   entry.Metadata.ID,
				Key:       key,
				Set:       meta.Set,
				UpdatedAt: meta.UpdatedAt,
				Source:    meta.Source,
			})
		}
	}
	return out
}

func providerSecretDescriptors(manager *reasoning.ProviderManager, snapshot secrets.Snapshot) []dto.SecretDescriptor {
	if manager == nil {
		return nil
	}
	entries := manager.Snapshot().Config.Entries
	out := make([]dto.SecretDescriptor, 0)
	for _, entry := range entries {
		ref := strings.TrimSpace(entry.APIKeyRef)
		if ref == "" {
			continue
		}
		meta, _ := snapshot.Entries[ref]
		out = append(out, dto.SecretDescriptor{
			Ref:       ref,
			OwnerType: "provider",
			OwnerID:   entry.ID,
			Key:       "api_key",
			Set:       meta.Set,
			UpdatedAt: meta.UpdatedAt,
			Source:    meta.Source,
		})
	}
	return out
}

func connectorTemplatesFromSnapshot(deps Dependencies) []dto.ConnectorTemplate {
	if deps.Connectors == nil {
		return nil
	}
	entries := deps.Connectors.Snapshot().Config.Entries
	out := make([]dto.ConnectorTemplate, 0)
	for _, entry := range entries {
		for _, template := range buildDefaultTemplates(entry) {
			out = append(out, dto.ConnectorTemplate{
				ConnectorID: entry.Metadata.ID,
				TemplateID:  template.ID,
				Name:        template.Name,
				Description: template.Description,
				Values:      cloneConnectorStringMap(template.Values),
				CreatedAt:   template.CreatedAt,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ConnectorID == out[j].ConnectorID {
			return out[i].TemplateID < out[j].TemplateID
		}
		return out[i].ConnectorID < out[j].ConnectorID
	})
	return out
}

func buildDefaultTemplates(entry connectors.Manifest) []connectors.TemplateAssignment {
	now := time.Now().UTC()
	switch strings.TrimSpace(entry.Spec.Protocol) {
	case "prometheus_http":
		return []connectors.TemplateAssignment{{
			ID:          "prometheus-pilot",
			Name:        "Pilot Prometheus",
			Description: "Common observability preset for a single Prometheus endpoint.",
			Values: map[string]string{
				"base_url": firstTemplateValue(entry.Config.Values["base_url"], "http://127.0.0.1:9090"),
			},
			CreatedAt: now,
		}}
	case "victoriametrics_http":
		return []connectors.TemplateAssignment{{
			ID:          "victoriametrics-pilot",
			Name:        "Pilot VictoriaMetrics",
			Description: "Common observability preset for a VictoriaMetrics endpoint.",
			Values: map[string]string{
				"base_url": firstTemplateValue(entry.Config.Values["base_url"], "http://127.0.0.1:8428"),
			},
			CreatedAt: now,
		}}
	case "jumpserver_api":
		return []connectors.TemplateAssignment{{
			ID:          "jumpserver-pilot",
			Name:        "Pilot JumpServer",
			Description: "Execution preset wired for a managed JumpServer entrypoint.",
			Values: map[string]string{
				"base_url": firstTemplateValue(entry.Config.Values["base_url"], "https://jumpserver.example.com"),
			},
			CreatedAt: now,
		}}
	case "ssh_native":
		return []connectors.TemplateAssignment{{
			ID:          "ssh-native-pilot",
			Name:        "Pilot SSH Native",
			Description: "Direct SSH execution preset. Create the credential in SSH Credential Custody first, then paste its credential_id here.",
			Values: map[string]string{
				"host":          firstTemplateValue(entry.Config.Values["host"], "192.168.3.100"),
				"port":          firstTemplateValue(entry.Config.Values["port"], "22"),
				"username":      firstTemplateValue(entry.Config.Values["username"], "root"),
				"credential_id": firstTemplateValue(entry.Config.Values["credential_id"], "REPLACE_WITH_SSH_CREDENTIAL_ID"),
			},
			CreatedAt: now,
		}}
	default:
		return nil
	}
}

func findConnectorTemplate(entry connectors.Manifest, templateID string) (connectors.TemplateAssignment, bool) {
	for _, item := range buildDefaultTemplates(entry) {
		if item.ID == templateID {
			return item, true
		}
	}
	return connectors.TemplateAssignment{}, false
}

func buildDashboardHealthResponse(deps Dependencies) dto.DashboardHealthResponse {
	response := dto.DashboardHealthResponse{
		Resources:  dashboardRuntimeResources(deps),
		Connectors: connectorHealthCards(deps),
		Providers:  providerHealthCards(deps),
		Secrets:    listSecretDescriptors(deps, snapshotSecrets(deps.Secrets)),
		Alerts:     dashboardAlerts(deps),
	}
	response.Summary = dto.DashboardHealthSummary{
		HealthyConnectors:  countConnectorHealth(response.Connectors, "healthy"),
		DegradedConnectors: countConnectorHealth(response.Connectors, "degraded"),
		DisabledConnectors: countConnectorHealth(response.Connectors, "disabled"),
		ConfiguredSecrets:  countSecretsSet(response.Secrets),
		MissingSecrets:     len(response.Secrets) - countSecretsSet(response.Secrets),
		ActiveAlerts:       len(response.Alerts),
		ProviderFailures:   countProviderFailures(response.Providers),
	}
	return response
}

func dashboardRuntimeResources(deps Dependencies) dto.DashboardRuntimeResources {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	resources := dto.DashboardRuntimeResources{
		UptimeSeconds:      int64(time.Since(processStartedAt).Seconds()),
		Goroutines:         runtime.NumGoroutine(),
		HeapAllocBytes:     mem.HeapAlloc,
		HeapSysBytes:       mem.HeapSys,
		HeapInUseBytes:     mem.HeapInuse,
		StackInUseBytes:    mem.StackInuse,
		GCCount:            mem.NumGC,
		LastGCPauseSeconds: lastGCPauseSeconds(mem),
		CPUCount:           runtime.NumCPU(),
		LoadAverage:        loadAverageSamples(),
		NetworkInterfaces:  networkInterfaceSamples(),
		LogLevel:           strings.TrimSpace(deps.Config.LogLevel),
		SpoolDir:           strings.TrimSpace(deps.Config.Output.SpoolDir),
	}
	if used, free, percent, ok := statfsUsage(firstExistingPath(deps.Config.Output.SpoolDir, deps.Config.Connectors.ConfigPath, deps.Config.Reasoning.ProvidersConfigPath)); ok {
		resources.DiskUsedBytes = used
		resources.DiskFreeBytes = free
		resources.DiskUsagePercent = percent
	}
	provider := tracing.New(deps.Config.Observability.OTLP)
	resources.TracingEnabled = provider.Enabled()
	resources.TracingProvider = provider.Name()
	return resources
}

func lastGCPauseSeconds(mem runtime.MemStats) float64 {
	if mem.NumGC == 0 {
		return 0
	}
	index := (mem.NumGC - 1) % uint32(len(mem.PauseNs))
	return float64(mem.PauseNs[index]) / float64(time.Second)
}

func loadAverageSamples() []dto.DashboardResourceSample {
	content, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(content))
	if len(fields) < 3 {
		return nil
	}
	labels := []string{"1m", "5m", "15m"}
	loads := make([]dto.DashboardResourceSample, 0, 3)
	for index := 0; index < 3; index++ {
		value, parseErr := parseFloat64(fields[index])
		if parseErr != nil {
			continue
		}
		loads = append(loads, dto.DashboardResourceSample{Label: labels[index], Value: value, Unit: "load"})
	}
	return loads
}

func networkInterfaceSamples() []dto.DashboardResourceSample {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]dto.DashboardResourceSample, 0, len(interfaces))
	for _, item := range interfaces {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		value := 0.0
		if item.Flags&net.FlagUp != 0 {
			value = 1
		}
		out = append(out, dto.DashboardResourceSample{Label: item.Name, Value: value, Unit: "up"})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Label < out[j].Label
	})
	return out
}

func statfsUsage(path string) (uint64, uint64, float64, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return 0, 0, 0, false
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, false
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	percent := 0.0
	if total > 0 {
		percent = (float64(used) / float64(total)) * 100
	}
	return used, free, percent, true
}

func firstExistingPath(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		candidate := trimmed
		if info, err := os.Stat(candidate); err == nil {
			if info.IsDir() {
				return candidate
			}
			return filepath.Dir(candidate)
		}
		return filepath.Dir(candidate)
	}
	return ""
}

func parseFloat64(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func connectorHealthCards(deps Dependencies) []dto.DashboardConnectorHealth {
	if deps.Connectors == nil {
		return nil
	}
	items := deps.Connectors.ListLifecycle()
	out := make([]dto.DashboardConnectorHealth, 0, len(items))
	for _, item := range items {
		out = append(out, dto.DashboardConnectorHealth{
			ConnectorID:      item.ConnectorID,
			DisplayName:      item.DisplayName,
			Protocol:         item.Runtime.Protocol,
			Type:             item.Runtime.Type,
			Vendor:           item.Runtime.Vendor,
			Status:           item.Health.Status,
			CredentialStatus: item.Health.CredentialStatus,
			Summary:          item.Health.Summary,
			CheckedAt:        item.Health.CheckedAt,
			CurrentVersion:   item.CurrentVersion,
		})
	}
	return out
}

func providerHealthCards(deps Dependencies) []dto.DashboardProviderHealth {
	if deps.Providers == nil {
		return nil
	}
	items := deps.Providers.Snapshot().Config.Entries
	out := make([]dto.DashboardProviderHealth, 0, len(items))
	for _, entry := range items {
		status := componentRuntimeStatus(deps, providerComponentName(entry.ID, entry.ID == deps.Providers.Snapshot().Config.Primary.ProviderID, entry.ID == deps.Providers.Snapshot().Config.Assist.ProviderID))
		out = append(out, dto.DashboardProviderHealth{
			ProviderID:    entry.ID,
			Vendor:        entry.Vendor,
			Protocol:      entry.Protocol,
			BaseURL:       entry.BaseURL,
			Enabled:       entry.Enabled,
			LastResult:    status.LastResult,
			LastDetail:    status.LastDetail,
			LastError:     status.LastError,
			LastChangedAt: status.LastChangedAt,
		})
	}
	return out
}

func dashboardAlerts(deps Dependencies) []dto.DashboardAlertItem {
	alerts := make([]dto.DashboardAlertItem, 0)
	for _, item := range connectorHealthCards(deps) {
		if item.Status == "degraded" {
			alerts = append(alerts, dto.DashboardAlertItem{
				Severity:  "warning",
				Resource:  item.ConnectorID,
				Title:     "Connector degraded",
				Summary:   item.Summary,
				CreatedAt: item.CheckedAt,
			})
		}
	}
	for _, item := range providerHealthCards(deps) {
		if strings.EqualFold(item.LastResult, "error") || strings.EqualFold(item.LastResult, "failed") {
			alerts = append(alerts, dto.DashboardAlertItem{
				Severity:  "critical",
				Resource:  item.ProviderID,
				Title:     "Provider unavailable",
				Summary:   firstTemplateValue(item.LastError, item.LastDetail),
				CreatedAt: item.LastChangedAt,
			})
		}
	}
	sort.SliceStable(alerts, func(i, j int) bool {
		return alerts[i].CreatedAt.After(alerts[j].CreatedAt)
	})
	return alerts
}

func snapshotSecrets(store *secrets.Store) secrets.Snapshot {
	if store == nil {
		return secrets.Snapshot{}
	}
	return store.Snapshot()
}

func countConnectorHealth(items []dto.DashboardConnectorHealth, status string) int {
	count := 0
	for _, item := range items {
		if item.Status == status {
			count++
		}
	}
	return count
}

func countSecretsSet(items []dto.SecretDescriptor) int {
	count := 0
	for _, item := range items {
		if item.Set {
			count++
		}
	}
	return count
}

func countProviderFailures(items []dto.DashboardProviderHealth) int {
	count := 0
	for _, item := range items {
		if strings.EqualFold(item.LastResult, "error") || strings.EqualFold(item.LastResult, "failed") {
			count++
		}
	}
	return count
}

func providerComponentName(providerID string, primary bool, assist bool) string {
	switch {
	case primary:
		return "model_primary"
	case assist:
		return "model_assist"
	default:
		return providerID
	}
}

func firstTemplateValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func secretRefsForAudit(items []dto.SecretValueInput) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if ref := strings.TrimSpace(item.Ref); ref != "" {
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}

func sortedMapKeys(input map[string]string) []string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	sort.Strings(keys)
	return keys
}
