// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'

const fetchSessionsMock = vi.fn()
const bulkExportSessionsMock = vi.fn()
const useNotifyMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  fetchSessions: fetchSessionsMock,
  bulkExportSessions: bulkExportSessionsMock,
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
        'nav.sessions': 'Sessions',
        'common.newest': 'Newest',
        'common.oldest': 'Oldest',
        'common.exportJson': 'Export JSON',
        'common.selectPage': 'Select page',
        'common.service': 'Service',
        'common.host': 'Host',
        'common.source': 'Source',
        'common.updated': 'Updated',
        'common.nextAction': 'Next action',
        'common.openDetail': 'Open detail',
        'common.copyToClipboard': 'Copy to clipboard',
        'common.copiedTitle': 'Copied to clipboard',
        'common.copiedDescription': 'Copied',
        'sessions.hero.title': 'Sessions',
        'sessions.hero.description': 'Use Sessions as the on-call incident queue: see what matters most, why it matters, and what to do next.',
        'sessions.chip.active': '{{count}} active',
        'sessions.chip.highRisk': '{{count}} critical-risk',
        'sessions.chip.smoke': '{{count}} smoke',
        'sessions.exportSelected': 'Export incident JSON',
        'sessions.openChecks': 'Open runtime checks',
        'sessions.stats.visible': 'Visible incidents',
        'sessions.stats.visibleDesc': 'Queue items returned for this view.',
        'sessions.stats.attention': 'Needs action',
        'sessions.stats.attentionDesc': 'Queue items waiting on approval or direct operator intervention.',
        'sessions.stats.highRisk': 'High risk',
        'sessions.stats.highRiskDesc': 'Visible incidents still carrying warning or critical risk.',
        'sessions.stats.resolved': 'Mitigated / closed',
        'sessions.stats.resolvedDesc': 'Visible incidents already stabilized or wrapped up.',
        'sessions.search': 'Search incident / conclusion / next step / host...',
        'sessions.status.all': 'All statuses',
        'sessions.status.open': 'Pending evidence (open)',
        'sessions.status.analyzing': 'Pending evidence (analyzing)',
        'sessions.status.pendingApproval': 'Pending approval',
        'sessions.status.executing': 'Executing',
        'sessions.status.failed': 'Needs human intervention',
        'sessions.status.resolved': 'Resolved',
        'sessions.status.verifying': 'Observing',
        'sessions.backendStatus.pending_approval': 'Queue status: approval handoff',
        'sessions.sort.triage': 'Sort: Triage priority',
        'sessions.sort.updated': 'Sort: Updated',
        'sessions.sort.created': 'Sort: Created',
        'sessions.sort.status': 'Sort: Status',
        'sessions.queue.title': 'Incident queue',
        'sessions.queue.description': 'Rank by risk, action needed, and latest update so the next incident to open is obvious.',
        'sessions.section.title': 'Queue now',
        'sessions.section.description': 'Lead with headline, conclusion, risk, and next step. Keep service, host, source, and timestamps secondary.',
        'sessions.empty.title': 'No incidents in queue',
        'sessions.empty.description': 'Runtime checks or new alerts will place incidents here for triage.',
        'sessions.bulk.selected': '{{count}} incidents selected',
        'sessions.bulk.clear': 'Clear selection',
        'sessions.bulk.export': 'Export incident JSON',
        'sessions.smoke': 'smoke',
        'sessions.fallback.alertname': 'Untitled incident',
        'sessions.fallback.conclusion': 'No conclusion recorded yet.',
        'sessions.fallback.headline': 'Diagnosis is still gathering evidence.',
        'sessions.fallback.nextAction': 'Review evidence and decide whether to execute or request approval.',
        'sessions.fallback.source': 'Source not tagged',
        'sessions.fallback.host': 'Host not identified',
        'sessions.fallback.service': 'Service not identified',
        'sessions.openIncident': 'Open incident',
        'sessions.selectAll': 'Select visible incidents',
        'sessions.clearPageSelection': 'Clear queue selection',
        'sessions.fields.currentState': 'Current state',
        'sessions.fields.currentConclusion': 'Current conclusion',
        'sessions.fields.evidence': 'Evidence',
        'sessions.fields.risk': 'Risk level',
        'sessions.states.pendingEvidence': 'Pending evidence',
        'sessions.states.pendingApproval': 'Pending approval',
        'sessions.states.executing': 'Executing',
        'sessions.states.observing': 'Observing',
        'sessions.states.resolved': 'Resolved',
        'sessions.states.manualIntervention': 'Needs human intervention',
        'sessions.kickerLabel': 'incident',
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

describe('SessionList', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    useNotifyMock.mockReset()
    useNotifyMock.mockReturnValue({ success: vi.fn(), error: vi.fn() })
    fetchSessionsMock.mockReset()
    bulkExportSessionsMock.mockReset()
    fetchSessionsMock.mockResolvedValue({
      items: [
        {
          session_id: 'sess-closed',
          status: 'resolved',
          alert: {
            labels: {
              alertname: 'InventorySaturationMitigated',
              service: 'svc-inventory',
              instance: 'inventory-host-1',
              source: 'vmalert',
            },
          },
          golden_summary: {
            headline: 'Inventory stable after mitigation',
            conclusion: 'CPU saturation is back to baseline and checkout impact is no longer spreading.',
            risk: 'critical',
            next_action: 'Keep the fix under watch for another fifteen minutes.',
            evidence: ['Rollback canary stayed clean for fifteen minutes after deploy 418 was reverted.'],
          },
          executions: [],
          timeline: [
            {
              event: 'resolved',
              message: 'Mitigation completed',
              created_at: '2026-04-18T09:09:00Z',
            },
          ],
          is_smoke: false,
        },
        {
          session_id: 'sess-collecting',
          status: 'analyzing',
          alert: {
            labels: {
              alertname: 'CheckoutLatencyHigh',
              service: 'svc-checkout',
              instance: 'checkout-host-1',
              source: 'vmalert',
            },
          },
          golden_summary: {
            headline: 'Checkout latency still spiking',
            conclusion: 'Checkout remains degraded because inventory CPU is saturated and downstream timeouts are rising.',
            risk: 'critical',
            next_action: 'Compare database latency against the latest deployment.',
            evidence: ['Deploy build 418 reached checkout before the latency spike.'],
          },
          executions: [],
          timeline: [
            {
              event: 'analysis_started',
              message: 'Diagnosis started',
              created_at: '2026-04-18T09:06:00Z',
            },
          ],
          is_smoke: false,
        },
        {
          session_id: 'sess-pending',
          status: 'pending_approval',
          alert: {
            labels: {
              alertname: 'RollbackApprovalPending',
              service: 'svc-rollout',
              instance: 'rollout-host-1',
              source: 'vmalert',
            },
          },
          golden_summary: {
            headline: 'Rollback approval still pending',
            conclusion: 'A safer rollback already exists, but the queue is waiting on approval before execution can continue.',
            risk: 'critical',
            next_action: 'Approve the rollback or reject it with a safer command.',
            evidence: ['Rollback command and change window are ready for approval.'],
          },
          executions: [
            {
              execution_id: 'exec-1',
              status: 'pending',
            },
          ],
          timeline: [
            {
              event: 'approval_requested',
              message: 'Waiting for approval',
              created_at: '2026-04-18T09:04:00Z',
            },
          ],
          is_smoke: false,
        },
      ],
      total: 3,
      page: 1,
      limit: 20,
      has_next: false,
    })
  })

  it('frames the list as an incident queue and orders visible incidents by triage priority', async () => {
    const { SessionList } = await import('../src/pages/sessions/SessionList')
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
            <SessionList />
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await flushAll()
    await flushAll()

    const text = container.textContent || ''

    expect(text).toContain('Use Sessions as the on-call incident queue: see what matters most, why it matters, and what to do next.')
    expect(text).toContain('Incident queue')
    expect(text).toContain('Rank by risk, action needed, and latest update so the next incident to open is obvious.')
    expect(text).toContain('Queue now')
    expect(text).toContain('Needs action')
    expect(text).toContain('High risk')
    expect(text).toContain('Sort: Triage priority')
    expect(text).toContain('Rollback approval still pending')
    expect(text).toContain('Pending evidence')
    expect(text).toContain('Pending approval')
    expect(text).toContain('Resolved')
    expect(text).toContain('Checkout remains degraded because inventory CPU is saturated and downstream timeouts are rising.')
    expect(text).toContain('Deploy build 418 reached checkout before the latency spike.')
    expect(text).toContain('Risk level')
    expect(text).toContain('critical')
    expect(text).toContain('Approve the rollback or reject it with a safer command.')
    expect(text).toContain('svc-rollout')
    expect(text).toContain('vmalert')
    expect(text.indexOf('Rollback approval still pending')).toBeLessThan(text.indexOf('Current state'))
    expect(text.indexOf('Current state')).toBeLessThan(text.indexOf('Current conclusion'))
    expect(text.indexOf('Current conclusion')).toBeLessThan(text.indexOf('Evidence'))
    expect(text.indexOf('Evidence')).toBeLessThan(text.indexOf('Risk level'))
    expect(text.indexOf('Risk level')).toBeLessThan(text.indexOf('Next action'))
    expect(text.indexOf('Rollback approval still pending')).toBeLessThan(text.indexOf('Checkout latency still spiking'))
    expect(text.indexOf('Checkout latency still spiking')).toBeLessThan(text.indexOf('Inventory stable after mitigation'))
    expect(text.indexOf('Approve the rollback or reject it with a safer command.')).toBeLessThan(text.indexOf('svc-rollout'))
    expect(fetchSessionsMock).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      limit: 20,
      sort_by: 'triage',
      sort_order: 'desc',
    }))

    await act(async () => {
      root.unmount()
    })
    queryClient.clear()
    container.remove()
  })

  it('selects sessions by session_id and keeps bulk selection copy incident-focused', async () => {
    fetchSessionsMock.mockResolvedValue({
      items: [
        {
          session_id: 'sess-missing',
          status: 'pending_approval',
          alert: { labels: {} },
          golden_summary: {},
          executions: [],
          timeline: [],
          is_smoke: false,
        },
      ],
      total: 1,
      page: 1,
      limit: 20,
      has_next: false,
    })

    const { SessionList } = await import('../src/pages/sessions/SessionList')
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
            <SessionList />
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await flushAll()
    await flushAll()

    const selectPageButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Select visible incidents'))
    const exportButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Export incident JSON'))
    expect(selectPageButton).toBeTruthy()
    expect(exportButton?.hasAttribute('disabled')).toBe(true)

    await act(async () => {
      selectPageButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await flushAll()

    const text = container.textContent || ''

    expect(exportButton?.hasAttribute('disabled')).toBe(false)
    expect(text).toContain('1 incidents selected')
    expect(text).toContain('Clear selection')
    expect(text).toContain('Untitled incident')
    expect(text).toContain('Service not identified')
    expect(text).toContain('Host not identified')
    expect(text).toContain('Source not tagged')
    expect(text).toContain('No conclusion recorded yet.')
    expect(text).toContain('incident')
    expect(text).toContain('Queue status: approval handoff')
    expect(text).not.toContain('Untitled alert')
    expect(text).not.toContain('platform')
    expect(text).not.toContain('unassigned')
    expect(text).not.toContain('pending approval')
    expect(text).not.toContain('1 item selected')

    await act(async () => {
      root.unmount()
    })
    queryClient.clear()
    container.remove()
  })

  it('uses an incident-queue empty state instead of a generic no-results message', async () => {
    fetchSessionsMock.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      limit: 20,
      has_next: false,
    })

    const { SessionList } = await import('../src/pages/sessions/SessionList')
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
            <SessionList />
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await flushAll()
    await flushAll()

    const text = container.textContent || ''

    expect(text).toContain('No incidents in queue')
    expect(text).toContain('Runtime checks or new alerts will place incidents here for triage.')
    expect(text).not.toContain('No sessions found')

    await act(async () => {
      root.unmount()
    })
    queryClient.clear()
    container.remove()
  })
})
