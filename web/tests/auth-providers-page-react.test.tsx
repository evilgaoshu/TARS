// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'

const createAuthProviderMock = vi.fn()
const fetchAccessConfigMock = vi.fn()
const fetchAuthProviderMock = vi.fn()
const fetchAuthProvidersMock = vi.fn()
const setAuthProviderEnabledMock = vi.fn()
const updateAuthProviderMock = vi.fn()

vi.mock('../src/lib/api/access', () => ({
  createAuthProvider: createAuthProviderMock,
  fetchAccessConfig: fetchAccessConfigMock,
  fetchAuthProvider: fetchAuthProviderMock,
  fetchAuthProviders: fetchAuthProvidersMock,
  setAuthProviderEnabled: setAuthProviderEnabledMock,
  updateAuthProvider: updateAuthProviderMock,
}))

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}))

vi.mock('../src/lib/FeatureGateContext', () => ({
  useCapabilities: () => ({
    capabilities: {
      'identity.oidc': { enabled: true, status: 'available' },
      'identity.local_password': { enabled: true, status: 'available' },
      'identity.local_token': { enabled: true, status: 'available' },
    },
    loading: false,
    refresh: async () => {},
  }),
}))

vi.mock('../src/lib/api/ops', () => ({
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('AuthProvidersPage', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    createAuthProviderMock.mockReset()
    fetchAccessConfigMock.mockReset()
    fetchAuthProviderMock.mockReset()
    fetchAuthProvidersMock.mockReset()
    setAuthProviderEnabledMock.mockReset()
    updateAuthProviderMock.mockReset()

    fetchAuthProvidersMock.mockResolvedValue({ items: [] })
    fetchAccessConfigMock.mockResolvedValue({ configured: true, config: { users: [] }, path: '' })
  })

  it('includes local_password in the provider type selector', async () => {
    const { AuthProvidersPage } = await import('../src/pages/identity/AuthProvidersPage')

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<AuthProvidersPage />)
    })
    await flush()

    const newButton = container.querySelector('button') as HTMLButtonElement | null
    expect(newButton).toBeTruthy()

    await act(async () => {
      newButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    const select = Array.from(container.querySelectorAll('select')).find((element) =>
      Array.from(element.options).some((option) => option.value === 'oidc'),
    )

    expect(select).toBeTruthy()
    const optionValues = Array.from(select?.options || []).map((option) => option.value)
    expect(optionValues).toContain('local_password')

    root.unmount()
    container.remove()
  })
})
