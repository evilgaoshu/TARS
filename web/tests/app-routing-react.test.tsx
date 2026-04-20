// @vitest-environment jsdom

import React from 'react'
import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { Outlet } from 'react-router-dom'

const fetchBootstrapStatusMock = vi.fn()
const useAuthMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  fetchBootstrapStatus: fetchBootstrapStatusMock,
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

vi.mock('../src/hooks/AuthProvider', () => ({
  AuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('../src/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

vi.mock('../src/hooks/useTheme', () => ({
  ThemeProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('../src/components/layout/AppLayout', () => ({
  AppLayout: () => <div>app-layout-shell<Outlet /></div>,
}))

vi.mock('../src/hooks/useI18n', () => ({
  I18nProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('../src/lib/FeatureGateProvider', () => ({
  FeatureGateProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))

vi.mock('../src/components/ui/toaster', () => ({
  Toaster: () => null,
}))

vi.mock('../src/pages/setup/SetupSmokeView', () => ({
  SetupSmokeView: () => <div>setup-wizard-screen</div>,
  RuntimeChecksPage: () => <div>runtime-checks-screen</div>,
}))

vi.mock('../src/pages/ops/LoginView', () => ({
  LoginView: () => <div>login-screen</div>,
}))

vi.mock('../src/pages/dashboard/DashboardView', () => ({
  DashboardView: () => <div>dashboard-screen</div>,
}))

vi.mock('../src/pages/sessions/SessionList', () => ({
  SessionList: () => <div>sessions-screen</div>,
}))

vi.mock('../src/pages/executions/ExecutionList', () => ({
  ExecutionList: () => <div>executions-screen</div>,
}))

vi.mock('../src/pages/ops/OpsActionView', () => ({
  OpsActionView: () => <div>ops-screen</div>,
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('App root routing', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    fetchBootstrapStatusMock.mockReset()
    useAuthMock.mockReset()
    useAuthMock.mockReturnValue({
      isAuthenticated: false,
      user: null,
    })
    window.history.replaceState({}, '', '/')
  })

  it('redirects unauthenticated root visits to /setup when initialization is still in wizard mode', async () => {
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: false,
      mode: 'wizard',
      next_step: 'admin',
    })

    const { default: App } = await import('../src/App')
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
    expect(container.textContent).not.toContain('login-screen')

    root.unmount()
    container.remove()
  })

  it('redirects login and protected paths to /setup while initialization is still in wizard mode', async () => {
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: false,
      mode: 'wizard',
      next_step: 'admin',
    })

    const { default: App } = await import('../src/App')

    for (const path of ['/login', '/providers', '/runtime-checks']) {
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
      expect(container.textContent).not.toContain('login-screen')

      root.unmount()
      container.remove()
    }
  })

  it('keeps unauthenticated users on login and redirects setup/runtime checks to login after initialization', async () => {
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: true,
      mode: 'runtime',
    })

    const { default: App } = await import('../src/App')

    for (const path of ['/login', '/setup', '/runtime-checks']) {
      window.history.replaceState({}, '', path)
      const container = document.createElement('div')
      document.body.appendChild(container)
      const root = createRoot(container)

      await act(async () => {
        root.render(<App />)
      })
      await flush()
      await flush()

      if (path === '/login') {
        expect(window.location.pathname).toBe('/login')
        expect(container.textContent).toContain('login-screen')
      } else {
        expect(window.location.pathname).toBe('/login')
      }
      expect(container.textContent).not.toContain('setup-wizard-screen')

      root.unmount()
      container.remove()
    }
  })

  it('redirects authenticated setup visits to /runtime-checks after initialization', async () => {
    useAuthMock.mockReturnValue({
      isAuthenticated: true,
      user: { username: 'alice' },
    })
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: true,
      mode: 'runtime',
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
    expect(container.textContent).not.toContain('setup-wizard-screen')

    root.unmount()
    container.remove()
  })

  it('lands authenticated operators on /sessions instead of the dashboard when sessions are available', async () => {
    useAuthMock.mockReturnValue({
      isAuthenticated: true,
      user: { username: 'alice', permissions: ['sessions.read'] },
    })
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: true,
      mode: 'runtime',
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

    expect(window.location.pathname).toBe('/sessions')
    expect(container.textContent).toContain('sessions-screen')
    expect(container.textContent).not.toContain('dashboard-screen')

    root.unmount()
    container.remove()
  })

  it('falls back to /executions when sessions are unavailable but execution access exists', async () => {
    useAuthMock.mockReturnValue({
      isAuthenticated: true,
      user: { username: 'alice', permissions: ['executions.read'] },
    })
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: true,
      mode: 'runtime',
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

    expect(window.location.pathname).toBe('/executions')
    expect(container.textContent).toContain('executions-screen')
    expect(container.textContent).not.toContain('dashboard-screen')

    root.unmount()
    container.remove()
  })

  it('shows a bootstrap error instead of guessing setup state when bootstrap status fails', async () => {
    fetchBootstrapStatusMock.mockRejectedValue(new Error('bootstrap unavailable'))

    const { default: App } = await import('../src/App')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    window.history.replaceState({}, '', '/providers')

    await act(async () => {
      root.render(<App />)
    })
    await flush()
    await flush()

    expect(window.location.pathname).toBe('/providers')
    expect(container.textContent).toContain('Unable to determine setup state.')
    expect(container.textContent).toContain('Retry')
    expect(container.textContent).not.toContain('setup-wizard-screen')
    expect(container.textContent).not.toContain('login-screen')
    expect(container.textContent).not.toContain('app-layout-shell')

    root.unmount()
    container.remove()
  })

  it('allows authenticated ssh credential readers to reach ops secrets after initialization', async () => {
    useAuthMock.mockReturnValue({
      isAuthenticated: true,
      user: {
        username: 'alice',
        permissions: ['ssh_credentials.read'],
      },
    })
    fetchBootstrapStatusMock.mockResolvedValue({
      initialized: true,
      mode: 'runtime',
    })

    const { default: App } = await import('../src/App')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    window.history.replaceState({}, '', '/ops?tab=secrets')

    await act(async () => {
      root.render(<App />)
    })
    await flush()
    await flush()

    expect(window.location.pathname).toBe('/ops')
    expect(window.location.search).toBe('?tab=secrets')
    expect(container.textContent).toContain('ops-screen')

    root.unmount()
    container.remove()
  })
})
