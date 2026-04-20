import { useEffect, useState, useCallback } from 'react';
import { ScrollText, AlertTriangle, Search } from 'lucide-react';
import { RegistryLayout } from '../../components/layout/patterns/RegistryLayout';
import { PaginationControls } from '../../components/list/PaginationControls';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { fetchLogs, getApiErrorMessage } from '../../lib/api/ops';
import type { LogListResponse, LogRecord } from '../../lib/api/types';
import { useI18n } from '@/hooks/useI18n';

export const LogsPage = () => {
  const { t } = useI18n();
  const [items, setItems] = useState<LogRecord[]>([]);
  const [pageMeta, setPageMeta] = useState<Pick<LogListResponse, 'page' | 'limit' | 'total' | 'has_next'>>({
    page: 1, limit: 20, total: 0, has_next: false,
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [level, setLevel] = useState('');
  const [component, setComponent] = useState('');
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);

  const load = useCallback(async () => {
    try {
      setLoading(true);
      setError('');
      const response = await fetchLogs({
        q: query || undefined,
        level: level || undefined,
        component: component || undefined,
        page,
        limit,
      });
      setItems(response.items || []);
      setPageMeta({
        page: response.page,
        limit: response.limit,
        total: response.total,
        has_next: response.has_next,
      });
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, 'Failed to load runtime logs.'));
    } finally {
      setLoading(false);
    }
  }, [component, level, limit, page, query]);

  useEffect(() => {
    void load();
  }, [load]);

  const errorCount = items.filter((item) => (item.level || '').toLowerCase() === 'error').length;
  const uniqueComponents = new Set(items.map((item) => item.component).filter(Boolean)).size;

  return (
    <RegistryLayout
      title={t('logs.title')}
      description={t('logs.subtitle')}
      icon={<ScrollText size={24} />}
      searchQuery={query}
      onSearchChange={(value) => { setQuery(value); setPage(1); }}
      searchPlaceholder={t('logs.search')}
      loading={loading}
      onRefresh={load}
      error={error}
      toolbarActions={
        <div className="flex gap-2">
          <NativeSelect className="h-10 w-36 bg-background" value={level} onChange={(event) => { setLevel(event.target.value); setPage(1); }}>
            <option value="">{t('logs.filter.allLevels')}</option>
            <option value="debug">Debug</option>
            <option value="info">Info</option>
            <option value="warn">Warn</option>
            <option value="error">Error</option>
          </NativeSelect>
          <Input
            className="h-10 w-44"
            value={component}
            onChange={(event) => { setComponent(event.target.value); setPage(1); }}
            placeholder={t('logs.filter.componentPlaceholder')}
          />
        </div>
      }
      footer={
        !loading && !error && (
          <PaginationControls
            page={pageMeta.page}
            limit={pageMeta.limit}
            total={pageMeta.total}
            hasNext={pageMeta.has_next}
            onPageChange={setPage}
            onLimitChange={(nextLimit) => { setLimit(nextLimit); setPage(1); }}
          />
        )
      }
    >
      <div className="flex flex-col gap-6">
        <SummaryGrid>
          <StatCard title={t('logs.stats.visible')} value={String(pageMeta.total)} subtitle={t('logs.stats.visibleDesc')} icon={<Search size={16} />} />
          <StatCard title={t('logs.stats.errors')} value={String(errorCount)} subtitle={t('logs.stats.errorsDesc')} icon={<AlertTriangle size={16} />} />
          <StatCard title={t('logs.stats.components')} value={String(uniqueComponents)} subtitle={t('logs.stats.componentsDesc')} icon={<ScrollText size={16} />} />
        </SummaryGrid>

        {error && <StatusMessage type="error" message={error} />}

        <div className="glass-card p-0 overflow-hidden">
          <div className="registry-table-wrap border-0 bg-transparent rounded-none">
            <table className="registry-table">
              <thead>
                <tr>
                  <th>{t('logs.col.timestamp')}</th>
                  <th>{t('logs.col.level')}</th>
                  <th>{t('logs.col.component')}</th>
                  <th>{t('logs.col.message')}</th>
                  <th>{t('logs.col.trace')}</th>
                </tr>
              </thead>
              <tbody>
                {items.length === 0 && !loading ? (
                  <tr><td colSpan={5} className="py-20 text-center text-muted-foreground italic">{t('logs.empty')}</td></tr>
                ) : items.map((item) => (
                  <tr key={item.id} className="hover:bg-white/[0.02] transition-colors">
                    <td className="text-muted-foreground whitespace-nowrap">{new Date(item.timestamp).toLocaleString()}</td>
                    <td><span className={`uppercase text-xs font-bold ${levelClass(item.level)}`}>{item.level || 'info'}</span></td>
                    <td>
                      <div className="font-bold text-foreground">{item.component || 'runtime'}</div>
                      <div className="text-xs font-mono text-muted-foreground">{item.route || item.actor || '—'}</div>
                    </td>
                    <td>
                      <div className="font-medium text-foreground">{item.message}</div>
                      <div className="text-xs text-muted-foreground font-mono">
                        {item.session_id || item.execution_id || item.trace_id || t('logs.noCorrelation')}
                      </div>
                    </td>
                    <td className="text-right text-xs font-mono text-muted-foreground">{item.trace_id || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </RegistryLayout>
  );
};

function levelClass(level?: string): string {
  switch ((level || '').toLowerCase()) {
    case 'error':
      return 'text-danger';
    case 'warn':
      return 'text-warning';
    case 'debug':
      return 'text-info';
    default:
      return 'text-success';
  }
}

export default LogsPage;
