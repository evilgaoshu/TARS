import { Activity, HardDrive, ScrollText, Waypoints, ExternalLink } from 'lucide-react';
import { useRequest } from 'ahooks';
import { fetchObservability, getApiErrorMessage } from '../../lib/api/ops';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { CollapsibleList } from '@/components/ui/collapsible-list';
import { EmptyState } from '@/components/ui/empty-state';
import { PanelCard } from '@/components/ui/panel-card';
import { SectionTitle, StatCard, SummaryGrid } from '@/components/ui/page-hero';
import { useI18n } from '@/hooks/useI18n';

export const ObservabilityPage = () => {
  const { t } = useI18n();
  const { data, loading, error, refresh } = useRequest(fetchObservability);
  const message = error ? getApiErrorMessage(error, 'Failed to load observability summary.') : '';

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <SectionTitle title={t('obs.title')} subtitle={t('obs.subtitle')} className="mb-0" />
        <Button variant="amber" type="button" onClick={() => refresh()}>
          <Activity size={14} /> {t('obs.refresh')}
        </Button>
      </div>

      {message ? <Card className="border-danger/35 text-danger">{message}</Card> : null}

      <SummaryGrid className="mb-0">
        <StatCard title={t('obs.stats.logs24h')} value={loading ? '...' : String(data?.summary.log_entries_24h ?? 0)} subtitle={t('obs.stats.errors24h', { count: data?.summary.error_entries_24h ?? 0 })} icon={<ScrollText size={16} />} />
        <StatCard title={t('obs.stats.events24h')} value={loading ? '...' : String(data?.summary.event_entries_24h ?? 0)} subtitle={t('obs.stats.activeTraces', { count: data?.summary.active_traces ?? 0 })} icon={<Waypoints size={16} />} />
        <StatCard title={t('obs.stats.logStorage')} value={formatBytes(data?.retention.logs.current_bytes ?? 0)} subtitle={t('obs.stats.retention', { period: formatHours(data?.retention.logs.retention_hours ?? 0) })} icon={<HardDrive size={16} />} />
        <StatCard title={t('obs.stats.metrics')} value={data?.metrics_endpoint || '/metrics'} subtitle={(data?.retention.exporters || []).join(', ') || t('obs.stats.prometheusReady')} icon={<ExternalLink size={16} />} />
      </SummaryGrid>

      <div className="grid grid-cols-1 xl:grid-cols-[1.2fr_0.8fr] gap-4">
        <PanelCard title={t('obs.panel.recentLogs')} subtitle={t('obs.panel.recentLogsDesc')} icon={<ScrollText size={16} />}>
          {data?.recent_logs?.length ? (
            <div className="grid gap-3">
              <CollapsibleList items={data.recent_logs.map((item) => (
                <div key={item.id} className="border border-white/10 rounded-xl p-3.5 bg-white/[0.03]">
                  <div className="flex justify-between gap-3 items-center">
                    <strong className="text-sm">{item.component || 'runtime'}</strong>
                    <span className={`text-xs uppercase font-bold ${levelClass(item.level)}`}>{item.level || 'info'}</span>
                  </div>
                  <div className="text-sm text-muted-foreground mt-2">{item.message}</div>
                  <div className="text-xs font-mono text-muted-foreground mt-2">{new Date(item.timestamp).toLocaleString()} · {item.trace_id || item.session_id || item.execution_id || 'no trace id'}</div>
                </div>
              ))} limit={4} />
            </div>
          ) : <EmptyState loading={loading} title={t('obs.panel.noRecentLogs')} />}
        </PanelCard>

        <PanelCard title={t('obs.panel.retention')} subtitle={t('obs.panel.retentionDesc')} icon={<HardDrive size={16} />}>
          <div className="grid gap-3 text-sm">
            <RetentionRow label="Metrics" retention={data?.retention.metrics.retention_hours} size={data?.retention.metrics.max_size_bytes} path={data?.retention.metrics.file_path} />
            <RetentionRow label="Logs" retention={data?.retention.logs.retention_hours} size={data?.retention.logs.max_size_bytes} path={data?.retention.logs.file_path} />
            <RetentionRow label="Traces" retention={data?.retention.traces.retention_hours} size={data?.retention.traces.max_size_bytes} path={data?.retention.traces.file_path} />
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
              <div className="text-xs uppercase tracking-widest text-muted-foreground font-bold">{t('obs.panel.otlp')}</div>
              <div className="mt-2 text-muted-foreground">{data?.retention.otlp.endpoint || 'disabled'} · {data?.retention.otlp.protocol || 'grpc'}</div>
              <div className="mt-1 text-xs text-muted-foreground">{(data?.retention.exporters || []).join(', ') || t('obs.panel.noOtlp')}</div>
            </div>
          </div>
        </PanelCard>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1fr_1fr] gap-4">
        <PanelCard title={t('obs.panel.traceSamples')} subtitle={t('obs.panel.traceSamplesDesc')} icon={<Waypoints size={16} />}>
          {data?.trace_samples?.length ? (
            <div className="grid gap-3">
              <CollapsibleList items={data.trace_samples.map((item) => (
                <div key={item.trace_id} className="border border-white/10 rounded-xl p-3.5 bg-white/[0.03]">
                  <div className="flex justify-between gap-3 items-center">
                    <strong className="font-mono text-sm truncate">{item.trace_id}</strong>
                    <span className="text-xs text-muted-foreground">{item.event_count} events</span>
                  </div>
                  <div className="text-sm text-muted-foreground mt-2">{item.last_message || 'No message'}</div>
                  <div className="text-xs text-muted-foreground mt-2">{(item.components || []).join(' · ') || item.component || 'unknown component'}</div>
                </div>
              ))} limit={4} />
            </div>
          ) : <EmptyState loading={loading} title={t('obs.panel.noTraceSamples')} />}
        </PanelCard>

        <PanelCard title={t('obs.panel.healthContext')} subtitle={t('obs.panel.healthContextDesc')} icon={<Activity size={16} />}>
          <div className="grid gap-3 text-sm">
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 flex justify-between"><span>{t('obs.health.tracingProvider')}</span><strong>{data?.health.resources.tracing_provider || 'disabled'}</strong></div>
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 flex justify-between"><span>{t('obs.health.healthyConnectors')}</span><strong>{data?.health.summary.healthy_connectors ?? 0}</strong></div>
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 flex justify-between"><span>{t('obs.health.providerFailures')}</span><strong>{data?.health.summary.provider_failures ?? 0}</strong></div>
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 flex justify-between"><span>{t('obs.health.spoolDir')}</span><strong className="font-mono text-xs">{data?.health.resources.spool_dir || 'n/a'}</strong></div>
          </div>
        </PanelCard>
      </div>
    </div>
  );
};

const RetentionRow = ({ label, retention, size, path }: { label: string; retention?: number; size?: number; path?: string }) => (
  <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
    <div className="flex justify-between gap-3 items-center">
      <strong>{label}</strong>
      <span className="text-xs text-muted-foreground">{formatHours(retention || 0)} / {formatBytes(size || 0)}</span>
    </div>
    <div className="text-xs font-mono text-muted-foreground mt-2">{path || 'in-memory / derived'}</div>
  </div>
);

function levelClass(level?: string): string {
  switch ((level || '').toLowerCase()) {
    case 'error': return 'text-danger';
    case 'warn': return 'text-warning';
    case 'debug': return 'text-info';
    default: return 'text-success';
  }
}

function formatBytes(value: number): string {
  if (!value) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(size >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatHours(hours: number): string {
  if (!hours) return '0h';
  if (hours % 24 === 0) return `${hours / 24}d`;
  return `${hours}h`;
}

export default ObservabilityPage;
