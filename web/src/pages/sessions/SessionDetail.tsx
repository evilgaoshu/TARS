import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import type { Components } from 'react-markdown'
import ReactMarkdown from 'react-markdown'
import { AlertCircle, ArrowLeft, Bell, BookOpenText, Clock3, ExternalLink, Gauge, History, Image as ImageIcon, ListChecks, ScrollText, Search, Send, TerminalSquare, Waypoints, Zap, Shield } from 'lucide-react'
import { fetchSession, fetchSessionTrace, getApiErrorMessage } from '../../lib/api/ops'
import type { AuditTraceEntry, MessageAttachment, NotificationDigest, SessionKnowledgeTrace, ToolPlanStep } from '../../lib/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state'
import { OperatorHero, OperatorSection, OperatorStack, OperatorKicker } from '@/components/operator/OperatorPage'
import { formatDateTime, formatRelativeTime, humanizeLabel, riskTone, sessionAttachmentCount, sessionConclusion, sessionCurrentState, sessionEvidence, sessionEvidenceSummary, sessionHeadline, sessionHost, sessionNextAction, sessionRisk, sessionService, shortID, statusTone } from '@/lib/operator'
import { cn } from '@/lib/utils'
import { useI18n } from '@/hooks/useI18n'

const markdownComponents: Components = {
  code(props) {
    const { className, children, ...rest } = props
    const match = /language-(\w+)/.exec(className || '')
    return match ? (
      <pre className="overflow-x-auto rounded-2xl border border-border bg-black/40 p-4 text-sm leading-6 text-slate-200"><code className={className} {...rest}>{String(children).replace(/\n$/, '')}</code></pre>
    ) : (
      <code className="rounded bg-white/10 px-1.5 py-0.5 font-mono text-[0.9em]" {...rest}>{children}</code>
    )
  },
}

export const SessionDetailView = () => {
  const { t } = useI18n()
  const { id = '' } = useParams<{ id: string }>()

  const { data: session, isLoading: sessionLoading, error: sessionError } = useQuery({
    queryKey: ['session', id],
    queryFn: () => fetchSession(id),
    enabled: !!id,
  })

  const { data: trace, error: traceQueryError } = useQuery({
    queryKey: ['session-trace', id],
    queryFn: () => fetchSessionTrace(id),
    enabled: !!id,
    retry: 1,
  })

  if (sessionLoading) {
    return <div className="p-12 text-sm text-muted-foreground">{t('sessions.loading')}</div>
  }

  if (!session) {
    const message = sessionError ? getApiErrorMessage(sessionError, t('sessions.error.load')) : t('sessions.notFound')
    return <EmptyState icon={AlertCircle} title={t('sessions.notFound')} description={message} action={<Button variant="outline" asChild><Link to="/sessions">{t('sessions.backToQueue')}</Link></Button>} />
  }

  const traceError = traceQueryError ? getApiErrorMessage(traceQueryError, t('sessions.empty.traceUnavailable')) : ''
  const currentState = sessionCurrentState(session)
  const risk = sessionRisk(session)
  const evidenceSummary = sessionEvidenceSummary(session)
  const diagnosisEvidence = sessionEvidence(session)
  const attachmentCount = sessionAttachmentCount(session)
  const lastUpdated = formatRelativeTime(session.timeline?.[session.timeline.length - 1]?.created_at)

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('nav.sessions')}
        title={sessionHeadline(session)}
        description={sessionConclusion(session)}
        chips={[
          { label: t(`sessions.states.${currentState.code}`), tone: currentState.tone },
          { label: humanizeLabel(risk, 'Info'), tone: riskTone(risk) },
          { label: shortID(session.session_id), tone: 'muted' },
          ...(session.is_smoke ? [{ label: t('sessions.smoke'), tone: 'warning' as const }] : []),
        ]}
        primaryAction={<Button variant="amber" asChild><Link to="/executions">{t('sessions.action.reviewApprovals')}</Link></Button>}
        secondaryAction={<Button variant="outline" asChild><Link to="/sessions"><ArrowLeft size={14} />{t('sessions.backToQueue')}</Link></Button>}
      />

      <OperatorSection title={t('sessions.sections.currentDiagnosis')} description={t('sessions.sections.currentDiagnosisDesc')} icon={ListChecks}>
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(320px,1fr)]">
          <Card className="overflow-hidden border-primary/20 bg-primary/5">
            <CardHeader className="pb-4">
              <CardTitle className="text-[0.72rem] font-black uppercase tracking-[0.18em] text-primary">{t('sessions.fields.currentConclusion')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="text-lg font-semibold leading-8 text-foreground sm:text-[1.35rem]">{sessionConclusion(session)}</div>
              <div className="flex flex-wrap gap-2">
                <OperatorKicker label={t('common.service')} value={sessionService(session)} tone="info" />
                <OperatorKicker label={t('common.host')} value={sessionHost(session)} tone="muted" />
                <OperatorKicker label={t('sessions.stats.updated')} value={lastUpdated} tone="muted" />
                <OperatorKicker label={t('sessions.stats.executions')} value={String(session.executions.length)} tone={session.executions.length > 0 ? 'warning' : 'muted'} />
              </div>
            </CardContent>
          </Card>

          <OperatorStack>
            <SignalCard
              label={t('sessions.fields.evidence')}
              value={diagnosisEvidence[0] || t('sessions.fields.evidenceEmpty')}
              description={diagnosisEvidence[1]}
              tone="info"
            />
            <SignalCard
              label={t('sessions.fields.currentState')}
              value={t(`sessions.states.${currentState.code}`)}
              description={session.verification?.summary || t(`sessions.states.${currentState.code}Desc`)}
              tone={currentState.tone}
            />
            <SignalCard
              label={t('sessions.fields.risk')}
              value={humanizeLabel(risk, 'Info')}
              description={t('sessions.fields.statusHint', { status: humanizeLabel(session.status) })}
              tone={riskTone(risk)}
            />
          </OperatorStack>
        </div>

        <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_repeat(2,minmax(0,1fr))]">
          <SignalCard
            label={t('sessions.fields.nextStep')}
            value={sessionNextAction(session)}
            description={session.executions.length > 0 ? t('sessions.fields.nextStepExecutionHint') : t('sessions.fields.nextStepEvidenceHint')}
            tone="warning"
          />
          <SignalCard
            label={t('sessions.stats.updated')}
            value={lastUpdated}
            description={t('sessions.stats.updatedDesc')}
            tone="muted"
          />
          <SignalCard
            label={t('sessions.stats.executions')}
            value={String(session.executions.length)}
            description={t('sessions.stats.executionsDesc')}
            tone={session.executions.length > 0 ? 'warning' : 'muted'}
          />
        </div>
      </OperatorSection>

      <OperatorSection title={t('sessions.sections.evidenceSummary')} description={t('sessions.sections.evidenceSummaryDesc')} icon={Search}>
        <div className="grid gap-4 xl:grid-cols-3">
          <EvidenceSummaryCard title={t('sessions.evidence.metrics')} icon={Gauge} items={evidenceSummary.metrics} />
          <EvidenceSummaryCard title={t('sessions.evidence.logs')} icon={ScrollText} items={evidenceSummary.logs} />
          <EvidenceSummaryCard title={t('sessions.evidence.traces')} icon={Waypoints} items={evidenceSummary.traces} />
        </div>
        <div className="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_280px]">
          <EvidenceSummaryCard
            title={t('sessions.evidence.delivery')}
            icon={Send}
            items={evidenceSummary.delivery}
            emptyTitle={t('sessions.empty.delivery')}
            emptyDescription={t('sessions.empty.deliveryDesc')}
          />
          <EvidenceSummaryCard
            title={t('sessions.evidence.ssh')}
            icon={Shield}
            items={evidenceSummary.ssh}
            emptyTitle={t('sessions.empty.ssh')}
            emptyDescription={t('sessions.empty.sshDesc')}
          />
          <Card className="overflow-hidden border-border bg-white/[0.03]">
            <CardHeader className="pb-3">
              <div className="flex items-center gap-3">
                <div className="flex size-10 items-center justify-center rounded-2xl border border-border bg-white/[0.04] text-primary">
                  <ImageIcon size={18} />
                </div>
                <CardTitle className="text-base font-semibold text-foreground">{t('sessions.sections.attachments')}</CardTitle>
              </div>
            </CardHeader>
            <CardContent>
              {attachmentCount > 0 ? (
                <div className="rounded-2xl border border-border bg-black/20 px-4 py-3 text-sm leading-6 text-foreground">{t('sessions.evidence.attachments', { count: attachmentCount })}</div>
              ) : (
                <EmptyState icon={ImageIcon} title={t('sessions.empty.attachments')} description={t('sessions.empty.attachmentsDesc')} />
              )}
            </CardContent>
          </Card>
        </div>
      </OperatorSection>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <OperatorStack>
          <OperatorSection title={t('sessions.sections.narrative')} description={t('sessions.sections.narrativeDesc')}>
            {session.diagnosis_summary ? (
              <div className="prose max-w-none prose-invert text-sm leading-7 text-foreground">
                <ReactMarkdown components={markdownComponents}>{session.diagnosis_summary}</ReactMarkdown>
              </div>
            ) : (
              <EmptyState icon={Search} title={t('sessions.empty.narrativeUnavailable')} description={t('sessions.empty.narrativeUnavailableDesc')} />
            )}
          </OperatorSection>

          <OperatorSection title={t('sessions.sections.knowledge')} description={t('sessions.sections.knowledgeDesc')}>
            <KnowledgeTraceCard knowledge={trace?.knowledge} traceError={traceError} />
          </OperatorSection>

          <OperatorSection title={t('sessions.sections.linkedExecutions')} description={t('sessions.sections.linkedExecutionsDesc')}>
            <ExecutionListCard count={session.executions.length} items={session.executions} />
          </OperatorSection>

          <OperatorSection title={t('sessions.sections.toolPlan')} description={t('sessions.sections.toolPlanDesc')}>
            <ToolPlanList items={session.tool_plan || []} />
          </OperatorSection>

          <OperatorSection title={t('sessions.sections.attachments')} description={t('sessions.sections.attachmentsDesc')}>
            <AttachmentsCard items={session.attachments || []} />
          </OperatorSection>

          <OperatorSection title={t('sessions.sections.rawContext')} description={t('sessions.sections.rawContextDesc')}>
            <pre className="overflow-x-auto rounded-2xl border border-border bg-black/40 p-4 text-xs leading-6 text-slate-300">{JSON.stringify(session.alert, null, 2)}</pre>
          </OperatorSection>
        </OperatorStack>

        <OperatorStack>
          {session.verification ? (
            <OperatorSection title={t('sessions.sections.verification')} description={t('sessions.sections.verificationDesc')}>
              <div className="space-y-3">
                <Badge variant={statusTone(session.verification.status)}>{humanizeLabel(session.verification.status)}</Badge>
                <div className="text-sm leading-6 text-foreground">{session.verification.summary}</div>
                {session.verification.checked_at ? <div className="text-sm text-muted-foreground">{t('sessions.fields.checkedAt', { time: formatDateTime(session.verification.checked_at) })}</div> : null}
              </div>
            </OperatorSection>
          ) : null}

          <OperatorSection title={t('sessions.sections.notifications')} description={t('sessions.sections.notificationsDesc')}>
            <NotificationDigestCard items={session.notifications || []} />
          </OperatorSection>

          <OperatorSection title={t('sessions.sections.timeline')} description={t('sessions.sections.timelineDesc')}>
            <TimelineCard items={session.timeline} />
          </OperatorSection>

          <OperatorSection title={t('sessions.sections.audit')} description={t('sessions.sections.auditDesc')}>
            <AuditTrailCard items={trace?.audit_entries || []} traceError={traceError} />
          </OperatorSection>
        </OperatorStack>
      </div>
    </div>
  )
}

function SignalCard({
  label,
  value,
  description,
  tone = 'info',
}: {
  label: string
  value: string
  description?: string
  tone?: 'success' | 'warning' | 'danger' | 'info' | 'muted'
}) {
  return (
    <Card className={cn('overflow-hidden border-border bg-white/[0.03]', signalToneClass(tone))}>
      <CardHeader className="pb-3">
        <CardTitle className="text-[0.72rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{label}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="text-base font-semibold leading-7 text-foreground">{value}</div>
        {description ? <div className="text-sm leading-6 text-muted-foreground">{description}</div> : null}
      </CardContent>
    </Card>
  )
}

function EvidenceSummaryCard({
  title,
  icon: Icon,
  items,
  emptyTitle,
  emptyDescription,
}: {
  title: string
  icon: typeof Gauge
  items: string[]
  emptyTitle?: string
  emptyDescription?: string
}) {
  const { t } = useI18n()

  return (
    <Card className="overflow-hidden border-border bg-white/[0.03]">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-2xl border border-border bg-white/[0.04] text-primary">
            <Icon size={18} />
          </div>
          <CardTitle className="text-base font-semibold text-foreground">{title}</CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        {items.length > 0 ? (
          <ul className="space-y-3 text-sm leading-6 text-foreground">
            {items.map((item) => (
              <li key={item} className="rounded-2xl border border-border bg-black/20 px-4 py-3">{item}</li>
            ))}
          </ul>
        ) : (
          <div className="rounded-2xl border border-dashed border-border bg-black/10 px-4 py-6 text-sm text-muted-foreground">
            <div className="font-semibold text-foreground">{emptyTitle || t('sessions.evidence.none')}</div>
            {emptyDescription ? <div className="mt-2 leading-6">{emptyDescription}</div> : null}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function NotificationDigestCard({ items }: { items: NotificationDigest[] }) {
  const { t } = useI18n()

  if (items.length === 0) {
    return <EmptyState icon={Bell} title={t('sessions.empty.notifications')} description={t('sessions.empty.notificationsDesc')} />
  }
  return (
    <OperatorStack>
      {items.map((item, index) => (
        <div key={`${item.stage || 'notification'}-${index}`} className="rounded-2xl border border-border bg-white/[0.03] p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-foreground">{item.reason || item.stage || t('sessions.fields.notificationFallback')}</div>
              <div className="mt-1 text-sm text-muted-foreground">
                {`${t('sessions.fields.target')}: ${item.target || t('sessions.fields.noTarget')}`}
                {item.preview ? ` · ${item.preview}` : ''}
              </div>
            </div>
            <div className="text-xs text-muted-foreground">{item.created_at ? formatRelativeTime(item.created_at) : '-'}</div>
          </div>
        </div>
      ))}
    </OperatorStack>
  )
}

function KnowledgeTraceCard({ knowledge, traceError }: { knowledge?: SessionKnowledgeTrace; traceError: string }) {
  const { t } = useI18n()

  if (traceError) {
    return <EmptyState icon={AlertCircle} title={t('sessions.empty.knowledgeTrace')} description={traceError} />
  }
  if (!knowledge) {
    return <EmptyState icon={BookOpenText} title={t('sessions.empty.knowledgeMaterialized')} description={t('sessions.empty.knowledgeMaterializedDesc')} />
  }
  return (
    <OperatorStack>
      <div className="rounded-2xl border border-border bg-white/[0.03] p-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-base font-semibold text-foreground">{knowledge.title}</div>
            <div className="mt-1 text-sm text-muted-foreground">{knowledge.document_id} · {knowledge.updated_at ? formatDateTime(knowledge.updated_at) : t('common.na')}</div>
          </div>
          <Button variant="ghost" size="sm"><ExternalLink size={14} />{t('sessions.action.viewDocument')}</Button>
        </div>
      </div>
      {knowledge.summary ? <div className="rounded-2xl border border-border bg-primary/5 p-4 text-sm leading-6 text-foreground">{knowledge.summary}</div> : null}
      {knowledge.conversation?.length ? (
        <div className="flex flex-wrap gap-2">
          {knowledge.conversation.map((line, index) => <Badge key={`${line}-${index}`} variant="outline">{line}</Badge>)}
        </div>
      ) : null}
      {knowledge.content_preview ? <pre className="overflow-x-auto rounded-2xl border border-border bg-black/40 p-4 text-xs leading-6 text-slate-300">{knowledge.content_preview}</pre> : null}
    </OperatorStack>
  )
}

function ToolPlanList({ items }: { items: ToolPlanStep[] }) {
  const { t } = useI18n()

  if (items.length === 0) {
    return <EmptyState icon={TerminalSquare} title={t('sessions.empty.toolPlan')} description={t('sessions.empty.toolPlanDesc')} />
  }
  return (
    <OperatorStack>
      {items.map((item, index) => (
        <div key={`${item.tool}-${index}`} className="rounded-2xl border border-border bg-white/[0.03] p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-foreground">{humanizeLabel(item.tool, item.tool)}</div>
              <div className="mt-1 text-sm text-muted-foreground">{item.reason || t('sessions.fields.noReason')}</div>
            </div>
            <Badge variant={statusTone(item.status)}>{humanizeLabel(item.status, 'Planned')}</Badge>
          </div>
          {item.runtime ? <div className="mt-3 text-xs text-muted-foreground">{item.runtime.connector_id || item.runtime.runtime || 'runtime'} · {item.runtime.protocol || t('common.na')}</div> : null}
          {item.input ? <pre className="mt-3 overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-slate-300">{JSON.stringify(item.resolved_input || item.input, null, 2)}</pre> : null}
          {item.output ? <pre className="mt-3 overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-emerald-300">{JSON.stringify(item.output, null, 2)}</pre> : null}
        </div>
      ))}
    </OperatorStack>
  )
}

function AttachmentsCard({ items }: { items: MessageAttachment[] }) {
  const { t } = useI18n()

  if (items.length === 0) {
    return <EmptyState icon={ImageIcon} title={t('sessions.empty.attachments')} description={t('sessions.empty.attachmentsDesc')} />
  }
  return (
    <div className="grid gap-4 md:grid-cols-2">
      {items.map((item, index) => {
        const href = buildAttachmentHref(item)
        const isImage = item.type === 'image' || item.mime_type?.startsWith('image/')
        return (
          <div key={`${item.type}-${item.name || index}`} className="rounded-2xl border border-border bg-white/[0.03] p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-foreground">{item.name || `${item.type} ${t('sessions.fields.attachmentFallback')}`}</div>
                <div className="mt-1 text-xs text-muted-foreground">{item.mime_type || 'binary/stream'}</div>
              </div>
              {href ? <a href={href} download={item.name} className="text-primary hover:underline">{t('sessions.action.download')}</a> : null}
            </div>
            <div className="mt-3">
              {isImage && href ? <img src={href} alt={item.name || t('sessions.fields.attachmentFallback')} className="max-h-[280px] rounded-xl border border-border object-contain" /> : item.content ? <pre className="overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-slate-300">{item.content}</pre> : <div className="text-sm text-muted-foreground">{item.preview_text || t('sessions.empty.previewUnavailable')}</div>}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function ExecutionListCard({ count, items }: { count: number; items: Array<{ execution_id: string; status: string; risk_level?: string; golden_summary?: { headline?: string; next_action?: string } }> }) {
  const { t } = useI18n()

  if (count === 0) {
    return <EmptyState icon={Zap} title={t('sessions.empty.linkedExecutions')} description={t('sessions.empty.linkedExecutionsDesc')} />
  }
  return (
    <OperatorStack>
      {items.map((item) => (
        <Link key={item.execution_id} to={`/executions/${item.execution_id}`} className="rounded-2xl border border-border bg-white/[0.03] p-4 transition-colors hover:bg-white/[0.05]">
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-foreground">{item.golden_summary?.headline || item.execution_id}</div>
              <div className="mt-1 text-sm text-muted-foreground">{item.golden_summary?.next_action || t('sessions.fields.executionFallback')}</div>
            </div>
            <div className="flex flex-col items-end gap-2">
              <Badge variant={statusTone(item.status)}>{humanizeLabel(item.status)}</Badge>
              <OperatorKicker label={t('sessions.fields.risk')} value={humanizeLabel(item.risk_level, 'Info')} tone={riskTone(item.risk_level)} />
            </div>
          </div>
        </Link>
      ))}
    </OperatorStack>
  )
}

function TimelineCard({ items }: { items: Array<{ event: string; message: string; created_at: string }> }) {
  const { t } = useI18n()

  if (items.length === 0) {
    return <EmptyState icon={Clock3} title={t('sessions.empty.timeline')} description={t('sessions.empty.timelineDesc')} />
  }

  return (
    <OperatorStack>
      {items.map((item, index) => (
        <div key={`${item.event}-${index}`} className="rounded-2xl border border-border bg-white/[0.03] p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-foreground">{humanizeLabel(item.event, item.event)}</div>
              <div className="mt-1 text-sm text-muted-foreground">{item.message || t('sessions.fields.noMessage')}</div>
            </div>
            <div className="text-xs text-muted-foreground">{formatRelativeTime(item.created_at)}</div>
          </div>
        </div>
      ))}
    </OperatorStack>
  )
}

function AuditTrailCard({ items, traceError }: { items: AuditTraceEntry[]; traceError: string }) {
  const { t } = useI18n()

  if (traceError) {
    return <EmptyState icon={AlertCircle} title={t('sessions.empty.auditTrace')} description={traceError} />
  }
  if (items.length === 0) {
    return <EmptyState icon={History} title={t('sessions.empty.audit')} description={t('sessions.empty.auditDesc')} />
  }
  return (
    <OperatorStack>
      {items.map((item, index) => (
        <div key={`${item.resource_id}-${item.action}-${index}`} className="rounded-2xl border border-border bg-white/[0.03] p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold text-foreground">{item.resource_type}::{item.action}</div>
              <div className="mt-1 text-sm text-muted-foreground">{t('sessions.fields.actor')}: {item.actor || t('sessions.fields.actorSystem')}</div>
            </div>
            <div className="text-xs text-muted-foreground">{formatRelativeTime(item.created_at)}</div>
          </div>
          {item.metadata ? <pre className="mt-3 overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-slate-300">{JSON.stringify(item.metadata, null, 2)}</pre> : null}
        </div>
      ))}
    </OperatorStack>
  )
}

function buildAttachmentHref(item: MessageAttachment) {
  if (item.url) return item.url
  if (!item.content) return ''
  const encoding = String(item.metadata?.encoding || '').toLowerCase()
  const mimeType = item.mime_type || (item.type === 'image' ? 'image/png' : 'text/plain;charset=utf-8')
  if (encoding === 'base64') return `data:${mimeType};base64,${item.content}`
  return `data:${mimeType},${encodeURIComponent(item.content)}`
}

function signalToneClass(tone: 'success' | 'warning' | 'danger' | 'info' | 'muted') {
  switch (tone) {
    case 'success':
      return 'border-success/25 bg-success/10'
    case 'warning':
      return 'border-warning/25 bg-warning/10'
    case 'danger':
      return 'border-danger/25 bg-danger/10'
    case 'muted':
      return 'border-border bg-white/[0.03]'
    case 'info':
    default:
      return 'border-info/25 bg-info/10'
  }
}
