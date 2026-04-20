package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/connectors"
	"tars/internal/modules/reasoning"
)

type smokeAlertRequest struct {
	AlertName string `json:"alertname"`
	Service   string `json:"service"`
	Host      string `json:"host"`
	Severity  string `json:"severity"`
	Summary   string `json:"summary"`
}

func bootstrapStatusHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		initialization, err := buildSetupInitializationStatus(r, deps)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, dto.BootstrapStatusResponse{
			Initialized: initialization.Initialized,
			Mode:        initialization.Mode,
			NextStep:    initialization.NextStep,
		})
	}
}

func setupStatusHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		initialization, err := buildSetupInitializationStatus(r, deps)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if !setupWizardAccessAllowed(deps, r, initialization) && !requireOpsAccess(deps, w, r) {
			return
		}

		status, err := buildSetupStatusResponse(r, deps)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		status.Initialization = initialization

		auditOpsRead(r.Context(), deps, "setup_status", "runtime", "get", map[string]any{
			"has_latest_smoke": status.LatestSmoke != nil,
			"telegram_mode":    status.Telegram.Mode,
			"rollout_mode":     status.RolloutMode,
		})
		writeJSON(w, http.StatusOK, status)
	}
}

func smokeAlertHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		var req smokeAlertRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
			return
		}
		if strings.TrimSpace(req.AlertName) == "" || strings.TrimSpace(req.Service) == "" || strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.Severity) == "" || strings.TrimSpace(req.Summary) == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "alertname, service, host, severity, summary are required")
			return
		}

		smokeID := fmt.Sprintf("smoke-%d", time.Now().UTC().UnixNano())
		labels := map[string]string{
			"alertname":      strings.TrimSpace(req.AlertName),
			"service":        strings.TrimSpace(req.Service),
			"instance":       strings.TrimSpace(req.Host),
			"host":           strings.TrimSpace(req.Host),
			"severity":       strings.TrimSpace(req.Severity),
			"tars_smoke":     "true",
			"tars_smoke_id":  smokeID,
			"tars_generated": "ops_setup",
		}
		annotations := map[string]string{
			"summary":       strings.TrimSpace(req.Summary),
			"smoke_trigger": "setup_console",
		}
		alertPayload := map[string]any{
			"labels":      labels,
			"annotations": annotations,
		}
		telegramTarget := resolveAlertTelegramTarget(deps.Approval, alertPayload)
		if telegramTarget != "" {
			labels["telegram_target"] = telegramTarget
			labels["chat_id"] = telegramTarget
		}
		payload := map[string]any{
			"status": "firing",
			"alerts": []map[string]any{alertPayload},
		}

		rawPayload, err := json.Marshal(payload)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		events, err := deps.AlertIngest.IngestVMAlert(r.Context(), rawPayload)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		if len(events) == 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", "no alert events generated")
			return
		}

		event := events[0]
		event.Fingerprint = fmt.Sprintf("%s:%s", event.Fingerprint, smokeID)
		event.IdempotencyKey = fmt.Sprintf("smoke:%s:%s", event.RequestHash, smokeID)
		if event.Labels == nil {
			event.Labels = map[string]string{}
		}
		event.Labels["tars_smoke"] = "true"
		event.Labels["tars_smoke_id"] = smokeID
		event.Labels["instance"] = strings.TrimSpace(req.Host)
		event.Labels["host"] = strings.TrimSpace(req.Host)

		result, err := deps.Workflow.HandleAlertEvent(r.Context(), event)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		if deps.Audit != nil {
			deps.Audit.Log(r.Context(), audit.Entry{
				ResourceType: "smoke_alert",
				ResourceID:   result.SessionID,
				Action:       "trigger",
				Actor:        "ops_api",
				Metadata: map[string]any{
					"alertname":      req.AlertName,
					"service":        req.Service,
					"host":           req.Host,
					"severity":       req.Severity,
					"telegram_tgt":   telegramTarget,
					"session_status": result.Status,
				},
			})
		}

		writeJSON(w, http.StatusOK, dto.SmokeAlertResponse{
			Accepted:   true,
			SessionID:  result.SessionID,
			Status:     result.Status,
			Duplicated: result.Duplicated,
			TGTarget:   telegramTarget,
		})
	}
}

func buildSetupStatusResponse(r *http.Request, deps Dependencies) (dto.SetupStatusResponse, error) {
	var latestSmoke *dto.SmokeSessionStatus

	sessions, err := deps.Workflow.ListSessions(r.Context(), contracts.ListSessionsFilter{})
	if err != nil {
		return dto.SetupStatusResponse{}, err
	}
	initialization, err := buildSetupInitializationStatus(r, deps)
	if err != nil {
		return dto.SetupStatusResponse{}, err
	}
	for _, session := range sessions {
		if !isSmokeAlert(session.Alert) {
			continue
		}
		latestSmoke = smokeSessionStatus(deps.Approval, session)
		break
	}

	primaryModelStatus, assistModelStatus, providersStatus := providerSetupStatuses(deps)
	if !primaryModelStatus.Configured {
		primaryModelStatus = dto.ProviderSetupStatus{
			Configured:             strings.TrimSpace(deps.Config.Model.BaseURL) != "" && strings.TrimSpace(deps.Config.Model.Model) != "",
			Protocol:               deps.Config.Model.Protocol,
			BaseURL:                deps.Config.Model.BaseURL,
			ModelName:              deps.Config.Model.Model,
			ComponentRuntimeStatus: componentRuntimeStatus(deps, "model"),
		}
	}
	if !assistModelStatus.Configured {
		assistModelStatus = assistProviderSetupStatus(deps)
	}

	return dto.SetupStatusResponse{
		RolloutMode: deps.Config.Features.RolloutMode,
		Features: dto.SetupFeatures{
			DiagnosisEnabled:       deps.Config.Features.DiagnosisEnabled,
			ApprovalEnabled:        deps.Config.Features.ApprovalEnabled,
			ExecutionEnabled:       deps.Config.Features.ExecutionEnabled,
			KnowledgeIngestEnabled: deps.Config.Features.KnowledgeIngestEnabled,
		},
		Initialization: initialization,
		Telegram: dto.TelegramSetupStatus{
			Configured:             strings.TrimSpace(deps.Config.Telegram.BotToken) != "",
			Polling:                deps.Config.Telegram.PollingEnabled,
			BaseURL:                deps.Config.Telegram.BaseURL,
			Mode:                   telegramMode(deps.Config),
			ComponentRuntimeStatus: componentRuntimeStatus(deps, "telegram"),
		},
		Model:           primaryModelStatus,
		AssistModel:     assistModelStatus,
		Providers:       providersStatus,
		Connectors:      connectorsSetupStatus(deps),
		LegacyFallbacks: nil,
		SmokeDefaults: dto.SmokeDefaultsSetupStatus{
			Hosts: smokeDefaultHosts(latestSmoke, deps.Config.SSH.AllowedHosts),
		},
		Authorization:   authorizationSetupStatus(deps),
		Approval:        approvalRoutingSetupStatus(deps),
		Reasoning:       reasoningPromptSetupStatus(deps),
		Desensitization: desensitizationSetupStatus(deps),
		LatestSmoke:     latestSmoke,
	}, nil
}

func smokeDefaultHosts(latestSmoke *dto.SmokeSessionStatus, allowedHosts []string) []string {
	seen := map[string]struct{}{}
	hosts := make([]string, 0, len(allowedHosts)+1)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		hosts = append(hosts, value)
	}
	for _, host := range allowedHosts {
		add(host)
	}
	if latestSmoke != nil {
		add(latestSmoke.Host)
	}
	return hosts
}

func providerSetupStatuses(deps Dependencies) (dto.ProviderSetupStatus, dto.ProviderSetupStatus, dto.ProvidersSetupStatus) {
	if deps.Providers == nil {
		return dto.ProviderSetupStatus{}, dto.ProviderSetupStatus{}, dto.ProvidersSetupStatus{}
	}

	snapshot := deps.Providers.Snapshot()
	registry := dto.ProvidersSetupStatus{
		Configured:        strings.TrimSpace(snapshot.Path) != "",
		Loaded:            snapshot.Loaded,
		Path:              snapshot.Path,
		UpdatedAt:         snapshot.UpdatedAt,
		PrimaryProviderID: snapshot.Config.Primary.ProviderID,
		AssistProviderID:  snapshot.Config.Assist.ProviderID,
	}

	primary := providerTargetSetupStatus(deps, deps.Providers.ResolvePrimaryModelTarget(), "model_primary")
	assist := providerTargetSetupStatus(deps, deps.Providers.ResolveAssistModelTarget(), "model_assist")
	return primary, assist, registry
}

func connectorsSetupStatus(deps Dependencies) dto.ConnectorsSetupStatus {
	if deps.Connectors == nil {
		return dto.ConnectorsSetupStatus{}
	}
	snapshot := deps.Connectors.Snapshot()
	kindsSet := map[string]struct{}{}
	enabledEntries := 0
	for _, entry := range snapshot.Config.Entries {
		if entry.Enabled() {
			enabledEntries++
		}
		if strings.TrimSpace(entry.Spec.Type) != "" {
			kindsSet[strings.TrimSpace(entry.Spec.Type)] = struct{}{}
		}
	}
	kinds := make([]string, 0, len(kindsSet))
	for kind := range kindsSet {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return dto.ConnectorsSetupStatus{
		Configured:           strings.TrimSpace(snapshot.Path) != "",
		Loaded:               snapshot.Loaded,
		Path:                 snapshot.Path,
		UpdatedAt:            snapshot.UpdatedAt,
		TotalEntries:         len(snapshot.Config.Entries),
		EnabledEntries:       enabledEntries,
		Kinds:                kinds,
		MetricsRuntime:       runtimeSetupStatus(deps, "metrics", map[string]struct{}{"prometheus_http": {}, "victoriametrics_http": {}}, "victoriametrics", "legacy_provider", "victoriametrics_http", "", "stub"),
		ExecutionRuntime:     runtimeSetupStatus(deps, "execution", map[string]struct{}{"jumpserver_api": {}, "ssh_native": {}}, "ssh", "ssh", "ssh_native", "ssh", "ssh"),
		VerificationRuntime:  runtimeSetupStatus(deps, "execution", map[string]struct{}{"jumpserver_api": {}, "ssh_native": {}}, "ssh", "ssh", "ssh_native", "ssh", "ssh"),
		ObservabilityRuntime: runtimeSetupStatus(deps, "observability", map[string]struct{}{"observability_http": {}, "log_file": {}, "stub": {}}, "observability", "connector_capability", "observability_http", "", "stub"),
		DeliveryRuntime:      runtimeSetupStatus(deps, "delivery", map[string]struct{}{"delivery_git": {}, "delivery_github": {}, "stub": {}}, "delivery", "connector_capability", "delivery_git", "", "stub"),
	}
}

func runtimeSetupStatus(deps Dependencies, expectedType string, supported map[string]struct{}, fallbackComponent string, fallbackRuntime string, fallbackProtocol string, fallbackMode string, fallbackTarget string) *dto.RuntimeSetupStatus {
	if deps.Connectors == nil {
		return nil
	}
	entry, ok := connectors.SelectHealthyRuntimeManifest(deps.Connectors, expectedType, "", supported)
	setup := &dto.RuntimeSetupStatus{
		Name: expectedType,
		Fallback: &dto.RuntimeMetadata{
			Runtime:         fallbackRuntime,
			Selection:       "fallback",
			Protocol:        fallbackProtocol,
			ExecutionMode:   fallbackMode,
			RuntimeState:    runtimeStateForProtocol(fallbackProtocol),
			FallbackEnabled: true,
			FallbackUsed:    !ok,
			FallbackTarget:  fallbackTarget,
		},
		CapabilityTool: runtimeCapabilityTool(expectedType),
	}
	if !ok {
		setup.Fallback.FallbackReason = "no_healthy_connector_selected"
		setup.Component = fallbackComponent
		setup.ComponentRuntime = componentRuntimeStatus(deps, fallbackComponent)
		return setup
	}

	setup.Component = entry.Metadata.ID
	setup.ComponentRuntime = connectorRuntimeStatus(deps, entry.Metadata.ID)
	if ok {
		setup.Primary = &dto.RuntimeMetadata{
			Runtime:         "connector",
			Selection:       "auto_selector",
			ConnectorID:     entry.Metadata.ID,
			ConnectorType:   entry.Spec.Type,
			ConnectorVendor: entry.Metadata.Vendor,
			Protocol:        entry.Spec.Protocol,
			ExecutionMode:   connectors.DefaultExecutionMode(entry.Spec.Protocol),
			RuntimeState:    runtimeStateForProtocol(entry.Spec.Protocol),
			FallbackEnabled: true,
			FallbackUsed:    false,
			FallbackTarget:  fallbackTarget,
		}
	}
	return setup
}

func runtimeCapabilityTool(expectedType string) string {
	switch strings.ToLower(strings.TrimSpace(expectedType)) {
	case "observability":
		return "observability.query"
	case "delivery":
		return "delivery.query"
	case "metrics":
		return "metrics.query_range"
	default:
		return ""
	}
}

func runtimeStateForProtocol(protocol string) string {
	if strings.EqualFold(strings.TrimSpace(protocol), "stub") {
		return "stub"
	}
	if strings.TrimSpace(protocol) == "" {
		return ""
	}
	return "real"
}

func connectorRuntimeStatus(deps Dependencies, connectorID string) dto.ComponentRuntimeStatus {
	if deps.Connectors == nil {
		return dto.ComponentRuntimeStatus{}
	}
	state, ok := deps.Connectors.GetLifecycle(strings.TrimSpace(connectorID))
	if !ok {
		return dto.ComponentRuntimeStatus{}
	}
	status := dto.ComponentRuntimeStatus{
		LastResult:    state.Health.Status,
		LastDetail:    state.Health.Summary,
		LastChangedAt: state.Health.CheckedAt,
	}
	if strings.EqualFold(state.Health.Status, "healthy") {
		status.LastSuccessAt = state.Health.CheckedAt
	} else if strings.TrimSpace(state.Health.Status) != "" {
		status.LastError = state.Health.Summary
		status.LastErrorAt = state.Health.CheckedAt
	}
	return status
}

func providerTargetSetupStatus(deps Dependencies, target *reasoning.ModelTarget, component string) dto.ProviderSetupStatus {
	if target == nil {
		return dto.ProviderSetupStatus{}
	}
	return dto.ProviderSetupStatus{
		Configured:             strings.TrimSpace(target.BaseURL) != "" && strings.TrimSpace(target.Model) != "",
		ProviderID:             target.ProviderID,
		Vendor:                 target.Vendor,
		Protocol:               target.Protocol,
		BaseURL:                target.BaseURL,
		ModelName:              target.Model,
		ComponentRuntimeStatus: componentRuntimeStatus(deps, component),
	}
}

func assistProviderSetupStatus(deps Dependencies) dto.ProviderSetupStatus {
	if deps.Desense == nil {
		return dto.ProviderSetupStatus{}
	}
	cfg := deps.Desense.CurrentDesensitizationConfig()
	if cfg == nil || !cfg.LocalLLMAssist.Enabled {
		return dto.ProviderSetupStatus{}
	}
	return dto.ProviderSetupStatus{
		Configured:             strings.TrimSpace(cfg.LocalLLMAssist.BaseURL) != "" && strings.TrimSpace(cfg.LocalLLMAssist.Model) != "",
		Vendor:                 cfg.LocalLLMAssist.Provider,
		Protocol:               cfg.LocalLLMAssist.Provider,
		BaseURL:                cfg.LocalLLMAssist.BaseURL,
		ModelName:              cfg.LocalLLMAssist.Model,
		ComponentRuntimeStatus: componentRuntimeStatus(deps, "desensitization_local_llm"),
	}
}

func authorizationSetupStatus(deps Dependencies) dto.AuthorizationSetupStatus {
	if deps.Authorization == nil {
		return dto.AuthorizationSetupStatus{}
	}
	snapshot := deps.Authorization.Snapshot()
	return dto.AuthorizationSetupStatus{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
	}
}

func approvalRoutingSetupStatus(deps Dependencies) dto.ApprovalRoutingSetupStatus {
	if deps.Approval == nil {
		return dto.ApprovalRoutingSetupStatus{}
	}
	snapshot := deps.Approval.Snapshot()
	return dto.ApprovalRoutingSetupStatus{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
	}
}

func reasoningPromptSetupStatus(deps Dependencies) dto.ReasoningPromptSetupStatus {
	if deps.Prompts == nil {
		return dto.ReasoningPromptSetupStatus{
			LocalCommandFallbackEnabled: deps.Config.Reasoning.LocalCommandFallbackEnable,
		}
	}
	snapshot := deps.Prompts.Snapshot()
	return dto.ReasoningPromptSetupStatus{
		Configured:                  strings.TrimSpace(snapshot.Path) != "",
		Loaded:                      snapshot.Loaded,
		Path:                        snapshot.Path,
		UpdatedAt:                   snapshot.UpdatedAt,
		LocalCommandFallbackEnabled: deps.Config.Reasoning.LocalCommandFallbackEnable,
	}
}

func desensitizationSetupStatus(deps Dependencies) dto.DesensitizationSetupStatus {
	if deps.Desense == nil {
		return dto.DesensitizationSetupStatus{}
	}
	snapshot := deps.Desense.Snapshot()
	return dto.DesensitizationSetupStatus{
		Configured:            strings.TrimSpace(snapshot.Path) != "",
		Loaded:                snapshot.Loaded,
		Path:                  snapshot.Path,
		UpdatedAt:             snapshot.UpdatedAt,
		Enabled:               snapshot.Config.Enabled,
		LocalLLMAssistEnabled: snapshot.Config.LocalLLMAssist.Enabled,
		LocalLLMBaseURL:       snapshot.Config.LocalLLMAssist.BaseURL,
		LocalLLMModel:         snapshot.Config.LocalLLMAssist.Model,
		LocalLLMMode:          snapshot.Config.LocalLLMAssist.Mode,
	}
}

func smokeSessionStatus(router approvalrouting.Router, session contracts.SessionDetail) *dto.SmokeSessionStatus {
	if !isSmokeAlert(session.Alert) {
		return nil
	}

	updatedAt := latestSessionTimestamp(session)
	executionStatus := ""
	approvalRequested := len(session.Executions) > 0
	if len(session.Executions) > 0 {
		executionStatus = session.Executions[len(session.Executions)-1].Status
	}

	return &dto.SmokeSessionStatus{
		SessionID:          session.SessionID,
		Status:             session.Status,
		AlertName:          alertLabelValue(session.Alert, "alertname"),
		Service:            alertLabelValue(session.Alert, "service"),
		Host:               fallbackString(alertStringValue(session.Alert, "host"), alertLabelValue(session.Alert, "instance")),
		TelegramTarget:     resolveAlertTelegramTarget(router, session.Alert),
		ApprovalRequested:  approvalRequested,
		ExecutionStatus:    executionStatus,
		VerificationStatus: verificationStatus(session.Verification),
		UpdatedAt:          updatedAt,
	}
}

func latestSessionTimestamp(session contracts.SessionDetail) time.Time {
	var latest time.Time
	for _, item := range session.Timeline {
		if item.CreatedAt.After(latest) {
			latest = item.CreatedAt
		}
	}
	for _, execution := range session.Executions {
		for _, ts := range []time.Time{execution.CreatedAt, execution.ApprovedAt, execution.CompletedAt} {
			if ts.After(latest) {
				latest = ts
			}
		}
	}
	if session.Verification != nil && session.Verification.CheckedAt.After(latest) {
		latest = session.Verification.CheckedAt
	}
	return latest
}

func verificationStatus(verification *contracts.SessionVerification) string {
	if verification == nil {
		return ""
	}
	return verification.Status
}

func resolveAlertTelegramTarget(router approvalrouting.Router, alert map[string]interface{}) string {
	target := strings.TrimSpace(alertStringValue(alert, "telegram_target"))
	if target != "" {
		return target
	}

	if router == nil {
		return notificationTargetFromAlert(alert)
	}
	route := router.Resolve(alert, "ops_api", notificationTargetFromAlert(alert))
	if len(route.Targets) == 0 {
		return notificationTargetFromAlert(alert)
	}
	return strings.TrimSpace(route.Targets[0])
}

func telegramMode(cfg config.Config) string {
	if strings.TrimSpace(cfg.Telegram.BotToken) == "" {
		return "disabled"
	}
	if cfg.Telegram.PollingEnabled {
		return "polling"
	}
	return "webhook"
}

func componentRuntimeStatus(deps Dependencies, component string) dto.ComponentRuntimeStatus {
	if deps.Metrics == nil {
		return dto.ComponentRuntimeStatus{}
	}
	status, ok := deps.Metrics.GetComponentStatus(component)
	if !ok {
		return dto.ComponentRuntimeStatus{}
	}
	return dto.ComponentRuntimeStatus{
		LastResult:    status.Result,
		LastDetail:    status.Detail,
		LastChangedAt: status.LastChangedAt,
		LastSuccessAt: status.LastSuccessAt,
		LastError:     status.LastError,
		LastErrorAt:   status.LastErrorAt,
	}
}

func cloneStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, len(items))
	copy(out, items)
	return out
}

func notificationTargetFromAlert(alert map[string]interface{}) string {
	for _, key := range []string{"telegram_target", "chat_id", "room"} {
		if value := strings.TrimSpace(alertStringValue(alert, key)); value != "" {
			return value
		}
	}
	return "ops-room"
}

func fallbackString(primary string, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return fallback
}
