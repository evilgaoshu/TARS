import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { BulkActionsBar } from '../../components/list/BulkActionsBar';
import { PaginationControls } from '../../components/list/PaginationControls';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { fetchKnowledgeRecords, getApiErrorMessage } from '../../lib/api/ops';
import type { KnowledgeListResponse, KnowledgeRecord } from '../../lib/api/types';
import { Card } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useI18n } from '@/hooks/useI18n';

/** KnowledgeList - Converged UI/UX - CacheBust: ID_7788 */
export const KnowledgeList = () => {
  const { t } = useI18n();
  const [items, setItems] = useState<KnowledgeRecord[]>([]);
  const [pageMeta, setPageMeta] = useState<Pick<KnowledgeListResponse, 'page' | 'limit' | 'total' | 'has_next'>>({
    page: 1, limit: 20, total: 0, has_next: false
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [sortBy, setSortBy] = useState('updated_at');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');
  const [page, setPage] = useState(1);
  const [limit] = useState(20);
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);


  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        setLoading(true);
        const res = await fetchKnowledgeRecords({ q: query, page, limit, sort_by: sortBy, sort_order: sortOrder });
        if (active) {
          setItems(res.items);
          setPageMeta({ page: res.page, limit: res.limit, total: res.total, has_next: res.has_next });
        }
      } catch (err) {
        if (active) setError(getApiErrorMessage(err, t('knowledge.error.load')));
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => { active = false; };
  }, [query, page, limit, sortBy, sortOrder, t]);

  return (
    <div className="animate-fade-in flex flex-col gap-6 text-foreground">
      <SectionTitle title={t('knowledge.title')} subtitle={t('knowledge.subtitle')} />

      <SummaryGrid>
        <StatCard title={t('knowledge.stats.total')} value={String(pageMeta.total)} subtitle={t('knowledge.stats.totalDesc')} />
        <StatCard title={t('knowledge.stats.visible')} value={String(items.length)} subtitle={t('knowledge.stats.visibleDesc')} />
        <StatCard title={t('knowledge.stats.selected')} value={String(selectedIDs.length)} subtitle={t('knowledge.stats.selectedDesc')} />
        <StatCard title={t('knowledge.stats.sort')} value={sortBy} subtitle={sortOrder === 'desc' ? t('knowledge.sort.newest') : t('knowledge.sort.oldest')} />
      </SummaryGrid>

      <BulkActionsBar
        selectedCount={selectedIDs.length}
        onClear={() => setSelectedIDs([])}
        actions={[{ key: 'export', label: t('knowledge.export.label'), onClick: () => {} }]}
      />

      <Card className="flex flex-col gap-5 p-6 border-white/5 bg-white/[0.02]">
        <div className="flex flex-col gap-3 md:flex-row md:flex-wrap md:items-center">
          <Input 
            placeholder={t('knowledge.search')} 
            value={query} 
            onChange={e => { setQuery(e.target.value); setPage(1); }} 
            className="md:max-w-sm" 
          />
          <NativeSelect value={sortBy} onChange={e => setSortBy(e.target.value)} className="md:w-48">
             <option value="updated_at">{t('knowledge.sort.updated')}</option>
             <option value="title">{t('knowledge.sort.title')}</option>
          </NativeSelect>
          <NativeSelect value={sortOrder} onChange={e => setSortOrder(e.target.value as 'asc' | 'desc')} className="md:w-40">
             <option value="desc">{t('knowledge.sort.newest')}</option>
             <option value="asc">{t('knowledge.sort.oldest')}</option>
          </NativeSelect>
        </div>
        
        {loading ? (
          <div className="p-12 text-center text-muted-foreground">{t('knowledge.loading')}</div>
        ) : error ? (
          <StatusMessage type="error" message={error} />
        ) : items.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-border px-6 py-12 text-center text-muted-foreground">{t('knowledge.empty')}</div>
        ) : (
          <Card className="overflow-hidden p-0 border-white/5 bg-transparent">
             <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="w-12">
                      <Checkbox checked={items.length > 0 && items.every(i => selectedIDs.includes(i.document_id))} onCheckedChange={() => {}} />
                    </TableHead>
                    <TableHead>{t('common.updated')}</TableHead>
                    <TableHead>{t('common.name')}</TableHead>
                    <TableHead>Session</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((item) => (
                    <TableRow key={item.document_id}>
                      <TableCell><Checkbox checked={selectedIDs.includes(item.document_id)} onCheckedChange={() => {}} /></TableCell>
                      <TableCell className="text-sm text-muted-foreground">{item.updated_at ? new Date(item.updated_at).toLocaleString() : '—'}</TableCell>
                      <TableCell><div className="font-semibold">{item.title}</div><div className="font-mono text-xs opacity-50">{item.document_id}</div></TableCell>
                      <TableCell><Link to={`/sessions/${item.session_id}`} className="text-primary hover:underline font-mono text-xs">{item.session_id}</Link></TableCell>
                    </TableRow>
                  ))}
                </TableBody>
             </Table>
          </Card>
        )}

        {!loading && !error && (
          <PaginationControls page={pageMeta.page} limit={pageMeta.limit} total={pageMeta.total} hasNext={pageMeta.has_next} onPageChange={setPage} onLimitChange={() => {}} />
        )}
      </Card>
    </div>
  );
};
