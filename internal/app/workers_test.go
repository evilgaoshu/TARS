package app

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"tars/internal/contracts"
)

func TestStartWorkersHandlesMissingOptionalServices(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workflow := &startWorkersWorkflowStub{cancel: cancel}
	app := &App{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		services: Services{
			Workflow: workflow,
			Channel:  startWorkersChannelStub{},
		},
	}

	done := make(chan struct{})
	go func() {
		app.StartWorkers(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartWorkers did not stop")
	}

	workflow.mu.Lock()
	defer workflow.mu.Unlock()
	if workflow.claimEventsCalls == 0 {
		t.Fatal("expected dispatcher to claim events")
	}
	if workflow.resolveEventCalls != 1 {
		t.Fatalf("expected one resolved event, got %d", workflow.resolveEventCalls)
	}
	if workflow.lastDecision.Decision != contracts.DeliveryDecisionAck {
		t.Fatalf("expected session.closed to be acknowledged, got %#v", workflow.lastDecision)
	}
}

type startWorkersWorkflowStub struct {
	cancel context.CancelFunc

	mu                sync.Mutex
	claimed           bool
	claimEventsCalls  int
	resolveEventCalls int
	lastDecision      contracts.DeliveryResult
}

func (s *startWorkersWorkflowStub) HandleAlertEvent(context.Context, contracts.AlertEvent) (contracts.SessionMutationResult, error) {
	return contracts.SessionMutationResult{}, nil
}

func (s *startWorkersWorkflowStub) HandleChannelEvent(context.Context, contracts.ChannelEvent) (contracts.WorkflowDispatchResult, error) {
	return contracts.WorkflowDispatchResult{}, nil
}

func (s *startWorkersWorkflowStub) HandleExecutionResult(context.Context, contracts.ExecutionResult) (contracts.SessionMutationResult, error) {
	return contracts.SessionMutationResult{}, nil
}

func (s *startWorkersWorkflowStub) HandleCapabilityResult(context.Context, contracts.CapabilityExecutionResult) (contracts.SessionMutationResult, error) {
	return contracts.SessionMutationResult{}, nil
}

func (s *startWorkersWorkflowStub) HandleVerificationResult(context.Context, contracts.VerificationResult) (contracts.SessionMutationResult, error) {
	return contracts.SessionMutationResult{}, nil
}

func (s *startWorkersWorkflowStub) ListSessions(context.Context, contracts.ListSessionsFilter) ([]contracts.SessionDetail, error) {
	return nil, nil
}

func (s *startWorkersWorkflowStub) ListExecutions(context.Context, contracts.ListExecutionsFilter) ([]contracts.ExecutionDetail, error) {
	return nil, nil
}

func (s *startWorkersWorkflowStub) GetSession(context.Context, string) (contracts.SessionDetail, error) {
	return contracts.SessionDetail{}, nil
}

func (s *startWorkersWorkflowStub) GetExecution(context.Context, string) (contracts.ExecutionDetail, error) {
	return contracts.ExecutionDetail{}, nil
}

func (s *startWorkersWorkflowStub) GetExecutionOutput(context.Context, string) ([]contracts.ExecutionOutputChunk, error) {
	return nil, nil
}

func (s *startWorkersWorkflowStub) PublishEvent(context.Context, contracts.EventPublishRequest) (contracts.EventEnvelope, error) {
	return contracts.EventEnvelope{}, nil
}

func (s *startWorkersWorkflowStub) ListOutbox(context.Context, contracts.ListOutboxFilter) ([]contracts.OutboxEvent, error) {
	return nil, nil
}

func (s *startWorkersWorkflowStub) ReplayOutbox(context.Context, string, string) error {
	return nil
}

func (s *startWorkersWorkflowStub) DeleteOutbox(context.Context, string, string) error {
	return nil
}

func (s *startWorkersWorkflowStub) RecoverPendingEvents(context.Context) (int, error) {
	return 0, nil
}

func (s *startWorkersWorkflowStub) SweepApprovalTimeouts(context.Context, time.Time) ([]contracts.ChannelMessage, error) {
	return nil, nil
}

func (s *startWorkersWorkflowStub) ClaimEvents(context.Context, int) ([]contracts.EventEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimEventsCalls++
	if s.claimed {
		return nil, nil
	}
	s.claimed = true
	return []contracts.EventEnvelope{{
		EventID:     "evt-1",
		Topic:       "session.closed",
		AggregateID: "session-1",
		Attempt:     1,
	}}, nil
}

func (s *startWorkersWorkflowStub) ApplyDiagnosis(context.Context, string, contracts.DiagnosisOutput) (contracts.WorkflowDispatchResult, error) {
	return contracts.WorkflowDispatchResult{}, nil
}

func (s *startWorkersWorkflowStub) CreateCapabilityApproval(context.Context, contracts.ApprovedCapabilityRequest) (contracts.ExecutionDetail, []contracts.ChannelMessage, error) {
	return contracts.ExecutionDetail{}, nil, nil
}

func (s *startWorkersWorkflowStub) EnqueueNotifications(context.Context, string, []contracts.ChannelMessage) error {
	return nil
}

func (s *startWorkersWorkflowStub) ResolveEvent(_ context.Context, _ string, result contracts.DeliveryResult) error {
	s.mu.Lock()
	s.resolveEventCalls++
	s.lastDecision = result
	s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *startWorkersWorkflowStub) RecoverProcessingOutbox(context.Context) (int, error) {
	return 0, nil
}

func (s *startWorkersWorkflowStub) ClaimOutboxBatch(context.Context, int) ([]contracts.DispatchableOutboxEvent, error) {
	return nil, nil
}

func (s *startWorkersWorkflowStub) CompleteOutbox(context.Context, string) error {
	return nil
}

func (s *startWorkersWorkflowStub) MarkOutboxFailed(context.Context, string, string) error {
	return nil
}

type startWorkersChannelStub struct{}

func (startWorkersChannelStub) SendMessage(context.Context, contracts.ChannelMessage) (contracts.SendResult, error) {
	return contracts.SendResult{}, nil
}
