import { useCallback, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { AlertTriangle, CheckCircle2, Copy, Download, Filter, Siren, Sparkles } from 'lucide-react'
import { BulkActionsBar } from '../../components/list/BulkActionsBar'
import { PaginationControls } from '../../components/list/PaginationControls'
import { useRegistry } from '../../hooks/registry/useRegistry'
import { useNotify } from '../../hooks/ui/useNotify'
import { bulkExportSessions, fetchSessions as fetchSessionList, getBlobApiErrorMessage, parseExportBlob } from '../../lib/api/ops'
import type { SessionDetail, SessionExportResponse } from '../../lib/api/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { EmptyState } from '@/components/ui/empty-state'
import { FilterBar } from '@/components/ui/filter-bar'
import { OperatorActionBar } from '@/components/operator/OperatorActionBar'
import { ActionResultNotice } from '@/components/operator/ActionResultNotice'
import { OperatorHero, OperatorSection, OperatorStats, OperatorStack, OperatorKicker } from '@/components/operator/OperatorPage'
import { alertLabel, formatRelativeTime, riskTone, sessionCurrentState, sessionEvidence, sessionHeadline, sessionLastUpdatedAt, sessionNextAction, sessionRisk, sessionSource, shortID, sortSessionsForTriage } from '@/lib/operator'
import { useI18n } from '@/hooks/useI18n'

export const SessionList = () => {
  const notify = useNotify()
  const { t } = useI18n()
  const [actionMessage, setActionMessage] = useState('')
  const [actionError, setActionError] = useState('')

  const {
    items: sessions, total, page, limit, loading, error, query, filters,
    setPage, setLimit, setQuery, setFilters,
    selectedIDs, toggleSelection, selectAll, setSelectedIDs,
  } = useRegistry<SessionDetail, { status: string; sortBy: string; sortOrder: string }>({
    key: 'sessions',
    getItemId: (session) => session.session_id,
    fetcher: (params) => fetchSessionList({
      ...params,
      status: params.filters?.status,
      sort_by: params.filters?.sortBy,
      sort_order: params.filters?.sortOrder,
    }),
    initialFilters: { status: '', sortBy: 'triage', sortOrder: 'desc' },
  })

  const sortedSessions = useMemo(() => {
    if (filters.sortBy === 'triage') {
      return sortSessionsForTriage(sessions, filters.sortOrder)
    }
    return sessions
  }, [filters.sortBy, filters.sortOrder, sessions])

  const handleBulkExport = useCallback(async () => {
    try {
      setActionError('')
      setActionMessage('')
      const download = await bulkExportSessions(selectedIDs, t('sessions.exportRequest', 'Export sessions'))
      const payload = await parseExportBlob<SessionExportResponse>(download.content)
      const url = window.URL.createObjectURL(download.content)
      const link = document.createElement('a')
      link.href = url
      link.download = download.filename
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
      setActionMessage(t('sessions.exportedSummary', `Exported ${payload.exported_count} session(s); ${payload.failed_count} failed.`))
      setSelectedIDs([])
      notify.success(t('sessions.exportSuccess', 'Sessions exported successfully'))
    } catch (exportError) {
      const msg = await getBlobApiErrorMessage(exportError, t('sessions.exportFailed', 'Failed to export sessions.'))
      setActionError(msg)
      notify.error(exportError, t('sessions.exportFailedShort', 'Export failed'))
    }
  }, [notify, selectedIDs, setSelectedIDs, t])

  const allSelected = sortedSessions.length > 0 && sortedSessions.every((session) => selectedIDs.includes(session.session_id))

  const stats = useMemo(() => {
    const needsAction = sortedSessions.filter((item) => {
      const state = sessionCurrentState(item).code
      return ['pendingApproval', 'executing', 'manualIntervention'].includes(state)
    }).length
    const highRisk = sortedSessions.filter((item) => ['critical', 'warning'].includes(sessionRisk(item).toLowerCase())).length
    const smokeCount = sortedSessions.filter((item) => item.is_smoke).length
    return { needsAction, highRisk, smokeCount }
  }, [sortedSessions])

  const resolvedCount = useMemo(() => sortedSessions.filter((item) => sessionCurrentState(item).code === 'resolved').length, [sortedSessions])

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('nav.sessions')}
        title={t('sessions.hero.title')}
        description={t('sessions.hero.description')}
        chips={[
          { label: t('sessions.chip.active', { count: sortedSessions.length - resolvedCount }), tone: sortedSessions.length - resolvedCount > 0 ? 'warning' : 'success' },
          { label: t('sessions.chip.highRisk', { count: stats.highRisk }), tone: stats.highRisk > 0 ? 'danger' : 'info' },
          { label: t('sessions.chip.smoke', { count: stats.smokeCount }), tone: 'muted' },
        ]}
        primaryAction={<Button variant="amber" onClick={() => void handleBulkExport()} disabled={selectedIDs.length === 0}><Download size={14} />{t('sessions.exportSelected')}</Button>}
        secondaryAction={<Button variant="outline" asChild><Link to="/runtime-checks"><Sparkles size={14} />{t('sessions.openChecks')}</Link></Button>}
      />

      <ActionResultNotice tone="error" message={error || undefined} />
      <ActionResultNotice tone="error" message={actionError} />
      <ActionResultNotice tone="success" message={actionMessage} />

      <OperatorStats
        stats={[
          { title: t('sessions.stats.visible'), value: total, description: t('sessions.stats.visibleDesc'), icon: Siren, tone: 'info' },
          { title: t('sessions.stats.attention'), value: stats.needsAction, description: t('sessions.stats.attentionDesc'), icon: AlertTriangle, tone: stats.needsAction > 0 ? 'warning' : 'success' },
          { title: t('sessions.stats.highRisk'), value: stats.highRisk, description: t('sessions.stats.highRiskDesc'), icon: Filter, tone: stats.highRisk > 0 ? 'danger' : 'muted' },
          { title: t('sessions.stats.resolved'), value: resolvedCount, description: t('sessions.stats.resolvedDesc'), icon: CheckCircle2, tone: 'success' },
        ]}
      />

      <FilterBar
        search={{ value: query, onChange: setQuery, placeholder: t('sessions.search') }}
        filters={[
          {
            key: 'status',
            value: filters.status,
            onChange: (value) => setFilters({ status: value }),
            options: [
              { value: '', label: t('sessions.status.all') },
              { value: 'open', label: t('sessions.status.open') },
              { value: 'analyzing', label: t('sessions.status.analyzing') },
              { value: 'pending_approval', label: t('sessions.status.pendingApproval') },
              { value: 'executing', label: t('sessions.status.executing') },
              { value: 'verifying', label: t('sessions.status.verifying') },
              { value: 'resolved', label: t('sessions.status.resolved') },
              { value: 'failed', label: t('sessions.status.failed') },
            ],
          },
          {
            key: 'sortBy',
            value: filters.sortBy,
            onChange: (value) => setFilters({ sortBy: value }),
            options: [
              { value: 'triage', label: t('sessions.sort.triage') },
              { value: 'updated_at', label: t('sessions.sort.updated') },
              { value: 'created_at', label: t('sessions.sort.created') },
              { value: 'status', label: t('sessions.sort.status') },
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

      <BulkActionsBar
        selectedCount={selectedIDs.length}
        onClear={() => setSelectedIDs([])}
        summaryText={t('sessions.bulk.selected', { count: selectedIDs.length })}
        clearLabel={t('sessions.bulk.clear')}
        actions={[{ key: 'export', label: t('sessions.bulk.export'), onClick: () => { void handleBulkExport() } }]}
      />

      <OperatorActionBar
        title={t('sessions.queue.title')}
        description={t('sessions.queue.description')}
        actions={<Button variant="outline" size="sm" onClick={() => selectAll()} aria-label={t('sessions.selectAll')}>{allSelected ? t('sessions.clearPageSelection') : t('sessions.selectAll')}</Button>}
      />

      <OperatorSection title={t('sessions.section.title')} description={t('sessions.section.description')}>
        {sortedSessions.length === 0 && !loading ? (
          <EmptyState
            icon={CheckCircle2}
            title={t('sessions.empty.title')}
            description={t('sessions.empty.description')}
            action={<Button variant="outline" asChild><Link to="/runtime-checks"><Sparkles size={14} />{t('sessions.openChecks')}</Link></Button>}
          />
        ) : (
          <OperatorStack>
            {sortedSessions.map((session) => {
              const service = alertLabel(session.alert, 'service') || t('sessions.fallback.service')
              const host = alertLabel(session.alert, 'instance') || alertLabel(session.alert, 'host') || t('sessions.fallback.host')
              const source = sessionSource(session) || t('sessions.fallback.source')
              const triageState = sessionCurrentState(session)
              const updatedAt = formatRelativeTime(sessionLastUpdatedAt(session))
              const risk = sessionRisk(session)
              const topEvidence = sessionEvidence(session)[0] || t('sessions.fields.evidenceEmpty')
              return (
                <div key={session.session_id} className="rounded-3xl border border-border bg-white/[0.03] p-5 transition-colors hover:bg-white/[0.05]">
                  <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                    <div className="flex items-start gap-4">
                      <Checkbox checked={selectedIDs.includes(session.session_id)} onCheckedChange={() => toggleSelection(session.session_id)} aria-label={`Select session ${session.session_id}`} className="mt-1" />
                      <div className="space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-base font-semibold text-foreground">{sessionHeadline(session)}</span>
                          <Badge variant={riskTone(risk)}>{risk}</Badge>
                          <Badge variant={triageState.tone}>{t(`sessions.states.${triageState.code}`)}</Badge>
                          {session.is_smoke ? <Badge variant="outline">{t('sessions.smoke')}</Badge> : null}
                        </div>
                        <div className="grid gap-3 lg:grid-cols-3">
                          <div className="rounded-2xl border border-border bg-black/20 p-3">
                            <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('sessions.fields.currentState')}</div>
                            <div className="mt-2 text-sm font-semibold leading-6 text-foreground">{t(`sessions.states.${triageState.code}`)}</div>
                          </div>
                          <div className="rounded-2xl border border-border bg-black/20 p-3 lg:col-span-2">
                            <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('sessions.fields.currentConclusion')}</div>
                            <div className="mt-2 text-sm leading-6 text-foreground">{session.golden_summary?.conclusion || t('sessions.fallback.conclusion')}</div>
                          </div>
                        </div>
                        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                          <div className="rounded-2xl border border-border bg-black/20 p-3">
                            <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('sessions.fields.evidence')}</div>
                            <div className="mt-2 text-sm leading-6 text-foreground">{topEvidence}</div>
                          </div>
                          <div className="rounded-2xl border border-border bg-black/20 p-3">
                            <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('sessions.fields.risk')}</div>
                            <div className="mt-2 text-sm font-semibold leading-6 text-foreground">{risk}</div>
                          </div>
                          <div className="rounded-2xl border border-border bg-black/20 p-3">
                            <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('common.nextAction')}</div>
                            <div className="mt-2 text-sm leading-6 text-foreground">{sessionNextAction(session)}</div>
                          </div>
                        </div>
                        <div className="grid gap-3 text-sm text-muted-foreground md:grid-cols-4">
                          <div><span className="font-semibold text-foreground">{t('common.service')}:</span> {service}</div>
                          <div><span className="font-semibold text-foreground">{t('common.host')}:</span> {host}</div>
                          <div><span className="font-semibold text-foreground">{t('common.source')}:</span> {source}</div>
                          <div><span className="font-semibold text-foreground">{t('common.updated')}:</span> {updatedAt}</div>
                        </div>
                      </div>
                    </div>

                    <div className="grid gap-3 xl:max-w-[360px] xl:min-w-[320px]">
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex items-center gap-1.5">
                          <OperatorKicker label={t('sessions.kickerLabel')} value={shortID(session.session_id)} tone={riskTone(risk)} />
                          <button
                            type="button"
                            className="rounded p-0.5 text-muted-foreground hover:text-foreground transition-colors"
                            onClick={() => {
                              void navigator.clipboard.writeText(session.session_id)
                              notify.success(t('common.copiedTitle'), t('common.copiedDescription'))
                            }}
                            title={t('sessions.copyIdTitle', { id: session.session_id })}
                          >
                            <Copy size={11} />
                          </button>
                        </div>
                        <Button variant="ghost" size="sm" asChild>
                          <Link to={`/sessions/${session.session_id}`}>{t('sessions.openIncident')}</Link>
                        </Button>
                      </div>
                      <div className="rounded-2xl border border-border bg-white/[0.03] p-3 text-sm text-muted-foreground">
                        <div className="font-semibold text-foreground">{alertLabel(session.alert, 'alertname') || t('sessions.fallback.alertname')}</div>
                        <div className="mt-1">{sessionStatusLabel(t, session.status)}</div>
                      </div>
                    </div>
                  </div>
                </div>
              )
            })}
          </OperatorStack>
        )}
      </OperatorSection>

      {!loading && !error ? (
        <PaginationControls page={page} limit={limit} total={total} hasNext={sortedSessions.length === limit} onPageChange={setPage} onLimitChange={setLimit} />
      ) : null}
    </div>
  )
}

function sessionStatusLabel(t: ReturnType<typeof useI18n>['t'], status: SessionDetail['status']) {
  return t(`sessions.backendStatus.${status}`, status.replaceAll('_', ' '))
}
