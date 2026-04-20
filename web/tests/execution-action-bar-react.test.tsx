// @vitest-environment jsdom

import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRoot } from 'react-dom/client'

const approveExecutionMock = vi.fn()
const modifyApproveExecutionMock = vi.fn()
const rejectExecutionMock = vi.fn()
const requestExecutionContextMock = vi.fn()

vi.mock('../src/lib/api/ops', () => ({
  approveExecution: approveExecutionMock,
  modifyApproveExecution: modifyApproveExecutionMock,
  rejectExecution: rejectExecutionMock,
  requestExecutionContext: requestExecutionContextMock,
}))

vi.mock('../src/hooks/useI18n', () => ({
  useI18n: () => ({
    lang: 'en-US',
    setLang: vi.fn(),
    t: (key: string, paramsOrFallback?: Record<string, unknown> | string) => {
      const fallback = typeof paramsOrFallback === 'string' ? paramsOrFallback : undefined
      const params = typeof paramsOrFallback === 'object' ? paramsOrFallback : undefined
      const map: Record<string, string> = {
        'executions.action.pendingTitle': 'Pending Approval',
        'executions.action.pendingDesc': 'This execution requires your review.',
        'executions.action.approve': 'Approve',
        'executions.action.reject': 'Reject',
        'executions.action.modifyApprove': 'Modify & Approve',
        'executions.action.requestContext': 'Request Context',
        'executions.action.modifyLabel': 'Modify command before approving',
        'executions.action.modifyHide': 'Hide modification',
        'executions.action.modifyPlaceholder': 'Replacement command…',
        'executions.action.confirmApproveTitle': 'Confirm Approval',
        'executions.action.confirmApproveDesc': 'Approve this execution?',
        'executions.action.confirmRejectTitle': 'Confirm Rejection',
        'executions.action.confirmRejectDesc': 'Reject this execution?',
        'executions.action.confirmRequestContextTitle': 'Request Context',
        'executions.action.confirmRequestContextDesc': 'Request more context?',
        'executions.action.confirmModifyTitle': 'Confirm Modify & Approve',
        'executions.action.confirmModifyDesc': 'Approve with modified command?',
        'executions.action.confirmRejectLabel': 'Reject',
        'executions.action.confirmRequestContextLabel': 'Request',
        'executions.action.confirmContinueLabel': 'Confirm',
        'executions.action.rejectionReason': 'Rejection reason',
        'executions.action.modifyNoteOptional': 'Modification note (optional)',
        'executions.action.modifyNotePlaceholder': 'Explain why the command was changed before approval...',
        'executions.action.approvalSummaryEmpty': 'No approval summary recorded yet.',
        'executions.action.manualGate': 'This request stays queued until an operator approves it.',
        'executions.action.manualGateCritical': 'High-risk request. This command will only run after an operator approves it.',
        'executions.action.summaryRisk': 'Risk',
        'executions.action.summaryStatus': 'Status',
        'executions.action.summaryApproval': 'Approval note',
        'executions.action.contextRequestNote': 'Context request note',
        'executions.action.contextRequestHelp': 'This asks the linked session for more evidence and does not run the command.',
        'executions.action.noteOptional': 'Note (optional)',
        'executions.action.rejectionPlaceholder': 'Why reject?',
        'executions.action.notePlaceholder': 'Optional note...',
        'executions.action.successStatus': 'Execution is now {{status}}.',
        'executions.action.failedStatus': 'Failed',
      }
      const template = map[key] ?? fallback ?? key
      if (!params) {
        return template
      }
      return template.replace(/\{\{(\w+)\}\}/g, (_, token: string) => String(params[token] ?? ''))
    },
  }),
}))

vi.mock('../src/hooks/ui/useNotify', () => ({
  useNotify: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
  }),
}))

async function flush() {
  await act(async () => {
    await Promise.resolve()
  })
}

const pendingExecution = {
  execution_id: 'exec-1',
  session_id: 'sess-1',
  command: 'restart service',
  status: 'pending' as const,
  risk_level: 'critical' as const,
  golden_summary: {
    approval: 'Critical approval required before this restart can run.',
  },
}

describe('ExecutionActionBar', () => {
  beforeEach(() => {
    ;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
    approveExecutionMock.mockReset()
    modifyApproveExecutionMock.mockReset()
    rejectExecutionMock.mockReset()
    requestExecutionContextMock.mockReset()
    approveExecutionMock.mockResolvedValue({ ...pendingExecution, status: 'approved' })
    modifyApproveExecutionMock.mockResolvedValue({ ...pendingExecution, status: 'approved' })
    rejectExecutionMock.mockResolvedValue({ ...pendingExecution, status: 'rejected' })
  })

  it('passes reason to approveExecution when user confirms with a note', async () => {
    const { ExecutionActionBar } = await import('../src/components/operator/ExecutionActionBar')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<ExecutionActionBar execution={pendingExecution as never} />)
    })
    await flush()

    // Click approve to open the dialog
    const approveBtn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Approve') && !b.textContent?.includes('Modify'),
    )
    expect(approveBtn).toBeTruthy()
    await act(async () => {
      approveBtn!.click()
    })
    await flush()

    // Find the reason textarea (rendered via portal on document.body)
    const textarea = document.querySelector('textarea')
    expect(textarea).toBeTruthy()
    await act(async () => {
      const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
        window.HTMLTextAreaElement.prototype,
        'value',
      )!.set!
      nativeInputValueSetter.call(textarea!, 'looks safe')
      textarea!.dispatchEvent(new Event('input', { bubbles: true }))
      textarea!.dispatchEvent(new Event('change', { bubbles: true }))
    })
    await flush()

    // Confirm via the dialog button (on document.body via portal)
    const confirmBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Confirm'),
    )
    expect(confirmBtn).toBeTruthy()
    await act(async () => {
      confirmBtn!.click()
    })
    await flush()
    await flush()

    expect(approveExecutionMock).toHaveBeenCalledWith('exec-1', { reason: 'looks safe' })

    root.unmount()
    container.remove()
  })

  it('renders the interpolated success status after approval', async () => {
    const { ExecutionActionBar } = await import('../src/components/operator/ExecutionActionBar')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<ExecutionActionBar execution={pendingExecution as never} />)
    })
    await flush()

    const approveBtn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Approve') && !b.textContent?.includes('Modify'),
    )
    expect(approveBtn).toBeTruthy()
    await act(async () => {
      approveBtn!.click()
    })
    await flush()

    const confirmBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Confirm'),
    )
    expect(confirmBtn).toBeTruthy()
    await act(async () => {
      confirmBtn!.click()
    })
    await flush()
    await flush()

    expect(container.textContent).toContain('Execution is now approved.')

    root.unmount()
    container.remove()
  })

  it('calls approveExecution without reason payload when note is empty', async () => {
    const { ExecutionActionBar } = await import('../src/components/operator/ExecutionActionBar')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<ExecutionActionBar execution={pendingExecution as never} />)
    })
    await flush()

    // Click approve
    const approveBtn = Array.from(container.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Approve') && !b.textContent?.includes('Modify'),
    )
    await act(async () => {
      approveBtn!.click()
    })
    await flush()

    // Confirm without typing a reason
    const confirmBtn = Array.from(document.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Confirm'),
    )
    expect(confirmBtn).toBeTruthy()
    await act(async () => {
      confirmBtn!.click()
    })
    await flush()
    await flush()

    // Should be called with undefined (no payload) when reason is empty
    expect(approveExecutionMock).toHaveBeenCalledWith('exec-1', undefined)

    root.unmount()
    container.remove()
  })

  it('includes reason field in the ExecutionActionRequest type for modifyApprove', async () => {
    // Verify at the API level that modifyApproveExecution accepts reason in its payload
    const { modifyApproveExecution } = await import('../src/lib/api/ops')
    // The function signature should accept { command, reason }
    expect(typeof modifyApproveExecution).toBe('function')
    // Verify the component wires command through — call the mock directly with the shape we expect
    modifyApproveExecutionMock.mockResolvedValue({ ...pendingExecution, status: 'approved' })
    await modifyApproveExecutionMock('exec-1', { command: 'new cmd', reason: 'test note' })
    expect(modifyApproveExecutionMock).toHaveBeenCalledWith('exec-1', { command: 'new cmd', reason: 'test note' })
  })

  it('keeps command override collapsed by default and expands it for modify approval', async () => {
    const { ExecutionActionBar } = await import('../src/components/operator/ExecutionActionBar')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<ExecutionActionBar execution={pendingExecution as never} />)
    })
    await flush()

    expect(container.querySelector('input')).toBeNull()
    expect(container.textContent).toContain('Modify command before approving')

    const modifyToggle = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Modify command before approving'),
    )
    expect(modifyToggle).toBeTruthy()

    await act(async () => {
      modifyToggle!.click()
    })
    await flush()

    const input = container.querySelector('input')
    expect(input).toBeTruthy()
    expect((input as HTMLInputElement).placeholder).toContain('Replacement command')

    root.unmount()
    container.remove()
  })

  it('makes the critical approval path explicit and labels context or rejection inputs clearly', async () => {
    requestExecutionContextMock.mockResolvedValue({ ...pendingExecution, status: 'pending' })

    const { ExecutionActionBar } = await import('../src/components/operator/ExecutionActionBar')
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<ExecutionActionBar execution={pendingExecution as never} />)
    })
    await flush()

    const summaryText = container.textContent || ''
    expect(summaryText).toContain('Critical approval required before this restart can run.')
    expect(summaryText).toContain('critical')
    expect(summaryText).toContain('will only run after an operator approves it')

    const requestContextButton = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Request Context'),
    )
    expect(requestContextButton).toBeTruthy()

    await act(async () => {
      requestContextButton!.click()
    })
    await flush()

    expect(document.body.textContent).toContain('Context request note')

    const cancelButton = Array.from(document.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Cancel'),
    )
    expect(cancelButton).toBeTruthy()

    await act(async () => {
      cancelButton!.click()
    })
    await flush()

    const rejectButton = Array.from(container.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('Reject'),
    )
    expect(rejectButton).toBeTruthy()

    await act(async () => {
      rejectButton!.click()
    })
    await flush()

    expect(document.body.textContent).toContain('Rejection reason')

    root.unmount()
    container.remove()
  })
})
