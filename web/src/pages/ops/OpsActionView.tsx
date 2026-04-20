import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useI18n } from '@/hooks/useI18n';
import { 
  ShieldCheck, 
  Shuffle, 
  KeyRound, 
  Waves, 
  Layers, 
  Cpu, 
  EyeOff, 
  Clock,
  Terminal,
  Settings,
  Database
} from 'lucide-react';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import {
  fetchApprovalRoutingConfig,
  fetchAuthorizationConfig,
  fetchConnectorsConfig,
  fetchDesensitizationConfig,
  fetchProvidersConfig,
  fetchReasoningPromptConfig,
  fetchSecretsInventory,
  fetchSSHCredentials,
  getApiErrorMessage,
  importConnectorManifest,
  createSSHCredential,
  deleteSSHCredential,
  setSSHCredentialStatus,
  triggerReindex,
  updateSSHCredential,
  updateSecretsInventory,
  updateApprovalRoutingConfig,
  updateAuthorizationConfig,
  updateConnectorsConfig,
  updateDesensitizationConfig,
  updateProvidersConfig,
  updateReasoningPromptConfig,
} from '../../lib/api/ops';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { CollapsibleList } from '@/components/ui/collapsible-list';
import { ConfirmActionDialog } from '@/components/operator/ConfirmActionDialog';
import { InlineStatus } from '@/components/ui/inline-status';
import { Input } from '@/components/ui/input';
import { LabeledField } from '@/components/ui/labeled-field';
import { Label } from '@/components/ui/label';
import { PanelCard } from '@/components/ui/panel-card';
import { SectionTitle } from '@/components/ui/page-hero';
import { Textarea } from '@/components/ui/textarea';
import { NativeSelect } from '@/components/ui/select';
import { canAccessOpsTab, canReadConfigOps, canReadSSHCredentials, canWriteConfigOps, canWriteSSHCredentials, type OpsTabID } from '@/lib/auth/permissions';
import { cn } from '@/lib/utils';
import type {
  ApprovalRoutingConfig,
  ApprovalRoutingConfigResponse,
  AuthorizationOverrideConfig,
  AuthorizationPolicyConfig,
  AuthorizationConfigResponse,
  ConnectorManifest,
  ConnectorsConfigResponse,
  DesensitizationConfig,
  DesensitizationConfigResponse,
  SecretDescriptor,
  SecretValueInput,
  SSHCredential,
  SSHCredentialListResponse,
  SSHCredentialUpsertRequest,
  ProvidersConfigResponse,
  ReasoningPromptConfig,
  ReasoningPromptConfigResponse,
  RouteEntry,
} from '../../lib/api/types';
import { connectorSamples } from '@/lib/connector-samples';

type FlashMessage = { type: 'success' | 'error'; text: string };
type EditorMode = 'form' | 'yaml';
type AuthorizationAction = AuthorizationPolicyConfig['whitelist_action'];

const actionOptions: AuthorizationAction[] = ['direct_execute', 'require_approval', 'suggest_only', 'deny'];

const emptyAuthorizationConfig = (): AuthorizationPolicyConfig => ({
  whitelist_action: 'direct_execute',
  blacklist_action: 'suggest_only',
  unmatched_action: 'require_approval',
  normalize_whitespace: true,
  hard_deny_ssh_command: [],
  hard_deny_mcp_skill: [],
  whitelist: [],
  blacklist: [],
  overrides: [],
});

const emptyApprovalConfig = (): ApprovalRoutingConfig => ({
  prohibit_self_approval: true,
  service_owners: [],
  oncall_groups: [],
  command_allowlist: [],
});

const emptyPromptConfig = (): ReasoningPromptConfig => ({
  system_prompt: '',
  user_prompt_template: '',
});

const emptySecretDraft = (): SecretValueInput => ({ ref: '', value: '' });

const emptySSHCredentialDraft = (): SSHCredentialUpsertRequest => ({
  credential_id: '',
  display_name: '',
  connector_id: 'ssh-main',
  username: '',
  auth_type: 'private_key',
  private_key: '',
  password: '',
  passphrase: '',
  host_scope: '',
  operator_reason: 'Create SSH Credential',
});

const emptyDesensitizationConfig = (): DesensitizationConfig => ({
  enabled: true,
  secrets: {
    key_names: ['password', 'passwd', 'token', 'secret', 'api_key'],
    query_key_names: ['access_token', 'refresh_token', 'token', 'secret', 'api_key'],
    additional_patterns: [],
    redact_bearer: true,
    redact_basic_auth_url: true,
    redact_sk_tokens: true,
  },
  placeholders: {
    host_key_fragments: ['host', 'hostname', 'instance', 'node', 'address'],
    path_key_fragments: ['path', 'file', 'filename', 'dir', 'directory'],
    replace_inline_ip: true,
    replace_inline_host: true,
    replace_inline_path: true,
  },
  rehydration: {
    host: true,
    ip: true,
    path: true,
  },
  local_llm_assist: {
    enabled: false,
    provider: 'openai_compatible',
    base_url: '',
    model: '',
    mode: 'detect_only',
  },
});

export const OpsActionView = () => {
  const { t } = useI18n();
  const { user } = useAuth();
  const [reindexLoading, setReindexLoading] = useState(false);
  const [reindexMessage, setReindexMessage] = useState<FlashMessage | null>(null);
  const [reindexConfirmOpen, setReindexConfirmOpen] = useState(false);

  const [authConfig, setAuthConfig] = useState<AuthorizationPolicyConfig>(emptyAuthorizationConfig());
  const [authContent, setAuthContent] = useState('');
  const [authPath, setAuthPath] = useState<string | undefined>();
  const [authUpdatedAt, setAuthUpdatedAt] = useState<string | undefined>();
  const [authLoaded, setAuthLoaded] = useState(false);
  const [authConfigured, setAuthConfigured] = useState(false);
  const [authLoading, setAuthLoading] = useState(true);
  const [authSaving, setAuthSaving] = useState(false);
  const [authMessage, setAuthMessage] = useState<FlashMessage | null>(null);
  const [authMode, setAuthMode] = useState<EditorMode>('form');

  const [approvalConfig, setApprovalConfig] = useState<ApprovalRoutingConfig>(emptyApprovalConfig());
  const [approvalContent, setApprovalContent] = useState('');
  const [approvalPath, setApprovalPath] = useState<string | undefined>();
  const [approvalUpdatedAt, setApprovalUpdatedAt] = useState<string | undefined>();
  const [approvalLoaded, setApprovalLoaded] = useState(false);
  const [approvalConfigured, setApprovalConfigured] = useState(false);
  const [approvalLoading, setApprovalLoading] = useState(true);
  const [approvalSaving, setApprovalSaving] = useState(false);
  const [approvalMessage, setApprovalMessage] = useState<FlashMessage | null>(null);
  const [approvalMode, setApprovalMode] = useState<EditorMode>('form');

  const [connectorsContent, setConnectorsContent] = useState('');
  const [connectorsPath, setConnectorsPath] = useState<string | undefined>();
  const [connectorsUpdatedAt, setConnectorsUpdatedAt] = useState<string | undefined>();
  const [connectorsLoaded, setConnectorsLoaded] = useState(false);
  const [connectorsConfigured, setConnectorsConfigured] = useState(false);
  const [connectorsLoading, setConnectorsLoading] = useState(true);
  const [connectorsSaving, setConnectorsSaving] = useState(false);
  const [connectorsMessage, setConnectorsMessage] = useState<FlashMessage | null>(null);
  const [connectorsMode, setConnectorsMode] = useState<EditorMode>('yaml');

  const [promptConfig, setPromptConfig] = useState<ReasoningPromptConfig>(emptyPromptConfig());
  const [promptContent, setPromptContent] = useState('');
  const [promptPath, setPromptPath] = useState<string | undefined>();
  const [promptUpdatedAt, setPromptUpdatedAt] = useState<string | undefined>();
  const [promptLoaded, setPromptLoaded] = useState(false);
  const [promptConfigured, setPromptConfigured] = useState(false);
  const [promptLoading, setPromptLoading] = useState(true);
  const [promptSaving, setPromptSaving] = useState(false);
  const [promptMessage, setPromptMessage] = useState<FlashMessage | null>(null);
  const [promptMode, setPromptMode] = useState<EditorMode>('form');

  const [providersContent, setProvidersContent] = useState('');
  const [providersPath, setProvidersPath] = useState<string | undefined>();
  const [providersUpdatedAt, setProvidersUpdatedAt] = useState<string | undefined>();
  const [providersLoaded, setProvidersLoaded] = useState(false);
  const [providersConfigured, setProvidersConfigured] = useState(false);
  const [providersLoading, setProvidersLoading] = useState(true);
  const [providersSaving, setProvidersSaving] = useState(false);
  const [providersMessage, setProvidersMessage] = useState<FlashMessage | null>(null);
  const [providersMode, setProvidersMode] = useState<EditorMode>('yaml');

  const [secretsInventory, setSecretsInventory] = useState<SecretDescriptor[]>([]);
  const [secretsPath, setSecretsPath] = useState<string | undefined>();
  const [secretsUpdatedAt, setSecretsUpdatedAt] = useState<string | undefined>();
  const [secretsLoaded, setSecretsLoaded] = useState(false);
  const [secretsConfigured, setSecretsConfigured] = useState(false);
  const [secretsLoading, setSecretsLoading] = useState(true);
  const [secretsSaving, setSecretsSaving] = useState(false);
  const [secretsMessage, setSecretsMessage] = useState<FlashMessage | null>(null);
  const [secretDrafts, setSecretDrafts] = useState<SecretValueInput[]>([emptySecretDraft()]);
  const [sshCredentials, setSSHCredentials] = useState<SSHCredential[]>([]);
  const [sshCredentialsConfigured, setSSHCredentialsConfigured] = useState(false);
  const [sshCredentialDraft, setSSHCredentialDraft] = useState<SSHCredentialUpsertRequest>(emptySSHCredentialDraft());
  const [editingSSHCredentialID, setEditingSSHCredentialID] = useState<string | null>(null);
  const [pendingDeleteSSHCredentialID, setPendingDeleteSSHCredentialID] = useState<string | null>(null);

  const location = useLocation();
  const navigate = useNavigate();

  const [desenseConfig, setDesenseConfig] = useState<DesensitizationConfig>(emptyDesensitizationConfig());
  const [desenseContent, setDesenseContent] = useState('');
  const [desensePath, setDesensePath] = useState<string | undefined>();
  const [desenseUpdatedAt, setDesenseUpdatedAt] = useState<string | undefined>();
  const [desenseLoaded, setDesenseLoaded] = useState(false);
  const [desenseConfigured, setDesenseConfigured] = useState(false);
  const [desenseLoading, setDesenseLoading] = useState(true);
  const [desenseSaving, setDesenseSaving] = useState(false);
  const [desenseMessage, setDesenseMessage] = useState<FlashMessage | null>(null);
  const [desenseMode, setDesenseMode] = useState<EditorMode>('form');
  const canViewConfigOps = canReadConfigOps(user);
  const canSaveConfigOps = canWriteConfigOps(user);
  const canViewSecretsInventory = canViewConfigOps;
  const canSaveSecretsInventory = canSaveConfigOps;
  const canViewSSHCredentialOps = canReadSSHCredentials(user);
  const canSaveSSHCredentialOps = canWriteSSHCredentials(user);

  const tabs = useMemo(() => [
    { id: 'auth', label: t('ops.tab.auth') },
    { id: 'approval', label: t('ops.tab.approval') },
    { id: 'secrets', label: t('ops.tab.secrets') },
    { id: 'providers', label: t('ops.tab.providers') },
    { id: 'connectors', label: t('ops.tab.connectors') },
    { id: 'prompts', label: t('ops.tab.prompts') },
    { id: 'desense', label: t('ops.tab.desense') },
    { id: 'advanced', label: t('ops.tab.advanced') },
  ].filter((tab) => canAccessOpsTab(user, tab.id as OpsTabID)), [t, user]);

  const defaultTab = tabs.find((tab) => tab.id === 'advanced')?.id || tabs[0]?.id || 'auth';
  const requestedTab = new URLSearchParams(location.search).get('tab');
  const hashTabMap: Record<string, string> = {
    '#secret-inventory': 'secrets',
  };
  const hashTab = hashTabMap[location.hash] || null;
  const activeTab = tabs.some((tab) => tab.id === requestedTab) ? requestedTab || defaultTab : defaultTab;
  const resolvedActiveTab = hashTab || activeTab;

  useEffect(() => {
    let active = true;
    const updateIfActive = (fn: () => void) => {
      if (active) {
        fn();
      }
    };

    const loadActiveTab = () => {
      switch (resolvedActiveTab) {
        case 'auth':
          if (!canViewConfigOps) {
            updateIfActive(() => setAuthLoading(false));
            return;
          }
          updateIfActive(() => setAuthLoading(true));
          void fetchAuthorizationConfig()
            .then((response) => updateIfActive(() => applyAuthorizationResponse(response)))
            .catch((error) => updateIfActive(() => {
              setAuthMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.auth.loadFailed')) });
            }))
            .finally(() => updateIfActive(() => setAuthLoading(false)));
          return;
        case 'approval':
          if (!canViewConfigOps) {
            updateIfActive(() => setApprovalLoading(false));
            return;
          }
          updateIfActive(() => setApprovalLoading(true));
          void fetchApprovalRoutingConfig()
            .then((response) => updateIfActive(() => applyApprovalResponse(response)))
            .catch((error) => updateIfActive(() => {
              setApprovalMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.approval.loadFailed')) });
            }))
            .finally(() => updateIfActive(() => setApprovalLoading(false)));
          return;
        case 'connectors':
          if (!canViewConfigOps) {
            updateIfActive(() => setConnectorsLoading(false));
            return;
          }
          updateIfActive(() => setConnectorsLoading(true));
          void fetchConnectorsConfig()
            .then((response) => updateIfActive(() => applyConnectorsResponse(response)))
            .catch((error) => updateIfActive(() => {
              setConnectorsMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.connectors.loadFailed')) });
            }))
            .finally(() => updateIfActive(() => setConnectorsLoading(false)));
          return;
        case 'prompts':
          if (!canViewConfigOps) {
            updateIfActive(() => setPromptLoading(false));
            return;
          }
          updateIfActive(() => setPromptLoading(true));
          void fetchReasoningPromptConfig()
            .then((response) => updateIfActive(() => applyPromptResponse(response)))
            .catch((error) => updateIfActive(() => {
              setPromptMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.prompts.loadFailed')) });
            }))
            .finally(() => updateIfActive(() => setPromptLoading(false)));
          return;
        case 'providers':
          if (!canViewConfigOps) {
            updateIfActive(() => setProvidersLoading(false));
            return;
          }
          updateIfActive(() => setProvidersLoading(true));
          void fetchProvidersConfig()
            .then((response) => updateIfActive(() => applyProvidersResponse(response)))
            .catch((error) => updateIfActive(() => {
              setProvidersMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.providers.loadFailed')) });
            }))
            .finally(() => updateIfActive(() => setProvidersLoading(false)));
          return;
        case 'desense':
          if (!canViewConfigOps) {
            updateIfActive(() => setDesenseLoading(false));
            return;
          }
          updateIfActive(() => setDesenseLoading(true));
          void fetchDesensitizationConfig()
            .then((response) => updateIfActive(() => applyDesensitizationResponse(response)))
            .catch((error) => updateIfActive(() => {
              setDesenseMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.desense.loadFailed')) });
            }))
            .finally(() => updateIfActive(() => setDesenseLoading(false)));
          return;
        case 'secrets': {
          updateIfActive(() => setSecretsLoading(canViewSecretsInventory || canViewSSHCredentialOps));
          const secretRequests: Promise<unknown>[] = [];
          if (canViewSecretsInventory) {
            secretRequests.push(
              fetchSecretsInventory()
                .then((response) => updateIfActive(() => applySecretsResponse(response)))
                .catch((error) => updateIfActive(() => {
                  setSecretsMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.secrets.loadFailed')) });
                })),
            );
          } else {
            updateIfActive(() => applySecretsResponse({ configured: false, loaded: false, items: [] }));
          }
          if (canViewSSHCredentialOps) {
            secretRequests.push(
              fetchSSHCredentials().then((response) => updateIfActive(() => applySSHCredentialsResponse(response))),
            );
          } else {
            updateIfActive(() => applySSHCredentialsResponse({ configured: false, items: [] }));
          }
          if (secretRequests.length > 0) {
            void Promise.allSettled(secretRequests).then(() => updateIfActive(() => setSecretsLoading(false)));
          } else {
            updateIfActive(() => setSecretsLoading(false));
          }
          return;
        }
        default:
          updateIfActive(() => {
            setAuthLoading(false);
            setApprovalLoading(false);
            setConnectorsLoading(false);
            setPromptLoading(false);
            setProvidersLoading(false);
            setDesenseLoading(false);
            setSecretsLoading(false);
          });
      }
    };

    loadActiveTab();

    return () => {
      active = false;
    };
  }, [canViewConfigOps, canViewSecretsInventory, canViewSSHCredentialOps, resolvedActiveTab]);

  const applyAuthorizationResponse = (response: AuthorizationConfigResponse) => {
    setAuthConfig({
      ...emptyAuthorizationConfig(),
      ...response.config,
      hard_deny_ssh_command: response.config.hard_deny_ssh_command || [],
      hard_deny_mcp_skill: response.config.hard_deny_mcp_skill || [],
      whitelist: response.config.whitelist || [],
      blacklist: response.config.blacklist || [],
      overrides: response.config.overrides || [],
    });
    setAuthContent(response.content || '');
    setAuthPath(response.path);
    setAuthUpdatedAt(response.updated_at);
    setAuthLoaded(response.loaded);
    setAuthConfigured(response.configured);
  };

  const applyApprovalResponse = (response: ApprovalRoutingConfigResponse) => {
    setApprovalConfig({
      prohibit_self_approval: response.config.prohibit_self_approval,
      service_owners: response.config.service_owners || [],
      oncall_groups: response.config.oncall_groups || [],
      command_allowlist: response.config.command_allowlist || [],
    });
    setApprovalContent(response.content || '');
    setApprovalPath(response.path);
    setApprovalUpdatedAt(response.updated_at);
    setApprovalLoaded(response.loaded);
    setApprovalConfigured(response.configured);
  };

  const applyConnectorsResponse = (response: ConnectorsConfigResponse) => {
    setConnectorsContent(response.content || '');
    setConnectorsPath(response.path);
    setConnectorsUpdatedAt(response.updated_at);
    setConnectorsLoaded(response.loaded);
    setConnectorsConfigured(response.configured);
  };

  const applyPromptResponse = (response: ReasoningPromptConfigResponse) => {
    setPromptConfig({
      system_prompt: response.config.system_prompt || '',
      user_prompt_template: response.config.user_prompt_template || '',
    });
    setPromptContent(response.content || '');
    setPromptPath(response.path);
    setPromptUpdatedAt(response.updated_at);
    setPromptLoaded(response.loaded);
    setPromptConfigured(response.configured);
  };

  const applyProvidersResponse = (response: ProvidersConfigResponse) => {
    setProvidersContent(response.content || '');
    setProvidersPath(response.path);
    setProvidersUpdatedAt(response.updated_at);
    setProvidersLoaded(response.loaded);
    setProvidersConfigured(response.configured);
  };

  const applySecretsResponse = (response: { configured: boolean; loaded: boolean; path?: string; updated_at?: string; items: SecretDescriptor[] }) => {
    setSecretsInventory(response.items || []);
    setSecretsPath(response.path);
    setSecretsUpdatedAt(response.updated_at);
    setSecretsLoaded(response.loaded);
    setSecretsConfigured(response.configured);
  };

  const applySSHCredentialsResponse = (response: SSHCredentialListResponse) => {
    setSSHCredentials(response.items || []);
    setSSHCredentialsConfigured(response.configured);
  };

  const applyDesensitizationResponse = (response: DesensitizationConfigResponse) => {
    setDesenseConfig({
      ...emptyDesensitizationConfig(),
      ...response.config,
      secrets: {
        ...emptyDesensitizationConfig().secrets,
        ...response.config.secrets,
        key_names: response.config.secrets?.key_names || [],
        query_key_names: response.config.secrets?.query_key_names || [],
        additional_patterns: response.config.secrets?.additional_patterns || [],
      },
      placeholders: {
        ...emptyDesensitizationConfig().placeholders,
        ...response.config.placeholders,
        host_key_fragments: response.config.placeholders?.host_key_fragments || [],
        path_key_fragments: response.config.placeholders?.path_key_fragments || [],
      },
      rehydration: {
        ...emptyDesensitizationConfig().rehydration,
        ...response.config.rehydration,
      },
      local_llm_assist: {
        ...emptyDesensitizationConfig().local_llm_assist,
        ...response.config.local_llm_assist,
      },
    });
    setDesenseContent(response.content || '');
    setDesensePath(response.path);
    setDesenseUpdatedAt(response.updated_at);
    setDesenseLoaded(response.loaded);
    setDesenseConfigured(response.configured);
  };

  const handleReindex = async () => {
    try {
      setReindexLoading(true);
      setReindexMessage(null);
      await triggerReindex('Trigger Reindex');
      setReindexMessage({ type: 'success', text: t('ops.advanced.reindexOk') });
    } catch (error) {
      setReindexMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.advanced.reindexFailed')) });
    } finally {
      setReindexLoading(false);
    }
  };

  const handleAuthorizationSave = async () => {
    try {
      setAuthSaving(true);
      setAuthMessage(null);
      const response = await updateAuthorizationConfig({
        operator_reason: 'Update Authorization',
        ...(authMode === 'yaml' ? { content: authContent } : { config: authConfig }),
      });
      applyAuthorizationResponse(response);
      setAuthMessage({ type: 'success', text: t('ops.auth.saveOk') });
    } catch (error) {
      setAuthMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.auth.saveFailed')) });
    } finally {
      setAuthSaving(false);
    }
  };

  const handleApprovalSave = async () => {
    try {
      setApprovalSaving(true);
      setApprovalMessage(null);
      const response = await updateApprovalRoutingConfig({
        operator_reason: 'Update Approval Routing',
        ...(approvalMode === 'yaml' ? { content: approvalContent } : { config: approvalConfig }),
      });
      applyApprovalResponse(response);
      setApprovalMessage({ type: 'success', text: t('ops.approval.saveOk') });
    } catch (error) {
      setApprovalMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.approval.saveFailed')) });
    } finally {
      setApprovalSaving(false);
    }
  };

  const handleConnectorsSave = async () => {
    try {
      setConnectorsSaving(true);
      setConnectorsMessage(null);
      const response = await updateConnectorsConfig({
        operator_reason: 'Update Connector Registry',
        content: connectorsContent,
      });
      applyConnectorsResponse(response);
      setConnectorsMessage({ type: 'success', text: t('ops.connectors.saveOk') });
    } catch (error) {
      setConnectorsMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.connectors.saveFailed')) });
    } finally {
      setConnectorsSaving(false);
    }
  };

  const handleImportConnectorSample = async (manifest: ConnectorManifest) => {
    try {
      setConnectorsSaving(true);
      setConnectorsMessage(null);
      const response = await importConnectorManifest({
        manifest,
        operator_reason: 'Import official connector sample',
      });
      applyConnectorsResponse(response);
      setConnectorsMessage({ type: 'success', text: t('ops.connectors.importOk', { id: manifest.metadata.id || manifest.metadata.name || 'sample' }) });
    } catch (error) {
      setConnectorsMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.connectors.importFailed')) });
    } finally {
      setConnectorsSaving(false);
    }
  };

  const handlePromptSave = async () => {
    try {
      setPromptSaving(true);
      setPromptMessage(null);
      const response = await updateReasoningPromptConfig({
        operator_reason: 'Update Reasoning Prompts',
        ...(promptMode === 'yaml' ? { content: promptContent } : { config: promptConfig }),
      });
      applyPromptResponse(response);
      setPromptMessage({ type: 'success', text: t('ops.prompts.saveOk') });
    } catch (error) {
      setPromptMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.prompts.saveFailed')) });
    } finally {
      setPromptSaving(false);
    }
  };

  const handleProvidersSave = async () => {
    try {
      setProvidersSaving(true);
      setProvidersMessage(null);
      const response = await updateProvidersConfig({
        operator_reason: 'Update Model Providers',
        content: providersContent,
      });
      applyProvidersResponse(response);
      setProvidersMessage({ type: 'success', text: t('ops.providers.saveOk') });
    } catch (error) {
      setProvidersMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.providers.saveFailed')) });
    } finally {
      setProvidersSaving(false);
    }
  };

  const handleDesensitizationSave = async () => {
    try {
      setDesenseSaving(true);
      setDesenseMessage(null);
      const response = await updateDesensitizationConfig({
        operator_reason: 'Update Desensitization Config',
        ...(desenseMode === 'yaml' ? { content: desenseContent } : { config: desenseConfig }),
      });
      applyDesensitizationResponse(response);
      setDesenseMessage({ type: 'success', text: t('ops.desense.saveOk') });
    } catch (error) {
      setDesenseMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.desense.saveFailed')) });
    } finally {
      setDesenseSaving(false);
    }
  };

  const handleSecretsSave = async () => {
    if (!canSaveSecretsInventory) {
      setSecretsMessage({ type: 'error', text: 'Insufficient permissions to update shared secret inventory.' });
      return;
    }
    const upserts = secretDrafts.filter((item) => item.ref.trim() || item.value.trim()).filter((item) => item.ref.trim());
    if (!upserts.length) {
      setSecretsMessage({ type: 'error', text: t('ops.secrets.addAtLeastOne') });
      return;
    }
    try {
      setSecretsSaving(true);
      setSecretsMessage(null);
      const response = await updateSecretsInventory({
        operator_reason: 'Update Secrets Inventory',
        upserts,
      });
      applySecretsResponse(response);
      setSecretDrafts([emptySecretDraft()]);
      setSecretsMessage({ type: 'success', text: t('ops.secrets.saveOk') });
    } catch (error) {
      setSecretsMessage({ type: 'error', text: getApiErrorMessage(error, t('ops.secrets.saveFailed')) });
    } finally {
      setSecretsSaving(false);
    }
  };

  const handleSSHCredentialCreate = async () => {
    if (!canSaveSSHCredentialOps) {
      setSecretsMessage({ type: 'error', text: 'Insufficient permissions to store SSH credentials.' });
      return;
    }
    if (!sshCredentialsConfigured) {
      setSecretsMessage({ type: 'error', text: 'Encrypted SSH credential custody is not configured. Set TARS_POSTGRES_DSN and TARS_SECRET_CUSTODY_KEY first.' });
      return;
    }
    try {
      setSecretsSaving(true);
      setSecretsMessage(null);
      const created = await createSSHCredential({
        ...sshCredentialDraft,
        operator_reason: sshCredentialDraft.operator_reason || 'Create SSH Credential',
      });
      setSSHCredentials((items) => [created, ...items.filter((item) => item.credential_id !== created.credential_id)]);
      setSSHCredentialDraft(emptySSHCredentialDraft());
      setSecretsMessage({ type: 'success', text: 'SSH credential stored in encrypted custody.' });
    } catch (error) {
      setSecretsMessage({ type: 'error', text: getApiErrorMessage(error, 'Failed to save SSH credential.') });
    } finally {
      setSecretsSaving(false);
    }
  };

  const startSSHCredentialEdit = (credential: SSHCredential) => {
    if (!canSaveSSHCredentialOps) {
      return;
    }
    setEditingSSHCredentialID(credential.credential_id);
    setSSHCredentialDraft({
      credential_id: credential.credential_id,
      display_name: credential.display_name || '',
      connector_id: credential.connector_id || 'ssh-main',
      username: credential.username || '',
      auth_type: credential.auth_type || 'private_key',
      private_key: '',
      password: '',
      passphrase: '',
      host_scope: credential.host_scope || '',
      expires_at: credential.expires_at || '',
      operator_reason: 'Replace SSH Credential',
    });
  };

  const handleSSHCredentialReplace = async () => {
    if (!editingSSHCredentialID) {
      return;
    }
    if (!canSaveSSHCredentialOps) {
      setSecretsMessage({ type: 'error', text: 'Insufficient permissions to replace SSH credentials.' });
      return;
    }
    try {
      setSecretsSaving(true);
      setSecretsMessage(null);
      const updated = await updateSSHCredential(editingSSHCredentialID, {
        ...sshCredentialDraft,
        operator_reason: sshCredentialDraft.operator_reason || 'Replace SSH Credential',
      });
      setSSHCredentials((items) => items.map((item) => item.credential_id === updated.credential_id ? updated : item));
      setEditingSSHCredentialID(null);
      setSSHCredentialDraft(emptySSHCredentialDraft());
      setSecretsMessage({ type: 'success', text: 'SSH credential replaced in encrypted custody.' });
    } catch (error) {
      setSecretsMessage({ type: 'error', text: getApiErrorMessage(error, 'Failed to replace SSH credential.') });
    } finally {
      setSecretsSaving(false);
    }
  };

  const handleSSHCredentialStatus = async (credentialID: string, status: 'active' | 'disabled' | 'rotation_required') => {
    if (!canSaveSSHCredentialOps) {
      setSecretsMessage({ type: 'error', text: 'Insufficient permissions to update SSH credential status.' });
      return;
    }
    try {
      setSecretsSaving(true);
      setSecretsMessage(null);
      const operatorReason = status === 'active'
        ? 'Enable SSH Credential'
        : status === 'disabled'
          ? 'Disable SSH Credential'
          : 'Mark rotation required for SSH credential';
      const updated = await setSSHCredentialStatus(credentialID, status, operatorReason);
      setSSHCredentials((items) => items.map((item) => item.credential_id === updated.credential_id ? updated : item));
      const actionLabel = status === 'active' ? 'enabled' : status === 'disabled' ? 'disabled' : 'marked rotation required';
      setSecretsMessage({ type: 'success', text: `SSH credential ${actionLabel}.` });
    } catch (error) {
      setSecretsMessage({ type: 'error', text: getApiErrorMessage(error, 'Failed to update SSH credential.') });
    } finally {
      setSecretsSaving(false);
    }
  };

  const handleSSHCredentialDelete = async () => {
    if (!pendingDeleteSSHCredentialID) {
      return;
    }
    if (!canSaveSSHCredentialOps) {
      setSecretsMessage({ type: 'error', text: 'Insufficient permissions to delete SSH credentials.' });
      return;
    }
    try {
      setSecretsSaving(true);
      setSecretsMessage(null);
      const deleted = await deleteSSHCredential(pendingDeleteSSHCredentialID, 'Delete SSH Credential');
      setSSHCredentials((items) => items.filter((item) => item.credential_id !== deleted.credential_id));
      if (editingSSHCredentialID === pendingDeleteSSHCredentialID) {
        setEditingSSHCredentialID(null);
        setSSHCredentialDraft(emptySSHCredentialDraft());
      }
      setPendingDeleteSSHCredentialID(null);
      setSecretsMessage({ type: 'success', text: 'SSH credential deleted.' });
    } catch (error) {
      setSecretsMessage({ type: 'error', text: getApiErrorMessage(error, 'Failed to delete SSH credential.') });
    } finally {
      setSecretsSaving(false);
    }
  };

  useEffect(() => {
    if (!location.hash) {
      return;
    }
    const target = location.hash.slice(1);
    if (!target) {
      return;
    }
    const node = document.getElementById(target);
    if (node) {
      node.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  }, [resolvedActiveTab, location.hash]);

  const changeTab = (tabID: string) => {
    const params = new URLSearchParams(location.search);
    params.set('tab', tabID);
    navigate({ pathname: location.pathname, search: params.toString() ? `?${params.toString()}` : '', hash: '' }, { replace: true });
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle
        title={t('ops.title')}
        subtitle={t('ops.subtitle')}
        className="mb-0"
      />

      <Card className="border border-border p-5">
        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div className="max-w-3xl">
            <div className="text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">{t('ops.repairConsole.badge')}</div>
            <div className="mt-1 text-lg font-semibold text-foreground">{t('ops.repairConsole.headline')}</div>
            <div className="mt-2 text-sm text-muted-foreground">
              {t('ops.repairConsole.hint')}
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" asChild>
              <Link to="/logs">Logs</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/ops/observability">Observability</Link>
            </Button>
            <Button variant="outline" asChild>
              <Link to="/audit">Audit Trail</Link>
            </Button>
          </div>
        </div>
      </Card>

      <div className="hide-scrollbar mb-6 flex gap-8 overflow-x-auto border-b border-border/60">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => changeTab(tab.id)}
            className={cn(
              'whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors',
              resolvedActiveTab === tab.id
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {resolvedActiveTab === 'auth' && (

      <ConfigSection
        icon={<ShieldCheck size={20} />}
        title={t('ops.auth.title')}
        description={t('ops.auth.desc')}
        meta={{
          configured: authConfigured,
          loaded: authLoaded,
          path: authPath,
          updatedAt: authUpdatedAt,
        }}
        mode={authMode}
        setMode={setAuthMode}
        loading={authLoading}
        saving={authSaving}
        message={authMessage}
        onSave={handleAuthorizationSave}
        saveLabel={t('ops.auth.saveLabel')}
        canSave={canSaveConfigOps}
      >
        {authMode === 'form' ? (
          <div className={panelGridClass}>
            <div className={autoFitGridClass}>
              <LabeledField label={t('ops.auth.whitelistAction')}>
                <ActionSelect value={authConfig.whitelist_action} onChange={(value) => setAuthConfig((current) => ({ ...current, whitelist_action: value }))} disabled={authLoading || authSaving} />
              </LabeledField>
              <LabeledField label={t('ops.auth.blacklistAction')}>
                <ActionSelect value={authConfig.blacklist_action} onChange={(value) => setAuthConfig((current) => ({ ...current, blacklist_action: value }))} disabled={authLoading || authSaving} />
              </LabeledField>
              <LabeledField label={t('ops.auth.unmatchedAction')}>
                <ActionSelect value={authConfig.unmatched_action} onChange={(value) => setAuthConfig((current) => ({ ...current, unmatched_action: value }))} disabled={authLoading || authSaving} />
              </LabeledField>
            </div>

            <label className="flex items-center gap-3 text-sm text-muted-foreground">
              <Checkbox
                checked={authConfig.normalize_whitespace}
                onCheckedChange={(value) => setAuthConfig((current) => ({ ...current, normalize_whitespace: value === true }))}
                disabled={authLoading || authSaving}
              />
              {t('ops.auth.normalizeWhitespace')}
            </label>

            <StringListEditor
              label={t('ops.auth.whitelistPatterns')}
              description={t('ops.auth.whitelistPatternsDesc')}
              values={authConfig.whitelist || []}
              onChange={(values) => setAuthConfig((current) => ({ ...current, whitelist: values }))}
              disabled={authLoading || authSaving}
              addLabel={t('ops.auth.whitelistAdd')}
            />

            <StringListEditor
              label={t('ops.auth.blacklistPatterns')}
              description={t('ops.auth.blacklistPatternsDesc')}
              values={authConfig.blacklist || []}
              onChange={(values) => setAuthConfig((current) => ({ ...current, blacklist: values }))}
              disabled={authLoading || authSaving}
              addLabel={t('ops.auth.blacklistAdd')}
            />

            <StringListEditor
              label={t('ops.auth.hardDenySsh')}
              description={t('ops.auth.hardDenySshDesc')}
              values={authConfig.hard_deny_ssh_command || []}
              onChange={(values) => setAuthConfig((current) => ({ ...current, hard_deny_ssh_command: values }))}
              disabled={authLoading || authSaving}
              addLabel={t('ops.auth.hardDenySshAdd')}
            />

            <StringListEditor
              label={t('ops.auth.hardDenyMcp')}
              description={t('ops.auth.hardDenyMcpDesc')}
              values={authConfig.hard_deny_mcp_skill || []}
              onChange={(values) => setAuthConfig((current) => ({ ...current, hard_deny_mcp_skill: values }))}
              disabled={authLoading || authSaving}
              addLabel={t('ops.auth.hardDenyMcpAdd')}
            />

            <OverrideEditor
              overrides={authConfig.overrides || []}
              onChange={(overrides) => setAuthConfig((current) => ({ ...current, overrides }))}
              disabled={authLoading || authSaving}
            />
          </div>
        ) : (
          <YamlEditor
            label={t('ops.auth.yamlLabel')}
            value={authContent}
            onChange={setAuthContent}
            disabled={authLoading || authSaving}
            placeholder="authorization:\n  defaults:\n    whitelist_action: direct_execute"
          />
        )}
      </ConfigSection>
      )}

      {resolvedActiveTab === 'approval' && (
      <ConfigSection
        icon={<Shuffle size={20} />}
        title={t('ops.approval.title')}
        description={t('ops.approval.desc')}
        meta={{
          configured: approvalConfigured,
          loaded: approvalLoaded,
          path: approvalPath,
          updatedAt: approvalUpdatedAt,
        }}
        mode={approvalMode}
        setMode={setApprovalMode}
        loading={approvalLoading}
        saving={approvalSaving}
        message={approvalMessage}
        onSave={handleApprovalSave}
        saveLabel={t('ops.approval.saveLabel')}
        canSave={canSaveConfigOps}
      >
        {approvalMode === 'form' ? (
          <div className={panelGridClass}>
            <label className="flex items-center gap-3 text-sm text-muted-foreground">
              <Checkbox
                checked={approvalConfig.prohibit_self_approval}
                onCheckedChange={(value) => setApprovalConfig((current) => ({ ...current, prohibit_self_approval: value === true }))}
                disabled={approvalLoading || approvalSaving}
              />
              {t('ops.approval.prohibitSelf')}
            </label>

            <RouteEntryEditor
              label={t('ops.approval.serviceOwners')}
              description={t('ops.approval.serviceOwnersDesc')}
              values={approvalConfig.service_owners || []}
              onChange={(values) => setApprovalConfig((current) => ({ ...current, service_owners: values }))}
              disabled={approvalLoading || approvalSaving}
              keyPlaceholder="service key"
              valuesPlaceholder="445308292, -1001234567890"
              addLabel={t('ops.approval.serviceOwnersAdd')}
            />

            <RouteEntryEditor
              label={t('ops.approval.oncall')}
              description={t('ops.approval.oncallDesc')}
              values={approvalConfig.oncall_groups || []}
              onChange={(values) => setApprovalConfig((current) => ({ ...current, oncall_groups: values }))}
              disabled={approvalLoading || approvalSaving}
              keyPlaceholder="default"
              valuesPlaceholder="445308292"
              addLabel={t('ops.approval.oncallAdd')}
            />

            <RouteEntryEditor
              label={t('ops.approval.commandAllowlist')}
              description={t('ops.approval.commandAllowlistDesc')}
              values={approvalConfig.command_allowlist || []}
              onChange={(values) => setApprovalConfig((current) => ({ ...current, command_allowlist: values }))}
              disabled={approvalLoading || approvalSaving}
              keyPlaceholder="sshd"
              valuesPlaceholder="systemctl restart sshd, journalctl -u sshd"
              addLabel={t('ops.approval.commandAllowlistAdd')}
            />
          </div>
        ) : (
          <YamlEditor
            label={t('ops.approval.yamlLabel')}
            value={approvalContent}
            onChange={setApprovalContent}
            disabled={approvalLoading || approvalSaving}
            placeholder="approval:\n  prohibit_self_approval: true"
          />
        )}
      </ConfigSection>
      )}

      {resolvedActiveTab === 'secrets' && (
      <ConfigSection
        icon={<KeyRound size={20} />}
        anchorId="secret-inventory"
        title={t('ops.secrets.title')}
        description={t('ops.secrets.desc')}
        meta={{
          configured: secretsConfigured,
          loaded: secretsLoaded,
          path: secretsPath,
          updatedAt: secretsUpdatedAt,
        }}
        mode="form"
        setMode={() => undefined}
        loading={secretsLoading}
        saving={secretsSaving}
        message={secretsMessage}
        onSave={handleSecretsSave}
        saveLabel={t('ops.secrets.saveLabel')}
        canSave={canSaveSecretsInventory}
      >
        <div className={panelGridClass}>
          {canViewSecretsInventory ? (
          <PanelCard title={t('ops.secrets.writeOnly')} subtitle={t('ops.secrets.writeOnlyDesc')}>
            <div className="grid gap-3">
              {secretDrafts.map((item, index) => (
                <div key={`secret-draft-${index}`} className="grid gap-3 xl:grid-cols-[minmax(260px,1fr)_minmax(220px,1fr)_auto]">
                  <Input
                    value={item.ref}
                    onChange={(event) => setSecretDrafts(updateSecretDraft(secretDrafts, index, { ...item, ref: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !canSaveSecretsInventory}
                    placeholder="connector.prometheus-main.bearer_token"
                  />
                  <Input
                    type="password"
                    value={item.value}
                    onChange={(event) => setSecretDrafts(updateSecretDraft(secretDrafts, index, { ...item, value: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !canSaveSecretsInventory}
                    placeholder="Enter secret value"
                  />
                  <Button variant="outline" type="button" disabled={secretsLoading || secretsSaving || !canSaveSecretsInventory} onClick={() => setSecretDrafts(secretDrafts.filter((_, itemIndex) => itemIndex !== index))}>
                    {t('ops.secrets.remove')}
                  </Button>
                </div>
              ))}
              <div>
                <Button variant="outline" type="button" disabled={secretsLoading || secretsSaving || !canSaveSecretsInventory} onClick={() => setSecretDrafts([...secretDrafts, emptySecretDraft()])}>
                  {t('ops.secrets.addSecret')}
                </Button>
              </div>
            </div>
          </PanelCard>
          ) : null}

          {canViewSSHCredentialOps ? (
          <PanelCard title="SSH Credential Custody" subtitle="Store SSH passwords or private keys in the encrypted custody backend. Secret values are write-only and never echoed back.">
            <div className="grid gap-4">
              {!sshCredentialsConfigured && (
                <InlineStatus type="warning" message="Requires encrypted backend: set TARS_POSTGRES_DSN and TARS_SECRET_CUSTODY_KEY to enable SSH credential custody." />
              )}
              <div className="grid gap-3 xl:grid-cols-2">
                <LabeledField label="Credential ID">
                  <Input
                    value={sshCredentialDraft.credential_id || ''}
                    onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, credential_id: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                    placeholder="ops-root"
                  />
                </LabeledField>
                <LabeledField label="Connector ID">
                  <Input
                    value={sshCredentialDraft.connector_id || ''}
                    onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, connector_id: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                    placeholder="ssh-main"
                  />
                </LabeledField>
                <LabeledField label="Username">
                  <Input
                    value={sshCredentialDraft.username || ''}
                    onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, username: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                    placeholder="root"
                  />
                </LabeledField>
                <LabeledField label="Host scope">
                  <Input
                    value={sshCredentialDraft.host_scope || ''}
                    onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, host_scope: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                    placeholder="192.168.3.100,192.168.3.9 or 192.168.3.0/24"
                  />
                </LabeledField>
                <LabeledField label="Auth type">
                  <NativeSelect
                    value={sshCredentialDraft.auth_type || 'private_key'}
                    onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, auth_type: event.target.value, password: '', private_key: '', passphrase: '' }))}
                    disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                  >
                    <option value="private_key">Private key</option>
                    <option value="password">Password</option>
                  </NativeSelect>
                </LabeledField>
                <LabeledField label="Display name">
                  <Input
                    value={sshCredentialDraft.display_name || ''}
                    onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, display_name: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                    placeholder="Production root key"
                  />
                </LabeledField>
              </div>
              {sshCredentialDraft.auth_type === 'password' ? (
                <LabeledField label="Password" hint="Write-only. The API response never returns this value.">
                  <Input
                    type="password"
                    value={sshCredentialDraft.password || ''}
                    onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, password: event.target.value }))}
                    disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                    placeholder="Enter SSH password"
                  />
                </LabeledField>
              ) : (
                <div className="grid gap-3">
                  <LabeledField label="Private key" hint="Write-only PEM/OpenSSH private key. Prefer scoped, rotated keys.">
                    <Textarea
                      value={sshCredentialDraft.private_key || ''}
                      onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, private_key: event.target.value }))}
                      disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                      placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                      rows={6}
                    />
                  </LabeledField>
                  <LabeledField label="Passphrase">
                    <Input
                      type="password"
                      value={sshCredentialDraft.passphrase || ''}
                      onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, passphrase: event.target.value }))}
                      disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                      placeholder="Optional private key passphrase"
                    />
                  </LabeledField>
                </div>
              )}
              <LabeledField label="Expires at" hint="Optional RFC3339 timestamp for expiry tracking.">
                <Input
                  value={sshCredentialDraft.expires_at || ''}
                  onChange={(event) => setSSHCredentialDraft((current) => ({ ...current, expires_at: event.target.value }))}
                  disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps}
                  placeholder="2026-05-01T00:00:00Z"
                />
              </LabeledField>
              <div className="flex flex-wrap items-center gap-3">
                <Button type="button" disabled={secretsLoading || secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps} onClick={editingSSHCredentialID ? handleSSHCredentialReplace : handleSSHCredentialCreate}>
                  {editingSSHCredentialID ? 'Replace SSH Credential' : 'Store SSH Credential'}
                </Button>
                {editingSSHCredentialID ? (
                  <Button type="button" variant="outline" disabled={secretsLoading || secretsSaving} onClick={() => {
                    setEditingSSHCredentialID(null);
                    setSSHCredentialDraft(emptySSHCredentialDraft());
                  }}>
                    Cancel Edit
                  </Button>
                ) : null}
                <span className="text-xs text-muted-foreground">Metadata is kept in PostgreSQL; secret material is encrypted separately.</span>
              </div>
              <CollapsibleList
                items={sshCredentials.map((item) => (
                  <div key={item.credential_id} className="flex flex-wrap items-start justify-between gap-4 border-b border-border/50 pb-2">
                    <div>
                      <div className="font-semibold text-foreground">{item.display_name || item.credential_id}</div>
                      <div className="text-sm text-muted-foreground">{item.connector_id || 'ssh'} · {item.username || 'user'} · {item.auth_type} · {item.host_scope || 'all hosts'}</div>
                      <div className="text-xs text-muted-foreground">Last rotated: {item.last_rotated_at ? new Date(item.last_rotated_at).toLocaleString() : 'never recorded'}</div>
                      <div className="text-xs text-muted-foreground">Expires: {item.expires_at ? new Date(item.expires_at).toLocaleString() : 'not set'}</div>
                    </div>
                    <div className="flex flex-wrap items-center justify-end gap-2">
                      <span className={cn('text-sm font-semibold', item.status === 'active' ? 'text-success' : 'text-warning')}>{item.status}</span>
                      <Button variant="outline" size="sm" type="button" disabled={secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps} onClick={() => startSSHCredentialEdit(item)}>
                        Replace
                      </Button>
                      {item.status !== 'rotation_required' ? (
                        <Button variant="outline" size="sm" type="button" disabled={secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps} onClick={() => handleSSHCredentialStatus(item.credential_id, 'rotation_required')}>
                          Mark rotation required
                        </Button>
                      ) : null}
                      <Button variant="outline" size="sm" type="button" disabled={secretsSaving || !canSaveSSHCredentialOps} onClick={() => handleSSHCredentialStatus(item.credential_id, item.status === 'active' ? 'disabled' : 'active')}>
                        {item.status === 'active' ? 'Disable' : 'Enable'}
                      </Button>
                      <Button variant="destructive" size="sm" type="button" disabled={secretsSaving || !sshCredentialsConfigured || !canSaveSSHCredentialOps} onClick={() => setPendingDeleteSSHCredentialID(item.credential_id)}>
                        Delete
                      </Button>
                    </div>
                  </div>
                ))}
              />
              <ConfirmActionDialog
                open={pendingDeleteSSHCredentialID !== null}
                onOpenChange={(open) => {
                  if (!open) {
                    setPendingDeleteSSHCredentialID(null);
                  }
                }}
                title="Delete SSH Credential"
                description="Delete this SSH credential from encrypted custody. Secret material stays write-only and is not recoverable from the UI."
                confirmLabel="Delete SSH Credential"
                danger
                loading={secretsSaving}
                onConfirm={() => {
                  void handleSSHCredentialDelete();
                }}
              />
            </div>
          </PanelCard>
          ) : null}

          {canViewSecretsInventory ? (
          <PanelCard title={t('ops.secrets.referenced')} subtitle={t('ops.secrets.referencedDesc')}>
            <CollapsibleList 
              items={(secretsInventory || []).map((item) => (
                <div key={`${item.ref}-${item.owner_id}-${item.key}`} className="flex flex-wrap items-start justify-between gap-4 border-b border-border/50 pb-2">
                  <div>
                    <div className="font-semibold text-foreground">{item.owner_id || item.ref || 'secret ref'}</div>
                    <div className="text-sm text-muted-foreground">{item.owner_type || 'owner'} · {item.key || 'key'} · {item.ref || 'ref'}</div>
                  </div>
                  <span className={cn('text-sm font-semibold', item.set ? 'text-success' : 'text-danger')}>{item.set ? t('ops.secrets.set') : t('ops.secrets.missing')}</span>
                </div>
              )) }
            />
          </PanelCard>
          ) : null}
        </div>
      </ConfigSection>
      )}

      {resolvedActiveTab === 'providers' && (
      <ConfigSection
        icon={<Waves size={20} />}
        title={t('ops.providers.title')}
        description={t('ops.providers.desc')}
        meta={{
          configured: providersConfigured,
          loaded: providersLoaded,
          path: providersPath,
          updatedAt: providersUpdatedAt,
        }}
        mode={providersMode}
        setMode={setProvidersMode}
        loading={providersLoading}
        saving={providersSaving}
        message={providersMessage}
        onSave={handleProvidersSave}
        saveLabel={t('ops.providers.saveLabel')}
        canSave={canSaveConfigOps}
        showModeSwitch={false}
      >
        <div className={panelGridClass}>
          <PanelCard title={t('ops.providers.handoffTitle')} subtitle={t('ops.providers.handoffDesc')}>
            <div className="flex flex-wrap gap-3">
              <Button variant="outline" asChild>
                <Link to="/providers">{t('ops.providers.openObjectPage')}</Link>
              </Button>
              <Button variant="outline" asChild>
                <Link to="/ops?tab=secrets#secret-inventory">{t('ops.providers.openSecrets')}</Link>
              </Button>
            </div>
          </PanelCard>

          <PanelCard title={t('ops.providers.advancedTitle')} subtitle={t('ops.providers.advancedDesc')}>
            <div className="grid gap-2 text-sm text-muted-foreground">
              <p>{t('ops.providers.advancedBullet1')}</p>
              <p>{t('ops.providers.advancedBullet2')}</p>
            </div>
          </PanelCard>

          <YamlEditor
            label={t('ops.providers.yamlLabel')}
            value={providersContent}
            onChange={setProvidersContent}
            disabled={providersLoading || providersSaving}
            placeholder="providers:\n  primary:\n    provider_id: openrouter-main\n    model: openai/gpt-4.1-mini"
          />
        </div>
      </ConfigSection>
      )}

      {resolvedActiveTab === 'connectors' && (
      <ConfigSection
        icon={<Layers size={20} />}
        title={t('ops.connectors.title')}
        description={t('ops.connectors.desc')}
        meta={{
          configured: connectorsConfigured,
          loaded: connectorsLoaded,
          path: connectorsPath,
          updatedAt: connectorsUpdatedAt,
        }}
        mode={connectorsMode}
        setMode={setConnectorsMode}
        loading={connectorsLoading}
        saving={connectorsSaving}
        message={connectorsMessage}
        onSave={handleConnectorsSave}
        saveLabel={t('ops.connectors.saveLabel')}
        canSave={canSaveConfigOps}
        showModeSwitch={false}
      >
        <div className={panelGridClass}>
          <PanelCard title={t('ops.connectors.handoffTitle')} subtitle={t('ops.connectors.handoffDesc')}>
            <div className="flex flex-wrap gap-3">
              <Button variant="outline" asChild>
                <Link to="/connectors">{t('ops.connectors.openObjectPage')}</Link>
              </Button>
              <Button variant="outline" asChild>
                <Link to="/ops?tab=secrets#secret-inventory">{t('ops.connectors.openSecrets')}</Link>
              </Button>
            </div>
          </PanelCard>

          <PanelCard title={t('ops.connectors.advancedTitle')} subtitle={t('ops.connectors.advancedDesc')}>
            <div className="grid gap-2 text-sm text-muted-foreground">
              <p>{t('ops.connectors.advancedBullet1')}</p>
              <p>{t('ops.connectors.advancedBullet2')}</p>
            </div>
          </PanelCard>

          <PanelCard title={t('ops.connectors.samples')} subtitle={t('ops.connectors.samplesDesc')}>
            <div className="flex flex-wrap gap-3">
              {connectorSamples.map((sample) => (
                <Button
                  key={sample.metadata.id}
                  variant="outline"
                  type="button"
                  disabled={connectorsLoading || connectorsSaving}
                  onClick={() => void handleImportConnectorSample(sample)}
                >
                  {t('ops.connectors.import', { name: sample.metadata.display_name || sample.metadata.id || '' })}
                </Button>
              ))}
            </div>
          </PanelCard>

          <YamlEditor
            label={t('ops.connectors.yamlLabel')}
            value={connectorsContent}
            onChange={setConnectorsContent}
            disabled={connectorsLoading || connectorsSaving}
            placeholder="connectors:\n  entries:\n    - api_version: tars.connector/v1alpha1"
          />
        </div>
      </ConfigSection>
      )}

      {resolvedActiveTab === 'prompts' && (
      <ConfigSection
        icon={<Cpu size={20} />}
        title={t('ops.prompts.title')}
        description={t('ops.prompts.desc')}
        meta={{
          configured: promptConfigured,
          loaded: promptLoaded,
          path: promptPath,
          updatedAt: promptUpdatedAt,
        }}
        mode={promptMode}
        setMode={setPromptMode}
        loading={promptLoading}
        saving={promptSaving}
        message={promptMessage}
        onSave={handlePromptSave}
        saveLabel={t('ops.prompts.saveLabel')}
        canSave={canSaveConfigOps}
      >
        {promptMode === 'form' ? (
          <div className={panelGridClass}>
            <LabeledField label={t('ops.prompts.systemPrompt')}>
              <Textarea
                rows={10}
                value={promptConfig.system_prompt}
                onChange={(event) => setPromptConfig((current) => ({ ...current, system_prompt: event.target.value }))}
                disabled={promptLoading || promptSaving}
                className="resize-y"
              />
            </LabeledField>
            <LabeledField label={t('ops.prompts.userPrompt')}>
              <Textarea
                rows={8}
                value={promptConfig.user_prompt_template}
                onChange={(event) => setPromptConfig((current) => ({ ...current, user_prompt_template: event.target.value }))}
                disabled={promptLoading || promptSaving}
                className="resize-y"
              />
            </LabeledField>
            <div className="text-sm leading-6 text-muted-foreground">
              {t('ops.prompts.placeholders')} <code>{'{{ .SessionID }}'}</code>, <code>{'{{ .ContextJSON }}'}</code>, <code>{'{{ .Context }}'}</code>
            </div>
          </div>
        ) : (
          <YamlEditor
            label={t('ops.prompts.yamlLabel')}
            value={promptContent}
            onChange={setPromptContent}
            disabled={promptLoading || promptSaving}
            placeholder="reasoning:\n  system_prompt: |\n    You are TARS..."
          />
        )}
      </ConfigSection>
      )}

      {resolvedActiveTab === 'desense' && (
      <ConfigSection
        icon={<EyeOff size={20} />}
        title={t('ops.desense.title')}
        description={t('ops.desense.desc')}
        meta={{
          configured: desenseConfigured,
          loaded: desenseLoaded,
          path: desensePath,
          updatedAt: desenseUpdatedAt,
        }}
        mode={desenseMode}
        setMode={setDesenseMode}
        loading={desenseLoading}
        saving={desenseSaving}
        message={desenseMessage}
        onSave={handleDesensitizationSave}
        saveLabel={t('ops.desense.saveLabel')}
        canSave={canSaveConfigOps}
      >
        {desenseMode === 'form' ? (
          <div className={panelGridClass}>
            <label className="flex items-center gap-3 text-sm text-muted-foreground">
              <Checkbox
                checked={desenseConfig.enabled}
                onCheckedChange={(value) => setDesenseConfig((current) => ({ ...current, enabled: value === true }))}
                disabled={desenseLoading || desenseSaving}
              />
              {t('ops.desense.enableRule')}
            </label>

            <StringListEditor
              label={t('ops.desense.secretKeys')}
              description={t('ops.desense.secretKeysDesc')}
              values={desenseConfig.secrets.key_names || []}
              onChange={(values) => setDesenseConfig((current) => ({ ...current, secrets: { ...current.secrets, key_names: values } }))}
              disabled={desenseLoading || desenseSaving}
              addLabel={t('ops.desense.secretKeysAdd')}
            />

            <StringListEditor
              label={t('ops.desense.queryKeys')}
              description={t('ops.desense.queryKeysDesc')}
              values={desenseConfig.secrets.query_key_names || []}
              onChange={(values) => setDesenseConfig((current) => ({ ...current, secrets: { ...current.secrets, query_key_names: values } }))}
              disabled={desenseLoading || desenseSaving}
              addLabel={t('ops.desense.queryKeysAdd')}
            />

            <StringListEditor
              label={t('ops.desense.additionalPatterns')}
              description={t('ops.desense.additionalPatternsDesc')}
              values={desenseConfig.secrets.additional_patterns || []}
              onChange={(values) => setDesenseConfig((current) => ({ ...current, secrets: { ...current.secrets, additional_patterns: values } }))}
              disabled={desenseLoading || desenseSaving}
              addLabel={t('ops.desense.additionalPatternsAdd')}
            />

            <div className={autoFitGridClass}>
              <CheckboxField
                label={t('ops.desense.redactBearer')}
                checked={desenseConfig.secrets.redact_bearer}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, secrets: { ...current.secrets, redact_bearer: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
              <CheckboxField
                label={t('ops.desense.redactBasicAuth')}
                checked={desenseConfig.secrets.redact_basic_auth_url}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, secrets: { ...current.secrets, redact_basic_auth_url: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
              <CheckboxField
                label={t('ops.desense.redactSk')}
                checked={desenseConfig.secrets.redact_sk_tokens}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, secrets: { ...current.secrets, redact_sk_tokens: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
            </div>

            <StringListEditor
              label={t('ops.desense.hostFragments')}
              description={t('ops.desense.hostFragmentsDesc')}
              values={desenseConfig.placeholders.host_key_fragments || []}
              onChange={(values) => setDesenseConfig((current) => ({ ...current, placeholders: { ...current.placeholders, host_key_fragments: values } }))}
              disabled={desenseLoading || desenseSaving}
              addLabel={t('ops.desense.hostFragmentsAdd')}
            />

            <StringListEditor
              label={t('ops.desense.pathFragments')}
              description={t('ops.desense.pathFragmentsDesc')}
              values={desenseConfig.placeholders.path_key_fragments || []}
              onChange={(values) => setDesenseConfig((current) => ({ ...current, placeholders: { ...current.placeholders, path_key_fragments: values } }))}
              disabled={desenseLoading || desenseSaving}
              addLabel={t('ops.desense.pathFragmentsAdd')}
            />

            <div className={autoFitGridClass}>
              <CheckboxField
                label={t('ops.desense.replaceIp')}
                checked={desenseConfig.placeholders.replace_inline_ip}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, placeholders: { ...current.placeholders, replace_inline_ip: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
              <CheckboxField
                label={t('ops.desense.replaceHost')}
                checked={desenseConfig.placeholders.replace_inline_host}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, placeholders: { ...current.placeholders, replace_inline_host: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
              <CheckboxField
                label={t('ops.desense.replacePath')}
                checked={desenseConfig.placeholders.replace_inline_path}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, placeholders: { ...current.placeholders, replace_inline_path: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
            </div>

            <div className={autoFitGridClass}>
              <CheckboxField
                label={t('ops.desense.rehydrateHost')}
                checked={desenseConfig.rehydration.host}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, rehydration: { ...current.rehydration, host: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
              <CheckboxField
                label={t('ops.desense.rehydrateIp')}
                checked={desenseConfig.rehydration.ip}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, rehydration: { ...current.rehydration, ip: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
              <CheckboxField
                label={t('ops.desense.rehydratePath')}
                checked={desenseConfig.rehydration.path}
                onChange={(checked) => setDesenseConfig((current) => ({ ...current, rehydration: { ...current.rehydration, path: checked } }))}
                disabled={desenseLoading || desenseSaving}
              />
            </div>

            <PanelCard title={t('ops.desense.llmAssist')} subtitle={t('ops.desense.llmAssistDesc')}>
              <div className={panelGridClass}>
                <CheckboxField
                  label={t('ops.desense.llmAssistEnable')}
                  checked={desenseConfig.local_llm_assist.enabled}
                  onChange={(checked) => setDesenseConfig((current) => ({ ...current, local_llm_assist: { ...current.local_llm_assist, enabled: checked } }))}
                  disabled={desenseLoading || desenseSaving}
                />
                <div className={autoFitGridClass}>
                  <LabeledField label={t('ops.desense.provider')}>
                    <Input
                      value={desenseConfig.local_llm_assist.provider || ''}
                      onChange={(event) => setDesenseConfig((current) => ({ ...current, local_llm_assist: { ...current.local_llm_assist, provider: event.target.value } }))}
                      disabled={desenseLoading || desenseSaving}
                    />
                  </LabeledField>
                  <LabeledField label={t('ops.desense.mode')}>
                    <NativeSelect
                      value={desenseConfig.local_llm_assist.mode || 'detect_only'}
                      onChange={(event) => setDesenseConfig((current) => ({ ...current, local_llm_assist: { ...current.local_llm_assist, mode: event.target.value } }))}
                      disabled={desenseLoading || desenseSaving}
                      className="bg-background"
                    >
                      <option value="detect_only">detect_only</option>
                      <option value="classify_only">classify_only</option>
                    </NativeSelect>
                  </LabeledField>
                  <LabeledField label={t('ops.desense.baseUrl')}>
                    <Input
                      value={desenseConfig.local_llm_assist.base_url || ''}
                      onChange={(event) => setDesenseConfig((current) => ({ ...current, local_llm_assist: { ...current.local_llm_assist, base_url: event.target.value } }))}
                      disabled={desenseLoading || desenseSaving}
                    />
                  </LabeledField>
                  <LabeledField label={t('ops.desense.model')}>
                    <Input
                      value={desenseConfig.local_llm_assist.model || ''}
                      onChange={(event) => setDesenseConfig((current) => ({ ...current, local_llm_assist: { ...current.local_llm_assist, model: event.target.value } }))}
                      disabled={desenseLoading || desenseSaving}
                    />
                  </LabeledField>
                </div>
              </div>
            </PanelCard>
          </div>
        ) : (
          <YamlEditor
            label={t('ops.desense.yamlLabel')}
            value={desenseContent}
            onChange={setDesenseContent}
            disabled={desenseLoading || desenseSaving}
            placeholder="desensitization:\n  enabled: true\n  rehydration:\n    host: true"
          />
        )}
      </ConfigSection>
      )}

      {resolvedActiveTab === 'advanced' && (
      <Card className="max-w-[720px] p-6">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-2">
            <h3 className="text-lg font-semibold text-danger">{t('ops.advanced.reindexTitle')}</h3>
            <p className="text-sm leading-6 text-muted-foreground">
              {t('ops.advanced.reindexDesc')}
            </p>
          </div>

          {reindexMessage && <StatusMessage message={reindexMessage} />}

          <div className="flex justify-end">
            <Button
              variant="destructive"
              onClick={() => setReindexConfirmOpen(true)}
              disabled={reindexLoading}
            >
              {reindexLoading ? t('ops.advanced.triggering') : t('ops.advanced.trigger')}
            </Button>
          </div>
        </div>

        <ConfirmActionDialog
          open={reindexConfirmOpen}
          onOpenChange={setReindexConfirmOpen}
          title={t('ops.advanced.reindexTitle')}
          description={t('ops.advanced.reindexConfirm')}
          confirmLabel={t('ops.advanced.trigger')}
          onConfirm={() => {
            setReindexConfirmOpen(false);
            void handleReindex();
          }}
        />
      </Card>
      )}
    </div>
  );
};

type ConfigSectionProps = {
  anchorId?: string;
  title: string;
  description: string;
  meta: {
    configured: boolean;
    loaded: boolean;
    path?: string;
    updatedAt?: string;
  };
  mode: EditorMode;
  setMode: (mode: EditorMode) => void;
  loading: boolean;
  saving: boolean;
  message: FlashMessage | null;
  onSave: () => void;
  saveLabel: string;
  canSave?: boolean;
  children: ReactNode;
  showModeSwitch?: boolean;
};

const panelGridClass = 'grid gap-4';
const autoFitGridClass = 'grid gap-4 [grid-template-columns:repeat(auto-fit,minmax(220px,1fr))]';

const ConfigSection = ({
  anchorId,
  title,
  description,
  icon,
  meta,
  mode,
  setMode,
  loading,
  saving,
  message,
  onSave,
  saveLabel,
  canSave = true,
  children,
  showModeSwitch = true,
}: ConfigSectionProps & { icon?: ReactNode }) => {
  const { t } = useI18n();
  return (
  <Card id={anchorId} className="overflow-hidden p-0">
    <CardHeader className="gap-4 border-b border-border/50 bg-white/[0.02] p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <CardTitle className="flex items-center gap-3 text-xl text-foreground">
            {icon ? <span className="text-primary">{icon}</span> : null}
            {title}
          </CardTitle>
          <CardDescription className="max-w-4xl text-sm leading-6">
            {description}
          </CardDescription>
        </div>
        {showModeSwitch ? <ModeSwitch mode={mode} setMode={setMode} /> : null}
      </div>
      <MetaRow {...meta} />
    </CardHeader>

    <CardContent className="flex min-h-[100px] flex-col gap-4 p-6">
      {children}
      {message ? <StatusMessage message={message} /> : null}
    </CardContent>

    {canSave ? (
    <CardFooter className="justify-end border-t border-border/50 p-6 pt-6">
      <Button
        variant="amber"
        onClick={onSave}
        disabled={loading || saving}
        className="min-w-36"
      >
        {saving ? t('ops.saving') : saveLabel}
      </Button>
    </CardFooter>
    ) : null}
  </Card>
  );
};

const MetaRow = ({ configured, loaded, path, updatedAt }: { configured: boolean; loaded: boolean; path?: string; updatedAt?: string }) => {
  const { t } = useI18n();
  return (
  <div className="flex flex-wrap gap-4 rounded-xl border border-border/50 bg-background/40 px-4 py-3 text-xs text-muted-foreground">
    <span className="flex items-center gap-2">
      <Settings size={14} />
      {t('ops.meta.configured')}
      <strong className={cn(configured ? 'text-success' : 'text-muted-foreground')}>
        {configured ? t('ops.meta.configuredActive') : t('ops.meta.configuredMissing')}
      </strong>
    </span>
    <span className="flex items-center gap-2">
      <Database size={14} />
      {t('ops.meta.loaded')}
      <strong className={cn(loaded ? 'text-success' : 'text-danger')}>
        {loaded ? t('ops.meta.loadedSuccess') : t('ops.meta.loadedReady')}
      </strong>
    </span>
    {path ? (
      <span className="flex items-center gap-2">
        <Terminal size={14} />
        <code className="truncate font-mono text-[11px] text-foreground/80">{path}</code>
      </span>
    ) : null}
    {updatedAt ? (
      <span className="flex items-center gap-2">
        <Clock size={14} />
        {new Date(updatedAt).toLocaleString()}
      </span>
    ) : null}
  </div>
  );
};

const ModeSwitch = ({ mode, setMode }: { mode: EditorMode; setMode: (mode: EditorMode) => void }) => {
  const { t } = useI18n();
  return (
  <div className="inline-flex rounded-lg border border-border/60 bg-background/60 p-1">
    <Button
      variant="ghost"
      size="sm"
      type="button"
      onClick={() => setMode('form')}
      className={cn(
        'px-3 text-xs',
        mode === 'form'
          ? 'bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 hover:text-primary-foreground'
          : 'text-muted-foreground'
      )}
    >
      {t('ops.mode.form')}
    </Button>
    <Button
      variant="ghost"
      size="sm"
      type="button"
      onClick={() => setMode('yaml')}
      className={cn(
        'px-3 text-xs',
        mode === 'yaml'
          ? 'bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 hover:text-primary-foreground'
          : 'text-muted-foreground'
      )}
    >
      {t('ops.mode.yaml')}
    </Button>
  </div>
  );
};

function updateSecretDraft(entries: SecretValueInput[], index: number, next: SecretValueInput): SecretValueInput[] {
  return entries.map((item, itemIndex) => (itemIndex === index ? next : item));
}

const ActionSelect = ({
  value,
  onChange,
  disabled,
}: {
  value: AuthorizationAction;
  onChange: (value: AuthorizationAction) => void;
  disabled: boolean;
}) => (
  <NativeSelect
    value={value}
    onChange={(event) => onChange(event.target.value as AuthorizationAction)}
    disabled={disabled}
    className="bg-background"
  >
    {actionOptions.map((item) => (
      <option key={item} value={item}>{item}</option>
    ))}
  </NativeSelect>
);

const CheckboxField = ({
  label,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled: boolean;
}) => (
  <label className="flex items-start gap-3 rounded-lg border border-border/60 bg-background/30 px-3 py-2 text-sm text-foreground">
    <Checkbox
      checked={checked}
      onCheckedChange={(value) => onChange(value === true)}
      disabled={disabled}
    />
    <span className="leading-5 text-muted-foreground">{label}</span>
  </label>
);

const StringListEditor = ({
  label,
  description,
  values,
  onChange,
  disabled,
  addLabel,
}: {
  label: string;
  description: string;
  values: string[];
  onChange: (values: string[]) => void;
  disabled: boolean;
  addLabel: string;
}) => {
  const { t } = useI18n();
  return (
  <PanelCard title={label} subtitle={description}>
    <div className="grid gap-3">
      {values.map((value, index) => (
        <div key={`${label}-${index}`} className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
          <Input
            value={value}
            onChange={(event) => onChange(values.map((item, itemIndex) => itemIndex === index ? event.target.value : item))}
            disabled={disabled}
            placeholder="e.g., systemctl status *"
          />
          <Button variant="outline" type="button" disabled={disabled} onClick={() => onChange(values.filter((_, itemIndex) => itemIndex !== index))}>
            {t('ops.remove')}
          </Button>
        </div>
      ))}
      <div>
        <Button variant="outline" type="button" disabled={disabled} onClick={() => onChange([...values, ''])}>
          {addLabel}
        </Button>
      </div>
    </div>
  </PanelCard>
  );
};

const OverrideEditor = ({
  overrides,
  onChange,
  disabled,
}: {
  overrides: AuthorizationOverrideConfig[];
  onChange: (overrides: AuthorizationOverrideConfig[]) => void;
  disabled: boolean;
}) => {
  const { t } = useI18n();
  return (
  <PanelCard
    title={t('ops.auth.overrides')}
    subtitle={t('ops.auth.overridesDesc')}
  >
    {overrides.map((override, index) => (
      <Card key={`override-${index}`} className="border border-border/60 bg-background/30 p-4">
        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,240px)]">
          <LabeledField label={t('ops.auth.overrideRuleId')}>
            <Input
              value={override.id || ''}
              onChange={(event) => onChange(updateOverride(overrides, index, { ...override, id: event.target.value }))}
              disabled={disabled}
              placeholder="optional-id"
            />
          </LabeledField>
          <LabeledField label={t('ops.auth.overrideAction')}>
            <ActionSelect
              value={override.action || 'require_approval'}
              onChange={(value) => onChange(updateOverride(overrides, index, { ...override, action: value }))}
              disabled={disabled}
            />
          </LabeledField>
        </div>
        <div className={autoFitGridClass}>
          <LabeledField label={t('ops.auth.overrideServices')}>
            <Input
              value={joinValues(override.services)}
              onChange={(event) => onChange(updateOverride(overrides, index, { ...override, services: splitValues(event.target.value) }))}
              disabled={disabled}
              placeholder="sshd, nginx"
            />
          </LabeledField>
          <LabeledField label={t('ops.auth.overrideHosts')}>
            <Input
              value={joinValues(override.hosts)}
              onChange={(event) => onChange(updateOverride(overrides, index, { ...override, hosts: splitValues(event.target.value) }))}
              disabled={disabled}
              placeholder="192.168.3.106, web-*"
            />
          </LabeledField>
          <LabeledField label={t('ops.auth.overrideChannels')}>
            <Input
              value={joinValues(override.channels)}
              onChange={(event) => onChange(updateOverride(overrides, index, { ...override, channels: splitValues(event.target.value) }))}
              disabled={disabled}
              placeholder="telegram_chat, vmalert"
            />
          </LabeledField>
          <LabeledField label={t('ops.auth.overrideCommandGlobs')}>
            <Input
              value={joinValues(override.command_globs)}
              onChange={(event) => onChange(updateOverride(overrides, index, { ...override, command_globs: splitValues(event.target.value) }))}
              disabled={disabled}
              placeholder="systemctl restart sshd, systemctl restart nginx"
            />
          </LabeledField>
        </div>
        <div className="flex justify-end">
          <Button variant="outline" type="button" disabled={disabled} onClick={() => onChange(overrides.filter((_, itemIndex) => itemIndex !== index))}>
            {t('ops.auth.removeOverride')}
          </Button>
        </div>
      </Card>
    ))}
    <div>
      <Button
        variant="outline"
        type="button"
        disabled={disabled}
        onClick={() => onChange([...overrides, { id: '', action: 'require_approval', services: [], hosts: [], channels: [], command_globs: [] }])}
      >
        {t('ops.auth.addOverride')}
      </Button>
    </div>
  </PanelCard>
  );
};

const RouteEntryEditor = ({
  label,
  description,
  values,
  onChange,
  disabled,
  keyPlaceholder,
  valuesPlaceholder,
  addLabel,
}: {
  label: string;
  description: string;
  values: RouteEntry[];
  onChange: (values: RouteEntry[]) => void;
  disabled: boolean;
  keyPlaceholder: string;
  valuesPlaceholder: string;
  addLabel: string;
}) => {
  const { t } = useI18n();
  return (
  <PanelCard title={label} subtitle={description}>
    {values.map((entry, index) => (
      <div key={`${label}-${index}`} className="grid gap-3 lg:grid-cols-[minmax(180px,220px)_minmax(0,1fr)_auto]">
        <Input
          value={entry.key}
          onChange={(event) => onChange(updateRouteEntry(values, index, { ...entry, key: event.target.value }))}
          disabled={disabled}
          placeholder={keyPlaceholder}
        />
        <Input
          value={joinValues(entry.targets)}
          onChange={(event) => onChange(updateRouteEntry(values, index, { ...entry, targets: splitValues(event.target.value) }))}
          disabled={disabled}
          placeholder={valuesPlaceholder}
        />
        <Button variant="outline" type="button" disabled={disabled} onClick={() => onChange(values.filter((_, itemIndex) => itemIndex !== index))}>
          {t('ops.remove')}
        </Button>
      </div>
    ))}
    <div>
      <Button variant="outline" type="button" disabled={disabled} onClick={() => onChange([...values, { key: '', targets: [] }])}>
        {addLabel}
      </Button>
    </div>
  </PanelCard>
  );
};

function YamlEditor({
  label,
  value,
  onChange,
  disabled,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
  placeholder: string;
}) {
  return (
    <div className="flex flex-col gap-2">
      <Label>{label}</Label>
      <Textarea
        rows={18}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        placeholder={placeholder}
        className="min-h-[26rem] resize-y font-mono text-[0.85rem] leading-6"
      />
    </div>
  );
}

function StatusMessage({ message }: { message: FlashMessage }) {
  const { t } = useI18n();
  return (
    <InlineStatus
      type={message.type === 'error' ? 'error' : 'success'}
      message={`${message.type === 'error' ? t('ops.warning') : t('ops.success')}: ${message.text}`}
    />
  );
}

function splitValues(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

function joinValues(values?: string[]): string {
  return (values || []).join(', ');
}

function updateOverride(overrides: AuthorizationOverrideConfig[], index: number, next: AuthorizationOverrideConfig): AuthorizationOverrideConfig[] {
  return overrides.map((item, itemIndex) => (itemIndex === index ? next : item));
}

function updateRouteEntry(entries: RouteEntry[], index: number, next: RouteEntry): RouteEntry[] {
  return entries.map((item, itemIndex) => (itemIndex === index ? next : item));
}
