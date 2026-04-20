// @vitest-environment jsdom

import React from 'react'
import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'

const createAutomationMock = vi.fn()
const fetchAutomationsMock = vi.fn()
const fetchConnectorsMock = vi.fn()
const fetchSkillsMock = vi.fn()
const runAutomationNowMock = vi.fn()
const setAutomationEnabledMock = vi.fn()
const updateAutomationMock = vi.fn()
const fetchAgentRolesMock = vi.fn()
const listTriggersMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  createAutomation: createAutomationMock,
  fetchAutomations: fetchAutomationsMock,
  fetchConnectors: fetchConnectorsMock,
  fetchSkills: fetchSkillsMock,
  getApiErrorMessage: (_error: unknown, fallback: string) => fallback,
  runAutomationNow: runAutomationNowMock,
  setAutomationEnabled: setAutomationEnabledMock,
  updateAutomation: updateAutomationMock,
}))

vi.mock('../src/lib/api/agent-roles', () => ({
  fetchAgentRoles: fetchAgentRolesMock,
}))

vi.mock('../src/lib/api/triggers', () => ({
  listTriggers: listTriggersMock,
}))

vi.mock('../src/components/operator/GuidedFormDialog', () => ({
  GuidedFormDialog: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

describe('AutomationsPage', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    createAutomationMock.mockReset()
    fetchAutomationsMock.mockReset()
    fetchConnectorsMock.mockReset()
    fetchSkillsMock.mockReset()
    runAutomationNowMock.mockReset()
    setAutomationEnabledMock.mockReset()
    updateAutomationMock.mockReset()
    fetchAgentRolesMock.mockReset()
    listTriggersMock.mockReset()

    fetchAutomationsMock.mockResolvedValue({
      items: [
        {
          id: 'daily-health',
          display_name: 'Daily Health',
          type: 'skill',
          target_ref: 'health.check',
          schedule: '@every 15m',
          enabled: true,
          governance_policy: 'auto',
        },
      ],
    })
    fetchSkillsMock.mockResolvedValue({ items: [] })
    fetchConnectorsMock.mockResolvedValue({ items: [] })
    fetchAgentRolesMock.mockResolvedValue({ items: [] })
    listTriggersMock.mockResolvedValue({
      items: [
        {
          id: 'trg-daily-health',
          display_name: 'Notify Daily Health',
          event_type: 'on_execution_completed',
          channel_id: 'inbox-primary',
          enabled: true,
          automation_job_id: 'daily-health',
        },
      ],
    })
  })

  it('shows linked trigger counts for each automation', async () => {
    const { AutomationsPage } = await import('../src/pages/automations/AutomationsPage')

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <MemoryRouter>
          <AutomationsPage />
        </MemoryRouter>,
      )
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('Linked triggers')
    expect(container.textContent).toContain('1')
    expect(container.textContent).toContain('Notify Daily Health')

    root.unmount()
    container.remove()
  })
})
