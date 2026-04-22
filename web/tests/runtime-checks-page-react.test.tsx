// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'

const fetchSecretsInventoryMock = vi.fn()
const fetchSetupStatusMock = vi.fn()
const fetchSetupWizardMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  completeSetupWizard: vi.fn(),
  checkProviderAvailability: vi.fn(),
  checkSetupWizardProvider: vi.fn(),
  fetchSecretsInventory: fetchSecretsInventoryMock,
  fetchSetupStatus: fetchSetupStatusMock,
  fetchSetupWizard: fetchSetupWizardMock,
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
  saveSetupWizardAdmin: vi.fn(),
  saveSetupWizardAuth: vi.fn(),
  saveSetupWizardChannel: vi.fn(),
  saveSetupWizardProvider: vi.fn(),
  triggerSmokeAlert: vi.fn(),
}))

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => ({
    lang: 'en-US',
    setLang: vi.fn(),
    t: (key: string) => {
      const messages: Record<string, string> = {
        'header.languageEnglish': 'English',
        'header.languageChinese': 'Chinese',
      }
      return messages[key] ?? key
    },
  }),
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('RuntimeChecksPage', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    fetchSecretsInventoryMock.mockReset()
    fetchSetupStatusMock.mockReset()
    fetchSetupWizardMock.mockReset()

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
        primary_provider_id: 'primary-openai',
        default_channel_id: 'inbox-primary',
      },
      telegram: {
        configured: true,
        mode: 'polling',
        last_result: 'healthy',
      },
      model: {
        configured: true,
        model_name: 'gpt-4o-mini',
        last_result: 'healthy',
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
        metrics_runtime: {
          primary: { connector_id: 'victoriametrics-main' },
          fallback: { runtime: 'none' },
          component_runtime: { last_result: 'healthy', last_changed_at: '2026-04-22T00:00:00Z' },
        },
        execution_runtime: {
          primary: { connector_id: 'jumpserver-main' },
          fallback: { runtime: 'ssh' },
          component_runtime: { last_result: 'healthy', last_changed_at: '2026-04-22T00:00:00Z' },
        },
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
    fetchSecretsInventoryMock.mockResolvedValue({ items: [] })
  })

  it('renders initialized runtime health items without loading setup wizard state', async () => {
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

    expect(fetchSetupStatusMock).toHaveBeenCalledTimes(1)
    expect(fetchSecretsInventoryMock).toHaveBeenCalledTimes(1)
    expect(fetchSetupWizardMock).not.toHaveBeenCalled()
    expect(container.textContent).toContain('Runtime Checks')
    expect(container.textContent).toContain('Metrics Pipeline')
    expect(container.textContent).toContain('Execution Pipeline')
    expect(container.textContent).toContain('Telegram Bot')
    expect(container.textContent).toContain('Primary Model')
    expect(container.textContent).toContain('Connectors')
    expect(container.textContent).toContain('3/3 enabled')
    expect(container.textContent).toContain('Run Runtime Check')
    expect(container.textContent).not.toContain('Step 1 · Primary Administrator')
    expect(container.textContent).not.toContain('Complete Setup')

    root.unmount()
    container.remove()
  })
})
