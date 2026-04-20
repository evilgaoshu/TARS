package reasoning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/foundation/secrets"
)

type Options struct {
	Logger                     *slog.Logger
	Metrics                    *foundationmetrics.Registry
	Audit                      audit.Logger
	Protocol                   string
	BaseURL                    string
	APIKey                     string
	Model                      string
	Client                     *http.Client
	Prompts                    *PromptSet
	PromptProvider             PromptProvider
	DesensitizationProvider    DesensitizationProvider
	ProviderRegistry           ProviderRegistry
	SecretStore                *secrets.Store
	LocalCommandFallbackEnable bool
}

type Service struct {
	logger                     *slog.Logger
	metrics                    *foundationmetrics.Registry
	audit                      audit.Logger
	protocol                   string
	baseURL                    string
	apiKey                     string
	model                      string
	client                     *http.Client
	promptProvider             PromptProvider
	desensitizationProvider    DesensitizationProvider
	providerRegistry           ProviderRegistry
	secretStore                *secrets.Store
	localCommandFallbackEnable bool
	localLLMDetector           *localLLMDetector
}

type modelRuntime struct {
	Role       string
	ProviderID string
	Protocol   string
	BaseURL    string
	APIKey     string
	Model      string
}

func NewService(opts Options) *Service {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	model := opts.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &Service{
		logger:                     fallbackLogger(opts.Logger),
		metrics:                    opts.Metrics,
		audit:                      opts.Audit,
		protocol:                   normalizeModelProtocol(opts.Protocol),
		baseURL:                    strings.TrimRight(opts.BaseURL, "/"),
		apiKey:                     opts.APIKey,
		model:                      model,
		client:                     client,
		promptProvider:             selectPromptProvider(opts.PromptProvider, opts.Prompts),
		desensitizationProvider:    opts.DesensitizationProvider,
		providerRegistry:           opts.ProviderRegistry,
		secretStore:                opts.SecretStore,
		localCommandFallbackEnable: opts.LocalCommandFallbackEnable,
		localLLMDetector:           newLocalLLMDetector(opts.Logger, opts.Metrics, opts.Audit, client),
	}
}

func (s *Service) BuildDiagnosis(ctx context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	desensitizationConfig := s.currentDesensitizationConfig()
	detections := s.detectSensitiveValues(ctx, input.SessionID, input.Context, desensitizationConfig)
	sanitizedContext, desenseMap := desensitizeContextWithConfigAndDetections(input.Context, desensitizationConfig, detections)
	sanitizedInput := input
	sanitizedInput.Context = sanitizedContext

	primary, assist := s.runtimesForInput(input)
	output, usedRuntime, err := s.buildDiagnosisWithFallback(ctx, input, sanitizedInput, primary, assist)
	if err != nil {
		if s.metrics != nil {
			if strings.Contains(err.Error(), "not configured") {
				s.metrics.IncExternalProvider("model", "chat_completions", "stub")
				s.metrics.RecordComponentResult("model", "stub", err.Error())
			} else {
				s.metrics.IncExternalProvider("model", "chat_completions", "error")
				s.metrics.RecordComponentResult("model", "error", err.Error())
			}
		}
		s.logger.Warn("reasoning model request failed, falling back", "session_id", input.SessionID, "error", err)
		return s.fallbackDiagnosis(input), nil
	}
	s.logger.Info("reasoning model response accepted", "session_id", input.SessionID, "model", usedRuntime.Model, "protocol", usedRuntime.Protocol, "role", usedRuntime.Role)
	if s.metrics != nil {
		s.metrics.IncExternalProvider("model", "chat_completions", "success")
		if usedRuntime.Role == "assist" {
			s.metrics.RecordComponentResult("model", "success", "assist fallback generated diagnosis")
		} else {
			s.metrics.RecordComponentResult("model", "success", "diagnosis generated")
		}
	}
	output.Summary = rehydratePlaceholdersWithConfig(output.Summary, desenseMap, desensitizationConfig)
	if strings.TrimSpace(output.Summary) == "" {
		output.Summary = s.fallbackDiagnosis(input).Summary
	}
	output.ExecutionHint = s.sanitizeExecutionHint(input, rehydratePlaceholdersWithConfig(output.ExecutionHint, desenseMap, desensitizationConfig))
	output.ToolPlan = s.buildToolPlan(input, output)
	output.DesenseMap = desenseMap
	return output, nil
}

func (s *Service) runtimesForInput(input contracts.DiagnosisInput) (modelRuntime, *modelRuntime) {
	platformPrimary := s.currentPrimaryRuntime()
	platformAssist := s.currentAssistRuntime()
	binding := input.RoleModelBinding
	if binding == nil {
		return platformPrimary, chooseDistinctAssistRuntime(platformPrimary, platformAssist)
	}

	rolePrimary := s.runtimeForRoleBindingTarget(binding.Primary, "primary")
	roleFallback := s.runtimeForRoleBindingTarget(binding.Fallback, "fallback")

	if rolePrimary != nil {
		return *rolePrimary, chooseDistinctAssistRuntime(*rolePrimary, roleFallback, platformAssist, &platformPrimary)
	}
	if binding.InheritPlatformDefault || binding.Primary == nil {
		return platformPrimary, chooseDistinctAssistRuntime(platformPrimary, roleFallback, platformAssist)
	}
	if roleFallback != nil {
		return *roleFallback, chooseDistinctAssistRuntime(*roleFallback, platformAssist, &platformPrimary)
	}
	return platformPrimary, chooseDistinctAssistRuntime(platformPrimary, platformAssist)
}

func (s *Service) currentPrimaryRuntime() modelRuntime {
	if target := s.currentPrimaryTarget(); target != nil {
		return modelRuntime{
			Role:       "primary",
			ProviderID: strings.TrimSpace(target.ProviderID),
			Protocol:   normalizeModelProtocol(target.Protocol),
			BaseURL:    strings.TrimSpace(target.BaseURL),
			APIKey:     target.APIKey,
			Model:      strings.TrimSpace(target.Model),
		}
	}
	return modelRuntime{
		Role:       "primary",
		ProviderID: "runtime-default",
		Protocol:   normalizeModelProtocol(s.protocol),
		BaseURL:    strings.TrimSpace(s.baseURL),
		APIKey:     s.apiKey,
		Model:      strings.TrimSpace(s.model),
	}
}

func (s *Service) currentPrimaryTarget() *ModelTarget {
	if s == nil || s.providerRegistry == nil {
		return nil
	}
	return s.providerRegistry.ResolvePrimaryModelTargetWithSecrets(s.secretStore)
}

func (s *Service) currentAssistTarget() *ModelTarget {
	if s == nil || s.providerRegistry == nil {
		return nil
	}
	return s.providerRegistry.ResolveAssistModelTargetWithSecrets(s.secretStore)
}

func (s *Service) currentAssistRuntime() *modelRuntime {
	target := s.currentAssistTarget()
	if target == nil {
		return nil
	}
	return &modelRuntime{
		Role:       "assist",
		ProviderID: strings.TrimSpace(target.ProviderID),
		Protocol:   normalizeModelProtocol(target.Protocol),
		BaseURL:    strings.TrimSpace(target.BaseURL),
		APIKey:     target.APIKey,
		Model:      strings.TrimSpace(target.Model),
	}
}

func (s *Service) runtimeForRoleBindingTarget(target *contracts.RoleModelTargetBinding, role string) *modelRuntime {
	if s == nil || target == nil {
		return nil
	}
	providerID := strings.TrimSpace(target.ProviderID)
	model := strings.TrimSpace(target.Model)
	if providerID == "" || model == "" {
		return nil
	}
	snapshot := s.providerSnapshot()
	for _, entry := range snapshot.Config.Entries {
		if strings.TrimSpace(entry.ID) != providerID || !entry.Enabled {
			continue
		}
		return &modelRuntime{
			Role:       role,
			ProviderID: providerID,
			Protocol:   normalizeModelProtocol(entry.Protocol),
			BaseURL:    strings.TrimSpace(entry.BaseURL),
			APIKey:     resolveProviderAPIKey(entry, s.secretStore),
			Model:      model,
		}
	}
	return nil
}

func (s *Service) providerSnapshot() ProvidersSnapshot {
	if s == nil || s.providerRegistry == nil {
		return ProvidersSnapshot{}
	}
	return s.providerRegistry.Snapshot()
}

func chooseDistinctAssistRuntime(primary modelRuntime, candidates ...*modelRuntime) *modelRuntime {
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if strings.TrimSpace(candidate.BaseURL) == "" || strings.TrimSpace(candidate.Model) == "" {
			continue
		}
		if sameModelRuntime(primary, *candidate) {
			continue
		}
		assist := *candidate
		return &assist
	}
	return nil
}

func (s *Service) buildDiagnosisWithFallback(
	ctx context.Context,
	rawInput contracts.DiagnosisInput,
	sentInput contracts.DiagnosisInput,
	primary modelRuntime,
	assist *modelRuntime,
) (contracts.DiagnosisOutput, modelRuntime, error) {
	if strings.TrimSpace(primary.BaseURL) != "" {
		output, err := s.buildFromModel(ctx, rawInput, sentInput, primary.Protocol, primary.BaseURL, primary.APIKey, primary.Model)
		if err == nil {
			s.recordModelAttempt(primary, "success", "diagnosis generated")
			return output, primary, nil
		}
		s.recordModelAttempt(primary, "error", err.Error())
		if canUseAssistFallback(primary, assist) {
			s.logger.Warn("reasoning primary model failed, trying assist model", "session_id", rawInput.SessionID, "error", err, "primary_model", primary.Model, "assist_model", assist.Model)
			s.auditModelFailover(ctx, rawInput.SessionID, primary, assist, err.Error())
			if output, assistErr := s.buildFromModel(ctx, rawInput, sentInput, assist.Protocol, assist.BaseURL, assist.APIKey, assist.Model); assistErr == nil {
				s.recordModelAttempt(*assist, "success", "assist fallback diagnosis generated")
				return output, *assist, nil
			} else {
				s.recordModelAttempt(*assist, "error", assistErr.Error())
				return contracts.DiagnosisOutput{}, primary, fmt.Errorf("primary model failed: %w; assist model failed: %v", err, assistErr)
			}
		}
		return contracts.DiagnosisOutput{}, primary, err
	}

	if canUseAssistFallback(primary, assist) {
		reason := "primary model not configured"
		s.auditModelFailover(ctx, rawInput.SessionID, primary, assist, reason)
		output, err := s.buildFromModel(ctx, rawInput, sentInput, assist.Protocol, assist.BaseURL, assist.APIKey, assist.Model)
		if err == nil {
			s.recordModelAttempt(*assist, "success", "assist fallback diagnosis generated")
			return output, *assist, nil
		}
		s.recordModelAttempt(*assist, "error", err.Error())
		return contracts.DiagnosisOutput{}, primary, fmt.Errorf("%s; assist model failed: %v", reason, err)
	}

	return contracts.DiagnosisOutput{}, primary, fmt.Errorf("model base url not configured")
}

func canUseAssistFallback(primary modelRuntime, assist *modelRuntime) bool {
	if assist == nil {
		return false
	}
	if strings.TrimSpace(assist.BaseURL) == "" || strings.TrimSpace(assist.Model) == "" {
		return false
	}
	return !sameModelRuntime(primary, *assist)
}

func sameModelRuntime(left modelRuntime, right modelRuntime) bool {
	return normalizeModelProtocol(left.Protocol) == normalizeModelProtocol(right.Protocol) &&
		strings.TrimSpace(left.BaseURL) == strings.TrimSpace(right.BaseURL) &&
		strings.TrimSpace(left.Model) == strings.TrimSpace(right.Model)
}

func (s *Service) recordModelAttempt(runtime modelRuntime, result string, detail string) {
	if s == nil || s.metrics == nil {
		return
	}
	providerLabel := "model_" + fallback(strings.TrimSpace(runtime.Role), "primary")
	s.metrics.IncExternalProvider(providerLabel, "chat_completions", result)
	s.metrics.RecordComponentResult(providerLabel, result, detail)
}

func (s *Service) auditModelFailover(ctx context.Context, sessionID string, primary modelRuntime, assist *modelRuntime, reason string) {
	if s == nil || s.audit == nil || strings.TrimSpace(sessionID) == "" || assist == nil {
		return
	}
	s.audit.Log(ctx, audit.Entry{
		ResourceType: "llm_request",
		ResourceID:   sessionID,
		Action:       "chat_completions_failover",
		Actor:        "tars_reasoning",
		Metadata: map[string]any{
			"session_id":       sessionID,
			"from_role":        primary.Role,
			"from_provider_id": primary.ProviderID,
			"from_protocol":    primary.Protocol,
			"from_model":       primary.Model,
			"to_role":          assist.Role,
			"to_provider_id":   assist.ProviderID,
			"to_protocol":      assist.Protocol,
			"to_model":         assist.Model,
			"reason":           strings.TrimSpace(reason),
		},
	})
}

func (s *Service) buildFromModel(ctx context.Context, rawInput contracts.DiagnosisInput, sentInput contracts.DiagnosisInput, protocol string, baseURL string, apiKey string, model string) (contracts.DiagnosisOutput, error) {
	prompts := s.currentPromptSet()
	systemPrompt := prompts.SystemPrompt
	// Prepend agent role system prompt if present in context.
	if rolePrompt, ok := rawInput.Context["agent_role_system_prompt"].(string); ok && strings.TrimSpace(rolePrompt) != "" {
		systemPrompt = strings.TrimSpace(rolePrompt) + "\n\n" + systemPrompt
	}
	rawUserPrompt, err := prompts.RenderUserPrompt(rawInput.SessionID, rawInput.Context)
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}
	sentUserPrompt, err := prompts.RenderUserPrompt(sentInput.SessionID, sentInput.Context)
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}

	rawInvocation, err := buildModelInvocation(protocol, baseURL, apiKey, model, systemPrompt, rawUserPrompt)
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}
	sentInvocation, err := buildModelInvocation(protocol, baseURL, apiKey, model, systemPrompt, sentUserPrompt)
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}

	requestBody, err := json.Marshal(sentInvocation.Payload)
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}
	s.auditModelRequest(ctx, rawInput.SessionID, "diagnosis", systemPrompt, rawUserPrompt, sentUserPrompt, rawInput.Context, sentInput.Context, rawInvocation, sentInvocation, model, protocol)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sentInvocation.Endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}
	for key, value := range sentInvocation.Headers {
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}
	defer resp.Body.Close()

	var output struct {
		Summary       string `json:"summary"`
		ExecutionHint string `json:"execution_hint"`
	}
	content, err := extractModelResponseContent(protocol, resp.StatusCode, resp.Body)
	if err != nil {
		return contracts.DiagnosisOutput{}, err
	}
	if err := decodeDiagnosisJSON(content, &output); err != nil {
		return contracts.DiagnosisOutput{}, err
	}

	return contracts.DiagnosisOutput{
		Summary:       output.Summary,
		ExecutionHint: output.ExecutionHint,
	}, nil
}

func (s *Service) auditModelRequest(
	ctx context.Context,
	sessionID string,
	phase string,
	systemPrompt string,
	rawUserPrompt string,
	sentUserPrompt string,
	rawContext map[string]interface{},
	sentContext map[string]interface{},
	rawInvocation modelInvocation,
	sentInvocation modelInvocation,
	model string,
	protocol string,
) {
	if s == nil || s.audit == nil || strings.TrimSpace(sessionID) == "" {
		return
	}

	s.audit.Log(ctx, audit.Entry{
		ResourceType: "llm_request",
		ResourceID:   sessionID,
		Action:       "chat_completions_send",
		Actor:        "tars_reasoning",
		Metadata: map[string]any{
			"session_id":        sessionID,
			"provider":          protocol,
			"model":             model,
			"phase":             phase,
			"endpoint":          sentInvocation.Endpoint,
			"system_prompt_raw": systemPrompt,
			"user_prompt_raw":   rawUserPrompt,
			"user_prompt_sent":  sentUserPrompt,
			"context_raw":       cloneInterfaceMap(rawContext),
			"context_sent":      cloneInterfaceMap(sentContext),
			"request_raw":       rawInvocation.Payload,
			"request_sent":      sentInvocation.Payload,
			"contains_raw":      true,
			"contains_sent":     true,
		},
	})
}

func (s *Service) fallbackDiagnosis(input contracts.DiagnosisInput) contracts.DiagnosisOutput {
	alertName := stringFromContext(input.Context, "alert_name")
	host := stringFromContext(input.Context, "host")
	service := stringFromContext(input.Context, "service")
	severity := stringFromContext(input.Context, "severity")
	userRequest := stringFromContext(input.Context, "user_request")

	summary := fmt.Sprintf("diagnosis ready for session %s", input.SessionID)
	if alertName != "" || host != "" {
		summary = fmt.Sprintf("diagnosis ready: %s on %s", fallback(alertName, "unknown-alert"), fallback(host, "unknown-host"))
	}
	if strings.TrimSpace(userRequest) != "" {
		summary = fmt.Sprintf("已分析请求：%s", userRequest)
		if host != "" {
			summary += fmt.Sprintf("（目标主机 %s）", host)
		}
	}
	if severity != "" {
		summary += fmt.Sprintf(" (severity=%s)", severity)
	}

	readOnlyPlan := fallbackReadOnlyToolPlan(input)
	executionHint := ""
	if s.localCommandFallbackEnable && host != "" {
		if candidate := buildDirectExecutionHintForUserRequest(userRequest, service); candidate != "" {
			executionHint = candidate
		}
	}

	toolPlan := s.buildToolPlan(input, contracts.DiagnosisOutput{Summary: summary, ExecutionHint: executionHint})
	if executionHint == "" && len(readOnlyPlan) > 0 {
		toolPlan = readOnlyPlan
	}

	return contracts.DiagnosisOutput{
		Summary:       summary,
		Citations:     nil,
		ExecutionHint: executionHint,
		ToolPlan:      toolPlan,
	}
}

func buildDirectExecutionHintForUserRequest(userRequest string, service string) string {
	lower := strings.ToLower(strings.TrimSpace(userRequest))
	if lower == "" {
		return ""
	}

	switch {
	case strings.Contains(lower, "public ip"), strings.Contains(lower, "egress ip"), strings.Contains(lower, "exit ip"),
		strings.Contains(lower, "出口ip"), strings.Contains(lower, "公网ip"), strings.Contains(lower, "外网ip"):
		return "curl -fsS https://api.ipify.org && echo"
	case service != "" && (strings.Contains(lower, "restart") || strings.Contains(userRequest, "重启")):
		return fmt.Sprintf("systemctl restart %s", service)
	case service != "" && (strings.Contains(lower, "stop") || strings.Contains(userRequest, "停止")):
		return fmt.Sprintf("systemctl stop %s", service)
	case service != "" && (strings.Contains(lower, "status") || strings.Contains(userRequest, "状态")) && (strings.Contains(lower, "run") || strings.Contains(userRequest, "执行") || strings.Contains(userRequest, "命令")):
		return fmt.Sprintf("systemctl status %s --no-pager --lines=20 || true", service)
	default:
		return ""
	}
}

func (s *Service) buildToolPlan(input contracts.DiagnosisInput, output contracts.DiagnosisOutput) []contracts.ToolPlanStep {
	steps := make([]contracts.ToolPlanStep, 0, 3)
	steps = append(steps, contracts.ToolPlanStep{
		Tool:     "metrics.query_range",
		Reason:   "Inspect the past 1h resource trend before proposing action.",
		Priority: 1,
		Status:   "completed",
		Input: map[string]interface{}{
			"query":  stringFromContext(input.Context, "metrics_query"),
			"window": fallback(stringFromContext(input.Context, "metrics_query_window"), "1h"),
			"mode":   fallback(stringFromContext(input.Context, "metrics_query_mode"), "range"),
		},
		Output: map[string]interface{}{
			"series_count": len(interfaceSliceFromContext(input.Context, "metrics_series")),
		},
	})
	steps = append(steps, contracts.ToolPlanStep{
		Tool:     "knowledge.search",
		Reason:   "Check existing operational knowledge before suggesting execution.",
		Priority: 2,
		Status:   "completed",
		Output: map[string]interface{}{
			"hits": intFromContext(input.Context, "knowledge_hits"),
		},
	})
	if strings.TrimSpace(output.ExecutionHint) != "" {
		steps = append(steps, contracts.ToolPlanStep{
			Tool:     "execution.run_command",
			Reason:   "Only propose controlled execution when explicitly requested; after running, verify service state and re-check the relevant evidence.",
			Priority: 3,
			Status:   "planned",
			OnPendingApproval: "stop",
			OnDenied:          "stop",
			Input: map[string]interface{}{
				"command": output.ExecutionHint,
			},
		})
	}
	return steps
}

func (s *Service) detectSensitiveValues(ctx context.Context, sessionID string, input map[string]interface{}, cfg *DesensitizationConfig) *SensitiveDetections {
	if s == nil || s.localLLMDetector == nil || cfg == nil {
		return nil
	}
	assist := cfg.LocalLLMAssist
	if !assist.Enabled {
		return nil
	}
	assistAPIKey := ""
	if target := s.currentAssistTarget(); target != nil {
		assist.Provider = target.Protocol
		assist.BaseURL = target.BaseURL
		assist.Model = target.Model
		assistAPIKey = target.APIKey
		if strings.TrimSpace(assist.Mode) == "" {
			assist.Mode = "detect_only"
		}
	}
	detections, err := s.localLLMDetector.DetectSensitiveValues(ctx, sessionID, input, assist, assistAPIKey)
	if err != nil {
		s.logger.Warn("local llm desensitization assist failed, continuing with rule-based desensitization", "session_id", sessionID, "error", err)
		return nil
	}
	return detections
}

func (s *Service) currentDesensitizationConfig() *DesensitizationConfig {
	if s == nil || s.desensitizationProvider == nil {
		cfg := DefaultDesensitizationConfig()
		return &cfg
	}
	return s.desensitizationProvider.CurrentDesensitizationConfig()
}

func buildExecutionHintForUserRequest(userRequest string, service string) string {
	lower := strings.ToLower(strings.TrimSpace(userRequest))
	if lower == "" {
		return ""
	}

	switch {
	case strings.Contains(lower, "load"), strings.Contains(lower, "uptime"), strings.Contains(userRequest, "负载"):
		return "uptime && cat /proc/loadavg"
	case strings.Contains(lower, "public ip"), strings.Contains(lower, "egress ip"), strings.Contains(lower, "exit ip"),
		strings.Contains(lower, "出口ip"), strings.Contains(lower, "公网ip"), strings.Contains(lower, "外网ip"):
		return "curl -fsS https://api.ipify.org && echo"
	case strings.Contains(lower, "memory"), strings.Contains(userRequest, "内存"):
		return "free -m"
	case strings.Contains(lower, "disk"), strings.Contains(userRequest, "磁盘"), strings.Contains(lower, "storage"):
		return "df -h"
	case strings.Contains(lower, "port"), strings.Contains(userRequest, "端口"), strings.Contains(lower, "socket"), strings.Contains(userRequest, "连接"):
		return "ss -lntp"
	case strings.Contains(lower, "cpu"), strings.Contains(userRequest, "进程"), strings.Contains(lower, "process"):
		return "ps aux --sort=-%cpu | head"
	case service != "" && (strings.Contains(lower, "restart") || strings.Contains(userRequest, "重启")):
		return fmt.Sprintf("systemctl restart %s", service)
	case service != "" && (strings.Contains(lower, "stop") || strings.Contains(userRequest, "停止")):
		return fmt.Sprintf("systemctl stop %s", service)
	case service != "" && (strings.Contains(lower, "status") || strings.Contains(userRequest, "状态") || strings.Contains(lower, "service")):
		return fmt.Sprintf("systemctl status %s --no-pager --lines=20 || true", service)
	}
	return ""
}

func stringFromContext(ctx map[string]interface{}, key string) string {
	if value, ok := ctx[key].(string); ok {
		return value
	}
	return ""
}

func intFromContext(ctx map[string]interface{}, key string) int {
	value, ok := ctx[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func interfaceSliceFromContext(ctx map[string]interface{}, key string) []interface{} {
	value, ok := ctx[key]
	if !ok {
		return nil
	}
	if items, ok := value.([]interface{}); ok {
		return items
	}
	return nil
}

func fallback(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func buildExecutionHint(service string) string {
	command := "hostname && uptime"
	if service != "" {
		command += fmt.Sprintf(" && systemctl status %s --no-pager --lines=20 || true", service)
	}
	return command
}

func cloneInterfaceMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]interface{}, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func (s *Service) sanitizeExecutionHint(input contracts.DiagnosisInput, candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"metrics.query_", "logs.query", "observability.query", "delivery.query", "connector.invoke_capability", "knowledge.search"} {
		if strings.HasPrefix(lower, prefix) {
			return ""
		}
	}
	for _, fragment := range []string{"[redacted]", "password=", "passwd=", "token=", "secret=", "api_key=", "apikey=", "access_token=", "refresh_token=", "authorization:", "bearer "} {
		if strings.Contains(lower, fragment) {
			return ""
		}
	}
	if executionHintExplicitlyAllowed(input, trimmed) {
		return trimmed
	}
	if toolEvidenceMakesExecutionHintRedundant(input.Context) {
		return ""
	}
	if isGenericReadOnlyExecutionHint(trimmed) {
		return ""
	}
	for _, fragment := range []string{"ssh ", "sudo ", "\n", "\r", "```", "investigate ", "consider ", "check ", "--connector="} {
		if strings.Contains(lower, fragment) {
			return ""
		}
	}
	return trimmed
}

func executionHintExplicitlyAllowed(input contracts.DiagnosisInput, candidate string) bool {
	userRequest := stringFromContext(input.Context, "user_request")
	service := stringFromContext(input.Context, "service")
	if expected := strings.TrimSpace(buildDirectExecutionHintForUserRequest(userRequest, service)); expected != "" && strings.TrimSpace(candidate) == expected {
		return true
	}
	return explicitlyRequestsHostLevelActionForReasoning(userRequest, candidate)
}

func isGenericReadOnlyExecutionHint(candidate string) bool {
	switch classifyExecutionHintIntent(candidate) {
	case "generic_host", "host_metrics", "endpoint_probe":
		return true
	default:
		return false
	}
}

func classifyExecutionHintIntent(candidate string) string {
	lower := strings.ToLower(strings.TrimSpace(candidate))
	switch {
	case lower == "hostname && uptime":
		return "generic_host"
	case (strings.HasPrefix(lower, "curl ") || strings.HasPrefix(lower, "wget ")) &&
		(strings.Contains(lower, "http://") || strings.Contains(lower, "https://")):
		return "endpoint_probe"
	case strings.Contains(lower, "uptime") || strings.Contains(lower, "/proc/loadavg") || strings.Contains(lower, "free -m") || strings.Contains(lower, "df -h") || strings.Contains(lower, "ss -lntp") || strings.Contains(lower, "ps aux"):
		return "host_metrics"
	case strings.Contains(lower, "systemctl status") || strings.Contains(lower, "journalctl"):
		return "service_status"
	default:
		return "generic"
	}
}

func explicitlyRequestsHostLevelActionForReasoning(userRequest string, executionHint string) bool {
	lower := strings.ToLower(strings.TrimSpace(userRequest))
	for _, fragment := range []string{
		"是否上机", "是否上机器", "要不要上机", "要不要上机器", "是否需要上机", "是否需要上机器",
		"再判断是否上机", "再判断是否上机器", "先看", "先查",
	} {
		if strings.Contains(lower, fragment) {
			return false
		}
	}
	for _, fragment := range []string{
		"执行", "命令", "run ", "ssh ", "shell", "登录", "登陆", "进机器", "上机器", "上主机", "/proc", "systemctl", "journalctl",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	if strings.Contains(lower, "状态") && strings.Contains(strings.ToLower(executionHint), "systemctl status") {
		return true
	}
	return false
}

func toolEvidenceMakesExecutionHintRedundant(ctx map[string]interface{}) bool {
	if len(ctx) == 0 {
		return false
	}
	if hasSufficientReasoningEvidence(ctx["logs_query_result"]) || hasSufficientReasoningEvidence(ctx["observability_query_result"]) || hasSufficientReasoningEvidence(ctx["delivery_query_result"]) {
		return true
	}
	if hasSufficientReasoningEvidence(ctx["knowledge_hits"]) {
		return true
	}
	if hasSufficientReasoningEvidence(ctx["metrics_series"]) {
		return true
	}
	if raw, ok := ctx["tool_results"].([]interface{}); ok {
		for _, item := range raw {
			if result, ok := item.(map[string]interface{}); ok && strings.EqualFold(strings.TrimSpace(interfaceStringFromAny(result["status"])), "completed") {
				if hasSufficientReasoningEvidence(result["output"]) {
					return true
				}
			}
		}
	}
	return false
}

func hasSufficientReasoningEvidence(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case []interface{}:
		return len(typed) > 0
	case []map[string]interface{}:
		return len(typed) > 0
	case map[string]interface{}:
		if len(typed) == 0 {
			return false
		}
		for _, key := range []string{"result", "results", "commits", "series", "summary", "hit_count", "result_count", "artifact_count", "points", "release", "branch"} {
			if field, ok := typed[key]; ok && hasSufficientReasoningEvidence(field) {
				return true
			}
		}
		for _, value := range typed {
			if hasSufficientReasoningEvidence(value) {
				return true
			}
		}
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case int:
		return typed > 0
	case int64:
		return typed > 0
	case float64:
		return typed > 0
	case bool:
		return typed
	default:
		return true
	}
}

func interfaceStringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func decodeDiagnosisJSON(content string, target interface{}) error {
	trimmed := strings.TrimSpace(content)
	if err := json.Unmarshal([]byte(trimmed), target); err == nil {
		return nil
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return fmt.Errorf("model response does not contain a json object")
	}
	return json.Unmarshal([]byte(trimmed[start:end+1]), target)
}

func fallbackLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}

func defaultPromptSet(prompts *PromptSet) *PromptSet {
	if prompts != nil {
		return prompts
	}
	return DefaultPromptSet()
}

func selectPromptProvider(provider PromptProvider, prompts *PromptSet) PromptProvider {
	if provider != nil {
		return provider
	}
	return defaultPromptSet(prompts)
}

func (s *Service) currentPromptSet() *PromptSet {
	if s == nil || s.promptProvider == nil {
		return DefaultPromptSet()
	}
	return defaultPromptSet(s.promptProvider.CurrentPromptSet())
}
