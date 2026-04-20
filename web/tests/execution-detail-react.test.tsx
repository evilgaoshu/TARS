// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'

const fetchExecutionMock = vi.fn()
const fetchExecutionOutputMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  fetchExecution: fetchExecutionMock,
  fetchExecutionOutput: fetchExecutionOutputMock,
  approveExecution: vi.fn(),
  modifyApproveExecution: vi.fn(),
  rejectExecution: vi.fn(),
  requestExecutionContext: vi.fn(),
}))

vi.mock('../src/hooks/useNotify', () => ({
  useNotify: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  }),
}))

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => {
      const map: Record<string, string> = {
        'executions.detail.loading': 'Loading execution workbench...',
        'executions.detail.notFound': 'Execution not found',
        'executions.detail.notFoundDesc': 'The selected execution could not be loaded.',
        'executions.detail.backToList': 'Back to executions',
        'executions.detail.eyebrow': 'Execution workbench',
        'executions.detail.refresh': 'Refresh result',
        'executions.detail.backToQueue': 'Back to queue',
        'executions.detail.stats.status': 'Status',
        'executions.detail.stats.statusDesc': 'Current execution lifecycle state.',
        'executions.detail.stats.target': 'Target',
        'executions.detail.stats.targetDesc': 'Target host or runtime surface.',
        'executions.detail.stats.created': 'Created',
        'executions.detail.stats.createdDesc': 'When the execution request entered the system.',
        'executions.detail.stats.outputSize': 'Output size',
        'executions.detail.stats.outputSizeDescFull': 'Full output size reported by backend.',
        'executions.detail.stats.outputSizeDescTruncated': 'Output was truncated in storage/export.',
        'executions.detail.sections.goldenPath': 'Golden path',
        'executions.detail.sections.goldenPathDesc': 'Lead with what will run, why it exists, the approval summary, current status, result, and the next observation step.',
        'executions.detail.sections.command': 'Requested command',
        'executions.detail.sections.commandDesc': 'Keep the raw command collapsed until someone needs to inspect or edit it.',
        'executions.detail.sections.output': 'Console output',
        'executions.detail.sections.outputDesc': 'Primary output surface for review and debugging.',
        'executions.detail.sections.outputNone': 'No console output recorded.',
        'executions.detail.sections.metadata': 'Execution metadata',
        'executions.detail.sections.metadataDesc': 'Context needed for approval, audit, and follow-up.',
        'executions.detail.sections.followThrough': 'Follow-through',
        'executions.detail.sections.followThroughDesc': 'Related objects reachable from this workbench.',
        'executions.detail.sections.openSession': 'Open linked session',
        'executions.detail.fields.what': 'What will execute',
        'executions.detail.fields.why': 'Why this run exists',
        'executions.detail.fields.risk': 'Risk level',
        'executions.detail.fields.approval': 'Approval note',
        'executions.detail.fields.status': 'Execution status',
        'executions.detail.fields.result': 'Result summary',
        'executions.detail.fields.nextAction': 'Next observation step',
        'executions.detail.fields.command': 'Command input',
        'executions.detail.fields.headline': 'Headline',
        'executions.detail.fields.meta.execution': 'Execution ID',
        'executions.detail.fields.meta.session': 'Session',
        'executions.detail.fields.meta.kind': 'Kind',
        'executions.detail.fields.meta.mode': 'Mode',
        'executions.detail.fields.meta.connector': 'Connector',
        'executions.detail.fields.meta.capability': 'Capability',
        'executions.detail.fields.meta.approvalGroup': 'Approval group',
        'executions.detail.fields.meta.exitCode': 'Exit code',
        'executions.detail.command.show': 'Show command input',
        'executions.detail.command.hide': 'Hide command input',
        'executions.detail.approval.autoRunWarning': 'This request is gated by operator approval and will not run automatically.',
      }
      return map[key] ?? fallback ?? key
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

describe('ExecutionDetailView', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    fetchExecutionMock.mockReset()
    fetchExecutionOutputMock.mockReset()
    fetchExecutionMock.mockResolvedValue({
      execution_id: 'exec-77',
      session_id: 'sess-22',
      command: 'systemctl restart checkout-api',
      status: 'pending',
      risk_level: 'critical',
      request_kind: 'execution',
      execution_mode: 'manual',
      connector_id: 'ssh-prod',
      capability_id: 'raw_shell',
      approval_group: 'prod-ops',
      target_host: 'checkout-api-1',
      output_bytes: 0,
      created_at: '2026-04-18T00:00:00Z',
      golden_summary: {
        headline: 'Restart checkout-api on prod node',
        approval: 'Customer checkouts are stalling; request approval before restarting the serving process.',
        result: 'Still pending operator approval before any command runs.',
        next_action: 'If approved, watch checkout latency and 5xx rate for 15 minutes.',
        risk: 'critical',
        command_preview: 'systemctl restart checkout-api',
      },
    })
    fetchExecutionOutputMock.mockResolvedValue({ chunks: [] })
  })

  it('puts the first-screen approval workbench summary ahead of raw command and output details', async () => {
    const { ExecutionDetailView } = await import('../src/pages/executions/ExecutionDetail')
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
          <MemoryRouter initialEntries={['/executions/exec-77']}>
            <Routes>
              <Route path="/executions/:id" element={<ExecutionDetailView />} />
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>,
      )
    })
    await flushAll()
    await flushAll()

    const text = container.textContent || ''

    expect(text).toContain('Golden path')
    expect(text).toContain('What will execute')
    expect(text).toContain('Why this run exists')
    expect(text).toContain('Risk level')
    expect(text).toContain('Approval note')
    expect(text).toContain('Execution status')
    expect(text).toContain('Result summary')
    expect(text).toContain('Next observation step')
    expect(text).toContain('This request is gated by operator approval and will not run automatically.')
    expect(text).toContain('Show command input')
    expect(text).not.toContain('systemctl restart checkout-api')
    expect(text.indexOf('What will execute')).toBeLessThan(text.indexOf('Requested command'))
    expect(text.indexOf('Approval note')).toBeLessThan(text.indexOf('Requested command'))
    expect(text.indexOf('Next observation step')).toBeLessThan(text.indexOf('Console output'))

    await act(async () => {
      root.unmount()
    })
    queryClient.clear()
    container.remove()
  })
})
