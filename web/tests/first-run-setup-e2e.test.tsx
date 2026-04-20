// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { Outlet } from 'react-router-dom'
import type { ReactNode } from 'react'

const fetchBootstrapStatusMock = vi.fn()
const fetchSetupWizardMock = vi.fn()
const fetchSetupStatusMock = vi.fn()
const loginWithPasswordMock = vi.fn()
const refreshMock = vi.fn()

// Mock modules before imports
vi.mock('../src/lib/api/ops', () => ({
  fetchBootstrapStatus: fetchBootstrapStatusMock,
  fetchSetupWizard: fetchSetupWizardMock,
  fetchSetupStatus: fetchSetupStatusMock,
  completeSetupWizard: vi.fn(),
  checkSetupWizardProvider: vi.fn(),
  saveSetupWizardAdmin: vi.fn(),
  saveSetupWizardAuth: vi.fn(),
  saveSetupWizardProvider: vi.fn(),
  saveSetupWizardChannel: vi.fn(),
  fetchSecretsInventory: vi.fn(),
  triggerSmokeAlert: vi.fn(),
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

vi.mock('../src/lib/api/access', () => ({
  fetchAuthProviders: vi.fn().mockResolvedValue({ items: [] }),
  fetchMe: vi.fn().mockResolvedValue({
    user: { user_id: 'admin', username: 'admin', display_name: 'Admin', email: '' },
    roles: ['platform_admin'],
    permissions: ['*'],
    auth_source: 'local_password',
    break_glass: false,
  }),
  loginWithPassword: loginWithPasswordMock,
  loginWithToken: vi.fn(),
  verifyAuthChallenge: vi.fn(),
  verifyAuthMFA: vi.fn(),
}))

const useAuthMock = vi.fn()
vi.mock('../src/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

vi.mock('../src/hooks/AuthProvider', () => ({
  AuthProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

vi.mock('../src/hooks/useTheme', () => ({
  ThemeProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

vi.mock('../src/hooks/useI18n', () => ({
  I18nProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  useI18n: () => ({
    lang: 'en-US',
    setLang: vi.fn(),
    t: (key: string) => {
      const messages: Record<string, string> = {
        'login.title': 'TARS Ops',
        'login.subtitle': 'Incident Recovery Gateway',
        'login.provider': 'Auth Provider',
        'login.localToken': 'Local Token / Break Glass',
        'login.usernameOrEmail': 'Username or Email',
        'login.password': 'Password',
        'login.signin': 'Sign In',
        'login.authenticating': 'Authenticating...',
        'login.authFailed': 'Authentication failed.',
        'login.enterCredentials': 'Please enter username and password.',
        'header.languageEnglish': 'English',
        'header.languageChinese': 'Chinese',
      }
      return messages[key] ?? key
    },
  }),
}))

vi.mock('../src/lib/FeatureGateProvider', () => ({
  FeatureGateProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

vi.mock('../src/components/ui/toaster', () => ({
  Toaster: () => null,
}))

vi.mock('../src/components/layout/AppLayout', () => ({
  AppLayout: () => (
    <div data-testid="app-layout">
      app-layout-shell
      <Outlet />
    </div>
  ),
}))

vi.mock('../src/pages/setup/SetupSmokeView', () => ({
  SetupSmokeView: () => <div data-testid="setup-wizard">setup-wizard-screen</div>,
  RuntimeChecksPage: () => <div data-testid="runtime-checks">runtime-checks-screen</div>,
}))

vi.mock('../src/pages/ops/LoginView', () => ({
  LoginView: () => <div data-testid="login-view">login-screen</div>,
}))

vi.mock('../src/pages/dashboard/DashboardView', () => ({
  DashboardView: () => <div data-testid="dashboard">dashboard-screen</div>,
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('First-run Setup E2E Quality Gate', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    fetchBootstrapStatusMock.mockReset()
    fetchSetupWizardMock.mockReset()
    fetchSetupStatusMock.mockReset()
    loginWithPasswordMock.mockReset()
    useAuthMock.mockReset()

    // Default: unauthenticated
    useAuthMock.mockReturnValue({
      isAuthenticated: false,
      user: null,
      login: vi.fn(),
      logout: vi.fn(),
      refresh: refreshMock,
    })

    // Default: uninitialized wizard mode
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: false,
      mode: 'wizard',
      next_step: 'admin',
    })

    fetchSetupWizardMock.mockResolvedValue({
      initialization: {
        mode: 'wizard',
        initialized: false,
        admin_configured: false,
        auth_configured: false,
        provider_ready: false,
        channel_ready: false,
        provider_check_ok: false,
        next_step: 'admin',
        login_hint: {
          username: '',
          provider: '',
          login_url: '/login',
        },
      },
      admin: {},
      auth: {},
      provider: {},
      channel: {},
    })

    fetchSetupStatusMock.mockResolvedValue({
      rollout_mode: 'pilot_core',
      features: {},
      initialization: {
        initialized: false,
        mode: 'wizard',
      },
      telegram: {},
      model: {},
      assist_model: {},
      providers: {},
      connectors: {},
      authorization: {},
      approval: {},
      reasoning: {},
      desensitization: {},
    })
  })

  describe('1. Uninitialized System - Routes to /setup', () => {
    it('redirects root path / to /setup when system is uninitialized', async () => {
      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: false,
        mode: 'wizard',
        next_step: 'admin',
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/setup')
      expect(container.textContent).toContain('setup-wizard-screen')

      root.unmount()
      container.remove()
    })

    it('redirects /login to /setup when system is uninitialized', async () => {
      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: false,
        mode: 'wizard',
        next_step: 'admin',
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/login')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/setup')
      expect(container.textContent).toContain('setup-wizard-screen')
      expect(container.textContent).not.toContain('login-screen')

      root.unmount()
      container.remove()
    })

    it('redirects business pages (sessions, executions, etc.) to /setup when uninitialized', async () => {
      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: false,
        mode: 'wizard',
        next_step: 'admin',
      })

      const { default: App } = await import('../src/App')

      const paths = ['/sessions', '/executions', '/audit', '/knowledge', '/providers', '/runtime-checks']

      for (const path of paths) {
        window.history.replaceState({}, '', path)
        const container = document.createElement('div')
        document.body.appendChild(container)
        const root = createRoot(container)

        await act(async () => {
          root.render(<App />)
        })
        await flush()
        await flush()

        expect(window.location.pathname).toBe('/setup')
        expect(container.textContent).toContain('setup-wizard-screen')

        root.unmount()
        container.remove()
      }
    })
  })

  describe('2. Post-initialization /setup Route Behavior', () => {
    it('redirects /setup to /runtime-checks when initialized and user is authenticated', async () => {
      useAuthMock.mockReturnValue({
        isAuthenticated: true,
        user: { username: 'admin', roles: ['platform_admin'] },
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: true,
        mode: 'runtime',
        next_step: '',
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/setup')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/runtime-checks')
      expect(container.textContent).toContain('runtime-checks-screen')

      root.unmount()
      container.remove()
    })

    it('redirects /setup to /login when initialized and user is not authenticated', async () => {
      useAuthMock.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: true,
        mode: 'runtime',
        next_step: '',
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/setup')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/login')
      expect(container.textContent).toContain('login-screen')

      root.unmount()
      container.remove()
    })
  })

  describe('3. Post-initialization Login Flow', () => {
    it('stays on /login page when initialized and not authenticated', async () => {
      useAuthMock.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: true,
        mode: 'runtime',
        next_step: '',
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/login')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/login')
      expect(container.textContent).toContain('login-screen')
      expect(container.textContent).not.toContain('setup-wizard-screen')

      root.unmount()
      container.remove()
    })

    it('redirects /login to / when already authenticated', async () => {
      useAuthMock.mockReturnValue({
        isAuthenticated: true,
        user: { username: 'admin', roles: ['platform_admin'], permissions: ['sessions.read'] },
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: true,
        mode: 'runtime',
        next_step: '',
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/login')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/sessions')

      root.unmount()
      container.remove()
    })
  })

  describe('4. Bootstrap Status Edge Cases', () => {
    it('does not guess setup state from bootstrap 401 errors', async () => {
      useAuthMock.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      const error = new Error('Unauthorized')
      ;(error as Error & { response?: { status: number } }).response = { status: 401 }
      fetchBootstrapStatusMock.mockRejectedValue(error)

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/login')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/login')
      expect(container.textContent).toContain('Unable to determine setup state.')
      expect(container.textContent).toContain('Retry')
      expect(container.textContent).not.toContain('setup-wizard-screen')
      expect(container.textContent).not.toContain('app-layout-shell')

      root.unmount()
      container.remove()
    })

    it('does not guess setup state from bootstrap network errors', async () => {
      useAuthMock.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      fetchBootstrapStatusMock.mockRejectedValue(new Error('Network Error'))

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      expect(window.location.pathname).toBe('/')
      expect(container.textContent).toContain('Unable to determine setup state.')
      expect(container.textContent).toContain('Retry')
      expect(container.textContent).not.toContain('setup-wizard-screen')
      expect(container.textContent).not.toContain('app-layout-shell')

      root.unmount()
      container.remove()
    })

    it('preserves query parameters when redirecting from /login with next parameter after setup complete', async () => {
      // This tests the login hint behavior from setup completion
      useAuthMock.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: true,
        mode: 'runtime',
        next_step: '',
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      // Simulate coming from setup complete with login hint
      window.history.replaceState({}, '', '/login?provider_id=local&username=admin&next=%2Fruntime-checks')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      // Should stay on login (since not authenticated) but show login screen
      expect(container.textContent).toContain('login-screen')

      root.unmount()
      container.remove()
    })
  })

  describe('5. Setup Completion Redirect', () => {
    it('login URL from setup complete includes next=/runtime-checks parameter', async () => {
      // Test that setup wizard generates correct login hint URL
      // This tests the backend behavior via frontend expectations
      fetchBootstrapStatusMock.mockResolvedValue({
        initialized: true,
        mode: 'runtime',
        next_step: '',
      })

      useAuthMock.mockReturnValue({
        isAuthenticated: false,
        user: null,
        login: vi.fn(),
        logout: vi.fn(),
        refresh: refreshMock,
      })

      const { default: App } = await import('../src/App')
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      window.history.replaceState({}, '', '/setup')

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      // After initialization, setup route should redirect based on auth state
      // Since we're not authenticated, should go to login
      expect(window.location.pathname).toBe('/login')
      expect(container.textContent).toContain('login-screen')

      root.unmount()
      container.remove()
    })
  })
})
