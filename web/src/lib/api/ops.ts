import axios from 'axios';
import { api } from './client';
import type {
  AcceptedResponse,
  ApiErrorEnvelope,
  AuditListResponse,
  ApprovalRoutingConfig,
  AuditExportResponse,
  AccessConfigResponse,
  ApprovalRoutingConfigResponse,
  AuthorizationPolicyConfig,
  BatchOperationResponse,
  BootstrapStatusResponse,
  ConnectorManifest,
  ConnectorListResponse,
  ConnectorUpsertRequest,
  ConnectorExecutionResponse,
  ConnectorInvokeCapabilityRequest,
  ConnectorInvokeCapabilityResponse,
  ExtensionBundle,
  ExtensionCandidate,
  ExtensionImportResponse,
  ExtensionListResponse,
  ExtensionValidationResponse,
  SkillImportResponse,
  SkillListResponse,
  SkillManifest,
  AutomationJob,
  AutomationListResponse,
  ConnectorsConfig,
  ConnectorsConfigResponse,
  ConnectorLifecycle,
  ConnectorMetricsQueryResponse,
  ConnectorTemplateListResponse,
  DashboardHealthResponse,
  DesensitizationConfig,
  DesensitizationConfigResponse,
  ExecutionDetail,
  ExecutionActionRequest,
  ExecutionExportResponse,
  ExecutionListResponse,
  ExecutionOutputResponse,
  KnowledgeExportResponse,
  LogListResponse,
  ObservabilityResponse,
  KnowledgeListResponse,
  OpsSummaryResponse,
  OutboxListResponse,
  ReasoningPromptConfig,
  ReasoningPromptConfigResponse,
  SessionDetail,
  SessionExportResponse,
  SessionListResponse,
  SessionTraceResponse,
  AuthorizationConfigResponse,
  ProvidersConfig,
  ProvidersConfigResponse,
  ProviderListModelsResponse,
  ProviderCheckResponse,
  PlatformDiscoveryResponse,
  SecretsInventoryResponse,
  SecretValueInput,
  SSHCredential,
  SSHCredentialListResponse,
  SSHCredentialUpsertRequest,
  SetupStatusResponse,
  SetupWizardResponse,
  SmokeAlertResponse,
} from './types';

type DownloadResponse = {
  filename: string;
  content: Blob;
};

export async function validateOpsToken(token: string): Promise<void> {
  await api.get<SessionListResponse>('/sessions', {
    headers: { Authorization: `Bearer ${token}` },
  });
}

export async function fetchAccessConfig(): Promise<AccessConfigResponse> {
  const response = await api.get<AccessConfigResponse>('/config/auth');
  return response.data;
}

export async function fetchSessions(params?: {
  status?: string;
  host?: string;
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<SessionListResponse> {
  const response = await api.get<SessionListResponse>('/sessions', {
    params,
  });
  return response.data;
}

export async function fetchExecutions(params?: {
  status?: string;
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<ExecutionListResponse> {
  const response = await api.get<ExecutionListResponse>('/executions', {
    params,
  });
  return response.data;
}

export async function fetchAuditRecords(params?: {
  resource_type?: string;
  action?: string;
  actor?: string;
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<AuditListResponse> {
  const response = await api.get<AuditListResponse>('/audit', { params });
  return response.data;
}

export async function fetchKnowledgeRecords(params?: {
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<KnowledgeListResponse> {
  const response = await api.get<KnowledgeListResponse>('/knowledge', { params });
  return response.data;
}

export async function fetchOpsSummary(): Promise<OpsSummaryResponse> {
  const response = await api.get<OpsSummaryResponse>('/summary');
  return response.data;
}

export async function fetchSetupStatus(): Promise<SetupStatusResponse> {
  const response = await api.get<SetupStatusResponse>('/setup/status');
  return response.data;
}

export async function fetchBootstrapStatus(): Promise<BootstrapStatusResponse> {
  const response = await api.get<BootstrapStatusResponse>('/bootstrap/status');
  return response.data;
}

export async function fetchSetupWizard(): Promise<SetupWizardResponse> {
  const response = await api.get<SetupWizardResponse>('/setup/wizard');
  return response.data;
}

export async function saveSetupWizardAdmin(payload: {
  username: string;
  display_name?: string;
  email?: string;
  password: string;
}): Promise<SetupWizardResponse> {
  const response = await api.post<SetupWizardResponse>('/setup/wizard/admin', payload);
  return response.data;
}

export async function saveSetupWizardAuth(payload: {
  type: string;
  name?: string;
}): Promise<SetupWizardResponse> {
  const response = await api.post<SetupWizardResponse>('/setup/wizard/auth', payload);
  return response.data;
}

export async function saveSetupWizardProvider(payload: {
  provider_id: string;
  vendor?: string;
  protocol?: string;
  base_url: string;
  api_key?: string;
  api_key_ref?: string;
  model: string;
}): Promise<SetupWizardResponse> {
  const response = await api.post<SetupWizardResponse>('/setup/wizard/provider', payload);
  return response.data;
}

export async function checkSetupWizardProvider(payload: {
  provider_id: string;
  vendor?: string;
  protocol?: string;
  base_url: string;
  api_key?: string;
  api_key_ref?: string;
  model?: string;
}): Promise<ProviderCheckResponse> {
  const response = await api.post<ProviderCheckResponse>('/setup/wizard/provider/check', payload);
  return response.data;
}

export async function saveSetupWizardChannel(payload: {
  channel_id: string;
  name?: string;
  kind?: string;
  usages?: string[];
  type?: string;
  target: string;
}): Promise<SetupWizardResponse> {
  const response = await api.post<SetupWizardResponse>('/setup/wizard/channel', payload);
  return response.data;
}

export async function completeSetupWizard(): Promise<SetupWizardResponse> {
  const response = await api.post<SetupWizardResponse>('/setup/wizard/complete');
  return response.data;
}

export async function fetchAuthorizationConfig(): Promise<AuthorizationConfigResponse> {
  const response = await api.get<AuthorizationConfigResponse>('/config/authorization');
  return response.data;
}

export async function updateAuthorizationConfig(payload: {
  content?: string;
  config?: AuthorizationPolicyConfig;
  operator_reason: string;
}): Promise<AuthorizationConfigResponse> {
  const response = await api.put<AuthorizationConfigResponse>('/config/authorization', payload);
  return response.data;
}

export async function fetchApprovalRoutingConfig(): Promise<ApprovalRoutingConfigResponse> {
  const response = await api.get<ApprovalRoutingConfigResponse>('/config/approval-routing');
  return response.data;
}

export async function fetchConnectorsConfig(): Promise<ConnectorsConfigResponse> {
  const response = await api.get<ConnectorsConfigResponse>('/config/connectors');
  return response.data;
}

export async function fetchConnectors(params?: {
  kind?: string;
  protocol?: string;
  type?: string;
  vendor?: string;
  enabled?: boolean;
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<ConnectorListResponse> {
  const response = await api.get<ConnectorListResponse>('/connectors', {
    params,
  });
  return response.data;
}

export async function fetchConnector(connectorID: string): Promise<ConnectorManifest> {
  const response = await api.get<ConnectorManifest>(`/connectors/${connectorID}`);
  return response.data;
}

export async function createConnector(payload: ConnectorUpsertRequest): Promise<ConnectorManifest> {
  const response = await api.post<ConnectorManifest>('/connectors', payload);
  return response.data;
}

export async function probeConnectorManifest(payload: {
  manifest: ConnectorManifest;
}): Promise<ConnectorLifecycle> {
  const response = await api.post<ConnectorLifecycle>('/connectors/probe', payload);
  return response.data;
}

export async function updateConnector(connectorID: string, payload: ConnectorUpsertRequest): Promise<ConnectorManifest> {
  const response = await api.put<ConnectorManifest>(`/connectors/${connectorID}`, payload);
  return response.data;
}

export async function fetchSkills(params?: {
  status?: string;
  source?: string;
  enabled?: boolean;
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<SkillListResponse> {
  const response = await api.get<SkillListResponse>('/skills', { params });
  return response.data;
}

export async function fetchSkill(skillID: string): Promise<SkillManifest> {
  const response = await api.get<SkillManifest>(`/skills/${skillID}`);
  return response.data;
}

export async function createSkill(payload: { manifest: SkillManifest; operator_reason: string }): Promise<SkillImportResponse> {
  const response = await api.post<SkillImportResponse>('/skills', payload);
  return response.data;
}

export async function updateSkill(skillID: string, payload: { manifest: SkillManifest; operator_reason: string }): Promise<SkillManifest> {
  const response = await api.put<SkillManifest>(`/skills/${skillID}`, payload);
  return response.data;
}

export async function setSkillEnabled(skillID: string, enabled: boolean, operatorReason: string): Promise<SkillManifest> {
  const response = await api.post<SkillManifest>(`/skills/${skillID}/${enabled ? 'enable' : 'disable'}`, {
    operator_reason: operatorReason,
  });
  return response.data;
}



export async function exportSkill(skillID: string, format: 'zip' | 'yaml' | 'json' | 'md' = 'zip'): Promise<DownloadResponse> {
  const response = await api.get<Blob>(`/skills/${skillID}/export`, {
    params: { format },
    responseType: 'blob',
  });
  return {
    filename: getDownloadFilename(response.headers['content-disposition'], `tars-skill-${skillID}.${format}`),
    content: response.data,
  };
}

export async function deleteSkill(skillID: string, operatorReason: string): Promise<AcceptedResponse> {
  const response = await api.delete<AcceptedResponse>(`/skills/${skillID}`, {
    data: { operator_reason: operatorReason },
  });
  return response.data;
}

export async function importSkill(payload: { manifest: SkillManifest; operator_reason: string }): Promise<SkillImportResponse> {
  const response = await api.post<SkillImportResponse>('/config/skills/import', payload);
  return response.data;
}

export async function fetchAutomations(params?: {
  type?: string;
  status?: string;
  enabled?: boolean;
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<AutomationListResponse> {
  const response = await api.get<AutomationListResponse>('/automations', { params });
  return response.data;
}

export async function fetchAutomation(jobID: string): Promise<AutomationJob> {
  const response = await api.get<AutomationJob>(`/automations/${jobID}`);
  return response.data;
}

export async function createAutomation(payload: { job: AutomationJob }): Promise<AutomationJob> {
  const response = await api.post<AutomationJob>('/automations', payload);
  return response.data;
}

export async function updateAutomation(jobID: string, payload: { job: AutomationJob }): Promise<AutomationJob> {
  const response = await api.put<AutomationJob>(`/automations/${jobID}`, payload);
  return response.data;
}

export async function setAutomationEnabled(jobID: string, enabled: boolean): Promise<AutomationJob> {
  const response = await api.post<AutomationJob>(`/automations/${jobID}/${enabled ? 'enable' : 'disable'}`);
  return response.data;
}

export async function runAutomationNow(jobID: string): Promise<AutomationJob> {
  const response = await api.post<AutomationJob>(`/automations/${jobID}/run`);
  return response.data;
}

export async function fetchExtensions(params?: {
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<ExtensionListResponse> {
  const response = await api.get<ExtensionListResponse>('/extensions', { params });
  return response.data;
}

export async function fetchExtension(candidateID: string): Promise<ExtensionCandidate> {
  const response = await api.get<ExtensionCandidate>(`/extensions/${candidateID}`);
  return response.data;
}

export async function createExtensionCandidate(payload: { bundle: ExtensionBundle; operator_reason: string }): Promise<ExtensionCandidate> {
  const response = await api.post<ExtensionCandidate>('/extensions', payload);
  return response.data;
}

export async function validateExtensionBundle(payload: { bundle: ExtensionBundle }): Promise<ExtensionValidationResponse> {
  const response = await api.post<ExtensionValidationResponse>('/extensions/validate', payload);
  return response.data;
}

export async function validateExtensionCandidate(candidateID: string): Promise<ExtensionCandidate> {
  const response = await api.post<ExtensionCandidate>(`/extensions/${candidateID}/validate`);
  return response.data;
}

export async function importExtensionBundle(payload: { bundle: ExtensionBundle; operator_reason: string }): Promise<ExtensionImportResponse> {
  const response = await api.post<ExtensionImportResponse>('/extensions/import', payload);
  return response.data;
}

export async function importExtensionCandidate(candidateID: string, operatorReason: string): Promise<ExtensionImportResponse> {
  const response = await api.post<ExtensionImportResponse>(`/extensions/${candidateID}/import`, {
    operator_reason: operatorReason,
  });
  return response.data;
}

export async function reviewExtensionCandidate(candidateID: string, payload: { review_state: string; operator_reason: string }): Promise<ExtensionCandidate> {
  const response = await api.post<ExtensionCandidate>(`/extensions/${candidateID}/review`, payload);
  return response.data;
}

export async function exportConnector(connectorID: string, format: 'yaml' | 'json' = 'yaml'): Promise<DownloadResponse> {
  const response = await api.get<Blob>(`/connectors/${connectorID}/export`, {
    params: { format },
    responseType: 'blob',
  });
  return {
    filename: getDownloadFilename(response.headers['content-disposition'], `tars-connector-${connectorID}.${format}`),
    content: response.data,
  };
}

export async function setConnectorEnabled(connectorID: string, enabled: boolean, operatorReason: string): Promise<ConnectorManifest> {
  const response = await api.post<ConnectorManifest>(`/connectors/${connectorID}/${enabled ? 'enable' : 'disable'}`, {
    operator_reason: operatorReason,
  });
  return response.data;
}

export async function queryConnectorMetrics(connectorID: string, payload: {
  service?: string;
  host?: string;
}): Promise<ConnectorMetricsQueryResponse> {
  const response = await api.post<ConnectorMetricsQueryResponse>(`/connectors/${connectorID}/metrics/query`, payload);
  return response.data;
}

export async function executeConnectorCommand(connectorID: string, payload: {
  session_id?: string;
  target_host: string;
  command: string;
  service?: string;
  operator_reason: string;
  execution_mode?: string;
}): Promise<ConnectorExecutionResponse> {
  const response = await api.post<ConnectorExecutionResponse>(`/connectors/${connectorID}/execution/execute`, payload);
  return response.data;
}

export async function checkConnectorHealth(connectorID: string): Promise<ConnectorLifecycle> {
	const response = await api.post<ConnectorLifecycle>(`/connectors/${connectorID}/health`);
	return response.data;
}

export async function invokeConnectorCapability(connectorID: string, payload: ConnectorInvokeCapabilityRequest): Promise<ConnectorInvokeCapabilityResponse> {
  const response = await api.post<ConnectorInvokeCapabilityResponse>(`/connectors/${connectorID}/capabilities/invoke`, payload);
  return response.data;
}

export async function upgradeConnector(connectorID: string, payload: {
  manifest: ConnectorManifest;
  operator_reason: string;
  available_version?: string;
}): Promise<ConnectorManifest> {
  const response = await api.post<ConnectorManifest>(`/connectors/${connectorID}/upgrade`, payload);
  return response.data;
}

export async function rollbackConnector(connectorID: string, payload: {
  target_version?: string;
  operator_reason: string;
}): Promise<ConnectorManifest> {
  const response = await api.post<ConnectorManifest>(`/connectors/${connectorID}/rollback`, payload);
  return response.data;
}

export async function fetchSecretsInventory(): Promise<SecretsInventoryResponse> {
  const response = await api.get<SecretsInventoryResponse>('/config/secrets');
  return response.data;
}

export async function updateSecretsInventory(payload: {
  upserts?: SecretValueInput[];
  deletes?: string[];
  operator_reason: string;
}): Promise<SecretsInventoryResponse> {
  const response = await api.put<SecretsInventoryResponse>('/config/secrets', payload);
  return response.data;
}

export async function fetchSSHCredentials(): Promise<SSHCredentialListResponse> {
  const response = await api.get<SSHCredentialListResponse>('/ssh-credentials');
  return response.data;
}

export async function createSSHCredential(payload: SSHCredentialUpsertRequest): Promise<SSHCredential> {
  const response = await api.post<SSHCredential>('/ssh-credentials', payload);
  return response.data;
}

export async function updateSSHCredential(credentialID: string, payload: SSHCredentialUpsertRequest): Promise<SSHCredential> {
  const response = await api.put<SSHCredential>(`/ssh-credentials/${credentialID}`, payload);
  return response.data;
}

export async function deleteSSHCredential(credentialID: string, operatorReason: string): Promise<SSHCredential> {
  const response = await api.delete<SSHCredential>(`/ssh-credentials/${credentialID}`, { data: { operator_reason: operatorReason } });
  return response.data;
}

export async function setSSHCredentialStatus(credentialID: string, status: 'active' | 'disabled' | 'rotation_required', operatorReason: string): Promise<SSHCredential> {
  const action = status === 'active' ? 'enable' : status === 'disabled' ? 'disable' : 'rotation-required';
  const response = await api.post<SSHCredential>(`/ssh-credentials/${credentialID}/${action}`, { operator_reason: operatorReason });
  return response.data;
}

export async function fetchConnectorTemplates(): Promise<ConnectorTemplateListResponse> {
  const response = await api.get<ConnectorTemplateListResponse>('/connectors/templates');
  return response.data;
}

export async function applyConnectorTemplate(connectorID: string, payload: {
  template_id: string;
  operator_reason: string;
}): Promise<ConnectorManifest> {
  const response = await api.post<ConnectorManifest>(`/connectors/${connectorID}/templates/apply`, payload);
  return response.data;
}

export async function fetchDashboardHealth(): Promise<DashboardHealthResponse> {
  const response = await api.get<DashboardHealthResponse>('/dashboard/health');
  return response.data;
}

export async function fetchLogs(params?: {
  q?: string;
  level?: string;
  component?: string;
  session_id?: string;
  execution_id?: string;
  trace_id?: string;
  from?: string;
  to?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}): Promise<LogListResponse> {
  const response = await api.get<LogListResponse>('/logs', { params });
  return response.data;
}

export async function fetchObservability(): Promise<ObservabilityResponse> {
  const response = await api.get<ObservabilityResponse>('/observability');
  return response.data;
}

export async function fetchPlatformDiscovery(): Promise<PlatformDiscoveryResponse> {
  const response = await api.get<PlatformDiscoveryResponse>('/platform/discovery');
  return response.data;
}

export async function updateConnectorsConfig(payload: {
  content?: string;
  config?: ConnectorsConfig;
  operator_reason: string;
}): Promise<ConnectorsConfigResponse> {
  const response = await api.put<ConnectorsConfigResponse>('/config/connectors', payload);
  return response.data;
}

export async function importConnectorManifest(payload: {
  manifest: ConnectorManifest;
  operator_reason: string;
}): Promise<ConnectorsConfigResponse> {
  const response = await api.post<ConnectorsConfigResponse>('/config/connectors/import', payload);
  return response.data;
}

export async function updateApprovalRoutingConfig(payload: {
  content?: string;
  config?: ApprovalRoutingConfig;
  operator_reason: string;
}): Promise<ApprovalRoutingConfigResponse> {
  const response = await api.put<ApprovalRoutingConfigResponse>('/config/approval-routing', payload);
  return response.data;
}

export async function fetchReasoningPromptConfig(): Promise<ReasoningPromptConfigResponse> {
  const response = await api.get<ReasoningPromptConfigResponse>('/config/reasoning-prompts');
  return response.data;
}

export async function updateReasoningPromptConfig(payload: {
  content?: string;
  config?: ReasoningPromptConfig;
  operator_reason: string;
}): Promise<ReasoningPromptConfigResponse> {
  const response = await api.put<ReasoningPromptConfigResponse>('/config/reasoning-prompts', payload);
  return response.data;
}

export async function fetchDesensitizationConfig(): Promise<DesensitizationConfigResponse> {
  const response = await api.get<DesensitizationConfigResponse>('/config/desensitization');
  return response.data;
}

export async function updateDesensitizationConfig(payload: {
  content?: string;
  config?: DesensitizationConfig;
  operator_reason: string;
}): Promise<DesensitizationConfigResponse> {
  const response = await api.put<DesensitizationConfigResponse>('/config/desensitization', payload);
  return response.data;
}

export async function fetchProvidersConfig(): Promise<ProvidersConfigResponse> {
  const response = await api.get<ProvidersConfigResponse>('/config/providers');
  return response.data;
}

export async function updateProvidersConfig(payload: {
  content?: string;
  config?: ProvidersConfig;
  operator_reason: string;
}): Promise<ProvidersConfigResponse> {
  const response = await api.put<ProvidersConfigResponse>('/config/providers', payload);
  return response.data;
}

export async function listProviderModels(providerID?: string): Promise<ProviderListModelsResponse> {
  const response = await api.post<ProviderListModelsResponse>('/config/providers/models', {
    provider_id: providerID,
  });
  return response.data;
}

export async function checkProviderAvailability(providerID?: string, model?: string): Promise<ProviderCheckResponse> {
  const response = await api.post<ProviderCheckResponse>('/config/providers/check', {
    provider_id: providerID,
    model,
  });
  return response.data;
}

export async function fetchSession(sessionID: string): Promise<SessionDetail> {
  const response = await api.get<SessionDetail>(`/sessions/${sessionID}`);
  return response.data;
}

export async function fetchSessionTrace(sessionID: string): Promise<SessionTraceResponse> {
  const response = await api.get<SessionTraceResponse>(`/sessions/${sessionID}/trace`);
  return response.data;
}

export async function fetchExecution(executionID: string): Promise<ExecutionDetail> {
  const response = await api.get<ExecutionDetail>(`/executions/${executionID}`);
  return response.data;
}

export async function fetchExecutionOutput(executionID: string): Promise<ExecutionOutputResponse> {
  const response = await api.get<ExecutionOutputResponse>(`/executions/${executionID}/output`);
  return response.data;
}

export async function approveExecution(executionID: string, payload?: { reason?: string }): Promise<ExecutionDetail> {
  const response = await api.post<ExecutionDetail>(`/executions/${executionID}/approve`, payload);
  return response.data;
}

export async function rejectExecution(executionID: string, payload?: { reason?: string }): Promise<ExecutionDetail> {
  const response = await api.post<ExecutionDetail>(`/executions/${executionID}/reject`, payload);
  return response.data;
}

export async function requestExecutionContext(executionID: string): Promise<ExecutionDetail> {
  const response = await api.post<ExecutionDetail>(`/executions/${executionID}/request-context`);
  return response.data;
}

export async function modifyApproveExecution(executionID: string, payload: ExecutionActionRequest): Promise<ExecutionDetail> {
  const response = await api.post<ExecutionDetail>(`/executions/${executionID}/modify-approve`, payload);
  return response.data;
}

export async function fetchOutbox(params?: {
  status?: string;
  q?: string;
  page?: number;
  limit?: number;
  sort_by?: string;
  sort_order?: string;
}) {
  const response = await api.get<OutboxListResponse>('/outbox', {
    params,
  });
  return response.data;
}

export async function replayOutbox(eventID: string, operatorReason: string): Promise<AcceptedResponse> {
  const response = await api.post<AcceptedResponse>(`/outbox/${eventID}/replay`, {
    operator_reason: operatorReason,
  });
  return response.data;
}

export async function deleteOutbox(eventID: string, operatorReason: string): Promise<AcceptedResponse> {
  const response = await api.delete<AcceptedResponse>(`/outbox/${eventID}`, {
    data: {
      operator_reason: operatorReason,
    },
  });
  return response.data;
}

export async function bulkReplayOutbox(ids: string[], operatorReason: string): Promise<BatchOperationResponse> {
  const response = await api.post<BatchOperationResponse>('/outbox/bulk/replay', {
    ids,
    operator_reason: operatorReason,
  });
  return response.data;
}

export async function bulkDeleteOutbox(ids: string[], operatorReason: string): Promise<BatchOperationResponse> {
  const response = await api.post<BatchOperationResponse>('/outbox/bulk/delete', {
    ids,
    operator_reason: operatorReason,
  });
  return response.data;
}

export async function bulkExportSessions(ids: string[], operatorReason: string): Promise<DownloadResponse> {
  const response = await api.post<Blob>('/sessions/bulk/export', {
    ids,
    operator_reason: operatorReason,
  }, {
    responseType: 'blob',
  });
  return {
    filename: getDownloadFilename(response.headers['content-disposition'], 'tars-sessions-export.json'),
    content: response.data,
  };
}

export async function bulkExportExecutions(ids: string[], operatorReason: string): Promise<DownloadResponse> {
  const response = await api.post<Blob>('/executions/bulk/export', {
    ids,
    operator_reason: operatorReason,
  }, {
    responseType: 'blob',
  });
  return {
    filename: getDownloadFilename(response.headers['content-disposition'], 'tars-executions-export.json'),
    content: response.data,
  };
}

export async function bulkExportAudit(ids: string[], operatorReason: string): Promise<DownloadResponse> {
  const response = await api.post<Blob>('/audit/bulk/export', {
    ids,
    operator_reason: operatorReason,
  }, {
    responseType: 'blob',
  });
  return {
    filename: getDownloadFilename(response.headers['content-disposition'], 'tars-audit-export.json'),
    content: response.data,
  };
}

export async function bulkExportKnowledge(ids: string[], operatorReason: string): Promise<DownloadResponse> {
  const response = await api.post<Blob>('/knowledge/bulk/export', {
    ids,
    operator_reason: operatorReason,
  }, {
    responseType: 'blob',
  });
  return {
    filename: getDownloadFilename(response.headers['content-disposition'], 'tars-knowledge-export.json'),
    content: response.data,
  };
}

export async function triggerReindex(operatorReason: string): Promise<AcceptedResponse> {
  const response = await api.post<AcceptedResponse>('/reindex/documents', {
    operator_reason: operatorReason,
  });
  return response.data;
}

export async function triggerSmokeAlert(payload: {
  alertname: string;
  service: string;
  host: string;
  severity: string;
  summary: string;
}): Promise<SmokeAlertResponse> {
  const response = await api.post<SmokeAlertResponse>('/smoke/alerts', payload);
  return response.data;
}

export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (axios.isAxiosError<ApiErrorEnvelope>(error)) {
    return error.response?.data?.error?.message || fallback;
  }
  return fallback;
}

export async function getBlobApiErrorMessage(error: unknown, fallback: string): Promise<string> {
  if (!axios.isAxiosError(error)) {
    return fallback;
  }
  const data = error.response?.data;
  if (!(data instanceof Blob)) {
    return getApiErrorMessage(error, fallback);
  }
  try {
    const text = await data.text();
    const parsed = JSON.parse(text) as ApiErrorEnvelope;
    return parsed.error?.message || fallback;
  } catch {
    return fallback;
  }
}

export function parseExportBlob<T extends SessionExportResponse | ExecutionExportResponse>(blob: Blob): Promise<T> {
  return blob.text().then((text) => JSON.parse(text) as T);
}

export function parseBulkExportBlob<T extends SessionExportResponse | ExecutionExportResponse | AuditExportResponse | KnowledgeExportResponse>(blob: Blob): Promise<T> {
  return blob.text().then((text) => JSON.parse(text) as T);
}

function getDownloadFilename(contentDisposition: string | undefined, fallback: string): string {
  if (!contentDisposition) {
    return fallback;
  }
  const match = contentDisposition.match(/filename="?([^"]+)"?/i);
  return match?.[1] || fallback;
}
