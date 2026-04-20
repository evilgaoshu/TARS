// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'

const fetchExtensionsMock = vi.fn()
const validateExtensionBundleMock = vi.fn()
const validateExtensionCandidateMock = vi.fn()
const importExtensionCandidateMock = vi.fn()
const reviewExtensionCandidateMock = vi.fn()
const createExtensionCandidateMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  fetchExtensions: fetchExtensionsMock,
  validateExtensionBundle: validateExtensionBundleMock,
  validateExtensionCandidate: validateExtensionCandidateMock,
  importExtensionCandidate: importExtensionCandidateMock,
  reviewExtensionCandidate: reviewExtensionCandidateMock,
  createExtensionCandidate: createExtensionCandidateMock,
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('ExtensionsPage', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    vi.stubGlobal('ResizeObserver', class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    })
    fetchExtensionsMock.mockReset()
    fetchExtensionsMock.mockResolvedValue({ items: [], total: 0 })
  })

  it('renders Extensions Center title with English subtitle', async () => {
    const { ExtensionsPage } = await import('../src/pages/extensions/ExtensionsPage')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <ExtensionsPage />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('Extensions Center')
    // English subtitle, no Chinese
    expect(container.textContent).not.toContain('生成受治理')
    expect(container.textContent).toContain('Generate governed skill bundle candidates')

    root.unmount()
    container.remove()
  })

  it('shows + New button and candidate registry sidebar', async () => {
    const { ExtensionsPage } = await import('../src/pages/extensions/ExtensionsPage')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <ExtensionsPage />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const newBtn = Array.from(container.querySelectorAll('button')).find((b) => b.textContent?.includes('+ New'))
    expect(newBtn).toBeTruthy()

    root.unmount()
    container.remove()
  })

  it('shows Candidate Composer with TagInput (no CSV label) when + New clicked', async () => {
    const { ExtensionsPage } = await import('../src/pages/extensions/ExtensionsPage')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <ExtensionsPage />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    const newBtn = Array.from(container.querySelectorAll('button')).find((b) => b.textContent?.includes('+ New'))
    await act(async () => {
      newBtn?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    // Should not show raw CSV label
    expect(container.textContent).not.toContain('Tags (CSV)')
    // Composer subtitle should be in English
    expect(container.textContent).not.toContain('填写字段后点击')
    expect(container.textContent).toContain('Candidate Composer')

    root.unmount()
    container.remove()
  })
})
