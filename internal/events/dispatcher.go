package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	foundationmetrics "tars/internal/foundation/metrics"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/connectors"
	"tars/internal/modules/skills"
)

type Dispatcher struct {
	logger     *slog.Logger
	metrics    *foundationmetrics.Registry
	workflow   contracts.WorkflowService
	reasoning  contracts.ReasoningService
	action     contracts.ActionService
	knowledge  contracts.KnowledgeService
	channel    contracts.ChannelService
	audit      audit.Logger
	connectors *connectors.Manager
	skills     *skills.Manager
	agentRoles *agentrole.Manager
	triggers   *TriggerWorker
	interval   time.Duration
}

func NewDispatcher(
	logger *slog.Logger,
	metrics *foundationmetrics.Registry,
	workflow contracts.WorkflowService,
	reasoning contracts.ReasoningService,
	action contracts.ActionService,
	knowledge contracts.KnowledgeService,
	channel contracts.ChannelService,
	auditLogger audit.Logger,
	optionalManagers ...interface{},
) *Dispatcher {
	var connectorManager *connectors.Manager
	var skillManager *skills.Manager
	var agentRoleManager *agentrole.Manager
	var triggerWorker *TriggerWorker
	for _, item := range optionalManagers {
		switch typed := item.(type) {
		case *connectors.Manager:
			if connectorManager == nil {
				connectorManager = typed
			}
		case *skills.Manager:
			if skillManager == nil {
				skillManager = typed
			}
		case *agentrole.Manager:
			if agentRoleManager == nil {
				agentRoleManager = typed
			}
		case *TriggerWorker:
			if triggerWorker == nil {
				triggerWorker = typed
			}
		}
	}
	return &Dispatcher{
		logger:     logger,
		metrics:    metrics,
		workflow:   workflow,
		reasoning:  reasoning,
		action:     action,
		knowledge:  knowledge,
		channel:    channel,
		audit:      auditLogger,
		connectors: connectorManager,
		skills:     skillManager,
		agentRoles: agentRoleManager,
		triggers:   triggerWorker,
		interval:   200 * time.Millisecond,
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	d.logger.Info("outbox dispatcher started")
	if recovered, err := d.workflow.RecoverPendingEvents(ctx); err != nil {
		d.logger.Error("recover processing outbox failed", "error", err)
	} else if recovered > 0 {
		d.logger.Warn("recovered stuck processing outbox events", "count", recovered)
		if d.metrics != nil {
			for i := 0; i < recovered; i++ {
				d.metrics.IncOutbox("unknown", "recovered")
			}
		}
	}
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		if err := d.RunOnce(ctx); err != nil {
			d.logger.Error("outbox dispatcher cycle failed", "error", err)
		}

		select {
		case <-ctx.Done():
			d.logger.Info("outbox dispatcher stopped")
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) RunOnce(ctx context.Context) error {
	items, err := d.workflow.ClaimEvents(ctx, 10)
	if err != nil {
		if d.metrics != nil {
			d.metrics.IncDispatcherCycle("error")
		}
		return err
	}
	if d.metrics != nil {
		if len(items) == 0 {
			d.metrics.IncDispatcherCycle("idle")
		} else {
			d.metrics.IncDispatcherCycle("ok")
		}
	}

	for _, item := range items {
		if d.metrics != nil {
			d.metrics.IncOutbox(item.Topic, "claimed")
			d.metrics.IncEventBus(item.Topic, "claimed", item.Attempt, "")
		}
		decision, err := d.dispatch(ctx, item)
		if err != nil {
			d.logger.Error("dispatch outbox event failed", "event_id", item.EventID, "topic", item.Topic, "attempt", item.Attempt, "error", err)
			decision = contracts.DefaultDeliveryPolicy(item.Topic).Decide(item.Attempt, err)
		}
		if err := d.workflow.ResolveEvent(ctx, item.EventID, decision); err != nil {
			d.logger.Error("resolve outbox event failed", "event_id", item.EventID, "topic", item.Topic, "decision", decision.Decision, "error", err)
			continue
		}
		if d.metrics != nil {
			result := string(decision.Decision)
			if result == "" {
				result = "unknown"
			}
			d.metrics.IncEventBus(item.Topic, result, item.Attempt, decision.Reason)
			switch decision.Decision {
			case contracts.DeliveryDecisionAck:
				d.metrics.IncOutbox(item.Topic, "completed")
			case contracts.DeliveryDecisionRetry:
				d.metrics.IncOutbox(item.Topic, "retry")
			case contracts.DeliveryDecisionDeadLetter:
				d.metrics.IncOutbox(item.Topic, "failed")
			}
		}
	}

	return nil
}

func (d *Dispatcher) dispatch(ctx context.Context, item contracts.EventEnvelope) (contracts.DeliveryResult, error) {
	switch item.Topic {
	case "session.analyze_requested":
		return d.dispatchDiagnosis(ctx, item)
	case "session.closed":
		return d.dispatchSessionClosed(ctx, item)
	case "telegram.send":
		return d.dispatchTelegramSend(ctx, item)
	default:
		return contracts.DeliveryResult{Decision: contracts.DeliveryDecisionDeadLetter, Reason: "unsupported_topic", LastError: fmt.Sprintf("unsupported outbox topic: %s", item.Topic)}, nil
	}
}

func (d *Dispatcher) dispatchDiagnosis(ctx context.Context, item contracts.EventEnvelope) (contracts.DeliveryResult, error) {
	sessionDetail, err := d.workflow.GetSession(ctx, item.AggregateID)
	if err != nil {
		return contracts.DeliveryResult{}, fmt.Errorf("load session: %w", err)
	}

	host := alertString(sessionDetail.Alert, "host")
	service := labelString(sessionDetail.Alert, "service")
	alertName := labelString(sessionDetail.Alert, "alertname")
	severity := alertString(sessionDetail.Alert, "severity")
	userRequest := annotationString(sessionDetail.Alert, "user_request")
	summary := annotationString(sessionDetail.Alert, "summary")

	baseContext := map[string]interface{}{
		"alert_name":   alertName,
		"host":         host,
		"service":      service,
		"severity":     severity,
		"user_request": fallbackString(userRequest, summary),
		"summary":      summary,
		"source":       alertString(sessionDetail.Alert, "source"),
	}
	var roleBinding *contracts.RoleModelBinding
	// Inject agent role context.
	if d.agentRoles != nil && sessionDetail.AgentRoleID != "" {
		role := d.agentRoles.ResolveForSession(sessionDetail.AgentRoleID)
		baseContext["agent_role_id"] = role.RoleID
		if role.Profile.SystemPrompt != "" {
			baseContext["agent_role_system_prompt"] = role.Profile.SystemPrompt
		}
		roleBinding = roleModelBinding(role.ModelBinding)
	}
	toolCapabilities := plannerToolCapabilities(d.connectors)
	if len(toolCapabilities) > 0 {
		baseContext["tool_capabilities"] = toolCapabilities
		baseContext["tool_capabilities_summary"] = toolCapabilitySummary(toolCapabilities)
	}
	if activeSkills := activeSkillSummaries(d.skills); len(activeSkills) > 0 {
		baseContext["active_skills"] = activeSkills
	}
	var (
		plan       contracts.DiagnosisPlan
		forcedPlan []contracts.ToolPlanStep
	)
	if d.skills != nil {
		if match := d.skills.Match(contracts.DiagnosisInput{SessionID: sessionDetail.SessionID, Context: baseContext}); match != nil {
			baseContext["skill_match"] = map[string]interface{}{
				"skill_id":   match.SkillID,
				"matched_by": match.MatchedBy,
				"trigger":    match.Trigger,
				"summary":    match.Summary,
			}
			d.auditSkillSelected(ctx, sessionDetail.SessionID, *match)
			forced := d.skills.Expand(*match, contracts.DiagnosisInput{SessionID: sessionDetail.SessionID, Context: baseContext}, toToolCapabilityDescriptors(toolCapabilities))
			if len(forced) > 0 {
				forcedPlan = append([]contracts.ToolPlanStep(nil), forced...)
				plan = contracts.DiagnosisPlan{
					Summary:  fallbackString(match.Summary, fallbackString(summary, userRequest)),
					ToolPlan: forcedPlan,
				}
				baseContext["planner_path"] = "skill_runtime_primary"
				d.auditSkillExpanded(ctx, sessionDetail.SessionID, *match, forced)
			} else {
				baseContext["skill_match_failed"] = true
			}
		}
	}
	if len(forcedPlan) == 0 {
		plan, err = d.reasoning.PlanDiagnosis(ctx, contracts.DiagnosisInput{
			SessionID:        sessionDetail.SessionID,
			Context:          baseContext,
			RoleModelBinding: roleBinding,
		})
		if err != nil {
			return contracts.DeliveryResult{}, fmt.Errorf("plan diagnosis: %w", err)
		}
	}
	d.auditToolPlanGenerated(ctx, sessionDetail.SessionID, plan)

	executedTools, err := d.executeToolPlan(ctx, sessionDetail, plan.ToolPlan)
	if err != nil {
		return contracts.DeliveryResult{}, fmt.Errorf("execute tool plan: %w", err)
	}

	// Fire on_skill_completed / on_skill_failed when the session was driven by a skill.
	if len(forcedPlan) > 0 && d.triggers != nil {
		skillFailed := false
		for _, step := range executedTools.ExecutedPlan {
			if step.Status == "failed" {
				skillFailed = true
				break
			}
		}
		skillDetail := contracts.ExecutionDetail{
			ExecutionID: sessionDetail.SessionID, // use sessionID as ref
			TargetHost:  alertString(sessionDetail.Alert, "host"),
			Status: func() string {
				if skillFailed {
					return "failed"
				}
				return "completed"
			}(),
		}
		d.triggers.FireSkillEvent(ctx, skillDetail)
	}

	finalContext := cloneMap(baseContext)
	if finalContext == nil {
		finalContext = map[string]interface{}{}
	}
	finalContext["planner_summary"] = plan.Summary
	finalContext["tool_plan"] = executedTools.ExecutedPlan
	for key, value := range executedTools.Context {
		finalContext[key] = value
	}
	if diskAnalysis := interfaceMap(finalContext["disk_space_analysis"]); len(diskAnalysis) > 0 {
		if shouldPreferMetricsOnlyForDisk(sessionDetail, diskAnalysis) {
			finalContext["disk_space_metrics_sufficient"] = true
		}
	}

	diagnosis, err := d.reasoning.FinalizeDiagnosis(ctx, contracts.DiagnosisInput{
		SessionID:        sessionDetail.SessionID,
		Context:          finalContext,
		RoleModelBinding: roleBinding,
	})
	if err != nil {
		return contracts.DeliveryResult{}, fmt.Errorf("finalize diagnosis: %w", err)
	}
	diagnosis.ToolPlan = append([]contracts.ToolPlanStep(nil), executedTools.ExecutedPlan...)
	if suppressed, reason := d.shouldSuppressExecutionHint(sessionDetail, executedTools, diagnosis.ExecutionHint); suppressed {
		if d.audit != nil {
			d.audit.Log(ctx, audit.Entry{
				ResourceType: "tool_plan",
				ResourceID:   sessionDetail.SessionID,
				Action:       "execution_hint_suppressed",
				Actor:        "tars_dispatcher",
				Metadata: map[string]any{
					"session_id":      sessionDetail.SessionID,
					"reason":          reason,
					"execution_hint":  diagnosis.ExecutionHint,
					"tool_step_count": len(executedTools.ExecutedPlan),
				},
			})
		}
		diagnosis.ExecutionHint = ""
		diagnosis.ToolPlan = removeExecutionPlanSteps(diagnosis.ToolPlan)
	}
	if strings.TrimSpace(diagnosis.ExecutionHint) != "" {
		diagnosis.ToolPlan = appendOrReplaceExecutionPlanStep(diagnosis.ToolPlan, diagnosis.ExecutionHint, host, service)
	}
	diagnosis.Attachments = append([]contracts.MessageAttachment(nil), executedTools.Attachments...)
	diagnosis.DesenseMap = mergeStringMaps(plan.DesenseMap, diagnosis.DesenseMap)

	dispatchResult, err := d.workflow.ApplyDiagnosis(ctx, item.EventID, diagnosis)
	if err != nil {
		return contracts.DeliveryResult{}, fmt.Errorf("apply diagnosis: %w", err)
	}

	for _, notification := range dispatchResult.Notifications {
		if _, err := d.channel.SendMessage(ctx, notification); err != nil {
			d.logger.Error(
				"dispatch notification failed",
				"event_id", item.EventID,
				"topic", item.Topic,
				"channel", notification.Channel,
				"target", notification.Target,
				"error", err,
			)
			if enqueueErr := d.workflow.EnqueueNotifications(ctx, sessionDetail.SessionID, []contracts.ChannelMessage{notification}); enqueueErr != nil {
				return contracts.DeliveryResult{}, fmt.Errorf("enqueue notification retry: %w", enqueueErr)
			}
			d.auditChannelMessage(ctx, sessionDetail.SessionID, item.Topic, "queued", notification)
			continue
		}
		d.auditChannelMessage(ctx, sessionDetail.SessionID, item.Topic, "sent", notification)
	}

	for _, executionReq := range dispatchResult.Executions {
		if err := d.runImmediateExecution(ctx, item, executionReq); err != nil {
			return contracts.DeliveryResult{}, err
		}
	}
	for _, capabilityReq := range dispatchResult.Capabilities {
		if err := d.runImmediateCapability(ctx, item, capabilityReq); err != nil {
			return contracts.DeliveryResult{}, err
		}
	}

	return contracts.DeliveryResult{Decision: contracts.DeliveryDecisionAck}, nil
}

func (d *Dispatcher) runImmediateCapability(ctx context.Context, item contracts.EventEnvelope, req contracts.ApprovedCapabilityRequest) error {
	result, invokeErr := d.action.InvokeApprovedCapability(ctx, req)
	if invokeErr != nil {
		_, mutationErr := d.workflow.HandleCapabilityResult(ctx, contracts.CapabilityExecutionResult{
			ApprovalID:   req.ApprovalID,
			SessionID:    req.SessionID,
			StepID:       req.StepID,
			Status:       "failed",
			ConnectorID:  req.ConnectorID,
			CapabilityID: req.CapabilityID,
			Runtime:      contracts.CloneRuntimeMetadata(req.Runtime),
			Error:        invokeErr.Error(),
		})
		if mutationErr != nil {
			return mutationErr
		}
		return invokeErr
	}
	if _, err := d.workflow.HandleCapabilityResult(ctx, contracts.CapabilityExecutionResult{
		ApprovalID:   req.ApprovalID,
		SessionID:    req.SessionID,
		StepID:       req.StepID,
		Status:       result.Status,
		ConnectorID:  req.ConnectorID,
		CapabilityID: req.CapabilityID,
		Output:       result.Output,
		Artifacts:    result.Artifacts,
		Metadata:     result.Metadata,
		Runtime:      result.Runtime,
		Error:        result.Error,
	}); err != nil {
		return err
	}
	if d.audit != nil {
		d.audit.Log(ctx, audit.Entry{
			ResourceType: "capability_approval",
			ResourceID:   req.ApprovalID,
			Action:       "capability_invoked",
			Actor:        "tars_dispatcher",
			Metadata: map[string]any{
				"event_id":      item.EventID,
				"session_id":    req.SessionID,
				"step_id":       req.StepID,
				"connector_id":  req.ConnectorID,
				"capability_id": req.CapabilityID,
				"status":        result.Status,
				"runtime":       runtimeMap(result.Runtime),
			},
		})
	}
	return nil
}

func (d *Dispatcher) runImmediateExecution(ctx context.Context, item contracts.EventEnvelope, executionReq contracts.ApprovedExecutionRequest) error {
	result, execErr := d.action.ExecuteApproved(ctx, executionReq)
	if execErr != nil {
		result = contracts.ExecutionResult{
			ExecutionID:   executionReq.ExecutionID,
			SessionID:     executionReq.SessionID,
			Status:        "failed",
			ConnectorID:   executionReq.ConnectorID,
			Protocol:      executionReq.Protocol,
			ExecutionMode: executionReq.ExecutionMode,
			Runtime: &contracts.RuntimeMetadata{
				Runtime:         fallbackString(executionReq.Protocol, "ssh"),
				Selection:       "auto_selector",
				ConnectorID:     executionReq.ConnectorID,
				ConnectorType:   executionReq.ConnectorType,
				ConnectorVendor: executionReq.ConnectorVendor,
				Protocol:        executionReq.Protocol,
				ExecutionMode:   executionReq.ExecutionMode,
				FallbackEnabled: true,
				FallbackTarget:  "ssh",
			},
			ExitCode: 1,
			OutputPreview: fmt.Sprintf(
				"direct execution failed before command completion: %s",
				execErr.Error(),
			),
		}
	}

	mutation, mutationErr := d.workflow.HandleExecutionResult(ctx, result)
	if mutationErr != nil {
		return fmt.Errorf("handle direct execution result: %w", mutationErr)
	}

	// Fire on_approval_requested if execution is now waiting for human approval.
	if mutation.Status == "pending_approval" && d.triggers != nil {
		executionDetailForApproval, loadErr := d.workflow.GetExecution(ctx, executionReq.ExecutionID)
		if loadErr == nil {
			d.triggers.FireApprovalRequested(ctx, executionDetailForApproval)
		} else {
			d.logger.Warn("could not load execution detail for approval trigger", "execution_id", executionReq.ExecutionID, "error", loadErr)
		}
	}

	if mutation.Status == "verifying" {
		verificationResult, verifyErr := d.action.VerifyExecution(ctx, contracts.VerificationRequest{
			SessionID:     executionReq.SessionID,
			ExecutionID:   executionReq.ExecutionID,
			TargetHost:    executionReq.TargetHost,
			Service:       executionReq.Service,
			ConnectorID:   executionReq.ConnectorID,
			Protocol:      executionReq.Protocol,
			ExecutionMode: executionReq.ExecutionMode,
		})
		if verifyErr != nil {
			verificationResult = contracts.VerificationResult{
				SessionID:   executionReq.SessionID,
				ExecutionID: executionReq.ExecutionID,
				Status:      "failed",
				Summary:     fmt.Sprintf("verification failed: %s", verifyErr.Error()),
				Runtime:     contracts.CloneRuntimeMetadata(result.Runtime),
				CheckedAt:   time.Now().UTC(),
				Details: map[string]interface{}{
					"error": verifyErr.Error(),
				},
			}
		}
		if _, err := d.workflow.HandleVerificationResult(ctx, verificationResult); err != nil {
			return fmt.Errorf("handle direct verification result: %w", err)
		}
	}

	sessionDetail, sessionErr := d.workflow.GetSession(ctx, executionReq.SessionID)
	if sessionErr != nil {
		return fmt.Errorf("load session after direct execution: %w", sessionErr)
	}
	executionDetail, detailErr := d.workflow.GetExecution(ctx, executionReq.ExecutionID)
	if detailErr != nil {
		return fmt.Errorf("load execution after direct execution: %w", detailErr)
	}
	executionOutput, outputErr := d.workflow.GetExecutionOutput(ctx, executionReq.ExecutionID)
	if outputErr != nil && !errors.Is(outputErr, contracts.ErrNotFound) {
		return fmt.Errorf("load execution output after direct execution: %w", outputErr)
	}

	message := contracts.ChannelMessage{
		Channel: executionResultNotificationChannel(sessionDetail),
		Target:  directExecutionNotificationTarget(sessionDetail),
		Subject: "执行结果",
		RefType: "execution",
		RefID:   executionDetail.ExecutionID,
		Source:  fallbackString(alertString(sessionDetail.Alert, "source"), "system"),
		Body:    formatExecutionResultMessage(sessionDetail, executionDetail, executionOutput),
	}
	if _, err := d.channel.SendMessage(ctx, message); err != nil {
		if enqueueErr := d.workflow.EnqueueNotifications(ctx, mutation.SessionID, []contracts.ChannelMessage{message}); enqueueErr != nil {
			return fmt.Errorf("enqueue direct execution result notification: %w", enqueueErr)
		}
		d.auditExecutionResultMessage(ctx, mutation.SessionID, item.Topic, "queued", message, executionDetail)
		return nil
	}
	d.auditExecutionResultMessage(ctx, mutation.SessionID, item.Topic, "sent", message, executionDetail)

	// Fire execution trigger (completed or failed)
	if d.triggers != nil {
		d.triggers.FireExecutionCompleted(ctx, executionDetail)
	}

	return nil
}

func (d *Dispatcher) dispatchTelegramSend(ctx context.Context, item contracts.EventEnvelope) (contracts.DeliveryResult, error) {
	message, err := contracts.DecodeChannelMessage(item.Payload)
	if err != nil {
		return contracts.DeliveryResult{Decision: contracts.DeliveryDecisionDeadLetter, Reason: "decode_failed", LastError: fmt.Sprintf("decode telegram outbox payload: %v", err)}, nil
	}
	if _, err := d.channel.SendMessage(ctx, message); err != nil {
		return contracts.DeliveryResult{}, fmt.Errorf("send telegram retry message: %w", err)
	}
	d.auditChannelMessage(ctx, item.AggregateID, item.Topic, "sent", message)
	return contracts.DeliveryResult{Decision: contracts.DeliveryDecisionAck}, nil
}

func (d *Dispatcher) dispatchSessionClosed(ctx context.Context, item contracts.EventEnvelope) (contracts.DeliveryResult, error) {
	if d.knowledge != nil {
		_, err := d.knowledge.IngestResolvedSession(ctx, contracts.SessionClosedEvent{
			SessionID:  item.AggregateID,
			TenantID:   "default",
			ResolvedAt: time.Now().UTC(),
		})
		if err != nil {
			return contracts.DeliveryResult{}, fmt.Errorf("ingest resolved session: %w", err)
		}
	}

	// Fire on_session_closed trigger
	if d.triggers != nil {
		d.triggers.FireSessionClosed(ctx, item.AggregateID, "")
	}

	return contracts.DeliveryResult{Decision: contracts.DeliveryDecisionAck}, nil
}

func alertString(alert map[string]interface{}, key string) string {
	if value, ok := alert[key].(string); ok {
		return value
	}
	return ""
}

func annotationString(alert map[string]interface{}, key string) string {
	annotations, ok := alert["annotations"]
	if !ok {
		return ""
	}

	switch typed := annotations.(type) {
	case map[string]string:
		return typed[key]
	case map[string]interface{}:
		if value, ok := typed[key].(string); ok {
			return value
		}
	}
	return ""
}

func fallbackString(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

func labelString(alert map[string]interface{}, key string) string {
	labels, ok := alert["labels"]
	if !ok {
		return ""
	}

	switch typed := labels.(type) {
	case map[string]string:
		return typed[key]
	case map[string]interface{}:
		if value, ok := typed[key].(string); ok {
			return value
		}
	}

	return ""
}

func (d *Dispatcher) auditChannelMessage(ctx context.Context, sessionID string, topic string, delivery string, message contracts.ChannelMessage) {
	if d.audit == nil {
		return
	}

	d.audit.Log(ctx, audit.Entry{
		ResourceType: "telegram_message",
		ResourceID:   fallbackString(sessionID, message.Target),
		Action:       "dispatch",
		Actor:        "tars_dispatcher",
		Metadata: map[string]any{
			"session_id":   sessionID,
			"topic":        topic,
			"delivery":     delivery,
			"channel":      message.Channel,
			"target":       message.Target,
			"actions":      len(message.Actions),
			"body_preview": compactMessage(message.Body, 240),
		},
	})
}

func (d *Dispatcher) auditExecutionResultMessage(ctx context.Context, sessionID string, topic string, delivery string, message contracts.ChannelMessage, executionDetail contracts.ExecutionDetail) {
	if d.audit == nil {
		return
	}

	d.audit.Log(ctx, audit.Entry{
		ResourceType: "telegram_message",
		ResourceID:   fallbackString(sessionID, message.Target),
		Action:       "dispatch",
		Actor:        "tars_dispatcher",
		Metadata: map[string]any{
			"session_id":      sessionID,
			"topic":           topic,
			"delivery":        delivery,
			"channel":         message.Channel,
			"target":          message.Target,
			"actions":         len(message.Actions),
			"message_type":    "execution_result",
			"execution_id":    executionDetail.ExecutionID,
			"runtime":         fallbackString(runtimeMetadataValue(executionDetail.Runtime, "runtime"), "n/a"),
			"connector_id":    fallbackString(executionDetail.ConnectorID, "n/a"),
			"protocol":        fallbackString(executionDetail.Protocol, "n/a"),
			"execution_mode":  fallbackString(executionDetail.ExecutionMode, "n/a"),
			"fallback_used":   runtimeMetadataBool(executionDetail.Runtime, "fallback_used"),
			"fallback_target": fallbackString(runtimeMetadataValue(executionDetail.Runtime, "fallback_target"), "n/a"),
			"body_preview":    compactMessage(message.Body, 240),
		},
	})
}

func (d *Dispatcher) shouldSuppressExecutionHint(sessionDetail contracts.SessionDetail, executed executedToolContext, executionHint string) (bool, string) {
	hint := strings.TrimSpace(executionHint)
	if hint == "" {
		return false, ""
	}

	userRequest := fallbackString(annotationString(sessionDetail.Alert, "user_request"), annotationString(sessionDetail.Alert, "summary"))
	if strings.TrimSpace(userRequest) == "" {
		return false, ""
	}
	if explicitlyRequestsHostLevelAction(userRequest, hint) {
		return false, ""
	}
	requestIntent := classifyUserRequestIntent(userRequest)
	hintIntent := classifyExecutionHintIntent(hint)
	hasMeaningfulEvidence := hasMeaningfulToolEvidence(executed.ExecutedPlan, executed.Attachments)
	attemptedMetrics := hasAttemptedTool(executed.ExecutedPlan, "metrics.query_range", "metrics.query_instant")
	attemptedLogs := hasAttemptedTool(executed.ExecutedPlan, "logs.query")
	attemptedObservability := hasAttemptedTool(executed.ExecutedPlan, "observability.query")
	attemptedDelivery := hasAttemptedTool(executed.ExecutedPlan, "delivery.query")
	attemptedSystemQueries := hasAttemptedTool(executed.ExecutedPlan,
		"metrics.query_range",
		"metrics.query_instant",
		"logs.query",
		"knowledge.search",
		"observability.query",
		"delivery.query",
		"connector.invoke_capability",
	)

	if requestIntent == "metrics_history" && attemptedMetrics {
		switch hintIntent {
		case "generic_host", "host_metrics", "service_status", "endpoint_probe":
			return true, "metrics_query_already_attempted"
		}
	}

	if attemptedSystemQueries {
		switch requestIntent {
		case "observability":
			if hintIntent == "generic_host" || hintIntent == "host_metrics" || hintIntent == "endpoint_probe" {
				return true, "tool_results_sufficient_for_non_host_request"
			}
		case "delivery":
			if (attemptedLogs || attemptedObservability) && attemptedDelivery && (hintIntent == "generic_host" || hintIntent == "host_metrics" || hintIntent == "endpoint_probe") {
				return true, "tool_results_sufficient_for_non_host_request"
			}
		}
		if hintIntent == "endpoint_probe" {
			return true, "tool_results_already_attempted_system_probe"
		}
	}

	if !hasMeaningfulEvidence {
		return false, ""
	}

	if requestIntent == "disk_incident" {
		if sufficient, _ := executed.Context["disk_space_metrics_sufficient"].(bool); sufficient && (hintIntent == "host_metrics" || hintIntent == "generic_host" || hintIntent == "service_status") {
			return true, "disk_metrics_evidence_sufficient"
		}
	}

	switch requestIntent {
	case "observability":
		if hintIntent == "generic_host" || hintIntent == "host_metrics" {
			return true, "tool_results_sufficient_for_non_host_request"
		}
	case "delivery":
		if (attemptedLogs || attemptedObservability) && attemptedDelivery && (hintIntent == "generic_host" || hintIntent == "host_metrics") {
			return true, "tool_results_sufficient_for_non_host_request"
		}
	case "metrics_history":
		if hintIntent == "generic_host" || hintIntent == "host_metrics" {
			return true, "metrics_history_answered_without_host_command"
		}
	}

	return false, ""
}

func explicitlyRequestsHostLevelAction(userRequest string, executionHint string) bool {
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

func hasMeaningfulToolEvidence(steps []contracts.ToolPlanStep, attachments []contracts.MessageAttachment) bool {
	_ = attachments
	for _, step := range steps {
		if strings.TrimSpace(step.Status) != "completed" {
			continue
		}
		switch strings.TrimSpace(step.Tool) {
		case "metrics.query_range", "metrics.query_instant":
			if intFromOutput(step.Output, "points") > 0 || intFromOutput(step.Output, "series_count") > 0 {
				return true
			}
		case "knowledge.search":
			if intFromOutput(step.Output, "hit_count") > 0 {
				return true
			}
		case "logs.query", "observability.query", "connector.invoke_capability":
			if intFromOutput(step.Output, "artifact_count") > 0 || intFromOutput(step.Output, "result_count") > 0 {
				return true
			}
			if result, ok := step.Output["result"].(map[string]interface{}); ok && len(result) > 0 {
				return true
			}
		case "delivery.query":
			if intFromOutput(step.Output, "result_count") > 0 {
				return true
			}
			if result, ok := step.Output["result"].(map[string]interface{}); ok && len(result) > 0 {
				return true
			}
		}
	}
	return false
}

func hasAttemptedTool(steps []contracts.ToolPlanStep, tools ...string) bool {
	if len(steps) == 0 || len(tools) == 0 {
		return false
	}
	allowed := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		allowed[strings.TrimSpace(tool)] = struct{}{}
	}
	for _, step := range steps {
		if _, ok := allowed[strings.TrimSpace(step.Tool)]; !ok {
			continue
		}
		if toolStepAttempted(step.Status) {
			return true
		}
	}
	return false
}

func toolStepAttempted(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "pending_approval", "denied", "running":
		return true
	default:
		return false
	}
}

func intFromOutput(output map[string]interface{}, key string) int {
	if len(output) == 0 {
		return 0
	}
	value, ok := output[key]
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

func classifyUserRequestIntent(userRequest string) string {
	lower := strings.ToLower(strings.TrimSpace(userRequest))
	switch {
	case (strings.Contains(userRequest, "报错") || strings.Contains(userRequest, "错误") || strings.Contains(userRequest, "日志") || strings.Contains(lower, "trace") || strings.Contains(lower, "span") || strings.Contains(lower, "latency") || strings.Contains(lower, "root cause") || strings.Contains(lower, "error")) &&
		(strings.Contains(userRequest, "发布") || strings.Contains(userRequest, "部署") || strings.Contains(lower, "deploy") || strings.Contains(lower, "release") || strings.Contains(lower, "pipeline") || strings.Contains(lower, "commit")):
		return "delivery"
	case strings.Contains(userRequest, "发布"), strings.Contains(userRequest, "部署"), strings.Contains(lower, "deploy"), strings.Contains(lower, "release"), strings.Contains(lower, "pipeline"), strings.Contains(lower, "commit"):
		return "delivery"
	case strings.Contains(userRequest, "报错"), strings.Contains(userRequest, "错误"), strings.Contains(userRequest, "日志"), strings.Contains(lower, "trace"), strings.Contains(lower, "span"), strings.Contains(lower, "latency"), strings.Contains(lower, "root cause"), strings.Contains(lower, "error"):
		return "observability"
	case strings.Contains(userRequest, "过去一小时"), strings.Contains(userRequest, "最近"), strings.Contains(lower, "trend"), strings.Contains(lower, "history"):
		return "metrics_history"
	case strings.Contains(userRequest, "磁盘"), strings.Contains(lower, "disk"), strings.Contains(lower, "filesystem"), strings.Contains(lower, "storage"):
		return "disk_incident"
	default:
		return "generic"
	}
}

func shouldPreferMetricsOnlyForDisk(sessionDetail contracts.SessionDetail, analysis map[string]interface{}) bool {
	if len(analysis) == 0 {
		return false
	}
	alertName := strings.ToLower(strings.TrimSpace(alertLabel(sessionDetail.Alert, "alertname")))
	userRequest := strings.ToLower(strings.TrimSpace(fallbackString(annotationString(sessionDetail.Alert, "user_request"), annotationString(sessionDetail.Alert, "summary"))))
	if !(strings.Contains(alertName, "disk") || strings.Contains(userRequest, "磁盘") || strings.Contains(userRequest, "disk") || strings.Contains(userRequest, "filesystem")) {
		return false
	}
	currentUsage, hasUsage := numericFromAnalysis(analysis["current_usage_percent"])
	forecastRisk := strings.ToLower(strings.TrimSpace(interfaceString(analysis["forecast_risk"])))
	growthRate, hasGrowth := numericFromAnalysis(analysis["growth_percent_per_hour"])
	if hasUsage && currentUsage >= 85 {
		if !hasGrowth || growthRate <= 0.5 {
			return true
		}
	}
	if forecastRisk == "stable" {
		return true
	}
	return false
}

func numericFromAnalysis(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func classifyExecutionHintIntent(hint string) string {
	lower := strings.ToLower(strings.TrimSpace(hint))
	switch {
	case lower == "hostname && uptime":
		return "generic_host"
	case (strings.HasPrefix(lower, "curl ") || strings.HasPrefix(lower, "wget ")) &&
		(strings.Contains(lower, "http://") || strings.Contains(lower, "https://")):
		return "endpoint_probe"
	case strings.Contains(lower, "uptime") || strings.Contains(lower, "/proc/loadavg") || strings.Contains(lower, "free -m") || strings.Contains(lower, "df -h"):
		return "host_metrics"
	case strings.Contains(lower, "systemctl status") || strings.Contains(lower, "journalctl"):
		return "service_status"
	default:
		return "generic"
	}
}

func compactMessage(value string, maxLen int) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen] + "..."
}

func directExecutionNotificationTarget(sessionDetail contracts.SessionDetail) string {
	if target := alertString(sessionDetail.Alert, "chat_id"); strings.TrimSpace(target) != "" {
		return strings.TrimSpace(target)
	}
	switch labels := sessionDetail.Alert["labels"].(type) {
	case map[string]interface{}:
		if value, ok := labels["chat_id"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	case map[string]string:
		if value := strings.TrimSpace(labels["chat_id"]); value != "" {
			return value
		}
	}
	return "ops-room"
}

func executionResultNotificationChannel(sessionDetail contracts.SessionDetail) string {
	switch strings.TrimSpace(alertString(sessionDetail.Alert, "source")) {
	case "web_chat", "ops_api":
		return "in_app_inbox"
	default:
		return "telegram"
	}
}

func formatExecutionResultMessage(sessionDetail contracts.SessionDetail, executionDetail contracts.ExecutionDetail, outputChunks []contracts.ExecutionOutputChunk) string {
	lines := []string{
		"[TARS] 执行结果",
		fmt.Sprintf("主机: %s", executionDetail.TargetHost),
		fmt.Sprintf("状态: %s", executionDetail.Status),
		fmt.Sprintf("执行链: %s", compactExecutionRuntimeLabel(executionDetail)),
	}
	if executionDetail.ExitCode != 0 {
		lines = append(lines, fmt.Sprintf("退出码: %d", executionDetail.ExitCode))
	}
	if sessionDetail.Verification != nil {
		lines = append(lines, fmt.Sprintf("校验: %s", sessionDetail.Verification.Status))
		if strings.TrimSpace(sessionDetail.Verification.Summary) != "" {
			lines = append(lines, fmt.Sprintf("校验说明: %s", sessionDetail.Verification.Summary))
		}
	}
	if executionDetail.OutputRef != "" {
		lines = append(lines, fmt.Sprintf("日志: %s", executionDetail.OutputRef))
	}
	if preview := compactExecutionOutput(outputChunks); preview != "" {
		label := "输出"
		if executionDetail.OutputTruncated {
			label = "输出（已截断）"
		}
		lines = append(lines, fmt.Sprintf("%s:\n%s", label, preview))
	}
	lines = append(lines, fmt.Sprintf("会话: %s", sessionDetail.SessionID))
	return strings.Join(lines, "\n")
}

func compactExecutionRuntimeLabel(executionDetail contracts.ExecutionDetail) string {
	parts := []string{}
	if strings.TrimSpace(executionDetail.ConnectorID) != "" {
		parts = append(parts, executionDetail.ConnectorID)
	}
	if strings.TrimSpace(executionDetail.ExecutionMode) != "" {
		parts = append(parts, executionDetail.ExecutionMode)
	} else if strings.TrimSpace(executionDetail.Protocol) != "" {
		parts = append(parts, executionDetail.Protocol)
	} else if runtime := strings.TrimSpace(runtimeMetadataValue(executionDetail.Runtime, "runtime")); runtime != "" {
		parts = append(parts, runtime)
	}
	if len(parts) == 0 {
		parts = append(parts, "ssh")
	}
	if runtimeMetadataBool(executionDetail.Runtime, "fallback_used") {
		parts = append(parts, fmt.Sprintf("fallback→%s", fallbackString(runtimeMetadataValue(executionDetail.Runtime, "fallback_target"), "ssh")))
	}
	return strings.Join(parts, " · ")
}

func runtimeMetadataValue(runtime *contracts.RuntimeMetadata, field string) string {
	if runtime == nil {
		return ""
	}
	switch field {
	case "runtime":
		return runtime.Runtime
	case "fallback_target":
		return runtime.FallbackTarget
	default:
		return ""
	}
}

func runtimeMetadataBool(runtime *contracts.RuntimeMetadata, field string) bool {
	if runtime == nil {
		return false
	}
	switch field {
	case "fallback_used":
		return runtime.FallbackUsed
	default:
		return false
	}
}

func compactExecutionOutput(chunks []contracts.ExecutionOutputChunk) string {
	if len(chunks) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		builder.WriteString(chunk.Content)
		if builder.Len() >= 480 {
			break
		}
	}

	preview := strings.TrimSpace(builder.String())
	if preview == "" {
		return ""
	}
	if len(preview) > 480 {
		preview = preview[:480]
	}

	lines := strings.Split(preview, "\n")
	if len(lines) > 6 {
		lines = lines[:6]
	}
	return strings.Join(lines, "\n")
}

func roleModelBinding(binding agentrole.ModelBinding) *contracts.RoleModelBinding {
	primary := roleModelTargetBinding(binding.Primary)
	fallback := roleModelTargetBinding(binding.Fallback)
	if primary == nil && fallback == nil && !binding.InheritPlatformDefault {
		return nil
	}
	return &contracts.RoleModelBinding{
		Primary:                primary,
		Fallback:               fallback,
		InheritPlatformDefault: binding.InheritPlatformDefault,
	}
}

func roleModelTargetBinding(binding *agentrole.ModelTargetBinding) *contracts.RoleModelTargetBinding {
	if binding == nil {
		return nil
	}
	target := &contracts.RoleModelTargetBinding{
		ProviderID: strings.TrimSpace(binding.ProviderID),
		Model:      strings.TrimSpace(binding.Model),
	}
	if target.ProviderID == "" && target.Model == "" {
		return nil
	}
	return target
}

func buildDiagnosisMetricsQuery(host string, service string) string {
	host = strings.TrimSpace(host)
	service = strings.TrimSpace(service)
	switch {
	case host != "":
		return fmt.Sprintf(`node_load1{instance=%q}`, host)
	case service != "":
		return fmt.Sprintf(`node_load1{job=~".*%s.*"}`, service)
	default:
		return "node_load1"
	}
}
