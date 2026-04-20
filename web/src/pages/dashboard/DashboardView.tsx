import { AlertTriangle, ArrowRight, Bell, BrainCircuit, CheckCircle2, Clock3, RefreshCw, ShieldAlert, Siren, Sparkles, TerminalSquare } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useMemo } from 'react'
import { useRequest } from 'ahooks'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { EmptyState } from '@/components/ui/empty-state'
import { OperatorHero, OperatorSection, OperatorStats, OperatorCardGrid, OperatorStack, OperatorKicker } from '@/components/operator/OperatorPage'
import { fetchDashboardHealth, fetchOpsSummary, fetchSessions, fetchExecutions, getApiErrorMessage } from '@/lib/api/ops'
import { alertTone, formatRelativeTime, riskTone, sessionAlertName, sessionHeadline, shortID, statusTone } from '@/lib/operator'
import { cn } from '@/lib/utils'
import { useI18n } from '@/hooks/useI18n'

export const DashboardView = () => {
  const { t } = useI18n()
  const { data: summary, loading: summaryLoading, run: refreshSummary, error: summaryError } = useRequest(fetchOpsSummary)
  const { data: health, loading: healthLoading, run: refreshHealth, error: healthError } = useRequest(fetchDashboardHealth)
  const { data: sessionsData, loading: sessionsLoading, run: refreshSessions } = useRequest(() => fetchSessions({ limit: 6, sort_by: 'updated_at', sort_order: 'desc' }))
  const { data: executionsData, loading: executionsLoading, run: refreshExecutions } = useRequest(() => fetchExecutions({ limit: 6, sort_by: 'created_at', sort_order: 'desc' }))

  const loading = summaryLoading || healthLoading || sessionsLoading || executionsLoading
  const error = summaryError || healthError ? getApiErrorMessage(summaryError || healthError, t('error.failedToLoad')) : ''

  const sessions = sessionsData?.items ?? []
  const executions = executionsData?.items ?? []

  const pendingSessions = useMemo(() => sessions.filter((item) => ['open', 'analyzing', 'pending_approval', 'executing', 'verifying'].includes(item.status)), [sessions])
  const pendingExecutions = useMemo(() => executions.filter((item) => ['pending', 'approved', 'executing'].includes(item.status)), [executions])
  const failingExecutions = useMemo(() => executions.filter((item) => ['failed', 'timeout', 'rejected'].includes(item.status)), [executions])

  const handleRefresh = () => {
    refreshSummary()
    refreshHealth()
    refreshSessions()
    refreshExecutions()
  }

  return (
    <div className="flex flex-col gap-6 pb-10">
      <OperatorHero
        eyebrow={t('nav.dashboard')}
        title={t('dash.hero.title')}
        description={t('dash.hero.description')}
        chips={[
          { label: `${summary?.active_alerts ?? 0} ${t('dash.hero.activeAlerts')}`, tone: (summary?.active_alerts ?? 0) > 0 ? 'warning' : 'success' },
          { label: `${summary?.pending_approvals ?? 0} ${t('dash.hero.waitingApprovals')}`, tone: (summary?.pending_approvals ?? 0) > 0 ? 'warning' : 'info' },
          { label: `${summary?.provider_failures ?? 0} ${t('dash.hero.providerFailures')}`, tone: (summary?.provider_failures ?? 0) > 0 ? 'danger' : 'success' },
        ]}
        primaryAction={
          <div className="flex items-center gap-3">
            <Button variant="outline" asChild><Link to="/executions">{t('dash.sections.openRuns')}</Link></Button>
            <Button variant="amber" asChild><Link to="/sessions"><Sparkles size={14} />{t('dash.hero.reviewIncidents')}</Link></Button>
          </div>
        }
        secondaryAction={
          <div className="flex items-center gap-3">
            <Button variant="ghost" size="sm" asChild className="text-muted-foreground hover:text-foreground">
              <Link to="/runtime-checks">{t('dash.hero.runtimeChecks')}</Link>
            </Button>
            <Button variant="outline" size="sm" onClick={handleRefresh} className="text-muted-foreground flex items-center gap-2">
              <RefreshCw size={14} className={cn(loading && 'animate-spin')} />
              <span>{loading ? t('dash.hero.refreshing') : t('dash.hero.refresh')}</span>
            </Button>
          </div>
        }
      />

      {error ? <RuntimeAlert tone="danger" title={t('error.runtimeDegraded')} description={error} /> : null}

      <OperatorStats
        stats={[
          { title: t('dash.stats.activeIncidents'), value: summary?.active_sessions ?? pendingSessions.length ?? 0, description: t('dash.stats.activeIncidentsDesc'), icon: Siren, tone: pendingSessions.length > 0 ? 'warning' : 'success' },
          { title: t('dash.stats.pendingApprovals'), value: summary?.pending_approvals ?? pendingExecutions.length, description: t('dash.stats.pendingApprovalsDesc'), icon: ShieldAlert, tone: (summary?.pending_approvals ?? pendingExecutions.length) > 0 ? 'warning' : 'success' },
          { title: t('dash.stats.failedRuns'), value: failingExecutions.length, description: t('dash.stats.failedRunsDesc'), icon: TerminalSquare, tone: failingExecutions.length > 0 ? 'danger' : 'success' },
          { title: t('dash.stats.signalHealth'), value: `${health?.summary.healthy_connectors ?? 0}/${(health?.summary.healthy_connectors ?? 0) + (health?.summary.degraded_connectors ?? 0) + (health?.summary.disabled_connectors ?? 0)}`, description: t('dash.stats.signalHealthDesc'), icon: BrainCircuit, tone: (health?.summary.degraded_connectors ?? 0) > 0 ? 'warning' : 'info' },
        ]}
      />

      <OperatorCardGrid>
        <OperatorSection
          title={t('dash.sections.incidentQueue')}
          description={t('dash.sections.incidentQueueDesc')}
          action={<Button variant="ghost" size="sm" asChild><Link to="/sessions">{t('dash.sections.openSessions')} <ArrowRight size={14} /></Link></Button>}
        >
          {pendingSessions.length === 0 ? (
            <EmptyState icon={CheckCircle2} title={t('dash.alerts.noIncidents')} description={t('dash.alerts.noIncidentsDesc')} />
          ) : (
            <OperatorStack>
              {pendingSessions.slice(0, 5).map((session) => (
                <Link key={session.session_id} to={`/sessions/${session.session_id}`} className="rounded-2xl border border-border bg-white/[0.03] p-4 transition-colors hover:bg-white/[0.05]">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-semibold text-foreground">{sessionAlertName(session)}</span>
                        <Badge variant={statusTone(session.status)}>{session.status.replaceAll('_', ' ')}</Badge>
                        {session.is_smoke ? <Badge variant="outline">smoke</Badge> : null}
                      </div>
                      <div className="text-sm leading-6 text-muted-foreground">{sessionHeadline(session)}</div>
                    </div>
                    <div className="flex flex-col items-end gap-2">
                      <OperatorKicker label="session" value={shortID(session.session_id)} tone={riskTone(session.golden_summary?.risk)} />
                      <span className="text-xs text-muted-foreground">{formatRelativeTime(session.timeline?.[session.timeline.length - 1]?.created_at)}</span>
                    </div>
                  </div>
                </Link>
              ))}
            </OperatorStack>
          )}
        </OperatorSection>

        <OperatorSection
          title={t('dash.sections.executionQueue')}
          description={t('dash.sections.executionQueueDesc')}
          action={<Button variant="ghost" size="sm" asChild><Link to="/executions">{t('dash.sections.openRuns')} <ArrowRight size={14} /></Link></Button>}
        >
          {pendingExecutions.length === 0 ? (
            <EmptyState icon={TerminalSquare} title={t('dash.alerts.noExecutions')} description={t('dash.alerts.noExecutionsDesc')} />
          ) : (
            <OperatorStack>
              {pendingExecutions.slice(0, 5).map((execution) => (
                <Link key={execution.execution_id} to={`/executions/${execution.execution_id}`} className="rounded-2xl border border-border bg-white/[0.03] p-4 transition-colors hover:bg-white/[0.05]">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-semibold text-foreground">{execution.golden_summary?.headline || execution.command || execution.capability_id || 'Execution request'}</span>
                        <Badge variant={statusTone(execution.status)}>{execution.status}</Badge>
                      </div>
                      <div className="text-sm leading-6 text-muted-foreground">{execution.golden_summary?.result || execution.golden_summary?.approval || execution.golden_summary?.next_action || 'Waiting for execution update.'}</div>
                    </div>
                    <div className="flex flex-col items-end gap-2">
                      <OperatorKicker label="risk" value={execution.golden_summary?.risk || execution.risk_level || 'info'} tone={riskTone(execution.golden_summary?.risk || execution.risk_level)} />
                      <span className="text-xs text-muted-foreground">{execution.target_host || 'infrastructure'}</span>
                    </div>
                  </div>
                </Link>
              ))}
            </OperatorStack>
          )}
        </OperatorSection>
      </OperatorCardGrid>

      <OperatorCardGrid>
        <OperatorSection title={t('dash.sections.signalPosture')} description={t('dash.sections.signalPostureDesc')}>
          <div className="grid gap-3">
            <RuntimeSignalRow icon={Bell} label={t('dash.signals.activeAlerts')} value={String(health?.summary.active_alerts ?? 0)} tone={(health?.summary.active_alerts ?? 0) > 0 ? 'warning' : 'success'} detail={`${health?.alerts?.length ?? 0} surfaced in dashboard feed`} />
            <RuntimeSignalRow icon={BrainCircuit} label={t('dash.signals.providerFailures')} value={String(health?.summary.provider_failures ?? 0)} tone={(health?.summary.provider_failures ?? 0) > 0 ? 'danger' : 'success'} detail={`${health?.providers?.length ?? 0} providers tracked`} />
            <RuntimeSignalRow icon={Clock3} label={t('dash.signals.outboxPressure')} value={`${summary?.blocked_outbox ?? 0}/${summary?.visible_outbox ?? 0}`} tone={(summary?.blocked_outbox ?? 0) > 0 || (summary?.failed_outbox ?? 0) > 0 ? 'warning' : 'info'} detail={`${summary?.failed_outbox ?? 0} failed delivery events`} />
          </div>
        </OperatorSection>

        <OperatorSection title={t('dash.sections.hotAlerts')} description={t('dash.sections.hotAlertsDesc')}>
          {health?.alerts?.length ? (
            <OperatorStack>
              {health.alerts.slice(0, 5).map((alert, index) => (
                <div key={`${alert.title || 'alert'}-${index}`} className="rounded-2xl border border-border bg-white/[0.03] p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1.5">
                      <div className="text-sm font-semibold text-foreground">{alert.title || alert.resource || 'Untitled alert'}</div>
                      <div className="text-sm leading-6 text-muted-foreground">{alert.summary || 'No alert summary provided.'}</div>
                    </div>
                    <Badge variant={alertTone(alert)}>{alert.severity || 'info'}</Badge>
                  </div>
                </div>
              ))}
            </OperatorStack>
          ) : (
            <EmptyState icon={CheckCircle2} title={t('dash.alerts.noHotAlerts')} description={t('dash.alerts.noHotAlertsDesc')} />
          )}
        </OperatorSection>
      </OperatorCardGrid>
    </div>
  )
}

function RuntimeAlert({ tone, title, description }: { tone: 'danger' | 'warning'; title: string; description: string }) {
  return (
    <div className={cn('flex items-start gap-3 rounded-2xl border px-4 py-3', tone === 'danger' ? 'border-danger/30 bg-danger/10 text-danger' : 'border-warning/30 bg-warning/10 text-warning')}>
      <AlertTriangle className="mt-0.5 size-4 shrink-0" />
      <div>
        <div className="text-sm font-semibold">{title}</div>
        <div className="text-sm leading-6 opacity-90">{description}</div>
      </div>
    </div>
  )
}

function RuntimeSignalRow({ icon: Icon, label, value, detail, tone }: { icon: typeof Bell; label: string; value: string; detail: string; tone: 'success' | 'warning' | 'danger' | 'info' }) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-2xl border border-border bg-white/[0.03] p-4">
      <div className="flex items-start gap-3">
        <div className={cn('mt-0.5 flex size-10 items-center justify-center rounded-2xl border', tone === 'success' ? 'border-success/20 bg-success/10 text-success' : tone === 'warning' ? 'border-warning/20 bg-warning/10 text-warning' : tone === 'danger' ? 'border-danger/20 bg-danger/10 text-danger' : 'border-info/20 bg-info/10 text-info')}>
          <Icon className="size-4" />
        </div>
        <div>
          <div className="text-sm font-semibold text-foreground">{label}</div>
          <div className="text-sm text-muted-foreground">{detail}</div>
        </div>
      </div>
      <div className="text-lg font-black text-foreground">{value}</div>
    </div>
  )
}
