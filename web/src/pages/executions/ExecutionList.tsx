import { useCallback, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { CheckCircle2, Clock3, Download, ShieldAlert, TerminalSquare, XCircle } from 'lucide-react'
import { BulkActionsBar } from '../../components/list/BulkActionsBar'
import { PaginationControls } from '../../components/list/PaginationControls'
import { useRegistry } from '../../hooks/registry/useRegistry'
import { useNotify } from '../../hooks/ui/useNotify'
import { bulkExportExecutions, fetchExecutions as fetchExecutionList, getBlobApiErrorMessage, parseExportBlob } from '../../lib/api/ops'
import type { ExecutionDetail, ExecutionExportResponse } from '../../lib/api/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { EmptyState } from '@/components/ui/empty-state'
import { FilterBar } from '@/components/ui/filter-bar'
import { ActionResultNotice } from '@/components/operator/ActionResultNotice'
import { OperatorActionBar } from '@/components/operator/OperatorActionBar'
import { OperatorHero, OperatorSection, OperatorStats, OperatorStack, OperatorKicker } from '@/components/operator/OperatorPage'
import { executionHeadline, executionNextAction, executionRisk, formatRelativeTime, riskTone, shortID, statusTone } from '@/lib/operator'
import { useI18n } from '@/hooks/useI18n'

export const ExecutionList = () => {
  const notify = useNotify()
  const { t } = useI18n()
  const [actionMessage, setActionMessage] = useState('')
  const [actionError, setActionError] = useState('')

  const {
    items, total, page, limit, loading, error, query, filters,
    setPage, setLimit, setQuery, setFilters,
    selectedIDs, toggleSelection, selectAll, setSelectedIDs,
  } = useRegistry<ExecutionDetail, { status: string; sortBy: string; sortOrder: string }>({
    key: 'executions',
    getItemId: (execution) => execution.execution_id,
    fetcher: (params) => fetchExecutionList({
      ...params,
      status: params.filters?.status,
      sort_by: params.filters?.sortBy,
      sort_order: params.filters?.sortOrder,
    }),
    initialFilters: { status: '', sortBy: 'triage', sortOrder: 'desc' },
  })

  const queuedItems = useMemo(() => [...items].sort((left, right) => {
    const stateDelta = queueStateWeight(right.status) - queueStateWeight(left.status)
    if (stateDelta !== 0) {
      return stateDelta
    }

    const riskDelta = queueRiskWeight(executionRisk(right)) - queueRiskWeight(executionRisk(left))
    if (riskDelta !== 0) {
      return riskDelta
    }

    return queueTimestamp(right) - queueTimestamp(left)
  }), [items])

  const handleBulkExport = useCallback(async () => {
    try {
      setActionError('')
      setActionMessage('')
      const download = await bulkExportExecutions(selectedIDs, t('executions.exportRequest', 'Export executions'))
      const payload = await parseExportBlob<ExecutionExportResponse>(download.content)
      const url = window.URL.createObjectURL(download.content)
      const link = document.createElement('a')
      link.href = url
      link.download = download.filename
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
      setActionMessage(t('executions.exportedSummary', `Exported ${payload.exported_count} execution(s); ${payload.failed_count} failed.`))
      setSelectedIDs([])
      notify.success(t('executions.exportSuccess', 'Executions exported successfully'))
    } catch (exportError) {
      const msg = await getBlobApiErrorMessage(exportError, t('executions.exportFailed', 'Failed to export executions.'))
      setActionError(msg)
      notify.error(exportError, t('executions.exportFailedShort', 'Export failed'))
    }
  }, [notify, selectedIDs, setSelectedIDs, t])

  const stats = useMemo(() => ({
    pending: items.filter((item) => ['pending', 'approved', 'executing'].includes(item.status)).length,
    failed: items.filter((item) => ['failed', 'timeout', 'rejected'].includes(item.status)).length,
    completed: items.filter((item) => item.status === 'completed').length,
  }), [items])

  const allSelected = items.length > 0 && items.every((item) => selectedIDs.includes(item.execution_id))

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('nav.executions')}
        title={t('executions.hero.title')}
        description={t('executions.hero.description')}
        chips={[
          { label: t('executions.stats.pending', { count: stats.pending }), tone: stats.pending > 0 ? 'warning' : 'success' },
          { label: t('executions.stats.failed', { count: stats.failed }), tone: stats.failed > 0 ? 'danger' : 'success' },
          { label: t('executions.stats.completed', { count: stats.completed }), tone: 'info' },
        ]}
        primaryAction={<Button variant="amber" onClick={() => void handleBulkExport()} disabled={selectedIDs.length === 0}><Download size={14} />{t('executions.exportSelected')}</Button>}
        secondaryAction={<Button variant="outline" asChild><Link to="/sessions"><ShieldAlert size={14} />{t('executions.backToIncidents')}</Link></Button>}
      />

      <ActionResultNotice tone="error" message={error || undefined} />
      <ActionResultNotice tone="error" message={actionError} />
      <ActionResultNotice tone="success" message={actionMessage} />

      <OperatorStats
        stats={[
          { title: t('executions.stats.visible'), value: total, description: t('executions.stats.visibleDesc'), icon: TerminalSquare, tone: 'info' },
          { title: t('executions.stats.pending'), value: stats.pending, description: t('executions.stats.pendingDesc'), icon: Clock3, tone: stats.pending > 0 ? 'warning' : 'success' },
          { title: t('executions.stats.completed'), value: stats.completed, description: t('executions.stats.completedDesc'), icon: CheckCircle2, tone: 'success' },
          { title: t('executions.stats.failed'), value: stats.failed, description: t('executions.stats.failedDesc'), icon: XCircle, tone: stats.failed > 0 ? 'danger' : 'success' },
        ]}
      />

      <FilterBar
        search={{ value: query, onChange: setQuery, placeholder: t('executions.search') }}
        filters={[
          {
            key: 'status',
            value: filters.status,
            onChange: (value) => setFilters({ status: value }),
            options: [
              { value: '', label: t('executions.status.all') },
              { value: 'pending', label: t('executions.status.pending') },
              { value: 'approved', label: t('executions.status.approved') },
              { value: 'executing', label: t('executions.status.executing') },
              { value: 'completed', label: t('executions.status.completed') },
              { value: 'failed', label: t('executions.status.failed') },
              { value: 'timeout', label: t('executions.status.timeout') },
              { value: 'rejected', label: t('executions.status.rejected') },
            ],
          },
          {
            key: 'sortBy',
            value: filters.sortBy,
            onChange: (value) => setFilters({ sortBy: value }),
            options: [
              { value: 'triage', label: t('executions.sort.triage') },
              { value: 'created_at', label: t('executions.sort.created') },
              { value: 'completed_at', label: t('executions.sort.completed') },
              { value: 'status', label: t('executions.sort.status') },
            ],
          },
          {
            key: 'sortOrder',
            value: filters.sortOrder,
            onChange: (value) => setFilters({ sortOrder: value }),
            options: [
              { value: 'desc', label: t('common.newest') },
              { value: 'asc', label: t('common.oldest') },
            ],
            className: 'md:w-32',
          },
        ]}
      />

      <BulkActionsBar selectedCount={selectedIDs.length} onClear={() => setSelectedIDs([])} actions={[{ key: 'export', label: t('common.exportJson'), onClick: () => { void handleBulkExport() } }]} />

      <OperatorActionBar title={t('executions.queue.title')} description={t('executions.queue.description')} actions={<Button variant="outline" size="sm" onClick={() => selectAll()} aria-label={t('executions.selectAll')}>{allSelected ? t('executions.clearPageSelection') : t('common.selectPage')}</Button>} />

      <OperatorSection title={t('executions.section.title')} description={t('executions.section.description')}>
        {items.length === 0 && !loading ? (
          <EmptyState
            icon={CheckCircle2}
            title={t('executions.empty.title')}
            description={t('executions.empty.description')}
            action={<Button variant="outline" asChild><Link to="/sessions"><ShieldAlert size={14} />{t('executions.backToIncidents')}</Link></Button>}
          />
        ) : (
          <OperatorStack>
            {queuedItems.map((item) => (
              <div key={item.execution_id} className="rounded-3xl border border-border bg-white/[0.03] p-5 transition-colors hover:bg-white/[0.05]">
                <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                  <div className="flex items-start gap-4">
                    <Checkbox checked={selectedIDs.includes(item.execution_id)} onCheckedChange={() => toggleSelection(item.execution_id)} aria-label={`Select execution ${item.execution_id}`} className="mt-1" />
                    <div className="space-y-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-base font-semibold text-foreground">{executionHeadline(item)}</span>
                        <Badge variant={statusTone(item.status)}>{item.status}</Badge>
                        <Badge variant={riskTone(executionRisk(item))}>{executionRisk(item)}</Badge>
                      </div>
                      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                        <QueueField label={t('executions.fields.what', 'What will execute')} value={executionHeadline(item)} />
                        <QueueField label={t('executions.fields.why', 'Why it was requested')} value={item.golden_summary?.approval || t('executions.fields.approvalEmpty', 'No approval summary recorded.')} />
                        <QueueField label={t('executions.fields.status', 'Execution status')} value={item.status} />
                        <QueueField label={t('executions.fields.result', 'Result')} value={item.golden_summary?.result || item.golden_summary?.approval || t('executions.fallback.result')} />
                        <QueueField label={t('executions.fields.nextAction', 'Next action')} value={executionNextAction(item)} />
                        <QueueField label={t('executions.fields.risk', 'Risk level')} value={executionRisk(item)} />
                      </div>
                      <div className="grid gap-3 text-sm text-muted-foreground md:grid-cols-3">
                        <div><span className="font-semibold text-foreground">{t('common.host')}:</span> {item.target_host || t('executions.fallback.host')}</div>
                        <div><span className="font-semibold text-foreground">{t('executions.fields.kind')}:</span> {item.request_kind || t('executions.fallback.kind')}</div>
                        <div><span className="font-semibold text-foreground">{t('common.updated')}:</span> {formatRelativeTime(item.completed_at || item.approved_at || item.created_at)}</div>
                      </div>
                    </div>
                  </div>

                  <div className="grid gap-3 xl:max-w-[360px] xl:min-w-[320px]">
                    <div className="rounded-2xl border border-border bg-black/20 p-3">
                      <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('common.nextAction')}</div>
                       <div className="mt-2 text-sm leading-6 text-foreground">{executionNextAction(item)}</div>
                     </div>
                     <div className="flex items-center justify-between gap-3">
                       <OperatorKicker label={t('executions.kickerLabel')} value={shortID(item.execution_id)} tone={riskTone(executionRisk(item))} />
                       <Button variant="ghost" size="sm" asChild>
                        <Link to={`/executions/${item.execution_id}`}>{t('common.openDetail')}</Link>
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </OperatorStack>
        )}
      </OperatorSection>

      {!loading && !error ? (
        <PaginationControls page={page} limit={limit} total={total} hasNext={items.length === limit} onPageChange={setPage} onLimitChange={setLimit} />
      ) : null}
    </div>
  )
}

function QueueField({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-border/60 bg-black/20 p-3">
      <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{label}</div>
      <div className="mt-2 text-sm leading-6 text-foreground">{value}</div>
    </div>
  )
}

function queueStateWeight(status: ExecutionDetail['status']): number {
  switch (status) {
    case 'pending':
      return 5
    case 'executing':
      return 4
    case 'approved':
      return 3
    case 'failed':
    case 'timeout':
      return 2
    case 'rejected':
      return 1
    case 'completed':
    default:
      return 0
  }
}

function queueRiskWeight(risk: string): number {
  switch (risk.toLowerCase()) {
    case 'critical':
      return 3
    case 'warning':
      return 2
    default:
      return 1
  }
}

function queueTimestamp(item: ExecutionDetail): number {
  const value = item.completed_at || item.approved_at || item.created_at
  if (!value) {
    return 0
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 0 : date.getTime()
}
