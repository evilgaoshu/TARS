package contracts

import "time"

type AlertEvent struct {
	Source         string
	Severity       string
	Fingerprint    string
	IdempotencyKey string
	RequestHash    string
	Labels         map[string]string
	Annotations    map[string]string
	ReceivedAt     time.Time
}

type ChannelEvent struct {
	EventType      string
	Channel        string
	UserID         string
	ChatID         string
	Action         string
	ExecutionID    string
	RequestKind    string
	Command        string
	Reason         string
	IdempotencyKey string
}

type SessionClosedEvent struct {
	SessionID  string
	TenantID   string
	ResolvedAt time.Time
}

type ExecutionResult struct {
	ExecutionID     string
	SessionID       string
	Status          string
	ConnectorID     string
	Protocol        string
	ExecutionMode   string
	Runtime         *RuntimeMetadata
	ExitCode        int
	Output          string
	OutputRef       string
	OutputPreview   string
	OutputBytes     int64
	OutputTruncated bool
}

type CapabilityExecutionResult struct {
	ApprovalID   string
	SessionID    string
	StepID       string
	Status       string
	ConnectorID  string
	CapabilityID string
	Output       map[string]interface{}
	Artifacts    []MessageAttachment
	Metadata     map[string]interface{}
	Runtime      *RuntimeMetadata
	Error        string
}

type VerificationResult struct {
	SessionID   string
	ExecutionID string
	Status      string
	Summary     string
	Details     map[string]interface{}
	Runtime     *RuntimeMetadata
	CheckedAt   time.Time
}
