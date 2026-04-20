import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Layers3, Activity, Tag, ShieldCheck, Cpu, Plus, Wand2, ArrowRight } from 'lucide-react';
import { PaginationControls } from '../../components/list/PaginationControls';
import { DataTable } from '../../components/list/DataTable';
import { fetchConnectors, fetchPlatformDiscovery, createConnector, probeConnectorManifest, getApiErrorMessage } from '../../lib/api/ops';
import { useRegistry } from '../../hooks/registry/useRegistry';
import type { ConnectorManifest } from '../../lib/api/types';
import type { ColumnDef } from '@tanstack/react-table';
import { FilterBar } from '@/components/ui/filter-bar';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { StatusBadge } from '@/components/ui/status-badge';
import { InlineStatus } from '@/components/ui/inline-status';
import { useI18n } from '@/hooks/useI18n';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { GuidedFormDialog } from '@/components/operator/GuidedFormDialog';
import { ConnectorManifestEditor } from '@/components/operator/ConnectorManifestEditor';
import {
  connectorSamples,
  createConnectorDraftFromTemplate,
  createEmptyConnectorManifest,
  findConnectorSampleByID,
  normalizeConnectorManifest,
} from '@/lib/connector-samples';
import { useNotify } from '@/hooks/ui/useNotify';
import { connectorProbeResultFromLifecycle } from '@/lib/connectors/probe-status';

export const ConnectorsList = () => {
  const { t } = useI18n();
  const notify = useNotify();
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorSaving, setEditorSaving] = useState(false);
  const [draft, setDraft] = useState<ConnectorManifest>(createEmptyConnectorManifest());
  const [selectedTemplateID, setSelectedTemplateID] = useState<string | null>(null);
  const [probeResult, setProbeResult] = useState<{ status: 'idle' | 'success' | 'error'; summary?: string }>({ status: 'idle' });
  // Discovery info
  const { data: discovery } = useQuery({
    queryKey: ['platform-discovery'],
    queryFn: fetchPlatformDiscovery,
  });

  const probeMutation = useMutation({
    mutationFn: async () => probeConnectorManifest({
      manifest: normalizeConnectorManifest(draft),
    }),
    onSuccess: (result) => {
      const probeResult = connectorProbeResultFromLifecycle(
        result,
        t('connectors.status.testSuccess'),
        t('connectors.status.testFailed'),
      );
      setProbeResult(probeResult);
      if (probeResult.status === 'success') {
        notify.success(t('connectors.status.testPassed'));
        return;
      }
      notify.error(null, probeResult.summary || t('connectors.status.testFailed'));
    },
    onError: (err) => {
      const summary = getApiErrorMessage(err, t('connectors.status.testFailed'));
      setProbeResult({
        status: 'error',
        summary,
      });
      notify.error(err, t('connectors.status.testFailed'));
    },
  });

  const {
    items, total, page, limit, loading, error, query, filters,
    setPage, setLimit, setQuery, setFilters, refresh
  } = useRegistry<ConnectorManifest, { kind: string; enabled: string }>({
    key: 'connectors',
    fetcher: (params) => fetchConnectors({
      ...params,
      kind: params.filters?.kind,
      enabled: params.filters?.enabled === 'all' ? undefined : params.filters?.enabled === 'enabled',
    }),
    initialFilters: { kind: '', enabled: 'all' },
  });

  const columns = useMemo<ColumnDef<ConnectorManifest>[]>(() => [
    {
      accessorKey: "metadata.display_name",
      header: t('connectors.list.header.name'),
      cell: ({ row }) => (
        <div>
          <Link to={`/connectors/${row.original.metadata.id}`} className="font-bold text-text-primary hover:text-primary block text-base">
            {row.original.metadata.display_name || row.original.metadata.id}
          </Link>
          <div className="text-xs font-mono text-text-muted mt-1">{row.original.metadata.id} · v{row.original.metadata.version || '—'}</div>
        </div>
      ),
    },
    {
      accessorKey: "enabled",
      header: t('connectors.list.header.status'),
      cell: ({ row }) => (
        <StatusBadge status={(row.original.enabled ?? true) ? 'enabled' : 'disabled'} />
      ),
    },
    {
      id: "kind_type",
      header: t('connectors.list.header.type'),
      cell: ({ row }) => (
        <div className="text-text-secondary">
          <div className="flex items-center gap-1.5 font-medium"><Cpu size={12} className="opacity-50" /> {row.original.kind || '—'}</div>
          <div className="text-xs text-text-muted mt-1 font-mono uppercase tracking-tighter">{row.original.spec.protocol || '—'}</div>
        </div>
      ),
    },
    {
      id: "health",
      header: t('connectors.list.header.health'),
      cell: ({ row }) => (
        <div className="flex flex-col gap-1">
          <StatusBadge status={row.original.lifecycle?.health?.status || 'unknown'} />
          <div className="text-[0.7rem] text-text-muted line-clamp-1 max-w-[180px]">
            {row.original.lifecycle?.health?.summary || t('connectors.list.noReport')}
          </div>
        </div>
      ),
    },
    {
      id: "credentials",
      header: t('connectors.list.header.credentials'),
      cell: ({ row }) => {
        const credStatus = row.original.lifecycle?.health?.credential_status;
        if (credStatus === 'missing_credentials') return <StatusBadge status="danger" label={t('connectors.status.credMissing')} />;
        if (credStatus === 'configured') return <StatusBadge status="success" label={t('connectors.status.credConfigured')} />;
        return <StatusBadge status="info" label={t('connectors.status.credNone')} className="opacity-50" />;
      },
    },
    {
      id: "last_check",
      header: t('connectors.list.header.lastCheck'),
      cell: ({ row }) => {
        const checkedAt = row.original.lifecycle?.health?.checked_at;
        if (!checkedAt) return <span className="text-text-muted text-xs italic">{t('connectors.status.neverChecked')}</span>;
        return <span className="text-text-secondary text-xs font-mono">{new Date(checkedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>;
      },
    },
  ], [t]);

  const resetCreateFlow = (manifest: ConnectorManifest, templateID: string | null) => {
    setDraft(manifest);
    setSelectedTemplateID(templateID);
    setProbeResult({ status: 'idle' });
    setEditorOpen(true);
  };

  const openCreateDialog = () => {
    resetCreateFlow(createEmptyConnectorManifest(), null);
  };

  const openTemplateCreateDialog = (sample: ConnectorManifest) => {
    resetCreateFlow(createConnectorDraftFromTemplate(sample), sample.metadata.id || null);
  };

  const handleTemplateSelect = (templateID: string) => {
    const sample = findConnectorSampleByID(templateID);
    if (!sample) {
      return;
    }
    setSelectedTemplateID(templateID);
    setDraft((current) => createConnectorDraftFromTemplate(sample, current));
    setProbeResult({ status: 'idle' });
  };

  const handleCreate = async () => {
    try {
      setEditorSaving(true);
      await createConnector({ manifest: normalizeConnectorManifest(draft), operator_reason: 'Create connector from registry page' });
      notify.success(t('connectors.status.created'));
      setEditorOpen(false);
      await refresh();
    } catch (err) {
      notify.error(err, t('connectors.status.createFailed'));
    } finally {
      setEditorSaving(false);
    }
  };

  const handleDraftChange = (manifest: ConnectorManifest) => {
    setDraft(manifest);
    setProbeResult({ status: 'idle' });
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle
        title={t('connectors.hero.title')}
        subtitle={t('connectors.hero.description')}
      />

      {error ? <InlineStatus type="error" message={error} /> : null}

      <SummaryGrid>
        <StatCard title={t('connectors.stats.total')} value={String(total)} subtitle={t('connectors.stats.totalDesc')} icon={<Layers3 size={16} />} />
        <StatCard title={t('connectors.stats.discovered')} value={String(discovery?.registered_connectors_count ?? 0)} subtitle={discovery?.registered_connector_kinds?.join(', ') || t('connectors.stats.noKinds')} icon={<Activity size={16} />} />
        <StatCard title={t('connectors.stats.capabilities')} value={String(discovery?.tool_plan_capabilities?.length ?? 0)} subtitle={t('connectors.stats.capabilitiesDesc')} icon={<Tag size={16} />} />
        <StatCard title={t('connectors.stats.health')} value={total > 0 ? `${Math.round(((discovery?.registered_connectors_count || 0) / total) * 100)}%` : '—'} subtitle={t('connectors.stats.healthDesc')} icon={<ShieldCheck size={16} />} />
      </SummaryGrid>

      <div className="grid gap-4 xl:grid-cols-[1.2fr_1fr]">
        <Card className="glass-card p-5 border-white/10 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="space-y-1">
            <div className="text-[0.65rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('connectors.action.creationTitle')}</div>
            <p className="text-sm text-muted-foreground">
              {t('connectors.action.creationDesc')}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" onClick={openCreateDialog}><Plus size={16} />{t('connectors.action.add')}</Button>
            <Button variant="outline" asChild><Link to="/ops?tab=secrets#secret-inventory">{t('connectors.action.secrets')}</Link></Button>
          </div>
        </Card>

        <Card className="glass-card p-5 border-white/10">
          <div className="flex items-center justify-between gap-3 mb-4">
            <div>
              <div className="text-[0.65rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('connectors.action.commonTypes')}</div>
              <p className="mt-1 text-sm text-muted-foreground">{t('connectors.action.commonTypesDesc')}</p>
            </div>
            <Wand2 size={18} className="text-primary" />
          </div>
          <div className="grid gap-2">
            {connectorSamples.slice(0, 3).map((sample) => (
              <button
                key={sample.metadata.id}
                type="button"
                className="flex items-center justify-between rounded-xl border border-white/10 bg-white/5 px-3 py-3 text-left transition-colors hover:bg-white/10"
                onClick={() => openTemplateCreateDialog(sample)}
              >
                <div>
                  <div className="text-sm font-bold text-foreground">{sample.metadata.display_name || sample.metadata.id}</div>
                  <div className="text-xs text-muted-foreground font-mono uppercase tracking-tighter">{sample.spec.protocol} v{sample.metadata.version}</div>
                </div>
                <ArrowRight size={16} className="text-muted-foreground" />
              </button>
            ))}
          </div>
        </Card>
      </div>

      <FilterBar
        filters={[
          {
            key: 'kind',
            value: filters.kind,
            onChange: (v) => setFilters({ kind: v }),
            options: [
               { value: '', label: t('connectors.filter.allKinds') },
               { value: 'connector', label: t('connectors.filter.standard') },
            ],
          },
          {
            key: 'enabled',
            value: filters.enabled,
            onChange: (v) => setFilters({ enabled: v }),
            options: [
               { value: 'all', label: t('connectors.filter.allStates') },
               { value: 'enabled', label: t('connectors.filter.enabled') },
               { value: 'disabled', label: t('connectors.filter.disabled') },
            ],
          },
        ]}
        search={{ value: query, onChange: setQuery, placeholder: t('connectors.filter.searchPlaceholder') }}
      />

      <DataTable columns={columns} data={items} loading={loading} />

      {!loading && !error ? (
        <PaginationControls
          page={page}
          limit={limit}
          total={total}
          hasNext={items.length === limit}
          onPageChange={setPage}
          onLimitChange={setLimit}
        />
      ) : null}

      <GuidedFormDialog
        open={editorOpen}
        onOpenChange={setEditorOpen}
        title={t('connectors.action.creationTitle')}
        description={t('connectors.action.creationDesc')}
        wide
      >
        <ConnectorManifestEditor 
          key={`connector-create-${editorOpen ? 'open' : 'closed'}-${selectedTemplateID ?? 'blank'}`}
          manifest={draft} 
          onChange={handleDraftChange}
          disabled={editorSaving} 
          isEdit={false} 
          createTemplates={connectorSamples}
          selectedCreateTemplate={selectedTemplateID}
          onSelectCreateTemplate={handleTemplateSelect}
          onTest={() => probeMutation.mutate()}
          testing={probeMutation.isPending}
          testStatus={probeResult}
          onConfirm={() => void handleCreate()}
          onCancel={() => setEditorOpen(false)}
          confirmLabel={editorSaving ? t('common.saving') : t('connectors.editor.createAction')}
        />
      </GuidedFormDialog>
    </div>
  );
};
