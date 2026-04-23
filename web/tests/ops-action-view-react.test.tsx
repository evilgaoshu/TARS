// @vitest-environment jsdom

import { act } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { MemoryRouter } from 'react-router-dom';
import type { SSHCredential } from '../src/lib/api/types';

const resolvedConfig = {
  content: '',
  path: '/tmp/config.yaml',
  updated_at: '2026-03-29T08:00:00Z',
  loaded: true,
  configured: true,
};

const fetchApprovalRoutingConfigMock = vi.fn();
const fetchAuthorizationConfigMock = vi.fn();
const fetchConnectorsConfigMock = vi.fn();
const fetchDesensitizationConfigMock = vi.fn();
const fetchProvidersConfigMock = vi.fn();
const fetchReasoningPromptConfigMock = vi.fn();
const fetchSecretsInventoryMock = vi.fn();
const fetchSSHCredentialsMock = vi.fn();
const createSSHCredentialMock = vi.fn();
const updateSSHCredentialMock = vi.fn();
const deleteSSHCredentialMock = vi.fn();
const setSSHCredentialStatusMock = vi.fn();
const triggerReindexMock = vi.fn();
const useAuthMock = vi.fn();

vi.mock('../src/lib/api/ops', () => {
  return {
    fetchApprovalRoutingConfig: fetchApprovalRoutingConfigMock,
    fetchAuthorizationConfig: fetchAuthorizationConfigMock,
    fetchConnectorsConfig: fetchConnectorsConfigMock,
    fetchDesensitizationConfig: fetchDesensitizationConfigMock,
    fetchProvidersConfig: fetchProvidersConfigMock,
    fetchReasoningPromptConfig: fetchReasoningPromptConfigMock,
    fetchSecretsInventory: fetchSecretsInventoryMock,
    fetchSSHCredentials: fetchSSHCredentialsMock,
    getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
    createSSHCredential: createSSHCredentialMock,
    updateSSHCredential: updateSSHCredentialMock,
    deleteSSHCredential: deleteSSHCredentialMock,
    importConnectorManifest: vi.fn(),
    setSSHCredentialStatus: setSSHCredentialStatusMock,
    triggerReindex: triggerReindexMock,
    updateSecretsInventory: vi.fn(),
    updateApprovalRoutingConfig: vi.fn(),
    updateAuthorizationConfig: vi.fn(),
    updateConnectorsConfig: vi.fn(),
    updateDesensitizationConfig: vi.fn(),
    updateProvidersConfig: vi.fn(),
    updateReasoningPromptConfig: vi.fn(),
  };
});

vi.mock('../src/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}));

vi.mock('../src/hooks/useI18n', () => {
  return {
    useI18n: () => ({
      t: (key: string) => key,
    }),
  };
});

async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}

async function setInputValue(input: HTMLInputElement | HTMLTextAreaElement, value: string) {
  await act(async () => {
    const prototype = input instanceof window.HTMLTextAreaElement
      ? window.HTMLTextAreaElement.prototype
      : window.HTMLInputElement.prototype;
    const setter = Object.getOwnPropertyDescriptor(prototype, 'value')?.set;
    setter?.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
  });
}

async function renderOpsSecretsView() {
  const { OpsActionView } = await import('../src/pages/ops/OpsActionView');
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);

  await act(async () => {
    root.render(
      <MemoryRouter initialEntries={['/?tab=secrets']}>
        <OpsActionView />
      </MemoryRouter>,
    );
  });
  await flush();
  await flush();

  return { container, root };
}

async function cleanupRenderedView(root: { unmount: () => void }, container: HTMLElement) {
  await act(async () => {
    root.unmount();
  });
  container.remove();
  document.body.replaceChildren();
}

describe('OpsActionView', () => {
  beforeEach(() => {
    (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    fetchApprovalRoutingConfigMock.mockReset();
    fetchApprovalRoutingConfigMock.mockResolvedValue({ ...resolvedConfig, config: { prohibit_self_approval: true, service_owners: [], oncall_groups: [], command_allowlist: [] } });
    fetchAuthorizationConfigMock.mockReset();
    fetchAuthorizationConfigMock.mockResolvedValue({ ...resolvedConfig, config: { whitelist_action: 'direct_execute', blacklist_action: 'suggest_only', unmatched_action: 'require_approval', normalize_whitespace: true, hard_deny_ssh_command: [], hard_deny_mcp_skill: [], whitelist: [], blacklist: [], overrides: [] } });
    fetchConnectorsConfigMock.mockReset();
    fetchConnectorsConfigMock.mockResolvedValue({ ...resolvedConfig, config: {} });
    fetchDesensitizationConfigMock.mockReset();
    fetchDesensitizationConfigMock.mockResolvedValue({ ...resolvedConfig, config: { enabled: true, secrets: { key_names: [], query_key_names: [], additional_patterns: [], redact_bearer: true, redact_basic_auth_url: true, redact_sk_tokens: true }, placeholders: { host_key_fragments: [], path_key_fragments: [], replace_inline_ip: true, replace_inline_host: true, replace_inline_path: true }, rehydration: { host: true, ip: true, path: true }, local_llm_assist: { enabled: false, provider: 'openai_compatible', base_url: '', model: '', mode: 'detect_only' } } });
    fetchProvidersConfigMock.mockReset();
    fetchProvidersConfigMock.mockResolvedValue({ ...resolvedConfig, config: {} });
    fetchReasoningPromptConfigMock.mockReset();
    fetchReasoningPromptConfigMock.mockResolvedValue({ ...resolvedConfig, config: { system_prompt: '', user_prompt_template: '' } });
    fetchSecretsInventoryMock.mockReset();
    fetchSecretsInventoryMock.mockResolvedValue({ configured: true, loaded: true, path: '/tmp/secrets.yaml', updated_at: '2026-03-29T08:00:00Z', custody_configured: true, custody_key_id: 'local', ssh_credential_configured: true, items: [] });
    fetchSSHCredentialsMock.mockReset();
    fetchSSHCredentialsMock.mockResolvedValue({ configured: false, items: [] });
    createSSHCredentialMock.mockReset();
    updateSSHCredentialMock.mockReset();
    deleteSSHCredentialMock.mockReset();
    setSSHCredentialStatusMock.mockReset();
    triggerReindexMock.mockReset();
    triggerReindexMock.mockResolvedValue({});
    useAuthMock.mockReset();
    useAuthMock.mockReturnValue({
      isAuthenticated: true,
      user: { permissions: ['*'] },
    });
  });

  it('shows repair-console framing and quick links', async () => {
    const { OpsActionView } = await import('../src/pages/ops/OpsActionView');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <OpsActionView />
        </MemoryRouter>,
      );
    });
    await flush();
    await flush();

    expect(container.textContent).toContain('ops.title');
    expect(container.textContent).toContain('ops.subtitle');
    expect(container.textContent).toContain('ops.repairConsole.badge');
    expect(container.textContent).toContain('Logs');
    expect(container.textContent).toContain('Observability');
    expect(container.textContent).toContain('Audit Trail');

    await cleanupRenderedView(root, container);
  });

  it('lands on the repair tab first and does not eagerly load config documents', async () => {
    const { OpsActionView } = await import('../src/pages/ops/OpsActionView');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <OpsActionView />
        </MemoryRouter>,
      );
    });
    await flush();
    await flush();

    expect(container.textContent).toContain('ops.repairConsole.badge');
    expect(container.textContent).toContain('ops.advanced.trigger');
    expect(fetchAuthorizationConfigMock).not.toHaveBeenCalled();
    expect(fetchApprovalRoutingConfigMock).not.toHaveBeenCalled();
    expect(fetchConnectorsConfigMock).not.toHaveBeenCalled();
    expect(fetchProvidersConfigMock).not.toHaveBeenCalled();
    expect(fetchReasoningPromptConfigMock).not.toHaveBeenCalled();
    expect(fetchDesensitizationConfigMock).not.toHaveBeenCalled();
    expect(fetchSecretsInventoryMock).not.toHaveBeenCalled();
    expect(fetchSSHCredentialsMock).not.toHaveBeenCalled();

    await cleanupRenderedView(root, container);
  });

  it('loads config documents only after switching into a config tab', async () => {
    const { OpsActionView } = await import('../src/pages/ops/OpsActionView');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter>
          <OpsActionView />
        </MemoryRouter>,
      );
    });
    await flush();
    await flush();

    const authTab = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('ops.tab.auth'));
    expect(authTab).toBeTruthy();

    await act(async () => {
      authTab?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();
    await flush();

    expect(fetchAuthorizationConfigMock).toHaveBeenCalledTimes(1);
    expect(fetchApprovalRoutingConfigMock).not.toHaveBeenCalled();
    expect(fetchConnectorsConfigMock).not.toHaveBeenCalled();
    expect(fetchProvidersConfigMock).not.toHaveBeenCalled();
    expect(fetchReasoningPromptConfigMock).not.toHaveBeenCalled();
    expect(fetchDesensitizationConfigMock).not.toHaveBeenCalled();

    await cleanupRenderedView(root, container);
  });

  it('shows reindex confirm dialog when trigger button clicked — does not call window.confirm', async () => {
    const confirmSpy = vi.fn(() => true);
    vi.stubGlobal('confirm', confirmSpy);

    const { OpsActionView } = await import('../src/pages/ops/OpsActionView');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter initialEntries={['/?tab=advanced']}>
          <OpsActionView />
        </MemoryRouter>,
      );
    });
    await flush();
    await flush();

    // Click the reindex button
    const reindexBtn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('ops.advanced.trigger'),
    );
    expect(reindexBtn).toBeTruthy();

    await act(async () => {
      reindexBtn?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();

    // window.confirm must NOT have been called
    expect(confirmSpy).not.toHaveBeenCalled();
    // triggerReindex should NOT have been called yet (dialog not confirmed)
    expect(triggerReindexMock).not.toHaveBeenCalled();

    vi.unstubAllGlobals();
    await cleanupRenderedView(root, container);
  });

  it('does not render ssh custody secret refs in the ops view', async () => {
    const opsApi = await import('../src/lib/api/ops');
    vi.mocked(opsApi.fetchSSHCredentials).mockResolvedValueOnce({
      configured: true,
      items: [
        {
          credential_id: 'ops-key',
          display_name: 'Ops key',
          connector_id: 'ssh-main',
          username: 'root',
          auth_type: 'password',
          host_scope: '192.168.3.100',
          status: 'active',
          secret_ref: 'ssh/ssh-main/ops-key/material',
        } as SSHCredential & { secret_ref?: string },
      ],
    });

    const { OpsActionView } = await import('../src/pages/ops/OpsActionView');
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    await act(async () => {
      root.render(
        <MemoryRouter initialEntries={['/?tab=secrets']}>
          <OpsActionView />
        </MemoryRouter>,
      );
    });
    await flush();
    await flush();

    expect(container.textContent).toContain('Ops key');
    expect(container.textContent).not.toContain('ssh/ssh-main/ops-key/material');

    await cleanupRenderedView(root, container);
  });

  it('renders ssh credential governance metadata in the secrets tab', async () => {
    fetchSecretsInventoryMock.mockResolvedValueOnce({
      configured: true,
      loaded: true,
      path: '/tmp/secrets.yaml',
      updated_at: '2026-03-29T08:00:00Z',
      custody_configured: true,
      custody_key_id: 'key-2026-04',
      ssh_credential_configured: true,
      items: [
        { owner_type: 'ssh_credential', owner_id: 'ops-key', key: 'material', set: false, status: 'rotation_required' },
        { owner_type: 'ssh_credential', owner_id: 'missing-key', key: 'material', set: false, status: 'missing' },
      ],
    });
    fetchSSHCredentialsMock.mockResolvedValueOnce({
      configured: true,
      items: [
        {
          credential_id: 'ops-key',
          display_name: 'Ops key',
          connector_id: 'ssh-main',
          username: 'root',
          auth_type: 'password',
          host_scope: '192.168.3.100',
          status: 'rotation_required',
          last_rotated_at: '2026-04-01T12:34:56Z',
          expires_at: '2026-05-01T00:00:00Z',
        },
      ],
    });

    const { container, root } = await renderOpsSecretsView();

    expect(container.textContent).toContain('Ops key');
    expect(container.textContent).toContain('rotation_required');
    expect(container.textContent).toContain('key-2026-04');
    expect(container.textContent).toContain('missing');
    expect(container.textContent).toContain('Last rotated');
    expect(container.textContent).toContain('Expires');
    expect(container.textContent).toContain('2026');
    expect(container.textContent).not.toContain('Enable');

    await cleanupRenderedView(root, container);
  });

  it('supports replacing an ssh credential from the secrets tab', async () => {
    fetchSSHCredentialsMock.mockResolvedValueOnce({
      configured: true,
      items: [
        {
          credential_id: 'ops-key',
          display_name: 'Ops key',
          connector_id: 'ssh-main',
          username: 'root',
          auth_type: 'password',
          host_scope: '192.168.3.100',
          status: 'active',
        },
      ],
    });
    updateSSHCredentialMock.mockResolvedValueOnce({
      credential_id: 'ops-key',
      display_name: 'Ops key rotated',
      connector_id: 'ssh-main',
      username: 'root',
      auth_type: 'password',
      host_scope: '192.168.3.100',
      status: 'active',
      last_rotated_at: '2026-04-10T00:00:00Z',
    });

    const { container, root } = await renderOpsSecretsView();

    const replaceButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Replace'));
    expect(replaceButton).toBeTruthy();
    await act(async () => {
      replaceButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();

    const displayNameInput = Array.from(container.querySelectorAll('input')).find((input) => input.placeholder === 'Production root key') as HTMLInputElement | undefined;
    const passwordInput = Array.from(container.querySelectorAll('input')).find((input) => input.placeholder === 'Enter SSH password') as HTMLInputElement | undefined;
    expect(displayNameInput).toBeTruthy();
    expect(passwordInput).toBeTruthy();

    await setInputValue(displayNameInput!, 'Ops key rotated');
    await setInputValue(passwordInput!, 'fresh-password');

    const submitButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Replace SSH Credential'));
    expect(submitButton).toBeTruthy();
    await act(async () => {
      submitButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();

    expect(updateSSHCredentialMock).toHaveBeenCalledWith('ops-key', expect.objectContaining({
      display_name: 'Ops key rotated',
      password: 'fresh-password',
    }));
    expect(container.textContent).not.toContain('fresh-password');

    await cleanupRenderedView(root, container);
  });

  it('supports marking rotation required and deleting an ssh credential', async () => {
    fetchSSHCredentialsMock.mockResolvedValueOnce({
      configured: true,
      items: [
        {
          credential_id: 'ops-key',
          display_name: 'Ops key',
          connector_id: 'ssh-main',
          username: 'root',
          auth_type: 'password',
          host_scope: '192.168.3.100',
          status: 'active',
        },
      ],
    });
    setSSHCredentialStatusMock.mockResolvedValueOnce({
      credential_id: 'ops-key',
      display_name: 'Ops key',
      connector_id: 'ssh-main',
      username: 'root',
      auth_type: 'password',
      host_scope: '192.168.3.100',
      status: 'rotation_required',
    });
    deleteSSHCredentialMock.mockResolvedValueOnce({
      credential_id: 'ops-key',
      status: 'deleted',
    });

    const { container, root } = await renderOpsSecretsView();

    const rotateButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Mark rotation required'));
    expect(rotateButton).toBeTruthy();
    await act(async () => {
      rotateButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();

    expect(setSSHCredentialStatusMock).toHaveBeenCalledWith('ops-key', 'rotation_required', expect.stringContaining('rotation'));

    const deleteButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Delete');
    expect(deleteButton).toBeTruthy();
    await act(async () => {
      deleteButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();

    const confirmDeleteButton = Array.from(document.body.querySelectorAll('button')).find((button) => button.textContent?.includes('Delete SSH Credential'));
    expect(confirmDeleteButton).toBeTruthy();
    await act(async () => {
      confirmDeleteButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flush();
    await flush();

    expect(deleteSSHCredentialMock).toHaveBeenCalledWith('ops-key', expect.stringContaining('Delete SSH Credential'));

    await cleanupRenderedView(root, container);
  });

  it('limits secrets tab to ssh custody read access without loading unrelated config tabs', async () => {
    useAuthMock.mockReturnValue({
      isAuthenticated: true,
      user: { permissions: ['ssh_credentials.read'] },
    });
    fetchSSHCredentialsMock.mockResolvedValueOnce({
      configured: true,
      items: [
        {
          credential_id: 'ops-key',
          display_name: 'Ops key',
          connector_id: 'ssh-main',
          username: 'root',
          auth_type: 'password',
          host_scope: '192.168.3.100',
          status: 'active',
        },
      ],
    });

    const { container, root } = await renderOpsSecretsView();

    expect(fetchAuthorizationConfigMock).not.toHaveBeenCalled();
    expect(fetchApprovalRoutingConfigMock).not.toHaveBeenCalled();
    expect(fetchConnectorsConfigMock).not.toHaveBeenCalled();
    expect(fetchProvidersConfigMock).not.toHaveBeenCalled();
    expect(fetchReasoningPromptConfigMock).not.toHaveBeenCalled();
    expect(fetchDesensitizationConfigMock).not.toHaveBeenCalled();
    expect(fetchSecretsInventoryMock).not.toHaveBeenCalled();
    expect(fetchSSHCredentialsMock).toHaveBeenCalledTimes(1);
    expect(container.textContent).toContain('Ops key');

    const storeButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Store SSH Credential')) as HTMLButtonElement | undefined;
    expect(storeButton).toBeTruthy();
    expect(storeButton?.disabled).toBe(true);

    await cleanupRenderedView(root, container);
  });

  it('does not render raw secret refs in referenced secrets inventory', async () => {
    fetchSecretsInventoryMock.mockResolvedValueOnce({
      configured: true,
      loaded: true,
      path: '/tmp/secrets.yaml',
      updated_at: '2026-03-29T08:00:00Z',
      custody_configured: false,
      ssh_credential_configured: false,
      items: [
        { owner_type: 'connector', owner_id: 'prometheus-main', key: 'bearer_token', set: false, status: 'missing' },
      ],
    });

    const { container, root } = await renderOpsSecretsView();

    expect(container.textContent).toContain('prometheus-main');
    expect(container.textContent).toContain('missing');
    expect(container.textContent).not.toContain('secret://');

    await cleanupRenderedView(root, container);
  });
});
