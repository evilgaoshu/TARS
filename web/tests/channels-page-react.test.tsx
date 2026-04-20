// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'

const fetchChannelsMock = vi.fn()
const fetchChannelMock = vi.fn()
const fetchAccessConfigMock = vi.fn()
const updateChannelMock = vi.fn()
const createChannelMock = vi.fn()
const setChannelEnabledMock = vi.fn()
const notify = {
  success: vi.fn(),
  error: vi.fn(),
  warn: vi.fn(),
}
const i18n = {
  t: (_key: string, fallback?: string) => fallback ?? _key,
  lang: 'en-US',
  setLang: vi.fn(),
}

vi.mock('../src/lib/api/access', () => ({
  fetchChannels: fetchChannelsMock,
  fetchChannel: fetchChannelMock,
  fetchAccessConfig: fetchAccessConfigMock,
  updateChannel: updateChannelMock,
  createChannel: createChannelMock,
  setChannelEnabled: setChannelEnabledMock,
}))

vi.mock('../src/hooks/ui/useNotify', () => ({
  useNotify: () => notify,
}))

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => i18n,
}))

vi.mock('../src/lib/FeatureGateContext', () => ({
  useCapabilities: () => ({
    capabilities: {},
    loading: false,
    refresh: async () => {},
  }),
}))

vi.mock('../src/components/operator/GuidedFormDialog', () => ({
  GuidedFormDialog: ({ children, onConfirm, confirmLabel = 'Save' }: { children: React.ReactNode; onConfirm?: () => void; confirmLabel?: string }) => (
    <div>
      {children}
      {onConfirm ? <button type="button" onClick={onConfirm}>{confirmLabel}</button> : null}
    </div>
  ),
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('ChannelsPage edit form', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    vi.stubGlobal('ResizeObserver', class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    })
    fetchChannelsMock.mockReset()
    fetchChannelMock.mockReset()
    fetchAccessConfigMock.mockReset()
    updateChannelMock.mockReset()
    createChannelMock.mockReset()
    setChannelEnabledMock.mockReset()

    fetchChannelsMock.mockResolvedValue({
      items: [
        {
          id: 'web-chat-primary',
          name: 'Web Chat Primary',
          kind: 'web_chat',
          type: 'web_chat',
          target: 'default',
          enabled: true,
          usages: ['conversation_entry'],
          capabilities: ['supports_session_reply'],
          linked_users: [],
        },
      ],
      page: 1,
      limit: 20,
      total: 1,
      has_next: false,
    })
    fetchAccessConfigMock.mockResolvedValue({ config: { users: [] }, path: '' })
    fetchChannelMock.mockResolvedValue({
      id: 'web-chat-primary',
      name: 'Web Chat Primary',
      kind: 'web_chat',
      type: 'web_chat',
      target: 'default',
      enabled: true,
      usages: ['conversation_entry'],
      capabilities: ['supports_session_reply'],
      linked_users: [],
    })
    updateChannelMock.mockImplementation(async (_id: string, payload: unknown) => payload)
  })

  it('preserves distinct usages and capabilities when editing and saving', async () => {
    const { ChannelsPage } = await import('../src/pages/channels/ChannelsPage')

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<ChannelsPage />)
    })
    await flush()

    // debug fallback if card actions do not render as expected
    const editButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('action.edit'))
    expect(editButton).toBeTruthy()

    await act(async () => {
      editButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    const usagesCsvInput = Array.from(container.querySelectorAll('input')).find((input) => input.placeholder === 'approval, notifications')
    expect(usagesCsvInput).toBeFalsy()

    const attachmentsToggle = container.querySelector('[aria-label="attachments"]') as HTMLElement | null
    expect(attachmentsToggle).toBeTruthy()

    await act(async () => {
      attachmentsToggle?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Save'))
    expect(saveButton).toBeTruthy()

    await act(async () => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    expect(updateChannelMock).toHaveBeenCalledTimes(1)
    const payload = updateChannelMock.mock.calls[0]?.[1] as { usages?: string[]; capabilities?: string[] }
    expect(payload.usages).toEqual(['conversation_entry'])
    expect(payload.capabilities).toEqual(['supports_session_reply', 'attachments'])

    root.unmount()
    container.remove()
  })
})
