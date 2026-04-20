package contracts

import (
	"context"
	"time"

	"tars/internal/modules/connectors"
)

type AlertIngestService interface {
	IngestVMAlert(ctx context.Context, rawPayload []byte) ([]AlertEvent, error)
}

type WorkflowService interface {
	HandleAlertEvent(ctx context.Context, event AlertEvent) (SessionMutationResult, error)
	HandleChannelEvent(ctx context.Context, event ChannelEvent) (WorkflowDispatchResult, error)
	HandleExecutionResult(ctx context.Context, result ExecutionResult) (SessionMutationResult, error)
	HandleCapabilityResult(ctx context.Context, result CapabilityExecutionResult) (SessionMutationResult, error)
	HandleVerificationResult(ctx context.Context, result VerificationResult) (SessionMutationResult, error)
	ListSessions(ctx context.Context, filter ListSessionsFilter) ([]SessionDetail, error)
	ListExecutions(ctx context.Context, filter ListExecutionsFilter) ([]ExecutionDetail, error)
	GetSession(ctx context.Context, sessionID string) (SessionDetail, error)
	GetExecution(ctx context.Context, executionID string) (ExecutionDetail, error)
	GetExecutionOutput(ctx context.Context, executionID string) ([]ExecutionOutputChunk, error)
	PublishEvent(ctx context.Context, event EventPublishRequest) (EventEnvelope, error)
	ListOutbox(ctx context.Context, filter ListOutboxFilter) ([]OutboxEvent, error)
	ReplayOutbox(ctx context.Context, eventID string, operatorReason string) error
	DeleteOutbox(ctx context.Context, eventID string, operatorReason string) error
	RecoverPendingEvents(ctx context.Context) (int, error)
	SweepApprovalTimeouts(ctx context.Context, now time.Time) ([]ChannelMessage, error)
	ClaimEvents(ctx context.Context, limit int) ([]EventEnvelope, error)
	ApplyDiagnosis(ctx context.Context, eventID string, diagnosis DiagnosisOutput) (WorkflowDispatchResult, error)
	CreateCapabilityApproval(ctx context.Context, req ApprovedCapabilityRequest) (ExecutionDetail, []ChannelMessage, error)
	EnqueueNotifications(ctx context.Context, sessionID string, messages []ChannelMessage) error
	ResolveEvent(ctx context.Context, eventID string, result DeliveryResult) error
	RecoverProcessingOutbox(ctx context.Context) (int, error)
	ClaimOutboxBatch(ctx context.Context, limit int) ([]DispatchableOutboxEvent, error)
	CompleteOutbox(ctx context.Context, eventID string) error
	MarkOutboxFailed(ctx context.Context, eventID string, lastError string) error
}

type ReasoningService interface {
	BuildDiagnosis(ctx context.Context, input DiagnosisInput) (DiagnosisOutput, error)
	PlanDiagnosis(ctx context.Context, input DiagnosisInput) (DiagnosisPlan, error)
	FinalizeDiagnosis(ctx context.Context, input DiagnosisInput) (DiagnosisOutput, error)
}

type ActionService interface {
	QueryMetrics(ctx context.Context, query MetricsQuery) (MetricsResult, error)
	ExecuteApproved(ctx context.Context, req ApprovedExecutionRequest) (ExecutionResult, error)
	InvokeApprovedCapability(ctx context.Context, req ApprovedCapabilityRequest) (CapabilityResult, error)
	VerifyExecution(ctx context.Context, req VerificationRequest) (VerificationResult, error)
	CheckConnectorHealth(ctx context.Context, connectorID string) (connectors.LifecycleState, error)
	InvokeCapability(ctx context.Context, req CapabilityRequest) (CapabilityResult, error)
}

type KnowledgeService interface {
	Search(ctx context.Context, query KnowledgeQuery) ([]KnowledgeHit, error)
	IngestResolvedSession(ctx context.Context, event SessionClosedEvent) (KnowledgeIngestResult, error)
	ReindexDocuments(ctx context.Context, operatorReason string) error
}

type ChannelService interface {
	SendMessage(ctx context.Context, msg ChannelMessage) (SendResult, error)
}

type SessionMutationResult struct {
	SessionID  string
	Status     string
	Duplicated bool
}

type WorkflowDispatchResult struct {
	SessionID     string
	Notifications []ChannelMessage
	Executions    []ApprovedExecutionRequest
	Capabilities  []ApprovedCapabilityRequest
}

type KnowledgeIngestResult struct {
	DocumentID string
	Chunks     int
}

type SendResult struct {
	MessageID string
}
