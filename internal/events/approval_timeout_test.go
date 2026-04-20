package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"tars/internal/contracts"
	"tars/internal/foundation/logger"
	"tars/internal/modules/action"
	actionssh "tars/internal/modules/action/ssh"
	"tars/internal/modules/knowledge"
	"tars/internal/modules/reasoning"
	"tars/internal/modules/workflow"
)

func TestApprovalTimeoutWorkerRejectsExpiredExecution(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	channelSvc := &captureChannel{}
	workflowSvc := workflow.NewService(workflow.Options{
		DiagnosisEnabled: true,
		ApprovalEnabled:  true,
		ExecutionEnabled: true,
		ApprovalTimeout:  time.Minute,
	})
	dispatcher := NewDispatcher(
		logger.New("ERROR"),
		nil,
		workflowSvc,
		reasoning.NewService(reasoning.Options{LocalCommandFallbackEnable: true}),
		action.NewService(action.Options{
			Executor: &fakeExecutor{
				result: actionssh.Result{ExitCode: 0, Output: "hostname"},
			},
			AllowedHosts:   []string{"host-1"},
			OutputSpoolDir: t.TempDir(),
		}),
		knowledge.NewService(),
		channelSvc,
		nil,
	)
	timeoutWorker := NewApprovalTimeoutWorker(logger.New("ERROR"), nil, workflowSvc, channelSvc)

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
		Annotations: map[string]string{"user_request": "重启 api"},
	})
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
	if len(sessionDetail.Executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(sessionDetail.Executions))
	}

	expireAt := sessionDetail.Executions[0].CreatedAt.Add(2 * time.Minute)
	if err := timeoutWorker.RunOnce(ctx, expireAt); err != nil {
		t.Fatalf("run timeout worker: %v", err)
	}

	sessionDetail, err = workflowSvc.GetSession(ctx, result.SessionID)
	if err != nil {
		t.Fatalf("get session after timeout: %v", err)
	}
	if sessionDetail.Status != "open" {
		t.Fatalf("expected session to return to open, got %s", sessionDetail.Status)
	}
	if sessionDetail.Executions[0].Status != "rejected" {
		t.Fatalf("expected execution to be rejected, got %s", sessionDetail.Executions[0].Status)
	}
	if len(channelSvc.messages) < 2 {
		t.Fatalf("expected timeout notification, got %d messages", len(channelSvc.messages))
	}

	_, err = workflowSvc.HandleChannelEvent(ctx, contracts.ChannelEvent{
		EventType:   "approval",
		Channel:     "telegram",
		UserID:      "alice",
		ChatID:      "ops-room",
		Action:      "approve",
		ExecutionID: sessionDetail.Executions[0].ExecutionID,
	})
	if !errors.Is(err, contracts.ErrInvalidState) {
		t.Fatalf("expected invalid state after timeout, got %v", err)
	}
}
