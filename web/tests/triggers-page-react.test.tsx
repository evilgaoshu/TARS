// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'

const getTriggerMock = vi.fn()
const listTriggersMock = vi.fn()
const setTriggerEnabledMock = vi.fn()
const upsertTriggerMock = vi.fn()
const updateTriggerMock = vi.fn()
const fetchChannelsMock = vi.fn()
const fetchAutomationsMock = vi.fn()

vi.mock('../src/lib/api/triggers', () => ({
  getTrigger: getTriggerMock,
  listTriggers: listTriggersMock,
  setTriggerEnabled: setTriggerEnabledMock,
  upsertTrigger: upsertTriggerMock,
  updateTrigger: updateTriggerMock,
}))

vi.mock('../src/lib/api/access', () => ({
  fetchChannels: fetchChannelsMock,
}))

vi.mock('../src/lib/api/ops', () => ({
  fetchAutomations: fetchAutomationsMock,
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('TriggersPage', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    getTriggerMock.mockReset()
    listTriggersMock.mockReset()
    setTriggerEnabledMock.mockReset()
    upsertTriggerMock.mockReset()
    updateTriggerMock.mockReset()
    fetchChannelsMock.mockReset()
    fetchAutomationsMock.mockReset()

    listTriggersMock.mockResolvedValue({
      items: [
        {
          id: 'trg-1',
          display_name: 'Inbox incident updates',
          event_type: 'on_execution_completed',
          channel_id: 'inbox-primary',
          enabled: true,
          automation_job_id: 'daily-health',
        },
      ],
    })
    fetchChannelsMock.mockResolvedValue({ items: [{ id: 'inbox-primary', name: 'Primary Inbox', kind: 'in_app_inbox' }] })
    fetchAutomationsMock.mockResolvedValue({ items: [{ id: 'daily-health', display_name: 'Daily Health' }] })
    getTriggerMock.mockResolvedValue({
      id: 'trg-1',
      display_name: 'Inbox incident updates',
      description: '',
      event_type: 'on_execution_completed',
      channel_id: 'inbox-primary',
      governance: 'delivery_policy',
      template_id: 'diagnosis-zh-CN',
      target_audience: '',
      filter_expr: '',
      cooldown_sec: 30,
      enabled: true,
      automation_job_id: 'daily-health',
    })
    updateTriggerMock.mockImplementation(async (_id: string, payload: unknown) => payload)
  })

  it('shows automation ownership and preserves it on save', async () => {
    const { TriggersPage } = await import('../src/pages/triggers/TriggersPage')

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<TriggersPage />)
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('Daily Health')

    const saveButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Save rule'))
    expect(saveButton).toBeTruthy()

    await act(async () => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flush()

    expect(updateTriggerMock).toHaveBeenCalledTimes(1)
    const payload = updateTriggerMock.mock.calls[0]?.[1] as { automation_job_id?: string }
    expect(payload.automation_job_id).toBe('daily-health')

    root.unmount()
    container.remove()
  })
})
