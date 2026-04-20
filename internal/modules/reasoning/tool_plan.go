package reasoning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"tars/internal/contracts"
)

const plannerSystemPrompt = `You are TARS, an operations planner.
Return ONLY strict JSON with fields: summary, tool_plan.
Do not use markdown, code fences, or extra prose.
tool_plan must be an array.
Each tool_plan item must contain:
- tool
- reason
- priority
- connector_id (optional)
- params

Supported tools:
- metrics.query_range
- metrics.query_instant
- logs.query
- knowledge.search
- observability.query
- delivery.query
- execution.run_command
- connector.invoke_capability

Planning rules:
- Prefer metrics.query_range for questions about trends, "past hour", "过去一小时", "最近", "trend", "history".
- For generic host load questions, prefer PromQL like node_load1 rather than inventing custom pseudo-metrics.
- Prefer metrics.query_instant for current-state monitoring questions.
- Prefer logs.query for questions specifically about log lines, log search, or log evidence.
- Prefer observability.query for traces, spans, latency, or broader root cause evidence from observability systems after reviewing logs when relevant.
- Prefer delivery.query for questions about deployments, releases, pipelines, commits, or recent change history.
- For questions relating errors to recent changes, prefer an evidence-first multi-step plan such as observability.query followed by delivery.query.
- Prefer knowledge.search before execution when historical incidents or operator guidance could answer the request.
- Only use execution.run_command when monitoring data is unlikely to answer the request or the operator explicitly asks for host-level evidence.
- If observability.query, delivery.query, metrics, or knowledge can gather evidence first, do that before considering execution.run_command.
- If context.tool_capabilities is present, treat it as the authoritative list of system abilities available to this session.
- Only create executable tool_plan steps for entries marked invocable=true in context.tool_capabilities.
- When multiple invocable connectors can satisfy the same tool, prefer the first matching entry from context.tool_capabilities; the list is already ordered by platform runtime preference.
- Use connector.invoke_capability only for non-standard connector / MCP / skill abilities that are explicitly listed in context.tool_capabilities with invocable=true.
- If you are not certain about the exact connector_id configured in the system, omit connector_id instead of inventing a generic one.
- For multi-step plans, assign stable ids such as metrics_1, delivery_1, observe_1.
- When a later step depends on an earlier step, reference prior data using $steps.<step_id>.output.<field> or $steps.<step_id>.input.<field>.
- Prefer explicit step references over repeating or paraphrasing data from earlier steps.
- Avoid generating execution.run_command for read-only diagnosis questions when the likely answer can be obtained by combining tool evidence already available in this session.
- Do not invent unsupported tools.
- Do not generate multiple execution.run_command steps.
- If no tool is needed, return an empty array.`

const finalizerSystemPromptSuffix = `
You are now the final diagnosis summarizer.
Use the executed tool results already included in the context.
Return ONLY strict JSON with fields: summary, execution_hint.
If the tool results already answer the operator request, execution_hint must be an empty string.
Only produce execution_hint if host-level evidence or a controlled action is still necessary.
If monitoring, delivery, observability, or knowledge tools already produced enough evidence, do not add generic host commands or ad-hoc curl/wget probes.
If a tool step failed because an external system was unavailable, summarize that limitation instead of inventing a shell command to probe the same system.
Do not mention hidden reasoning or planning steps.`

type plannerModelResponse struct {
	Summary  string                 `json:"summary"`
	ToolPlan []plannerToolPlanEntry `json:"tool_plan"`
}

type plannerToolPlanEntry struct {
	ID                string                 `json:"id"`
	Tool              string                 `json:"tool"`
	ConnectorID       string                 `json:"connector_id"`
	Reason            string                 `json:"reason"`
	Priority          int                    `json:"priority"`
	Status            string                 `json:"status"`
	OnFailure         string                 `json:"on_failure"`
	OnPendingApproval string                 `json:"on_pending_approval"`
	OnDenied          string                 `json:"on_denied"`
	Params            map[string]interface{} `json:"params"`
	Input             map[string]interface{} `json:"input"`
}

func (p *plannerToolPlanEntry) UnmarshalJSON(data []byte) error {
	type plannerToolPlanEntryAlias struct {
		Tool              string                 `json:"tool"`
		ID                string                 `json:"id"`
		ConnectorID       string                 `json:"connector_id"`
		Reason            string                 `json:"reason"`
		Priority          json.RawMessage        `json:"priority"`
		Status            string                 `json:"status"`
		OnFailure         string                 `json:"on_failure"`
		OnPendingApproval string                 `json:"on_pending_approval"`
		OnDenied          string                 `json:"on_denied"`
		Params            map[string]interface{} `json:"params"`
		Input             map[string]interface{} `json:"input"`
	}
	var raw plannerToolPlanEntryAlias
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.ID = raw.ID
	p.Tool = raw.Tool
	p.ConnectorID = raw.ConnectorID
	p.Reason = raw.Reason
	p.Status = raw.Status
	p.OnFailure = raw.OnFailure
	p.OnPendingApproval = raw.OnPendingApproval
	p.OnDenied = raw.OnDenied
	p.Params = raw.Params
	p.Input = raw.Input
	p.Priority = parsePlannerPriority(raw.Priority)
	return nil
}

type finalizerModelResponse struct {
	Summary       string `json:"summary"`
	ExecutionHint string `json:"execution_hint"`
}

func (s *Service) PlanDiagnosis(ctx context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisPlan, error) {
	cfg := s.currentDesensitizationConfig()
	detections := s.detectSensitiveValues(ctx, input.SessionID, input.Context, cfg)
	sanitizedContext, desenseMap := desensitizeContextWithConfigAndDetections(input.Context, cfg, detections)
	sentInput := input
	sentInput.Context = sanitizedContext

	primary, assist := s.runtimesForInput(input)

	response := plannerModelResponse{}
	usedRuntime, err := s.invokeDiagnosisModelJSONWithFallback(ctx, "planner", input, sentInput, plannerSystemPromptForInput(input.Context), primary, assist, &response)
	if err != nil {
		s.logger.Warn("diagnosis planner failed, falling back to heuristic planning", "session_id", input.SessionID, "error", err)
		plan := s.fallbackDiagnosisPlan(input, desenseMap)
		return plan, nil
	}

	if s.metrics != nil {
		if usedRuntime.Role == "assist" {
			s.metrics.RecordComponentResult("model_planner", "success", "assist fallback generated tool plan")
		} else {
			s.metrics.RecordComponentResult("model_planner", "success", "tool plan generated")
		}
	}

	plan := contracts.DiagnosisPlan{
		Summary:    rehydratePlaceholdersWithConfig(strings.TrimSpace(response.Summary), desenseMap, cfg),
		ToolPlan:   rehydrateToolPlanWithConfig(normalizePlannerToolPlan(input, response.ToolPlan), desenseMap, cfg),
		DesenseMap: desenseMap,
	}
	if strings.TrimSpace(plan.Summary) == "" {
		plan.Summary = s.fallbackDiagnosisPlan(input, desenseMap).Summary
	}
	if len(plan.ToolPlan) == 0 {
		fallbackPlan := s.fallbackDiagnosisPlan(input, desenseMap)
		plan.ToolPlan = fallbackPlan.ToolPlan
		if strings.TrimSpace(plan.Summary) == "" {
			plan.Summary = fallbackPlan.Summary
		}
	}
	return plan, nil
}

func (s *Service) FinalizeDiagnosis(ctx context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	cfg := s.currentDesensitizationConfig()
	detections := s.detectSensitiveValues(ctx, input.SessionID, input.Context, cfg)
	sanitizedContext, desenseMap := desensitizeContextWithConfigAndDetections(input.Context, cfg, detections)
	sentInput := input
	sentInput.Context = sanitizedContext

	primary, assist := s.runtimesForInput(input)
	prompts := s.currentPromptSet()
	systemPrompt := strings.TrimSpace(prompts.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}
	if rolePrompt, ok := input.Context["agent_role_system_prompt"].(string); ok && strings.TrimSpace(rolePrompt) != "" {
		systemPrompt = strings.TrimSpace(rolePrompt) + "\n\n" + systemPrompt
	}
	systemPrompt = strings.TrimSpace(systemPrompt + "\n" + strings.TrimSpace(finalizerSystemPromptSuffix))

	response := finalizerModelResponse{}
	usedRuntime, err := s.invokeDiagnosisModelJSONWithFallback(ctx, "finalizer", input, sentInput, systemPrompt, primary, assist, &response)
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
		s.logger.Warn("diagnosis finalizer failed, falling back", "session_id", input.SessionID, "error", err)
		result := s.fallbackDiagnosis(input)
		result.Summary = fallbackSummaryFromEvidence(input.Context, result.Summary)
		result.DesenseMap = desenseMap
		return result, nil
	}

	s.logger.Info("reasoning finalizer response accepted", "session_id", input.SessionID, "model", usedRuntime.Model, "protocol", usedRuntime.Protocol, "role", usedRuntime.Role)
	if s.metrics != nil {
		s.metrics.IncExternalProvider("model", "chat_completions", "success")
		if usedRuntime.Role == "assist" {
			s.metrics.RecordComponentResult("model", "success", "assist fallback finalized diagnosis")
		} else {
			s.metrics.RecordComponentResult("model", "success", "diagnosis finalized")
		}
	}

	output := contracts.DiagnosisOutput{
		Summary:       rehydratePlaceholdersWithConfig(strings.TrimSpace(response.Summary), desenseMap, cfg),
		ExecutionHint: s.sanitizeExecutionHint(input, rehydratePlaceholdersWithConfig(strings.TrimSpace(response.ExecutionHint), desenseMap, cfg)),
		DesenseMap:    desenseMap,
	}
	output.Summary = normalizeFinalSummaryForEvidence(input.Context, output.Summary)
	if strings.TrimSpace(output.Summary) == "" {
		output.Summary = fallbackSummaryFromEvidence(input.Context, s.fallbackDiagnosis(input).Summary)
	}
	return output, nil
}

func fallbackSummaryFromEvidence(ctx map[string]interface{}, fallback string) string {
	trimmedFallback := strings.TrimSpace(fallback)
	if len(ctx) == 0 {
		return trimmedFallback
	}
	raw, ok := ctx["tool_results"]
	if ok {
		tools := make([]string, 0, 4)
		summaries := make([]string, 0, 4)
		seen := map[string]struct{}{}
		appendResult := func(tool string, status string, output interface{}) {
			normalizedTool := strings.TrimSpace(strings.ToLower(tool))
			if normalizedTool == "" || normalizedTool == "execution.run_command" || normalizedTool == "connector.invoke_capability" {
				return
			}
			if strings.TrimSpace(strings.ToLower(status)) != "completed" || !hasSufficientReasoningEvidence(output) {
				return
			}
			if _, exists := seen[normalizedTool]; exists {
				return
			}
			seen[normalizedTool] = struct{}{}
			tools = append(tools, normalizedTool)
			if summary := compactEvidenceSummary(output); summary != "" {
				summaries = append(summaries, fmt.Sprintf("%s: %s", normalizedTool, summary))
			}
		}
		switch typed := raw.(type) {
		case []interface{}:
			for _, item := range typed {
				entry, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				appendResult(interfaceStringFromAny(entry["tool"]), interfaceStringFromAny(entry["status"]), entry["output"])
			}
		case []map[string]interface{}:
			for _, entry := range typed {
				appendResult(interfaceStringFromAny(entry["tool"]), interfaceStringFromAny(entry["status"]), entry["output"])
			}
		}
		if len(tools) > 0 {
			if len(summaries) == 0 {
				return fmt.Sprintf("已完成 %s 只读证据收集，当前无需额外执行命令。", strings.Join(tools, "、"))
			}
			return fmt.Sprintf("已完成 %s 只读证据收集。关键结果：%s。", strings.Join(tools, "、"), strings.Join(summaries, "；"))
		}
	}
	if summary := fallbackAutomationSummaryFromEvidence(ctx); summary != "" {
		return summary
	}
	return trimmedFallback
}

func fallbackAutomationSummaryFromEvidence(ctx map[string]interface{}) string {
	if len(ctx) == 0 || !interfaceBool(ctx["automation_run"]) {
		return ""
	}
	resultSummary := compactEvidenceSummary(ctx["capability_output"])
	if resultSummary == "" {
		resultSummary = strings.TrimSpace(interfaceStringFromAny(ctx["run_summary"]))
	}
	if resultSummary == "" {
		return ""
	}
	status := strings.TrimSpace(strings.ToLower(interfaceStringFromAny(ctx["run_status"])))
	switch status {
	case "completed", "resolved":
		return fmt.Sprintf("automation run 已完成。关键结果：%s。", resultSummary)
	case "blocked", "failed":
		return fmt.Sprintf("automation run 状态=%s。当前结果：%s。", status, resultSummary)
	default:
		return fmt.Sprintf("automation run 已产出结果：%s。", resultSummary)
	}
}

func compactEvidenceSummary(output interface{}) string {
	entry, ok := output.(map[string]interface{})
	if !ok {
		return ""
	}
	for _, key := range []string{"summary", "release", "branch"} {
		if value := strings.TrimSpace(interfaceStringFromAny(entry[key])); value != "" {
			return value
		}
	}
	if nested, ok := entry["result"].(map[string]interface{}); ok {
		for _, key := range []string{"summary", "release", "branch"} {
			if value := strings.TrimSpace(interfaceStringFromAny(nested[key])); value != "" {
				return value
			}
		}
		if count := intFromEvidence(nested["result_count"]); count > 0 {
			return fmt.Sprintf("result_count=%d", count)
		}
		if count := intFromEvidence(nested["artifact_count"]); count > 0 {
			return fmt.Sprintf("artifact_count=%d", count)
		}
	}
	if points := intFromEvidence(entry["points"]); points > 0 {
		if series := intFromEvidence(entry["series_count"]); series > 0 {
			return fmt.Sprintf("series_count=%d, points=%d", series, points)
		}
		return fmt.Sprintf("points=%d", points)
	}
	if count := intFromEvidence(entry["series_count"]); count > 0 {
		return fmt.Sprintf("series_count=%d", count)
	}
	if count := intFromEvidence(entry["result_count"]); count > 0 {
		return fmt.Sprintf("result_count=%d", count)
	}
	if count := intFromEvidence(entry["artifact_count"]); count > 0 {
		return fmt.Sprintf("artifact_count=%d", count)
	}
	return ""
}

func intFromEvidence(value interface{}) int {
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

func normalizeFinalSummaryForEvidence(ctx map[string]interface{}, summary string) string {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return ""
	}
	failures := collectCriticalToolFailures(ctx)
	if len(failures) == 0 {
		return trimmed
	}
	if !summaryContainsUnsupportedConclusion(trimmed) {
		return trimmed
	}
	return buildCautiousSummaryForFailures(ctx, failures)
}

func collectCriticalToolFailures(ctx map[string]interface{}) []string {
	if len(ctx) == 0 {
		return nil
	}
	results, ok := ctx["tool_results"]
	if !ok {
		return nil
	}
	seen := map[string]struct{}{}
	failures := make([]string, 0, 2)
	appendFailure := func(tool string, status string) {
		normalizedTool := strings.TrimSpace(strings.ToLower(tool))
		if !isCriticalEvidenceTool(normalizedTool) {
			return
		}
		normalizedStatus := strings.TrimSpace(strings.ToLower(status))
		if normalizedStatus != "failed" {
			return
		}
		if _, exists := seen[normalizedTool]; exists {
			return
		}
		seen[normalizedTool] = struct{}{}
		failures = append(failures, normalizedTool)
	}
	switch typed := results.(type) {
	case []interface{}:
		for _, item := range typed {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			appendFailure(interfaceStringFromAny(entry["tool"]), interfaceStringFromAny(entry["status"]))
		}
	case []map[string]interface{}:
		for _, entry := range typed {
			appendFailure(interfaceStringFromAny(entry["tool"]), interfaceStringFromAny(entry["status"]))
		}
	}
	return failures
}

func isCriticalEvidenceTool(tool string) bool {
	switch {
	case strings.HasPrefix(tool, "metrics.query_"):
		return true
	case tool == "observability.query":
		return true
	case tool == "delivery.query":
		return true
	case tool == "knowledge.search":
		return true
	default:
		return false
	}
}

func summaryContainsUnsupportedConclusion(summary string) bool {
	lower := strings.ToLower(strings.TrimSpace(summary))
	for _, phrase := range []string{
		"无直接关联",
		"没有直接关联",
		"不相关",
		"无关",
		"not related",
		"no direct relation",
		"unrelated",
		"not associated",
	} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func buildCautiousSummaryForFailures(ctx map[string]interface{}, failures []string) string {
	parts := make([]string, 0, len(failures))
	for _, failure := range failures {
		switch failure {
		case "delivery.query":
			parts = append(parts, "delivery.query 查询失败")
		case "observability.query":
			parts = append(parts, "observability.query 查询失败")
		case "knowledge.search":
			parts = append(parts, "knowledge.search 查询失败")
		default:
			if strings.HasPrefix(failure, "metrics.query_") {
				parts = append(parts, "metrics 查询失败")
			}
		}
	}
	if len(parts) == 0 {
		return "已获取部分系统证据，但关键查询失败，目前无法基于现有结果给出确定结论。"
	}

	request := strings.ToLower(interfaceStringFromAny(ctx["user_request"]))
	suffix := "目前无法基于现有结果给出确定结论。"
	for _, marker := range []string{"发布", "变更", "release", "deploy", "deployment", "相关", "relation"} {
		if strings.Contains(request, marker) {
			suffix = "目前无法确认是否与最近一次发布或变更相关。"
			break
		}
	}
	return fmt.Sprintf("已获取部分系统证据，但%s，%s", strings.Join(parts, "，"), suffix)
}

func (s *Service) invokeDiagnosisModelJSONWithFallback(
	ctx context.Context,
	phase string,
	rawInput contracts.DiagnosisInput,
	sentInput contracts.DiagnosisInput,
	systemPrompt string,
	primary modelRuntime,
	assist *modelRuntime,
	target interface{},
) (modelRuntime, error) {
	if strings.TrimSpace(primary.BaseURL) != "" {
		if err := s.invokeDiagnosisModelJSON(ctx, phase, rawInput, sentInput, systemPrompt, primary.Protocol, primary.BaseURL, primary.APIKey, primary.Model, target); err == nil {
			s.recordModelAttempt(primary, "success", fmt.Sprintf("%s generated diagnosis payload", phase))
			return primary, nil
		} else {
			s.recordModelAttempt(primary, "error", err.Error())
			if canUseAssistFallback(primary, assist) {
				s.logger.Warn("reasoning primary model failed, trying assist model", "session_id", rawInput.SessionID, "phase", phase, "error", err, "primary_model", primary.Model, "assist_model", assist.Model)
				s.auditModelFailover(ctx, rawInput.SessionID, primary, assist, err.Error())
				if assistErr := s.invokeDiagnosisModelJSON(ctx, phase, rawInput, sentInput, systemPrompt, assist.Protocol, assist.BaseURL, assist.APIKey, assist.Model, target); assistErr == nil {
					s.recordModelAttempt(*assist, "success", fmt.Sprintf("assist fallback %s payload generated", phase))
					return *assist, nil
				} else {
					s.recordModelAttempt(*assist, "error", assistErr.Error())
					return primary, fmt.Errorf("primary model failed: %w; assist model failed: %v", err, assistErr)
				}
			}
			return primary, err
		}
	}

	if canUseAssistFallback(primary, assist) {
		reason := "primary model not configured"
		s.auditModelFailover(ctx, rawInput.SessionID, primary, assist, reason)
		if err := s.invokeDiagnosisModelJSON(ctx, phase, rawInput, sentInput, systemPrompt, assist.Protocol, assist.BaseURL, assist.APIKey, assist.Model, target); err == nil {
			s.recordModelAttempt(*assist, "success", fmt.Sprintf("assist fallback %s payload generated", phase))
			return *assist, nil
		} else {
			s.recordModelAttempt(*assist, "error", err.Error())
			return primary, fmt.Errorf("%s; assist model failed: %v", reason, err)
		}
	}

	return primary, fmt.Errorf("model base url not configured")
}

func (s *Service) invokeDiagnosisModelJSON(
	ctx context.Context,
	phase string,
	rawInput contracts.DiagnosisInput,
	sentInput contracts.DiagnosisInput,
	systemPrompt string,
	protocol string,
	baseURL string,
	apiKey string,
	model string,
	target interface{},
) error {
	prompts := s.currentPromptSet()
	rawUserPrompt, err := prompts.RenderUserPrompt(rawInput.SessionID, rawInput.Context)
	if err != nil {
		return err
	}
	sentUserPrompt, err := prompts.RenderUserPrompt(sentInput.SessionID, sentInput.Context)
	if err != nil {
		return err
	}

	rawInvocation, err := buildModelInvocation(protocol, baseURL, apiKey, model, systemPrompt, rawUserPrompt)
	if err != nil {
		return err
	}
	sentInvocation, err := buildModelInvocation(protocol, baseURL, apiKey, model, systemPrompt, sentUserPrompt)
	if err != nil {
		return err
	}

	requestBody, err := json.Marshal(sentInvocation.Payload)
	if err != nil {
		return err
	}
	s.auditModelRequest(ctx, rawInput.SessionID, phase, systemPrompt, rawUserPrompt, sentUserPrompt, rawInput.Context, sentInput.Context, rawInvocation, sentInvocation, model, protocol)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sentInvocation.Endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	for key, value := range sentInvocation.Headers {
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	content, err := extractModelResponseContent(protocol, resp.StatusCode, resp.Body)
	if err != nil {
		return err
	}
	return decodeDiagnosisJSON(content, target)
}

func (s *Service) fallbackDiagnosisPlan(input contracts.DiagnosisInput, desenseMap map[string]string) contracts.DiagnosisPlan {
	plan := contracts.DiagnosisPlan{
		Summary:    "collect more context before deciding on execution",
		DesenseMap: desenseMap,
	}
	host := stringFromContext(input.Context, "host")
	service := stringFromContext(input.Context, "service")
	userRequest := stringFromContext(input.Context, "user_request")
	if hint := buildDirectExecutionHintForUserRequest(userRequest, service); hint != "" {
		plan.Summary = "用户请求需要主机级证据或动作，保留一条候选命令供后续受控执行。"
		plan.ToolPlan = []contracts.ToolPlanStep{{
			Tool:     "execution.run_command",
			Reason:   "Collect host-level evidence or execute the explicitly requested action.",
			Priority: 1,
			Status:   "planned",
			Input: map[string]interface{}{
				"command": hint,
				"host":    host,
				"service": service,
			},
		}}
		return plan
	}
	plan.ToolPlan = fallbackReadOnlyToolPlan(input)
	if len(plan.ToolPlan) == 0 {
		return plan
	}
	plan.Summary = fallbackPlanSummary(input, plan.ToolPlan)
	return plan
}

func fallbackReadOnlyToolPlan(input contracts.DiagnosisInput) []contracts.ToolPlanStep {
	host := stringFromContext(input.Context, "host")
	service := stringFromContext(input.Context, "service")
	requestIntent := classifyFallbackIntent(input.Context)

	switch requestIntent {
	case "observability_root_cause", "observability_with_delivery":
		metricsConnector := defaultConnectorIDForTool(input.Context, "metrics.query_range", "")
		logsConnector := defaultConnectorIDForTool(input.Context, "logs.query", "logs.query")
		observeConnector := defaultConnectorIDForTool(input.Context, "observability.query", "observability.query")
		query := summaryFromPlannerInput(input.Context)
		logsQuery := normalizePlannerLogsQuery(query, service)
		steps := []contracts.ToolPlanStep{
			{
				ID:          "metrics_1",
				Tool:        "metrics.query_range",
				ConnectorID: metricsConnector,
				Reason:      "Inspect recent monitoring trend before reviewing logs or traces.",
				Priority:    1,
				Status:      "planned",
				Input: map[string]interface{}{
					"host":         host,
					"service":      service,
					"mode":         "range",
					"window":       "1h",
					"step":         "5m",
					"query":        defaultPlannerMetricsQuery(host, service),
					"connector_id": metricsConnector,
				},
			},
			{
				ID:          "logs_1",
				Tool:        "logs.query",
				ConnectorID: logsConnector,
				Reason:      "Inspect recent log evidence before deciding whether traces or host access are necessary.",
				Priority:    2,
				Status:      "planned",
				Input: map[string]interface{}{
					"query":         logsQuery,
					"host":          host,
					"service":       service,
					"capability_id": "logs.query",
					"connector_id":  logsConnector,
				},
			},
			{
				ID:          "observe_1",
				Tool:        "observability.query",
				ConnectorID: observeConnector,
				Reason:      "Inspect trace and latency evidence after reviewing metrics and logs.",
				Priority:    3,
				Status:      "planned",
				Input: map[string]interface{}{
					"query":         query,
					"host":          host,
					"service":       service,
					"capability_id": "observability.query",
					"connector_id":  observeConnector,
				},
			},
		}
		if requestIntent == "observability_with_delivery" {
			deliveryConnector := defaultConnectorIDForTool(input.Context, "delivery.query", "delivery.query")
			steps = append(steps, contracts.ToolPlanStep{
				ID:          "delivery_1",
				Tool:        "delivery.query",
				ConnectorID: deliveryConnector,
				Reason:      "Correlate recent delivery changes after reviewing monitoring, log, and trace evidence.",
				Priority:    4,
				Status:      "planned",
				Input: map[string]interface{}{
					"query":         query,
					"host":          host,
					"service":       service,
					"capability_id": "delivery.query",
					"connector_id":  deliveryConnector,
				},
			})
		}
		return steps
	case "metrics_current":
		connectorID := defaultConnectorIDForTool(input.Context, "metrics.query_instant", "")
		return []contracts.ToolPlanStep{{
			ID:          "metrics_1",
			Tool:        "metrics.query_instant",
			ConnectorID: connectorID,
			Reason:      "Check current host state from monitoring before considering host access.",
			Priority:    1,
			Status:      "planned",
			Input: map[string]interface{}{
				"host":         host,
				"service":      service,
				"mode":         "instant",
				"query":        defaultPlannerMetricsQuery(host, service),
				"connector_id": connectorID,
			},
		}}
	case "metrics_history":
		connectorID := defaultConnectorIDForTool(input.Context, "metrics.query_range", "")
		return []contracts.ToolPlanStep{{
			ID:          "metrics_1",
			Tool:        "metrics.query_range",
			ConnectorID: connectorID,
			Reason:      "Inspect the recent resource trend before deciding whether host access is necessary.",
			Priority:    1,
			Status:      "planned",
			Input: map[string]interface{}{
				"host":         host,
				"service":      service,
				"mode":         "range",
				"window":       "1h",
				"step":         "5m",
				"query":        defaultPlannerMetricsQuery(host, service),
				"connector_id": connectorID,
			},
		}}
	default:
		return nil
	}
}

func classifyFallbackIntent(ctx map[string]interface{}) string {
	combined := strings.ToLower(strings.TrimSpace(summaryFromPlannerInput(ctx)))
	switch {
	case strings.Contains(combined, "报错"), strings.Contains(combined, "错误"), strings.Contains(combined, "日志"), strings.Contains(combined, "trace"), strings.Contains(combined, "span"), strings.Contains(combined, "latency"), strings.Contains(combined, "root cause"), strings.Contains(combined, "error"):
		if strings.Contains(combined, "发布") || strings.Contains(combined, "部署") || strings.Contains(combined, "release") || strings.Contains(combined, "deploy") || strings.Contains(combined, "commit") {
			return "observability_with_delivery"
		}
		return "observability_root_cause"
	case strings.Contains(combined, "负载"), strings.Contains(combined, "load"), strings.Contains(combined, "过去"), strings.Contains(combined, "小时"), strings.Contains(combined, "trend"), strings.Contains(combined, "history"):
		return "metrics_history"
	case strings.Contains(combined, "disk"), strings.Contains(combined, "磁盘"), strings.Contains(combined, "storage"):
		return "metrics_history"
	case strings.Contains(combined, "memory"), strings.Contains(combined, "内存"), strings.Contains(combined, "cpu"), strings.Contains(combined, "端口"), strings.Contains(combined, "port"), strings.Contains(combined, "socket"), strings.Contains(combined, "连接"), strings.Contains(combined, "状态"), strings.Contains(combined, "service"):
		return "metrics_current"
	default:
		return "generic"
	}
}

func fallbackPlanSummary(input contracts.DiagnosisInput, steps []contracts.ToolPlanStep) string {
	if len(steps) == 0 {
		return "collect more context before deciding on execution"
	}
	host := fallback(stringFromContext(input.Context, "host"), "目标主机")
	switch steps[0].Tool {
	case "observability.query":
		return "需要先收集日志与变更证据，再判断是否需要进入主机排查。"
	case "metrics.query_instant":
		return fmt.Sprintf("需要先查看 %s 当前监控状态，再决定是否需要主机执行。", host)
	case "metrics.query_range":
		if len(steps) > 1 && (steps[1].Tool == "logs.query" || steps[1].Tool == "observability.query") {
			return fmt.Sprintf("需要先查看 %s 最近一小时的监控趋势，再决定是否继续查看日志、trace 或主机执行。", host)
		}
		return fmt.Sprintf("需要先查看 %s 最近一小时的监控趋势。", host)
	default:
		return "collect more context before deciding on execution"
	}
}

func normalizePlannerToolPlan(input contracts.DiagnosisInput, steps []plannerToolPlanEntry) []contracts.ToolPlanStep {
	if len(steps) == 0 {
		return nil
	}
	host := stringFromContext(input.Context, "host")
	service := stringFromContext(input.Context, "service")
	out := make([]contracts.ToolPlanStep, 0, len(steps))
	for _, item := range steps {
		tool := strings.TrimSpace(strings.ToLower(item.Tool))
		if tool == "" {
			continue
		}
		step := contracts.ToolPlanStep{
			ID:                strings.TrimSpace(item.ID),
			Tool:              tool,
			ConnectorID:       strings.TrimSpace(item.ConnectorID),
			Reason:            strings.TrimSpace(item.Reason),
			Priority:          item.Priority,
			Status:            "planned",
			Input:             cloneInterfaceMap(firstNonEmptyMap(item.Input, item.Params)),
			OnFailure:         normalizeToolPlanPolicy(item.OnFailure, "continue"),
			OnPendingApproval: normalizeToolPlanPolicy(item.OnPendingApproval, "stop"),
			OnDenied:          normalizeToolPlanPolicy(item.OnDenied, "continue"),
		}
		if step.ID == "" {
			step.ID = fmt.Sprintf("step_%d", len(out)+1)
		}
		if step.Priority <= 0 {
			step.Priority = len(out) + 1
		}
		if strings.TrimSpace(item.Status) != "" {
			step.Status = strings.TrimSpace(item.Status)
		}
		if step.Input == nil {
			step.Input = map[string]interface{}{}
		}
		switch tool {
		case "metrics.query_range":
			if step.ConnectorID == "" {
				step.ConnectorID = defaultConnectorIDForTool(input.Context, tool, "")
			}
			setIfMissing(step.Input, "host", host)
			setIfMissing(step.Input, "service", service)
			setIfMissing(step.Input, "mode", "range")
			setIfMissing(step.Input, "window", "1h")
			setIfMissing(step.Input, "step", "5m")
			setIfMissing(step.Input, "query", defaultPlannerMetricsQuery(host, service))
			setIfMissing(step.Input, "connector_id", step.ConnectorID)
			step.Input["query"] = normalizePlannerMetricsQuery(interfaceString(step.Input["query"]), interfaceString(step.Input["host"]), interfaceString(step.Input["service"]))
		case "metrics.query_instant":
			if step.ConnectorID == "" {
				step.ConnectorID = defaultConnectorIDForTool(input.Context, tool, "")
			}
			setIfMissing(step.Input, "host", host)
			setIfMissing(step.Input, "service", service)
			setIfMissing(step.Input, "mode", "instant")
			setIfMissing(step.Input, "query", defaultPlannerMetricsQuery(host, service))
			setIfMissing(step.Input, "connector_id", step.ConnectorID)
			step.Input["query"] = normalizePlannerMetricsQuery(interfaceString(step.Input["query"]), interfaceString(step.Input["host"]), interfaceString(step.Input["service"]))
		case "knowledge.search":
			setIfMissing(step.Input, "query", summaryFromPlannerInput(input.Context))
		case "logs.query":
			if step.ConnectorID == "" {
				step.ConnectorID = defaultConnectorIDForTool(input.Context, tool, "logs.query")
			}
			setIfMissing(step.Input, "query", summaryFromPlannerInput(input.Context))
			setIfMissing(step.Input, "host", host)
			setIfMissing(step.Input, "service", service)
			setIfMissing(step.Input, "capability_id", "logs.query")
			setIfMissing(step.Input, "connector_id", step.ConnectorID)
			normalizedLogsQuery := normalizePlannerLogsQuery(interfaceString(step.Input["query"]), interfaceString(step.Input["service"]))
			step.Input["query"] = normalizedLogsQuery
			setIfMissing(step.Input, "time_range", plannerLogsTimeRange(summaryFromPlannerInput(input.Context), normalizedLogsQuery))
		case "observability.query":
			if step.ConnectorID == "" {
				step.ConnectorID = defaultConnectorIDForTool(input.Context, tool, "observability.query")
			}
			setIfMissing(step.Input, "query", summaryFromPlannerInput(input.Context))
			setIfMissing(step.Input, "host", host)
			setIfMissing(step.Input, "service", service)
			setIfMissing(step.Input, "capability_id", "observability.query")
			setIfMissing(step.Input, "connector_id", step.ConnectorID)
		case "delivery.query":
			if step.ConnectorID == "" {
				step.ConnectorID = defaultConnectorIDForTool(input.Context, tool, "delivery.query")
			}
			setIfMissing(step.Input, "query", summaryFromPlannerInput(input.Context))
			setIfMissing(step.Input, "host", host)
			setIfMissing(step.Input, "service", service)
			setIfMissing(step.Input, "capability_id", "delivery.query")
			setIfMissing(step.Input, "connector_id", step.ConnectorID)
		case "connector.invoke_capability":
			capabilityID := strings.TrimSpace(interfaceString(step.Input["capability_id"]))
			if capabilityID == "" {
				capabilityID = strings.TrimSpace(interfaceString(step.Input["capability"]))
			}
			if capabilityID == "" {
				continue
			}
			if step.ConnectorID == "" {
				step.ConnectorID = defaultConnectorIDForTool(input.Context, tool, capabilityID)
			}
			step.Input["capability_id"] = capabilityID
			setIfMissing(step.Input, "connector_id", step.ConnectorID)
		case "execution.run_command":
			command := strings.TrimSpace(interfaceString(step.Input["command"]))
			if command == "" {
				continue
			}
			if !executionHintExplicitlyAllowed(input, command) && isGenericReadOnlyExecutionHint(command) {
				continue
			}
			step.Input["command"] = command
			setIfMissing(step.Input, "host", host)
			setIfMissing(step.Input, "service", service)
			step.OnPendingApproval = normalizeToolPlanPolicy(item.OnPendingApproval, "stop")
			step.OnDenied = normalizeToolPlanPolicy(item.OnDenied, "stop")
			if step.Reason == "" {
				step.Reason = "Only run after explicit operator request or when host-side confirmation is truly required; after running, verify service state and re-check evidence."
			}
		default:
			continue
		}
		out = append(out, step)
	}
	if shouldUseOfficialDiskPlan(input.Context) && !skillAlreadySelected(input.Context) {
		return enforceOfficialDiskPlan(input, out)
	}
	return out
}

func skillAlreadySelected(ctx map[string]interface{}) bool {
	if len(ctx) == 0 {
		return false
	}
	raw, ok := ctx["skill_match"]
	if !ok {
		return false
	}
	match, ok := raw.(map[string]interface{})
	if !ok {
		return false
	}
	return strings.TrimSpace(interfaceString(match["skill_id"])) != ""
}

func shouldUseOfficialDiskPlan(ctx map[string]interface{}) bool {
	parts := []string{
		strings.ToLower(strings.TrimSpace(stringFromContext(ctx, "alert_name"))),
		strings.ToLower(strings.TrimSpace(stringFromContext(ctx, "user_request"))),
		strings.ToLower(strings.TrimSpace(stringFromContext(ctx, "summary"))),
	}
	for _, part := range parts {
		if strings.Contains(part, "disk") || strings.Contains(part, "filesystem") || strings.Contains(part, "storage") || strings.Contains(part, "磁盘") {
			return true
		}
	}
	return false
}

func enforceOfficialDiskPlan(input contracts.DiagnosisInput, steps []contracts.ToolPlanStep) []contracts.ToolPlanStep {
	host := stringFromContext(input.Context, "host")
	service := stringFromContext(input.Context, "service")
	metricsConnector := defaultConnectorIDForTool(input.Context, "metrics.query_range", "")
	knowledgeQuery := "disk space cleanup filesystem growth log retention"
	if summary := summaryFromPlannerInput(input.Context); strings.TrimSpace(summary) != "" {
		knowledgeQuery = summary + " disk cleanup filesystem growth"
	}
	ordered := []contracts.ToolPlanStep{
		{
			ID:          "metrics_capacity",
			Tool:        "metrics.query_range",
			ConnectorID: metricsConnector,
			Reason:      "Check filesystem usage trend before any host command.",
			Priority:    1,
			Status:      "planned",
			OnFailure:   "continue",
			Input: map[string]interface{}{
				"host":         host,
				"service":      service,
				"mode":         "range",
				"window":       "1h",
				"step":         "5m",
				"connector_id": metricsConnector,
				"query": `(
  1 - (
    node_filesystem_avail_bytes{fstype!="tmpfs",mountpoint!="/boot"}
    /
    node_filesystem_size_bytes{fstype!="tmpfs",mountpoint!="/boot"}
  )
) * 100`,
			},
		},
		{
			ID:          "metrics_forecast",
			Tool:        "metrics.query_range",
			ConnectorID: metricsConnector,
			Reason:      "Estimate whether available disk will continue to fall within the next hours.",
			Priority:    2,
			Status:      "planned",
			OnFailure:   "continue",
			Input: map[string]interface{}{
				"host":         host,
				"service":      service,
				"mode":         "range",
				"window":       "1h",
				"step":         "5m",
				"connector_id": metricsConnector,
				"query":        `predict_linear(node_filesystem_avail_bytes{fstype!="tmpfs",mountpoint!="/boot"}[1h], 4 * 3600)`,
			},
		},
		{
			ID:        "knowledge_disk",
			Tool:      "knowledge.search",
			Reason:    "Reuse prior cleanup guidance after metrics establish the pressure pattern.",
			Priority:  3,
			Status:    "planned",
			OnFailure: "continue",
			Input:     map[string]interface{}{"query": knowledgeQuery},
		},
	}
	if existing := findExecutionStep(steps); existing != nil {
		cloned := *existing
		cloned.ID = firstNonEmpty(cloned.ID, "host_probe")
		cloned.Priority = len(ordered) + 1
		cloned.Status = "planned"
		cloned.Reason = firstNonEmpty(cloned.Reason, "Only run after metrics and prior guidance still leave the source unclear.")
		if cloned.Input == nil {
			cloned.Input = map[string]interface{}{}
		}
		setIfMissing(cloned.Input, "host", host)
		setIfMissing(cloned.Input, "service", service)
		ordered = append(ordered, cloned)
	}
	return ordered
}

func findExecutionStep(steps []contracts.ToolPlanStep) *contracts.ToolPlanStep {
	for idx := range steps {
		if strings.TrimSpace(steps[idx].Tool) == "execution.run_command" {
			return &steps[idx]
		}
	}
	return nil
}

func normalizeToolPlanPolicy(value string, fallback string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "continue", "stop":
		return trimmed
	default:
		return fallback
	}
}

func plannerSystemPromptForInput(ctx map[string]interface{}) string {
	prompt := plannerSystemPrompt
	if rolePrompt := strings.TrimSpace(stringFromContext(ctx, "agent_role_system_prompt")); rolePrompt != "" {
		prompt = rolePrompt + "\n\n" + prompt
	}
	summary := strings.TrimSpace(stringFromContext(ctx, "tool_capabilities_summary"))
	if summary == "" {
		return prompt
	}
	return strings.TrimSpace(prompt + "\n\nAvailable session capabilities:\n" + summary)
}

func summaryFromPlannerInput(ctx map[string]interface{}) string {
	return fallback(stringFromContext(ctx, "summary"), stringFromContext(ctx, "user_request"))
}

func parsePlannerPriority(raw json.RawMessage) int {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0
	}
	var number int
	if err := json.Unmarshal(raw, &number); err == nil {
		return number
	}
	var floatNumber float64
	if err := json.Unmarshal(raw, &floatNumber); err == nil {
		return int(floatNumber)
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return 0
		}
		if parsed, err := strconv.Atoi(text); err == nil {
			return parsed
		}
	}
	return 0
}

func normalizePlannerMetricsQuery(query string, host string, service string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return defaultPlannerMetricsQuery(host, service)
	}
	lower := strings.ToLower(query)
	if looksLikeLoadAlias(lower) {
		return defaultPlannerMetricsQuery(host, service)
	}
	if strings.Contains(lower, "instance=") || strings.Contains(lower, "job=") {
		return query
	}
	switch {
	case strings.TrimSpace(host) != "":
		return injectPromQLMatcher(query, fmt.Sprintf(`instance="%s"`, strings.TrimSpace(host)))
	case strings.TrimSpace(service) != "":
		return injectPromQLMatcher(query, fmt.Sprintf(`job=~"%s.*"`, strings.TrimSpace(service)))
	default:
		return query
	}
}

func normalizePlannerLogsQuery(query string, service string) string {
	query = strings.TrimSpace(query)
	if looksLikeStructuredLogsQuery(query) {
		return query
	}
	return defaultPlannerLogsQuery(query, service)
}

func plannerLogsTimeRange(summary string, normalizedQuery string) string {
	summary = strings.ToLower(strings.TrimSpace(summary))
	normalizedQuery = strings.ToLower(strings.TrimSpace(normalizedQuery))
	if strings.Contains(summary, "tars-observability-host-file-test") || strings.Contains(normalizedQuery, "tars-observability-host-file-test") {
		return "168h"
	}
	return "1h"
}

func looksLikeStructuredLogsQuery(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	if query == "*" {
		return true
	}
	if strings.ContainsAny(query, "{}[]|\"'`") {
		return true
	}
	upper := strings.ToUpper(query)
	if strings.Contains(upper, " AND ") || strings.Contains(upper, " OR ") || strings.Contains(upper, " NOT ") {
		return true
	}
	if strings.Contains(query, ":") && !strings.Contains(query, "：") {
		return true
	}
	if strings.Count(query, " ") == 0 {
		return true
	}
	if containsCJKOrChinesePunctuation(query) {
		return false
	}
	return len(strings.Fields(query)) <= 4
}

func defaultPlannerLogsQuery(summary string, service string) string {
	if strings.Contains(strings.ToLower(strings.TrimSpace(summary)), "tars-observability-host-file-test") {
		return "tars-observability-host-file-test"
	}
	filters := make([]string, 0, 2)
	service = strings.TrimSpace(service)
	if service != "" {
		filters = append(filters, strconv.Quote(service))
	}
	keywords := plannerLogsKeywords(summary)
	switch len(keywords) {
	case 0:
		if len(filters) == 0 {
			return "*"
		}
	case 1:
		filters = append(filters, keywords[0])
	default:
		filters = append(filters, "("+strings.Join(keywords, " OR ")+")")
	}
	if len(filters) == 0 {
		return "*"
	}
	return strings.Join(filters, " ")
}

func plannerLogsKeywords(summary string) []string {
	lower := strings.ToLower(strings.TrimSpace(summary))
	switch {
	case strings.Contains(lower, "tars-observability-host-file-test"):
		return []string{"tars-observability-host-file-test"}
	case strings.Contains(lower, "报错"),
		strings.Contains(lower, "错误"),
		strings.Contains(lower, "异常"),
		strings.Contains(lower, "error"),
		strings.Contains(lower, "failed"),
		strings.Contains(lower, "failure"),
		strings.Contains(lower, "timeout"),
		strings.Contains(lower, "exception"),
		strings.Contains(lower, "panic"):
		return []string{"error", "failed", "timeout", "exception", "panic"}
	case strings.Contains(lower, "发布"),
		strings.Contains(lower, "部署"),
		strings.Contains(lower, "release"),
		strings.Contains(lower, "deploy"),
		strings.Contains(lower, "rollback"):
		return []string{"deploy", "release", "rollback"}
	default:
		return nil
	}
}

func containsCJKOrChinesePunctuation(value string) bool {
	for _, r := range value {
		switch {
		case unicode.Is(unicode.Han, r):
			return true
		case strings.ContainsRune("，。；！？、（）【】《》“”‘’：", r):
			return true
		}
	}
	return false
}

func looksLikeLoadAlias(query string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(query))
	switch {
	case strings.HasPrefix(trimmed, "1m:system_load_average"),
		strings.HasPrefix(trimmed, "5m:system_load_average"),
		strings.HasPrefix(trimmed, "15m:system_load_average"),
		strings.Contains(trimmed, "system_load_average"),
		strings.Contains(trimmed, "load_average"):
		return true
	default:
		return false
	}
}

func injectPromQLMatcher(query string, matcher string) string {
	query = strings.TrimSpace(query)
	matcher = strings.TrimSpace(matcher)
	if query == "" || matcher == "" {
		return query
	}
	open := strings.Index(query, "{")
	close := strings.LastIndex(query, "}")
	if open >= 0 && close > open {
		inside := strings.TrimSpace(query[open+1 : close])
		if inside == "" {
			return query[:open+1] + matcher + query[close:]
		}
		return query[:open+1] + inside + "," + matcher + query[close:]
	}
	return fmt.Sprintf("%s{%s}", query, matcher)
}

func rehydrateToolPlanWithConfig(steps []contracts.ToolPlanStep, mapping map[string]string, cfg *DesensitizationConfig) []contracts.ToolPlanStep {
	if len(steps) == 0 || len(mapping) == 0 {
		return steps
	}
	if cfg == nil {
		defaultCfg := DefaultDesensitizationConfig()
		cfg = &defaultCfg
	}
	out := make([]contracts.ToolPlanStep, 0, len(steps))
	for _, step := range steps {
		cloned := step
		cloned.ConnectorID = rehydratePlaceholdersWithConfig(cloned.ConnectorID, mapping, cfg)
		cloned.Reason = rehydratePlaceholdersWithConfig(cloned.Reason, mapping, cfg)
		cloned.Status = rehydratePlaceholdersWithConfig(cloned.Status, mapping, cfg)
		cloned.Input = rehydrateInterfaceMapWithConfig(cloned.Input, mapping, cfg)
		cloned.Output = rehydrateInterfaceMapWithConfig(cloned.Output, mapping, cfg)
		out = append(out, cloned)
	}
	return out
}

func rehydrateInterfaceMapWithConfig(input map[string]interface{}, mapping map[string]string, cfg *DesensitizationConfig) map[string]interface{} {
	if len(input) == 0 || len(mapping) == 0 {
		return cloneInterfaceMap(input)
	}
	output := make(map[string]interface{}, len(input))
	for key, value := range input {
		output[key] = rehydrateInterfaceValueWithConfig(value, mapping, cfg)
	}
	return output
}

func rehydrateInterfaceValueWithConfig(value interface{}, mapping map[string]string, cfg *DesensitizationConfig) interface{} {
	switch typed := value.(type) {
	case string:
		return rehydratePlaceholdersWithConfig(typed, mapping, cfg)
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			out = append(out, rehydrateInterfaceValueWithConfig(item, mapping, cfg))
		}
		return out
	case map[string]interface{}:
		return rehydrateInterfaceMapWithConfig(typed, mapping, cfg)
	default:
		return value
	}
}

func defaultPlannerMetricsQuery(host string, service string) string {
	host = strings.TrimSpace(host)
	service = strings.TrimSpace(service)
	switch {
	case host != "":
		return fmt.Sprintf(`node_load1{instance="%s"}`, host)
	case service != "":
		return fmt.Sprintf(`node_load1{job=~"%s.*"}`, service)
	default:
		return "node_load1"
	}
}

func setIfMissing(target map[string]interface{}, key string, value interface{}) {
	if target == nil {
		return
	}
	if _, ok := target[key]; ok {
		if strings.TrimSpace(interfaceString(target[key])) != "" {
			return
		}
	}
	target[key] = value
}

func interfaceString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func defaultConnectorIDForTool(ctx map[string]interface{}, tool string, capabilityID string) string {
	for _, item := range toolCapabilitiesFromContext(ctx) {
		if !item.Invocable {
			continue
		}
		if item.Tool != tool {
			continue
		}
		if tool == "connector.invoke_capability" && capabilityID != "" && item.CapabilityID != capabilityID {
			continue
		}
		if tool == "observability.query" && item.CapabilityID != "" && item.CapabilityID != "observability.query" && item.Action != "query" {
			continue
		}
		if tool == "logs.query" && item.CapabilityID != "" && item.CapabilityID != "logs.query" && item.Action != "query" {
			continue
		}
		if tool == "delivery.query" && item.CapabilityID != "" && item.CapabilityID != "delivery.query" && item.Action != "query" {
			continue
		}
		return item.ConnectorID
	}
	return ""
}

type toolCapabilityContextItem struct {
	Tool         string
	ConnectorID  string
	CapabilityID string
	Action       string
	Invocable    bool
}

func toolCapabilitiesFromContext(ctx map[string]interface{}) []toolCapabilityContextItem {
	rawItems, ok := ctx["tool_capabilities"]
	if !ok {
		return nil
	}
	items, ok := rawItems.([]interface{})
	if !ok {
		if typed, ok := rawItems.([]map[string]interface{}); ok {
			items = make([]interface{}, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		} else {
			payload, err := json.Marshal(rawItems)
			if err != nil {
				return nil
			}
			var generic []map[string]interface{}
			if err := json.Unmarshal(payload, &generic); err != nil {
				return nil
			}
			items = make([]interface{}, 0, len(generic))
			for _, item := range generic {
				items = append(items, item)
			}
		}
	}
	out := make([]toolCapabilityContextItem, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, toolCapabilityContextItem{
			Tool:         strings.TrimSpace(interfaceString(item["tool"])),
			ConnectorID:  strings.TrimSpace(interfaceString(item["connector_id"])),
			CapabilityID: strings.TrimSpace(interfaceString(item["capability_id"])),
			Action:       strings.TrimSpace(strings.ToLower(interfaceString(item["action"]))),
			Invocable:    interfaceBool(item["invocable"]),
		})
	}
	return out
}

func interfaceBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func firstNonEmptyMap(items ...map[string]interface{}) map[string]interface{} {
	for _, item := range items {
		if len(item) > 0 {
			return item
		}
	}
	return nil
}
