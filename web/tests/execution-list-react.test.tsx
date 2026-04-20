// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'

const fetchExecutionsMock = vi.fn()
const bulkExportExecutionsMock = vi.fn()
const useNotifyMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  fetchExecutions: fetchExecutionsMock,
  bulkExportExecutions: bulkExportExecutionsMock,
  getBlobApiErrorMessage: async (_error: unknown, fallback: string) => fallback,
  parseExportBlob: vi.fn(),
}))

vi.mock('../src/hooks/ui/useNotify', () => ({
  useNotify: () => useNotifyMock(),
}))

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => ({
    t: (key: string, paramsOrFallback?: Record<string, unknown> | string) => {
      const fallback = typeof paramsOrFallback === 'string' ? paramsOrFallback : undefined
      const params = typeof paramsOrFallback === 'object' ? paramsOrFallback : undefined
      const table: Record<string, string> = {
        'nav.executions': 'Executions',
        'common.newest': 'Newest',
        'common.oldest': 'Oldest',
        'common.exportJson': 'Export JSON',
        'common.selectPage': 'Select page',
        'common.host': 'Host',
        'common.updated': 'Updated',
        'common.openDetail': 'Open detail',
        'common.nextAction': 'Next action',
        'executions.hero.title': 'Executions',
        'executions.hero.description': 'Treat executions as an operator queue for approvals and active runs, not a passive object table.',
        'executions.exportSelected': 'Export selected',
        'executions.backToIncidents': 'Back to incidents',
        'executions.stats.visible': 'Visible executions',
        'executions.stats.visibleDesc': 'Total execution requests for the current query.',
        'executions.stats.pending': '{{count}} pending approvals',
        'executions.stats.pendingDesc': 'Still waiting for approval, executing, or mid-flight.',
        'executions.stats.completed': '{{count}} completed',
        'executions.stats.completedDesc': 'Runs that already finished successfully in the visible page.',
        'executions.stats.failed': '{{count}} failed or rejected',
        'executions.stats.failedDesc': 'Runs that need follow-up because they failed, timed out, or were rejected.',
        'executions.search': 'Search execution / host / command / capability...',
        'executions.status.all': 'All statuses',
        'executions.status.pending': 'Pending',
        'executions.status.approved': 'Approved',
        'executions.status.executing': 'Executing',
        'executions.status.completed': 'Completed',
        'executions.status.failed': 'Failed',
        'executions.status.timeout': 'Timeout',
        'executions.status.rejected': 'Rejected',
        'executions.sort.created': 'Sort: Created',
        'executions.sort.completed': 'Sort: Completed',
        'executions.sort.status': 'Sort: Status',
        'executions.sort.triage': 'Sort: Triage priority',
        'executions.queue.title': 'Execution queue',
        'executions.queue.description': 'Show approval state, risk, next action, and target context first.',
        'executions.section.title': 'Runs',
        'executions.section.description': 'Approval and execution outcomes presented as an operator lane.',
        'executions.empty.description': 'Trigger actions from sessions or wait for approval requests',
        'executions.empty.title': 'No pending executions',
        'executions.fallback.host': 'Pending target',
        'executions.fallback.kind': 'Execution request',
        'executions.fallback.result': 'Waiting for execution update.',
        'executions.fields.kind': 'Kind',
        'executions.selectAll': 'Select all executions',
        'executions.clearPageSelection': 'Clear page selection',
        'executions.kickerLabel': 'run',
      }
      const template = table[key] ?? fallback ?? key
      if (!params) {
        return template
      }
      return template.replace(/\{\{(\w+)\}\}/g, (_, token: string) => String(params[token] ?? ''))
    },
  }),
}))

async function flushAll() {
  await act(async () => {
    await Promise.resolve()
    await new Promise((resolve) => setTimeout(resolve, 10))
  })
  await act(async () => {
    await Promise.resolve()
  })
}

describe('ExecutionList', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    useNotifyMock.mockReset()
    useNotifyMock.mockReturnValue({ success: vi.fn(), error: vi.fn() })
    fetchExecutionsMock.mockReset()
    bulkExportExecutionsMock.mockReset()
    fetchExecutionsMock.mockResolvedValue({
      items: [
        {
          execution_id: 'exec-1',
          session_id: 'sess-1',
          status: 'pending',
          request_kind: 'execution',
          risk_level: 'critical',
          target_host: 'checkout-api-1',
          created_at: '2026-04-18T00:00:00Z',
          golden_summary: {
            headline: 'Restart checkout worker',
            result: 'Waiting for approval before restarting the degraded checkout worker.',
            next_action: 'Review the blast radius and approve if customer impact is growing.',
            risk: 'critical',
          },
        },
      ],
      total: 1,
      page: 1,
      limit: 20,
      has_next: false,
    })
  })

  it('renders execution queue cards and keeps result plus next action ahead of detail navigation', async () => {
    const { ExecutionList } = await import('../src/pages/executions/ExecutionList')
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <ExecutionList />
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await flushAll()
    await flushAll()

    const text = container.textContent || ''

    expect(text).toContain('Treat executions as an operator queue for approvals and active runs, not a passive object table.')
    expect(text).toContain('Execution queue')
    expect(text).toContain('Restart checkout worker')
    expect(text).toContain('Waiting for approval before restarting the degraded checkout worker.')
    expect(text).toContain('Review the blast radius and approve if customer impact is growing.')
    expect(text.indexOf('Waiting for approval before restarting the degraded checkout worker.')).toBeLessThan(text.indexOf('Open detail'))
    expect(fetchExecutionsMock).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      limit: 20,
      sort_by: 'triage',
      sort_order: 'desc',
    }))
    expect(text).toContain('Sort: Triage priority')
    expect(Array.from(container.querySelectorAll('select option')).some((option) => option.getAttribute('value') === 'triage')).toBe(true)

    await act(async () => {
      root.unmount()
    })
    queryClient.clear()
    container.remove()
  })

  it('selects executions by execution_id and renders localized fallbacks', async () => {
    fetchExecutionsMock.mockResolvedValue({
      items: [
        {
          execution_id: 'exec-missing',
          session_id: 'sess-1',
          status: 'pending',
          created_at: '2026-04-18T00:00:00Z',
          golden_summary: {},
        },
      ],
      total: 1,
      page: 1,
      limit: 20,
      has_next: false,
    })

    const { ExecutionList } = await import('../src/pages/executions/ExecutionList')
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <ExecutionList />
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await flushAll()
    await flushAll()

    const selectPageButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Select page'))
    const exportButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Export selected'))
    expect(selectPageButton).toBeTruthy()
    expect(exportButton?.hasAttribute('disabled')).toBe(true)

    await act(async () => {
      selectPageButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flushAll()

    const text = container.textContent || ''

    expect(exportButton?.hasAttribute('disabled')).toBe(false)
    expect(text).toContain('Waiting for execution update.')
    expect(text).toContain('Pending target')
    expect(text).toContain('Execution request')
    expect(text).toContain('run')
    expect(text).not.toContain('infrastructure')

    await act(async () => {
      root.unmount()
    })
    queryClient.clear()
    container.remove()
  })

  it('surfaces the queue narrative with approval reason, risk, status, result, and observation advice ahead of detail affordances', async () => {
    fetchExecutionsMock.mockResolvedValue({
      items: [
        {
          execution_id: 'exec-critical-1',
          session_id: 'sess-9',
          status: 'pending',
          request_kind: 'execution',
          risk_level: 'critical',
          target_host: 'payments-db-1',
          command: 'systemctl restart payments-db',
          created_at: '2026-04-18T00:00:00Z',
          golden_summary: {
            headline: 'Restart payments database',
            approval: 'High-risk restart requested after repeated connection pool exhaustion.',
            result: 'Blocked until an operator approves the restart window.',
            next_action: 'Observe DB saturation, replica lag, and customer checkout failures for 15 minutes after any approval.',
            risk: 'critical',
          },
        },
      ],
      total: 1,
      page: 1,
      limit: 20,
      has_next: false,
    })

    const { ExecutionList } = await import('../src/pages/executions/ExecutionList')
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <ExecutionList />
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await flushAll()
    await flushAll()

    const text = container.textContent || ''

    expect(text).toContain('Restart payments database')
    expect(text).toContain('High-risk restart requested after repeated connection pool exhaustion.')
    expect(text).toContain('Blocked until an operator approves the restart window.')
    expect(text).toContain('Observe DB saturation, replica lag, and customer checkout failures for 15 minutes after any approval.')
    expect(text).toContain('critical')
    expect(text).toContain('pending')
    expect(text.indexOf('High-risk restart requested after repeated connection pool exhaustion.')).toBeLessThan(text.indexOf('Open detail'))
    expect(text.indexOf('Blocked until an operator approves the restart window.')).toBeLessThan(text.indexOf('Open detail'))
    expect(text.indexOf('Observe DB saturation, replica lag, and customer checkout failures for 15 minutes after any approval.')).toBeLessThan(text.indexOf('Open detail'))

    await act(async () => {
      root.unmount()
    })
    queryClient.clear()
    container.remove()
  })
})
