import { useState, useEffect } from 'react';
import { Copy } from 'lucide-react';
import { bulkDeleteOutbox, bulkReplayOutbox, deleteOutbox, fetchOutbox, getApiErrorMessage, replayOutbox } from '../../lib/api/ops';
import { BulkActionsBar } from '../../components/list/BulkActionsBar';
import { PaginationControls } from '../../components/list/PaginationControls';
import { ConfirmActionDialog } from '../../components/operator/ConfirmActionDialog';
import type { OutboxEvent, OutboxListResponse } from '../../lib/api/types';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { SectionTitle } from '@/components/ui/page-hero';
import { StatusBadge } from '@/components/ui/status-badge';
import { InlineStatus } from '@/components/ui/inline-status';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Checkbox } from '@/components/ui/checkbox';
import { formatDistanceToNow } from 'date-fns';
import { useI18n } from '@/hooks/useI18n';
import { useNotify } from '@/hooks/ui/useNotify';

type DialogState =
  | { type: 'replay'; eventId: string }
  | { type: 'delete'; eventId: string }
  | { type: 'bulkReplay'; ids: string[] }
  | { type: 'bulkDelete'; ids: string[] }
  | null;

export const OutboxConsole = () => {
  const { t } = useI18n();
  const notify = useNotify();
  const [events, setEvents] = useState<OutboxEvent[]>([]);
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [pageMeta, setPageMeta] = useState<Pick<OutboxListResponse, 'page' | 'limit' | 'total' | 'has_next'>>({
    page: 1,
    limit: 20,
    total: 0,
    has_next: false,
  });
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [replayingId, setReplayingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [sortBy, setSortBy] = useState('created_at');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc');
  const [dialogState, setDialogState] = useState<DialogState>(null);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        setLoading(true);
        setError('');
        const response = await fetchOutbox({
          status: statusFilter || undefined,
          q: query || undefined,
          page,
          limit,
          sort_by: sortBy,
          sort_order: sortOrder,
        });
        if (!active) {
          return;
        }
        setEvents(response.items);
        setPageMeta({
          page: response.page,
          limit: response.limit,
          total: response.total,
          has_next: response.has_next,
        });
        setSelectedIDs((prev) => prev.filter((id) => response.items.some((item) => item.id === id)));
      } catch (loadError) {
        if (!active) {
          return;
        }
        setError(getApiErrorMessage(loadError, t('outbox.error.replay')));
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    };

    void load();
    return () => {
      active = false;
    };
  }, [statusFilter, query, page, limit, sortBy, sortOrder]);

  const handleConfirmAction = async () => {
    if (!dialogState) return;
    setActionLoading(true);
    try {
      if (dialogState.type === 'replay') {
        setReplayingId(dialogState.eventId);
        await replayOutbox(dialogState.eventId, 'Manual intervention');
        setEvents((prev) => prev.filter((event) => event.id !== dialogState.eventId));
        notify.success(t('outbox.alert.replayOk'), 'Replayed');
        setPage(1);
      } else if (dialogState.type === 'delete') {
        setDeletingId(dialogState.eventId);
        await deleteOutbox(dialogState.eventId, 'Clean up historical outbox residue');
        setEvents((prev) => prev.filter((event) => event.id !== dialogState.eventId));
        notify.success(t('outbox.alert.deleteOk'), 'Deleted');
      } else if (dialogState.type === 'bulkReplay') {
        setReplayingId('bulk');
        const result = await bulkReplayOutbox(dialogState.ids, 'Manual intervention');
        notify.success(t('outbox.alert.bulkReplayOk', { succeeded: result.succeeded, failed: result.failed }), 'Bulk replayed');
        setSelectedIDs([]);
        setPage(1);
      } else if (dialogState.type === 'bulkDelete') {
        setDeletingId('bulk');
        const result = await bulkDeleteOutbox(dialogState.ids, 'Clean up historical outbox residue');
        notify.success(t('outbox.alert.bulkDeleteOk', { succeeded: result.succeeded, failed: result.failed }), 'Bulk deleted');
        setSelectedIDs([]);
        setPage(1);
      }
    } catch (requestError) {
      const msg = dialogState.type === 'delete' || dialogState.type === 'bulkDelete'
        ? getApiErrorMessage(requestError, dialogState.type === 'bulkDelete' ? t('outbox.error.bulkDelete') : t('outbox.error.delete'))
        : getApiErrorMessage(requestError, dialogState.type === 'bulkReplay' ? t('outbox.error.bulkReplay') : t('outbox.error.replay'));
      notify.error(null, msg);
    } finally {
      setActionLoading(false);
      setReplayingId(null);
      setDeletingId(null);
      setDialogState(null);
    }
  };

  const toggleSelected = (eventID: string) => {
    setSelectedIDs((prev) => (prev.includes(eventID) ? prev.filter((id) => id !== eventID) : [...prev, eventID]));
  };

  const allVisibleSelected = events.length > 0 && events.every((event) => selectedIDs.includes(event.id));

  const handleSelectVisible = () => {
    if (allVisibleSelected) {
      setSelectedIDs((prev) => prev.filter((id) => !events.some((event) => event.id === id)));
      return;
    }
    setSelectedIDs((prev) => {
      const next = new Set(prev);
      events.forEach((event) => next.add(event.id));
      return Array.from(next);
    });
  };

  const dialogTitle = () => {
    if (!dialogState) return '';
    if (dialogState.type === 'replay') return 'Replay event';
    if (dialogState.type === 'delete') return 'Delete event';
    if (dialogState.type === 'bulkReplay') return `Replay ${dialogState.ids.length} events`;
    if (dialogState.type === 'bulkDelete') return `Delete ${dialogState.ids.length} events`;
    return '';
  };

  const dialogDescription = () => {
    if (!dialogState) return '';
    if (dialogState.type === 'replay') return t('outbox.confirm.replay');
    if (dialogState.type === 'delete') return t('outbox.confirm.delete');
    if (dialogState.type === 'bulkReplay') return t('outbox.confirm.bulkReplay', { count: dialogState.ids.length });
    if (dialogState.type === 'bulkDelete') return t('outbox.confirm.bulkDelete', { count: dialogState.ids.length });
    return '';
  };

  const isDanger = dialogState?.type === 'delete' || dialogState?.type === 'bulkDelete';

  const copyToClipboard = (text: string) => {
    void navigator.clipboard.writeText(text);
    notify.success('Copied to clipboard', 'Copied');
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle
        title={t('outbox.title')}
        subtitle={t('outbox.subtitle')}
      />

      {error ? <InlineStatus type="error" message={error} /> : null}

      <Card className="flex flex-col gap-5 p-5 md:p-6">
        <BulkActionsBar
          selectedCount={selectedIDs.length}
          onClear={() => setSelectedIDs([])}
          actions={[
            {
              key: 'replay',
              label: replayingId === 'bulk' ? t('outbox.bulk.replaying') : t('outbox.bulk.replay'),
              disabled: replayingId !== null || deletingId !== null,
              onClick: () => selectedIDs.length > 0 && setDialogState({ type: 'bulkReplay', ids: [...selectedIDs] }),
            },
            {
              key: 'delete',
              label: deletingId === 'bulk' ? t('outbox.bulk.deleting') : t('outbox.bulk.delete'),
              tone: 'danger',
              disabled: deletingId !== null || replayingId !== null,
              onClick: () => selectedIDs.length > 0 && setDialogState({ type: 'bulkDelete', ids: [...selectedIDs] }),
            },
          ]}
        />

        <div className="flex flex-col gap-3 md:flex-row md:flex-wrap md:items-center">
          <Input
            type="text"
            placeholder={t('outbox.search')}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
            className="h-10 w-full md:max-w-sm"
            aria-label={t('outbox.aria.search')}
          />
          <NativeSelect
            value={statusFilter}
            onChange={(event) => {
              setStatusFilter(event.target.value);
              setPage(1);
            }}
            className="h-10 w-full md:w-44 bg-background"
            aria-label={t('outbox.aria.filterStatus')}
          >
            <option value="">{t('outbox.filter.failedBlocked')}</option>
            <option value="failed">{t('outbox.filter.failed')}</option>
            <option value="blocked">{t('outbox.filter.blocked')}</option>
          </NativeSelect>
          <NativeSelect
            value={sortBy}
            onChange={(event) => {
              setSortBy(event.target.value);
              setPage(1);
            }}
            className="h-10 w-full md:w-44 bg-background"
            aria-label={t('outbox.aria.sortField')}
          >
            <option value="created_at">{t('outbox.sort.created')}</option>
            <option value="status">{t('outbox.sort.status')}</option>
            <option value="topic">{t('outbox.sort.topic')}</option>
          </NativeSelect>
          <NativeSelect
            value={sortOrder}
            onChange={(event) => {
              setSortOrder(event.target.value as 'asc' | 'desc');
              setPage(1);
            }}
            className="h-10 w-full md:w-40 bg-background"
            aria-label={t('outbox.aria.sortOrder')}
          >
            <option value="desc">{t('outbox.sort.newest')}</option>
            <option value="asc">{t('outbox.sort.oldest')}</option>
          </NativeSelect>
        </div>

        {loading ? (
          <div className="rounded-2xl border border-dashed border-border bg-white/[0.02] px-6 py-16 text-center text-sm text-muted-foreground">
            {t('outbox.loading')}
          </div>
        ) : events.length === 0 ? (
          <div className="rounded-2xl border border-success/20 bg-success/5 px-6 py-16 text-center">
            <div className="text-3xl font-black text-success">{t('outbox.empty.ok')}</div>
            <p className="mt-3 text-sm text-muted-foreground">{t('outbox.empty.desc')}</p>
          </div>
        ) : (
          <Card className="overflow-hidden p-0">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="w-12 text-center">
                    <Checkbox
                      checked={allVisibleSelected}
                      onCheckedChange={() => handleSelectVisible()}
                      aria-label={t('outbox.aria.selectAll')}
                    />
                  </TableHead>
                  <TableHead>{t('outbox.col.eventId')}</TableHead>
                  <TableHead>{t('outbox.col.topic')}</TableHead>
                  <TableHead>{t('outbox.col.status')}</TableHead>
                  <TableHead>{t('outbox.col.error')}</TableHead>
                  <TableHead>{t('outbox.col.age')}</TableHead>
                  <TableHead className="text-right">{t('outbox.col.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {events.map((evt) => (
                  <TableRow key={evt.id}>
                    <TableCell className="text-center">
                      <Checkbox
                        checked={selectedIDs.includes(evt.id)}
                        onCheckedChange={() => toggleSelected(evt.id)}
                        aria-label={t('outbox.aria.selectRow', { id: evt.id })}
                      />
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <span className="font-mono text-xs text-muted-foreground">{evt.id}</span>
                        <button
                          type="button"
                          className="rounded p-0.5 text-muted-foreground hover:text-foreground transition-colors"
                          onClick={() => copyToClipboard(evt.id)}
                          title="Copy event ID"
                        >
                          <Copy size={11} />
                        </button>
                      </div>
                    </TableCell>
                    <TableCell className="font-medium text-foreground">{evt.topic}</TableCell>
                    <TableCell>
                      <StatusBadge status={evt.status} />
                    </TableCell>
                    <TableCell className="max-w-[320px]">
                      <div className="flex flex-col gap-2">
                        {evt.blocked_reason ? (
                          <div className="text-sm text-warning">{t('outbox.locked', { reason: evt.blocked_reason })}</div>
                        ) : null}
                        {evt.last_error ? (
                          <div className="truncate text-sm text-danger" title={evt.last_error}>
                            {evt.last_error}
                          </div>
                        ) : (
                          <span className="text-sm text-muted-foreground">{t('outbox.noError')}</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {formatDistanceToNow(new Date(evt.created_at), { addSuffix: true })}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setDialogState({ type: 'replay', eventId: evt.id })}
                          disabled={replayingId !== null || deletingId !== null}
                        >
                          {replayingId === evt.id ? t('outbox.bulk.replaying') : t('outbox.replay')}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="border-danger/40 text-danger hover:bg-danger/10 hover:text-danger"
                          onClick={() => setDialogState({ type: 'delete', eventId: evt.id })}
                          disabled={deletingId !== null || replayingId !== null}
                        >
                          {deletingId === evt.id ? t('outbox.bulk.deleting') : t('outbox.delete')}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>
        )}

        {!loading && !error ? (
          <PaginationControls
            page={pageMeta.page}
            limit={pageMeta.limit}
            total={pageMeta.total}
            hasNext={pageMeta.has_next}
            onPageChange={setPage}
            onLimitChange={(nextLimit) => {
              setLimit(nextLimit);
              setPage(1);
            }}
          />
        ) : null}
      </Card>

      <ConfirmActionDialog
        open={dialogState !== null}
        onOpenChange={(open) => { if (!open) setDialogState(null); }}
        title={dialogTitle()}
        description={dialogDescription()}
        confirmLabel={isDanger ? 'Delete' : 'Replay'}
        loading={actionLoading}
        danger={isDanger}
        onConfirm={() => void handleConfirmAction()}
      />
    </div>
  );
};
