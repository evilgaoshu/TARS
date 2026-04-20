export interface SessionVerification {
  status: string;
  summary: string;
  details?: Record<string, unknown>;
  runtime?: RuntimeMetadata;
  checked_at?: string;
}

export interface TimelineEvent {
  event: string;
  message: string;
  created_at: string;
}

export interface SessionDetail {
  session_id: string;
  agent_role_id?: string;
  status:
    | "open"
    | "analyzing"
    | "pending_approval"
    | "executing"
    | "verifying"
    | "resolved"
    | "failed";
  is_smoke?: boolean;
  diagnosis_summary?: string;
  golden_summary?: SessionGoldenSummary;
  tool_plan?: ToolPlanStep[];
  attachments?: MessageAttachment[];
  alert: Record<string, unknown>;
  verification?: SessionVerification;
  notifications?: NotificationDigest[];
  executions: ExecutionDetail[];
  timeline: TimelineEvent[];
}

export interface SessionGoldenSummary {
  headline?: string;
  conclusion?: string;
  risk?: string;
  next_action?: string;
  evidence?: string[];
  notification_headline?: string;
  execution_headline?: string;
  verification_headline?: string;
}

export interface NotificationDigest {
  stage?: string;
  target?: string;
  reason?: string;
  preview?: string;
  created_at?: string;
}

export interface ListPage {
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
  q?: string;
  sort_by?: string;
  sort_order?: "asc" | "desc" | string;
}

export interface AuditTraceEntry {
  id?: string;
  resource_type: string;
  resource_id: string;
  action: string;
  actor?: string;
  trace_id?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface LogRecord {
  id: string;
  timestamp: string;
  level?: string;
  component?: string;
  message: string;
  route?: string;
  actor?: string;
  session_id?: string;
  execution_id?: string;
  trace_id?: string;
  metadata?: Record<string, unknown>;
}

export interface LogListResponse {
  items: LogRecord[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
  q?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface TraceEventRecord {
  id: string;
  timestamp: string;
  kind: string;
  component?: string;
  message: string;
  session_id?: string;
  execution_id?: string;
  trace_id?: string;
  actor?: string;
  metadata?: Record<string, unknown>;
}

export interface TraceSample {
  trace_id: string;
  session_id?: string;
  execution_id?: string;
  component?: string;
  event_count: number;
  last_event_at?: string;
  last_message?: string;
  components?: string[];
}

export interface ObservabilitySummary {
  log_entries_24h: number;
  error_entries_24h: number;
  event_entries_24h: number;
  active_traces: number;
  last_log_at?: string;
  last_event_at?: string;
}

export interface ObservabilitySignalConfig {
  retention_hours: number;
  max_size_bytes: number;
  current_bytes: number;
  file_path?: string;
}

export interface OTLPStatus {
  endpoint?: string;
  protocol?: string;
  insecure: boolean;
  metrics_enabled: boolean;
  logs_enabled: boolean;
  traces_enabled: boolean;
}

export interface ObservabilityRetentionStatus {
  data_dir?: string;
  metrics: ObservabilitySignalConfig;
  logs: ObservabilitySignalConfig;
  traces: ObservabilitySignalConfig;
  otlp: OTLPStatus;
  exporters?: string[];
}

export interface ObservabilityResponse {
  summary: ObservabilitySummary;
  metrics_endpoint: string;
  retention: ObservabilityRetentionStatus;
  health: DashboardHealthResponse;
  recent_logs?: LogRecord[];
  recent_events?: TraceEventRecord[];
  trace_samples?: TraceSample[];
}

export interface SessionKnowledgeTrace {
  document_id: string;
  title: string;
  summary?: string;
  content_preview?: string;
  conversation?: string[];
  runtime?: RuntimeMetadata;
  updated_at?: string;
}

export interface SessionTraceResponse {
  session_id: string;
  audit_entries: AuditTraceEntry[];
  knowledge?: SessionKnowledgeTrace;
}

export interface AuditListResponse {
  items: AuditTraceEntry[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
  q?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface KnowledgeRecord {
  document_id: string;
  session_id: string;
  title: string;
  summary?: string;
  updated_at?: string;
}

export interface KnowledgeListResponse {
  items: KnowledgeRecord[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
  q?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface ExecutionDetail {
  execution_id: string;
  session_id?: string;
  agent_role_id?: string;
  request_kind?: "execution" | "capability";
  status:
    | "pending"
    | "approved"
    | "executing"
    | "completed"
    | "failed"
    | "timeout"
    | "rejected";
  risk_level?: "info" | "warning" | "critical";
  golden_summary?: ExecutionGoldenSummary;
  command?: string;
  target_host?: string;
  step_id?: string;
  capability_id?: string;
  capability_params?: Record<string, unknown>;
  connector_id?: string;
  connector_type?: string;
  connector_vendor?: string;
  protocol?: string;
  execution_mode?: string;
  requested_by?: string;
  approval_group?: string;
  runtime?: RuntimeMetadata;
  exit_code?: number;
  output_ref?: string;
  output_bytes?: number;
  output_truncated?: boolean;
  created_at?: string;
  approved_at?: string;
  completed_at?: string;
}

export interface ExecutionActionRequest {
  command?: string;
  reason?: string;
}

export interface ChannelAction {
  label: string;
  value: string;
}

export interface ExecutionGoldenSummary {
  headline?: string;
  risk?: string;
  approval?: string;
  result?: string;
  next_action?: string;
  command_preview?: string;
}

export interface OutboxEvent {
  id: string;
  topic: string;
  status: "pending" | "processing" | "failed" | "blocked" | "delivered";
  aggregate_id: string;
  retry_count: number;
  last_error?: string;
  blocked_reason?: string;
  created_at: string;
}

export interface SessionListResponse {
  items: SessionDetail[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
  q?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface OutboxListResponse {
  items: OutboxEvent[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
  q?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface ExecutionListResponse {
  items: ExecutionDetail[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
  q?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface AcceptedResponse {
  accepted: boolean;
}

export interface BatchOperationResult {
  id: string;
  success: boolean;
  code?: string;
  message?: string;
}

export interface BatchOperationResponse {
  operation: string;
  resource_type: string;
  total: number;
  succeeded: number;
  failed: number;
  results: BatchOperationResult[];
}

export interface ExportFailure {
  id: string;
  success: boolean;
  code?: string;
  message?: string;
}

export interface SessionExportResponse {
  resource_type: string;
  exported_at: string;
  operator_reason: string;
  total_requested: number;
  exported_count: number;
  failed_count: number;
  items: SessionDetail[];
  failures?: ExportFailure[];
}

export interface ExecutionExportResponse {
  resource_type: string;
  exported_at: string;
  operator_reason: string;
  total_requested: number;
  exported_count: number;
  failed_count: number;
  items: ExecutionDetail[];
  failures?: ExportFailure[];
}

export interface AuditExportResponse {
  resource_type: string;
  exported_at: string;
  operator_reason: string;
  total_requested: number;
  exported_count: number;
  failed_count: number;
  items: AuditTraceEntry[];
  failures?: ExportFailure[];
}

export interface KnowledgeExportResponse {
  resource_type: string;
  exported_at: string;
  operator_reason: string;
  total_requested: number;
  exported_count: number;
  failed_count: number;
  items: KnowledgeRecord[];
  failures?: ExportFailure[];
}

export interface ExecutionOutputChunk {
  seq: number;
  stream_type: string;
  content: string;
  byte_size: number;
  created_at: string;
}

export interface ExecutionOutputResponse {
  execution_id: string;
  chunks: ExecutionOutputChunk[];
}

export interface ApiErrorEnvelope {
  error?: {
    code?: string;
    message?: string;
  };
}

export interface OpsSummaryResponse {
  active_sessions: number;
  pending_approvals: number;
  executions_total: number;
  executions_completed: number;
  execution_success_rate: number;
  blocked_outbox: number;
  failed_outbox: number;
  visible_outbox: number;
  healthy_connectors: number;
  degraded_connectors: number;
  configured_secrets: number;
  missing_secrets: number;
  provider_failures: number;
  active_alerts: number;
}

export interface ComponentRuntimeStatus {
  last_result?: string;
  last_detail?: string;
  last_changed_at?: string;
  last_success_at?: string;
  last_error?: string;
  last_error_at?: string;
}

export interface RuntimeMetadata {
  runtime?: string;
  selection?: string;
  connector_id?: string;
  connector_type?: string;
  connector_vendor?: string;
  protocol?: string;
  execution_mode?: string;
  runtime_state?: string;
  fallback_enabled?: boolean;
  fallback_used?: boolean;
  fallback_reason?: string;
  fallback_target?: string;
}

export interface ToolPlanStep {
  id?: string;
  tool: string;
  connector_id?: string;
  reason?: string;
  priority?: number;
  status?: string;
  input?: Record<string, unknown>;
  resolved_input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  on_failure?: string;
  on_pending_approval?: string;
  on_denied?: string;
  runtime?: RuntimeMetadata;
  started_at?: string;
  completed_at?: string;
}

export interface MessageAttachment {
  type: "image" | "file" | string;
  name?: string;
  mime_type?: string;
  url?: string;
  content?: string;
  preview_text?: string;
  metadata?: Record<string, unknown>;
}

export interface RuntimeSetupStatus {
  name?: string;
  primary?: RuntimeMetadata;
  fallback?: RuntimeMetadata;
  component?: string;
  capability_tool?: string;
  component_runtime: ComponentRuntimeStatus;
}

export interface SetupFeatures {
  diagnosis_enabled: boolean;
  approval_enabled: boolean;
  execution_enabled: boolean;
  knowledge_ingest_enabled: boolean;
}

export interface TelegramSetupStatus extends ComponentRuntimeStatus {
  configured: boolean;
  polling: boolean;
  base_url?: string;
  mode: "polling" | "webhook" | "disabled";
}

export interface ProviderSetupStatus extends ComponentRuntimeStatus {
  configured: boolean;
  provider_id?: string;
  vendor?: string;
  protocol?: string;
  base_url?: string;
  model_name?: string;
}

export interface ProvidersSetupStatus {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  primary_provider_id?: string;
  assist_provider_id?: string;
}

export interface ConnectorsSetupStatus {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  total_entries: number;
  enabled_entries: number;
  kinds?: string[];
  metrics_runtime?: RuntimeSetupStatus;
  execution_runtime?: RuntimeSetupStatus;
  verification_runtime?: RuntimeSetupStatus;
  observability_runtime?: RuntimeSetupStatus;
  delivery_runtime?: RuntimeSetupStatus;
}

export interface LegacyFallbacksSetupStatus {
  metrics?: ProviderSetupStatus;
  execution?: SSHSetupStatus;
  verification?: SSHSetupStatus;
}

export interface SmokeDefaultsSetupStatus {
  hosts?: string[];
}

export interface SSHSetupStatus extends ComponentRuntimeStatus {
  configured: boolean;
  user?: string;
  allowed_hosts?: string[];
  allowed_hosts_count: number;
  private_key_configured: boolean;
  private_key_exists: boolean;
  host_key_checking_disabled: boolean;
  service_command_allowlist_set: boolean;
}

export interface AuthorizationSetupStatus {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
}

export interface ApprovalRoutingSetupStatus {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
}

export interface ReasoningPromptSetupStatus {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  local_command_fallback_enabled: boolean;
}

export interface DesensitizationSetupStatus {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  enabled: boolean;
  local_llm_assist_enabled: boolean;
  local_llm_base_url?: string;
  local_llm_model?: string;
  local_llm_mode?: string;
}

export interface AuthorizationOverrideConfig {
  id?: string;
  services?: string[];
  hosts?: string[];
  channels?: string[];
  command_globs?: string[];
  action: "direct_execute" | "require_approval" | "suggest_only" | "deny";
}

export interface AuthorizationPolicyConfig {
  whitelist_action:
    | "direct_execute"
    | "require_approval"
    | "suggest_only"
    | "deny";
  blacklist_action:
    | "direct_execute"
    | "require_approval"
    | "suggest_only"
    | "deny";
  unmatched_action:
    | "direct_execute"
    | "require_approval"
    | "suggest_only"
    | "deny";
  normalize_whitespace: boolean;
  hard_deny_ssh_command?: string[];
  hard_deny_mcp_skill?: string[];
  whitelist?: string[];
  blacklist?: string[];
  overrides?: AuthorizationOverrideConfig[];
}

export interface RouteEntry {
  key: string;
  targets: string[];
}

export interface ApprovalRoutingConfig {
  prohibit_self_approval: boolean;
  service_owners?: RouteEntry[];
  oncall_groups?: RouteEntry[];
  command_allowlist?: RouteEntry[];
}

export interface ReasoningPromptConfig {
  system_prompt: string;
  user_prompt_template: string;
}

export interface DesensitizationSecretConfig {
  key_names?: string[];
  query_key_names?: string[];
  additional_patterns?: string[];
  redact_bearer: boolean;
  redact_basic_auth_url: boolean;
  redact_sk_tokens: boolean;
}

export interface DesensitizationPlaceholderConfig {
  host_key_fragments?: string[];
  path_key_fragments?: string[];
  replace_inline_ip: boolean;
  replace_inline_host: boolean;
  replace_inline_path: boolean;
}

export interface DesensitizationRehydrationConfig {
  host: boolean;
  ip: boolean;
  path: boolean;
}

export interface LocalLLMAssistConfig {
  enabled: boolean;
  provider?: string;
  base_url?: string;
  model?: string;
  mode?: string;
}

export interface DesensitizationConfig {
  enabled: boolean;
  secrets: DesensitizationSecretConfig;
  placeholders: DesensitizationPlaceholderConfig;
  rehydration: DesensitizationRehydrationConfig;
  local_llm_assist: LocalLLMAssistConfig;
}

export interface SmokeSessionStatus {
  session_id: string;
  status: SessionDetail["status"];
  alertname?: string;
  service?: string;
  host?: string;
  telegram_target?: string;
  approval_requested: boolean;
  execution_status?: ExecutionDetail["status"];
  verification_status?: string;
  updated_at?: string;
}

export interface SetupStatusResponse {
  rollout_mode: string;
  features: SetupFeatures;
  initialization: SetupInitializationStatus;
  telegram: TelegramSetupStatus;
  model: ProviderSetupStatus;
  assist_model: ProviderSetupStatus;
  providers: ProvidersSetupStatus;
  connectors: ConnectorsSetupStatus;
  legacy_fallbacks?: LegacyFallbacksSetupStatus | null;
  smoke_defaults: SmokeDefaultsSetupStatus;
  authorization: AuthorizationSetupStatus;
  approval: ApprovalRoutingSetupStatus;
  reasoning: ReasoningPromptSetupStatus;
  desensitization: DesensitizationSetupStatus;
  latest_smoke?: SmokeSessionStatus;
}

export interface BootstrapStatusResponse {
  initialized: boolean;
  mode: "wizard" | "runtime" | string;
  next_step?: string;
}

export interface SetupInitializationStatus {
  initialized: boolean;
  mode: "wizard" | "runtime" | string;
  current_step?: string;
  admin_configured: boolean;
  auth_configured: boolean;
  provider_ready: boolean;
  channel_ready: boolean;
  provider_checked?: boolean;
  provider_check_ok?: boolean;
  provider_check_note?: string;
  admin_user_id?: string;
  auth_provider_id?: string;
  primary_provider_id?: string;
  primary_model?: string;
  default_channel_id?: string;
  login_hint?: SetupLoginHint;
  completed_at?: string;
  updated_at?: string;
  next_step?: string;
  required_steps?: string[];
  completed_steps?: string[];
}

export interface SetupLoginHint {
  username?: string;
  provider?: string;
  login_url?: string;
}

export interface SetupWizardAdmin {
  user: AccessUser;
}

export interface SetupWizardAuth {
  provider: AuthProviderInfo;
  supported_types?: string[];
  recommended_type?: string;
}

export interface SetupWizardProvider {
  provider: ProviderRegistryEntry;
}

export interface SetupWizardChannel {
  channel: AccessChannel;
}

export interface SetupWizardResponse {
  initialization: SetupInitializationStatus;
  admin: SetupWizardAdmin;
  auth: SetupWizardAuth;
  provider: SetupWizardProvider;
  channel: SetupWizardChannel;
}

export interface SmokeAlertResponse {
  accepted: boolean;
  session_id: string;
  status: SessionDetail["status"];
  duplicated?: boolean;
  tg_target?: string;
}

export interface AuthorizationConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: AuthorizationPolicyConfig;
}

export interface ApprovalRoutingConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: ApprovalRoutingConfig;
}

export interface ReasoningPromptConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: ReasoningPromptConfig;
}

export interface DesensitizationConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: DesensitizationConfig;
}

export interface ProviderBinding {
  provider_id?: string;
  model?: string;
}

export interface ProviderBindings {
  primary: ProviderBinding;
  assist: ProviderBinding;
}

export interface ProviderBindingsResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  bindings: ProviderBindings;
}

export interface ProviderBindingsUpdateRequest {
  bindings: ProviderBindings;
  operator_reason: string;
}

export interface ProviderEntry {
  id: string;
  vendor?: string;
  protocol?: string;
  base_url?: string;
  api_key?: string;
  api_key_ref?: string;
  api_key_set: boolean;
  org_id?: string;
  tenant_id?: string;
  workspace_id?: string;
  enabled: boolean;
  templates?: ProviderTemplate[];
}

export interface ProviderTemplate {
  id?: string;
  name?: string;
  description?: string;
  values?: Record<string, string>;
  created_at?: string;
}

export interface ProvidersConfig {
  primary: ProviderBinding;
  assist: ProviderBinding;
  entries?: ProviderEntry[];
}

export interface ProvidersConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: ProvidersConfig;
}

export interface ConnectorCapability {
  id?: string;
  action?: string;
  read_only: boolean;
  invocable?: boolean;
  scopes?: string[];
  description?: string;
}

export interface ConnectorInvokeCapabilityRequest {
  capability_id: string;
  params?: Record<string, unknown>;
  session_id?: string;
  caller?: string;
}

export interface ConnectorInvokeCapabilityResponse {
  connector_id: string;
  capability_id: string;
  status: string;
  output?: Record<string, unknown>;
  artifacts?: MessageAttachment[];
  metadata?: Record<string, unknown>;
  error?: string;
  runtime?: RuntimeMetadata;
}

export interface ConnectorField {
  key?: string;
  label?: string;
  type?: string;
  required: boolean;
  secret?: boolean;
  default?: string;
  options?: string[];
  description?: string;
}

export interface ConnectorImportExport {
  exportable: boolean;
  importable: boolean;
  formats?: string[];
}

export interface ConnectorCompatibility {
  tars_major_versions?: string[];
  upstream_major_versions?: string[];
  modes?: string[];
}

export interface ConnectorMarketplace {
  category?: string;
  tags?: string[];
  source?: string;
}

export interface ConnectorMetadata {
  id?: string;
  name?: string;
  display_name?: string;
  vendor?: string;
  version?: string;
  description?: string;
  org_id?: string;
  tenant_id?: string;
  workspace_id?: string;
}

export interface ConnectorSpec {
  type?: string;
  protocol?: string;
  capabilities?: ConnectorCapability[];
  connection_form?: ConnectorField[];
  import_export: ConnectorImportExport;
}

export interface ConnectorRuntimeConfig {
  values?: Record<string, string>;
  secret_refs?: Record<string, string>;
}

export interface ConnectorRuntimeMetadata {
  type?: string;
  protocol?: string;
  vendor?: string;
  mode?: string;
  state?: string;
}

export interface ConnectorCompatibilityReport {
  compatible: boolean;
  current_tars_major?: string;
  reasons?: string[];
  checked_at?: string;
}

export interface ConnectorHealthStatus {
  status?: string;
  credential_status?: string;
  summary?: string;
  checked_at?: string;
  runtime_state?: string;
}

export interface ConnectorLifecycleEvent {
  type?: string;
  summary?: string;
  version?: string;
  from_version?: string;
  to_version?: string;
  enabled?: boolean;
  metadata?: Record<string, string>;
  created_at?: string;
}

export interface ConnectorRevision {
  version?: string;
  created_at?: string;
  reason?: string;
}

export interface ConnectorLifecycle {
  connector_id?: string;
  display_name?: string;
  current_version?: string;
  available_version?: string;
  current_status?: string;
  enabled: boolean;
  installed_at?: string;
  updated_at?: string;
  runtime: ConnectorRuntimeMetadata;
  compatibility: ConnectorCompatibilityReport;
  health: ConnectorHealthStatus;
  history?: ConnectorLifecycleEvent[];
  health_history?: ConnectorHealthStatus[];
  revisions?: ConnectorRevision[];
}

export interface ConnectorManifest {
  api_version?: string;
  kind?: string;
  enabled?: boolean;
  metadata: ConnectorMetadata;
  spec: ConnectorSpec;
  config?: ConnectorRuntimeConfig;
  compatibility: ConnectorCompatibility;
  marketplace: ConnectorMarketplace;
  lifecycle?: ConnectorLifecycle;
}

export interface ConnectorUpsertRequest {
  manifest: ConnectorManifest;
  operator_reason: string;
}

export interface ConnectorsConfig {
  entries?: ConnectorManifest[];
}

export interface ConnectorsConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: ConnectorsConfig;
}

export interface ConnectorListResponse extends ListPage {
  items: ConnectorManifest[];
}

export interface SkillRevision {
  created_at?: string;
  reason?: string;
  action?: string;
}

export interface SkillLifecycleEvent {
  type?: string;
  summary?: string;
  metadata?: Record<string, string>;
  created_at?: string;
}

export interface SkillLifecycle {
  skill_id?: string;
  display_name?: string;
  source?: string;
  current_status?: string;
  status?: string;
  review_state?: string;
  runtime_mode?: string;
  enabled: boolean;
  installed_at?: string;
  updated_at?: string;
  published_at?: string;
  history?: SkillLifecycleEvent[];
  revisions?: SkillRevision[];
}

export interface SkillMetadata {
  id: string;
  name: string;
  display_name: string;
  vendor?: string;
  description?: string;
  source?: string;
  /** Multi-value labels for search & pre-filtering. Replaces category + alerts + intents. */
  tags?: string[];
  /** SKILL.md markdown body (instructions). */
  content?: string;
  org_id?: string;
  tenant_id?: string;
  workspace_id?: string;
}

export interface SkillConnectorPreference {
  metrics?: string[];
  execution?: string[];
  observability?: string[];
  delivery?: string[];
}

export interface SkillGovernance {
  execution_policy?: string;
  read_only_first?: boolean;
  connector_preference?: SkillConnectorPreference;
}

export interface SkillSpec {
  governance?: SkillGovernance;
}

/** An entry in the files[] array for directory-based skills. */
export interface SkillFile {
  path: string;
  type: "file" | "directory";
  size?: number;
}

export interface SkillManifest {
  api_version?: string;
  kind?: string;
  enabled: boolean;
  metadata: SkillMetadata;
  spec: SkillSpec;
  compatibility?: Record<string, unknown>;
  lifecycle?: SkillLifecycle;
  /** Files in the skill directory (populated by backend for directory-based skills). */
  files?: SkillFile[];
}

export interface SkillListResponse extends ListPage {
  items: SkillManifest[];
}

export interface SkillsConfig {
  entries?: SkillManifest[];
}

export interface SkillsConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: SkillsConfig;
}

export interface SkillImportResponse {
  manifest: SkillManifest;
  state: SkillLifecycle;
}

export interface AutomationSkillTarget {
  skill_id: string;
  context?: Record<string, unknown>;
}

export interface AutomationConnectorCapability {
  connector_id: string;
  capability_id: string;
  params?: Record<string, unknown>;
}

export interface AutomationRun {
  run_id: string;
  job_id: string;
  trigger: string;
  status: string;
  started_at?: string;
  completed_at?: string;
  attempt_count?: number;
  summary?: string;
  error?: string;
  metadata?: Record<string, unknown>;
}

export interface AutomationJobState {
  status?: string;
  last_run_at?: string;
  next_run_at?: string;
  last_outcome?: string;
  last_error?: string;
  consecutive_failures?: number;
  updated_at?: string;
  runs?: AutomationRun[];
}

export interface AutomationJob {
  id: string;
  display_name: string;
  description?: string;
  agent_role_id?: string;
  governance_policy?: string;
  type: "skill" | "connector_capability" | string;
  target_ref: string;
  schedule: string;
  enabled: boolean;
  owner?: string;
  runtime_mode?: string;
  timeout_seconds?: number;
  retry_max_attempts?: number;
  retry_initial_backoff?: string;
  labels?: Record<string, string>;
  skill?: AutomationSkillTarget;
  connector_capability?: AutomationConnectorCapability;
  state?: AutomationJobState;
  last_run?: AutomationRun;
}

export interface AutomationListResponse extends ListPage {
  items: AutomationJob[];
}

export interface ExtensionDocAsset {
  id?: string;
  slug?: string;
  title?: string;
  format?: string;
  summary?: string;
  content?: string;
}

export interface ExtensionTestSpec {
  id?: string;
  name?: string;
  kind?: string;
  command?: string;
}

export interface ExtensionBundleMetadata {
  id?: string;
  display_name?: string;
  summary?: string;
  source?: string;
  generated_by?: string;
  created_at?: string;
}

export interface ExtensionBundle {
  api_version?: string;
  kind?: string;
  metadata: ExtensionBundleMetadata;
  skill: SkillManifest;
  docs?: ExtensionDocAsset[];
  tests?: ExtensionTestSpec[];
  compatibility?: Record<string, unknown>;
}

export interface ExtensionValidationReport {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
  checked_at?: string;
}

export interface ExtensionPreviewSummary {
  change_type?: string;
  existing_version?: string;
  proposed_version?: string;
  summary?: string[];
}

export interface ExtensionReviewEvent {
  state?: string;
  reason?: string;
  created_at?: string;
  imported_by?: string;
}

export interface ExtensionCandidate {
  id: string;
  status?: string;
  review_state?: string;
  review_history?: ExtensionReviewEvent[];
  imported_skill_id?: string;
  imported_at?: string;
  created_at?: string;
  updated_at?: string;
  bundle: ExtensionBundle;
  validation: ExtensionValidationReport;
  preview: ExtensionPreviewSummary;
}

export interface ExtensionListResponse extends ListPage {
  items: ExtensionCandidate[];
}

export interface ExtensionValidationResponse {
  bundle: ExtensionBundle;
  validation: ExtensionValidationReport;
  preview: ExtensionPreviewSummary;
}

export interface ExtensionImportResponse {
  candidate: ExtensionCandidate;
  manifest: SkillManifest;
  state: SkillLifecycle;
}

export interface ConnectorMetricsQueryResponse {
  connector_id: string;
  protocol: string;
  service?: string;
  host?: string;
  series: Array<Record<string, unknown>>;
  runtime?: RuntimeMetadata;
}

export interface PlatformDiscoveryResponse {
  product_name: string;
  api_base_path: string;
  api_version: string;
  manifest_version: string;
  skill_manifest_version?: string;
  marketplace_package_version: string;
  integration_modes: string[];
  connector_kinds: string[];
  registered_connectors_count: number;
  registered_connector_ids?: string[];
  registered_connector_kinds?: string[];
  supported_provider_protocols: string[];
  supported_provider_vendors: string[];
  import_export_formats: string[];
  tool_plan_capabilities?: ToolPlanCapabilityDescriptor[];
  docs: string[];
}

export interface ToolPlanCapabilityDescriptor {
  tool?: string;
  connector_id?: string;
  connector_type?: string;
  connector_vendor?: string;
  protocol?: string;
  capability_id?: string;
  action?: string;
  scopes?: string[];
  read_only: boolean;
  invocable: boolean;
  source?: string;
  description?: string;
}

export interface ProviderModelInfo {
  id: string;
  name?: string;
}

export interface ProviderListModelsResponse {
  provider_id: string;
  models: ProviderModelInfo[];
}

export interface ProviderCheckResponse {
  provider_id: string;
  available: boolean;
  detail?: string;
}

export interface SecretValueInput {
  ref: string;
  value: string;
}

export interface SecretDescriptor {
  ref?: string;
  owner_type?: string;
  owner_id?: string;
  key?: string;
  set: boolean;
  updated_at?: string;
  source?: string;
}

export interface SecretsInventoryResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  items: SecretDescriptor[];
}

export interface SSHCredential {
  credential_id: string;
  display_name?: string;
  owner_type?: string;
  owner_id?: string;
  connector_id?: string;
  username?: string;
  auth_type: 'password' | 'private_key' | string;
  host_scope?: string;
  status: string;
  created_by?: string;
  updated_by?: string;
  created_at?: string;
  updated_at?: string;
  last_rotated_at?: string;
  expires_at?: string;
}

export interface SSHCredentialListResponse {
  configured: boolean;
  items: SSHCredential[];
}

export interface SSHCredentialUpsertRequest {
  credential_id?: string;
  display_name?: string;
  owner_type?: string;
  owner_id?: string;
  connector_id?: string;
  username?: string;
  auth_type?: 'password' | 'private_key' | string;
  password?: string;
  private_key?: string;
  passphrase?: string;
  host_scope?: string;
  expires_at?: string;
  operator_reason: string;
}

export interface ConnectorTemplate {
  connector_id?: string;
  template_id?: string;
  name?: string;
  description?: string;
  values?: Record<string, string>;
  created_at?: string;
}

export interface ConnectorTemplateListResponse {
  items: ConnectorTemplate[];
}

export interface ConnectorExecutionResponse {
  connector_id: string;
  execution_id: string;
  session_id?: string;
  status: string;
  protocol?: string;
  execution_mode?: string;
  runtime?: RuntimeMetadata;
  target_host?: string;
  command?: string;
  exit_code?: number;
  output_ref?: string;
  output_bytes?: number;
  output_truncated?: boolean;
  output_preview?: string;
}

export interface DashboardConnectorHealth {
  connector_id: string;
  display_name: string;
  protocol: string;
  type: string;
  vendor: string;
  status: string;
  credential_status: string;
  summary: string;
  checked_at: string;
  current_version: string;
}

export interface DashboardProviderHealth {
  provider_id?: string;
  vendor?: string;
  protocol?: string;
  base_url?: string;
  enabled: boolean;
  last_result?: string;
  last_detail?: string;
  last_error?: string;
  last_changed_at?: string;
}

export interface DashboardAlertItem {
  severity?: string;
  resource?: string;
  title?: string;
  summary?: string;
  created_at?: string;
}

export interface DashboardHealthSummary {
  healthy_connectors: number;
  degraded_connectors: number;
  disabled_connectors: number;
  configured_secrets: number;
  missing_secrets: number;
  active_alerts: number;
  provider_failures: number;
}

export interface DashboardResourceSample {
  label?: string;
  value: number;
  unit?: string;
}

export interface DashboardRuntimeResources {
  uptime_seconds: number;
  goroutines: number;
  heap_alloc_bytes: number;
  heap_sys_bytes: number;
  heap_in_use_bytes: number;
  stack_in_use_bytes: number;
  gc_count: number;
  last_gc_pause_seconds: number;
  disk_used_bytes: number;
  disk_free_bytes: number;
  disk_usage_percent: number;
  cpu_count: number;
  load_average?: DashboardResourceSample[];
  network_interfaces?: DashboardResourceSample[];
  tracing_enabled: boolean;
  tracing_provider?: string;
  log_level?: string;
  spool_dir?: string;
}

export interface DashboardHealthResponse {
  summary: DashboardHealthSummary;
  resources: DashboardRuntimeResources;
  connectors: DashboardConnectorHealth[];
  providers: DashboardProviderHealth[];
  secrets: SecretDescriptor[];
  alerts: DashboardAlertItem[];
}

export interface IdentityLink {
  provider_type?: string;
  provider_id?: string;
  external_subject?: string;
  external_username?: string;
  external_email?: string;
}

export interface AccessUser {
  user_id?: string;
  username?: string;
  display_name?: string;
  email?: string;
  status?: string;
  source?: string;
  password_hash?: string;
  password_login_enabled?: boolean;
  password_updated_at?: string;
  challenge_required?: boolean;
  mfa_enabled?: boolean;
  mfa_method?: string;
  totp_secret?: string;
  roles?: string[];
  groups?: string[];
  identities?: IdentityLink[];
  created_at?: string;
  updated_at?: string;
}

export interface AccessGroup {
  group_id?: string;
  display_name?: string;
  description?: string;
  status?: string;
  roles?: string[];
  members?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface AccessRole {
  id?: string;
  display_name?: string;
  permissions?: string[];
}

export interface AccessPerson {
  id?: string;
  display_name?: string;
  email?: string;
  status?: string;
  linked_user_id?: string;
  channel_ids?: string[];
  team?: string;
  approval_target?: string;
  oncall_schedule?: string;
  preferences?: Record<string, string>;
  created_at?: string;
  updated_at?: string;
}

export interface AccessChannel {
  id?: string;
  kind?: string;
  type?: string;
  name?: string;
  target?: string;
  enabled: boolean;
  linked_users?: string[];
  usages?: string[];
  capabilities?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface AuthProviderInfo {
  id?: string;
  type?: string;
  name?: string;
  enabled: boolean;
  password_min_length?: number;
  require_challenge?: boolean;
  challenge_channel?: string;
  challenge_ttl_seconds?: number;
  challenge_code_length?: number;
  require_mfa?: boolean;
  login_url?: string;
  issuer_url?: string;
  client_id?: string;
  client_secret?: string;
  client_secret_set: boolean;
  client_secret_ref?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  session_ttl_seconds?: number;
  ldap_url?: string;
  bind_dn?: string;
  bind_password?: string;
  bind_password_set: boolean;
  bind_password_ref?: string;
  base_dn?: string;
  user_search_filter?: string;
  group_search_filter?: string;
  redirect_path?: string;
  success_redirect?: string;
  user_id_field?: string;
  username_field?: string;
  display_name_field?: string;
  email_field?: string;
  allowed_domains?: string[];
  scopes?: string[];
  default_roles?: string[];
  allow_jit: boolean;
}

export interface AuthProviderListResponse {
  items: AuthProviderInfo[];
}

export interface AuthLoginResponse {
  session_token?: string;
  redirect_url?: string;
  provider_id?: string;
  pending_token?: string;
  next_step?: string;
  challenge_id?: string;
  challenge_channel?: string;
  challenge_code?: string;
  challenge_expires_at?: string;
  user: AccessUser;
  roles?: string[];
  permissions?: string[];
}

export interface AuthChallengeResponse {
  provider_id?: string;
  pending_token?: string;
  next_step?: string;
  challenge_id?: string;
  challenge_channel?: string;
  challenge_code?: string;
  challenge_expires_at?: string;
}

export interface MeResponse {
  user: AccessUser;
  roles?: string[];
  permissions?: string[];
  auth_source?: string;
  break_glass: boolean;
  session_token?: string;
  session_expires_at?: string;
}

export interface SessionInventoryItem {
  token_masked?: string;
  user_id?: string;
  provider_id?: string;
  created_at?: string;
  expires_at?: string;
  last_seen_at?: string;
}

export interface SessionInventoryResponse {
  items: SessionInventoryItem[];
}

export interface UserListResponse extends ListPage {
  items: AccessUser[];
}

export interface GroupListResponse extends ListPage {
  items: AccessGroup[];
}

export interface RoleListResponse {
  items: AccessRole[];
}

export interface RoleBindingRequest {
  user_ids?: string[];
  group_ids?: string[];
  operator_reason: string;
}

export interface RoleBindingsResponse {
  role_id: string;
  user_ids: string[];
  group_ids: string[];
}

export interface PersonListResponse extends ListPage {
  items: AccessPerson[];
}

export interface ChannelListResponse extends ListPage {
  items: AccessChannel[];
}

export interface MsgTemplateContent {
  subject: string;
  body: string;
}

export interface MsgTemplate {
  id: string;
  kind: "diagnosis" | "approval" | "execution_result";
  locale: "zh-CN" | "en-US";
  name: string;
  status?: string;
  enabled: boolean;
  variable_schema?: Record<string, string>;
  usage_refs?: string[];
  content: MsgTemplateContent;
  updated_at?: string;
}

export interface MsgTemplateListResponse extends ListPage {
  items: MsgTemplate[];
}

export interface ProviderRegistryEntry {
  id?: string;
  vendor?: string;
  protocol?: string;
  base_url?: string;
  api_key?: string;
  api_key_ref?: string;
  api_key_set: boolean;
  enabled: boolean;
  org_id?: string;
  tenant_id?: string;
  workspace_id?: string;
  primary_model?: string;
  assist_model?: string;
  templates?: ProviderTemplate[];
}

export interface ProviderRegistryListResponse extends ListPage {
  items: ProviderRegistryEntry[];
}

export interface AccessConfig {
  users?: AccessUser[];
  groups?: AccessGroup[];
  auth_providers?: AuthProviderInfo[];
  roles?: AccessRole[];
  people?: AccessPerson[];
  channels?: AccessChannel[];
}

export interface AccessConfigResponse {
  configured: boolean;
  loaded: boolean;
  path?: string;
  updated_at?: string;
  content?: string;
  config: AccessConfig;
}

export interface AuthUserSummary {
  id: string;
  username: string;
  displayName: string;
  email: string;
  roles: string[];
  permissions: string[];
  authSource: string;
  breakGlass: boolean;
}

export interface AuthSession {
  token: string;
  user: AuthUserSummary;
  roles: string[];
  permissions: string[];
  authSource: string;
  breakGlass: boolean;
}

// ---------------------------------------------------------------------------
// Organization / Tenant / Workspace (ORG-N1..N5)
// ---------------------------------------------------------------------------

export interface OrgItem {
  id: string;
  name: string;
  slug?: string;
  status?: string;
  description?: string;
  domain?: string;
  created_at?: string;
  updated_at?: string;
}
export interface OrgListResponse {
  items: OrgItem[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
}

export interface TenantItem {
  id: string;
  org_id?: string;
  name: string;
  slug?: string;
  status?: string;
  description?: string;
  default_locale?: string;
  default_timezone?: string;
  created_at?: string;
  updated_at?: string;
}
export interface TenantListResponse {
  items: TenantItem[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
}

export interface WorkspaceItem {
  id: string;
  org_id?: string;
  tenant_id?: string;
  name: string;
  slug?: string;
  status?: string;
  description?: string;
  created_at?: string;
  updated_at?: string;
}
export interface WorkspaceListResponse {
  items: WorkspaceItem[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
}

export interface OrgContextResponse {
  default_org: OrgItem;
  default_tenant: TenantItem;
  default_workspace: WorkspaceItem;
}

export interface OrgPolicy {
  org_id?: string;
  allowed_auth_methods?: string[];
  require_mfa?: boolean;
  max_session_duration_s?: number;
  idle_session_timeout_s?: number;
  require_approval_for_execution?: boolean;
  prohibit_self_approval?: boolean;
  default_jit_roles?: string[];
  skill_allowlist?: string[];
  skill_blocklist?: string[];
  audit_retention_days?: number;
  knowledge_retention_days?: number;
  updated_at?: string;
}
export interface TenantPolicy {
  tenant_id: string;
  org_id?: string;
  allowed_auth_methods?: string[];
  require_mfa?: boolean;
  require_approval_for_execution?: boolean;
  prohibit_self_approval?: boolean;
  default_jit_roles?: string[];
  skill_allowlist?: string[];
  skill_blocklist?: string[];
  updated_at?: string;
}
export interface ResolvedPolicy {
  org_id: string;
  tenant_id: string;
  allowed_auth_methods?: string[];
  require_mfa: boolean;
  require_approval_for_execution: boolean;
  prohibit_self_approval: boolean;
  default_jit_roles?: string[];
  skill_allowlist?: string[];
  skill_blocklist?: string[];
  audit_retention_days?: number;
  knowledge_retention_days?: number;
}

// ---------------------------------------------------------------------------
// Inbox / Station Messages (MODULE C)
// ---------------------------------------------------------------------------

export interface InboxMessage {
  id: string;
  tenant_id: string;
  subject: string;
  body: string;
  channel: string;
  ref_type: string;
  ref_id: string;
  source: string;
  actions?: ChannelAction[];
  is_read: boolean;
  created_at: string;
  read_at?: string;
}

export interface InboxListResponse {
  items: InboxMessage[];
  unread_count: number;
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
}

// ---------------------------------------------------------------------------
// Triggers (MODULE B)
// ---------------------------------------------------------------------------

export interface TriggerDTO {
  id: string;
  tenant_id: string;
  display_name: string;
  description: string;
  enabled: boolean;
  event_type: string;
  channel_id?: string;
  automation_job_id?: string;
  channel: string;
  governance?: string;
  filter_expr?: string;
  target_audience?: string;
  template_id: string;
  cooldown_sec: number;
  last_fired_at?: string;
  created_at: string;
  updated_at: string;
}

export interface TriggerListResponse {
  items: TriggerDTO[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
}

export interface TriggerUpsertRequest {
  id?: string;
  tenant_id?: string;
  display_name: string;
  description?: string;
  enabled?: boolean;
  event_type: string;
  channel?: string;
  channel_id?: string;
  automation_job_id?: string;
  governance?: string;
  filter_expr?: string;
  target_audience?: string;
  template_id?: string;
  cooldown_sec?: number;
  trigger?: Partial<TriggerDTO>;
  operator_reason?: string;
}

// ─── Agent Roles ───────────────────────────────────────────────────────────────

export interface AgentRoleProfile {
  system_prompt: string;
  persona_tags?: string[];
}

export interface AgentRoleCapabilityBinding {
  allowed_connector_capabilities?: string[];
  denied_connector_capabilities?: string[];
  allowed_skills?: string[];
  allowed_skill_tags?: string[];
  mode: string;
}

export interface AgentRolePolicyBinding {
  max_risk_level: string;
  max_action: string;
  require_approval_for?: string[];
  hard_deny?: string[];
}

export interface AgentRoleModelBinding {
  primary?: AgentRoleModelTargetBinding;
  fallback?: AgentRoleModelTargetBinding;
  inherit_platform_default?: boolean;
}

export interface AgentRoleModelTargetBinding {
  provider_id?: string;
  model?: string;
}

export interface AgentRole {
  role_id: string;
  display_name: string;
  description?: string;
  status: string;
  is_builtin: boolean;
  profile: AgentRoleProfile;
  capability_binding: AgentRoleCapabilityBinding;
  policy_binding: AgentRolePolicyBinding;
  model_binding?: AgentRoleModelBinding;
  org_id?: string;
  tenant_id?: string;
  created_at: string;
  updated_at: string;
}

export interface AgentRoleListResponse {
  items: AgentRole[];
  page: number;
  limit: number;
  total: number;
  has_next: boolean;
}
