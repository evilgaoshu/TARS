package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/connectors"
)

func TestHandleAlertEventIdempotencySkipsTimelineMutation(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true})
	event := contracts.AlertEvent{
		Source:         "vmalert",
		Severity:       "critical",
		Fingerprint:    "HighCPU:host-1",
		IdempotencyKey: "vmalert:hash-1",
		RequestHash:    "hash-1",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-1",
			"severity":  "critical",
		},
		Annotations: map[string]string{
			"summary": "cpu too high",
		},
		ReceivedAt: time.Now().UTC(),
	}

	first, err := service.HandleAlertEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("first alert: %v", err)
	}
	second, err := service.HandleAlertEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("second alert: %v", err)
	}

	if second.SessionID != first.SessionID || !second.Duplicated {
		t.Fatalf("expected duplicate same session, got %+v", second)
	}

	session, err := service.GetSession(context.Background(), first.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(session.Timeline) != 2 {
		t.Fatalf("expected no extra timeline event on idempotent duplicate, got %d", len(session.Timeline))
	}
}

func TestHandleAlertEventFingerprintDuplicateStillAppendsTimeline(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true})
	firstEvent := contracts.AlertEvent{
		Source:         "vmalert",
		Severity:       "critical",
		Fingerprint:    "HighCPU:host-1",
		IdempotencyKey: "vmalert:hash-1",
		RequestHash:    "hash-1",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-1",
			"severity":  "critical",
		},
		Annotations: map[string]string{
			"summary": "cpu too high",
		},
		ReceivedAt: time.Now().UTC(),
	}
	secondEvent := contracts.AlertEvent{
		Source:         "vmalert",
		Severity:       "critical",
		Fingerprint:    "HighCPU:host-1",
		IdempotencyKey: "vmalert:hash-2",
		RequestHash:    "hash-2",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-1",
			"severity":  "critical",
		},
		Annotations: map[string]string{
			"summary": "cpu too high again",
		},
		ReceivedAt: time.Now().UTC(),
	}

	first, err := service.HandleAlertEvent(context.Background(), firstEvent)
	if err != nil {
		t.Fatalf("first alert: %v", err)
	}
	second, err := service.HandleAlertEvent(context.Background(), secondEvent)
	if err != nil {
		t.Fatalf("second alert: %v", err)
	}

	if second.SessionID != first.SessionID || !second.Duplicated {
		t.Fatalf("expected duplicate same session, got %+v", second)
	}

	session, err := service.GetSession(context.Background(), first.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(session.Timeline) != 3 {
		t.Fatalf("expected repeated alert to append timeline, got %d", len(session.Timeline))
	}
	if session.Timeline[2].Event != "alert_repeated" {
		t.Fatalf("unexpected repeated event: %+v", session.Timeline[2])
	}
}

func TestApplyDiagnosisBuildsApprovalMessagesWithRouting(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ApprovalTimeout:  10 * time.Minute,
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{
			ServiceOwners: map[string][]string{
				"api": {"owner-room"},
			},
		}),
	})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-9",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-9",
			"service":   "api",
			"severity":  "critical",
		},
		Annotations: map[string]string{
			"summary": "cpu too high",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outbox))
	}

	dispatchResult, err := service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:       "cpu is saturated",
		ExecutionHint: "hostname && uptime",
		Citations: []contracts.KnowledgeHit{
			{
				DocumentID: "doc-1",
				ChunkID:    "chunk-1",
				Snippet:    "restart guide for api service with a long line that should still be readable in telegram output without breaking the message layout",
			},
		},
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	if result.SessionID == "" {
		t.Fatalf("expected session id")
	}
	if len(dispatchResult.Notifications) != 2 {
		t.Fatalf("expected diagnosis + approval notifications, got %d", len(dispatchResult.Notifications))
	}
	if dispatchResult.Notifications[0].Target != "owner-room" {
		t.Fatalf("expected diagnosis notification target owner-room, got %s", dispatchResult.Notifications[0].Target)
	}
	diagnosisBody := dispatchResult.Notifications[0].Body
	for _, want := range []string{"[TARS] 诊断", "告警: HighCPU @ host-9", "服务 api", "结论: cpu is saturated", "下一步: hostname && uptime", "参考: 1 条知识", "会话:"} {
		if !strings.Contains(diagnosisBody, want) {
			t.Fatalf("expected diagnosis body to contain %q, got:\n%s", want, diagnosisBody)
		}
	}
	approvalMsg := dispatchResult.Notifications[1]
	if len(approvalMsg.Actions) != 3 {
		t.Fatalf("expected approval actions, got %+v", approvalMsg.Actions)
	}
	if approvalMsg.Target != "owner-room" {
		t.Fatalf("expected service owner target, got %s", approvalMsg.Target)
	}
	if approvalMsg.Actions[0].Value == "" || approvalMsg.Actions[0].Label == "" {
		t.Fatalf("unexpected approval action: %+v", approvalMsg.Actions[0])
	}
}

func TestGetSessionBuildsGoldenSummaryAndNotificationDigests(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ApprovalTimeout:  10 * time.Minute,
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{
			ServiceOwners: map[string][]string{"api": {"owner-room"}},
		}),
	})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "DiskFull:host-1",
		Labels: map[string]string{
			"alertname": "DiskFull",
			"instance":  "host-1",
			"service":   "api",
			"severity":  "critical",
		},
		Annotations: map[string]string{
			"summary": "disk usage high",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outbox))
	}

	_, err = service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:       "### Evidence\nDisk usage reached 95%.\n\n### Conclusion\nTemp files accumulated.\n\n### Recommended Action\nClean temp files after approval.",
		ExecutionHint: "rm -rf /tmp/cache/*",
		ToolPlan: []contracts.ToolPlanStep{{
			ID:     "step-1",
			Tool:   "metrics.query_range",
			Reason: "确认磁盘使用率曲线",
			Status: "completed",
		}},
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}

	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.GoldenSummary == nil {
		t.Fatalf("expected golden summary, got nil")
	}
	if session.GoldenSummary.Conclusion != "Temp files accumulated." {
		t.Fatalf("unexpected conclusion: %+v", session.GoldenSummary)
	}
	if session.GoldenSummary.NextAction == "" || !strings.Contains(session.GoldenSummary.NextAction, "等待审批") {
		t.Fatalf("expected pending approval next action, got %+v", session.GoldenSummary)
	}
	if len(session.GoldenSummary.Evidence) == 0 || !strings.Contains(session.GoldenSummary.Evidence[0], "Disk usage reached 95%") {
		t.Fatalf("unexpected evidence: %+v", session.GoldenSummary)
	}
	if len(session.Notifications) != 2 {
		t.Fatalf("expected diagnosis and approval notification digests, got %+v", session.Notifications)
	}
	if session.Notifications[0].Reason != "发送诊断结论" || session.Notifications[1].Reason != "请求人工审批" {
		t.Fatalf("unexpected notification reasons: %+v", session.Notifications)
	}
	if len(session.Executions) != 1 || session.Executions[0].GoldenSummary == nil {
		t.Fatalf("expected execution golden summary, got %+v", session.Executions)
	}
	if session.Executions[0].GoldenSummary.Approval == "" || !strings.Contains(session.Executions[0].GoldenSummary.Approval, "待审批") {
		t.Fatalf("unexpected execution approval summary: %+v", session.Executions[0].GoldenSummary)
	}
}

func TestGetExecutionBuildsGoldenSummaryWithSessionContext(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
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
				Whitelist:           []string{"systemctl restart *"},
			},
		}),
	})
	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "warning",
		Fingerprint: "CPUHigh:host-2",
		Labels: map[string]string{
			"alertname": "CPUHigh",
			"instance":  "host-2",
			"service":   "worker",
			"severity":  "warning",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}
	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected outbox event, got %d", len(outbox))
	}
	dispatchResult, err := service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:       "CPU load stays elevated; restart worker after inspection.",
		ExecutionHint: "systemctl restart worker",
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	if len(dispatchResult.Executions) != 1 {
		t.Fatalf("expected direct execution request, got %+v", dispatchResult.Executions)
	}
	executionID := dispatchResult.Executions[0].ExecutionID
	_, err = service.HandleExecutionResult(context.Background(), contracts.ExecutionResult{
		ExecutionID:   executionID,
		SessionID:     result.SessionID,
		Status:        "completed",
		ExitCode:      0,
		OutputPreview: "worker restarted successfully",
		OutputBytes:   27,
	})
	if err != nil {
		t.Fatalf("handle execution result: %v", err)
	}
	_, err = service.HandleVerificationResult(context.Background(), contracts.VerificationResult{
		SessionID:   result.SessionID,
		ExecutionID: executionID,
		Status:      "success",
		Summary:     "worker has recovered",
		CheckedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("handle verification result: %v", err)
	}

	execution, err := service.GetExecution(context.Background(), executionID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if execution.SessionID != result.SessionID {
		t.Fatalf("expected session id on execution, got %+v", execution)
	}
	if execution.GoldenSummary == nil {
		t.Fatalf("expected execution golden summary")
	}
	if !strings.Contains(execution.GoldenSummary.Result, "校验状态 success") {
		t.Fatalf("unexpected execution result summary: %+v", execution.GoldenSummary)
	}
	if !strings.Contains(execution.GoldenSummary.NextAction, "会话已闭环") {
		t.Fatalf("unexpected execution next action: %+v", execution.GoldenSummary)
	}
}

func TestApplyDiagnosisStoresDesenseMapOnSession(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  false,
	})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-9",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-9",
			"severity":  "critical",
		},
		Annotations: map[string]string{
			"summary": "cpu too high",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outbox))
	}

	_, err = service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:    "cpu is saturated",
		DesenseMap: map[string]string{"[HOST_1]": "host-9"},
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}

	record := service.sessions[result.SessionID]
	if record == nil {
		t.Fatalf("expected session record")
	}
	if record.desenseMap["[HOST_1]"] != "host-9" {
		t.Fatalf("unexpected stored desense map: %+v", record.desenseMap)
	}
}

func TestApplyDiagnosisRehydratesExecutionHintFromDesenseMap(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
	})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
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
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected 1 outbox event, got %d", len(outbox))
	}

	dispatchResult, err := service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:       "delete [PATH_1] on [IP_1]",
		ExecutionHint: "rm [PATH_1]",
		DesenseMap: map[string]string{
			"[PATH_1]": "/tmp/1.txt",
			"[IP_1]":   "192.168.3.106",
		},
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	if len(dispatchResult.Notifications) < 2 {
		t.Fatalf("expected diagnosis + approval notifications, got %+v", dispatchResult.Notifications)
	}

	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(session.Executions) != 1 {
		t.Fatalf("expected one execution draft, got %+v", session.Executions)
	}
	if session.Executions[0].Command != "rm /tmp/1.txt" {
		t.Fatalf("expected rehydrated command, got %+v", session.Executions[0])
	}
	if !strings.Contains(dispatchResult.Notifications[0].Body, "rm /tmp/1.txt") {
		t.Fatalf("expected diagnosis message to contain rehydrated command, got %s", dispatchResult.Notifications[0].Body)
	}
	if !strings.Contains(dispatchResult.Notifications[1].Body, "rm /tmp/1.txt") {
		t.Fatalf("expected approval message to contain rehydrated command, got %s", dispatchResult.Notifications[1].Body)
	}
}

func TestCreateCapabilityApprovalAndApproveDispatchesCapabilityRequest(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{
			ServiceOwners: map[string][]string{"api": {"owner-room"}},
		}),
	})

	alertResult, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "warning",
		Fingerprint: "cap-approval:host-1",
		Labels: map[string]string{
			"alertname": "CapabilityApproval",
			"instance":  "host-1",
			"service":   "api",
			"severity":  "warning",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	_, _, err = service.CreateCapabilityApproval(context.Background(), contracts.ApprovedCapabilityRequest{
		SessionID:    alertResult.SessionID,
		StepID:       "step_1",
		ConnectorID:  "delivery-main",
		CapabilityID: "deployment.promote",
		Params:       map[string]interface{}{"service": "api"},
		RequestedBy:  "tool_plan_executor",
		Runtime: &contracts.RuntimeMetadata{
			Runtime:       "capability",
			ConnectorID:   "delivery-main",
			ConnectorType: "delivery",
			Protocol:      "delivery_api",
		},
	})
	if err != nil {
		t.Fatalf("create capability approval: %v", err)
	}

	session, err := service.GetSession(context.Background(), alertResult.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != "pending_approval" {
		t.Fatalf("expected pending approval session, got %+v", session)
	}
	if len(session.Executions) != 1 {
		t.Fatalf("expected one approval record, got %+v", session.Executions)
	}
	if session.Executions[0].RequestKind != "capability" {
		t.Fatalf("expected capability request kind, got %+v", session.Executions[0])
	}

	dispatchResult, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{
		Channel:     "telegram",
		ChatID:      "owner-room",
		UserID:      "alice",
		Action:      "approve",
		ExecutionID: session.Executions[0].ExecutionID,
	})
	if err != nil {
		t.Fatalf("approve capability request: %v", err)
	}
	if len(dispatchResult.Capabilities) != 1 {
		t.Fatalf("expected approved capability dispatch, got %+v", dispatchResult)
	}
	if dispatchResult.Capabilities[0].CapabilityID != "deployment.promote" {
		t.Fatalf("unexpected capability dispatch: %+v", dispatchResult.Capabilities[0])
	}
}

func TestApplyDiagnosisDirectExecuteByAuthorizationPolicy(t *testing.T) {
	t.Parallel()

	manager := writeWorkflowTestConnectorConfig(t)
	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
		Connectors:       manager,
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
	if service.authorizationPolicy == nil {
		t.Fatalf("expected authorization policy to be set")
	}
	decision := service.resolveAuthorizationDecision(map[string]interface{}{
		"source": "telegram_chat",
		"host":   "host-9",
		"labels": map[string]interface{}{
			"service": "sshd",
		},
	}, "uptime && cat /proc/loadavg")
	if decision.Action != authorization.ActionDirectExecute {
		t.Fatalf("expected direct_execute decision, got %+v", decision)
	}

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "telegram_chat",
		Severity:    "info",
		Fingerprint: "chat:load",
		Labels: map[string]string{
			"alertname": "TarsChatLoadRequest",
			"instance":  "host-9",
			"host":      "host-9",
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
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}

	dispatchResult, err := service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:       "load is normal",
		ExecutionHint: "uptime && cat /proc/loadavg",
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	if len(dispatchResult.Executions) != 1 {
		t.Fatalf("expected immediate execution, got %+v", dispatchResult)
	}
	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != "executing" {
		t.Fatalf("expected executing status, got %+v", session)
	}
	if !hasTimelineEvent(session.Timeline, "execution_direct_ready") {
		t.Fatalf("expected execution_direct_ready timeline, got %+v", session.Timeline)
	}
	if !timelineContains(session.Timeline, "execution_direct_ready", "connector=jumpserver-main protocol=jumpserver_api mode=jumpserver_job") {
		t.Fatalf("expected runtime metadata in execution_direct_ready timeline, got %+v", session.Timeline)
	}
	if got := session.Executions[0]; got.ConnectorID != "jumpserver-main" || got.Protocol != "jumpserver_api" || got.ExecutionMode != "jumpserver_job" {
		t.Fatalf("expected jumpserver runtime selection, got %+v", got)
	}
}

func TestApplyDiagnosisApprovalUsesConnectorRuntimeMetadata(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		Connectors:       writeWorkflowTestConnectorConfig(t),
	})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-jumpserver",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "192.168.3.106",
			"host":      "192.168.3.106",
			"service":   "sshd",
			"severity":  "critical",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}
	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	_, err = service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:       "restart sshd",
		ExecutionHint: "systemctl restart sshd",
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if len(session.Executions) != 1 {
		t.Fatalf("expected one execution draft, got %+v", session.Executions)
	}
	got := session.Executions[0]
	if got.ConnectorID != "jumpserver-main" || got.ConnectorType != "execution" || got.ConnectorVendor != "jumpserver" || got.Protocol != "jumpserver_api" || got.ExecutionMode != "jumpserver_job" {
		t.Fatalf("unexpected execution runtime metadata: %+v", got)
	}
	if !timelineContains(session.Timeline, "execution_draft_ready", "connector=jumpserver-main protocol=jumpserver_api mode=jumpserver_job") {
		t.Fatalf("expected runtime metadata in execution_draft_ready timeline, got %+v", session.Timeline)
	}
}

func writeWorkflowTestConnectorConfig(t *testing.T) *connectors.Manager {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "connectors.yaml")
	content := `connectors:
  entries:
    - api_version: tars.connector/v1alpha1
      kind: connector
      metadata:
        id: jumpserver-main
        name: jumpserver
        display_name: JumpServer Main
        vendor: jumpserver
        version: 1.0.0
      spec:
        type: execution
        protocol: jumpserver_api
        import_export:
          exportable: true
          importable: true
          formats: ["yaml"]
      compatibility:
        tars_major_versions: ["1"]
`
	if err := os.WriteFile(configPath, []byte("connectors:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("seed connectors config: %v", err)
	}
	manager, err := connectors.NewManager(configPath)
	if err != nil {
		t.Fatalf("new connectors manager: %v", err)
	}
	if err := manager.Save(content); err != nil {
		t.Fatalf("save connectors config: %v", err)
	}
	return manager
}

func TestApplyDiagnosisSuggestOnlyResolvesTelegramChat(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		AuthorizationPolicy: authorization.New(authorization.Config{
			Defaults: authorization.Defaults{
				WhitelistAction: authorization.ActionDirectExecute,
				BlacklistAction: authorization.ActionSuggestOnly,
				UnmatchedAction: authorization.ActionRequireApproval,
			},
			SSH: authorization.SSHCommandConfig{
				NormalizeWhitespace: true,
				Blacklist:           []string{"systemctl restart *"},
			},
		}),
	})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "telegram_chat",
		Severity:    "info",
		Fingerprint: "chat:restart",
		Labels: map[string]string{
			"alertname": "TarsChatRestartRequest",
			"instance":  "host-9",
			"host":      "host-9",
			"service":   "sshd",
			"severity":  "info",
			"chat_id":   "445308292",
			"tars_chat": "true",
		},
		Annotations: map[string]string{
			"summary":      "重启 sshd",
			"user_request": "重启 sshd",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}

	dispatchResult, err := service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary:       "restart may help",
		ExecutionHint: "systemctl restart sshd",
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	if len(dispatchResult.Executions) != 0 {
		t.Fatalf("expected no immediate execution, got %+v", dispatchResult.Executions)
	}
	if len(dispatchResult.Notifications) < 2 {
		t.Fatalf("expected diagnosis plus policy notification, got %+v", dispatchResult.Notifications)
	}
	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != "resolved" {
		t.Fatalf("expected resolved status for suggest_only chat, got %+v", session)
	}
	if !hasTimelineEvent(session.Timeline, "execution_suggested_only") {
		t.Fatalf("expected execution_suggested_only timeline, got %+v", session.Timeline)
	}
}

func TestApplyDiagnosisRoutesWebChatNotificationsToInbox(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "web_chat",
		Severity:    "info",
		Fingerprint: "web-chat:load:host-9",
		Labels: map[string]string{
			"alertname":      "TarsChatLoadRequest",
			"instance":       "host-9",
			"host":           "host-9",
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
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected one outbox event, got %d", len(outbox))
	}

	dispatchResult, err := service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{Summary: "load is normal"})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	if len(dispatchResult.Notifications) != 1 {
		t.Fatalf("expected one diagnosis notification, got %+v", dispatchResult.Notifications)
	}
	msg := dispatchResult.Notifications[0]
	if msg.Channel != "in_app_inbox" {
		t.Fatalf("expected inbox channel, got %+v", msg)
	}
	if msg.Target != "alice-user" {
		t.Fatalf("expected web chat target alice-user, got %+v", msg)
	}
	if msg.RefType != "session" || msg.RefID != result.SessionID {
		t.Fatalf("expected session reference metadata, got %+v", msg)
	}

	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != "resolved" {
		t.Fatalf("expected resolved status, got %+v", session)
	}
	if !hasTimelineEvent(session.Timeline, "chat_answer_completed") {
		t.Fatalf("expected chat_answer_completed timeline, got %+v", session.Timeline)
	}
}

func TestApplyDiagnosisResolvesOpsSetupEvidenceOnlySession(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true, KnowledgeIngestEnabled: false})

	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "warning",
		Fingerprint: "ops-setup:tool-plan:host-9",
		Labels: map[string]string{
			"alertname":      "TarsToolPlanLiveValidation",
			"instance":       "host-9",
			"host":           "host-9",
			"service":        "api",
			"severity":       "warning",
			"tars_generated": "ops_setup",
		},
		Annotations: map[string]string{
			"summary":      "最近 api 报错和最近一次发布有关系吗",
			"user_request": "最近 api 报错和最近一次发布有关系吗",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected one outbox event, got %d", len(outbox))
	}

	dispatchResult, err := service.ApplyDiagnosis(context.Background(), outbox[0].EventID, contracts.DiagnosisOutput{
		Summary: "已通过 metrics、logs、observability、delivery 收集到只读证据",
		ToolPlan: []contracts.ToolPlanStep{{
			ID:     "metrics_1",
			Tool:   "metrics.query_range",
			Status: "completed",
			Output: map[string]interface{}{"result_count": 1},
		}},
		Attachments: []contracts.MessageAttachment{{
			Name: "metrics-range.json",
			Type: "application/json",
		}},
	})
	if err != nil {
		t.Fatalf("apply diagnosis: %v", err)
	}
	if len(dispatchResult.Executions) != 0 {
		t.Fatalf("expected no executions for evidence-only ops_setup session, got %+v", dispatchResult.Executions)
	}

	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != "resolved" {
		t.Fatalf("expected resolved status for ops_setup evidence-only session, got %+v", session)
	}
	if !hasTimelineEvent(session.Timeline, "chat_answer_completed") {
		t.Fatalf("expected completion timeline for ops_setup evidence-only session, got %+v", session.Timeline)
	}

	items, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "blocked"})
	if err != nil {
		t.Fatalf("list blocked outbox: %v", err)
	}
	found := false
	for _, item := range items {
		if item.Topic == "session.closed" && item.AggregateID == result.SessionID && item.BlockedReason == "knowledge_ingest_disabled" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected session.closed outbox event for resolved ops_setup session, got %+v", items)
	}
}

func TestRecoverProcessingOutboxMovesEventsBackToPending(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true})
	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-10",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-10",
			"severity":  "critical",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	outbox, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("claim outbox: %v", err)
	}
	if len(outbox) != 1 {
		t.Fatalf("expected one outbox item, got %d", len(outbox))
	}

	recovered, err := service.RecoverProcessingOutbox(context.Background())
	if err != nil {
		t.Fatalf("recover processing outbox: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered outbox event, got %d", recovered)
	}

	outboxItems, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{})
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(outboxItems) != 0 {
		t.Fatalf("expected no blocked/failed outbox items, got %+v", outboxItems)
	}

	reclaimed, err := service.ClaimOutboxBatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("reclaim outbox: %v", err)
	}
	if len(reclaimed) != 1 || reclaimed[0].AggregateID != result.SessionID {
		t.Fatalf("expected recovered event to be claimable again, got %+v", reclaimed)
	}
}

func TestDeleteOutboxRemovesFailedOrBlockedEvent(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: false})
	result, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "warning",
		Fingerprint: "DiskFull:host-4",
		Labels: map[string]string{
			"alertname": "DiskFull",
			"instance":  "host-4",
			"severity":  "warning",
		},
		Annotations: map[string]string{
			"summary": "disk usage high",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	items, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "blocked"})
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 blocked outbox item, got %d", len(items))
	}

	if err := service.DeleteOutbox(context.Background(), items[0].ID, "cleanup historical residue"); err != nil {
		t.Fatalf("delete outbox: %v", err)
	}

	items, err = service.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "blocked"})
	if err != nil {
		t.Fatalf("list outbox after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected blocked outbox to be empty, got %+v", items)
	}

	session, err := service.GetSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Timeline[len(session.Timeline)-1].Event != "outbox_deleted" {
		t.Fatalf("expected outbox_deleted timeline event, got %+v", session.Timeline[len(session.Timeline)-1])
	}
}

func TestDeleteOutboxRejectsPendingEvent(t *testing.T) {
	t.Parallel()

	service := NewService(Options{DiagnosisEnabled: true})
	_, err := service.HandleAlertEvent(context.Background(), contracts.AlertEvent{
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "HighCPU:host-5",
		Labels: map[string]string{
			"alertname": "HighCPU",
			"instance":  "host-5",
			"severity":  "critical",
		},
		Annotations: map[string]string{
			"summary": "cpu too high",
		},
	})
	if err != nil {
		t.Fatalf("handle alert: %v", err)
	}

	items, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("list pending outbox: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pending outbox item, got %d", len(items))
	}

	err = service.DeleteOutbox(context.Background(), items[0].ID, "should fail")
	if err == nil {
		t.Fatal("expected invalid state error")
	}
	if err != contracts.ErrInvalidState {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}
}

func TestCreateCapabilityApprovalReturnsErrNotFoundForMissingSession(t *testing.T) {
	t.Parallel()
	svc := NewService(Options{DiagnosisEnabled: true})
	_, _, err := svc.CreateCapabilityApproval(context.Background(), contracts.ApprovedCapabilityRequest{
		SessionID: "nonexistent-session-id",
	})
	if err != contracts.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func hasTimelineEvent(items []contracts.TimelineEvent, want string) bool {
	for _, item := range items {
		if item.Event == want {
			return true
		}
	}
	return false
}

func timelineContains(items []contracts.TimelineEvent, wantEvent string, wantText string) bool {
	for _, item := range items {
		if item.Event == wantEvent && strings.Contains(item.Message, wantText) {
			return true
		}
	}
	return false
}
