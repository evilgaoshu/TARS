import { afterEach, describe, expect, it, vi } from 'vitest'

import type { SessionDetail } from '../src/lib/api/types'
import i18n from '../src/lib/i18n'
import { formatRelativeTime, sessionConclusion, sessionCurrentState, sessionEvidenceSummary, sessionHeadline, sessionNextAction, sortSessionsForTriage } from '../src/lib/operator'

function makeSession(overrides: Partial<SessionDetail> = {}): SessionDetail {
  return {
    session_id: 'sess-1',
    status: 'analyzing',
    alert: {},
    executions: [],
    timeline: [],
    ...overrides,
  }
}

describe('session evidence summary', () => {
  it('keeps error-rate metrics out of the logs summary bucket', () => {
    const session = makeSession({
      golden_summary: {
        evidence: ['Metrics: checkout error rate rose from 1% to 8% in fifteen minutes.'],
      },
    })

    const summary = sessionEvidenceSummary(session)

    expect(summary.metrics).toEqual(['checkout error rate rose from 1% to 8% in fifteen minutes.'])
    expect(summary.logs).toEqual([])
  })

  it('pulls change and ssh evidence into separate quick-scan buckets and ignores notification receipts', () => {
    const session = makeSession({
      golden_summary: {
        evidence: [
          'Delivery: Deploy checkout-api build 418 rolled out to production canary at 09:12 UTC.',
          'Delivery: Telegram alert reached the primary on-call channel in 12 seconds.',
          'SSH: bastion ssh handshake to checkout-host-1 succeeded for manual fallback.',
        ],
      },
      tool_plan: [
        {
          tool: 'deploy.inspect',
          reason: 'Check production canary rollout and pipeline result',
          status: 'completed',
          output: { summary: 'Build 418 finished and canary advanced to 25% rollout.' },
        },
        {
          tool: 'notification.inspect',
          reason: 'Check telegram delivery receipts',
          status: 'completed',
          output: { summary: 'Slack fallback notification was delivered to the backup target.' },
        },
        {
          tool: 'ssh.check',
          reason: 'Validate bastion ssh access before manual intervention',
          status: 'completed',
          output: { summary: 'SSH command execution path is available through the production bastion.' },
        },
      ],
    })

    const summary = sessionEvidenceSummary(session)

    expect(summary.delivery).toEqual([
      'Deploy checkout-api build 418 rolled out to production canary at 09:12 UTC.',
      'Check production canary rollout and pipeline result',
      'Build 418 finished and canary advanced to 25% rollout.',
    ])
    expect(summary.delivery).not.toContain('Telegram alert reached the primary on-call channel in 12 seconds.')
    expect(summary.delivery).not.toContain('Check telegram delivery receipts')
    expect(summary.delivery).not.toContain('Slack fallback notification was delivered to the backup target.')
    expect(summary.ssh).toEqual([
      'bastion ssh handshake to checkout-host-1 succeeded for manual fallback.',
      'Validate bastion ssh access before manual intervention',
      'SSH command execution path is available through the production bastion.',
    ])
  })

  it('surfaces observability query results in the traces quick-scan bucket', () => {
    const session = makeSession({
      tool_plan: [
        {
          tool: 'observability.query',
          reason: 'Check firing rules from the observability backend',
          status: 'completed',
          runtime: {
            runtime: 'observability_http',
            connector_id: 'victoriametrics-main',
          },
          output: { summary: 'returned 2 alerting rules, 2 firing for checkout-api' },
        },
      ],
    })

    const summary = sessionEvidenceSummary(session)

    expect(summary.traces).toEqual([
      'Check firing rules from the observability backend',
      'returned 2 alerting rules, 2 firing for checkout-api',
    ])
  })
})

describe('session triage state and ordering', () => {
  it('maps operator-facing session states into the six queue buckets', () => {
    const pendingEvidence = makeSession({
      status: 'analyzing',
      golden_summary: {
        headline: 'Checkout latency is still rising',
        conclusion: 'Inventory CPU remains saturated.',
        risk: 'critical',
      },
    })

    const pendingApproval = makeSession({
      status: 'pending_approval',
      executions: [
        {
          execution_id: 'exec-pending',
          status: 'pending',
        },
      ],
    })

    const executing = makeSession({
      status: 'executing',
      executions: [
        {
          execution_id: 'exec-running',
          status: 'executing',
        },
      ],
    })

    const observing = makeSession({
      status: 'verifying',
      verification: {
        status: 'pending',
        summary: 'Watching canary error rate after the rollback.',
      },
    })

    const resolved = makeSession({
      status: 'resolved',
      verification: {
        status: 'success',
        summary: 'Latency and saturation have returned to baseline.',
      },
    })

    const manualIntervention = makeSession({
      status: 'failed',
    })

    expect(sessionCurrentState(pendingEvidence).code).toBe('pendingEvidence')
    expect(sessionCurrentState(pendingApproval).code).toBe('pendingApproval')
    expect(sessionCurrentState(executing).code).toBe('executing')
    expect(sessionCurrentState(observing).code).toBe('observing')
    expect(sessionCurrentState(resolved).code).toBe('resolved')
    expect(sessionCurrentState(manualIntervention).code).toBe('manualIntervention')
  })

  it('does not show accepted manual executions as still pending approval', () => {
    const acceptedManualExecution = makeSession({
      status: 'open',
      executions: [
        {
          execution_id: 'exec-approved',
          status: 'approved',
        },
      ],
    })

    expect(sessionCurrentState(acceptedManualExecution).code).toBe('manualIntervention')
  })

  it('sorts incident queue priority by active work first, then risk, queue state, and last update', () => {
    const sessions = [
      makeSession({
        session_id: 'sess-closed',
        status: 'resolved',
        golden_summary: {
          headline: 'Inventory saturation stabilized after mitigation',
          risk: 'critical',
        },
        timeline: [
          {
            event: 'resolved',
            message: 'Mitigation completed',
            created_at: '2026-04-18T09:09:00Z',
          },
        ],
      }),
      makeSession({
        session_id: 'sess-collecting',
        status: 'analyzing',
        golden_summary: {
          headline: 'Checkout latency still spiking',
          risk: 'critical',
        },
        timeline: [
          {
            event: 'analysis_started',
            message: 'Collecting more evidence',
            created_at: '2026-04-18T09:06:00Z',
          },
        ],
      }),
      makeSession({
        session_id: 'sess-manual',
        status: 'failed',
        golden_summary: {
          headline: 'Manual cache flush required',
          risk: 'warning',
        },
        timeline: [
          {
            event: 'failed',
            message: 'Automatic remediation failed',
            created_at: '2026-04-18T09:08:00Z',
          },
        ],
      }),
      makeSession({
        session_id: 'sess-pending',
        status: 'pending_approval',
        golden_summary: {
          headline: 'Checkout rollback awaiting approval',
          risk: 'critical',
        },
        timeline: [
          {
            event: 'approval_requested',
            message: 'Waiting for rollback approval',
            created_at: '2026-04-18T09:04:00Z',
          },
        ],
        executions: [
          {
            execution_id: 'exec-1',
            status: 'pending',
          },
        ],
      }),
    ]

    expect(sortSessionsForTriage(sessions).map((session) => session.session_id)).toEqual([
      'sess-pending',
      'sess-collecting',
      'sess-manual',
      'sess-closed',
    ])
  })
})

describe('operator localization fallbacks', () => {
  afterEach(async () => {
    vi.useRealTimers()
    await i18n.changeLanguage('zh-CN')
  })

  it('uses the active locale for diagnosis fallbacks and relative time', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-18T09:10:00Z'))
    await i18n.changeLanguage('zh-CN')

    const session = makeSession()

    expect(sessionHeadline(session)).toBe('诊断仍在补证据。')
    expect(sessionConclusion(session)).toBe('暂未记录结论。')
    expect(sessionNextAction(session)).toBe('先查看证据，再决定是否执行或请求审批。')
    expect(formatRelativeTime('2026-04-18T09:09:30Z')).toBe('刚刚')
    expect(formatRelativeTime('2026-04-18T08:10:00Z')).toBe('1小时前')
  })
})
