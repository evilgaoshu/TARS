// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'

const fetchAuthProvidersMock = vi.fn()
const fetchMeMock = vi.fn()
const loginWithPasswordMock = vi.fn()
const loginWithTokenMock = vi.fn()
const verifyAuthChallengeMock = vi.fn()
const verifyAuthMFAMock = vi.fn()
const loginMock = vi.fn()
const navigateMock = vi.fn()

const messages: Record<string, string> = {
  'login.title': 'TARS Ops',
  'login.subtitle': 'Incident Recovery Gateway',
  'login.provider': 'Auth Provider',
  'login.localToken': 'Local Token / Break Glass',
  'login.usernameOrEmail': 'Username or Email',
  'login.password': 'Password',
  'login.token': 'Local Token',
  'login.challengeCode': 'Challenge Code',
  'login.authenticatorCode': 'Authenticator Code',
  'login.verifyChallenge': 'Verify Challenge',
  'login.verifyMfa': 'Verify MFA',
  'login.continueWith': 'Continue with {{name}}',
  'login.redirectNoticePrefix': 'Continue with {{name}}.',
  'login.ldapNotice': 'LDAP is not enabled yet.',
  'login.enterCredentials': 'Please enter username and password.',
  'login.enterToken': 'Please provide your Ops Bearer Token',
  'login.challengeIssued': 'Challenge issued.',
  'login.challengeVerified': 'Challenge verified.',
  'login.signin': 'Sign In',
  'login.authenticating': 'Authenticating...',
  'login.authFailed': 'Authentication failed.',
  'header.languageEnglish': 'English',
  'header.languageChinese': 'Chinese',
}

vi.mock('../src/lib/api/access', () => ({
  fetchAuthProviders: fetchAuthProvidersMock,
  fetchMe: fetchMeMock,
  loginWithPassword: loginWithPasswordMock,
  loginWithToken: loginWithTokenMock,
  verifyAuthChallenge: verifyAuthChallengeMock,
  verifyAuthMFA: verifyAuthMFAMock,
}))

vi.mock('../src/hooks/useAuth', () => ({
  useAuth: () => ({ login: loginMock }),
}))

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom')
  return {
    ...actual,
    useNavigate: () => navigateMock,
  }
})

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => ({
    lang: 'en-US',
    setLang: vi.fn(),
    t: (key: string, paramsOrFallback?: Record<string, unknown> | string) => {
      if (typeof paramsOrFallback === 'string') {
        return messages[key] ?? paramsOrFallback
      }
      return messages[key] ?? key
    },
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

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, resolve, reject }
}

describe('LoginView', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    fetchAuthProvidersMock.mockReset()
    fetchMeMock.mockReset()
    loginWithPasswordMock.mockReset()
    loginWithTokenMock.mockReset()
    verifyAuthChallengeMock.mockReset()
    verifyAuthMFAMock.mockReset()
    loginMock.mockReset()
    navigateMock.mockReset()
  })

  it('defaults to local_password when it is available', async () => {
    fetchAuthProvidersMock.mockResolvedValue({
      items: [
        { id: 'corp-password', name: 'Corporate Password', type: 'local_password', enabled: true },
        { id: 'github-sso', name: 'GitHub', type: 'oidc', enabled: true },
      ],
    })

    const { LoginView } = await import('../src/pages/ops/LoginView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <LoginView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const providerSelect = container.querySelector('#login-provider') as HTMLSelectElement | null
    expect(providerSelect).toBeTruthy()
    expect(providerSelect?.value).toBe('corp-password')

    root.unmount()
    container.remove()
  })

  it('does not show break-glass local token unless it is explicitly offered', async () => {
    fetchAuthProvidersMock.mockResolvedValue({
      items: [
        { id: 'corp-password', name: 'Corporate Password', type: 'local_password', enabled: true },
        { id: 'github-sso', name: 'GitHub', type: 'oidc', enabled: true },
      ],
    })

    const { LoginView } = await import('../src/pages/ops/LoginView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <LoginView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const providerSelect = container.querySelector('#login-provider') as HTMLSelectElement | null
    const optionValues = Array.from(providerSelect?.querySelectorAll('option') || []).map((option) => option.value)
    expect(optionValues).not.toContain('local_token')

    root.unmount()
    container.remove()
  })

  it('does not expose break-glass local token while auth providers are still loading', async () => {
    const providers = deferred<{
      items: Array<{ id: string; name: string; type: string; enabled: boolean }>
    }>()
    fetchAuthProvidersMock.mockReturnValue(providers.promise)

    const { LoginView } = await import('../src/pages/ops/LoginView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <LoginView />
        </MemoryRouter>,
      )
    })
    await flush()

    expect(container.textContent).not.toContain('Local Token')

    await act(async () => {
      providers.resolve({
        items: [
          { id: 'corp-password', name: 'Corporate Password', type: 'local_password', enabled: true },
        ],
      })
      await providers.promise
    })
    await flush()

    const providerSelect = container.querySelector('#login-provider') as HTMLSelectElement | null
    expect(providerSelect?.value).toBe('corp-password')

    root.unmount()
    container.remove()
  })

  it('keeps break-glass local token available when it is explicitly requested', async () => {
    fetchAuthProvidersMock.mockResolvedValue({
      items: [
        { id: 'corp-password', name: 'Corporate Password', type: 'local_password', enabled: true },
      ],
    })

    const { LoginView } = await import('../src/pages/ops/LoginView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter initialEntries={['/login?provider_id=local_token']}>
          <LoginView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const providerSelect = container.querySelector('#login-provider') as HTMLSelectElement | null
    const optionValues = Array.from(providerSelect?.querySelectorAll('option') || []).map((option) => option.value)
    expect(optionValues).toContain('local_token')
    expect(providerSelect?.value).toBe('local_token')

    root.unmount()
    container.remove()
  })

  it('renders redirect notice copy from the correct translation key', async () => {
    fetchAuthProvidersMock.mockResolvedValue({
      items: [
        { id: 'github-sso', name: 'GitHub', type: 'oidc', enabled: true },
      ],
    })

    const { LoginView } = await import('../src/pages/ops/LoginView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter initialEntries={['/login?provider_id=github-sso']}>
          <LoginView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('Continue with GitHub.')
    expect(container.textContent).not.toContain('login.redirectNotice')

    root.unmount()
    container.remove()
  })

  it('redirects to the requested next path after session-token completion', async () => {
    fetchAuthProvidersMock.mockResolvedValue({ items: [] })
    fetchMeMock.mockResolvedValue({
      user: { user_id: 'setup-admin', username: 'setup-admin', display_name: 'Setup Admin', email: '' },
      roles: ['platform_admin'],
      permissions: ['*'],
      auth_source: 'local_password',
      break_glass: false,
    })

    const { LoginView } = await import('../src/pages/ops/LoginView')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter initialEntries={['/login?session_token=session-123&next=%2Fruntime-checks']}>
          <LoginView />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(navigateMock).toHaveBeenCalledWith('/runtime-checks', { replace: true })

    root.unmount()
    container.remove()
  })
})
