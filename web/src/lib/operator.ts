import type { DashboardAlertItem, ExecutionDetail, SessionDetail, ToolPlanStep } from '@/lib/api/types'
import i18n from '@/lib/i18n'

export type SessionEvidenceKind = 'metrics' | 'logs' | 'traces' | 'delivery' | 'ssh'

export interface SessionCurrentState {
  code: 'pendingEvidence' | 'pendingApproval' | 'executing' | 'observing' | 'resolved' | 'manualIntervention'
  tone: 'success' | 'warning' | 'danger' | 'info' | 'muted'
}

export function alertLabel(alert: Record<string, unknown>, key: string): string {
  const direct = alert[key]
  if (typeof direct === 'string' && direct.trim()) {
    return direct
  }
  const labels = alert.labels
  if (labels && typeof labels === 'object' && !Array.isArray(labels)) {
    const value = (labels as Record<string, unknown>)[key]
    if (typeof value === 'string' && value.trim()) {
      return value
    }
  }
  return ''
}

export function sessionService(session: SessionDetail): string {
  return alertLabel(session.alert, 'service') || 'platform'
}

export function sessionHost(session: SessionDetail): string {
  return alertLabel(session.alert, 'instance') || alertLabel(session.alert, 'host') || 'unassigned'
}

export function sessionSource(session: SessionDetail): string {
  return alertLabel(session.alert, 'source') || alertLabel(session.alert, 'generator') || alertLabel(session.alert, 'provider') || ''
}

export function sessionAlertName(session: SessionDetail): string {
  return alertLabel(session.alert, 'alertname') || 'Untitled alert'
}

export function sessionHeadline(session: SessionDetail): string {
  return session.golden_summary?.headline || session.golden_summary?.conclusion || session.diagnosis_summary || translate('sessions.fallback.headline', 'Diagnosis is still gathering evidence.')
}

export function sessionConclusion(session: SessionDetail): string {
  return session.golden_summary?.conclusion || translate('sessions.fallback.conclusion', 'No conclusion recorded yet.')
}

export function sessionNextAction(session: SessionDetail): string {
  return session.golden_summary?.next_action || translate('sessions.fallback.nextAction', 'Review evidence and decide whether to execute or request approval.')
}

export function sessionEvidence(session: SessionDetail): string[] {
  return session.golden_summary?.evidence || []
}

export function sessionRisk(session: SessionDetail): string {
  return session.golden_summary?.risk || highestRisk(session.executions.map((item) => item.risk_level)) || 'info'
}

export function humanizeLabel(value?: string, fallback = 'n/a'): string {
  if (!value) {
    return fallback
  }
  return value
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

export function sessionCurrentState(session: SessionDetail): SessionCurrentState {
  const executionPendingApproval = session.executions.some((item) => (item.status || '').toLowerCase() === 'pending')
  const executionApprovedManual = session.executions.some((item) => (item.status || '').toLowerCase() === 'approved')
  const executionRunning = session.executions.some((item) => (item.status || '').toLowerCase() === 'executing')
  const verificationStatus = (session.verification?.status || '').toLowerCase()
  const verificationSucceeded = ['completed', 'resolved', 'success'].includes(verificationStatus)

  if (session.status === 'failed' || verificationStatus === 'failed' || verificationStatus === 'error') {
    return { code: 'manualIntervention', tone: 'danger' }
  }
  if (session.status === 'resolved' || verificationSucceeded) {
    return { code: 'resolved', tone: 'success' }
  }
  if (session.status === 'verifying' || verificationStatus === 'pending' || verificationStatus === 'running') {
    return { code: 'observing', tone: 'info' }
  }
  if (session.status === 'executing' || executionRunning) {
    return { code: 'executing', tone: 'warning' }
  }
  if (session.status === 'pending_approval' || executionPendingApproval) {
    return { code: 'pendingApproval', tone: 'warning' }
  }
  if (executionApprovedManual) {
    return { code: 'manualIntervention', tone: 'warning' }
  }
  return { code: 'pendingEvidence', tone: 'info' }
}

export function sessionLastUpdatedAt(session: SessionDetail): string {
  const timestamps = [
    session.verification?.checked_at,
    ...(session.timeline || []).map((entry) => entry.created_at),
    ...session.executions.flatMap((execution) => [execution.completed_at, execution.approved_at, execution.created_at]),
  ]
    .filter((value): value is string => !!value)

  if (timestamps.length === 0) {
    return ''
  }

  return timestamps
    .slice()
    .sort((left, right) => toTimestamp(right) - toTimestamp(left))[0] || ''
}

export function sortSessionsForTriage(sessions: SessionDetail[], sortOrder = 'desc'): SessionDetail[] {
  const desc = sortOrder !== 'asc'
  return [...sessions].sort((left, right) => {
    const leftState = sessionCurrentState(left)
    const rightState = sessionCurrentState(right)
    const direction = desc ? 1 : -1
    const activeDelta = (isClosedQueueState(rightState.code) - isClosedQueueState(leftState.code)) * direction
    if (activeDelta !== 0) {
      return activeDelta
    }

    const riskDelta = (triageRiskWeight(sessionRisk(right)) - triageRiskWeight(sessionRisk(left))) * direction
    if (riskDelta !== 0) {
      return riskDelta
    }

    const stateDelta = (triageStateWeight(rightState.code) - triageStateWeight(leftState.code)) * direction
    if (stateDelta !== 0) {
      return stateDelta
    }

    const updatedDelta = (toTimestamp(sessionLastUpdatedAt(right)) - toTimestamp(sessionLastUpdatedAt(left))) * direction
    if (updatedDelta !== 0) {
      return updatedDelta
    }

    return sessionHeadline(left).localeCompare(sessionHeadline(right)) * direction
  })
}

export function sessionAttachmentCount(session: SessionDetail): number {
  return session.attachments?.length || 0
}

export function sessionEvidenceSummary(session: SessionDetail): Record<SessionEvidenceKind, string[]> {
  const summary: Record<SessionEvidenceKind, string[]> = {
    metrics: [],
    logs: [],
    traces: [],
    delivery: [],
    ssh: [],
  }
  const seen = new Set<string>()

  const addItem = (kind: SessionEvidenceKind, rawValue?: string) => {
    const value = normalizeEvidence(rawValue)
    if (!value || summary[kind].length >= 3) {
      return
    }
    const token = `${kind}:${value.toLowerCase()}`
    if (seen.has(token)) {
      return
    }
    seen.add(token)
    summary[kind].push(value)
  }

  session.golden_summary?.evidence?.forEach((entry) => {
    classifyEvidenceKinds(entry).forEach((kind) => addItem(kind, entry))
  })

  session.tool_plan?.forEach((step) => {
    collectToolPlanSummary(step).forEach((entry) => {
      const hintedKinds = toolPlanKinds(step)
      const kinds = hintedKinds.length ? hintedKinds : classifyEvidenceKinds(entry)
      kinds.forEach((kind) => addItem(kind, entry))
    })
  })

  return summary
}

export function executionHeadline(execution: ExecutionDetail): string {
  return execution.golden_summary?.headline || execution.golden_summary?.command_preview || execution.command || (execution.capability_id ? `${execution.connector_id || 'connector'}::${execution.capability_id}` : 'Execution request')
}

export function executionResult(execution: ExecutionDetail): string {
  return execution.golden_summary?.result || execution.golden_summary?.approval || 'Waiting for execution updates.'
}

export function executionNextAction(execution: ExecutionDetail): string {
  return execution.golden_summary?.next_action || 'Review output and follow the linked session if more context is needed.'
}

export function executionRisk(execution: ExecutionDetail): string {
  return execution.golden_summary?.risk || execution.risk_level || 'info'
}

export function shortID(value?: string): string {
  if (!value) {
    return 'n/a'
  }
  const token = value.split('-')[0]
  return token || value
}

export function formatDateTime(value?: string): string {
  if (!value) {
    return translate('common.na', 'n/a')
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString(activeLocale())
}

export function formatRelativeTime(value?: string): string {
  if (!value) {
    return translate('common.na', 'n/a')
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  const delta = Date.now() - date.getTime()
  const minutes = Math.floor(delta / 60000)
  if (minutes < 1) {
    return translate('common.relative.justNow', 'just now')
  }
  if (minutes < 60) {
    return translate('common.relative.minutesAgo', '{{count}}m ago', { count: minutes })
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return translate('common.relative.hoursAgo', '{{count}}h ago', { count: hours })
  }
  const days = Math.floor(hours / 24)
  return translate('common.relative.daysAgo', '{{count}}d ago', { count: days })
}

export function statusTone(status?: string): 'success' | 'warning' | 'danger' | 'info' | 'muted' {
  switch ((status || '').toLowerCase()) {
    case 'healthy':
    case 'success':
    case 'enabled':
    case 'active':
    case 'approved':
    case 'completed':
    case 'resolved':
    case 'set':
      return 'success'
    case 'pending_approval':
    case 'warning':
    case 'degraded':
    case 'timeout':
    case 'blocked':
      return 'warning'
    case 'critical':
    case 'failed':
    case 'rejected':
    case 'error':
    case 'missing':
    case 'disabled':
      return 'danger'
    case 'analyzing':
    case 'executing':
    case 'verifying':
    case 'open':
    case 'pending':
    default:
      return 'info'
  }
}

export function riskTone(risk?: string): 'success' | 'warning' | 'danger' | 'info' | 'muted' {
  switch ((risk || '').toLowerCase()) {
    case 'critical':
      return 'danger'
    case 'warning':
      return 'warning'
    case 'low':
    case 'safe':
      return 'success'
    default:
      return 'info'
  }
}

export function alertTone(alert?: DashboardAlertItem): 'warning' | 'danger' | 'info' {
  switch ((alert?.severity || '').toLowerCase()) {
    case 'critical':
      return 'danger'
    case 'warning':
      return 'warning'
    default:
      return 'info'
  }
}

function normalizeEvidence(value?: string): string {
  if (!value) {
    return ''
  }
  const cleaned = value
    .replace(/^(metrics?|logs?|traces?|observability|delivery|ssh)\s*:\s*/i, '')
    .replace(/\s+/g, ' ')
    .trim()
  if (!cleaned) {
    return ''
  }
  return cleaned.length > 180 ? `${cleaned.slice(0, 177).trimEnd()}...` : cleaned
}

function classifyEvidenceKinds(value?: string): SessionEvidenceKind[] {
  const text = normalizeEvidence(value).toLowerCase()
  const kinds: SessionEvidenceKind[] = []

  if (matchesAny(text, ['metric', 'latency', 'cpu', 'memory', 'utilization', 'saturation', 'qps', 'throughput', 'error rate', 'burn rate', 'p95', 'p99', 'dashboard', 'prometheus', 'victoria'])) {
    kinds.push('metrics')
  }
  if (matchesAny(text, ['log', 'exception', 'stack', 'timeout', 'context deadline', 'warn', 'victorialogs', 'stderr']) || hasStandaloneLogError(text)) {
    kinds.push('logs')
  }
  if (matchesAny(text, ['trace', 'span', 'jaeger', 'tempo', 'otel', 'distributed', 'end-to-end', 'observability', 'vmalert', 'alerting rule', 'alert rule', 'alertmanager', 'rule evaluation', 'firing'])) {
    kinds.push('traces')
  }
  if (matchesAny(text, ['deploy', 'release', 'rollback', 'commit', 'build', 'pipeline', 'canary', 'rollout', 'change window', 'change-window', 'change request', 'release train'])) {
    kinds.push('delivery')
  }
  if (matchesAny(text, ['ssh', 'bastion', 'shell', 'terminal', 'jumpserver', 'jump host', 'remote command', 'remote access'])) {
    kinds.push('ssh')
  }

  return Array.from(new Set(kinds))
}

function toolPlanKinds(step: ToolPlanStep): SessionEvidenceKind[] {
  const joined = [
    step.tool,
    step.connector_id,
    step.runtime?.runtime,
    step.runtime?.connector_id,
    step.reason,
  ]
    .filter(Boolean)
    .join(' ')

  return classifyEvidenceKinds(joined)
}

function collectToolPlanSummary(step: ToolPlanStep): string[] {
  const items = [step.reason, ...extractSummaryLines(step.output)]
    .map((entry) => normalizeEvidence(entry))
    .filter(Boolean)

  return Array.from(new Set(items))
}

function extractSummaryLines(value: unknown, depth = 0): string[] {
  if (depth > 2 || value == null) {
    return []
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return [String(value)]
  }
  if (Array.isArray(value)) {
    return value.flatMap((entry) => extractSummaryLines(entry, depth + 1)).slice(0, 3)
  }
  if (typeof value !== 'object') {
    return []
  }

  const record = value as Record<string, unknown>
  const preferredKeys = ['summary', 'headline', 'message', 'result', 'finding', 'reason', 'description', 'status', 'error']
  const preferred = preferredKeys.flatMap((key) => extractSummaryLines(record[key], depth + 1))
  if (preferred.length) {
    return preferred.slice(0, 3)
  }

  return Object.entries(record)
    .filter(([, entry]) => typeof entry === 'string' || typeof entry === 'number' || typeof entry === 'boolean')
    .slice(0, 3)
    .map(([key, entry]) => `${humanizeLabel(key)}: ${String(entry)}`)
}

function matchesAny(text: string, tokens: string[]): boolean {
  return tokens.some((token) => text.includes(token))
}

function hasStandaloneLogError(text: string): boolean {
  return /\berror(s)?\b/.test(text) && !/\berror rate\b/.test(text)
}

function highestRisk(risks: Array<string | undefined>): string {
  return risks
    .filter((value): value is string => !!value)
    .sort((left, right) => triageRiskWeight(right) - triageRiskWeight(left))[0] || ''
}

function isClosedQueueState(code: SessionCurrentState['code']): number {
  return code === 'resolved' ? 0 : 1
}

function triageStateWeight(code: SessionCurrentState['code']): number {
  switch (code) {
    case 'pendingApproval':
      return 6
    case 'executing':
      return 5
    case 'manualIntervention':
      return 4
    case 'pendingEvidence':
      return 3
    case 'observing':
      return 2
    case 'resolved':
      return 1
    default:
      return 0
  }
}

function triageRiskWeight(risk?: string): number {
  switch ((risk || '').toLowerCase()) {
    case 'critical':
      return 4
    case 'warning':
      return 3
    case 'info':
      return 2
    case 'low':
    case 'safe':
      return 1
    default:
      return 0
  }
}

function toTimestamp(value?: string): number {
  if (!value) {
    return 0
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 0 : date.getTime()
}

function activeLocale(): string {
  return i18n.resolvedLanguage || i18n.language || 'zh-CN'
}

function translate(key: string, fallback: string, options?: Record<string, unknown>): string {
  return i18n.t(key, { defaultValue: fallback, ...options })
}
