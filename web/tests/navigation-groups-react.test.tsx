// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'

const useCapabilitiesMock = vi.fn()
const useAuthMock = vi.fn()

vi.mock('../src/lib/FeatureGateContext', () => ({
  useCapabilities: () => useCapabilitiesMock(),
}))

vi.mock('../src/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

describe('navigation groups', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    useCapabilitiesMock.mockReset()
    useCapabilitiesMock.mockReturnValue({ capabilities: {} })
    useAuthMock.mockReset()
    useAuthMock.mockReturnValue({ user: { permissions: [] } })
  })

  it('shows ops navigation only when the user has ops access permissions', async () => {
    const { useNavigationGroups } = await import('../src/components/layout/navigation')
    const seenRoutes: string[][] = []

    const Probe = () => {
      seenRoutes.push(useNavigationGroups().flatMap((group) => group.routes.map((route) => route.id)))
      return null
    }

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<Probe />)
    })

    expect(seenRoutes.at(-1)).not.toContain('ops')

    useAuthMock.mockReturnValue({ user: { permissions: ['ssh_credentials.read'] } })

    await act(async () => {
      root.render(<Probe />)
    })

    expect(seenRoutes.at(-1)).toContain('ops')

    root.unmount()
    container.remove()
  })

  it('keeps sessions and executions as the primary runtime group and demotes object pages to later groups', async () => {
    useAuthMock.mockReturnValue({
      user: {
        permissions: ['sessions.read', 'executions.read', 'connectors.read', 'platform.read', 'skills.read', 'providers.read', 'channels.read', 'audit.read', 'outbox.read', 'configs.read', 'org.read', 'auth.read'],
      },
    })

    const { useNavigationGroups } = await import('../src/components/layout/navigation')
    const seenGroups: Array<{ id: string; routes: string[] }[]> = []

    const Probe = () => {
      seenGroups.push(
        useNavigationGroups().map((group) => ({
          id: group.id,
          routes: group.routes.map((route) => route.id),
        })),
      )
      return null
    }

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<Probe />)
    })

    const groups = seenGroups.at(-1) || []
    expect(groups[0]).toEqual({
      id: 'runtime',
      routes: ['sessions', 'executions', 'dashboard', 'runtime-checks'],
    })
    expect(groups[1]?.id).toBe('delivery')
    expect(groups[2]?.id).toBe('platform')
    expect(groups[3]?.id).toBe('governance')
    expect(groups[5]).toEqual({
      id: 'docs',
      routes: ['docs'],
    })

    root.unmount()
    container.remove()
  })
})
