package events

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"tars/internal/contracts"
	"tars/internal/foundation/audit"
	"tars/internal/foundation/logger"
	"tars/internal/modules/action"
	actionssh "tars/internal/modules/action/ssh"
	"tars/internal/modules/agentrole"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/knowledge"
	"tars/internal/modules/reasoning"
	"tars/internal/modules/skills"
	"tars/internal/modules/workflow"
)

func TestDispatcherRunOnceAppliesDiagnosis(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
	})
	channelSvc := &captureChannel{}
	auditLogger := &captureAuditLogger{}
	actionSvc := newActionService(t)
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		newFallbackReasoningService(),
		actionSvc,
		knowledge.NewService(),
		channelSvc,
		auditLogger,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-1",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-1",
			"service":   "api",
			"severity":  "critical",
		},
		Annotations: map[string]string{"user_request": "HighCPU 重启 api"},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sessionDetail.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %s", sessionDetail.Status)
	}
	if !strings.Contains(sessionDetail.DiagnosisSummary, "HighCPU") {
		t.Fatalf("unexpected diagnosis summary: %q", sessionDetail.DiagnosisSummary)
	}
	if len(channelSvc.messages) != 2 {
		t.Fatalf("expected diagnosis + approval notifications, got %d", len(channelSvc.messages))
	}
	if len(channelSvc.messages[1].Actions) != 3 {
		t.Fatalf("expected approval actions, got %+v", channelSvc.messages[1].Actions)
	}

	failedOutbox, err := workflowSvc.ListOutbox(ctx, contracts.ListOutboxFilter{Status: "failed"})
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(failedOutbox) != 0 {
		t.Fatalf("expected no failed outbox events, got %d", len(failedOutbox))
	}
}

func TestDispatcherRunOnceMarksOutboxFailed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
	})
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		failingReasoning{err: errors.New("model timeout")},
		newActionService(t),
		knowledge.NewService(),
		&captureChannel{},
		nil,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "warning",
		Fingerprint: "DiskFull:host-2",
		Labels: map[string]string{
			"alertname": "DiskFull",
			"instance":  "host-2",
			"service":   "worker",
			"severity":  "warning",
		},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	failedOutbox, err := workflowSvc.ListOutbox(ctx, contracts.ListOutboxFilter{Status: "failed"})
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(failedOutbox) != 1 {
		t.Fatalf("expected 1 failed outbox event, got %d", len(failedOutbox))
	}
	if !strings.Contains(failedOutbox[0].LastError, "model timeout") {
		t.Fatalf("unexpected outbox error: %q", failedOutbox[0].LastError)
	}

	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if !hasTimelineEvent(sessionDetail.Timeline, "outbox_failed") {
		t.Fatalf("expected outbox_failed timeline event, got %+v", sessionDetail.Timeline)
	}
}

func TestDispatcherRunOnceRetriesTelegramSendBeforeDeadLetter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{DiagnosisEnabled: true})
	if err := workflowSvc.EnqueueNotifications(ctx, "ses-telegram-1", []contracts.ChannelMessage{{
		Channel: "telegram",
		Target:  "ops-room",
		Body:    "hello",
	}}); err != nil {
		t.Fatalf("enqueue notifications: %v", err)
	}

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		newFallbackReasoningService(),
		newActionService(t),
		knowledge.NewService(),
		alwaysFailChannel{},
		nil,
	)

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	pending, err := workflowSvc.ClaimEvents(ctx, 10)
	if err != nil {
		t.Fatalf("claim events after retry: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected retry event to remain deferred, got %+v", pending)
	}

	items, err := workflowSvc.ListOutbox(ctx, contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected pending retry item, got %+v", items)
	}
	if items[0].RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %+v", items[0])
	}
	if items[0].AvailableAt.IsZero() {
		t.Fatalf("expected retry available_at to be set")
	}

	if err := workflowSvc.ResolveEvent(ctx, items[0].ID, contracts.DeliveryResult{Decision: contracts.DeliveryDecisionRetry, LastError: "second failure", Delay: 0, Reason: "test"}); err != nil {
		t.Fatalf("schedule second retry: %v", err)
	}
	items, err = workflowSvc.ListOutbox(ctx, contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list retried outbox: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected pending item after second retry, got %+v", items)
	}
	if err := workflowSvc.ResolveEvent(ctx, items[0].ID, contracts.DeliveryResult{Decision: contracts.DeliveryDecisionDeadLetter, LastError: "third failure", Reason: "max_attempts_exhausted"}); err != nil {
		t.Fatalf("dead letter event: %v", err)
	}

	failed, err := workflowSvc.ListOutbox(ctx, contracts.ListOutboxFilter{Status: "failed"})
	if err != nil {
		t.Fatalf("list failed outbox: %v", err)
	}
	if len(failed) != 1 {
		t.Fatalf("expected failed dead-letter item, got %+v", failed)
	}
	if failed[0].RetryCount != 3 {
		t.Fatalf("expected retry count 3 after dead letter, got %+v", failed[0])
	}
}

func TestDispatcherRunOnceDirectExecutesWhitelistCommand(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
		AuthorizationPolicy: authorization.New(authorization.Config{
			Defaults: authorization.Defaults{
				WhitelistAction: authorization.ActionDirectExecute,
				BlacklistAction: authorization.ActionSuggestOnly,
				UnmatchedAction: authorization.ActionRequireApproval,
			},
			SSH: authorization.SSHCommandConfig{
				NormalizeWhitespace: true,
				Whitelist:           []string{"uptime*"},
			},
		}),
	})
	channelSvc := &captureChannel{}
	auditLogger := &captureAuditLogger{}
	actionSvc := newActionService(t)
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		newFallbackReasoningService(),
		actionSvc,
		knowledge.NewService(),
		channelSvc,
		auditLogger,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "telegram_chat",
		Severity:    "info",
		Fingerprint: "chat:load",
		Labels: map[string]string{
			"alertname": "TarsChatLoadRequest",
			"instance":  "host-1",
			"host":      "host-1",
			"severity":  "info",
			"chat_id":   "445308292",
			"tars_chat": "true",
		},
		Annotations: map[string]string{
			"summary":      "看系统负载",
			"user_request": "看系统负载",
		},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sessionDetail.Status != "resolved" {
		t.Fatalf("expected resolved after evidence-only diagnosis, got %+v", sessionDetail)
	}
	if len(sessionDetail.Executions) != 0 {
		t.Fatalf("expected no execution drafts for read-only load diagnosis, got %+v", sessionDetail.Executions)
	}
	if len(channelSvc.messages) != 1 {
		t.Fatalf("expected only diagnosis notification, got %d", len(channelSvc.messages))
	}
	if !strings.Contains(channelSvc.messages[0].Body, "看系统负载") {
		t.Fatalf("expected diagnosis message to mention user request, got %+v", channelSvc.messages[0])
	}
	if len(auditLogger.entries) == 0 {
		t.Fatalf("expected audit entries to be recorded")
	}
	found := false
	for _, entry := range auditLogger.entries {
		if entry.ResourceType != "telegram_message" {
			continue
		}
		if topic, _ := entry.Metadata["topic"].(string); topic != "session.analyze_requested" {
			continue
		}
		found = true
	}
	if !found {
		t.Fatalf("expected diagnosis telegram audit entry, got %+v", auditLogger.entries)
	}
}

func TestDispatcherSuppressesDiskExecutionHintWhenMetricsEvidenceSuffices(t *testing.T) {
	t.Parallel()

	dispatcher := &Dispatcher{}
	session := contracts.SessionDetail{
		Alert: map[string]interface{}{
			"labels":      map[string]string{"alertname": "DiskFull"},
			"annotations": map[string]string{"summary": "disk usage high"},
		},
	}
	executed := executedToolContext{
		Context: map[string]interface{}{
			"disk_space_metrics_sufficient": true,
		},
		ExecutedPlan: []contracts.ToolPlanStep{{
			Tool:   "metrics.query_range",
			Status: "completed",
			Output: map[string]interface{}{
				"series_count": 1,
				"points":       12,
				"analysis": map[string]interface{}{
					"current_usage_percent":   88.0,
					"growth_percent_per_hour": 0.2,
				},
			},
		}},
		Attachments: []contracts.MessageAttachment{{Name: "disk-space-analysis.json"}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "df -h")
	if !suppressed {
		t.Fatalf("expected execution hint suppression")
	}
	if reason != "disk_metrics_evidence_sufficient" {
		t.Fatalf("unexpected suppression reason %q", reason)
	}
}

func TestDispatcherRunOnceRehydratesExecutionHintBeforeApproval(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
	})
	channelSvc := &captureChannel{}
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		staticReasoning{diagnosis: contracts.DiagnosisOutput{
			Summary:       "delete [PATH_1] on [IP_1]",
			ExecutionHint: "rm [PATH_1]",
			DesenseMap: map[string]string{
				"[PATH_1]": "/tmp/1.txt",
				"[IP_1]":   "192.168.3.106",
			},
		}},
		newActionService(t),
		knowledge.NewService(),
		channelSvc,
		nil,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "telegram_chat",
		Severity:    "info",
		Fingerprint: "chat:delete-file",
		Labels: map[string]string{
			"alertname": "TarsChatRequest",
			"instance":  "192.168.3.106",
			"host":      "192.168.3.106",
			"severity":  "info",
			"chat_id":   "445308292",
			"tars_chat": "true",
		},
		Annotations: map[string]string{
			"summary":      "执行命令 rm /tmp/1.txt",
			"user_request": "执行命令 rm /tmp/1.txt",
		},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sessionDetail.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %+v", sessionDetail)
	}
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected one execution draft, got %+v", sessionDetail.Executions)
	}
	if sessionDetail.Executions[0].Command != "rm /tmp/1.txt" {
		t.Fatalf("expected rehydrated command, got %+v", sessionDetail.Executions[0])
	}
	if len(channelSvc.messages) < 2 {
		t.Fatalf("expected diagnosis + approval notifications, got %d", len(channelSvc.messages))
	}
	if !strings.Contains(channelSvc.messages[0].Body, "rm /tmp/1.txt") {
		t.Fatalf("expected diagnosis message to contain rehydrated command, got %+v", channelSvc.messages[0])
	}
	if !strings.Contains(channelSvc.messages[1].Body, "rm /tmp/1.txt") {
		t.Fatalf("expected approval message to contain rehydrated command, got %+v", channelSvc.messages[1])
	}
}

func TestDispatcherRunOncePassesResolvedRoleModelBindingToReasoning(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{DiagnosisEnabled: true})
	roles, err := agentrole.NewManager("", agentrole.Options{})
	if err != nil {
		t.Fatalf("new agent role manager: %v", err)
	}
	if _, err := roles.Update(agentrole.AgentRole{
		RoleID:      "diagnosis",
		DisplayName: "Diagnosis",
		Status:      "active",
		Profile: agentrole.Profile{
			SystemPrompt: "diagnose carefully",
		},
		CapabilityBinding: agentrole.CapabilityBinding{Mode: "unrestricted"},
		PolicyBinding:     agentrole.PolicyBinding{MaxRiskLevel: "warning", MaxAction: "require_approval"},
		ModelBinding: agentrole.ModelBinding{
			Primary: &agentrole.ModelTargetBinding{
				ProviderID: "openai-main",
				Model:      "gpt-4.1-mini",
			},
			Fallback: &agentrole.ModelTargetBinding{
				ProviderID: "openai-backup",
				Model:      "gpt-4o-mini",
			},
		},
	}); err != nil {
		t.Fatalf("update diagnosis role: %v", err)
	}

	reasoningSvc := &captureReasoning{}
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		reasoningSvc,
		newActionService(t),
		knowledge.NewService(),
		&captureChannel{},
		nil,
		nil,
		nil,
		roles,
	)

	_, err = workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:role-binding",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-1",
			"service":   "api",
			"severity":  "critical",
		},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}
	if reasoningSvc.lastPlan.RoleModelBinding == nil {
		t.Fatalf("expected dispatcher to pass role model binding")
	}
	if reasoningSvc.lastPlan.RoleModelBinding.Primary == nil || reasoningSvc.lastPlan.RoleModelBinding.Primary.ProviderID != "openai-main" || reasoningSvc.lastPlan.RoleModelBinding.Primary.Model != "gpt-4.1-mini" {
		t.Fatalf("unexpected primary binding: %+v", reasoningSvc.lastPlan.RoleModelBinding)
	}
	if reasoningSvc.lastPlan.RoleModelBinding.Fallback == nil || reasoningSvc.lastPlan.RoleModelBinding.Fallback.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected fallback binding: %+v", reasoningSvc.lastPlan.RoleModelBinding)
	}
	if reasoningSvc.lastFinalize.RoleModelBinding == nil {
		t.Fatalf("expected dispatcher to pass role model binding to finalizer")
	}
	if reasoningSvc.lastFinalize.RoleModelBinding.Primary == nil || reasoningSvc.lastFinalize.RoleModelBinding.Primary.ProviderID != "openai-main" || reasoningSvc.lastFinalize.RoleModelBinding.Primary.Model != "gpt-4.1-mini" {
		t.Fatalf("unexpected finalize primary binding: %+v", reasoningSvc.lastFinalize.RoleModelBinding)
	}
}

func TestDispatcherSuppressesGenericExecutionHintWhenToolEvidenceIsSufficient(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-suppress-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "最近 api 报错和最近一次发布有关系吗",
			},
		},
	}
	executed := executedToolContext{
		Attachments: []contracts.MessageAttachment{{
			Type: "image",
			Name: "metrics-range.png",
		}},
		ExecutedPlan: []contracts.ToolPlanStep{
			{
				Tool:   "observability.query",
				Status: "completed",
				Output: map[string]interface{}{
					"result_count": 3,
				},
			},
			{
				Tool:   "delivery.query",
				Status: "completed",
				Output: map[string]interface{}{
					"result": map[string]interface{}{
						"release": "2026.03.20",
					},
				},
			},
		},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "uptime && cat /proc/loadavg")
	if !suppressed {
		t.Fatalf("expected execution hint to be suppressed")
	}
	if reason == "" {
		t.Fatalf("expected suppression reason")
	}
}

func TestDispatcherKeepsExecutionHintWhenOperatorExplicitlyRequestsHostCommand(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-keep-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "执行命令查看 api 状态",
			},
		},
	}
	executed := executedToolContext{
		ExecutedPlan: []contracts.ToolPlanStep{{
			Tool:   "delivery.query",
			Status: "completed",
			Output: map[string]interface{}{
				"result": map[string]interface{}{
					"release": "2026.03.20",
				},
			},
		}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "systemctl status api --no-pager --lines=20 || true")
	if suppressed {
		t.Fatalf("expected execution hint to be kept, got reason=%s", reason)
	}
}

func TestDispatcherSuppressesExecutionHintAfterFailedMetricsAttempt(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-metrics-suppress-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "过去一小时机器负载怎么样",
			},
		},
	}
	executed := executedToolContext{
		ExecutedPlan: []contracts.ToolPlanStep{
			{
				Tool:   "metrics.query_range",
				Status: "failed",
				Output: map[string]interface{}{
					"error": "connect: connection refused",
				},
			},
		},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "curl http://192.168.3.106:9090/api/v1/status")
	if !suppressed {
		t.Fatalf("expected execution hint to be suppressed")
	}
	if reason != "metrics_query_already_attempted" {
		t.Fatalf("unexpected suppression reason: %s", reason)
	}
}

func TestDispatcherSuppressesEndpointProbeAfterSystemQueries(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-delivery-suppress-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "最近 api 报错和最近一次发布有关系吗",
			},
		},
	}
	executed := executedToolContext{
		ExecutedPlan: []contracts.ToolPlanStep{
			{
				Tool:   "delivery.query",
				Status: "completed",
				Output: map[string]interface{}{
					"result": map[string]interface{}{
						"release": "2026.03.20",
					},
				},
			},
			{
				Tool:   "observability.query",
				Status: "completed",
				Output: map[string]interface{}{
					"result_count": 3,
				},
			},
		},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "curl http://192.168.3.106:9090/api/v1/status")
	if !suppressed {
		t.Fatalf("expected endpoint probe to be suppressed")
	}
	if reason == "" {
		t.Fatalf("expected suppression reason")
	}
}

func TestDispatcherSuppressesExecutionHintWhenLogsEvidenceIsSufficient(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-logs-suppress-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "先看 api 错误日志，再判断要不要上机器",
			},
		},
	}
	executed := executedToolContext{
		Attachments: []contracts.MessageAttachment{{
			Type: "file",
			Name: "logs-evidence.json",
		}},
		ExecutedPlan: []contracts.ToolPlanStep{{
			Tool:   "logs.query",
			Status: "completed",
			Output: map[string]interface{}{
				"artifact_count": 1,
				"result": map[string]interface{}{
					"result_count": 2,
					"summary":      "matched api errors",
				},
			},
		}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "uptime && cat /proc/loadavg")
	if !suppressed {
		t.Fatalf("expected execution hint to be suppressed")
	}
	if reason == "" {
		t.Fatalf("expected suppression reason")
	}
}

func TestDispatcherSuppressesGenericExecutionHintWhenObservabilityEvidenceIsSufficient(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-observe-suppress-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "trace api latency root cause",
			},
		},
	}
	executed := executedToolContext{
		ExecutedPlan: []contracts.ToolPlanStep{{
			Tool:   "observability.query",
			Status: "completed",
			Output: map[string]interface{}{
				"result_count": 2,
			},
		}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "hostname && uptime")
	if !suppressed {
		t.Fatalf("expected execution hint to be suppressed")
	}
	if reason == "" {
		t.Fatalf("expected suppression reason")
	}
}

func TestDispatcherKeepsExecutionHintForExplicitEnglishStatusCommand(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-explicit-english-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "run status command for api service",
			},
		},
	}
	executed := executedToolContext{
		ExecutedPlan: []contracts.ToolPlanStep{{
			Tool:   "metrics.query_range",
			Status: "completed",
			Output: map[string]interface{}{
				"series_count": 1,
				"points":       12,
			},
		}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "systemctl status api --no-pager --lines=20 || true")
	if suppressed {
		t.Fatalf("expected explicit English command to survive, reason=%s", reason)
	}
}

func TestDispatcherDoesNotSuppressGenericExecutionHintForAttachmentsWithoutReadOnlyEvidence(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-attachments-only-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "最近 api 报错",
			},
		},
	}
	executed := executedToolContext{
		Attachments: []contracts.MessageAttachment{{
			Type: "image",
			Name: "generic-screenshot.png",
		}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "hostname && uptime")
	if suppressed {
		t.Fatalf("expected attachments without read-only evidence not to suppress, reason=%s", reason)
	}
}

func TestDispatcherDoesNotSuppressGenericExecutionHintForDeliveryEvidenceOnly(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-delivery-only-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "最近 api 报错和最近一次发布有关系吗",
			},
		},
	}
	executed := executedToolContext{
		ExecutedPlan: []contracts.ToolPlanStep{{
			Tool:   "delivery.query",
			Status: "completed",
			Output: map[string]interface{}{
				"result": map[string]interface{}{
					"release": "2026.03.20",
				},
			},
		}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "hostname && uptime")
	if suppressed {
		t.Fatalf("expected delivery evidence alone not to suppress generic execution hint, reason=%s", reason)
	}
}

func TestDispatcherDoesNotSuppressGenericExecutionHintForDeliveryAttachmentOnlyEvidence(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		nil,
		staticReasoning{},
		nil,
		nil,
		nil,
		nil,
	)

	session := contracts.SessionDetail{
		SessionID: "ses-delivery-artifact-only-1",
		Alert: map[string]interface{}{
			"annotations": map[string]interface{}{
				"user_request": "最近 api 报错和最近一次发布有关系吗",
			},
		},
	}
	executed := executedToolContext{
		Attachments: []contracts.MessageAttachment{{
			Type: "file",
			Name: "release-diff.json",
		}},
		ExecutedPlan: []contracts.ToolPlanStep{{
			Tool:   "delivery.query",
			Status: "completed",
			Output: map[string]interface{}{
				"artifact_count": 1,
			},
		}},
	}

	suppressed, reason := dispatcher.shouldSuppressExecutionHint(session, executed, "hostname && uptime")
	if suppressed {
		t.Fatalf("expected delivery attachment-only evidence not to suppress generic execution hint, reason=%s", reason)
	}
}

func TestDispatcherRunOnceContinuesWhenOneNotificationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{
			ServiceOwners: map[string][]string{
				"api": {"445308292"},
			},
		}),
	})
	channelSvc := &failFirstChannel{}
	actionSvc := newActionService(t)
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		newFallbackReasoningService(),
		actionSvc,
		knowledge.NewService(),
		channelSvc,
		nil,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-5",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-1",
			"service":   "api",
			"severity":  "critical",
		},
		Annotations: map[string]string{"user_request": "重启 api"},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sessionDetail.Status != "pending_approval" {
		t.Fatalf("expected pending_approval, got %s", sessionDetail.Status)
	}
	if len(channelSvc.messages) != 1 {
		t.Fatalf("expected one successful notification after one failure, got %d", len(channelSvc.messages))
	}
	if len(channelSvc.messages[0].Actions) != 3 {
		t.Fatalf("expected approval notification to still be sent, got %+v", channelSvc.messages[0])
	}

	pendingOutbox, err := workflowSvc.ListOutbox(ctx, contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list pending outbox: %v", err)
	}
	if len(pendingOutbox) != 1 || pendingOutbox[0].Topic != "telegram.send" {
		t.Fatalf("expected one pending telegram retry outbox, got %+v", pendingOutbox)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run retry dispatcher: %v", err)
	}

	if len(channelSvc.messages) != 2 {
		t.Fatalf("expected retried diagnosis message to be delivered, got %d messages", len(channelSvc.messages))
	}

	failedOutbox, err := workflowSvc.ListOutbox(ctx, contracts.ListOutboxFilter{Status: "failed"})
	if err != nil {
		t.Fatalf("list failed outbox: %v", err)
	}
	if len(failedOutbox) != 0 {
		t.Fatalf("expected no failed outbox events, got %d", len(failedOutbox))
	}
}

func TestDispatcherDirectExecutionRoutesWebChatResultToInbox(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
		AuthorizationPolicy: authorization.New(authorization.Config{
			Defaults: authorization.Defaults{
				WhitelistAction: authorization.ActionDirectExecute,
				BlacklistAction: authorization.ActionSuggestOnly,
				UnmatchedAction: authorization.ActionRequireApproval,
			},
			SSH: authorization.SSHCommandConfig{
				NormalizeWhitespace: true,
				Whitelist:           []string{"uptime*"},
			},
		}),
	})
	channelSvc := &captureChannel{}
	auditLogger := &captureAuditLogger{}
	actionSvc := newActionService(t)
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		newFallbackReasoningService(),
		actionSvc,
		knowledge.NewService(),
		channelSvc,
		auditLogger,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "web_chat",
		Severity:    "info",
		Fingerprint: "web-chat:load:host-1",
		Labels: map[string]string{
			"alertname":      "TarsChatLoadRequest",
			"instance":       "host-1",
			"host":           "host-1",
			"severity":       "info",
			"chat_id":        "alice-user",
			"tars_chat":      "true",
			"tars_generated": "web_chat",
		},
		Annotations: map[string]string{
			"summary":      "检查系统负载",
			"user_request": "检查系统负载",
		},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}

	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sessionDetail.Status != "resolved" {
		t.Fatalf("expected resolved after evidence-only diagnosis, got %+v", sessionDetail)
	}
	if len(sessionDetail.Executions) != 0 {
		t.Fatalf("expected no execution drafts for read-only load diagnosis, got %+v", sessionDetail.Executions)
	}
	if len(channelSvc.messages) != 1 {
		t.Fatalf("expected only diagnosis message, got %d", len(channelSvc.messages))
	}
	resultMsg := channelSvc.messages[0]
	if resultMsg.Channel != "in_app_inbox" {
		t.Fatalf("expected inbox diagnosis message, got %+v", resultMsg)
	}
	if resultMsg.RefType != "session" || resultMsg.RefID == "" {
		t.Fatalf("expected session reference metadata, got %+v", resultMsg)
	}
	if resultMsg.Target != "alice-user" {
		t.Fatalf("expected web chat diagnosis target alice-user, got %+v", resultMsg)
	}
	if len(auditLogger.entries) == 0 {
		t.Fatalf("expected audit entries to be recorded")
	}
}

func TestDispatcherRunOnceIngestsResolvedSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled:       true,
		ApprovalEnabled:        true,
		ExecutionEnabled:       true,
		KnowledgeIngestEnabled: true,
	})
	knowledgeSvc := &captureKnowledge{}
	actionSvc := newActionService(t)
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		newFallbackReasoningService(),
		actionSvc,
		knowledgeSvc,
		&captureChannel{},
		nil,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-4",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-4",
			"service":   "api",
			"severity":  "critical",
		},
		Annotations: map[string]string{"user_request": "api 状态 执行"},
	})
	if err != nil {
		t.Fatalf("handle alert event: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run diagnosis dispatcher: %v", err)
	}

	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected 1 execution draft, got %d", len(sessionDetail.Executions))
	}

	dispatchResult, err := workflowSvc.HandleChannelEvent(ctx, contracts.ChannelEvent{
		EventType:   "approval",
		Channel:     "telegram",
		UserID:      "alice",
		ChatID:      "ops-room",
		Action:      "approve",
		ExecutionID: sessionDetail.Executions[0].ExecutionID,
	})
	if err != nil {
		t.Fatalf("approve execution: %v", err)
	}
	if len(dispatchResult.Executions) != 1 {
		t.Fatalf("expected 1 approved execution request, got %d", len(dispatchResult.Executions))
	}

	execResult, err := actionSvc.ExecuteApproved(ctx, dispatchResult.Executions[0])
	if err != nil {
		t.Fatalf("execute approved request: %v", err)
	}
	mutation, err := workflowSvc.HandleExecutionResult(ctx, execResult)
	if err != nil {
		t.Fatalf("handle execution result: %v", err)
	}
	verifyResult, err := actionSvc.VerifyExecution(ctx, contracts.VerificationRequest{
		SessionID:   result.SessionID,
		ExecutionID: dispatchResult.Executions[0].ExecutionID,
		TargetHost:  dispatchResult.Executions[0].TargetHost,
		Service:     "api",
	})
	if err != nil {
		t.Fatalf("verify execution: %v", err)
	}
	if mutation.Status != "verifying" {
		t.Fatalf("expected verifying status after execution, got %s", mutation.Status)
	}
	if _, err := workflowSvc.HandleVerificationResult(ctx, verifyResult); err != nil {
		t.Fatalf("handle verification result: %v", err)
	}

	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run knowledge dispatcher: %v", err)
	}

	if len(knowledgeSvc.events) != 1 {
		t.Fatalf("expected 1 knowledge ingest event, got %d", len(knowledgeSvc.events))
	}
	if knowledgeSvc.events[0].SessionID != result.SessionID {
		t.Fatalf("unexpected ingested session id: %s", knowledgeSvc.events[0].SessionID)
	}
}

func TestDispatcherUsesSkillRuntimeAsPrimaryPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	workflowSvc := workflow.NewService(workflow.Options{DiagnosisEnabled: true, ApprovalEnabled: true, ExecutionEnabled: true})
	auditLogger := &captureAuditLogger{}
	channelSvc := &captureChannel{}
	actionSvc := newActionService(t)
	skillPath := t.TempDir() + "/skills.yaml"
	if err := os.WriteFile(skillPath, []byte(`skills:
  entries:
    - api_version: tars.skill/v1alpha1
      kind: skill_package
      metadata:
        id: disk-space-incident
        name: disk-space-incident
        display_name: Disk Space Incident
        version: 1.0.0
        vendor: tars
        source: official
      spec:
        type: incident_skill
        triggers:
          alerts: ["DiskSpaceLow"]
        planner:
          summary: Skill runtime disk plan.
          preferred_tools: ["metrics.query_range"]
          steps:
            - id: metrics_capacity
              tool: metrics.query_range
              required: true
              reason: Skill runtime should force metrics first.
              params:
                query: node_filesystem_avail_bytes
                window: 1h
                step: 5m
`), 0o600); err != nil {
		t.Fatalf("write skills config: %v", err)
	}
	skillManager, err := skills.NewManager(skillPath, "")
	if err != nil {
		t.Fatalf("new skills manager: %v", err)
	}
	if _, _, err := skillManager.Promote("disk-space-incident", skills.PromoteOptions{OperatorReason: "activate skill", ReviewState: "approved", RuntimeMode: "planner_visible"}); err != nil {
		t.Fatalf("promote skill: %v", err)
	}

	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		staticReasoning{diagnosis: contracts.DiagnosisOutput{Summary: "planner fallback", ToolPlan: []contracts.ToolPlanStep{{ID: "exec_1", Tool: "execution.run_command", Input: map[string]interface{}{"command": "df -h"}}}}},
		actionSvc,
		knowledge.NewService(),
		channelSvc,
		auditLogger,
		nil,
		skillManager,
	)

	result, err := workflowSvc.HandleAlertEvent(ctx, contracts.AlertEvent{Source: "vmalert", Severity: "critical", Fingerprint: "DiskSpaceLow:host-1", Labels: map[string]string{"alertname": "DiskSpaceLow", "instance": "host-1", "host": "host-1", "service": "api", "severity": "critical"}, Annotations: map[string]string{"summary": "disk nearly full"}})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}
	if err := dispatcher.RunOnce(ctx); err != nil {
		t.Fatalf("run dispatcher: %v", err)
	}
	sessionDetail, err := workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(sessionDetail.ToolPlan) == 0 || sessionDetail.ToolPlan[0].Tool != "metrics.query_range" {
		t.Fatalf("expected skill runtime tool plan, got %+v", sessionDetail.ToolPlan)
	}
	for _, step := range sessionDetail.ToolPlan {
		if step.Tool == "execution.run_command" {
			t.Fatalf("did not expect planner execution step to override skill runtime: %+v", sessionDetail.ToolPlan)
		}
	}
	if !hasAuditAction(auditLogger.entries, "skill_selected_by_planner") {
		t.Fatalf("expected skill_selected_by_planner audit entry, got %+v", auditLogger.entries)
	}
	if !hasAuditAction(auditLogger.entries, "skill_expanded_to_tool_plan") {
		t.Fatalf("expected skill_expanded_to_tool_plan audit entry, got %+v", auditLogger.entries)
	}
}

func hasAuditAction(entries []audit.Entry, action string) bool {
	for _, entry := range entries {
		if entry.Action == action {
			return true
		}
	}
	return false
}

type captureChannel struct {
	messages []contracts.ChannelMessage
}

func (c *captureChannel) SendMessage(_ context.Context, msg contracts.ChannelMessage) (contracts.SendResult, error) {
	c.messages = append(c.messages, msg)
	return contracts.SendResult{MessageID: "msg-1"}, nil
}

type failFirstChannel struct {
	calls    int
	messages []contracts.ChannelMessage
}

type alwaysFailChannel struct{}

func (alwaysFailChannel) SendMessage(_ context.Context, _ contracts.ChannelMessage) (contracts.SendResult, error) {
	return contracts.SendResult{}, errors.New("telegram send failed: temporary outage")
}

func (c *failFirstChannel) SendMessage(_ context.Context, msg contracts.ChannelMessage) (contracts.SendResult, error) {
	c.calls++
	if c.calls == 1 {
		return contracts.SendResult{}, errors.New("telegram send failed: status=400 description=Bad Request: chat not found")
	}
	c.messages = append(c.messages, msg)
	return contracts.SendResult{MessageID: "msg-1"}, nil
}

type captureAuditLogger struct {
	entries []audit.Entry
}

func (c *captureAuditLogger) Log(_ context.Context, entry audit.Entry) {
	c.entries = append(c.entries, entry)
}

func newFallbackReasoningService() contracts.ReasoningService {
	return reasoning.NewService(reasoning.Options{LocalCommandFallbackEnable: true})
}

func newActionService(t *testing.T) *action.Service {
	t.Helper()

	return action.NewService(action.Options{
		Executor: &fakeExecutor{
			runFunc: func(_ context.Context, _ string, command string) (actionssh.Result, error) {
				if strings.HasPrefix(command, "systemctl is-active") {
					return actionssh.Result{
						ExitCode: 0,
						Output:   "active\n",
					}, nil
				}
				return actionssh.Result{
					ExitCode: 0,
					Output:   "hostname\nup 1 day",
				}, nil
			},
		},
		AllowedHosts:   []string{"host-1", "host-2", "host-4"},
		OutputSpoolDir: t.TempDir(),
	})
}

type fakeExecutor struct {
	result  actionssh.Result
	err     error
	runFunc func(context.Context, string, string) (actionssh.Result, error)
}

func (f *fakeExecutor) Run(ctx context.Context, host string, command string) (actionssh.Result, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, host, command)
	}
	return f.result, f.err
}

type failingReasoning struct {
	err error
}

func (f failingReasoning) BuildDiagnosis(_ context.Context, _ contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	return contracts.DiagnosisOutput{}, f.err
}

func (f failingReasoning) PlanDiagnosis(_ context.Context, _ contracts.DiagnosisInput) (contracts.DiagnosisPlan, error) {
	return contracts.DiagnosisPlan{}, f.err
}

func (f failingReasoning) FinalizeDiagnosis(_ context.Context, _ contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	return contracts.DiagnosisOutput{}, f.err
}

type staticReasoning struct {
	diagnosis contracts.DiagnosisOutput
}

type captureReasoning struct {
	lastPlan     contracts.DiagnosisInput
	lastFinalize contracts.DiagnosisInput
}

func (c *captureReasoning) BuildDiagnosis(_ context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	return contracts.DiagnosisOutput{Summary: "unused"}, nil
}

func (c *captureReasoning) PlanDiagnosis(_ context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisPlan, error) {
	c.lastPlan = input
	return contracts.DiagnosisPlan{Summary: "plan"}, nil
}

func (c *captureReasoning) FinalizeDiagnosis(_ context.Context, input contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	c.lastFinalize = input
	return contracts.DiagnosisOutput{Summary: "done"}, nil
}

func (s staticReasoning) BuildDiagnosis(_ context.Context, _ contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	return s.diagnosis, nil
}

func (s staticReasoning) PlanDiagnosis(_ context.Context, _ contracts.DiagnosisInput) (contracts.DiagnosisPlan, error) {
	return contracts.DiagnosisPlan{
		Summary:    s.diagnosis.Summary,
		ToolPlan:   s.diagnosis.ToolPlan,
		DesenseMap: s.diagnosis.DesenseMap,
	}, nil
}

func (s staticReasoning) FinalizeDiagnosis(_ context.Context, _ contracts.DiagnosisInput) (contracts.DiagnosisOutput, error) {
	return s.diagnosis, nil
}

func hasTimelineEvent(items []contracts.TimelineEvent, want string) bool {
	for _, item := range items {
		if item.Event == want {
			return true
		}
	}
	return false
}

type captureKnowledge struct {
	events []contracts.SessionClosedEvent
}

func (c *captureKnowledge) Search(_ context.Context, _ contracts.KnowledgeQuery) ([]contracts.KnowledgeHit, error) {
	return nil, nil
}

func (c *captureKnowledge) IngestResolvedSession(_ context.Context, event contracts.SessionClosedEvent) (contracts.KnowledgeIngestResult, error) {
	c.events = append(c.events, event)
	return contracts.KnowledgeIngestResult{
		DocumentID: "doc-" + event.SessionID,
		Chunks:     1,
	}, nil
}

func (c *captureKnowledge) ReindexDocuments(_ context.Context, _ string) error {
	return nil
}
