// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'

const completeSetupWizardMock = vi.fn()
const checkProviderAvailabilityMock = vi.fn()
const checkSetupWizardProviderMock = vi.fn()
const fetchSecretsInventoryMock = vi.fn()
const fetchSetupStatusMock = vi.fn()
const fetchSetupWizardMock = vi.fn()
const saveSetupWizardAdminMock = vi.fn()
const saveSetupWizardAuthMock = vi.fn()
const saveSetupWizardChannelMock = vi.fn()
const saveSetupWizardProviderMock = vi.fn()
const triggerSmokeAlertMock = vi.fn()
const loginWithPasswordMock = vi.fn()
const refreshMock = vi.fn()
const navigateMock = vi.fn()
const saveStoredSessionMock = vi.fn()
const useAuthMock = vi.fn()

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => navigateMock,
  }
})

vi.mock('../src/lib/api/ops', () => ({
  completeSetupWizard: completeSetupWizardMock,
  checkProviderAvailability: checkProviderAvailabilityMock,
  checkSetupWizardProvider: checkSetupWizardProviderMock,
  fetchSecretsInventory: fetchSecretsInventoryMock,
  fetchSetupStatus: fetchSetupStatusMock,
  fetchSetupWizard: fetchSetupWizardMock,
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
  saveSetupWizardAdmin: saveSetupWizardAdminMock,
  saveSetupWizardAuth: saveSetupWizardAuthMock,
  saveSetupWizardChannel: saveSetupWizardChannelMock,
  saveSetupWizardProvider: saveSetupWizardProviderMock,
  triggerSmokeAlert: triggerSmokeAlertMock,
}))

vi.mock('../src/lib/api/access', () => ({
  loginWithPassword: loginWithPasswordMock,
}))

vi.mock('../src/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

vi.mock('../src/lib/auth/storage', () => ({
  saveStoredSession: saveStoredSessionMock,
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set
  setter?.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
  input.dispatchEvent(new Event('change', { bubbles: true }))
}

describe('SetupSmokeView wizard mode', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    completeSetupWizardMock.mockReset()
    checkProviderAvailabilityMock.mockReset()
    checkSetupWizardProviderMock.mockReset()
    fetchSecretsInventoryMock.mockReset()
    fetchSetupStatusMock.mockReset()
    fetchSetupWizardMock.mockReset()
    saveSetupWizardAdminMock.mockReset()
    saveSetupWizardAuthMock.mockReset()
    saveSetupWizardChannelMock.mockReset()
    saveSetupWizardProviderMock.mockReset()
    triggerSmokeAlertMock.mockReset()
    loginWithPasswordMock.mockReset()
    refreshMock.mockReset()
    navigateMock.mockReset()
    saveStoredSessionMock.mockReset()
    useAuthMock.mockReset()

    useAuthMock.mockReturnValue({
      isAuthenticated: false,
      refresh: refreshMock,
    })

    fetchSetupWizardMock.mockResolvedValue({
      initialization: {
        mode: 'wizard',
        admin_configured: false,
        auth_configured: false,
        provider_ready: false,
        channel_ready: false,
        provider_check_ok: false,
        initialized: false,
        next_step: 'admin',
      },
      admin: {},
      auth: {},
      provider: {},
      channel: {},
    })
    fetchSetupStatusMock.mockResolvedValue({
      rollout_mode: 'pilot_core',
      features: {
        diagnosis_enabled: true,
        approval_enabled: true,
        execution_enabled: true,
        knowledge_ingest_enabled: false,
      },
      initialization: {
        initialized: false,
        mode: 'wizard',
        admin_configured: false,
        auth_configured: false,
        provider_ready: false,
        channel_ready: false,
        next_step: 'admin',
      },
      telegram: {
        configured: false,
        mode: 'disabled',
        last_result: 'stub',
      },
      model: {
        configured: false,
      },
      assist_model: {
        configured: false,
      },
      providers: {
        configured: false,
        loaded: false,
      },
      connectors: {
        configured: false,
        loaded: false,
        total_entries: 0,
        enabled_entries: 0,
      },
      smoke_defaults: {
        hosts: [],
      },
      authorization: {
        configured: false,
        loaded: false,
      },
      approval: {
        configured: false,
        loaded: false,
      },
      reasoning: {
        configured: false,
        loaded: false,
      },
      desensitization: {
        configured: false,
        loaded: false,
      },
    })
  })

  async function renderSetupSmokeView(container: HTMLDivElement) {
    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    return root
  }

  function primeSetupReadyForCompletion() {
    fetchSetupWizardMock.mockResolvedValue({
      initialization: {
        mode: 'wizard',
        admin_configured: true,
        auth_configured: true,
        provider_ready: true,
        channel_ready: true,
        provider_check_ok: true,
        provider_checked: true,
        initialized: false,
        next_step: 'complete',
        admin_user_id: 'setup-admin',
        auth_provider_id: 'local_password',
        login_hint: {
          username: 'setup-admin',
          provider: 'local_password',
          login_url: '/login?provider_id=local_password&username=setup-admin&next=%2Fruntime-checks',
        },
      },
      admin: {
        user: {
          user_id: 'setup-admin',
          username: 'setup-admin',
          display_name: 'Setup Admin',
          email: 'setup@example.com',
        },
      },
      auth: {
        provider: {
          id: 'local_password',
          type: 'local_password',
          enabled: true,
        },
      },
      provider: {
        provider: {
          id: 'primary-openai',
          vendor: 'openai',
          protocol: 'openai_compatible',
          base_url: 'https://api.openai.com/v1',
          api_key_ref: 'secret://providers/primary-openai/api-key',
          primary_model: 'gpt-4o-mini',
          enabled: true,
        },
      },
      channel: {
        channel: {
          id: 'inbox-primary',
          name: 'Primary Inbox',
          kind: 'in_app_inbox',
          type: 'in_app_inbox',
          target: 'default',
          usages: ['approval', 'notifications'],
          capabilities: ['approval', 'notifications'],
        },
      },
    })
    fetchSetupStatusMock.mockResolvedValue({
      rollout_mode: 'pilot_core',
      features: {
        diagnosis_enabled: true,
        approval_enabled: true,
        execution_enabled: true,
        knowledge_ingest_enabled: false,
      },
      initialization: {
        initialized: false,
        mode: 'wizard',
        admin_configured: true,
        auth_configured: true,
        provider_ready: true,
        channel_ready: true,
        provider_check_ok: true,
        provider_checked: true,
        next_step: 'complete',
      },
      telegram: {
        configured: false,
        mode: 'disabled',
        last_result: 'stub',
      },
      model: {
        configured: true,
        model_name: 'gpt-4o-mini',
      },
      assist_model: {
        configured: false,
      },
      providers: {
        configured: true,
        loaded: true,
      },
      connectors: {
        configured: false,
        loaded: false,
        total_entries: 0,
        enabled_entries: 0,
      },
      smoke_defaults: {
        hosts: [],
      },
      authorization: {
        configured: false,
        loaded: false,
      },
      approval: {
        configured: false,
        loaded: false,
      },
      reasoning: {
        configured: false,
        loaded: false,
      },
      desensitization: {
        configured: false,
        loaded: false,
      },
    })
  }

  it('does not describe the provider model field as initial model binding', async () => {
    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).not.toContain('Initial Model Binding')

    root.unmount()
    container.remove()
  })

  it('separates provider connectivity from the default model baseline', async () => {
    checkSetupWizardProviderMock.mockResolvedValue({
      provider_id: 'primary-openai',
      available: true,
      detail: 'provider is reachable',
    })
    fetchSetupWizardMock.mockResolvedValueOnce({
      initialization: {
        mode: 'wizard',
        admin_configured: false,
        auth_configured: false,
        provider_ready: false,
        channel_ready: false,
        provider_check_ok: false,
        initialized: false,
        next_step: 'admin',
      },
      admin: {},
      auth: {},
      provider: {
        provider: {
          id: 'primary-openai',
          vendor: 'openai',
          protocol: 'openai_compatible',
          base_url: 'https://api.openai.com/v1',
          api_key_ref: 'secret://providers/primary-openai/api-key',
          primary_model: 'gpt-4o-mini',
          api_key_set: true,
          enabled: true,
        },
      },
      channel: {},
    })

    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('Provider Connectivity')
    expect(container.textContent).toContain('Default Model Baseline')

    const modelInput = Array.from(container.querySelectorAll('input')).find((input) => input.value === 'gpt-4o-mini') as HTMLInputElement | undefined
    expect(modelInput).toBeTruthy()

    await act(async () => {
      setInputValue(modelInput!, '')
    })
    await flush()

    const checkButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Check Connectivity')) as HTMLButtonElement | undefined
    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Save Provider')) as HTMLButtonElement | undefined
    expect(checkButton).toBeTruthy()
    expect(saveButton).toBeTruthy()
    expect(checkButton?.disabled).toBe(false)
    expect(saveButton?.disabled).toBe(true)

    await act(async () => {
      checkButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    expect(checkSetupWizardProviderMock).toHaveBeenCalledWith(expect.objectContaining({
      provider_id: 'primary-openai',
      model: '',
    }))

    root.unmount()
    container.remove()
  })

  it('renders structured channel usage controls instead of a CSV input', async () => {
    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const csvInput = Array.from(container.querySelectorAll('input')).find((input) => input.placeholder === 'approval, notifications')
    expect(csvInput).toBeFalsy()
    expect(container.textContent).toContain('approval')
    expect(container.textContent).toContain('notifications')

    root.unmount()
    container.remove()
  })

  it('collapses auth into the administrator step instead of rendering a separate auth card', async () => {
    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).not.toContain('Step 2 · Authentication Method')
    expect(container.textContent).toContain('Local password sign-in is enabled automatically')

    root.unmount()
    container.remove()
  })

  it('labels required and optional setup fields clearly and keeps advanced IDs hidden by default', async () => {
    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('Required')
    expect(container.textContent).toContain('Optional')
    expect(container.textContent).not.toContain('Provider ID')
    expect(container.textContent).not.toContain('Protocol')
    expect(container.textContent).not.toContain('Channel ID')

    root.unmount()
    container.remove()
  })

  it('shows password guidance and lets operators reveal the admin password', async () => {
    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const passwordInput = container.querySelector('input[type="password"]') as HTMLInputElement | null
    expect(passwordInput).toBeTruthy()

    await act(async () => {
      passwordInput!.value = 'weak'
      passwordInput!.dispatchEvent(new Event('input', { bubbles: true }))
      passwordInput!.dispatchEvent(new Event('change', { bubbles: true }))
    })
    await flush()

    expect(container.textContent).toContain('Use at least 8 characters')
    expect(container.textContent).toContain('Add an uppercase letter')

    const revealButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Show password'))
    expect(revealButton).toBeTruthy()

    await act(async () => {
      revealButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    expect(passwordInput?.type).toBe('text')

    root.unmount()
    container.remove()
  })

  it('uses the setup-specific provider connectivity check and warns when telegram requires a bot token', async () => {
    checkSetupWizardProviderMock.mockResolvedValue({
      provider_id: 'primary-openai',
      available: true,
      detail: 'provider is reachable',
    })
    fetchSetupWizardMock.mockResolvedValueOnce({
      initialization: {
        mode: 'wizard',
        admin_configured: false,
        auth_configured: false,
        provider_ready: false,
        channel_ready: false,
        provider_check_ok: false,
        initialized: false,
        next_step: 'admin',
      },
      admin: {},
      auth: {},
      provider: {
        provider: {
          id: 'primary-openai',
          vendor: 'openai',
          protocol: 'openai_compatible',
          base_url: 'https://api.openai.com/v1',
          api_key_ref: 'secret://providers/primary-openai/api-key',
          primary_model: 'gpt-4o-mini',
          api_key_set: true,
          enabled: true,
        },
      },
      channel: {},
    })

    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const checkButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Check Connectivity'))
    expect(checkButton).toBeTruthy()

    await act(async () => {
      checkButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    expect(checkSetupWizardProviderMock).toHaveBeenCalledWith(expect.objectContaining({
      provider_id: 'primary-openai',
      model: 'gpt-4o-mini',
    }))
    expect(checkProviderAvailabilityMock).not.toHaveBeenCalled()

    const channelKindSelect = Array.from(container.querySelectorAll('select')).find((select) =>
      Array.from(select.querySelectorAll('option')).some((option) => option.value === 'telegram'),
    ) as HTMLSelectElement | undefined
    expect(channelKindSelect).toBeTruthy()

    await act(async () => {
      channelKindSelect!.value = 'telegram'
      channelKindSelect!.dispatchEvent(new Event('change', { bubbles: true }))
    })
    await flush()

    expect(container.textContent).toContain('Requires Telegram bot token')
    expect(container.textContent).toContain('Use in-app inbox until the bot token is configured')

    root.unmount()
    container.remove()
  })

  it('shows the provider check error inline when connectivity verification fails', async () => {
    checkSetupWizardProviderMock.mockRejectedValueOnce(new Error('provider check failed'))
    fetchSetupWizardMock.mockResolvedValueOnce({
      initialization: {
        mode: 'wizard',
        admin_configured: false,
        auth_configured: false,
        provider_ready: false,
        channel_ready: false,
        provider_check_ok: false,
        initialized: false,
        next_step: 'admin',
      },
      admin: {},
      auth: {},
      provider: {
        provider: {
          id: 'primary-openai',
          vendor: 'openai',
          protocol: 'openai_compatible',
          base_url: 'https://api.openai.com/v1',
          api_key_ref: 'secret://providers/primary-openai/api-key',
          primary_model: 'gpt-4o-mini',
          api_key_set: true,
          enabled: true,
        },
      },
      channel: {},
    })

    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const checkButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Check Connectivity'))
    expect(checkButton).toBeTruthy()

    await act(async () => {
      checkButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()
    await flush()

    expect(checkSetupWizardProviderMock).toHaveBeenCalledTimes(1)
    expect(container.textContent).toContain('Failed to check provider connectivity.')
    expect(container.textContent).not.toContain('Connectivity check passed:')

    root.unmount()
    container.remove()
  })

  it('shows a visible language toggle and lets operators enter a raw API key during setup', async () => {
    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('EN')
    expect(container.textContent).toContain('中')
    expect(container.textContent).toContain('API Key')
    expect(container.textContent).not.toContain('Secret Reference')

    root.unmount()
    container.remove()
  })

  it('renders setup errors inside the relevant step card instead of only below the page hero', async () => {
    saveSetupWizardProviderMock.mockRejectedValueOnce(new Error('provider save failed'))
    fetchSetupWizardMock.mockResolvedValueOnce({
      initialization: {
        mode: 'wizard',
        admin_configured: false,
        auth_configured: false,
        provider_ready: false,
        channel_ready: false,
        provider_check_ok: false,
        initialized: false,
        next_step: 'admin',
      },
      admin: {},
      auth: {},
      provider: {
        provider: {
          id: 'primary-openai',
          vendor: 'openai',
          protocol: 'openai_compatible',
          base_url: 'https://api.openai.com/v1',
          api_key_ref: 'secret://providers/primary-openai/api-key',
          primary_model: 'gpt-4o-mini',
          api_key_set: true,
          enabled: true,
        },
      },
      channel: {},
    })

    const { SetupSmokeView } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <SetupSmokeView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Save Provider'))
    expect(saveButton).toBeTruthy()

    await act(async () => {
      saveButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()
    await flush()

    const pageText = container.textContent || ''
    expect(pageText).toContain('Failed to save setup step.')
    expect(pageText.indexOf('Step 2 · Provider Connectivity & Baseline Model')).toBeLessThan(pageText.indexOf('Failed to save setup step.'))
    expect(pageText.indexOf('Failed to save setup step.')).toBeLessThan(pageText.indexOf('Step 3 · Primary Entrypoint & Delivery'))

    root.unmount()
    container.remove()
  })

  it('hands off directly to /runtime-checks after setup completion without reloading setup wizard state', async () => {
    primeSetupReadyForCompletion()
    completeSetupWizardMock.mockResolvedValue({
      initialization: {
        mode: 'runtime',
        admin_configured: true,
        auth_configured: true,
        provider_ready: true,
        channel_ready: true,
        provider_check_ok: true,
        provider_checked: true,
        initialized: true,
        next_step: '',
        login_hint: {
          username: 'setup-admin',
          provider: 'local_password',
          login_url: '/login?provider_id=local_password&username=setup-admin&next=%2Fruntime-checks',
        },
      },
      admin: {},
      auth: {},
      provider: {},
      channel: {},
    })
    loginWithPasswordMock.mockResolvedValue({
      session_token: 'session-123',
      provider_id: 'local_password',
      user: {
        user_id: 'setup-admin',
        username: 'setup-admin',
        display_name: 'Setup Admin',
        email: 'setup@example.com',
      },
      roles: ['platform_admin'],
      permissions: ['*'],
    })

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = await renderSetupSmokeView(container)

    const passwordInput = container.querySelector('input[type="password"]') as HTMLInputElement | null
    expect(passwordInput).toBeTruthy()

    await act(async () => {
      setInputValue(passwordInput!, 'Password-123!')
    })
    await flush()

    const completeButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Complete Setup'))
    expect(completeButton).toBeTruthy()

    await act(async () => {
      completeButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()
    await flush()

    expect(completeSetupWizardMock).toHaveBeenCalledTimes(1)
    expect(loginWithPasswordMock).toHaveBeenCalledWith('local_password', 'setup-admin', 'Password-123!')
    expect(saveStoredSessionMock).toHaveBeenCalledTimes(1)
    expect(refreshMock).toHaveBeenCalledTimes(1)
    expect(navigateMock).toHaveBeenCalledWith('/runtime-checks')
    expect(fetchSetupWizardMock).toHaveBeenCalledTimes(1)
    expect(fetchSetupStatusMock).toHaveBeenCalledTimes(1)

    root.unmount()
    container.remove()
  })

  it('falls back to the login hint after setup completion when auto-login cannot finish, without reloading setup wizard state', async () => {
    primeSetupReadyForCompletion()
    completeSetupWizardMock.mockResolvedValue({
      initialization: {
        mode: 'runtime',
        admin_configured: true,
        auth_configured: true,
        provider_ready: true,
        channel_ready: true,
        provider_check_ok: true,
        provider_checked: true,
        initialized: true,
        next_step: '',
        login_hint: {
          username: 'setup-admin',
          provider: 'local_password',
          login_url: '/login?provider_id=local_password&username=setup-admin&next=%2Fruntime-checks',
        },
      },
      admin: {},
      auth: {},
      provider: {},
      channel: {},
    })
    loginWithPasswordMock.mockRejectedValue(new Error('session bootstrap failed'))

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = await renderSetupSmokeView(container)

    const passwordInput = container.querySelector('input[type="password"]') as HTMLInputElement | null
    expect(passwordInput).toBeTruthy()

    await act(async () => {
      setInputValue(passwordInput!, 'Password-123!')
    })
    await flush()

    const completeButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Complete Setup'))
    expect(completeButton).toBeTruthy()

    await act(async () => {
      completeButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()
    await flush()

    expect(completeSetupWizardMock).toHaveBeenCalledTimes(1)
    expect(loginWithPasswordMock).toHaveBeenCalledWith('local_password', 'setup-admin', 'Password-123!')
    expect(saveStoredSessionMock).not.toHaveBeenCalled()
    expect(refreshMock).not.toHaveBeenCalled()
    expect(navigateMock).toHaveBeenCalledWith('/login?provider_id=local_password&username=setup-admin&next=%2Fruntime-checks')
    expect(fetchSetupWizardMock).toHaveBeenCalledTimes(1)
    expect(fetchSetupStatusMock).toHaveBeenCalledTimes(1)

    root.unmount()
    container.remove()
  })
})

describe('RuntimeChecksPage', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    completeSetupWizardMock.mockReset()
    checkProviderAvailabilityMock.mockReset()
    checkSetupWizardProviderMock.mockReset()
    fetchSecretsInventoryMock.mockReset()
    fetchSetupStatusMock.mockReset()
    fetchSetupWizardMock.mockReset()
    saveSetupWizardAdminMock.mockReset()
    saveSetupWizardAuthMock.mockReset()
    saveSetupWizardChannelMock.mockReset()
    saveSetupWizardProviderMock.mockReset()
    triggerSmokeAlertMock.mockReset()
    loginWithPasswordMock.mockReset()
    refreshMock.mockReset()

    fetchSetupStatusMock.mockResolvedValue({
      rollout_mode: 'pilot_core',
      features: {
        diagnosis_enabled: true,
        approval_enabled: true,
        execution_enabled: true,
        knowledge_ingest_enabled: false,
      },
      initialization: {
        initialized: true,
        mode: 'runtime',
        admin_configured: true,
        auth_configured: true,
        provider_ready: true,
        channel_ready: true,
        provider_check_ok: true,
        next_step: '',
      },
      telegram: {
        configured: true,
        mode: 'polling',
        last_result: 'healthy',
      },
      model: {
        configured: true,
        model_name: 'gpt-4o-mini',
      },
      assist_model: {
        configured: false,
      },
      providers: {
        configured: true,
        loaded: true,
      },
      connectors: {
        configured: true,
        loaded: true,
        total_entries: 3,
        enabled_entries: 3,
      },
      smoke_defaults: {
        hosts: ['192.168.3.100'],
      },
      authorization: {
        configured: true,
        loaded: true,
      },
      approval: {
        configured: true,
        loaded: true,
      },
      reasoning: {
        configured: true,
        loaded: true,
      },
      desensitization: {
        configured: true,
        loaded: true,
      },
      latest_smoke: null,
    })
    fetchSecretsInventoryMock.mockResolvedValue({
      items: [],
    })
  })

  it('renders runtime checks without loading wizard copy or setup wizard APIs', async () => {
    const { RuntimeChecksPage } = await import('../src/pages/setup/SetupSmokeView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <RuntimeChecksPage />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(fetchSetupStatusMock).toHaveBeenCalled()
    expect(fetchSecretsInventoryMock).toHaveBeenCalled()
    expect(fetchSetupWizardMock).not.toHaveBeenCalled()
    expect(container.textContent).toContain('Runtime Checks')
    expect(container.textContent).toContain('Manual Runtime Check')
    expect(container.textContent).not.toContain('Step 1 · Primary Administrator')
    expect(container.textContent).not.toContain('Complete Setup')

    root.unmount()
    container.remove()
  })
})
