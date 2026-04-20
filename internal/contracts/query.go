package contracts

import "time"

type DiagnosisInput struct {
	SessionID        string
	Context          map[string]interface{}
	RoleModelBinding *RoleModelBinding
}

type RoleModelBinding struct {
	Primary                *RoleModelTargetBinding
	Fallback               *RoleModelTargetBinding
	InheritPlatformDefault bool
}

type RoleModelTargetBinding struct {
	ProviderID string
	Model      string
}

type DiagnosisOutput struct {
	Summary       string
	Citations     []KnowledgeHit
	ExecutionHint string
	ToolPlan      []ToolPlanStep
	Attachments   []MessageAttachment
	DesenseMap    map[string]string
}

type DiagnosisPlan struct {
	Summary    string
	ToolPlan   []ToolPlanStep
	DesenseMap map[string]string
}

type ToolPlanStep struct {
	ID                string
	Tool              string
	ConnectorID       string
	Reason            string
	Priority          int
	Status            string
	Input             map[string]interface{}
	ResolvedInput     map[string]interface{}
	Output            map[string]interface{}
	OnFailure         string
	OnPendingApproval string
	OnDenied          string
	Runtime           *RuntimeMetadata
	StartedAt         time.Time
	CompletedAt       time.Time
}

type RuntimeMetadata struct {
	Runtime         string
	Selection       string
	ConnectorID     string
	ConnectorType   string
	ConnectorVendor string
	Protocol        string
	ExecutionMode   string
	FallbackEnabled bool
	FallbackUsed    bool
	FallbackReason  string
	FallbackTarget  string
}

type ToolCapabilityDescriptor struct {
	Tool            string
	ConnectorID     string
	ConnectorType   string
	ConnectorVendor string
	Protocol        string
	CapabilityID    string
	Action          string
	Scopes          []string
	ReadOnly        bool
	Invocable       bool
	Source          string
	Description     string
}

type SkillMatch struct {
	SkillID       string
	DisplayName   string
	Summary       string
	MatchedBy     string
	Trigger       string
	ReviewState   string
	RuntimeMode   string
	Source        string
	Manifest      map[string]interface{}
	ExpectedTools []string
}

func CloneRuntimeMetadata(in *RuntimeMetadata) *RuntimeMetadata {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

type MetricsQuery struct {
	Service         string
	Host            string
	Query           string
	Mode            string
	Start           time.Time
	End             time.Time
	Step            string
	Window          string
	ConnectorID     string
	ConnectorType   string
	ConnectorVendor string
	Protocol        string
}

type MetricsResult struct {
	Series  []map[string]interface{}
	Runtime *RuntimeMetadata
}

type ApprovedExecutionRequest struct {
	ExecutionID     string
	SessionID       string
	TargetHost      string
	Command         string
	Service         string
	ConnectorID     string
	ConnectorType   string
	ConnectorVendor string
	Protocol        string
	ExecutionMode   string
}

type ApprovedCapabilityRequest struct {
	ApprovalID    string
	SessionID     string
	StepID        string
	ConnectorID   string
	CapabilityID  string
	Params        map[string]interface{}
	RequestedBy   string
	ApprovalGroup string
	Runtime       *RuntimeMetadata
}

type VerificationRequest struct {
	SessionID     string
	ExecutionID   string
	TargetHost    string
	Service       string
	ConnectorID   string
	Protocol      string
	ExecutionMode string
}

type KnowledgeQuery struct {
	TenantID string
	Query    string
}

type KnowledgeHit struct {
	DocumentID string
	ChunkID    string
	Snippet    string
}

type ChannelMessage struct {
	Channel     string
	Target      string
	Subject     string // optional: used as inbox subject / message title
	Body        string
	RefType     string // optional: related object type (session, execution, …)
	RefID       string // optional: related object ID
	Source      string // optional: originating component
	Actions     []ChannelAction
	Attachments []MessageAttachment
}

type ChannelAction struct {
	Label string
	Value string
}

type MessageAttachment struct {
	Type        string
	Name        string
	MimeType    string
	URL         string
	Content     string
	PreviewText string
	Metadata    map[string]interface{}
}

type ListSessionsFilter struct {
	Status    string
	Host      string
	Query     string
	SortBy    string
	SortOrder string
}

type ListExecutionsFilter struct {
	Status    string
	Query     string
	SortBy    string
	SortOrder string
}

type ListKnowledgeFilter struct {
	Query     string
	SortBy    string
	SortOrder string
}

type SessionDetail struct {
	SessionID        string
	AgentRoleID      string
	Status           string
	DiagnosisSummary string
	GoldenSummary    *SessionGoldenSummary
	ToolPlan         []ToolPlanStep
	Attachments      []MessageAttachment
	Alert            map[string]interface{}
	Verification     *SessionVerification
	Notifications    []NotificationDigest
	Executions       []ExecutionDetail
	Timeline         []TimelineEvent
}

type SessionVerification struct {
	Status    string
	Summary   string
	Details   map[string]interface{}
	Runtime   *RuntimeMetadata
	CheckedAt time.Time
}

type TimelineEvent struct {
	Event     string
	Message   string
	CreatedAt time.Time
}

type ExecutionDetail struct {
	ExecutionID      string
	SessionID        string
	AgentRoleID      string
	RequestKind      string
	Status           string
	RiskLevel        string
	GoldenSummary    *ExecutionGoldenSummary
	Command          string
	TargetHost       string
	StepID           string
	CapabilityID     string
	CapabilityParams map[string]interface{}
	ConnectorID      string
	ConnectorType    string
	ConnectorVendor  string
	Protocol         string
	ExecutionMode    string
	RequestedBy      string
	ApprovalGroup    string
	Runtime          *RuntimeMetadata
	ExitCode         int
	OutputRef        string
	OutputBytes      int64
	OutputTruncated  bool
	CreatedAt        time.Time
	ApprovedAt       time.Time
	CompletedAt      time.Time
}

type ExecutionOutputChunk struct {
	Seq        int
	StreamType string
	Content    string
	ByteSize   int
	CreatedAt  time.Time
}

type KnowledgeRecordDetail struct {
	DocumentID string
	SessionID  string
	Title      string
	Summary    string
	UpdatedAt  time.Time
}

type ListOutboxFilter struct {
	Status    string
	Query     string
	SortBy    string
	SortOrder string
}

type OutboxEvent struct {
	ID            string
	Topic         string
	Status        string
	AggregateID   string
	Headers       map[string]string
	Metadata      EventMetadata
	RetryCount    int
	LastError     string
	BlockedReason string
	AvailableAt   time.Time
	CreatedAt     time.Time
}

type DispatchableOutboxEvent struct {
	EventID     string
	Topic       string
	AggregateID string
	Headers     map[string]string
	Metadata    EventMetadata
	Attempt     int
	Status      string
	LastError   string
	AvailableAt time.Time
	CreatedAt   time.Time
	Payload     []byte
}

// CapabilityRequest is the universal input for connector.invoke_capability.
type CapabilityRequest struct {
	ConnectorID       string
	CapabilityID      string
	Params            map[string]interface{}
	SessionID         string
	Caller            string
	SkipAuthorization bool
}

// CapabilityResult is the universal output from connector.invoke_capability.
type CapabilityResult struct {
	Status    string
	Output    map[string]interface{}
	Artifacts []MessageAttachment
	Metadata  map[string]interface{}
	Error     string
	Runtime   *RuntimeMetadata
}
