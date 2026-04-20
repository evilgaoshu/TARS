import React, { useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { 
  updateConnector,
  checkConnectorHealth, 
  probeConnectorManifest,
  executeConnectorCommand, 
  exportConnector,
  fetchConnector, 
  invokeConnectorCapability,
  queryConnectorMetrics, 
  applyConnectorTemplate,
  setConnectorEnabled,
  getApiErrorMessage,
} from '../../lib/api/ops';
import { 
  Activity, 
  ChevronRight, 
  Code, 
  FileJson, 
  History, 
  Play, 
  Power,
  RefreshCcw, 
  Terminal,
  Download,
  Wand2,
  Settings2,
  ChevronDown,
} from 'lucide-react';
import type { ConnectorExecutionResponse, ConnectorInvokeCapabilityResponse, ConnectorMetricsQueryResponse } from '../../lib/api/types';
import { clsx } from 'clsx';
import { SplitDetailLayout } from '../../components/layout/patterns/SplitDetailLayout';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useNotify } from '@/hooks/ui/useNotify';
import { useI18n } from '@/hooks/useI18n';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { GuidedFormDialog } from '@/components/operator/GuidedFormDialog';
import { ConnectorManifestEditor } from '@/components/operator/ConnectorManifestEditor';
import { normalizeConnectorManifest } from '@/lib/connector-samples';
import { connectorProbeResultFromLifecycle, type ConnectorProbeResult } from '@/lib/connectors/probe-status';

export const ConnectorDetail = () => {
  const { id } = useParams<{ id: string }>();
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const notify = useNotify();

  const [editorOpen, setEditorOpen] = useState(false);
  const [editorDraft, setEditorDraft] = useState<ConnectorDetailViewData | null>(null);
  const [editorProbeResult, setEditorProbeResult] = useState<ConnectorProbeResult>({ status: 'idle' });
  const [selectedTemplateID, setSelectedTemplateID] = useState<string>('');
  
  const [metricsForm, setMetricsForm] = useState({ service: '', host: '' });
  const [metricsResult, setMetricsResult] = useState<ConnectorMetricsQueryResponse | null>(null);
  const [executionForm, setExecutionForm] = useState({ targetHost: '', command: '' });
  const [executionResult, setExecutionResult] = useState<ConnectorExecutionResponse | null>(null);
  const [capabilityForm, setCapabilityForm] = useState({ capabilityID: '', paramsText: '' });
  const [invokeResult, setInvokeResult] = useState<ConnectorInvokeCapabilityResponse | null>(null);

  // Data fetching
  const { data: connector, isLoading, error } = useQuery({
    queryKey: ['connector', id],
    queryFn: () => fetchConnector(id!),
    enabled: !!id,
  });

  // Mutations
  const toggleMutation = useMutation({
    mutationFn: ({ enabled, reason }: { enabled: boolean; reason: string }) =>
      setConnectorEnabled(id!, enabled, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connector', id] });
      notify.success(t('connectors.status.updated', 'Connector state updated'));
    },
    onError: (err) => notify.error(err, t('connectors.status.toggleFailed', 'Failed to toggle connector')),
  });

  const healthMutation = useMutation({
    mutationFn: () => checkConnectorHealth(id!),
    onSuccess: (result) => {
      const probeResult = connectorProbeResultFromLifecycle(
        result,
        t('connectors.status.probeHealthSuccess', 'Health probe completed'),
        t('connectors.status.probeHealthFailed', 'Health check failed'),
      );
      queryClient.invalidateQueries({ queryKey: ['connector', id] });
      if (probeResult.status === 'success') {
        notify.success(t('connectors.status.probeHealthSuccess', 'Health probe completed'));
        return;
      }
      notify.error(null, probeResult.summary || t('connectors.status.probeHealthFailed', 'Health check failed'));
    },
    onError: (err) => notify.error(err, t('connectors.status.probeHealthFailed', 'Health check failed')),
  });

  const editorProbeMutation = useMutation({
    mutationFn: () => {
      if (!editorDraft) {
        throw new Error('connector draft is not loaded');
      }
      return probeConnectorManifest({
        manifest: normalizeConnectorManifest(editorDraft),
      });
    },
    onSuccess: (result) => {
      const probeResult = connectorProbeResultFromLifecycle(
        result,
        t('connectors.editor.testSuccess'),
        t('connectors.editor.testFailed'),
      );
      setEditorProbeResult(probeResult);
      if (probeResult.status === 'success') {
        notify.success(t('connectors.status.testPassed', 'Connection test passed'));
        return;
      }
      notify.error(null, probeResult.summary || t('connectors.editor.testFailed'));
    },
    onError: (err) => {
      const summary = getApiErrorMessage(err, t('connectors.editor.testFailed'));
      setEditorProbeResult({
        status: 'error',
        summary,
      });
      notify.error(err, t('connectors.editor.testFailed'));
    },
  });

  const metricsMutation = useMutation({
    mutationFn: () => queryConnectorMetrics(id!, { service: metricsForm.service, host: metricsForm.host }),
    onSuccess: (res) => setMetricsResult(res),
    onError: (err) => notify.error(err, t('connectors.status.metricsFailed', 'Metrics query failed')),
  });

  const executionMutation = useMutation({
    mutationFn: () => executeConnectorCommand(id!, {
      target_host: executionForm.targetHost,
      command: executionForm.command,
      operator_reason: t('connectors.status.manualVerify', 'Manual connector verification'),
    }),
    onSuccess: (res) => setExecutionResult(res),
    onError: (err) => notify.error(err, t('connectors.status.executionFailed', 'Execution failed')),
  });

  const exportMutation = useMutation({
    mutationFn: (format: 'yaml' | 'json') => exportConnector(id!, format),
    onSuccess: (_, format) => notify.success(t('connectors.status.exported', `Connector exported as ${format.toUpperCase()}`)),
    onError: (err) => notify.error(err, t('connectors.status.exportFailed', 'Export failed')),
  });

  const applyTemplateMutation = useMutation({
    mutationFn: (templateID: string) => applyConnectorTemplate(id!, {
      template_id: templateID,
      operator_reason: t('connectors.status.applyTemplateReason', `Apply template: ${templateID}`),
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connector', id] });
      notify.success(t('connectors.status.templateApplied', 'Template applied — connector config updated'));
    },
    onError: (err) => notify.error(err, t('connectors.status.templateApplyFailed', 'Template apply failed')),
  });

  const invokeMutation = useMutation({
    mutationFn: () => {
      const { value, error } = parseCapabilityParams(capabilityForm.paramsText);
      if (error) throw new Error(error);
      return invokeConnectorCapability(id!, {
        capability_id: capabilityForm.capabilityID,
        params: value,
      });
    },
    onSuccess: (res) => {
      setInvokeResult(res);
      notify.success(t('connectors.status.invokeSuccess', 'Capability invocation completed'));
    },
    onError: (err) => notify.error(err, t('connectors.status.invokeFailed', 'Capability invocation failed')),
  });

  const editMutation = useMutation({
    mutationFn: (manifest: ConnectorDetailViewData) => updateConnector(id!, {
      manifest,
      operator_reason: t('connectors.status.editReason', 'Edit connector from detail page'),
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connector', id] });
      notify.success(t('connectors.status.updated', 'Connector updated'));
      setEditorOpen(false);
    },
    onError: (err) => notify.error(err, t('connectors.status.updateFailed', 'Failed to update connector')),
  });

  const openEditor = () => {
    if (connector) {
      setEditorDraft(connector);
      setEditorProbeResult({ status: 'idle' });
      setEditorOpen(true);
    }
  };

  const isHealthy = connector?.lifecycle?.health?.status === 'healthy' || connector?.lifecycle?.health?.status === 'success';
  const status = connector?.lifecycle?.health?.status || 'unknown';
  const connectorEnabled = connector?.enabled ?? true;
  const metricsSupported = connector?.spec?.type === 'metrics';
  const executionSupported = connector?.spec?.type === 'execution';
  const invocableCapabilities = (connector?.spec?.capabilities || []).filter(cap => cap.invocable);
  
  const rawManifest = useMemo(() => JSON.stringify(connector, null, 2), [connector]);

  const missingSecrets = useMemo(() => {
    const required = connector?.spec?.connection_form?.filter(f => f.required && f.secret).map(f => f.key) || [];
    const configured = Object.keys(connector?.config?.secret_refs || {});
    return required.filter(r => !configured.includes(r || ''));
  }, [connector]);

  if (isLoading) {
    return (
      <div className="flex h-[400px] flex-col items-center justify-center gap-4 text-text-muted">
        <RefreshCcw size={32} className="animate-spin opacity-20" />
        <p className="animate-pulse">{t('common.loadingDetail', 'Loading connector details...')}</p>
      </div>
    );
  }

  if (error || !connector) {
    return (
      <div className="flex h-[400px] flex-col items-center justify-center gap-4">
        <div className="text-xl font-bold text-danger">{t('common.notFound', 'Connector not found')}</div>
        <Link to="/connectors"><Button variant="outline" className="mt-2 text-sm font-bold uppercase tracking-widest">{t('common.backToList', 'Back to Registry')}</Button></Link>
      </div>
    );
  }

  return (
    <SplitDetailLayout
      sidebarWidth="380px"
      sidebar={
        <div className="space-y-6">
          <div className="flex flex-col gap-1">
            <h2 className="text-lg font-bold m-0 flex items-center gap-2">
              <History size={18} className="text-text-muted" /> {t('connectors.detail.history.title')}
            </h2>
            <p className="text-[0.7rem] text-text-muted">{t('connectors.detail.history.description')}</p>
          </div>
          <div className="space-y-3">
            {connector.lifecycle?.health_history?.length ? (
              connector.lifecycle.health_history.slice().reverse().map((item, idx) => (
                <div key={idx} className="flex gap-3 p-3 rounded-xl bg-white/5 border border-white/5 hover:bg-white/10 transition-colors">
                  <div className={clsx("w-2 h-2 rounded-full mt-1 shrink-0", 
                    item.status === 'healthy' || item.status === 'success' ? "bg-success shadow-[0_0_8px_var(--success)]" : "bg-warning"
                  )} />
                  <div className="flex-1 min-w-0">
                    <div className="flex justify-between items-center">
                      <span className="text-xs font-bold uppercase">{item.status}</span>
                      <span className="text-[0.6rem] text-text-muted">{new Date(item.checked_at || '').toLocaleTimeString()}</span>
                    </div>
                    <p className="text-[0.7rem] text-text-muted italic mt-1 line-clamp-2">"{item.summary || t('connectors.detail.history.periodic')}"</p>
                  </div>
                </div>
              ))
            ) : (
                <div className="text-center py-8 text-text-muted text-xs italic">{t('connectors.detail.history.none')}</div>
            )}
          </div>

          <div className="pt-6 border-t border-white/5">
            <h3 className="text-xs font-bold uppercase tracking-widest text-text-muted mb-4 flex items-center gap-2"><Code size={14} /> {t('connectors.detail.revisions.title')}</h3>
            <div className="space-y-2">
              {connector.lifecycle?.revisions?.slice().reverse().map((rev, i) => (
                <div key={i} className="flex justify-between items-center p-2.5 rounded-lg bg-white/5 border border-white/5 text-xs">
                  <span className="font-mono font-bold">v{rev.version}</span>
                  <span className="text-text-muted">{new Date(rev.created_at || '').toLocaleDateString()}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="pt-6 border-t border-white/5">
            <h3 className="text-xs font-bold uppercase tracking-widest text-info mb-4 flex items-center gap-2"><Activity size={14} /> {t('connectors.detail.marketplace.title')}</h3>
            <div className="bg-info/5 border border-info/10 rounded-xl p-4 space-y-3">
              <div className="flex flex-col gap-1">
                <span className="text-[0.6rem] font-bold text-text-muted uppercase">{t('connectors.detail.marketplace.category')}</span>
                <span className="text-sm font-medium">{connector.marketplace.category || t('connectors.detail.marketplace.defaultCategory')}</span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {connector.marketplace.tags?.map(t => (
                  <span key={t} className="px-2 py-0.5 rounded bg-white/5 text-[0.6rem] text-text-muted border border-white/10">#{t}</span>
                ))}
              </div>
            </div>
          </div>
        </div>
      }
    >
      <div className="animate-fade-in space-y-6">
        {/* Header Strip */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 pb-6 border-b border-white/5">
          <div className="space-y-1.5">
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-2xl font-bold m-0">{connector.metadata.display_name || connector.metadata.name}</h1>
              <div className="flex gap-2">
                <Badge variant={isHealthy ? "success" : "warning"} className="uppercase tracking-wider text-[0.68rem] font-bold">{status.toUpperCase()}</Badge>
                  <Badge variant={connectorEnabled ? "default" : "muted"} className="uppercase tracking-wider text-[0.68rem] font-bold">
                   {connectorEnabled ? t('common.status.enabled') : t('common.status.disabled')}
                  </Badge>
              </div>
            </div>
          </div>

          <div className="flex gap-2.5">
            <Button variant="glass" onClick={openEditor}>
               <Settings2 size={16} /> {t('common.edit')}
            </Button>
            <Button variant="primary" onClick={() => healthMutation.mutate()} disabled={healthMutation.isPending || !connectorEnabled}>
               <RefreshCcw size={16} className={healthMutation.isPending ? 'animate-spin' : ''} /> {t('connectors.action.probeHealth')}
            </Button>
            <Button variant="secondary" onClick={() => toggleMutation.mutate({ enabled: !connectorEnabled, reason: 'Manual toggle' })} disabled={toggleMutation.isPending}>
               <Power size={16} /> {connectorEnabled ? t('common.disable') : t('common.enable')}
            </Button>
          </div>
        </div>

        {/* Diagnostic Posture */}
        <section className={clsx("glass-card p-6 border-l-4", isHealthy ? "border-l-success" : "border-l-warning")}>
          <div className="flex justify-between items-start mb-6">
            <div className="space-y-1">
              <span className="text-[0.65rem] font-bold text-text-muted uppercase tracking-widest">{t('connectors.detail.summaryTitle')}</span>
              <p className="text-lg font-medium m-0 leading-snug">{connector.lifecycle?.health?.summary || t('connectors.detail.noSummary')}</p>
            </div>
            <div className="text-right shrink-0 pl-6 border-l border-white/5 ml-6">
              <span className="text-[0.65rem] font-bold text-text-muted uppercase tracking-widest">{t('connectors.list.header.lastCheck')}</span>
              <div className="text-sm font-mono text-text-secondary">{connector.lifecycle?.health?.checked_at ? new Date(connector.lifecycle.health.checked_at).toLocaleTimeString() : t('connectors.status.neverChecked')}</div>
            </div>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-6 pt-6 border-t border-white/5">
            <StatItem label={t('connectors.detail.stats.runtime')} value={connector.lifecycle?.runtime.state || 'unknown'} />
            <StatItem label={t('connectors.detail.stats.compatibility')} value={connector.lifecycle?.compatibility.compatible ? t('common.status.compatible') : t('common.status.degraded')} success={connector.lifecycle?.compatibility.compatible} />
            <StatItem label={t('connectors.detail.stats.mode')} value={connector.lifecycle?.runtime.mode || 'managed'} />
            <StatItem label={t('connectors.detail.stats.tarsVer')} value={connector.compatibility.tars_major_versions?.[0] || 'Any'} />
            <StatItem label={t('connectors.detail.stats.tenant')} value={connector.metadata.tenant_id || connector.metadata.org_id || t('common.status.default')} />
            <StatItem label={t('connectors.detail.stats.lifecycle')} value={connectorEnabled ? t('common.status.ready') : t('common.status.halt')} success={connectorEnabled} />
          </div>
        </section>

        {/* JumpServer specific remediation */}
        {executionSupported && (
          <DetailSection title={t('connectors.detail.remediation.title', 'Access Remediation')} icon={<RefreshCcw size={18} className="text-warning" />} subtitle={t('connectors.detail.remediation.description', 'Fix connectivity issues for JumpServer.')}>
            <div className="flex flex-col md:flex-row md:items-center gap-6 p-4 rounded-2xl bg-warning/5 border border-warning/10">
              <div className="flex-1 space-y-1">
                <h3 className="m-0 text-lg font-bold">{t('connectors.detail.remediation.headline', 'JumpServer Access Remediation')}</h3>
                <p className="text-sm text-muted-foreground m-0">
                  {missingSecrets.length > 0 
                    ? t('connectors.detail.remediation.missing', { count: missingSecrets.length })
                    : isHealthy ? t('connectors.detail.remediation.healthy', 'All credentials verified and healthy.') : t('connectors.detail.remediation.failed', 'Secrets configured but probe failed. Verify JumpServer API permissions.')}
                </p>
              </div>
              <Button variant="outline" asChild>
                <Link to="/ops?tab=secrets#secret-inventory">
                  {t('connectors.action.secrets')} <ChevronRight size={16} />
                </Link>
              </Button>
            </div>
          </DetailSection>
        )}

        {/* Support for manual testing */}
        {(metricsSupported || executionSupported) && (
          <DetailSection title={t('connectors.detail.manualTest.title', 'Runtime Test')} icon={<Activity size={18} className="text-primary" />} subtitle={t('connectors.detail.manualTest.description', 'Verify runtime capabilities directly from the console.')}>
            {metricsSupported && (
              <div className="space-y-4">
                <div className="flex flex-wrap gap-3">
                  <Input className="max-w-[200px]" placeholder="service=api" value={metricsForm.service} onChange={e => setMetricsForm(f => ({...f, service: e.target.value}))} />
                  <Input className="max-w-[200px]" placeholder="host=127.0.0.1" value={metricsForm.host} onChange={e => setMetricsForm(f => ({...f, host: e.target.value}))} />
                  <Button variant="secondary" onClick={() => metricsMutation.mutate()} disabled={metricsMutation.isPending}><Activity size={14} /> {t('connectors.action.runQuery', 'Run Query')}</Button>
                </div>
                {metricsResult && <pre className="bg-black/40 p-4 rounded-xl text-[0.7rem] font-mono text-success overflow-auto max-h-[300px] border border-white/5">{typeof metricsResult === 'string' ? metricsResult : JSON.stringify(metricsResult, null, 2)}</pre>}
              </div>
            )}
            {executionSupported && (
              <div className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-[1fr_2fr_auto] gap-3">
                  <Input placeholder="Target Host" value={executionForm.targetHost} onChange={e => setExecutionForm(f => ({...f, targetHost: e.target.value}))} />
                  <Input placeholder="Shell Command" value={executionForm.command} onChange={e => setExecutionForm(f => ({...f, command: e.target.value}))} />
                  <Button variant="secondary" onClick={() => executionMutation.mutate()} disabled={executionMutation.isPending || missingSecrets.length > 0 || !connectorEnabled}>
                    <Terminal size={14} /> {t('connectors.action.execute', 'Execute')}
                  </Button>
                </div>
                {executionResult && <pre className="bg-black/40 p-4 rounded-xl text-[0.7rem] font-mono text-info overflow-auto border border-white/5">{typeof executionResult === 'string' ? executionResult : JSON.stringify(executionResult, null, 2)}</pre>}
              </div>
            )}
          </DetailSection>
        )}

        {invocableCapabilities.length > 0 && (
          <DetailSection title={t('connectors.detail.capabilities.title')} icon={<Terminal size={18} className="text-info" />} subtitle={t('connectors.detail.capabilities.description')}>
            <div className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-[1fr_2fr_auto] gap-3">
                <NativeSelect
                  value={capabilityForm.capabilityID}
                  onChange={(event) => setCapabilityForm((current) => ({ ...current, capabilityID: event.target.value }))}
                >
                  <option value="">{t('connectors.detail.capabilities.select')}</option>
                  {invocableCapabilities.map((capability) => (
                    <option key={capability.id || capability.action} value={capability.id || capability.action || ''}>
                      {capability.id || capability.action}
                    </option>
                  ))}
                </NativeSelect>
                <Input
                  value={capabilityForm.paramsText}
                  onChange={(event) => setCapabilityForm((current) => ({ ...current, paramsText: event.target.value }))}
                  placeholder='{"service":"api"}'
                />
                <Button
                  variant="secondary"
                  onClick={() => invokeMutation.mutate()}
                  disabled={invokeMutation.isPending || !capabilityForm.capabilityID || !connectorEnabled}
                >
                  <Play size={14} /> {t('connectors.action.invoke')}
                </Button>
              </div>
              {invokeResult && <pre className="bg-black/40 p-4 rounded-xl text-[0.7rem] font-mono text-info overflow-auto border border-white/5">{typeof invokeResult === 'string' ? invokeResult : JSON.stringify(invokeResult, null, 2)}</pre>}
            </div>
          </DetailSection>
        )}

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <DetailSection title={t('connectors.detail.metadata.title')} icon={<Settings2 size={18} className="text-muted-foreground" />} subtitle={t('connectors.detail.metadata.description')}>
            <div className="space-y-3">
              <div className="flex justify-between items-center py-2 border-b border-white/5">
                <span className="text-xs text-text-muted">{t('connectors.detail.metadata.id')}</span>
                <span className="text-sm font-mono">{connector.metadata.id}</span>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-white/5">
                <span className="text-xs text-text-muted">{t('connectors.detail.metadata.vendor')}</span>
                <span className="text-sm">{connector.metadata.vendor}</span>
              </div>
              <div className="flex justify-between items-center py-2 border-b border-white/5">
                <span className="text-xs text-text-muted">{t('connectors.detail.metadata.protocol')}</span>
                <span className="text-sm font-mono">{connector.spec.protocol} v{connector.metadata.version}</span>
              </div>
            </div>
          </DetailSection>

          <DetailSection title={t('connectors.detail.advanced.title')} icon={<Code size={18} className="text-muted-foreground" />}>
            <div className="space-y-4">
              <div className="space-y-2">
                <Button variant="outline" className="w-full justify-start" asChild>
                  <Link to="/ops?tab=connectors">{t('connectors.detail.advanced.rawConfig')}</Link>
                </Button>
                <Button variant="outline" className="w-full justify-start" onClick={() => exportMutation.mutate('yaml')} disabled={exportMutation.isPending || !connector.spec.import_export.exportable}>
                  <Download size={14} className="mr-2" /> {t('connectors.action.exportYaml')}
                </Button>
                <Button variant="outline" className="w-full justify-start" onClick={() => exportMutation.mutate('json')} disabled={exportMutation.isPending || !connector.spec.import_export.exportable}>
                  <Download size={14} className="mr-2" /> {t('connectors.action.exportJson')}
                </Button>
              </div>

              {selectedTemplateID && (
                <div className="pt-4 border-t border-white/5 space-y-3">
                  <div className="flex flex-col gap-1">
                    <span className="text-xs font-bold text-text-muted uppercase tracking-wider">{t('connectors.detail.template.title')}</span>
                    <p className="text-[0.7rem] text-text-muted">{t('connectors.detail.template.description')}</p>
                  </div>
                  <div className="flex gap-2">
                    <NativeSelect
                      value={selectedTemplateID}
                      onChange={(e) => setSelectedTemplateID(e.target.value)}
                      className="flex-1"
                    >
                      <option value="">{t('connectors.detail.template.select')}</option>
                    </NativeSelect>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => { if (selectedTemplateID) applyTemplateMutation.mutate(selectedTemplateID); }}
                      disabled={!selectedTemplateID || applyTemplateMutation.isPending}
                    >
                      <Wand2 size={14} /> {t('connectors.action.applyTemplate')}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          </DetailSection>
        </div>

        <details className="glass-card p-0 group overflow-hidden">
          <summary className="px-5 py-4 flex justify-between items-center cursor-pointer bg-white/[0.02] list-none select-none">
            <h3 className="m-0 text-text-muted flex items-center gap-2 text-base font-bold"><FileJson size={18} /> {t('connectors.detail.manifest.title')}</h3>
            <ChevronDown size={18} className="text-text-muted group-open:rotate-180 transition-transform" />
          </summary>
          <div className="p-4 bg-black/40">
            <pre className="text-[0.7rem] font-mono text-text-muted whitespace-pre-wrap leading-relaxed select-all m-0">{rawManifest}</pre>
          </div>
        </details>

        <GuidedFormDialog
          open={editorOpen}
          onOpenChange={setEditorOpen}
          title={t('connectors.editor.editAction')}
          description={t('connectors.editor.identityTitleEdit')}
          wide
        >
          {editorDraft ? (
            <ConnectorManifestEditor 
              manifest={editorDraft} 
              onChange={(next) => {
                setEditorDraft(next as ConnectorDetailViewData);
                setEditorProbeResult({ status: 'idle' });
              }} 
              disabled={editMutation.isPending}
              isEdit={true}
              onTest={() => editorProbeMutation.mutate()}
              testing={editorProbeMutation.isPending}
              testStatus={editorProbeResult}
              onConfirm={() => { if (editorDraft) editMutation.mutate(editorDraft); }}
              onCancel={() => {
                setEditorProbeResult({ status: 'idle' });
                setEditorOpen(false);
              }}
              confirmLabel={editMutation.isPending ? t('common.saving') : t('common.saveChanges', 'Save Changes')}
            />
          ) : null}
        </GuidedFormDialog>
      </div>
    </SplitDetailLayout>
  );
};

const StatItem = ({ label, value, success }: { label: string; value: string; success?: boolean }) => (
  <div className="flex flex-col gap-1">
    <span className="text-[0.65rem] font-bold text-text-muted uppercase tracking-wider">{label}</span>
    <span className={clsx("text-sm font-bold truncate", success ? "text-success" : "text-text-primary")}>{value}</span>
  </div>
);

const DetailSection = ({ title, icon, subtitle, children }: { title: string; icon: React.ReactNode; subtitle?: string; children: React.ReactNode }) => (
  <section className="space-y-4">
    <div className="flex items-center gap-2">
      {icon}
      <div className="flex flex-col">
        <h2 className="text-sm font-bold uppercase tracking-widest m-0">{title}</h2>
        {subtitle && <p className="text-[0.65rem] text-text-muted m-0">{subtitle}</p>}
      </div>
    </div>
    {children}
  </section>
);

function parseCapabilityParams(raw: string): { value: Record<string, unknown>; error?: string } {
  try {
    const parsed = JSON.parse(raw || '{}');
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return { value: parsed as Record<string, unknown> };
    }
    return { value: {}, error: 'Capability params must be a JSON object.' };
  } catch (error) {
    return { value: {}, error: error instanceof Error ? error.message : 'Invalid capability params.' };
  }
}

type ConnectorDetailViewData = NonNullable<Awaited<ReturnType<typeof fetchConnector>>>;
