// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'

const fetchSetupStatusMock = vi.fn()
const useAuthMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  fetchSetupStatus: fetchSetupStatusMock,
}))

vi.mock('../src/hooks/useAuth', () => ({
  useAuth: () => useAuthMock(),
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('FeatureGateProvider', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    fetchSetupStatusMock.mockReset()
    useAuthMock.mockReset()
  })

  it('does not fetch setup status while unauthenticated', async () => {
    useAuthMock.mockReturnValue({ isAuthenticated: false, user: null })

    const { FeatureGateProvider } = await import('../src/lib/FeatureGateProvider')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <FeatureGateProvider>
          <div>child-content</div>
        </FeatureGateProvider>,
      )
    })
    await flush()
    await flush()

    expect(fetchSetupStatusMock).not.toHaveBeenCalled()
    expect(container.textContent).toContain('child-content')

    root.unmount()
    container.remove()
  })
})
