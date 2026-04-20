import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, ChevronDown, ChevronUp, Clock3, FileText, Link2, Server, ShieldAlert, TerminalSquare } from 'lucide-react'
import { fetchExecution, fetchExecutionOutput } from '../../lib/api/ops'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state'
import { OperatorHero, OperatorSection, OperatorStats, OperatorStack, OperatorKicker } from '@/components/operator/OperatorPage'
import { ExecutionActionBar } from '@/components/operator/ExecutionActionBar'
import { executionHeadline, executionNextAction, executionResult, executionRisk, formatDateTime, riskTone, shortID, statusTone } from '@/lib/operator'
import { useI18n } from '@/hooks/useI18n'

export const ExecutionDetailView = () => {
  const { t } = useI18n()
  const { id = '' } = useParams<{ id: string }>()
  const [commandExpanded, setCommandExpanded] = useState(false)

  const { data: execution, isLoading: detailLoading, error: detailError } = useQuery({
    queryKey: ['execution', id],
    queryFn: () => fetchExecution(id),
    enabled: !!id,
  })

  const { data: outputData, isLoading: outputLoading } = useQuery({
    queryKey: ['execution-output', id],
    queryFn: () => fetchExecutionOutput(id),
    enabled: !!id,
  })

  if (detailLoading || outputLoading) {
    return <div className="p-12 text-sm text-muted-foreground">{t('executions.detail.loading')}</div>
  }

  if (!execution || detailError) {
    return <EmptyState icon={ShieldAlert} title={t('executions.detail.notFound')} description={t('executions.detail.notFoundDesc')} action={<Button variant="outline" asChild><Link to="/executions">{t('executions.detail.backToList')}</Link></Button>} />
  }

  const outputLines = outputData?.chunks.flatMap((chunk) => chunk.content.split('\n')) || []
  const commandText = execution.command || `${execution.connector_id || 'connector'}::${execution.capability_id || 'raw_shell'}`
  const summaryCards = [
    { label: t('executions.detail.fields.what', 'What will execute'), value: executionHeadline(execution) },
    { label: t('executions.detail.fields.why', 'Why this run exists'), value: execution.golden_summary?.approval || executionResult(execution) },
    { label: t('executions.detail.fields.risk', 'Risk level'), value: executionRisk(execution) },
    { label: t('executions.detail.fields.approval', 'Approval note'), value: execution.golden_summary?.approval || executionResult(execution) },
    { label: t('executions.detail.fields.status', 'Execution status'), value: execution.status },
    { label: t('executions.detail.fields.result', 'Result summary'), value: executionResult(execution) },
    { label: t('executions.detail.fields.nextAction', 'Next observation step'), value: executionNextAction(execution) },
  ]

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('executions.detail.eyebrow')}
        title={executionHeadline(execution)}
        description={executionResult(execution)}
        chips={[
          { label: execution.status, tone: statusTone(execution.status) },
          { label: executionRisk(execution), tone: riskTone(executionRisk(execution)) },
          { label: shortID(execution.execution_id), tone: 'muted' },
        ]}
        primaryAction={<Button variant="amber" onClick={() => window.location.reload()}>{t('executions.detail.refresh')}</Button>}
        secondaryAction={<Button variant="outline" asChild><Link to="/executions"><ArrowLeft size={14} />{t('executions.detail.backToQueue')}</Link></Button>}
      />

      <OperatorStats
        stats={[
          { title: t('executions.detail.stats.status'), value: execution.status, description: t('executions.detail.stats.statusDesc'), icon: TerminalSquare, tone: statusTone(execution.status) },
          { title: t('executions.detail.stats.target'), value: execution.target_host || 'infrastructure', description: t('executions.detail.stats.targetDesc'), icon: Server, tone: 'info' },
          { title: t('executions.detail.stats.created'), value: formatDateTime(execution.created_at), description: t('executions.detail.stats.createdDesc'), icon: Clock3, tone: 'muted' },
          { title: t('executions.detail.stats.outputSize'), value: `${execution.output_bytes || 0} bytes`, description: execution.output_truncated ? t('executions.detail.stats.outputSizeDescTruncated') : t('executions.detail.stats.outputSizeDescFull'), icon: FileText, tone: execution.output_truncated ? 'warning' : 'info' },
        ]}
      />

      <OperatorSection title={t('executions.detail.sections.goldenPath')} description={t('executions.detail.sections.goldenPathDesc')}>
        <div className="mb-4 rounded-3xl border border-warning/30 bg-warning/5 p-4 text-sm leading-6 text-foreground">
          {executionRisk(execution) === 'critical'
            ? t('executions.detail.approval.autoRunWarning', 'This request is gated by operator approval and will not run automatically.')
            : t('executions.detail.approval.manualGate', 'This request remains in the approval queue until an operator allows it to proceed.')}
        </div>
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {summaryCards.map((card) => (
            <InsightCard key={card.label} label={card.label} value={card.value} />
          ))}
        </div>
      </OperatorSection>

      <div className="grid gap-6 xl:grid-cols-[1fr_360px]">
        <OperatorStack>
          <OperatorSection title={t('executions.detail.sections.command')} description={t('executions.detail.sections.commandDesc')}>
            <ExecutionActionBar execution={execution} onUpdated={() => window.location.reload()} className="mb-4" />
            <div className="rounded-2xl border border-border bg-black/20 p-4">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('executions.detail.fields.command', 'Command input')}</div>
                  <div className="mt-2 text-sm text-muted-foreground">{t('executions.detail.sections.commandDesc')}</div>
                </div>
                <Button variant="ghost" size="sm" onClick={() => setCommandExpanded((prev) => !prev)}>
                  {commandExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                  {commandExpanded ? t('executions.detail.command.hide', 'Hide command input') : t('executions.detail.command.show', 'Show command input')}
                </Button>
              </div>
              {commandExpanded ? (
                <pre className="mt-4 overflow-x-auto rounded-2xl border border-border bg-black/40 p-4 text-sm leading-6 text-emerald-300">{commandText}</pre>
              ) : null}
            </div>
          </OperatorSection>

          <OperatorSection title={t('executions.detail.sections.output')} description={t('executions.detail.sections.outputDesc')}>
            <div className="rounded-2xl border border-border bg-black/60 p-4">
              {outputLines.length > 0 ? (
                <pre className="overflow-auto text-[0.8rem] leading-6 text-slate-200">{outputLines.join('\n')}</pre>
              ) : (
                <div className="text-sm text-muted-foreground">{execution.output_ref || t('executions.detail.sections.outputNone')}</div>
              )}
            </div>
          </OperatorSection>
        </OperatorStack>

        <OperatorStack>
          <OperatorSection title={t('executions.detail.sections.metadata')} description={t('executions.detail.sections.metadataDesc')}>
            <MetaRow label={t('executions.detail.fields.meta.execution')} value={execution.execution_id} />
            <MetaRow label={t('executions.detail.fields.meta.session')} value={execution.session_id || '-'} />
            <MetaRow label={t('executions.detail.fields.meta.kind')} value={execution.request_kind || 'execution'} />
            <MetaRow label={t('executions.detail.fields.meta.mode')} value={execution.execution_mode || 'standard'} />
            <MetaRow label={t('executions.detail.fields.meta.connector')} value={execution.connector_id || '-'} />
            <MetaRow label={t('executions.detail.fields.meta.capability')} value={execution.capability_id || 'raw_shell'} />
            <MetaRow label={t('executions.detail.fields.meta.approvalGroup')} value={execution.approval_group || '-'} />
            <MetaRow label={t('executions.detail.fields.meta.exitCode')} value={execution.exit_code === undefined ? '-' : String(execution.exit_code)} />
          </OperatorSection>

          <OperatorSection title={t('executions.detail.sections.followThrough')} description={t('executions.detail.sections.followThroughDesc')}>
            <div className="flex flex-col gap-3">
              {execution.session_id ? (
                <Button variant="outline" asChild>
                  <Link to={`/sessions/${execution.session_id}`}><Link2 size={14} />{t('executions.detail.sections.openSession')}</Link>
                </Button>
              ) : null}
              <OperatorKicker label="risk" value={executionRisk(execution)} tone={riskTone(executionRisk(execution))} />
              <Badge variant={statusTone(execution.status)} className="w-fit">{execution.status}</Badge>
            </div>
          </OperatorSection>
        </OperatorStack>
      </div>
    </div>
  )
}

function InsightCard({ label, value }: { label: string; value: string }) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-[0.72rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{label}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-sm leading-6 text-foreground">{value}</div>
      </CardContent>
    </Card>
  )
}

function MetaRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-border/50 py-2 text-sm last:border-b-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="max-w-[220px] truncate font-mono text-foreground">{value}</span>
    </div>
  )
}
