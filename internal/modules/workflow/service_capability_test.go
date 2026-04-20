package workflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/reasoning"
)

func TestCreateCapabilityApprovalUpdatesSessionAndToolPlan(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		ApprovalEnabled: true,
		ApprovalTimeout: 5 * time.Minute,
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{
			ServiceOwners: map[string][]string{"api": {"owner-room"}},
		}),
	})
	sessionID := mustCreateWorkflowSession(t, service, "capability-approval", "host-capability", "api")
	service.sessions[sessionID].detail.ToolPlan = []contracts.ToolPlanStep{{
		ID:     "step-1",
		Tool:   "connector.invoke_capability",
		Reason: "promote api canary",
	}}

	detail, messages, err := service.CreateCapabilityApproval(context.Background(), contracts.ApprovedCapabilityRequest{
		SessionID:    sessionID,
		StepID:       "step-1",
		ConnectorID:  "delivery-main",
		CapabilityID: "deployment.promote",
		Params:       map[string]interface{}{"service": "api", "percent": 50},
		Runtime: &contracts.RuntimeMetadata{
			Runtime:         "capability",
			ConnectorID:     "delivery-main",
			ConnectorType:   "delivery",
			ConnectorVendor: "internal",
			Protocol:        "delivery_api",
			ExecutionMode:   "sync",
		},
	})
	if err != nil {
		t.Fatalf("create capability approval: %v", err)
	}

	if !strings.HasPrefix(detail.ExecutionID, "cap-") {
		t.Fatalf("expected generated capability approval id, got %+v", detail)
	}
	if detail.RequestedBy != "tars" || detail.Status != "pending" {
		t.Fatalf("expected default requester and pending status, got %+v", detail)
	}
	if detail.ConnectorType != "delivery" || detail.Protocol != "delivery_api" || detail.ExecutionMode != "sync" {
		t.Fatalf("expected runtime fields to be copied onto execution detail, got %+v", detail)
	}
	if len(messages) != 1 || messages[0].Target != "owner-room" {
		t.Fatalf("expected approval messages routed to owner-room, got %+v", messages)
	}

	session, err := service.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != "pending_approval" || len(session.Executions) != 1 {
		t.Fatalf("expected pending approval session with execution, got %+v", session)
	}
	step := session.ToolPlan[0]
	if step.Status != "pending_approval" {
		t.Fatalf("expected tool plan step to move to pending_approval, got %+v", step)
	}
	if step.Output["approval_id"] != detail.ExecutionID || step.Output["status"] != "pending_approval" {
		t.Fatalf("expected approval metadata on tool plan step, got %+v", step.Output)
	}
	if step.CompletedAt.IsZero() {
		t.Fatalf("expected tool plan step completion timestamp, got %+v", step)
	}
}

func TestHandleCapabilityResultTransitionsSession(t *testing.T) {
	t.Parallel()

	t.Run("completed capability updates tool plan and attachments", func(t *testing.T) {
		t.Parallel()

		service := newCapabilityResultService(t)
		sessionID, approvalID := createCapabilityApprovalForResult(t, service, "completed")

		result, err := service.HandleCapabilityResult(context.Background(), contracts.CapabilityExecutionResult{
			ApprovalID:   approvalID,
			SessionID:    sessionID,
			StepID:       "step-1",
			Status:       "completed",
			ConnectorID:  "delivery-main",
			CapabilityID: "deployment.promote",
			Output:       map[string]interface{}{"release": "r-42"},
			Metadata:     map[string]interface{}{"environment": "prod"},
			Artifacts: []contracts.MessageAttachment{{
				Type:     "text",
				Name:     "release.txt",
				Content:  "promoted to 50%",
				MimeType: "text/plain",
			}},
			Runtime: &contracts.RuntimeMetadata{ConnectorID: "delivery-main", Protocol: "delivery_api"},
		})
		if err != nil {
			t.Fatalf("handle capability result: %v", err)
		}
		if result.Status != "open" {
			t.Fatalf("expected session to reopen after completed capability, got %+v", result)
		}

		session, err := service.GetSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		if session.Status != "open" || len(session.Attachments) != 1 {
			t.Fatalf("expected open session with artifact attachment, got %+v", session)
		}
		if !hasTimelineEvent(session.Timeline, "capability_completed") {
			t.Fatalf("expected capability_completed timeline, got %+v", session.Timeline)
		}
		step := session.ToolPlan[0]
		if step.Status != "completed" {
			t.Fatalf("expected completed tool plan step, got %+v", step)
		}
		if step.Output["approval_id"] != approvalID || step.Output["status"] != "completed" {
			t.Fatalf("expected approval metadata on completed step, got %+v", step.Output)
		}
		if _, ok := step.Output["result"]; !ok {
			t.Fatalf("expected result payload on step output, got %+v", step.Output)
		}
		if _, ok := step.Output["metadata"]; !ok {
			t.Fatalf("expected metadata payload on step output, got %+v", step.Output)
		}
	})

	t.Run("failed capability marks session failed", func(t *testing.T) {
		t.Parallel()

		service := newCapabilityResultService(t)
		sessionID, approvalID := createCapabilityApprovalForResult(t, service, "failed")

		result, err := service.HandleCapabilityResult(context.Background(), contracts.CapabilityExecutionResult{
			ApprovalID:   approvalID,
			SessionID:    sessionID,
			StepID:       "step-1",
			Status:       "failed",
			ConnectorID:  "delivery-main",
			CapabilityID: "deployment.promote",
			Error:        "rollback triggered",
		})
		if err != nil {
			t.Fatalf("handle capability failure: %v", err)
		}
		if result.Status != "failed" {
			t.Fatalf("expected failed session result, got %+v", result)
		}

		session, err := service.GetSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		if session.Status != "failed" {
			t.Fatalf("expected failed session status, got %+v", session)
		}
		if !timelineContains(session.Timeline, "capability_failed", "rollback triggered") {
			t.Fatalf("expected capability_failed timeline entry, got %+v", session.Timeline)
		}
		if session.ToolPlan[0].Output["error"] != "rollback triggered" {
			t.Fatalf("expected capability error on step output, got %+v", session.ToolPlan[0].Output)
		}
	})
}

func TestHandleVerificationResultUpdatesSessionAndClosedOutbox(t *testing.T) {
	t.Parallel()

	t.Run("success resolves session and enqueues session.closed", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{KnowledgeIngestEnabled: false})
		sessionID := mustCreateWorkflowSession(t, service, "verify-success", "host-verify-success", "api")

		result, err := service.HandleVerificationResult(context.Background(), contracts.VerificationResult{
			SessionID: sessionID,
			Status:    "success",
			Summary:   "api recovered",
			Details:   map[string]interface{}{"checks": 3},
			Runtime:   &contracts.RuntimeMetadata{ConnectorID: "verifier"},
		})
		if err != nil {
			t.Fatalf("handle verification success: %v", err)
		}
		if result.Status != "resolved" {
			t.Fatalf("expected resolved session result, got %+v", result)
		}

		session, err := service.GetSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		if session.Status != "resolved" || session.Verification == nil || session.Verification.Status != "success" {
			t.Fatalf("expected resolved verification state, got %+v", session)
		}
		if !hasTimelineEvent(session.Timeline, "verify_success") {
			t.Fatalf("expected verify_success timeline, got %+v", session.Timeline)
		}

		items, err := service.ListOutbox(context.Background(), contracts.ListOutboxFilter{Status: "blocked"})
		if err != nil {
			t.Fatalf("list outbox after verification: %v", err)
		}
		found := false
		for _, item := range items {
			if item.Topic == "session.closed" && item.AggregateID == sessionID && item.BlockedReason == "knowledge_ingest_disabled" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected blocked session.closed outbox event, got %+v", items)
		}
	})

	t.Run("failed verification returns session to analyzing", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{})
		sessionID := mustCreateWorkflowSession(t, service, "verify-failed", "host-verify-failed", "api")

		result, err := service.HandleVerificationResult(context.Background(), contracts.VerificationResult{
			SessionID: sessionID,
			Status:    "failed",
			Summary:   "health check still failing",
			CheckedAt: time.Date(2026, time.April, 2, 13, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("handle verification failure: %v", err)
		}
		if result.Status != "analyzing" {
			t.Fatalf("expected analyzing session result, got %+v", result)
		}

		session, err := service.GetSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("get session: %v", err)
		}
		if session.Status != "analyzing" || session.Verification == nil || session.Verification.CheckedAt.IsZero() {
			t.Fatalf("expected analyzing verification state, got %+v", session)
		}
		if !timelineContains(session.Timeline, "verify_failed", "health check still failing") {
			t.Fatalf("expected verify_failed timeline entry, got %+v", session.Timeline)
		}
	})
}

func TestSweepApprovalTimeoutsRejectsExpiredRequests(t *testing.T) {
	t.Parallel()

	service := NewService(Options{
		ApprovalEnabled: true,
		ApprovalTimeout: 5 * time.Minute,
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{
			ServiceOwners: map[string][]string{"api": {"owner-room"}},
		}),
	})
	sessionID := mustCreateWorkflowSession(t, service, "approval-timeout", "host-timeout", "api")
	service.sessions[sessionID].detail.Status = "pending_approval"
	service.executions["exe-expired"] = contracts.ExecutionDetail{
		ExecutionID:   "exe-expired",
		Status:        "pending",
		RequestedBy:   "tars",
		ApprovalGroup: "team:api",
		CreatedAt:     time.Date(2026, time.April, 2, 9, 0, 0, 0, time.UTC),
	}
	service.executions["exe-fresh"] = contracts.ExecutionDetail{
		ExecutionID:   "exe-fresh",
		Status:        "pending",
		RequestedBy:   "tars",
		ApprovalGroup: "team:api",
		CreatedAt:     time.Date(2026, time.April, 2, 13, 2, 0, 0, time.UTC),
	}
	service.executionSession["exe-expired"] = sessionID
	service.executionSession["exe-fresh"] = sessionID
	service.sessions[sessionID].detail.Executions = []contracts.ExecutionDetail{
		service.executions["exe-expired"],
		service.executions["exe-fresh"],
	}

	notifications, err := service.SweepApprovalTimeouts(context.Background(), time.Date(2026, time.April, 2, 13, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweep approval timeouts: %v", err)
	}
	if len(notifications) != 1 || notifications[0].Target != "owner-room" {
		t.Fatalf("expected one timeout notification to owner-room, got %+v", notifications)
	}
	if service.executions["exe-expired"].Status != "rejected" || service.executions["exe-fresh"].Status != "pending" {
		t.Fatalf("expected only expired execution to be rejected, got expired=%+v fresh=%+v", service.executions["exe-expired"], service.executions["exe-fresh"])
	}

	session, err := service.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if session.Status != "open" {
		t.Fatalf("expected session to reopen after timeout rejection, got %+v", session)
	}
	if !timelineContains(session.Timeline, "approval_timed_out", "exe-expired") {
		t.Fatalf("expected approval_timed_out timeline entry, got %+v", session.Timeline)
	}
}

func TestEnqueueNotificationsPublishesOutboxEvents(t *testing.T) {
	t.Parallel()

	t.Run("success publishes telegram events", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{})
		sessionID := mustCreateWorkflowSession(t, service, "enqueue-success", "host-enqueue-success", "api")
		message := contracts.ChannelMessage{
			Channel: "telegram",
			Target:  "owner-room",
			Subject: "Diagnosis",
			Body:    "service recovered",
			RefType: "session",
			RefID:   sessionID,
		}

		if err := service.EnqueueNotifications(context.Background(), sessionID, []contracts.ChannelMessage{message}); err != nil {
			t.Fatalf("enqueue notifications: %v", err)
		}

		var found contracts.EventEnvelope
		for eventID, item := range service.outbox {
			if item.Topic == "telegram.send" && item.AggregateID == sessionID {
				found = service.outboxEnvelope(eventID, *item)
			}
		}
		if found.EventID == "" {
			t.Fatalf("expected telegram.send outbox event to be published")
		}

		decoded, err := contracts.DecodeChannelMessage(found.Payload)
		if err != nil {
			t.Fatalf("decode outbox payload: %v", err)
		}
		if decoded.Target != "owner-room" || decoded.Body != "service recovered" {
			t.Fatalf("expected payload to encode original message, got %+v", decoded)
		}
	})

	t.Run("encoding error aborts enqueue", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{})
		sessionID := mustCreateWorkflowSession(t, service, "enqueue-error", "host-enqueue-error", "api")
		err := service.EnqueueNotifications(context.Background(), sessionID, []contracts.ChannelMessage{{
			Channel: "telegram",
			Target:  "owner-room",
			Body:    "bad payload",
			Attachments: []contracts.MessageAttachment{{
				Type: "json",
				Metadata: map[string]interface{}{
					"unsupported": func() {},
				},
			}},
		}})
		if err == nil {
			t.Fatal("expected JSON encoding error")
		}
		for _, item := range service.outbox {
			if item.Topic == "telegram.send" && item.AggregateID == sessionID {
				t.Fatalf("expected no telegram outbox event on encode failure, got %+v", item)
			}
		}
	})
}

func TestHandleChannelEventExecutionApprovalPaths(t *testing.T) {
	t.Parallel()

	t.Run("modify approve starts execution and honors idempotency", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{ExecutionEnabled: true})
		sessionID := mustCreateWorkflowSession(t, service, "channel-approve", "host-channel-approve", "api")
		execution := contracts.ExecutionDetail{
			ExecutionID:     "exe-approve",
			RequestKind:     "execution",
			Status:          "pending",
			Command:         "uptime",
			TargetHost:      "host-channel-approve",
			ConnectorID:     "jumpserver-main",
			ConnectorType:   "execution",
			ConnectorVendor: "jumpserver",
			Protocol:        "jumpserver_api",
			ExecutionMode:   "job",
		}
		service.executions[execution.ExecutionID] = execution
		service.executionSession[execution.ExecutionID] = sessionID
		service.sessions[sessionID].detail.Executions = []contracts.ExecutionDetail{execution}

		event := contracts.ChannelEvent{
			Channel:        "telegram",
			ChatID:         "owner-room",
			UserID:         "alice",
			Action:         "modify_approve",
			ExecutionID:    execution.ExecutionID,
			Command:        "uptime && date",
			IdempotencyKey: "channel-approve-1",
		}

		result, err := service.HandleChannelEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("handle channel approve: %v", err)
		}
		if len(result.Executions) != 1 || result.Executions[0].Command != "uptime && date" {
			t.Fatalf("expected modified execution dispatch, got %+v", result)
		}
		if service.executions[execution.ExecutionID].Status != "executing" || service.executions[execution.ExecutionID].Command != "uptime && date" {
			t.Fatalf("expected execution to move to executing with updated command, got %+v", service.executions[execution.ExecutionID])
		}
		if !hasTimelineEvent(service.sessions[sessionID].detail.Timeline, "approval_accepted") {
			t.Fatalf("expected approval_accepted timeline, got %+v", service.sessions[sessionID].detail.Timeline)
		}

		duplicate, err := service.HandleChannelEvent(context.Background(), event)
		if err != nil {
			t.Fatalf("handle duplicate channel approve: %v", err)
		}
		if len(duplicate.Executions) != 0 || len(duplicate.Notifications) != 0 || len(duplicate.Capabilities) != 0 {
			t.Fatalf("expected idempotent duplicate to no-op, got %+v", duplicate)
		}
	})

	t.Run("approve returns manual follow-up when execution is disabled", func(t *testing.T) {
		t.Parallel()

		service := NewService(Options{ExecutionEnabled: false})
		sessionID := mustCreateWorkflowSession(t, service, "channel-manual", "host-channel-manual", "api")
		execution := contracts.ExecutionDetail{
			ExecutionID:   "exe-manual",
			RequestKind:   "execution",
			Status:        "pending",
			Command:       "systemctl restart api",
			TargetHost:    "host-channel-manual",
			ConnectorID:   "jumpserver-main",
			ApprovalGroup: "team:api",
		}
		service.executions[execution.ExecutionID] = execution
		service.executionSession[execution.ExecutionID] = sessionID
		service.sessions[sessionID].detail.Executions = []contracts.ExecutionDetail{execution}

		result, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{
			Channel:     "telegram",
			ChatID:      "owner-room",
			UserID:      "alice",
			Action:      "approve",
			ExecutionID: execution.ExecutionID,
		})
		if err != nil {
			t.Fatalf("handle manual approval: %v", err)
		}
		if len(result.Executions) != 0 || len(result.Notifications) != 1 {
			t.Fatalf("expected manual notification without execution dispatch, got %+v", result)
		}
		if service.executions[execution.ExecutionID].Status != "approved" || service.sessions[sessionID].detail.Status != "open" {
			t.Fatalf("expected approved execution and reopened session, got execution=%+v session=%+v", service.executions[execution.ExecutionID], service.sessions[sessionID].detail)
		}
		if !hasTimelineEvent(service.sessions[sessionID].detail.Timeline, "approval_accepted_manual") {
			t.Fatalf("expected approval_accepted_manual timeline, got %+v", service.sessions[sessionID].detail.Timeline)
		}
	})
}

func TestHandleChannelEventCapabilityRejectionAndContext(t *testing.T) {
	t.Parallel()

	t.Run("reject updates capability tool plan", func(t *testing.T) {
		t.Parallel()

		service := newCapabilityResultService(t)
		sessionID, approvalID := createCapabilityApprovalForResult(t, service, "channel-reject")

		result, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{
			Channel:     "telegram",
			ChatID:      "owner-room",
			UserID:      "alice",
			Action:      "reject",
			ExecutionID: approvalID,
		})
		if err != nil {
			t.Fatalf("handle capability reject: %v", err)
		}
		if len(result.Notifications) != 1 || result.SessionID != sessionID {
			t.Fatalf("expected rejection notification for session, got %+v", result)
		}
		if service.executions[approvalID].Status != "rejected" || service.sessions[sessionID].detail.Status != "analyzing" {
			t.Fatalf("expected rejected capability execution and analyzing session, got execution=%+v session=%+v", service.executions[approvalID], service.sessions[sessionID].detail)
		}
		if !hasTimelineEvent(service.sessions[sessionID].detail.Timeline, "capability_approval_rejected") {
			t.Fatalf("expected capability_approval_rejected timeline, got %+v", service.sessions[sessionID].detail.Timeline)
		}
		if service.sessions[sessionID].detail.ToolPlan[0].Status != "rejected" {
			t.Fatalf("expected rejected tool plan step, got %+v", service.sessions[sessionID].detail.ToolPlan[0])
		}
	})

	t.Run("request context leaves capability pending and session analyzing", func(t *testing.T) {
		t.Parallel()

		service := newCapabilityResultService(t)
		sessionID, approvalID := createCapabilityApprovalForResult(t, service, "channel-context")

		result, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{
			Channel:     "telegram",
			ChatID:      "owner-room",
			UserID:      "alice",
			Action:      "request_context",
			ExecutionID: approvalID,
		})
		if err != nil {
			t.Fatalf("handle capability request_context: %v", err)
		}
		if len(result.Notifications) != 1 || result.SessionID != sessionID {
			t.Fatalf("expected context request notification, got %+v", result)
		}
		if service.executions[approvalID].Status != "pending" || service.sessions[sessionID].detail.Status != "analyzing" {
			t.Fatalf("expected pending capability execution and analyzing session, got execution=%+v session=%+v", service.executions[approvalID], service.sessions[sessionID].detail)
		}
		if !hasTimelineEvent(service.sessions[sessionID].detail.Timeline, "capability_approval_requested_context") {
			t.Fatalf("expected capability_approval_requested_context timeline, got %+v", service.sessions[sessionID].detail.Timeline)
		}
	})
}

func TestHandleChannelEventValidationErrors(t *testing.T) {
	t.Parallel()

	service := NewService(Options{ExecutionEnabled: true})
	sessionID := mustCreateWorkflowSession(t, service, "channel-errors", "host-channel-errors", "api")
	service.executions["exe-errors"] = contracts.ExecutionDetail{
		ExecutionID: "exe-errors",
		Status:      "approved",
	}
	service.executionSession["exe-errors"] = sessionID
	service.sessions[sessionID].detail.Executions = []contracts.ExecutionDetail{service.executions["exe-errors"]}

	if _, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{}); err != contracts.ErrNotFound {
		t.Fatalf("expected ErrNotFound for empty execution id, got %v", err)
	}
	if _, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{
		Action:      "approve",
		ExecutionID: "missing",
	}); err != contracts.ErrNotFound {
		t.Fatalf("expected ErrNotFound for missing execution, got %v", err)
	}
	if _, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{
		Action:      "reject",
		ExecutionID: "exe-errors",
	}); err != contracts.ErrInvalidState {
		t.Fatalf("expected ErrInvalidState for non-pending execution, got %v", err)
	}
	if _, err := service.HandleChannelEvent(context.Background(), contracts.ChannelEvent{
		Action:      "escalate",
		ExecutionID: "exe-errors",
	}); err == nil || !strings.Contains(err.Error(), "unsupported channel action") {
		t.Fatalf("expected unsupported action error, got %v", err)
	}
}

func TestWorkflowHelperFunctions(t *testing.T) {
	t.Parallel()

	if diagnosisPrivacyNote() == "" {
		t.Fatal("expected diagnosis privacy note")
	}
	if authorizationActionLabel("custom") != "custom" {
		t.Fatalf("expected unknown authorization action to round-trip")
	}
	if authorizationActionLabel("deny") != "已拒绝" {
		t.Fatalf("expected deny label, got %q", authorizationActionLabel("deny"))
	}
	if got := formatApprovalTimeout(0); got != "15m" {
		t.Fatalf("expected default approval timeout, got %q", got)
	}
	if got := formatApprovalTimeout(90 * time.Second); got != "1m30s" {
		t.Fatalf("expected second-rounded timeout, got %q", got)
	}
	if minInt(2, 5) != 2 || minInt(7, 3) != 3 {
		t.Fatalf("expected minInt to pick smaller operand")
	}
	if topicEnabled := NewService(Options{DiagnosisEnabled: false, KnowledgeIngestEnabled: false}).topicEnabled("session.analyze_requested"); topicEnabled {
		t.Fatalf("expected session.analyze_requested to honor diagnosis feature flag")
	}
	if topicEnabled := NewService(Options{DiagnosisEnabled: true, KnowledgeIngestEnabled: false}).topicEnabled("session.closed"); topicEnabled {
		t.Fatalf("expected session.closed to honor knowledge ingest feature flag")
	}
	if !NewService(Options{}).topicEnabled("telegram.send") {
		t.Fatalf("expected non-feature-gated topics to stay enabled")
	}
	if stringFromAlert(map[string]interface{}{"host": "api-1"}, "host") != "api-1" {
		t.Fatalf("expected stringFromAlert to read top-level string")
	}
	if stringFromAlert(map[string]interface{}{"host": 7}, "host") != "" {
		t.Fatalf("expected stringFromAlert to ignore non-string values")
	}
	if annotationString(map[string]interface{}{"annotations": map[string]string{"summary": "ok"}}, "summary") != "ok" {
		t.Fatalf("expected annotationString to read map[string]string")
	}
	if annotationString(map[string]interface{}{"annotations": map[string]interface{}{"summary": "ok"}}, "summary") != "ok" {
		t.Fatalf("expected annotationString to read map[string]interface{}")
	}
	if annotationString(map[string]interface{}{"annotations": map[string]interface{}{"summary": 9}}, "summary") != "" {
		t.Fatalf("expected annotationString to ignore non-string annotation values")
	}
	if alertLabel(map[string]interface{}{"labels": map[string]string{"service": "payments"}}, "service") != "payments" {
		t.Fatalf("expected alertLabel to read map[string]string")
	}
	if alertLabel(map[string]interface{}{"labels": map[string]interface{}{"service": "payments"}}, "service") != "payments" {
		t.Fatalf("expected alertLabel to read map[string]interface{}")
	}
	if alertLabel(map[string]interface{}{"labels": map[string]interface{}{"service": 5}}, "service") != "" {
		t.Fatalf("expected alertLabel to ignore non-string label values")
	}
	if firstNonEmpty(" ", "  api ", "worker") != "api" {
		t.Fatalf("expected firstNonEmpty to return first trimmed value")
	}
	if firstNonEmpty(" ", "\t") != "" {
		t.Fatalf("expected firstNonEmpty to return empty string when all values are blank")
	}
	if got := ensureMap(nil); got != nil {
		t.Fatalf("expected ensureMap(nil) to return nil, got %+v", got)
	}
	var nilService *Service
	if nilService.currentDesensitizationConfig() == nil || NewService(Options{}).currentDesensitizationConfig() == nil {
		t.Fatalf("expected default desensitization config")
	}
	if got := pickHost(map[string]string{"host": "api-1"}); got != "api-1" {
		t.Fatalf("expected pickHost to prefer explicit host, got %q", got)
	}
	if got := pickHost(map[string]string{"pod": "api-pod-1"}); got != "api-pod-1" {
		t.Fatalf("expected pickHost to fall back to pod, got %q", got)
	}
	if got := rehydratePlaceholders("host=[HOST_1] ip=[IP_1] path=[PATH_1]", map[string]string{
		"[HOST_1]": "api-1",
		"[IP_1]":   "10.0.0.1",
		"[PATH_1]": "/tmp/data",
	}, &reasoning.DesensitizationConfig{
		Rehydration: reasoning.RehydrationConfig{
			Host: true,
			IP:   false,
			Path: true,
		},
	}); got != "host=api-1 ip=[IP_1] path=/tmp/data" {
		t.Fatalf("unexpected selective rehydration output: %q", got)
	}
}

func newCapabilityResultService(t *testing.T) *Service {
	t.Helper()

	return NewService(Options{
		ApprovalEnabled: true,
		ApprovalTimeout: 5 * time.Minute,
		ApprovalRouter: approvalrouting.New(approvalrouting.Config{
			ServiceOwners: map[string][]string{"api": {"owner-room"}},
		}),
	})
}

func createCapabilityApprovalForResult(t *testing.T, service *Service, fingerprint string) (string, string) {
	t.Helper()

	sessionID := mustCreateWorkflowSession(t, service, "capability-result-"+fingerprint, "host-"+fingerprint, "api")
	service.sessions[sessionID].detail.ToolPlan = []contracts.ToolPlanStep{{
		ID:     "step-1",
		Tool:   "connector.invoke_capability",
		Reason: "promote api canary",
	}}

	detail, _, err := service.CreateCapabilityApproval(context.Background(), contracts.ApprovedCapabilityRequest{
		SessionID:    sessionID,
		StepID:       "step-1",
		ConnectorID:  "delivery-main",
		CapabilityID: "deployment.promote",
		Params:       map[string]interface{}{"service": "api"},
		RequestedBy:  "tool-runner",
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
	return sessionID, detail.ExecutionID
}
