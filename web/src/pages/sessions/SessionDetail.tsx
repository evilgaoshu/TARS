import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import type { Components } from 'react-markdown'
import ReactMarkdown from 'react-markdown'
import { AlertCircle, ArrowLeft, ChevronDown, Clock3, ExternalLink, History, ListChecks, Search } from 'lucide-react'
import { fetchSession, fetchSessionTrace, getApiErrorMessage } from '../../lib/api/ops'
import type { AuditTraceEntry, MessageAttachment, NotificationDigest, SessionKnowledgeTrace, TimelineEvent, ToolPlanStep } from '../../lib/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state'
import { OperatorHero, OperatorKicker, OperatorSection, OperatorStack } from '@/components/operator/OperatorPage'
import { formatDateTime, formatRelativeTime, humanizeLabel, riskTone, sessionAttachmentCount, sessionConclusion, sessionCurrentState, sessionHeadline, sessionHost, sessionNextAction, sessionRisk, sessionService, shortID, statusTone } from '@/lib/operator'
import { cn } from '@/lib/utils'
import { useI18n } from '@/hooks/useI18n'

const KEY_TIMELINE_ITEMS = 4

const markdownComponents: Components = {
  code(props) {
    const { className, children, ...rest } = props
    const match = /language-(\w+)/.exec(className || '')
    return match ? (
      <pre className="max-w-full overflow-x-auto rounded-2xl border border-border bg-black/40 p-4 text-sm leading-6 text-slate-200"><code className={className} {...rest}>{String(children).replace(/\n$/, '')}</code></pre>
    ) : (
      <code className="break-words rounded bg-white/10 px-1.5 py-0.5 font-mono text-[0.9em]" {...rest}>{children}</code>
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
  const attachmentCount = sessionAttachmentCount(session)
  const timelineItems = session.timeline || []
  const keyTimelineItems = timelineItems.slice(0, KEY_TIMELINE_ITEMS)
  const lastUpdated = formatRelativeTime(timelineItems[timelineItems.length - 1]?.created_at)
  const evidenceRows = buildEvidenceRows(session.tool_plan || [])
  const hasNarrative = !!session.diagnosis_summary?.trim()
  const hasKnowledge = !!trace?.knowledge || !!traceError
  const hasExecutions = session.executions.length > 0
  const hasAttachments = (session.attachments || []).length > 0
  const hasNotifications = (session.notifications || []).length > 0
  const hasVerification = !!session.verification
  const hasAudit = !!traceError || (trace?.audit_entries?.length || 0) > 0

  return (
    <div className="flex min-w-0 flex-col gap-6 pb-10">
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
        <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
          <Card className="min-w-0 overflow-hidden border-primary/20 bg-primary/5">
            <CardHeader className="pb-4">
              <CardTitle className="text-[0.72rem] font-black uppercase tracking-[0.18em] text-primary">{t('sessions.fields.currentConclusion')}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="break-words text-lg font-semibold leading-8 text-foreground sm:text-[1.35rem]">{sessionConclusion(session)}</div>
              <div className="flex flex-wrap gap-2">
                <OperatorKicker label={t('common.service')} value={sessionService(session)} tone="info" />
                <OperatorKicker label={t('common.host')} value={sessionHost(session)} tone="muted" />
                <OperatorKicker label={t('sessions.stats.updated')} value={lastUpdated} tone="muted" />
                <OperatorKicker label={t('sessions.stats.executions')} value={String(session.executions.length)} tone={session.executions.length > 0 ? 'warning' : 'muted'} />
              </div>
            </CardContent>
          </Card>

          <SignalCard
            label={t('sessions.fields.recommendedNextStep')}
            value={sessionNextAction(session)}
            description={session.executions.length > 0 ? t('sessions.fields.nextStepExecutionHint') : t('sessions.fields.nextStepEvidenceHint')}
            tone="warning"
          />
        </div>
      </OperatorSection>

      <div className="grid min-w-0 gap-6 xl:grid-cols-[minmax(280px,360px)_minmax(0,1fr)]">
        <OperatorSection title={t('sessions.sections.timeline')} description={t('sessions.sections.timelineDesc')} icon={Clock3} className="min-w-0 xl:sticky xl:top-4 xl:self-start">
          <TimelinePanel keyItems={keyTimelineItems} allItems={timelineItems} />
        </OperatorSection>

        <OperatorStack className="min-w-0">
          <OperatorSection title={t('sessions.sections.evidenceSummary')} description={t('sessions.sections.evidenceSummaryDesc')} icon={Search}>
            <EvidenceSummaryTable rows={evidenceRows} attachmentCount={attachmentCount} />
          </OperatorSection>

          {hasNarrative ? (
            <OperatorSection title={t('sessions.sections.narrative')} description={t('sessions.sections.narrativeDesc')}>
              <CollapsibleBlock title={t('sessions.sections.narrative')} summary={sessionConclusion(session)} defaultOpen={false}>
                <div className="prose max-w-none break-words prose-invert text-sm leading-7 text-foreground">
                  <ReactMarkdown components={markdownComponents}>{session.diagnosis_summary || ''}</ReactMarkdown>
                </div>
              </CollapsibleBlock>
            </OperatorSection>
          ) : null}

          {hasKnowledge ? (
            <OperatorSection title={t('sessions.sections.knowledge')} description={t('sessions.sections.knowledgeDesc')}>
              <KnowledgeTraceCard knowledge={trace?.knowledge} traceError={traceError} />
            </OperatorSection>
          ) : null}

          {hasExecutions ? (
            <OperatorSection title={t('sessions.sections.linkedExecutions')} description={t('sessions.sections.linkedExecutionsDesc')}>
              <ExecutionListCard items={session.executions} />
            </OperatorSection>
          ) : null}

          {hasAttachments ? (
            <OperatorSection title={t('sessions.sections.attachments')} description={t('sessions.sections.attachmentsDesc')}>
              <CollapsibleBlock title={t('sessions.fields.evidenceAttachments')} summary={t('sessions.fields.attachmentsCount', { count: attachmentCount })} defaultOpen={false}>
                <AttachmentsCard items={session.attachments || []} />
              </CollapsibleBlock>
            </OperatorSection>
          ) : null}

          {hasVerification ? (
            <OperatorSection title={t('sessions.sections.verification')} description={t('sessions.sections.verificationDesc')}>
              <div className="space-y-3">
                <Badge variant={statusTone(session.verification?.status)}>{humanizeLabel(session.verification?.status)}</Badge>
                <div className="break-words text-sm leading-6 text-foreground">{session.verification?.summary}</div>
                {session.verification?.checked_at ? <div className="text-sm text-muted-foreground">{t('sessions.fields.checkedAt', { time: formatDateTime(session.verification.checked_at) })}</div> : null}
              </div>
            </OperatorSection>
          ) : null}

          {hasNotifications ? (
            <OperatorSection title={t('sessions.sections.notifications')} description={t('sessions.sections.notificationsDesc')}>
              <NotificationDigestCard items={session.notifications || []} />
            </OperatorSection>
          ) : null}

          <OperatorSection title={t('sessions.sections.rawContext')} description={t('sessions.sections.rawContextDesc')}>
            <CollapsibleBlock title={t('sessions.sections.rawContext')} summary={sessionService(session)} defaultOpen={false}>
              <pre className="max-w-full overflow-x-auto rounded-2xl border border-border bg-black/40 p-4 text-xs leading-6 text-slate-300">{JSON.stringify(session.alert, null, 2)}</pre>
            </CollapsibleBlock>
          </OperatorSection>

          {hasAudit ? (
            <OperatorSection title={t('sessions.fields.auditCount', { count: trace?.audit_entries?.length || 0 })} description={t('sessions.sections.auditDesc')} icon={History}>
              <CollapsibleBlock title={t('sessions.fields.auditCount', { count: trace?.audit_entries?.length || 0 })} summary={traceError || session.session_id} defaultOpen={false}>
                <AuditTrailCard items={trace?.audit_entries || []} traceError={traceError} />
              </CollapsibleBlock>
            </OperatorSection>
          ) : null}
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
    <Card className={cn('min-w-0 overflow-hidden border-border bg-white/[0.03]', signalToneClass(tone))}>
      <CardHeader className="pb-3">
        <CardTitle className="text-[0.72rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{label}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="break-words text-base font-semibold leading-7 text-foreground">{value}</div>
        {description ? <div className="break-words text-sm leading-6 text-muted-foreground">{description}</div> : null}
      </CardContent>
    </Card>
  )
}

function TimelinePanel({ keyItems, allItems }: { keyItems: TimelineEvent[]; allItems: TimelineEvent[] }) {
  const { t } = useI18n()

  if (allItems.length === 0) {
    return <EmptyState icon={Clock3} title={t('sessions.empty.timeline')} description={t('sessions.empty.timelineDesc')} />
  }

  return (
    <div className="min-w-0 space-y-4">
      <div className="space-y-3">
        <div className="text-[0.72rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('sessions.fields.timelineKey')}</div>
        <OperatorStack>
          {keyItems.map((item, index) => (
            <div key={`${item.event}-${index}`} className="min-w-0 rounded-2xl border border-border bg-white/[0.03] p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="break-words text-sm font-semibold text-foreground">{humanizeLabel(item.event, item.event)}</div>
                  <div className="mt-1 break-words text-sm text-muted-foreground">{item.message || t('sessions.fields.noMessage')}</div>
                </div>
                <div className="shrink-0 text-xs text-muted-foreground">{formatRelativeTime(item.created_at)}</div>
              </div>
            </div>
          ))}
        </OperatorStack>
      </div>

      <CollapsibleBlock title={t('sessions.fields.timelineAll')} summary={`${allItems.length} events`} defaultOpen={false}>
          <OperatorStack>
            {allItems.map((item, index) => (
              <div key={`${item.event}-all-${index}`} className="min-w-0 rounded-2xl border border-border bg-black/20 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="break-words text-sm font-semibold text-foreground">{humanizeLabel(item.event, item.event)}</div>
                    <div className="mt-1 break-words text-sm text-muted-foreground">{item.message || t('sessions.fields.noMessage')}</div>
                  </div>
                  <div className="shrink-0 text-xs text-muted-foreground">{formatDateTime(item.created_at)}</div>
                </div>
              </div>
            ))}
          </OperatorStack>
        </CollapsibleBlock>
    </div>
  )
}

type EvidenceRow = {
  tool: string
  connector: string
  status: string
  result: string
  details: string
  input?: Record<string, unknown>
  output?: Record<string, unknown>
}

function EvidenceSummaryTable({ rows, attachmentCount }: { rows: EvidenceRow[]; attachmentCount: number }) {
  const { t } = useI18n()

  return (
    <div className="min-w-0 space-y-4">
      <div className="hidden grid-cols-[minmax(0,1fr)_minmax(120px,0.8fr)_110px_minmax(0,1fr)_minmax(0,1.2fr)] gap-3 rounded-2xl border border-border bg-white/[0.03] px-4 py-3 text-[0.72rem] font-black uppercase tracking-[0.18em] text-muted-foreground lg:grid">
        <div>{t('sessions.fields.tool')}</div>
        <div>{t('sessions.fields.connector')}</div>
        <div>{t('sessions.fields.status')}</div>
        <div>{t('sessions.fields.result')}</div>
        <div>{t('sessions.fields.details')}</div>
      </div>

      {rows.length > 0 ? (
        <OperatorStack>
          {rows.map((row, index) => (
            <CollapsibleBlock
              key={`${row.tool}-${index}`}
              title={row.tool}
              summary={`${row.status} · ${row.result}`}
              defaultOpen={index === 0}
              className="min-w-0 rounded-2xl border border-border bg-white/[0.03]"
              summaryClassName="px-4 py-4"
            >
              <div className="min-w-0 space-y-4 border-t border-border/60 px-4 py-4">
                <div className="grid min-w-0 gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(120px,0.8fr)_110px_minmax(0,1fr)_minmax(0,1.2fr)]">
                  <EvidenceCell label={t('sessions.fields.tool')} value={row.tool} />
                  <EvidenceCell label={t('sessions.fields.connector')} value={row.connector} />
                  <EvidenceCell label={t('sessions.fields.status')} value={row.status} badgeTone={statusTone(row.status)} />
                  <EvidenceCell label={t('sessions.fields.result')} value={row.result} />
                  <EvidenceCell label={t('sessions.fields.details')} value={row.details} />
                </div>

                {row.input ? (
                  <CollapsibleBlock title={t('sessions.fields.rawInput')} summary={row.tool} defaultOpen={false}>
                    <pre className="max-w-full overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-slate-300">{JSON.stringify(row.input, null, 2)}</pre>
                  </CollapsibleBlock>
                ) : null}

                {row.output ? (
                  <CollapsibleBlock title={t('sessions.fields.rawOutput')} summary={row.tool} defaultOpen={false}>
                    <pre className="max-w-full overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-emerald-300">{JSON.stringify(row.output, null, 2)}</pre>
                  </CollapsibleBlock>
                ) : null}
              </div>
            </CollapsibleBlock>
          ))}
        </OperatorStack>
      ) : (
        <div className="rounded-2xl border border-dashed border-border bg-black/10 px-4 py-6 text-sm text-muted-foreground">
          <div className="font-semibold text-foreground">{t('sessions.evidence.none')}</div>
        </div>
      )}

      {attachmentCount > 0 ? (
        <CollapsibleBlock title={t('sessions.fields.evidenceAttachments')} summary={t('sessions.fields.attachmentsCount', { count: attachmentCount })} defaultOpen={false}>
          <div className="text-sm text-muted-foreground">{t('sessions.evidence.attachments', { count: attachmentCount })}</div>
        </CollapsibleBlock>
      ) : null}
    </div>
  )
}

function EvidenceCell({ label, value, badgeTone }: { label: string; value: string; badgeTone?: 'success' | 'warning' | 'danger' | 'info' | 'muted' }) {
  return (
    <div className="min-w-0 rounded-2xl border border-border bg-black/20 p-3">
      <div className="text-[0.72rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 min-w-0 break-words text-sm leading-6 text-foreground">
        {badgeTone ? <Badge variant={badgeTone}>{value}</Badge> : value}
      </div>
    </div>
  )
}

function NotificationDigestCard({ items }: { items: NotificationDigest[] }) {
  const { t } = useI18n()
  return (
    <OperatorStack>
      {items.map((item, index) => (
        <div key={`${item.stage || 'notification'}-${index}`} className="min-w-0 rounded-2xl border border-border bg-white/[0.03] p-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="break-words text-sm font-semibold text-foreground">{item.reason || item.stage || t('sessions.fields.notificationFallback')}</div>
              <div className="mt-1 break-words text-sm text-muted-foreground">
                {`${t('sessions.fields.target')}: ${item.target || t('sessions.fields.noTarget')}`}
                {item.preview ? ` · ${item.preview}` : ''}
              </div>
            </div>
            <div className="shrink-0 text-xs text-muted-foreground">{item.created_at ? formatRelativeTime(item.created_at) : '-'}</div>
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
    return null
  }

  return (
    <OperatorStack>
      <div className="min-w-0 rounded-2xl border border-border bg-white/[0.03] p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="break-words text-base font-semibold text-foreground">{knowledge.title}</div>
            <div className="mt-1 break-all text-sm text-muted-foreground">{knowledge.document_id} · {knowledge.updated_at ? formatDateTime(knowledge.updated_at) : t('common.na')}</div>
          </div>
          <Button variant="ghost" size="sm"><ExternalLink size={14} />{t('sessions.action.viewDocument')}</Button>
        </div>
      </div>
      {knowledge.summary ? <div className="break-words rounded-2xl border border-border bg-primary/5 p-4 text-sm leading-6 text-foreground">{knowledge.summary}</div> : null}
      {knowledge.conversation?.length ? (
        <div className="flex flex-wrap gap-2">
          {knowledge.conversation.map((line, index) => <Badge key={`${line}-${index}`} variant="outline">{line}</Badge>)}
        </div>
      ) : null}
      {knowledge.content_preview ? <pre className="max-w-full overflow-x-auto rounded-2xl border border-border bg-black/40 p-4 text-xs leading-6 text-slate-300">{knowledge.content_preview}</pre> : null}
    </OperatorStack>
  )
}

function ExecutionListCard({ items }: { items: Array<{ execution_id: string; status: string; risk_level?: string; golden_summary?: { headline?: string; next_action?: string } }> }) {
  const { t } = useI18n()
  return (
    <OperatorStack>
      {items.map((item) => (
        <Link key={item.execution_id} to={`/executions/${item.execution_id}`} className="min-w-0 rounded-2xl border border-border bg-white/[0.03] p-4 transition-colors hover:bg-white/[0.05]">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="break-words text-sm font-semibold text-foreground">{item.golden_summary?.headline || item.execution_id}</div>
              <div className="mt-1 break-words text-sm text-muted-foreground">{item.golden_summary?.next_action || t('sessions.fields.executionFallback')}</div>
            </div>
            <div className="flex shrink-0 flex-col items-end gap-2">
              <Badge variant={statusTone(item.status)}>{humanizeLabel(item.status)}</Badge>
              <OperatorKicker label={t('sessions.fields.risk')} value={humanizeLabel(item.risk_level, 'Info')} tone={riskTone(item.risk_level)} />
            </div>
          </div>
        </Link>
      ))}
    </OperatorStack>
  )
}

function AttachmentsCard({ items }: { items: MessageAttachment[] }) {
  const { t } = useI18n()
  return (
    <div className="grid min-w-0 gap-4 md:grid-cols-2">
      {items.map((item, index) => {
        const href = buildAttachmentHref(item)
        const isImage = item.type === 'image' || item.mime_type?.startsWith('image/')
        return (
          <div key={`${item.type}-${item.name || index}`} className="min-w-0 rounded-2xl border border-border bg-white/[0.03] p-4">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="break-words text-sm font-semibold text-foreground">{item.name || `${item.type} ${t('sessions.fields.attachmentFallback')}`}</div>
                <div className="mt-1 break-all text-xs text-muted-foreground">{item.mime_type || 'binary/stream'}</div>
              </div>
              {href ? <a href={href} download={item.name} className="shrink-0 text-primary hover:underline">{t('sessions.action.download')}</a> : null}
            </div>
            <div className="mt-3 min-w-0">
              {isImage && href ? <img src={href} alt={item.name || t('sessions.fields.attachmentFallback')} className="max-h-[280px] max-w-full rounded-xl border border-border object-contain" /> : item.content ? <pre className="max-w-full overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-slate-300">{item.content}</pre> : <div className="break-words text-sm text-muted-foreground">{item.preview_text || t('sessions.empty.previewUnavailable')}</div>}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function AuditTrailCard({ items, traceError }: { items: AuditTraceEntry[]; traceError: string }) {
  const { t } = useI18n()

  if (traceError) {
    return <EmptyState icon={AlertCircle} title={t('sessions.empty.auditTrace')} description={traceError} />
  }

  return (
    <OperatorStack>
      {items.map((item, index) => (
        <div key={`${item.resource_id}-${item.action}-${index}`} className="min-w-0 rounded-2xl border border-border bg-white/[0.03] p-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="break-words text-sm font-semibold text-foreground">{item.resource_type}::{item.action}</div>
              <div className="mt-1 break-words text-sm text-muted-foreground">{t('sessions.fields.actor')}: {item.actor || t('sessions.fields.actorSystem')}</div>
            </div>
            <div className="shrink-0 text-xs text-muted-foreground">{formatRelativeTime(item.created_at)}</div>
          </div>
          {item.metadata ? <pre className="mt-3 max-w-full overflow-x-auto rounded-2xl border border-border bg-black/30 p-3 text-xs leading-6 text-slate-300">{JSON.stringify(item.metadata, null, 2)}</pre> : null}
        </div>
      ))}
    </OperatorStack>
  )
}

function CollapsibleBlock({
  title,
  summary,
  defaultOpen,
  children,
  className,
  summaryClassName,
}: {
  title: string
  summary: string
  defaultOpen?: boolean
  children: React.ReactNode
  className?: string
  summaryClassName?: string
}) {
  return (
    <details open={defaultOpen} className={cn('group min-w-0 rounded-2xl border border-border bg-black/10', className)}>
      <summary className={cn('flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3', summaryClassName)}>
        <div className="min-w-0">
          <div className="break-words text-sm font-semibold text-foreground">{title}</div>
          <div className="mt-1 break-words text-xs text-muted-foreground">{summary}</div>
        </div>
        <ChevronDown className="shrink-0 text-muted-foreground transition-transform group-open:rotate-180" size={16} />
      </summary>
      <div className="min-w-0 px-4 pb-4">{children}</div>
    </details>
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

function buildEvidenceRows(items: ToolPlanStep[]): EvidenceRow[] {
  return items.map((item) => ({
    tool: humanizeLabel(item.tool, item.tool),
    connector: item.connector_id || item.runtime?.connector_id || item.runtime?.runtime || 'n/a',
    status: humanizeLabel(item.status, 'Planned'),
    result: summarizeValue(item.output),
    details: item.reason || item.on_failure || 'No details recorded.',
    input: item.resolved_input || item.input,
    output: item.output,
  }))
}

function summarizeValue(value: unknown): string {
  if (value == null) {
    return 'No result recorded.'
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  if (Array.isArray(value)) {
    return value.length ? summarizeValue(value[0]) : 'No result recorded.'
  }
  if (typeof value !== 'object') {
    return 'No result recorded.'
  }

  const record = value as Record<string, unknown>
  for (const key of ['summary', 'result', 'message', 'headline', 'error', 'finding', 'description']) {
    const entry = record[key]
    if (typeof entry === 'string' || typeof entry === 'number' || typeof entry === 'boolean') {
      return String(entry)
    }
  }

  const [firstKey, firstValue] = Object.entries(record)[0] || []
  if (firstKey) {
    return `${humanizeLabel(firstKey)}: ${String(firstValue)}`
  }

  return 'No result recorded.'
}
