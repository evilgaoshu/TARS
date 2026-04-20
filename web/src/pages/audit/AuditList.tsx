import { useState, useCallback, useMemo, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { History } from 'lucide-react';
import { BulkActionsBar } from '../../components/list/BulkActionsBar';
import { PaginationControls } from '../../components/list/PaginationControls';
import { DataTable } from '../../components/list/DataTable';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import { FilterBar } from '@/components/ui/filter-bar';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { SectionTitle, StatCard, SummaryGrid } from '@/components/ui/page-hero';
import { useI18n } from '@/hooks/useI18n';
import { 
  bulkExportAudit, 
  fetchAuditRecords, 
  getApiErrorMessage, 
  getBlobApiErrorMessage, 
  parseBulkExportBlob 
} from '../../lib/api/ops';
import type { AuditExportResponse, AuditListResponse, AuditTraceEntry } from '../../lib/api/types';
import type { ColumnDef } from '@tanstack/react-table';

const getSessionLink = (item: AuditTraceEntry): string | null => {
  const sessionID = typeof item.metadata?.session_id === 'string' ? item.metadata.session_id : '';
  if (sessionID) {
    return `/sessions/${sessionID}`;
  }
  return item.resource_type === 'session' ? `/sessions/${item.resource_id}` : null;
};

export const AuditList = () => {
  const { t } = useI18n();
  const [items, setItems] = useState<AuditTraceEntry[]>([]);
  const [pageMeta, setPageMeta] = useState<Pick<AuditListResponse, 'page' | 'limit' | 'total' | 'has_next'>>({
    page: 1, limit: 20, total: 0, has_next: false,
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [resourceType, setResourceType] = useState('');
  const [sortBy, setSortBy] = useState('created_at');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [actionMessage, setActionMessage] = useState('');
  const [actionError, setActionError] = useState('');

  const load = useCallback(async () => {
    try {
      setLoading(true);
      setError('');
      const response = await fetchAuditRecords({
        resource_type: resourceType || undefined,
        q: query || undefined,
        page, limit, sort_by: sortBy, sort_order: sortOrder,
      });
      setItems(response.items);
      setPageMeta({
        page: response.page,
        limit: response.limit,
        total: response.total,
        has_next: response.has_next,
      });
      setSelectedIDs([]);
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, 'Failed to load audit records.'));
    } finally {
      setLoading(false);
    }
  }, [resourceType, query, page, limit, sortBy, sortOrder]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleBulkExport = async () => {
    try {
      setActionError('');
      setActionMessage('');
      const download = await bulkExportAudit(selectedIDs, 'Export audit records');
      const payload = await parseBulkExportBlob<AuditExportResponse>(download.content);
      const url = window.URL.createObjectURL(download.content);
      const link = document.createElement('a');
      link.href = url;
      link.download = download.filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      setActionMessage(`Exported ${payload.exported_count} record(s).`);
      setSelectedIDs([]);
    } catch (exportError) {
      setActionError(await getBlobApiErrorMessage(exportError, 'Export failed.'));
    }
  };

  const columns = useMemo<ColumnDef<AuditTraceEntry>[]>(() => [
    {
      id: "select",
      header: () => (
        <div className="px-1">
          <Checkbox
            checked={items.length > 0 && items.every((item) => item.id && selectedIDs.includes(item.id))}
            onCheckedChange={(value) => {
              if (value) {
                setSelectedIDs(items.map(i => i.id).filter((v): v is string => !!v));
              } else {
                setSelectedIDs([]);
              }
            }}
            aria-label="Select all"
          />
        </div>
      ),
      cell: ({ row }) => (
        <div className="px-1">
          <Checkbox
            checked={row.original.id ? selectedIDs.includes(row.original.id) : false}
            onCheckedChange={() => {
              const id = row.original.id;
              if (!id) return;
              setSelectedIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
            }}
            aria-label="Select row"
          />
        </div>
      ),
    },
    {
      accessorKey: "created_at",
      header: () => <span className="uppercase tracking-widest text-[0.65rem] font-black opacity-50">{t('audit.col.created')}</span>,
      cell: ({ row }) => <span className="text-muted-foreground whitespace-nowrap text-xs font-medium">{new Date(row.original.created_at).toLocaleString()}</span>,
    },
    {
      accessorKey: "resource_type",
      header: () => <span className="uppercase tracking-widest text-[0.65rem] font-black opacity-50">{t('audit.col.resource')}</span>,
      cell: ({ row }) => (
        <div className="space-y-0.5">
          <div className="font-bold text-foreground text-xs uppercase tracking-tighter">{row.original.resource_type}</div>
          <div className="text-[10px] font-mono text-muted-foreground opacity-60 truncate max-w-[120px]">{row.original.resource_id}</div>
        </div>
      ),
    },
    {
      accessorKey: "action",
      header: () => <span className="uppercase tracking-widest text-[0.65rem] font-black opacity-50">{t('audit.col.action')}</span>,
      cell: ({ row }) => <Badge variant="outline" className="font-bold text-[10px] uppercase tracking-tighter bg-white/5 border-white/10">{row.original.action}</Badge>,
    },
    {
      accessorKey: "actor",
      header: () => <span className="uppercase tracking-widest text-[0.65rem] font-black opacity-50">{t('audit.col.actor')}</span>,
      cell: ({ row }) => <span className="text-muted-foreground text-xs font-semibold">{row.original.actor || 'system'}</span>,
    },
    {
      id: "session",
      header: () => <div className="text-right uppercase tracking-widest text-[0.65rem] font-black opacity-50">{t('audit.col.session')}</div>,
      cell: ({ row }) => {
        const sessionLink = getSessionLink(row.original);
        return (
          <div className="text-right">
            {sessionLink ? (
              <Link to={sessionLink} className="text-primary hover:underline font-black text-[10px] uppercase tracking-widest">
                {t('audit.col.viewSession')}
              </Link>
            ) : <span className="text-muted-foreground opacity-20">—</span>}
          </div>
        );
      },
    },
  ], [items, selectedIDs, t]);

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle
        title={t('audit.title')}
        subtitle={t('audit.subtitle')}
      />

      {error ? <StatusMessage type="error" message={error} /> : null}

      <FilterBar
        search={{ value: query, onChange: (val) => { setQuery(val); setPage(1); }, placeholder: t('audit.search') }}
        filters={[
          {
            key: 'resourceType',
            value: resourceType,
            onChange: (value) => { setResourceType(value); setPage(1); },
            options: [
               { value: '', label: t('audit.filter.allResources') },
               { value: 'session', label: t('audit.filter.session') },
               { value: 'execution', label: t('audit.filter.execution') },
               { value: 'telegram_chat', label: t('audit.filter.telegramChat') },
               { value: 'telegram_message', label: t('audit.filter.telegramMessage') },
               { value: 'outbox_event', label: t('audit.filter.outbox') },
               { value: 'knowledge_record', label: t('audit.filter.knowledge') },
            ],
          },
          {
            key: 'sortBy',
            value: sortBy,
            onChange: (value) => { setSortBy(value); setPage(1); },
            options: [
               { value: 'created_at', label: t('audit.sort.created') },
               { value: 'resource_type', label: t('audit.sort.resource') },
               { value: 'action', label: t('audit.sort.action') },
               { value: 'actor', label: t('audit.sort.actor') },
            ],
          },
          {
            key: 'sortOrder',
            value: sortOrder,
            onChange: (value) => { setSortOrder(value as 'asc' | 'desc'); setPage(1); },
            options: [
               { value: 'desc', label: t('audit.sort.newest') },
               { value: 'asc', label: t('audit.sort.oldest') },
            ],
            className: 'md:w-32',
          },
        ]}
      />

      <BulkActionsBar
        selectedCount={selectedIDs.length}
        onClear={() => setSelectedIDs([])}
        actions={[{ key: 'export', label: t('audit.export'), onClick: () => { void handleBulkExport(); } }]}
      />

      <div className="flex flex-col gap-6">
        <SummaryGrid>
          <StatCard title={t('audit.stats.total')} value={String(pageMeta.total)} subtitle={t('audit.stats.totalDesc')} icon={<History size={16} />} />
          <StatCard title={t('audit.stats.selected')} value={String(selectedIDs.length)} subtitle={t('audit.stats.selectedDesc')} />
          <StatCard title={t('audit.stats.resource')} value={resourceType || t('audit.stats.resourceAll')} subtitle={t('audit.stats.resourceDesc')} />
        </SummaryGrid>

        {actionError && <StatusMessage type="error" message={actionError} />}
        {actionMessage && <StatusMessage type="success" message={actionMessage} />}

        <DataTable columns={columns} data={items} loading={loading} />
      </div>
      {!loading && !error ? (
        <PaginationControls
          page={pageMeta.page}
          limit={pageMeta.limit}
          total={pageMeta.total}
          hasNext={pageMeta.has_next}
          onPageChange={setPage}
          onLimitChange={(nextLimit) => { setLimit(nextLimit); setPage(1); }}
        />
      ) : null}
    </div>
  );
};
