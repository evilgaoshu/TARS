/**
 * MsgTemplatesPage — FE-25
 *
 * Notification template management for Inbox / Telegram / future channels.
 * Three template types: diagnosis / approval / execution_result.
 * Two locales per template: zh-CN / en-US.
 *
 * Data: backed by /api/v1/msg-templates (backend API).
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { ActiveBadge as StatusBadge } from '@/components/ui/active-badge';
import { CollapsibleList as CollapsibleRegistryList } from '@/components/ui/collapsible-list';
import { DetailHeader } from '@/components/ui/detail-header';
import { EmptyDetailState } from '@/components/ui/empty-detail-state';
import { FieldHint } from '@/components/ui/field-hint';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { LabeledField } from '@/components/ui/labeled-field';
import { SectionTitle, StatCard, SummaryGrid } from '@/components/ui/page-hero';
import {
  RegistryCard,
  RegistryDetail,
  RegistryPanel,
  RegistrySidebar,
  SplitLayout,
} from '@/components/ui/registry-primitives';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useNotify } from '@/hooks/ui/useNotify';
import { cn } from '@/lib/utils';
import { GuidedFormDialog } from '@/components/operator/GuidedFormDialog';
import {
  createMsgTemplate,
  exportMsgTemplate,
  fetchMsgTemplates,
  renderMsgTemplate,
  setMsgTemplateEnabled,
  updateMsgTemplate,
} from '../../lib/api/msgtpl';
import type { MsgTemplate } from '../../lib/api/types';
import { useI18n } from '@/hooks/useI18n';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type TemplateType = 'diagnosis' | 'approval' | 'execution_result';
export type TemplateLocale = 'zh-CN' | 'en-US';

// ---------------------------------------------------------------------------
// Variable whitelists (per type)
// ---------------------------------------------------------------------------

const VARIABLE_HINTS: Record<TemplateType, { name: string; description: string }[]> = {
  diagnosis: [
    { name: '{{AlertIcon}}', description: '严重性图标 (🔴/🟠/ℹ️)' },
    { name: '{{AlertName}}', description: '告警名称 (alertname)' },
    { name: '{{TargetContext}}', description: '服务 / 实例上下文' },
    { name: '{{Summary}}', description: 'AI 诊断摘要' },
    { name: '{{Recommendation}}', description: 'AI 建议执行命令描述' },
    { name: '{{Citations}}', description: '引用的 Runbook / 知识来源 (可选)' },
    { name: '{{SessionID}}', description: '会话 ID (ses_xxx)' },
  ],
  approval: [
    { name: '{{ExecutionID}}', description: '执行请求 ID (exe_xxx)' },
    { name: '{{TargetHost}}', description: '目标主机' },
    { name: '{{RiskLevel}}', description: '风险等级 (critical / warning / info)' },
    { name: '{{Command}}', description: '待执行命令' },
    { name: '{{ApprovalSource}}', description: '审批来源 (service_owner / oncall)' },
    { name: '{{Timeout}}', description: '审批时限 (分钟)' },
    { name: '{{SessionID}}', description: '会话 ID (ses_xxx)' },
  ],
  execution_result: [
    { name: '{{ExecutionStatus}}', description: '执行状态 (completed / failed / timeout)' },
    { name: '{{TargetHost}}', description: '执行主机' },
    { name: '{{ExitCode}}', description: '退出码 (失败时)' },
    { name: '{{OutputPreview}}', description: '命令输出预览 (前5行)' },
    { name: '{{TruncationFlag}}', description: '输出是否被截断' },
    { name: '{{ActionTip}}', description: 'AI 结果建议' },
    { name: '{{SessionID}}', description: '会话 ID (ses_xxx)' },
  ],
};

// ---------------------------------------------------------------------------
// Sample data for preview rendering
// ---------------------------------------------------------------------------

const SAMPLE_DATA: Record<string, string> = {
  AlertIcon: '🔴',
  AlertName: 'HighCPUUsage',
  TargetContext: 'prod-web-01 (web)',
  Summary: 'CPU 持续高于 90%，Nginx worker 异常且连接数暴增，优先确认流量异常与服务存活状态。',
  Recommendation: 'systemctl status nginx',
  Citations: '1 条知识',
  SessionID: 'ses_example',
  ExecutionID: 'exe_example',
  TargetHost: 'prod-web-01',
  RiskLevel: '🟠 WARNING',
  Command: 'systemctl restart nginx && systemctl status nginx',
  ApprovalSource: 'service_owner (web)',
  Timeout: '15',
  ExecutionStatus: 'completed',
  ExitCode: '0',
  OutputPreview: '● nginx.service - A high performance web server\n  Active: active (running)',
  TruncationFlag: 'false',
  ActionTip: '执行成功，建议确认服务持续运行状态。',
};

// ---------------------------------------------------------------------------
// Preview renderer
// ---------------------------------------------------------------------------

function renderPreview(body: string, data: Record<string, string>): string {
  return body.replaceAll(/\{\{(\w+)\}\}/g, (_, key: string) => data[key] ?? `{{${key}}}`);
}

// ---------------------------------------------------------------------------
// default initialization
// ---------------------------------------------------------------------------

const defaultForm = (): MsgTemplate => ({
  id: '',
  kind: 'diagnosis' as const,
  locale: 'zh-CN',
  name: '',
  status: 'draft',
  enabled: true,
  variable_schema: {},
  usage_refs: [],
  content: { subject: '', body: '' },
});

export const MsgTemplatesPage = () => {
  const { t } = useI18n();

  const KIND_LABELS: Record<MsgTemplate['kind'], string> = {
    diagnosis: t('msgtpl.type.diagnosis'),
    approval: t('msgtpl.type.approval'),
    execution_result: t('msgtpl.type.executionResult'),
  };

  const LOCALE_LABELS: Record<TemplateLocale, string> = {
    'zh-CN': t('msgtpl.detail.locale.zh'),
    'en-US': t('msgtpl.detail.locale.en'),
  };
  const notify = useNotify();
  const [items, setItems] = useState<MsgTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [selectedID, setSelectedID] = useState<string>('');
  const [isCreating, setIsCreating] = useState(false);
  const [form, setForm] = useState<MsgTemplate>(defaultForm());
  const [previewMode, setPreviewMode] = useState(false);
  const [serverRender, setServerRender] = useState<{ subject: string; body: string } | null>(null);
  const [message, setMessage] = useState<{ type: 'success' | 'warning' | 'error' | 'info'; text: string } | null>(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [createForm, setCreateForm] = useState<MsgTemplate>(defaultForm());

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchMsgTemplates({ limit: 100 })
      .then((resp) => {
        if (!cancelled) {
          setItems(resp.items ?? []);
          if (resp.items?.length > 0) {
            setSelectedID(resp.items[0].id);
            setForm({ ...resp.items[0] });
          }
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setMessage({ type: 'error', text: `${t('msgtpl.opFailed')} ${String(err)}` });
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const selectTemplate = useCallback((id: string) => {
    const found = items.find((tpl) => tpl.id === id);
    if (found) {
      setIsCreating(false);
      setSelectedID(id);
      setForm({ ...found });
      setPreviewMode(false);
      setServerRender(null);
      setMessage(null);
    }
  }, [items]);

  const startCreate = () => {
    setCreateForm(defaultForm());
    setCreateDialogOpen(true);
  };

  const handleCreateSave = async () => {
    if (!createForm.name.trim()) {
      setMessage({ type: 'error', text: t('msgtpl.error.nameRequired') });
      return;
    }
    if (!createForm.content.body.trim()) {
      setMessage({ type: 'error', text: t('msgtpl.error.bodyRequired') });
      return;
    }
    setSaving(true);
    try {
      const saved = await createMsgTemplate(
        { kind: createForm.kind, locale: createForm.locale, name: createForm.name, enabled: createForm.enabled, content: createForm.content },
        'Create message template',
      );
      setItems((prev) => [...prev, saved]);
      setIsCreating(false);
      setSelectedID(saved.id);
      setForm({ ...saved });
      setCreateDialogOpen(false);
      setMessage({ type: 'success', text: t('msgtpl.created') });
    } catch (err: unknown) {
      setMessage({ type: 'error', text: `${t('msgtpl.opFailed')} ${String(err)}` });
    } finally {
      setSaving(false);
    }
  };

  const handleSave = async () => {
    if (!isCreating && !form.id.trim()) {
      setMessage({ type: 'error', text: t('msgtpl.error.idRequired') });
      return;
    }
    if (!form.name.trim()) {
      setMessage({ type: 'error', text: t('msgtpl.error.nameRequired') });
      return;
    }
    if (!form.content.body.trim()) {
      setMessage({ type: 'error', text: t('msgtpl.error.bodyRequired') });
      return;
    }
    setSaving(true);
    try {
      let saved: MsgTemplate;
      if (isCreating) {
        saved = await createMsgTemplate(
          { kind: form.kind, locale: form.locale, name: form.name, enabled: form.enabled, content: form.content },
          'Create message template',
        );
        setItems((prev) => [...prev, saved]);
      } else {
        saved = await updateMsgTemplate(form.id, form, 'Update message template');
        setItems((prev) => prev.map((tpl) => (tpl.id === saved.id ? saved : tpl)));
      }
      setIsCreating(false);
      setSelectedID(saved.id);
      setForm({ ...saved });
      setMessage({ type: 'success', text: isCreating ? t('msgtpl.created') : t('msgtpl.saved') });
    } catch (err: unknown) {
      setMessage({ type: 'error', text: `${t('msgtpl.opFailed')} ${String(err)}` });
    } finally {
      setSaving(false);
    }
  };

  const handleToggle = async (id: string) => {
    const target = items.find((tpl) => tpl.id === id);
    if (!target) return;

    setSaving(true);
    try {
      const updated = await setMsgTemplateEnabled(id, !target.enabled, target.enabled ? 'Disable message template' : 'Enable message template');
      setItems((prev) => prev.map((tpl) => (tpl.id === id ? updated : tpl)));
      setForm({ ...updated });
         setMessage({ type: 'success', text: updated.status === 'active' ? t('msgtpl.enabled') : t('msgtpl.disabled') });
    } catch (err: unknown) {
      setMessage({ type: 'error', text: `${t('msgtpl.opFailed')} ${String(err)}` });
    } finally {
      setSaving(false);
    }
  };

  const handlePreview = async () => {
    if (previewMode) {
      setPreviewMode(false);
      setServerRender(null);
      return;
    }
    setPreviewMode(true);
    if (selectedID) {
      try {
        const result = await renderMsgTemplate(selectedID, SAMPLE_DATA);
        setServerRender({ subject: result.subject, body: result.body });
        setMessage({ type: 'info', text: t('msgtpl.preview.serverInfo') });
      } catch {
        setServerRender(null);
        setMessage({ type: 'info', text: t('msgtpl.preview.localFallback') });
      }
    } else {
      setServerRender(null);
    }
  };

  const handleExport = async (format: 'json' | 'yaml') => {
    if (!selectedID) return;
    try {
      const { blob, filename } = await exportMsgTemplate(selectedID, format);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      a.click();
      URL.revokeObjectURL(url);
      notify.success(t('msgtpl.export.success', { filename }), 'Export');
    } catch (err: unknown) {
      notify.error(`${t('msgtpl.export.failed')}: ${String(err)}`, t('msgtpl.export.failed'));
    }
  };

  const previewText = useMemo(() => {
    if (!previewMode) return '';
    if (serverRender) return serverRender.body;
    return renderPreview(form.content.body, SAMPLE_DATA);
  }, [previewMode, serverRender, form.content.body]);

  const byKind = useMemo(() => ({
    diagnosis: items.filter((t) => t.kind === 'diagnosis'),
    approval: items.filter((t) => t.kind === 'approval'),
    execution_result: items.filter((t) => t.kind === 'execution_result'),
  }), [items]);

  const showDetail = isCreating || Boolean(selectedID);
  const hints = VARIABLE_HINTS[form.kind as TemplateType] ?? [];
  const variableSchemaEntries = Object.entries(form.variable_schema || {});
  const usageRefs = form.usage_refs || [];

  if (loading) {
    return (
      <div className="animate-fade-in px-8 py-8 text-sm text-muted-foreground">
        {t('msgtpl.loading')}
      </div>
    );
  }

  return (
    <div className="animate-fade-in grid gap-6">
      <SectionTitle
        title={t('msgtpl.title')}
        subtitle={t('msgtpl.subtitle')}
      />

      <SummaryGrid>
        <StatCard title={t('msgtpl.stat.templates')} value={String(items.length)} subtitle={t('msgtpl.stat.totalDesc')} />
        <StatCard title={t('msgtpl.stat.enabled')} value={String(items.filter((tpl) => tpl.enabled).length)} subtitle={t('msgtpl.stat.enabledDesc')} />
        <StatCard title={t('msgtpl.stat.types')} value="3" subtitle={t('msgtpl.stat.typesDesc')} />
        <StatCard title={t('msgtpl.stat.storage')} value="backend" subtitle={t('msgtpl.stat.storageDesc')} />
      </SummaryGrid>

      {message ? <StatusMessage message={message.text} type={message.type} /> : null}

      <SplitLayout
        sidebar={
          <RegistrySidebar className="p-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="space-y-1.5">
                <h2 className="text-lg font-semibold tracking-tight text-foreground">{t('msgtpl.sidebar.title')}</h2>
                <p className="text-sm leading-6 text-muted-foreground">
                  {t('msgtpl.sidebar.desc')}
                </p>
              </div>
              <Button variant="amber" type="button" onClick={startCreate} disabled={saving}>
                {t('msgtpl.sidebar.new')}
              </Button>
            </div>

            <div className="mt-5 flex flex-col gap-5">
              {(['diagnosis', 'approval', 'execution_result'] as const).map((kind) => (
                <RegistryPanel key={kind} title={KIND_LABELS[kind]} emptyText={t('msgtpl.sidebar.empty')}>
                  <CollapsibleRegistryList
                    visibleCount={4}
                    emptyText={t('msgtpl.sidebar.empty')}
                    items={byKind[kind].map((tpl) => (
                      <button
                        key={tpl.id}
                        type="button"
                        onClick={() => selectTemplate(tpl.id)}
                        className="block w-full appearance-none border-0 bg-transparent p-0 text-left"
                      >
                        <RegistryCard
                          active={!isCreating && selectedID === tpl.id}
                          title={tpl.name || tpl.id}
                          subtitle={`${tpl.id} · ${LOCALE_LABELS[tpl.locale as TemplateLocale] ?? tpl.locale}`}
                          lines={[`Updated: ${tpl.updated_at ? new Date(tpl.updated_at).toLocaleString() : '—'}`]}
                          status={
                            <StatusBadge 
                              active={tpl.enabled} 
                              label={t(`msgtpl.status.${tpl.status?.toLowerCase() || 'draft'}`)} 
                            />
                          }
                        />
                      </button>
                    ))}
                  />
                </RegistryPanel>
              ))}
            </div>
          </RegistrySidebar>
        }
        detail={showDetail ? (
          <RegistryDetail className="p-6">
            <div className="grid gap-6">
              <DetailHeader
                title={isCreating ? t('msgtpl.detail.newTitle') : (form.name || form.id || 'Template Detail')}
                subtitle={isCreating ? t('msgtpl.detail.newSubtitle') : t('msgtpl.detail.editSubtitle')}
                status={
                  <StatusBadge 
                    active={form.enabled} 
                    label={t(`msgtpl.status.${form.status?.toLowerCase() || (form.enabled ? 'active' : 'draft')}`)} 
                  />
                }
                actions={selectedID ? (
                  <>
                    <Button
                      variant="outline"
                      type="button"
                      disabled={saving}
                      onClick={() => handleToggle(selectedID)}
                    >
                      {form.enabled ? t('msgtpl.detail.disable') : t('msgtpl.detail.enable')}
                    </Button>
                    <Button
                      variant={previewMode ? 'amber' : 'outline'}
                      type="button"
                      onClick={() => { void handlePreview(); }}
                    >
                      {previewMode ? t('msgtpl.detail.edit') : t('msgtpl.detail.preview')}
                    </Button>
                    <Button
                      variant="outline"
                      type="button"
                      onClick={() => { void handleExport('json'); }}
                    >
                      {t('msgtpl.detail.exportJson')}
                    </Button>
                    <Button
                      variant="outline"
                      type="button"
                      onClick={() => { void handleExport('yaml'); }}
                    >
                      {t('msgtpl.detail.exportYaml')}
                    </Button>
                  </>
                ) : undefined}
              />

              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                <LabeledField label={t('msgtpl.detail.displayName')} required>
                  <Input
                    value={form.name}
                    onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                    placeholder={t('msgtpl.detail.displayNamePlaceholder') || "诊断消息 (中文)"}
                  />
                </LabeledField>

                <LabeledField label={t('msgtpl.detail.type')} required>
                  <NativeSelect
                    value={form.kind}
                    disabled={!isCreating}
                    onChange={(e) => setForm((f) => ({ ...f, kind: e.target.value as MsgTemplate['kind'] }))}
                    className="bg-background"
                  >
                    <option value="diagnosis">{t('msgtpl.type.diagnosis')}</option>
                    <option value="approval">{t('msgtpl.type.approval')}</option>
                    <option value="execution_result">{t('msgtpl.type.executionResult')}</option>
                  </NativeSelect>
                </LabeledField>

                <LabeledField label={t('msgtpl.detail.locale')} required>
                  <NativeSelect
                    value={form.locale}
                    disabled={!isCreating}
                    onChange={(e) => setForm((f) => ({ ...f, locale: e.target.value as TemplateLocale }))}
                    className="bg-background"
                  >
                    <option value="zh-CN">{t('msgtpl.detail.locale.zh')}</option>
                    <option value="en-US">{t('msgtpl.detail.locale.en')}</option>
                  </NativeSelect>
                </LabeledField>

                <LabeledField label={t('msgtpl.detail.lifecycle')} required>
                  <NativeSelect
                    value={form.status || 'draft'}
                    onChange={(e) => setForm((f) => {
                      const status = e.target.value;
                      return { ...f, status, enabled: status === 'active' };
                    })}
                    className="bg-background"
                  >
                    <option value="draft">{t('msgtpl.status.draft')}</option>
                    <option value="active">{t('msgtpl.status.active')}</option>
                    <option value="deprecated">{t('msgtpl.status.deprecated')}</option>
                    <option value="archived">{t('msgtpl.status.archived')}</option>
                  </NativeSelect>
                </LabeledField>
              </div>

              <LabeledField label={t('msgtpl.detail.subject')} required>
                <Input
                  value={previewMode ? (serverRender?.subject ?? renderPreview(form.content.subject, SAMPLE_DATA)) : form.content.subject}
                  readOnly={previewMode}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, content: { ...f.content, subject: e.target.value } }))
                  }
                  placeholder="[TARS] 诊断"
                />
                <FieldHint>
                  {previewMode ? t('msgtpl.detail.subjectPreviewHint') : t('msgtpl.detail.subjectHint')}
                </FieldHint>
              </LabeledField>

              <LabeledField label={t('msgtpl.detail.body')} required>
                <Textarea
                  rows={10}
                  value={previewMode ? previewText : form.content.body}
                  readOnly={previewMode}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, content: { ...f.content, body: e.target.value } }))
                  }
                  placeholder={'[TARS] 诊断\n告警: {{AlertName}} @ {{TargetContext}}\n结论: {{Summary}}'}
                  className="resize-y font-mono"
                />
                {previewMode ? (
                  <FieldHint>{t('msgtpl.detail.bodyPreviewHint', { source: serverRender ? t('msgtpl.detail.bodyPreviewServer') : t('msgtpl.detail.bodyPreviewLocal') })}</FieldHint>
                ) : (
                  <FieldHint>{t('msgtpl.detail.bodyHint')}</FieldHint>
                )}
              </LabeledField>

              <div className="grid gap-3">
                <div className="text-[0.72rem] font-black uppercase tracking-[0.16em] text-muted-foreground">
                  {t('msgtpl.detail.variables')} ({KIND_LABELS[form.kind] || form.kind})
                </div>
                <CollapsibleRegistryList
                  visibleCount={4}
                  emptyText={t('msgtpl.sidebar.empty')}
                  items={hints.map((h) => (
                    <div
                      key={h.name}
                      className={cn(
                        'flex items-start gap-3 rounded-xl border border-border/60 bg-white/[0.03] px-3 py-2.5',
                        previewMode && 'bg-primary/5'
                      )}
                    >
                      <code className="shrink-0 font-mono text-xs text-info">{h.name}</code>
                      <span className="text-xs leading-5 text-muted-foreground">{h.description}</span>
                    </div>
                  ))}
                  />
              </div>

              <div className="grid gap-3">
                <div className="text-[0.72rem] font-black uppercase tracking-[0.16em] text-muted-foreground">
                  {t('msgtpl.detail.usageRefs')}
                </div>
                {usageRefs.length > 0 ? (
                  <div className="grid gap-2">
                    {usageRefs.map((usageRef) => (
                      <div key={usageRef} className="rounded-xl border border-border/60 bg-white/[0.03] px-3 py-2.5 text-sm text-foreground">
                        {usageRef}
                      </div>
                    ))}
                  </div>
                ) : (
                  <FieldHint>No usage references yet.</FieldHint>
                )}
              </div>

              <div className="grid gap-6 rounded-2xl border border-dashed border-border/60 bg-muted/20 p-5 pt-4">
                <div className="flex items-center justify-between">
                  <div className="text-[0.72rem] font-bold uppercase tracking-[0.16em] text-muted-foreground/80">
                    {t('msgtpl.detail.advanced')}
                  </div>
                </div>

                <div className="grid gap-5">
                  <div className="grid gap-3">
                    <div className="text-[0.62rem] font-black uppercase tracking-[0.12em] text-muted-foreground/60">
                      {t('msgtpl.detail.variableSchema')}
                    </div>
                    {variableSchemaEntries.length > 0 ? (
                      <div className="grid gap-2">
                        {variableSchemaEntries.map(([key, value]) => (
                          <div key={key} className="flex items-center justify-between gap-3 rounded-xl border border-border/40 bg-background/40 px-3 py-2 text-xs">
                            <code className="font-mono text-info">{key}</code>
                            <span className="text-muted-foreground">{String(value)}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <FieldHint>No variable schema defined.</FieldHint>
                    )}
                  </div>

                  {!isCreating && (
                    <LabeledField label={t('msgtpl.detail.templateId')}>
                      <div className="flex items-center gap-2">
                        <Input value={form.id} disabled readOnly className="h-8 text-xs font-mono" />
                      </div>
                      <FieldHint>{t('msgtpl.detail.idHint')}</FieldHint>
                    </LabeledField>
                  )}
                </div>
              </div>

              <div className="flex flex-wrap gap-3">
                <Button variant="amber" type="button" disabled={saving} onClick={handleSave}>
                  {saving ? t('msgtpl.detail.saving') : (isCreating ? t('msgtpl.detail.create') : t('msgtpl.detail.saveChanges'))}
                </Button>
                {isCreating ? (
                  <Button variant="outline" type="button" disabled={saving} onClick={startCreate}>
                    {t('msgtpl.detail.reset')}
                  </Button>
                ) : null}
              </div>
            </div>
          </RegistryDetail>
        ) : (
          <EmptyDetailState
            title={t('msgtpl.empty.title')}
            description={t('msgtpl.empty.desc')}
          />
        )}
      />

      {/* ── Create Template Dialog ── */}
      <GuidedFormDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        title={t('msgtpl.dialog.title')}
        description={t('msgtpl.dialog.desc')}
        onConfirm={() => void handleCreateSave()}
        confirmLabel={saving ? t('msgtpl.dialog.creating') : t('msgtpl.detail.create')}
        loading={saving}
        wide
      >
        <div className="space-y-5">
          <div className="space-y-4">
            <div className="text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">{t('msgtpl.dialog.required')}</div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <LabeledField label={t('msgtpl.detail.displayName')} required>
                <Input
                  value={createForm.name}
                  onChange={(e) => setCreateForm((f) => ({ ...f, name: e.target.value }))}
                  placeholder="Diagnosis Message (Chinese)"
                />
              </LabeledField>
              <LabeledField label={t('msgtpl.detail.type')} required>
                <NativeSelect
                  value={createForm.kind}
                  onChange={(e) => setCreateForm((f) => ({ ...f, kind: e.target.value as MsgTemplate['kind'] }))}
                  className="bg-background"
                >
                  <option value="diagnosis">{t('msgtpl.type.diagnosis')}</option>
                  <option value="approval">{t('msgtpl.type.approval')}</option>
                  <option value="execution_result">{t('msgtpl.type.executionResult')}</option>
                </NativeSelect>
              </LabeledField>
              <LabeledField label={t('msgtpl.detail.locale')} required>
                <NativeSelect
                  value={createForm.locale}
                  onChange={(e) => setCreateForm((f) => ({ ...f, locale: e.target.value as TemplateLocale }))}
                  className="bg-background"
                >
                  <option value="zh-CN">{t('msgtpl.detail.locale.zh')}</option>
                  <option value="en-US">{t('msgtpl.detail.locale.en')}</option>
                </NativeSelect>
              </LabeledField>
            </div>
            <LabeledField label={t('msgtpl.detail.subject')} required>
              <Input
                value={createForm.content.subject}
                onChange={(e) => setCreateForm((f) => ({ ...f, content: { ...f.content, subject: e.target.value } }))}
                placeholder="[TARS] Diagnosis"
              />
            </LabeledField>
            <LabeledField label={t('msgtpl.detail.body')} required>
              <Textarea
                rows={6}
                value={createForm.content.body}
                onChange={(e) => setCreateForm((f) => ({ ...f, content: { ...f.content, body: e.target.value } }))}
                placeholder={'[TARS] Diagnosis\nAlert: {{AlertName}} @ {{TargetContext}}\nSummary: {{Summary}}'}
                className="resize-y font-mono"
              />
              <FieldHint>{t('msgtpl.dialog.bodyHint')}</FieldHint>
            </LabeledField>
          </div>
        </div>
      </GuidedFormDialog>
    </div>
  );
};

export default MsgTemplatesPage;
