package dto

import "time"

type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type ReadyResponse struct {
	Status   string `json:"status"`
	Degraded bool   `json:"degraded"`
}

type VMAlertWebhookResponse struct {
	Accepted   bool     `json:"accepted"`
	Duplicated bool     `json:"duplicated,omitempty"`
	EventCount int      `json:"event_count"`
	SessionIDs []string `json:"session_ids"`
}

type TelegramWebhookResponse struct {
	Accepted bool `json:"accepted"`
}

type ListPage struct {
	Page      int    `json:"page"`
	Limit     int    `json:"limit"`
	Total     int    `json:"total"`
	HasNext   bool   `json:"has_next"`
	Query     string `json:"q,omitempty"`
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
}

type SessionListResponse struct {
	Items []SessionDetail `json:"items"`
	ListPage
}

type SessionDetail struct {
	SessionID        string                 `json:"session_id"`
	AgentRoleID      string                 `json:"agent_role_id,omitempty"`
	Status           string                 `json:"status"`
	IsSmoke          bool                   `json:"is_smoke,omitempty"`
	DiagnosisSummary string                 `json:"diagnosis_summary"`
	GoldenSummary    *SessionGoldenSummary  `json:"golden_summary,omitempty"`
	ToolPlan         []ToolPlanStep         `json:"tool_plan,omitempty"`
	Attachments      []MessageAttachment    `json:"attachments,omitempty"`
	Alert            map[string]interface{} `json:"alert"`
	Verification     *SessionVerification   `json:"verification,omitempty"`
	Notifications    []NotificationDigest   `json:"notifications,omitempty"`
	Executions       []ExecutionDetail      `json:"executions"`
	Timeline         []TimelineEvent        `json:"timeline"`
}

type SessionGoldenSummary struct {
	Headline             string   `json:"headline,omitempty"`
	Conclusion           string   `json:"conclusion,omitempty"`
	Risk                 string   `json:"risk,omitempty"`
	NextAction           string   `json:"next_action,omitempty"`
	Evidence             []string `json:"evidence,omitempty"`
	NotificationHeadline string   `json:"notification_headline,omitempty"`
	ExecutionHeadline    string   `json:"execution_headline,omitempty"`
	VerificationHeadline string   `json:"verification_headline,omitempty"`
}

type NotificationDigest struct {
	Stage     string    `json:"stage,omitempty"`
	Target    string    `json:"target,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Preview   string    `json:"preview,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type SessionTraceResponse struct {
	SessionID    string                 `json:"session_id"`
	AuditEntries []AuditRecord          `json:"audit_entries"`
	Knowledge    *SessionKnowledgeTrace `json:"knowledge,omitempty"`
}

type AuditListResponse struct {
	Items []AuditRecord `json:"items"`
	ListPage
}

type AuditRecord struct {
	ID           string                 `json:"id,omitempty"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Action       string                 `json:"action"`
	Actor        string                 `json:"actor,omitempty"`
	TraceID      string                 `json:"trace_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

type LogRecord struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level,omitempty"`
	Component   string                 `json:"component,omitempty"`
	Message     string                 `json:"message"`
	Route       string                 `json:"route,omitempty"`
	Actor       string                 `json:"actor,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type LogListResponse struct {
	Items []LogRecord `json:"items"`
	ListPage
}

type TraceEventRecord struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Kind        string                 `json:"kind"`
	Component   string                 `json:"component,omitempty"`
	Message     string                 `json:"message"`
	SessionID   string                 `json:"session_id,omitempty"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	Actor       string                 `json:"actor,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type TraceSample struct {
	TraceID     string    `json:"trace_id"`
	SessionID   string    `json:"session_id,omitempty"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Component   string    `json:"component,omitempty"`
	EventCount  int       `json:"event_count"`
	LastEventAt time.Time `json:"last_event_at,omitempty"`
	LastMessage string    `json:"last_message,omitempty"`
	Components  []string  `json:"components,omitempty"`
}

type ObservabilitySummary struct {
	LogEntries24h   int       `json:"log_entries_24h"`
	ErrorEntries24h int       `json:"error_entries_24h"`
	EventEntries24h int       `json:"event_entries_24h"`
	ActiveTraces    int       `json:"active_traces"`
	LastLogAt       time.Time `json:"last_log_at,omitempty"`
	LastEventAt     time.Time `json:"last_event_at,omitempty"`
}

type ObservabilitySignalConfig struct {
	RetentionHours float64 `json:"retention_hours"`
	MaxSizeBytes   int64   `json:"max_size_bytes"`
	CurrentBytes   int64   `json:"current_bytes"`
	FilePath       string  `json:"file_path,omitempty"`
}

type OTLPStatus struct {
	Endpoint       string `json:"endpoint,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	Insecure       bool   `json:"insecure"`
	MetricsEnabled bool   `json:"metrics_enabled"`
	LogsEnabled    bool   `json:"logs_enabled"`
	TracesEnabled  bool   `json:"traces_enabled"`
}

type ObservabilityRetentionStatus struct {
	DataDir   string                    `json:"data_dir,omitempty"`
	Metrics   ObservabilitySignalConfig `json:"metrics"`
	Logs      ObservabilitySignalConfig `json:"logs"`
	Traces    ObservabilitySignalConfig `json:"traces"`
	OTLP      OTLPStatus                `json:"otlp"`
	Exporters []string                  `json:"exporters,omitempty"`
}

type ObservabilityResponse struct {
	Summary         ObservabilitySummary         `json:"summary"`
	MetricsEndpoint string                       `json:"metrics_endpoint"`
	Retention       ObservabilityRetentionStatus `json:"retention"`
	Health          DashboardHealthResponse      `json:"health"`
	RecentLogs      []LogRecord                  `json:"recent_logs,omitempty"`
	RecentEvents    []TraceEventRecord           `json:"recent_events,omitempty"`
	TraceSamples    []TraceSample                `json:"trace_samples,omitempty"`
}

type SessionKnowledgeTrace struct {
	DocumentID     string           `json:"document_id"`
	Title          string           `json:"title"`
	Summary        string           `json:"summary,omitempty"`
	ContentPreview string           `json:"content_preview,omitempty"`
	Conversation   []string         `json:"conversation,omitempty"`
	Runtime        *RuntimeMetadata `json:"runtime,omitempty"`
	UpdatedAt      time.Time        `json:"updated_at,omitempty"`
}

type KnowledgeListResponse struct {
	Items []KnowledgeRecord `json:"items"`
	ListPage
}

type KnowledgeRecord struct {
	DocumentID string    `json:"document_id"`
	SessionID  string    `json:"session_id"`
	Title      string    `json:"title"`
	Summary    string    `json:"summary,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type SessionVerification struct {
	Status    string                 `json:"status"`
	Summary   string                 `json:"summary"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Runtime   *RuntimeMetadata       `json:"runtime,omitempty"`
	CheckedAt time.Time              `json:"checked_at,omitempty"`
}

type TimelineEvent struct {
	Event     string    `json:"event"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type ExecutionDetail struct {
	ExecutionID      string                  `json:"execution_id"`
	SessionID        string                  `json:"session_id,omitempty"`
	AgentRoleID      string                  `json:"agent_role_id,omitempty"`
	RequestKind      string                  `json:"request_kind,omitempty"`
	Status           string                  `json:"status"`
	RiskLevel        string                  `json:"risk_level,omitempty"`
	GoldenSummary    *ExecutionGoldenSummary `json:"golden_summary,omitempty"`
	Command          string                  `json:"command,omitempty"`
	TargetHost       string                  `json:"target_host,omitempty"`
	StepID           string                  `json:"step_id,omitempty"`
	CapabilityID     string                  `json:"capability_id,omitempty"`
	CapabilityParams map[string]interface{}  `json:"capability_params,omitempty"`
	ConnectorID      string                  `json:"connector_id,omitempty"`
	ConnectorType    string                  `json:"connector_type,omitempty"`
	ConnectorVendor  string                  `json:"connector_vendor,omitempty"`
	Protocol         string                  `json:"protocol,omitempty"`
	ExecutionMode    string                  `json:"execution_mode,omitempty"`
	RequestedBy      string                  `json:"requested_by,omitempty"`
	ApprovalGroup    string                  `json:"approval_group,omitempty"`
	Runtime          *RuntimeMetadata        `json:"runtime,omitempty"`
	ExitCode         int                     `json:"exit_code,omitempty"`
	OutputRef        string                  `json:"output_ref,omitempty"`
	OutputBytes      int64                   `json:"output_bytes,omitempty"`
	OutputTruncated  bool                    `json:"output_truncated,omitempty"`
	CreatedAt        time.Time               `json:"created_at,omitempty"`
	ApprovedAt       time.Time               `json:"approved_at,omitempty"`
	CompletedAt      time.Time               `json:"completed_at,omitempty"`
}

type ExecutionGoldenSummary struct {
	Headline       string `json:"headline,omitempty"`
	Risk           string `json:"risk,omitempty"`
	Approval       string `json:"approval,omitempty"`
	Result         string `json:"result,omitempty"`
	NextAction     string `json:"next_action,omitempty"`
	CommandPreview string `json:"command_preview,omitempty"`
}

type ExecutionOutputResponse struct {
	ExecutionID string                 `json:"execution_id"`
	Chunks      []ExecutionOutputChunk `json:"chunks"`
}

type ExecutionOutputChunk struct {
	Seq        int       `json:"seq"`
	StreamType string    `json:"stream_type"`
	Content    string    `json:"content"`
	ByteSize   int       `json:"byte_size"`
	CreatedAt  time.Time `json:"created_at"`
}

type OpsSummaryResponse struct {
	ActiveSessions       int `json:"active_sessions"`
	PendingApprovals     int `json:"pending_approvals"`
	ExecutionsTotal      int `json:"executions_total"`
	ExecutionsCompleted  int `json:"executions_completed"`
	ExecutionSuccessRate int `json:"execution_success_rate"`
	BlockedOutbox        int `json:"blocked_outbox"`
	FailedOutbox         int `json:"failed_outbox"`
	VisibleOutbox        int `json:"visible_outbox"`
	HealthyConnectors    int `json:"healthy_connectors"`
	DegradedConnectors   int `json:"degraded_connectors"`
	ConfiguredSecrets    int `json:"configured_secrets"`
	MissingSecrets       int `json:"missing_secrets"`
	ProviderFailures     int `json:"provider_failures"`
	ActiveAlerts         int `json:"active_alerts"`
}

type SetupStatusResponse struct {
	RolloutMode     string                      `json:"rollout_mode"`
	Features        SetupFeatures               `json:"features"`
	Initialization  SetupInitializationStatus   `json:"initialization"`
	Telegram        TelegramSetupStatus         `json:"telegram"`
	Model           ProviderSetupStatus         `json:"model"`
	AssistModel     ProviderSetupStatus         `json:"assist_model"`
	Providers       ProvidersSetupStatus        `json:"providers"`
	Connectors      ConnectorsSetupStatus       `json:"connectors"`
	LegacyFallbacks *LegacyFallbacksSetupStatus `json:"legacy_fallbacks,omitempty"`
	SmokeDefaults   SmokeDefaultsSetupStatus    `json:"smoke_defaults"`
	Authorization   AuthorizationSetupStatus    `json:"authorization"`
	Approval        ApprovalRoutingSetupStatus  `json:"approval"`
	Reasoning       ReasoningPromptSetupStatus  `json:"reasoning"`
	Desensitization DesensitizationSetupStatus  `json:"desensitization"`
	LatestSmoke     *SmokeSessionStatus         `json:"latest_smoke,omitempty"`
}

type BootstrapStatusResponse struct {
	Initialized bool   `json:"initialized"`
	Mode        string `json:"mode"`
	NextStep    string `json:"next_step,omitempty"`
}

type SetupInitializationStatus struct {
	Initialized       bool           `json:"initialized"`
	Mode              string         `json:"mode"`
	CurrentStep       string         `json:"current_step,omitempty"`
	AdminConfigured   bool           `json:"admin_configured"`
	AuthConfigured    bool           `json:"auth_configured"`
	ProviderReady     bool           `json:"provider_ready"`
	ChannelReady      bool           `json:"channel_ready"`
	ProviderChecked   bool           `json:"provider_checked,omitempty"`
	ProviderCheckOK   bool           `json:"provider_check_ok,omitempty"`
	ProviderCheckNote string         `json:"provider_check_note,omitempty"`
	AdminUserID       string         `json:"admin_user_id,omitempty"`
	AuthProviderID    string         `json:"auth_provider_id,omitempty"`
	PrimaryProviderID string         `json:"primary_provider_id,omitempty"`
	PrimaryModel      string         `json:"primary_model,omitempty"`
	DefaultChannelID  string         `json:"default_channel_id,omitempty"`
	LoginHint         SetupLoginHint `json:"login_hint,omitempty"`
	CompletedAt       time.Time      `json:"completed_at,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at,omitempty"`
	NextStep          string         `json:"next_step,omitempty"`
	RequiredSteps     []string       `json:"required_steps,omitempty"`
	CompletedSteps    []string       `json:"completed_steps,omitempty"`
}

type SetupLoginHint struct {
	Username string `json:"username,omitempty"`
	Provider string `json:"provider,omitempty"`
	LoginURL string `json:"login_url,omitempty"`
}

type SetupWizardResponse struct {
	Initialization SetupInitializationStatus `json:"initialization"`
	Admin          SetupWizardAdmin          `json:"admin"`
	Auth           SetupWizardAuth           `json:"auth"`
	Provider       SetupWizardProvider       `json:"provider"`
	Channel        SetupWizardChannel        `json:"channel"`
}

type SetupWizardAdmin struct {
	User User `json:"user"`
}

type SetupWizardAuth struct {
	Provider        AuthProvider `json:"provider"`
	SupportedTypes  []string     `json:"supported_types,omitempty"`
	RecommendedType string       `json:"recommended_type,omitempty"`
}

type SetupWizardProvider struct {
	Provider ProviderRegistryEntry `json:"provider"`
}

type SetupWizardChannel struct {
	Channel Channel `json:"channel"`
}

type ToolPlanStep struct {
	ID                string                 `json:"id,omitempty"`
	Tool              string                 `json:"tool"`
	ConnectorID       string                 `json:"connector_id,omitempty"`
	Reason            string                 `json:"reason,omitempty"`
	Priority          int                    `json:"priority,omitempty"`
	Status            string                 `json:"status,omitempty"`
	Input             map[string]interface{} `json:"input,omitempty"`
	ResolvedInput     map[string]interface{} `json:"resolved_input,omitempty"`
	Output            map[string]interface{} `json:"output,omitempty"`
	OnFailure         string                 `json:"on_failure,omitempty"`
	OnPendingApproval string                 `json:"on_pending_approval,omitempty"`
	OnDenied          string                 `json:"on_denied,omitempty"`
	Runtime           *RuntimeMetadata       `json:"runtime,omitempty"`
	StartedAt         time.Time              `json:"started_at,omitempty"`
	CompletedAt       time.Time              `json:"completed_at,omitempty"`
}

type MessageAttachment struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	MimeType    string                 `json:"mime_type,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Content     string                 `json:"content,omitempty"`
	PreviewText string                 `json:"preview_text,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type PlatformDiscoveryResponse struct {
	ProductName                string                         `json:"product_name"`
	APIBasePath                string                         `json:"api_base_path"`
	APIVersion                 string                         `json:"api_version"`
	ManifestVersion            string                         `json:"manifest_version"`
	SkillManifestVersion       string                         `json:"skill_manifest_version,omitempty"`
	MarketplacePackageVersion  string                         `json:"marketplace_package_version"`
	IntegrationModes           []string                       `json:"integration_modes"`
	ConnectorKinds             []string                       `json:"connector_kinds"`
	RegisteredConnectorsCount  int                            `json:"registered_connectors_count"`
	RegisteredConnectorIDs     []string                       `json:"registered_connector_ids,omitempty"`
	RegisteredConnectorKinds   []string                       `json:"registered_connector_kinds,omitempty"`
	SupportedProviderProtocols []string                       `json:"supported_provider_protocols"`
	SupportedProviderVendors   []string                       `json:"supported_provider_vendors"`
	ImportExportFormats        []string                       `json:"import_export_formats"`
	ToolPlanCapabilities       []ToolPlanCapabilityDescriptor `json:"tool_plan_capabilities,omitempty"`
	Docs                       []string                       `json:"docs"`
}

type ToolPlanCapabilityDescriptor struct {
	Tool            string   `json:"tool,omitempty"`
	ConnectorID     string   `json:"connector_id,omitempty"`
	ConnectorType   string   `json:"connector_type,omitempty"`
	ConnectorVendor string   `json:"connector_vendor,omitempty"`
	Protocol        string   `json:"protocol,omitempty"`
	CapabilityID    string   `json:"capability_id,omitempty"`
	Action          string   `json:"action,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
	ReadOnly        bool     `json:"read_only"`
	Invocable       bool     `json:"invocable"`
	Source          string   `json:"source,omitempty"`
	Description     string   `json:"description,omitempty"`
}

type AutomationListResponse struct {
	Items []AutomationJob `json:"items"`
	ListPage
}

type AutomationJob struct {
	ID                  string                         `json:"id"`
	DisplayName         string                         `json:"display_name"`
	Description         string                         `json:"description,omitempty"`
	AgentRoleID         string                         `json:"agent_role_id,omitempty"`
	GovernancePolicy    string                         `json:"governance_policy,omitempty"`
	Type                string                         `json:"type"`
	TargetRef           string                         `json:"target_ref"`
	Schedule            string                         `json:"schedule"`
	Enabled             bool                           `json:"enabled"`
	Owner               string                         `json:"owner,omitempty"`
	RuntimeMode         string                         `json:"runtime_mode,omitempty"`
	TimeoutSeconds      int                            `json:"timeout_seconds,omitempty"`
	RetryMaxAttempts    int                            `json:"retry_max_attempts,omitempty"`
	RetryInitialBackoff string                         `json:"retry_initial_backoff,omitempty"`
	Labels              map[string]string              `json:"labels,omitempty"`
	Skill               *AutomationSkillTarget         `json:"skill,omitempty"`
	ConnectorCapability *AutomationConnectorCapability `json:"connector_capability,omitempty"`
	State               *AutomationJobState            `json:"state,omitempty"`
	LastRun             *AutomationRun                 `json:"last_run,omitempty"`
}

type AutomationSkillTarget struct {
	SkillID string                 `json:"skill_id"`
	Context map[string]interface{} `json:"context,omitempty"`
}

type AutomationConnectorCapability struct {
	ConnectorID  string                 `json:"connector_id"`
	CapabilityID string                 `json:"capability_id"`
	Params       map[string]interface{} `json:"params,omitempty"`
}

type AutomationJobState struct {
	Status              string          `json:"status,omitempty"`
	LastRunAt           time.Time       `json:"last_run_at,omitempty"`
	NextRunAt           time.Time       `json:"next_run_at,omitempty"`
	LastOutcome         string          `json:"last_outcome,omitempty"`
	LastError           string          `json:"last_error,omitempty"`
	ConsecutiveFailures int             `json:"consecutive_failures,omitempty"`
	Runs                []AutomationRun `json:"runs,omitempty"`
	UpdatedAt           time.Time       `json:"updated_at,omitempty"`
}

type AutomationRun struct {
	RunID        string                 `json:"run_id"`
	JobID        string                 `json:"job_id"`
	Trigger      string                 `json:"trigger"`
	Status       string                 `json:"status"`
	StartedAt    time.Time              `json:"started_at,omitempty"`
	CompletedAt  time.Time              `json:"completed_at,omitempty"`
	AttemptCount int                    `json:"attempt_count,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type SkillRevision struct {
	CreatedAt time.Time `json:"created_at,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Action    string    `json:"action,omitempty"`
}

type SkillLifecycleEvent struct {
	Type      string            `json:"type,omitempty"`
	Summary   string            `json:"summary,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty"`
}

type SkillLifecycle struct {
	SkillID     string                `json:"skill_id,omitempty"`
	DisplayName string                `json:"display_name,omitempty"`
	Source      string                `json:"source,omitempty"`
	Status      string                `json:"status,omitempty"`
	ReviewState string                `json:"review_state,omitempty"`
	RuntimeMode string                `json:"runtime_mode,omitempty"`
	Enabled     bool                  `json:"enabled"`
	InstalledAt time.Time             `json:"installed_at,omitempty"`
	UpdatedAt   time.Time             `json:"updated_at,omitempty"`
	PublishedAt time.Time             `json:"published_at,omitempty"`
	History     []SkillLifecycleEvent `json:"history,omitempty"`
	Revisions   []SkillRevision       `json:"revisions,omitempty"`
}

type SkillMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Version     string   `json:"version,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Vendor      string   `json:"vendor,omitempty"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source,omitempty"`
	Content     string   `json:"content,omitempty"`
	OrgID       string   `json:"org_id,omitempty"`
	TenantID    string   `json:"tenant_id,omitempty"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
}

type SkillConnectorPreference struct {
	Metrics       []string `json:"metrics,omitempty"`
	Execution     []string `json:"execution,omitempty"`
	Observability []string `json:"observability,omitempty"`
	Delivery      []string `json:"delivery,omitempty"`
}

type SkillGovernance struct {
	ExecutionPolicy     string                   `json:"execution_policy,omitempty"`
	ReadOnlyFirst       bool                     `json:"read_only_first,omitempty"`
	ConnectorPreference SkillConnectorPreference `json:"connector_preference,omitempty"`
}

type SkillSpec struct {
	Governance SkillGovernance `json:"governance,omitempty"`
}

type SkillManifest struct {
	APIVersion    string                 `json:"api_version,omitempty"`
	Kind          string                 `json:"kind,omitempty"`
	Enabled       bool                   `json:"enabled"`
	Metadata      SkillMetadata          `json:"metadata"`
	Spec          SkillSpec              `json:"spec"`
	Compatibility map[string]interface{} `json:"compatibility,omitempty"`
	Lifecycle     *SkillLifecycle        `json:"lifecycle,omitempty"`
}

type SkillListResponse struct {
	Items []SkillManifest `json:"items"`
	ListPage
}

type SkillsConfig struct {
	Entries []SkillManifest `json:"entries,omitempty"`
}

type SkillsConfigResponse struct {
	Configured bool         `json:"configured"`
	Loaded     bool         `json:"loaded"`
	Path       string       `json:"path,omitempty"`
	UpdatedAt  time.Time    `json:"updated_at,omitempty"`
	Content    string       `json:"content,omitempty"`
	Config     SkillsConfig `json:"config"`
}

type SkillImportResponse struct {
	Manifest SkillManifest  `json:"manifest"`
	State    SkillLifecycle `json:"state"`
}

type ExtensionDocAsset struct {
	ID      string `json:"id,omitempty"`
	Slug    string `json:"slug,omitempty"`
	Title   string `json:"title,omitempty"`
	Format  string `json:"format,omitempty"`
	Summary string `json:"summary,omitempty"`
	Content string `json:"content,omitempty"`
}

type ExtensionTestSpec struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Command string `json:"command,omitempty"`
}

type ExtensionBundleMetadata struct {
	ID          string    `json:"id,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	Version     string    `json:"version,omitempty"`
	Summary     string    `json:"summary,omitempty"`
	Source      string    `json:"source,omitempty"`
	GeneratedBy string    `json:"generated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

type ExtensionBundle struct {
	APIVersion    string                  `json:"api_version,omitempty"`
	Kind          string                  `json:"kind,omitempty"`
	Metadata      ExtensionBundleMetadata `json:"metadata"`
	Skill         SkillManifest           `json:"skill"`
	Docs          []ExtensionDocAsset     `json:"docs,omitempty"`
	Tests         []ExtensionTestSpec     `json:"tests,omitempty"`
	Compatibility map[string]interface{}  `json:"compatibility,omitempty"`
}

type ExtensionValidationReport struct {
	Valid     bool      `json:"valid"`
	Errors    []string  `json:"errors,omitempty"`
	Warnings  []string  `json:"warnings,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitempty"`
}

type ExtensionPreviewSummary struct {
	ChangeType      string   `json:"change_type,omitempty"`
	ExistingVersion string   `json:"existing_version,omitempty"`
	ProposedVersion string   `json:"proposed_version,omitempty"`
	Summary         []string `json:"summary,omitempty"`
}

type ExtensionReviewEvent struct {
	State      string    `json:"state,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	ImportedBy string    `json:"imported_by,omitempty"`
}

type ExtensionCandidate struct {
	ID              string                    `json:"id"`
	Status          string                    `json:"status,omitempty"`
	ReviewState     string                    `json:"review_state,omitempty"`
	ReviewHistory   []ExtensionReviewEvent    `json:"review_history,omitempty"`
	ImportedSkillID string                    `json:"imported_skill_id,omitempty"`
	ImportedAt      time.Time                 `json:"imported_at,omitempty"`
	CreatedAt       time.Time                 `json:"created_at,omitempty"`
	UpdatedAt       time.Time                 `json:"updated_at,omitempty"`
	Bundle          ExtensionBundle           `json:"bundle"`
	Validation      ExtensionValidationReport `json:"validation"`
	Preview         ExtensionPreviewSummary   `json:"preview"`
}

type ExtensionListResponse struct {
	Items []ExtensionCandidate `json:"items"`
	ListPage
}

type ExtensionValidationResponse struct {
	Bundle     ExtensionBundle           `json:"bundle"`
	Validation ExtensionValidationReport `json:"validation"`
	Preview    ExtensionPreviewSummary   `json:"preview"`
}

type ExtensionImportResponse struct {
	Candidate ExtensionCandidate `json:"candidate"`
	Manifest  SkillManifest      `json:"manifest"`
	State     SkillLifecycle     `json:"state"`
}

type SetupFeatures struct {
	DiagnosisEnabled       bool `json:"diagnosis_enabled"`
	ApprovalEnabled        bool `json:"approval_enabled"`
	ExecutionEnabled       bool `json:"execution_enabled"`
	KnowledgeIngestEnabled bool `json:"knowledge_ingest_enabled"`
}

type ComponentRuntimeStatus struct {
	LastResult    string    `json:"last_result,omitempty"`
	LastDetail    string    `json:"last_detail,omitempty"`
	LastChangedAt time.Time `json:"last_changed_at,omitempty"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	LastErrorAt   time.Time `json:"last_error_at,omitempty"`
}

type RuntimeMetadata struct {
	Runtime         string `json:"runtime,omitempty"`
	Selection       string `json:"selection,omitempty"`
	ConnectorID     string `json:"connector_id,omitempty"`
	ConnectorType   string `json:"connector_type,omitempty"`
	ConnectorVendor string `json:"connector_vendor,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	ExecutionMode   string `json:"execution_mode,omitempty"`
	RuntimeState    string `json:"runtime_state,omitempty"`
	FallbackEnabled bool   `json:"fallback_enabled,omitempty"`
	FallbackUsed    bool   `json:"fallback_used,omitempty"`
	FallbackReason  string `json:"fallback_reason,omitempty"`
	FallbackTarget  string `json:"fallback_target,omitempty"`
}

type RuntimeSetupStatus struct {
	Name             string                 `json:"name,omitempty"`
	Primary          *RuntimeMetadata       `json:"primary,omitempty"`
	Fallback         *RuntimeMetadata       `json:"fallback,omitempty"`
	Component        string                 `json:"component,omitempty"`
	CapabilityTool   string                 `json:"capability_tool,omitempty"`
	ComponentRuntime ComponentRuntimeStatus `json:"component_runtime"`
}

type TelegramSetupStatus struct {
	Configured bool `json:"configured"`
	Polling    bool `json:"polling"`

	BaseURL string `json:"base_url,omitempty"`
	Mode    string `json:"mode"`

	ComponentRuntimeStatus
}

type ProviderSetupStatus struct {
	Configured bool   `json:"configured"`
	ProviderID string `json:"provider_id,omitempty"`
	Vendor     string `json:"vendor,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	ModelName  string `json:"model_name,omitempty"`

	ComponentRuntimeStatus
}

type ProvidersSetupStatus struct {
	Configured        bool      `json:"configured"`
	Loaded            bool      `json:"loaded"`
	Path              string    `json:"path,omitempty"`
	UpdatedAt         time.Time `json:"updated_at,omitempty"`
	PrimaryProviderID string    `json:"primary_provider_id,omitempty"`
	AssistProviderID  string    `json:"assist_provider_id,omitempty"`
}

type ConnectorsSetupStatus struct {
	Configured           bool                `json:"configured"`
	Loaded               bool                `json:"loaded"`
	Path                 string              `json:"path,omitempty"`
	UpdatedAt            time.Time           `json:"updated_at,omitempty"`
	TotalEntries         int                 `json:"total_entries"`
	EnabledEntries       int                 `json:"enabled_entries"`
	Kinds                []string            `json:"kinds,omitempty"`
	MetricsRuntime       *RuntimeSetupStatus `json:"metrics_runtime,omitempty"`
	ExecutionRuntime     *RuntimeSetupStatus `json:"execution_runtime,omitempty"`
	VerificationRuntime  *RuntimeSetupStatus `json:"verification_runtime,omitempty"`
	ObservabilityRuntime *RuntimeSetupStatus `json:"observability_runtime,omitempty"`
	DeliveryRuntime      *RuntimeSetupStatus `json:"delivery_runtime,omitempty"`
}

type LegacyFallbacksSetupStatus struct {
	Metrics      ProviderSetupStatus `json:"metrics,omitempty"`
	Execution    SSHSetupStatus      `json:"execution,omitempty"`
	Verification SSHSetupStatus      `json:"verification,omitempty"`
}

type SmokeDefaultsSetupStatus struct {
	Hosts []string `json:"hosts,omitempty"`
}

type SSHSetupStatus struct {
	Configured                 bool     `json:"configured"`
	User                       string   `json:"user,omitempty"`
	AllowedHosts               []string `json:"allowed_hosts,omitempty"`
	AllowedHostsCount          int      `json:"allowed_hosts_count"`
	PrivateKeyConfigured       bool     `json:"private_key_configured"`
	PrivateKeyExists           bool     `json:"private_key_exists"`
	HostKeyCheckingDisabled    bool     `json:"host_key_checking_disabled"`
	ServiceCommandAllowlistSet bool     `json:"service_command_allowlist_set"`

	ComponentRuntimeStatus
}

type SmokeSessionStatus struct {
	SessionID          string    `json:"session_id"`
	Status             string    `json:"status"`
	AlertName          string    `json:"alertname,omitempty"`
	Service            string    `json:"service,omitempty"`
	Host               string    `json:"host,omitempty"`
	TelegramTarget     string    `json:"telegram_target,omitempty"`
	ApprovalRequested  bool      `json:"approval_requested"`
	ExecutionStatus    string    `json:"execution_status,omitempty"`
	VerificationStatus string    `json:"verification_status,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitempty"`
}

type OutboxListResponse struct {
	Items []OutboxEvent `json:"items"`
	ListPage
}

type ExecutionListResponse struct {
	Items []ExecutionDetail `json:"items"`
	ListPage
}

type OutboxEvent struct {
	ID            string    `json:"id"`
	Topic         string    `json:"topic"`
	Status        string    `json:"status"`
	AggregateID   string    `json:"aggregate_id"`
	RetryCount    int       `json:"retry_count"`
	LastError     string    `json:"last_error,omitempty"`
	BlockedReason string    `json:"blocked_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type AcceptedResponse struct {
	Accepted   bool   `json:"accepted"`
	Message    string `json:"message,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
}

type BatchOperationResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type BatchOperationResponse struct {
	Operation    string                 `json:"operation"`
	ResourceType string                 `json:"resource_type"`
	Total        int                    `json:"total"`
	Succeeded    int                    `json:"succeeded"`
	Failed       int                    `json:"failed"`
	Results      []BatchOperationResult `json:"results"`
}

type SessionExportResponse struct {
	ResourceType   string                 `json:"resource_type"`
	ExportedAt     time.Time              `json:"exported_at"`
	OperatorReason string                 `json:"operator_reason"`
	TotalRequested int                    `json:"total_requested"`
	ExportedCount  int                    `json:"exported_count"`
	FailedCount    int                    `json:"failed_count"`
	Items          []SessionDetail        `json:"items"`
	Failures       []BatchOperationResult `json:"failures,omitempty"`
}

type AuditExportResponse struct {
	ResourceType   string                 `json:"resource_type"`
	ExportedAt     time.Time              `json:"exported_at"`
	OperatorReason string                 `json:"operator_reason"`
	TotalRequested int                    `json:"total_requested"`
	ExportedCount  int                    `json:"exported_count"`
	FailedCount    int                    `json:"failed_count"`
	Items          []AuditRecord          `json:"items"`
	Failures       []BatchOperationResult `json:"failures,omitempty"`
}

type KnowledgeExportResponse struct {
	ResourceType   string                 `json:"resource_type"`
	ExportedAt     time.Time              `json:"exported_at"`
	OperatorReason string                 `json:"operator_reason"`
	TotalRequested int                    `json:"total_requested"`
	ExportedCount  int                    `json:"exported_count"`
	FailedCount    int                    `json:"failed_count"`
	Items          []KnowledgeRecord      `json:"items"`
	Failures       []BatchOperationResult `json:"failures,omitempty"`
}

type ExecutionExportResponse struct {
	ResourceType   string                 `json:"resource_type"`
	ExportedAt     time.Time              `json:"exported_at"`
	OperatorReason string                 `json:"operator_reason"`
	TotalRequested int                    `json:"total_requested"`
	ExportedCount  int                    `json:"exported_count"`
	FailedCount    int                    `json:"failed_count"`
	Items          []ExecutionDetail      `json:"items"`
	Failures       []BatchOperationResult `json:"failures,omitempty"`
}

type SmokeAlertResponse struct {
	Accepted   bool   `json:"accepted"`
	SessionID  string `json:"session_id"`
	Status     string `json:"status"`
	Duplicated bool   `json:"duplicated,omitempty"`
	TGTarget   string `json:"tg_target,omitempty"`
}

type AuthorizationSetupStatus struct {
	Configured bool      `json:"configured"`
	Loaded     bool      `json:"loaded"`
	Path       string    `json:"path,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type ApprovalRoutingSetupStatus struct {
	Configured bool      `json:"configured"`
	Loaded     bool      `json:"loaded"`
	Path       string    `json:"path,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type ReasoningPromptSetupStatus struct {
	Configured                  bool      `json:"configured"`
	Loaded                      bool      `json:"loaded"`
	Path                        string    `json:"path,omitempty"`
	UpdatedAt                   time.Time `json:"updated_at,omitempty"`
	LocalCommandFallbackEnabled bool      `json:"local_command_fallback_enabled"`
}

type DesensitizationSetupStatus struct {
	Configured            bool      `json:"configured"`
	Loaded                bool      `json:"loaded"`
	Path                  string    `json:"path,omitempty"`
	UpdatedAt             time.Time `json:"updated_at,omitempty"`
	Enabled               bool      `json:"enabled"`
	LocalLLMAssistEnabled bool      `json:"local_llm_assist_enabled"`
	LocalLLMBaseURL       string    `json:"local_llm_base_url,omitempty"`
	LocalLLMModel         string    `json:"local_llm_model,omitempty"`
	LocalLLMMode          string    `json:"local_llm_mode,omitempty"`
}

type AuthorizationOverrideConfig struct {
	ID           string   `json:"id,omitempty"`
	Services     []string `json:"services,omitempty"`
	Hosts        []string `json:"hosts,omitempty"`
	Channels     []string `json:"channels,omitempty"`
	CommandGlobs []string `json:"command_globs,omitempty"`
	Action       string   `json:"action"`
}

type AuthorizationPolicyConfig struct {
	WhitelistAction     string                        `json:"whitelist_action"`
	BlacklistAction     string                        `json:"blacklist_action"`
	UnmatchedAction     string                        `json:"unmatched_action"`
	NormalizeWhitespace bool                          `json:"normalize_whitespace"`
	HardDenySSHCommand  []string                      `json:"hard_deny_ssh_command,omitempty"`
	HardDenyMCPSkill    []string                      `json:"hard_deny_mcp_skill,omitempty"`
	Whitelist           []string                      `json:"whitelist,omitempty"`
	Blacklist           []string                      `json:"blacklist,omitempty"`
	Overrides           []AuthorizationOverrideConfig `json:"overrides,omitempty"`
}

type RouteEntry struct {
	Key     string   `json:"key"`
	Targets []string `json:"targets"`
}

type ApprovalRoutingConfig struct {
	ProhibitSelfApproval bool         `json:"prohibit_self_approval"`
	ServiceOwners        []RouteEntry `json:"service_owners,omitempty"`
	OncallGroups         []RouteEntry `json:"oncall_groups,omitempty"`
	CommandAllowlist     []RouteEntry `json:"command_allowlist,omitempty"`
}

type ReasoningPromptConfig struct {
	SystemPrompt       string `json:"system_prompt"`
	UserPromptTemplate string `json:"user_prompt_template"`
}

type ProviderBinding struct {
	ProviderID string `json:"provider_id,omitempty"`
	Model      string `json:"model,omitempty"`
}

type ProviderEntry struct {
	ID          string             `json:"id"`
	Vendor      string             `json:"vendor,omitempty"`
	Protocol    string             `json:"protocol,omitempty"`
	BaseURL     string             `json:"base_url,omitempty"`
	APIKey      string             `json:"api_key,omitempty"`
	APIKeyRef   string             `json:"api_key_ref,omitempty"`
	APIKeySet   bool               `json:"api_key_set"`
	OrgID       string             `json:"org_id,omitempty"`
	TenantID    string             `json:"tenant_id,omitempty"`
	WorkspaceID string             `json:"workspace_id,omitempty"`
	Enabled     bool               `json:"enabled"`
	Templates   []ProviderTemplate `json:"templates,omitempty"`
}

type ProviderTemplate struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Values      map[string]string `json:"values,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
}

type ProvidersConfig struct {
	Primary ProviderBinding `json:"primary"`
	Assist  ProviderBinding `json:"assist"`
	Entries []ProviderEntry `json:"entries,omitempty"`
}

type ProvidersConfigResponse struct {
	Configured bool            `json:"configured"`
	Loaded     bool            `json:"loaded"`
	Path       string          `json:"path,omitempty"`
	UpdatedAt  time.Time       `json:"updated_at,omitempty"`
	Content    string          `json:"content,omitempty"`
	Config     ProvidersConfig `json:"config"`
}

type ProviderBindings struct {
	Primary ProviderBinding `json:"primary"`
	Assist  ProviderBinding `json:"assist"`
}

type ProviderBindingsResponse struct {
	Configured bool             `json:"configured"`
	Loaded     bool             `json:"loaded"`
	Path       string           `json:"path,omitempty"`
	UpdatedAt  time.Time        `json:"updated_at,omitempty"`
	Bindings   ProviderBindings `json:"bindings"`
}

type ProviderListModelsRequest struct {
	ProviderID string                 `json:"provider_id,omitempty"`
	Provider   *ProviderRegistryEntry `json:"provider,omitempty"`
}

type ProviderModelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type ProviderListModelsResponse struct {
	ProviderID string              `json:"provider_id"`
	Models     []ProviderModelInfo `json:"models,omitempty"`
}

type ProviderCheckRequest struct {
	ProviderID string                 `json:"provider_id,omitempty"`
	Model      string                 `json:"model,omitempty"`
	Provider   *ProviderRegistryEntry `json:"provider,omitempty"`
}

type ProviderCheckResponse struct {
	ProviderID string `json:"provider_id"`
	Available  bool   `json:"available"`
	Detail     string `json:"detail,omitempty"`
}

type ConnectorCapability struct {
	ID          string   `json:"id,omitempty"`
	Action      string   `json:"action,omitempty"`
	ReadOnly    bool     `json:"read_only"`
	Invocable   bool     `json:"invocable"`
	Scopes      []string `json:"scopes,omitempty"`
	Description string   `json:"description,omitempty"`
}

type ConnectorInvokeCapabilityRequest struct {
	CapabilityID string                 `json:"capability_id"`
	Params       map[string]interface{} `json:"params,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	Caller       string                 `json:"caller,omitempty"`
}

type ConnectorInvokeCapabilityResponse struct {
	ConnectorID  string                 `json:"connector_id"`
	CapabilityID string                 `json:"capability_id"`
	Status       string                 `json:"status"`
	Output       map[string]interface{} `json:"output,omitempty"`
	Artifacts    []MessageAttachment    `json:"artifacts,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Runtime      *RuntimeMetadata       `json:"runtime,omitempty"`
}

type ConnectorField struct {
	Key         string   `json:"key,omitempty"`
	Label       string   `json:"label,omitempty"`
	Type        string   `json:"type,omitempty"`
	Required    bool     `json:"required"`
	Secret      bool     `json:"secret,omitempty"`
	Default     string   `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
}

type ConnectorImportExport struct {
	Exportable bool     `json:"exportable"`
	Importable bool     `json:"importable"`
	Formats    []string `json:"formats,omitempty"`
}

type ConnectorCompatibility struct {
	TARSMajorVersions     []string `json:"tars_major_versions,omitempty"`
	UpstreamMajorVersions []string `json:"upstream_major_versions,omitempty"`
	Modes                 []string `json:"modes,omitempty"`
}

type ConnectorMarketplace struct {
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Source   string   `json:"source,omitempty"`
}

type ConnectorMetadata struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Vendor      string `json:"vendor,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	OrgID       string `json:"org_id,omitempty"`
	TenantID    string `json:"tenant_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type ConnectorSpec struct {
	Type           string                `json:"type,omitempty"`
	Protocol       string                `json:"protocol,omitempty"`
	Capabilities   []ConnectorCapability `json:"capabilities,omitempty"`
	ConnectionForm []ConnectorField      `json:"connection_form,omitempty"`
	ImportExport   ConnectorImportExport `json:"import_export"`
}

type ConnectorRuntimeConfig struct {
	Values     map[string]string `json:"values,omitempty"`
	SecretRefs map[string]string `json:"secret_refs,omitempty"`
}

type SecretValueInput struct {
	Ref   string `json:"ref"`
	Value string `json:"value"`
}

type SecretDescriptor struct {
	OwnerType string    `json:"owner_type,omitempty"`
	OwnerID   string    `json:"owner_id,omitempty"`
	Key       string    `json:"key,omitempty"`
	Set       bool      `json:"set"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Source    string    `json:"source,omitempty"`
	Status    string    `json:"status,omitempty"`
}

type SecretsInventoryResponse struct {
	Configured             bool               `json:"configured"`
	Loaded                 bool               `json:"loaded"`
	Path                   string             `json:"path,omitempty"`
	UpdatedAt              time.Time          `json:"updated_at,omitempty"`
	CustodyConfigured      bool               `json:"custody_configured"`
	CustodyKeyID           string             `json:"custody_key_id,omitempty"`
	SSHCredentialConfigured bool              `json:"ssh_credential_configured"`
	Items                  []SecretDescriptor `json:"items"`
}

type SSHCredential struct {
	CredentialID  string     `json:"credential_id"`
	DisplayName   string     `json:"display_name,omitempty"`
	OwnerType     string     `json:"owner_type,omitempty"`
	OwnerID       string     `json:"owner_id,omitempty"`
	ConnectorID   string     `json:"connector_id,omitempty"`
	Username      string     `json:"username,omitempty"`
	AuthType      string     `json:"auth_type"`
	HostScope     string     `json:"host_scope,omitempty"`
	Status        string     `json:"status"`
	CreatedBy     string     `json:"created_by,omitempty"`
	UpdatedBy     string     `json:"updated_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`
	LastRotatedAt time.Time  `json:"last_rotated_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

type SSHCredentialListResponse struct {
	Configured bool            `json:"configured"`
	Items      []SSHCredential `json:"items"`
}

type SSHCredentialUpsertRequest struct {
	CredentialID   string     `json:"credential_id,omitempty"`
	DisplayName    string     `json:"display_name,omitempty"`
	OwnerType      string     `json:"owner_type,omitempty"`
	OwnerID        string     `json:"owner_id,omitempty"`
	ConnectorID    string     `json:"connector_id,omitempty"`
	Username       string     `json:"username,omitempty"`
	AuthType       string     `json:"auth_type,omitempty"`
	Password       string     `json:"password,omitempty"`
	PrivateKey     string     `json:"private_key,omitempty"`
	Passphrase     string     `json:"passphrase,omitempty"`
	HostScope      string     `json:"host_scope,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	OperatorReason string     `json:"operator_reason"`
}

type SSHCredentialStatusRequest struct {
	Status         string `json:"status,omitempty"`
	OperatorReason string `json:"operator_reason"`
}

type ConnectorManifest struct {
	APIVersion    string                 `json:"api_version,omitempty"`
	Kind          string                 `json:"kind,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
	Metadata      ConnectorMetadata      `json:"metadata"`
	Spec          ConnectorSpec          `json:"spec"`
	Config        ConnectorRuntimeConfig `json:"config,omitempty"`
	Compatibility ConnectorCompatibility `json:"compatibility"`
	Marketplace   ConnectorMarketplace   `json:"marketplace"`
	Lifecycle     *ConnectorLifecycle    `json:"lifecycle,omitempty"`
}

type ConnectorRuntimeMetadata struct {
	Type     string `json:"type,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Vendor   string `json:"vendor,omitempty"`
	Mode     string `json:"mode,omitempty"`
	State    string `json:"state,omitempty"`
}

type ConnectorCompatibilityReport struct {
	Compatible       bool      `json:"compatible"`
	CurrentTARSMajor string    `json:"current_tars_major,omitempty"`
	Reasons          []string  `json:"reasons,omitempty"`
	CheckedAt        time.Time `json:"checked_at,omitempty"`
}

type ConnectorHealthStatus struct {
	Status           string    `json:"status,omitempty"`
	CredentialStatus string    `json:"credential_status,omitempty"`
	Summary          string    `json:"summary,omitempty"`
	CheckedAt        time.Time `json:"checked_at,omitempty"`
	RuntimeState     string    `json:"runtime_state,omitempty"`
}

type ConnectorLifecycleEvent struct {
	Type        string            `json:"type,omitempty"`
	Summary     string            `json:"summary,omitempty"`
	Version     string            `json:"version,omitempty"`
	FromVersion string            `json:"from_version,omitempty"`
	ToVersion   string            `json:"to_version,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
}

type ConnectorRevision struct {
	Version   string    `json:"version,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

type ConnectorLifecycle struct {
	ConnectorID      string                       `json:"connector_id,omitempty"`
	DisplayName      string                       `json:"display_name,omitempty"`
	CurrentVersion   string                       `json:"current_version,omitempty"`
	AvailableVersion string                       `json:"available_version,omitempty"`
	Enabled          bool                         `json:"enabled"`
	InstalledAt      time.Time                    `json:"installed_at,omitempty"`
	UpdatedAt        time.Time                    `json:"updated_at,omitempty"`
	Runtime          ConnectorRuntimeMetadata     `json:"runtime"`
	Compatibility    ConnectorCompatibilityReport `json:"compatibility"`
	Health           ConnectorHealthStatus        `json:"health"`
	History          []ConnectorLifecycleEvent    `json:"history,omitempty"`
	HealthHistory    []ConnectorHealthStatus      `json:"health_history,omitempty"`
	Revisions        []ConnectorRevision          `json:"revisions,omitempty"`
}

type ConnectorTemplate struct {
	ConnectorID string            `json:"connector_id,omitempty"`
	TemplateID  string            `json:"template_id,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Values      map[string]string `json:"values,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
}

type ConnectorTemplateListResponse struct {
	Items []ConnectorTemplate `json:"items"`
}

type ConnectorsConfig struct {
	Entries []ConnectorManifest `json:"entries,omitempty"`
}

type ConnectorsConfigResponse struct {
	Configured bool             `json:"configured"`
	Loaded     bool             `json:"loaded"`
	Path       string           `json:"path,omitempty"`
	UpdatedAt  time.Time        `json:"updated_at,omitempty"`
	Content    string           `json:"content,omitempty"`
	Config     ConnectorsConfig `json:"config"`
}

type ConnectorListResponse struct {
	Items []ConnectorManifest `json:"items"`
	ListPage
}

type ConnectorMetricsQueryResponse struct {
	ConnectorID string                   `json:"connector_id"`
	Protocol    string                   `json:"protocol"`
	Service     string                   `json:"service,omitempty"`
	Host        string                   `json:"host,omitempty"`
	Mode        string                   `json:"mode,omitempty"`
	Query       string                   `json:"query,omitempty"`
	Start       time.Time                `json:"start,omitempty"`
	End         time.Time                `json:"end,omitempty"`
	Step        string                   `json:"step,omitempty"`
	Window      string                   `json:"window,omitempty"`
	Series      []map[string]interface{} `json:"series"`
	Runtime     *RuntimeMetadata         `json:"runtime,omitempty"`
}

type ConnectorExecutionResponse struct {
	ConnectorID     string           `json:"connector_id"`
	ExecutionID     string           `json:"execution_id"`
	SessionID       string           `json:"session_id,omitempty"`
	Status          string           `json:"status"`
	Protocol        string           `json:"protocol,omitempty"`
	ExecutionMode   string           `json:"execution_mode,omitempty"`
	Runtime         *RuntimeMetadata `json:"runtime,omitempty"`
	TargetHost      string           `json:"target_host,omitempty"`
	Command         string           `json:"command,omitempty"`
	ExitCode        int              `json:"exit_code,omitempty"`
	OutputRef       string           `json:"output_ref,omitempty"`
	OutputBytes     int64            `json:"output_bytes,omitempty"`
	OutputTruncated bool             `json:"output_truncated,omitempty"`
	OutputPreview   string           `json:"output_preview,omitempty"`
}

type DashboardConnectorHealth struct {
	ConnectorID      string    `json:"connector_id,omitempty"`
	DisplayName      string    `json:"display_name,omitempty"`
	Type             string    `json:"type,omitempty"`
	Protocol         string    `json:"protocol,omitempty"`
	Vendor           string    `json:"vendor,omitempty"`
	Status           string    `json:"status,omitempty"`
	CredentialStatus string    `json:"credential_status,omitempty"`
	Summary          string    `json:"summary,omitempty"`
	CurrentVersion   string    `json:"current_version,omitempty"`
	CheckedAt        time.Time `json:"checked_at,omitempty"`
}

type DashboardProviderHealth struct {
	ProviderID    string    `json:"provider_id,omitempty"`
	Vendor        string    `json:"vendor,omitempty"`
	Protocol      string    `json:"protocol,omitempty"`
	BaseURL       string    `json:"base_url,omitempty"`
	Enabled       bool      `json:"enabled"`
	LastResult    string    `json:"last_result,omitempty"`
	LastDetail    string    `json:"last_detail,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	LastChangedAt time.Time `json:"last_changed_at,omitempty"`
}

type DashboardAlertItem struct {
	Severity  string    `json:"severity,omitempty"`
	Resource  string    `json:"resource,omitempty"`
	Title     string    `json:"title,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type DashboardHealthSummary struct {
	HealthyConnectors  int `json:"healthy_connectors"`
	DegradedConnectors int `json:"degraded_connectors"`
	DisabledConnectors int `json:"disabled_connectors"`
	ConfiguredSecrets  int `json:"configured_secrets"`
	MissingSecrets     int `json:"missing_secrets"`
	ActiveAlerts       int `json:"active_alerts"`
	ProviderFailures   int `json:"provider_failures"`
}

type DashboardResourceSample struct {
	Label string  `json:"label,omitempty"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

type DashboardRuntimeResources struct {
	UptimeSeconds      int64                     `json:"uptime_seconds"`
	Goroutines         int                       `json:"goroutines"`
	HeapAllocBytes     uint64                    `json:"heap_alloc_bytes"`
	HeapSysBytes       uint64                    `json:"heap_sys_bytes"`
	HeapInUseBytes     uint64                    `json:"heap_in_use_bytes"`
	StackInUseBytes    uint64                    `json:"stack_in_use_bytes"`
	GCCount            uint32                    `json:"gc_count"`
	LastGCPauseSeconds float64                   `json:"last_gc_pause_seconds"`
	DiskUsedBytes      uint64                    `json:"disk_used_bytes"`
	DiskFreeBytes      uint64                    `json:"disk_free_bytes"`
	DiskUsagePercent   float64                   `json:"disk_usage_percent"`
	CPUCount           int                       `json:"cpu_count"`
	LoadAverage        []DashboardResourceSample `json:"load_average,omitempty"`
	NetworkInterfaces  []DashboardResourceSample `json:"network_interfaces,omitempty"`
	TracingEnabled     bool                      `json:"tracing_enabled"`
	TracingProvider    string                    `json:"tracing_provider,omitempty"`
	LogLevel           string                    `json:"log_level,omitempty"`
	SpoolDir           string                    `json:"spool_dir,omitempty"`
}

type DashboardHealthResponse struct {
	Summary    DashboardHealthSummary     `json:"summary"`
	Resources  DashboardRuntimeResources  `json:"resources"`
	Connectors []DashboardConnectorHealth `json:"connectors"`
	Providers  []DashboardProviderHealth  `json:"providers"`
	Secrets    []SecretDescriptor         `json:"secrets"`
	Alerts     []DashboardAlertItem       `json:"alerts"`
}

type IdentityLink struct {
	ProviderType     string `json:"provider_type,omitempty"`
	ProviderID       string `json:"provider_id,omitempty"`
	ExternalSubject  string `json:"external_subject,omitempty"`
	ExternalUsername string `json:"external_username,omitempty"`
	ExternalEmail    string `json:"external_email,omitempty"`
}

type User struct {
	UserID               string         `json:"user_id,omitempty"`
	Username             string         `json:"username,omitempty"`
	DisplayName          string         `json:"display_name,omitempty"`
	Email                string         `json:"email,omitempty"`
	Status               string         `json:"status,omitempty"`
	Source               string         `json:"source,omitempty"`
	PasswordHash         string         `json:"password_hash,omitempty"`
	PasswordLoginEnabled bool           `json:"password_login_enabled"`
	PasswordUpdatedAt    time.Time      `json:"password_updated_at,omitempty"`
	ChallengeRequired    bool           `json:"challenge_required"`
	MFAEnabled           bool           `json:"mfa_enabled"`
	MFAMethod            string         `json:"mfa_method,omitempty"`
	TOTPSecret           string         `json:"totp_secret,omitempty"`
	Roles                []string       `json:"roles,omitempty"`
	Groups               []string       `json:"groups,omitempty"`
	Identities           []IdentityLink `json:"identities,omitempty"`
	// Org affiliation (ORG-N1)
	OrgID       string    `json:"org_id,omitempty"`
	TenantID    string    `json:"tenant_id,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type UserListResponse struct {
	Items []User `json:"items"`
	ListPage
}

type Group struct {
	GroupID     string   `json:"group_id,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Members     []string `json:"members,omitempty"`
	// Org affiliation (ORG-N1)
	OrgID       string    `json:"org_id,omitempty"`
	TenantID    string    `json:"tenant_id,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type GroupListResponse struct {
	Items []Group `json:"items"`
	ListPage
}

type AuthProvider struct {
	ID                  string   `json:"id,omitempty"`
	Type                string   `json:"type,omitempty"`
	Name                string   `json:"name,omitempty"`
	Enabled             bool     `json:"enabled"`
	PasswordMinLength   int      `json:"password_min_length,omitempty"`
	RequireChallenge    bool     `json:"require_challenge"`
	ChallengeChannel    string   `json:"challenge_channel,omitempty"`
	ChallengeTTLSeconds int      `json:"challenge_ttl_seconds,omitempty"`
	ChallengeCodeLength int      `json:"challenge_code_length,omitempty"`
	RequireMFA          bool     `json:"require_mfa"`
	LoginURL            string   `json:"login_url,omitempty"`
	IssuerURL           string   `json:"issuer_url,omitempty"`
	ClientID            string   `json:"client_id,omitempty"`
	ClientSecret        string   `json:"client_secret,omitempty"`
	ClientSecretSet     bool     `json:"client_secret_set"`
	ClientSecretRef     string   `json:"client_secret_ref,omitempty"`
	AuthURL             string   `json:"auth_url,omitempty"`
	TokenURL            string   `json:"token_url,omitempty"`
	UserInfoURL         string   `json:"user_info_url,omitempty"`
	SessionTTLSeconds   int      `json:"session_ttl_seconds,omitempty"`
	LDAPURL             string   `json:"ldap_url,omitempty"`
	BindDN              string   `json:"bind_dn,omitempty"`
	BindPassword        string   `json:"bind_password,omitempty"`
	BindPasswordSet     bool     `json:"bind_password_set"`
	BindPasswordRef     string   `json:"bind_password_ref,omitempty"`
	BaseDN              string   `json:"base_dn,omitempty"`
	UserSearchFilter    string   `json:"user_search_filter,omitempty"`
	GroupSearchFilter   string   `json:"group_search_filter,omitempty"`
	RedirectPath        string   `json:"redirect_path,omitempty"`
	SuccessRedirect     string   `json:"success_redirect,omitempty"`
	UserIDField         string   `json:"user_id_field,omitempty"`
	UsernameField       string   `json:"username_field,omitempty"`
	DisplayNameField    string   `json:"display_name_field,omitempty"`
	EmailField          string   `json:"email_field,omitempty"`
	AllowedDomains      []string `json:"allowed_domains,omitempty"`
	Scopes              []string `json:"scopes,omitempty"`
	DefaultRoles        []string `json:"default_roles,omitempty"`
	AllowJIT            bool     `json:"allow_jit"`
}

type AuthProviderListResponse struct {
	Items []AuthProvider `json:"items"`
}

type AuthLoginResponse struct {
	SessionToken       string    `json:"session_token,omitempty"`
	RedirectURL        string    `json:"redirect_url,omitempty"`
	ProviderID         string    `json:"provider_id,omitempty"`
	PendingToken       string    `json:"pending_token,omitempty"`
	NextStep           string    `json:"next_step,omitempty"`
	ChallengeID        string    `json:"challenge_id,omitempty"`
	ChallengeChannel   string    `json:"challenge_channel,omitempty"`
	ChallengeCode      string    `json:"challenge_code,omitempty"`
	ChallengeExpiresAt time.Time `json:"challenge_expires_at,omitempty"`
	User               User      `json:"user"`
	Roles              []string  `json:"roles,omitempty"`
	Permissions        []string  `json:"permissions,omitempty"`
}

type AuthChallengeRequest struct {
	PendingToken string `json:"pending_token,omitempty"`
}

type AuthChallengeResponse struct {
	ProviderID         string    `json:"provider_id,omitempty"`
	PendingToken       string    `json:"pending_token,omitempty"`
	NextStep           string    `json:"next_step,omitempty"`
	ChallengeID        string    `json:"challenge_id,omitempty"`
	ChallengeChannel   string    `json:"challenge_channel,omitempty"`
	ChallengeCode      string    `json:"challenge_code,omitempty"`
	ChallengeExpiresAt time.Time `json:"challenge_expires_at,omitempty"`
}

type AuthVerifyRequest struct {
	PendingToken string `json:"pending_token,omitempty"`
	ChallengeID  string `json:"challenge_id,omitempty"`
	Code         string `json:"code,omitempty"`
}

type AuthMFAVerifyRequest struct {
	PendingToken string `json:"pending_token,omitempty"`
	Code         string `json:"code,omitempty"`
}

type MeResponse struct {
	User             User      `json:"user"`
	Roles            []string  `json:"roles,omitempty"`
	Permissions      []string  `json:"permissions,omitempty"`
	AuthSource       string    `json:"auth_source,omitempty"`
	BreakGlass       bool      `json:"break_glass"`
	SessionToken     string    `json:"session_token,omitempty"`
	SessionExpiresAt time.Time `json:"session_expires_at,omitempty"`
}

type SessionInventoryItem struct {
	TokenMasked string    `json:"token_masked,omitempty"`
	UserID      string    `json:"user_id,omitempty"`
	ProviderID  string    `json:"provider_id,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	LastSeenAt  time.Time `json:"last_seen_at,omitempty"`
}

type SessionInventoryResponse struct {
	Items []SessionInventoryItem `json:"items"`
}

type Role struct {
	ID          string   `json:"id,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

type RoleListResponse struct {
	Items []Role `json:"items"`
}

type RoleBindingRequest struct {
	UserIDs        []string `json:"user_ids,omitempty"`
	GroupIDs       []string `json:"group_ids,omitempty"`
	OperatorReason string   `json:"operator_reason"`
}

type RoleBindingsResponse struct {
	RoleID   string   `json:"role_id"`
	UserIDs  []string `json:"user_ids"`
	GroupIDs []string `json:"group_ids"`
}

type Person struct {
	ID             string            `json:"id,omitempty"`
	DisplayName    string            `json:"display_name,omitempty"`
	Email          string            `json:"email,omitempty"`
	Status         string            `json:"status,omitempty"`
	LinkedUserID   string            `json:"linked_user_id,omitempty"`
	ChannelIDs     []string          `json:"channel_ids,omitempty"`
	Team           string            `json:"team,omitempty"`
	ApprovalTarget string            `json:"approval_target,omitempty"`
	OncallSchedule string            `json:"oncall_schedule,omitempty"`
	Preferences    map[string]string `json:"preferences,omitempty"`
	// Org affiliation (ORG-N1)
	OrgID       string    `json:"org_id,omitempty"`
	TenantID    string    `json:"tenant_id,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type PersonListResponse struct {
	Items []Person `json:"items"`
	ListPage
}

type ChannelUsage string

const (
	ChannelUsageApproval     ChannelUsage = "approval"
	ChannelUsageNotification ChannelUsage = "notification"
	ChannelUsageAlert        ChannelUsage = "alert"
)

type ChannelCapability string

const (
	ChannelCapabilityText   ChannelCapability = "text"
	ChannelCapabilityImage  ChannelCapability = "image"
	ChannelCapabilityFile   ChannelCapability = "file"
	ChannelCapabilityAction ChannelCapability = "action"
)

type Channel struct {
	ID           string              `json:"id,omitempty"`
	Kind         string              `json:"kind,omitempty"`
	Type         string              `json:"type,omitempty"` // Deprecated: use Kind
	Name         string              `json:"name,omitempty"`
	Target       string              `json:"target,omitempty"`
	Enabled      bool                `json:"enabled"`
	LinkedUsers  []string            `json:"linked_users,omitempty"`
	Usages       []ChannelUsage      `json:"usages,omitempty"`
	Capabilities []ChannelCapability `json:"capabilities,omitempty"`
	// Org affiliation (ORG-N1)
	OrgID       string    `json:"org_id,omitempty"`
	TenantID    string    `json:"tenant_id,omitempty"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type ChannelListResponse struct {
	Items []Channel `json:"items"`
	ListPage
}

type ProviderRegistryEntry struct {
	ID           string             `json:"id,omitempty"`
	Vendor       string             `json:"vendor,omitempty"`
	Protocol     string             `json:"protocol,omitempty"`
	BaseURL      string             `json:"base_url,omitempty"`
	APIKey       string             `json:"api_key,omitempty"`
	APIKeyRef    string             `json:"api_key_ref,omitempty"`
	APIKeySet    bool               `json:"api_key_set"`
	Enabled      bool               `json:"enabled"`
	OrgID        string             `json:"org_id,omitempty"`
	TenantID     string             `json:"tenant_id,omitempty"`
	WorkspaceID  string             `json:"workspace_id,omitempty"`
	PrimaryModel string             `json:"primary_model,omitempty"`
	AssistModel  string             `json:"assist_model,omitempty"`
	Templates    []ProviderTemplate `json:"templates,omitempty"`
}

type ProviderRegistryListResponse struct {
	Items []ProviderRegistryEntry `json:"items"`
	ListPage
}

type AccessConfig struct {
	Users         []User         `json:"users,omitempty"`
	Groups        []Group        `json:"groups,omitempty"`
	AuthProviders []AuthProvider `json:"auth_providers,omitempty"`
	Roles         []Role         `json:"roles,omitempty"`
	People        []Person       `json:"people,omitempty"`
	Channels      []Channel      `json:"channels,omitempty"`
}

type AccessConfigResponse struct {
	Configured bool         `json:"configured"`
	Loaded     bool         `json:"loaded"`
	Path       string       `json:"path,omitempty"`
	UpdatedAt  time.Time    `json:"updated_at,omitempty"`
	Content    string       `json:"content,omitempty"`
	Config     AccessConfig `json:"config"`
}

type DesensitizationSecretConfig struct {
	KeyNames           []string `json:"key_names,omitempty"`
	QueryKeyNames      []string `json:"query_key_names,omitempty"`
	AdditionalPatterns []string `json:"additional_patterns,omitempty"`
	RedactBearer       bool     `json:"redact_bearer"`
	RedactBasicAuthURL bool     `json:"redact_basic_auth_url"`
	RedactSKTokens     bool     `json:"redact_sk_tokens"`
}

type DesensitizationPlaceholderConfig struct {
	HostKeyFragments  []string `json:"host_key_fragments,omitempty"`
	PathKeyFragments  []string `json:"path_key_fragments,omitempty"`
	ReplaceInlineIP   bool     `json:"replace_inline_ip"`
	ReplaceInlineHost bool     `json:"replace_inline_host"`
	ReplaceInlinePath bool     `json:"replace_inline_path"`
}

type DesensitizationRehydrationConfig struct {
	Host bool `json:"host"`
	IP   bool `json:"ip"`
	Path bool `json:"path"`
}

type LocalLLMAssistConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider,omitempty"`
	BaseURL  string `json:"base_url,omitempty"`
	Model    string `json:"model,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

type DesensitizationConfig struct {
	Enabled        bool                             `json:"enabled"`
	Secrets        DesensitizationSecretConfig      `json:"secrets"`
	Placeholders   DesensitizationPlaceholderConfig `json:"placeholders"`
	Rehydration    DesensitizationRehydrationConfig `json:"rehydration"`
	LocalLLMAssist LocalLLMAssistConfig             `json:"local_llm_assist"`
}

type AuthorizationConfigResponse struct {
	Configured bool                      `json:"configured"`
	Loaded     bool                      `json:"loaded"`
	Path       string                    `json:"path,omitempty"`
	UpdatedAt  time.Time                 `json:"updated_at,omitempty"`
	Content    string                    `json:"content,omitempty"`
	Config     AuthorizationPolicyConfig `json:"config"`
}

type ApprovalRoutingConfigResponse struct {
	Configured bool                  `json:"configured"`
	Loaded     bool                  `json:"loaded"`
	Path       string                `json:"path,omitempty"`
	UpdatedAt  time.Time             `json:"updated_at,omitempty"`
	Content    string                `json:"content,omitempty"`
	Config     ApprovalRoutingConfig `json:"config"`
}

type ReasoningPromptConfigResponse struct {
	Configured bool                  `json:"configured"`
	Loaded     bool                  `json:"loaded"`
	Path       string                `json:"path,omitempty"`
	UpdatedAt  time.Time             `json:"updated_at,omitempty"`
	Content    string                `json:"content,omitempty"`
	Config     ReasoningPromptConfig `json:"config"`
}

type DesensitizationConfigResponse struct {
	Configured bool                  `json:"configured"`
	Loaded     bool                  `json:"loaded"`
	Path       string                `json:"path,omitempty"`
	UpdatedAt  time.Time             `json:"updated_at,omitempty"`
	Content    string                `json:"content,omitempty"`
	Config     DesensitizationConfig `json:"config"`
}

// ---------------------------------------------------------------------------
// Organization / Tenant / Workspace DTOs
// ---------------------------------------------------------------------------

// OrgAffiliation is embedded in platform objects that will gain org/tenant
// ownership in future iterations. All fields are optional-omitempty so
// existing objects are not broken.
type OrgAffiliation struct {
	OrgID       string `json:"org_id,omitempty"`
	TenantID    string `json:"tenant_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// Organization is the top-level enterprise entity.
type Organization struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug,omitempty"`
	Status      string    `json:"status,omitempty"`
	Description string    `json:"description,omitempty"`
	Domain      string    `json:"domain,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// Tenant is a logical partition inside an Organization.
type Tenant struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id,omitempty"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug,omitempty"`
	Status          string    `json:"status,omitempty"`
	Description     string    `json:"description,omitempty"`
	DefaultLocale   string    `json:"default_locale,omitempty"`
	DefaultTimezone string    `json:"default_timezone,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// Workspace is an operational context inside a Tenant.
type Workspace struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id,omitempty"`
	OrgID       string    `json:"org_id,omitempty"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug,omitempty"`
	Status      string    `json:"status,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type OrganizationListResponse struct {
	Items []Organization `json:"items"`
	ListPage
}

type TenantListResponse struct {
	Items []Tenant `json:"items"`
	ListPage
}

type WorkspaceListResponse struct {
	Items []Workspace `json:"items"`
	ListPage
}

// OrgContextResponse is returned by /api/v1/org/context to let clients
// discover the default single-tenant context without any extra config.
type OrgContextResponse struct {
	DefaultOrg       Organization `json:"default_org"`
	DefaultTenant    Tenant       `json:"default_tenant"`
	DefaultWorkspace Workspace    `json:"default_workspace"`
}

// ---------------------------------------------------------------------------
// Organization Policy DTOs (ORG-N5)
// ---------------------------------------------------------------------------

// OrgPolicyDTO is the JSON representation of an org-level policy.
type OrgPolicyDTO struct {
	OrgID string `json:"org_id,omitempty"`

	AllowedAuthMethods []string `json:"allowed_auth_methods,omitempty"`
	RequireMFA         bool     `json:"require_mfa"`

	MaxSessionDurationS int64 `json:"max_session_duration_s,omitempty"`
	IdleSessionTimeoutS int64 `json:"idle_session_timeout_s,omitempty"`

	RequireApprovalForExecution bool `json:"require_approval_for_execution"`
	ProhibitSelfApproval        bool `json:"prohibit_self_approval"`

	DefaultJITRoles []string `json:"default_jit_roles,omitempty"`

	SkillAllowlist []string `json:"skill_allowlist,omitempty"`
	SkillBlocklist []string `json:"skill_blocklist,omitempty"`

	AuditRetentionDays     int `json:"audit_retention_days,omitempty"`
	KnowledgeRetentionDays int `json:"knowledge_retention_days,omitempty"`

	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// TenantPolicyDTO is the JSON representation of a tenant-level policy override.
// Null / absent fields mean "inherit from org".
type TenantPolicyDTO struct {
	TenantID string `json:"tenant_id"`
	OrgID    string `json:"org_id,omitempty"`

	AllowedAuthMethods *[]string `json:"allowed_auth_methods,omitempty"`
	RequireMFA         *bool     `json:"require_mfa,omitempty"`

	MaxSessionDurationS *int64 `json:"max_session_duration_s,omitempty"`
	IdleSessionTimeoutS *int64 `json:"idle_session_timeout_s,omitempty"`

	RequireApprovalForExecution *bool `json:"require_approval_for_execution,omitempty"`
	ProhibitSelfApproval        *bool `json:"prohibit_self_approval,omitempty"`

	DefaultJITRoles *[]string `json:"default_jit_roles,omitempty"`
	SkillAllowlist  *[]string `json:"skill_allowlist,omitempty"`
	SkillBlocklist  *[]string `json:"skill_blocklist,omitempty"`

	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ResolvedPolicyDTO is the effective policy after merging org + tenant layers.
type ResolvedPolicyDTO struct {
	OrgID    string `json:"org_id"`
	TenantID string `json:"tenant_id"`

	AllowedAuthMethods          []string `json:"allowed_auth_methods,omitempty"`
	RequireMFA                  bool     `json:"require_mfa"`
	MaxSessionDurationS         int64    `json:"max_session_duration_s,omitempty"`
	IdleSessionTimeoutS         int64    `json:"idle_session_timeout_s,omitempty"`
	RequireApprovalForExecution bool     `json:"require_approval_for_execution"`
	ProhibitSelfApproval        bool     `json:"prohibit_self_approval"`
	DefaultJITRoles             []string `json:"default_jit_roles,omitempty"`
	SkillAllowlist              []string `json:"skill_allowlist,omitempty"`
	SkillBlocklist              []string `json:"skill_blocklist,omitempty"`
	AuditRetentionDays          int      `json:"audit_retention_days,omitempty"`
	KnowledgeRetentionDays      int      `json:"knowledge_retention_days,omitempty"`
}

// ---------------------------------------------------------------------------
// Inbox (in-app inbox / 站内信)
// ---------------------------------------------------------------------------

type InboxMessage struct {
	ID        string          `json:"id"`
	TenantID  string          `json:"tenant_id"`
	Subject   string          `json:"subject"`
	Body      string          `json:"body"`
	Channel   string          `json:"channel"`
	RefType   string          `json:"ref_type,omitempty"`
	RefID     string          `json:"ref_id,omitempty"`
	Source    string          `json:"source,omitempty"`
	Actions   []ChannelAction `json:"actions,omitempty"`
	IsRead    bool            `json:"is_read"`
	CreatedAt time.Time       `json:"created_at"`
	ReadAt    time.Time       `json:"read_at,omitempty"`
}

type ChannelAction struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type InboxListResponse struct {
	Items       []InboxMessage `json:"items"`
	UnreadCount int            `json:"unread_count"`
	ListPage
}

type InboxCreateRequest struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	RefType string `json:"ref_type,omitempty"`
	RefID   string `json:"ref_id,omitempty"`
	Source  string `json:"source,omitempty"`
}

// ---------------------------------------------------------------------------
// Triggers
// ---------------------------------------------------------------------------

type TriggerDTO struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenant_id"`
	DisplayName     string    `json:"display_name"`
	Description     string    `json:"description,omitempty"`
	Enabled         bool      `json:"enabled"`
	EventType       string    `json:"event_type"`
	ChannelID       string    `json:"channel_id,omitempty"`
	AutomationJobID string    `json:"automation_job_id,omitempty"`
	Governance      string    `json:"governance,omitempty"`
	FilterExpr      string    `json:"filter_expr,omitempty"`
	TargetAudience  string    `json:"target_audience,omitempty"`
	TemplateID      string    `json:"template_id,omitempty"`
	CooldownSec     int       `json:"cooldown_sec"`
	LastFiredAt     time.Time `json:"last_fired_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TriggerListResponse struct {
	Items []TriggerDTO `json:"items"`
	ListPage
}

type TriggerUpsertRequest struct {
	ID              string     `json:"id,omitempty"`
	TenantID        string     `json:"tenant_id,omitempty"`
	DisplayName     string     `json:"display_name,omitempty"`
	Description     string     `json:"description,omitempty"`
	Enabled         bool       `json:"enabled,omitempty"`
	EventType       string     `json:"event_type,omitempty"`
	ChannelID       string     `json:"channel_id,omitempty"`
	AutomationJobID string     `json:"automation_job_id,omitempty"`
	Governance      string     `json:"governance,omitempty"`
	FilterExpr      string     `json:"filter_expr,omitempty"`
	TargetAudience  string     `json:"target_audience,omitempty"`
	TemplateID      string     `json:"template_id,omitempty"`
	CooldownSec     int        `json:"cooldown_sec,omitempty"`
	Trigger         TriggerDTO `json:"trigger"`
	OperatorReason  string     `json:"operator_reason"`
}

// --- Chat API DTOs ---

type ChatMessageRequest struct {
	Message  string `json:"message"`
	Host     string `json:"host,omitempty"`
	Service  string `json:"service,omitempty"`
	Severity string `json:"severity,omitempty"`
}

type ChatMessageResponse struct {
	SessionID  string `json:"session_id"`
	Status     string `json:"status"`
	Duplicated bool   `json:"duplicated"`
	AckMessage string `json:"ack_message"`
}

type ChatSessionSummary struct {
	SessionID   string `json:"session_id"`
	Status      string `json:"status"`
	UserRequest string `json:"user_request"`
	Host        string `json:"host"`
	Service     string `json:"service"`
}

type ChatSessionsResponse struct {
	Items []ChatSessionSummary `json:"items"`
	Total int                  `json:"total"`
}

// ─── Agent Roles ───────────────────────────────────────────────────────────────

type AgentRoleListResponse struct {
	Items []AgentRole `json:"items"`
	ListPage
}

type AgentRole struct {
	RoleID      string `json:"role_id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	IsBuiltin   bool   `json:"is_builtin"`

	Profile           AgentRoleProfile           `json:"profile"`
	CapabilityBinding AgentRoleCapabilityBinding `json:"capability_binding"`
	PolicyBinding     AgentRolePolicyBinding     `json:"policy_binding"`
	ModelBinding      AgentRoleModelBinding      `json:"model_binding,omitempty"`

	OrgID    string `json:"org_id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AgentRoleProfile struct {
	SystemPrompt string   `json:"system_prompt"`
	PersonaTags  []string `json:"persona_tags,omitempty"`
}

type AgentRoleCapabilityBinding struct {
	AllowedConnectorCapabilities []string `json:"allowed_connector_capabilities,omitempty"`
	DeniedConnectorCapabilities  []string `json:"denied_connector_capabilities,omitempty"`
	AllowedSkills                []string `json:"allowed_skills,omitempty"`
	AllowedSkillTags             []string `json:"allowed_skill_tags,omitempty"`
	Mode                         string   `json:"mode"`
}

type AgentRolePolicyBinding struct {
	MaxRiskLevel       string   `json:"max_risk_level"`
	MaxAction          string   `json:"max_action"`
	RequireApprovalFor []string `json:"require_approval_for,omitempty"`
	HardDeny           []string `json:"hard_deny,omitempty"`
}

type AgentRoleModelTargetBinding struct {
	ProviderID string `json:"provider_id,omitempty"`
	Model      string `json:"model,omitempty"`
}

type AgentRoleModelBinding struct {
	Primary                *AgentRoleModelTargetBinding `json:"primary,omitempty"`
	Fallback               *AgentRoleModelTargetBinding `json:"fallback,omitempty"`
	InheritPlatformDefault bool                         `json:"inherit_platform_default,omitempty"`
}
