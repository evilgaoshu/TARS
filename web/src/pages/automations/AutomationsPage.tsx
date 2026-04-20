import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { AlertTriangle, Clock3, PlayCircle, Workflow } from 'lucide-react';
import {
  createAutomation,
  fetchAutomations,
  fetchConnectors,
  fetchSkills,
  getApiErrorMessage,
  runAutomationNow,
  setAutomationEnabled,
  updateAutomation,
} from '../../lib/api/ops';
import { fetchAgentRoles } from '../../lib/api/agent-roles';
import { listTriggers } from '../../lib/api/triggers';
import type { AgentRole, AutomationJob, ConnectorManifest, SkillManifest, TriggerDTO } from '../../lib/api/types';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { NativeSelect } from '@/components/ui/select';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { FilterBar } from '@/components/ui/filter-bar';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { LabeledField } from '@/components/ui/labeled-field';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { FieldHint } from '@/components/ui/field-hint';
import { GuidedFormDialog } from '@/components/operator/GuidedFormDialog';
import { useI18n } from '@/hooks/useI18n';

const schedulePresets = ['@every 5m', '@every 15m', '@every 1h', '0 * * * *', '0 8 * * *'];

export const AutomationsPage = () => {
  const { t } = useI18n();

  const [items, setItems] = useState<AutomationJob[]>([]);
  const [skills, setSkills] = useState<SkillManifest[]>([]);
  const [connectors, setConnectors] = useState<ConnectorManifest[]>([]);
  const [agentRoles, setAgentRoles] = useState<AgentRole[]>([]);
  const [triggers, setTriggers] = useState<TriggerDTO[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingID, setEditingID] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [type, setType] = useState('');
  const [form, setForm] = useState<AutomationJob>(() => defaultAutomationJob());
  const [skillContextText, setSkillContextText] = useState(() => JSON.stringify(defaultAutomationJob().skill?.context || {}, null, 2));
  const [connectorParamsText, setConnectorParamsText] = useState(() => JSON.stringify(defaultAutomationJob().connector_capability?.params || {}, null, 2));
  const [skillContextError, setSkillContextError] = useState('');
  const [connectorParamsError, setConnectorParamsError] = useState('');

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        setLoading(true);
        setError('');
        const [automationResp, skillResp, connectorResp, roleResp, triggerResp] = await Promise.all([
          fetchAutomations({ q: query || undefined, type: type || undefined, sort_by: 'id', sort_order: 'asc' }),
          fetchSkills({ limit: 100, sort_by: 'id', sort_order: 'asc' }),
          fetchConnectors({ limit: 100, sort_by: 'id', sort_order: 'asc' }),
          fetchAgentRoles({ limit: 100, sort_by: 'role_id', sort_order: 'asc' }),
          listTriggers({ limit: 200 }),
        ]);
        if (!active) return;
        setItems(automationResp.items);
        setSkills(skillResp.items);
        setConnectors(connectorResp.items);
        setAgentRoles(roleResp.items.filter((r) => r.status === 'active'));
        setTriggers(triggerResp.items || []);
      } catch (loadError) {
        if (!active) return;
        setError(getApiErrorMessage(loadError, 'Failed to load automations.'));
      } finally {
        if (active) setLoading(false);
      }
    };
    void load();
    return () => {
      active = false;
    };
  }, [query, type]);

  const capabilityChoices = useMemo(() => {
    const choices: Array<{ key: string; label: string; connector_id: string; capability_id: string; read_only: boolean }> = [];
    connectors.forEach((connector) => {
      (connector.spec.capabilities || []).forEach((capability) => {
        choices.push({
          key: `${connector.metadata.id}/${capability.id}`,
          label: `${connector.metadata.display_name || connector.metadata.name || connector.metadata.id} -> ${capability.id}`,
          connector_id: connector.metadata.id || '',
          capability_id: capability.id || '',
          read_only: capability.read_only,
        });
      });
    });
    return choices;
  }, [connectors]);

  const selectedCapability = capabilityChoices.find((item) => item.key === form.target_ref);
  const enabledCount = items.filter((item) => item.enabled).length;
  const failingCount = items.filter((item) => item.state?.last_outcome === 'failed' || item.last_run?.status === 'failed').length;
  const dueSoonCount = items.filter((item) => isDueSoon(item.state?.next_run_at)).length;

  const saveJob = async () => {
    try {
      setSaving(true);
      setError('');
      const payload = normalizeForm(form, selectedCapability?.connector_id, selectedCapability?.capability_id);
      const saved = editingID ? await updateAutomation(editingID, { job: payload }) : await createAutomation({ job: payload });
      setItems((current) => [saved, ...current.filter((item) => item.id !== saved.id)]);
      setDialogOpen(false);
      setEditingID(null);
      setForm(defaultAutomationJob());
      setSkillContextText(JSON.stringify(defaultAutomationJob().skill?.context || {}, null, 2));
      setConnectorParamsText(JSON.stringify(defaultAutomationJob().connector_capability?.params || {}, null, 2));
      setSkillContextError('');
      setConnectorParamsError('');
    } catch (saveError) {
      setError(getApiErrorMessage(saveError, 'Failed to save automation.'));
    } finally {
      setSaving(false);
    }
  };

  const toggleEnabled = async (job: AutomationJob) => {
    try {
      const updated = await setAutomationEnabled(job.id, !job.enabled);
      setItems((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    } catch (toggleError) {
      setError(getApiErrorMessage(toggleError, 'Failed to update automation state.'));
    }
  };

  const runNow = async (job: AutomationJob) => {
    try {
      const updated = await runAutomationNow(job.id);
      setItems((current) => current.map((item) => (item.id === updated.id ? updated : item)));
    } catch (runError) {
      setError(getApiErrorMessage(runError, 'Failed to run automation.'));
    }
  };

  const startEdit = (job: AutomationJob) => {
    setEditingID(job.id);
    setForm(structuredClone(job));
    setSkillContextText(JSON.stringify(job.skill?.context || {}, null, 2));
    setConnectorParamsText(JSON.stringify(job.connector_capability?.params || {}, null, 2));
    setSkillContextError('');
    setConnectorParamsError('');
    setDialogOpen(true);
  };

  const renderAutomationCard = (item: AutomationJob) => {
    const itemTriggers = triggers.filter((t) => t.automation_job_id === item.id);
    const isScheduled = !!item.schedule;
    const isEventDriven = itemTriggers.length > 0;

    return (
      <Card key={item.id} className="group relative overflow-hidden border border-white/10 bg-white/[0.02] p-0 transition-all hover:border-primary/30 hover:bg-white/[0.04]">
        <div className={cn("absolute left-0 top-0 h-full w-1", item.enabled ? "bg-success shadow-[0_0_8px_var(--success)]" : "bg-muted")} />

        <div className="p-6">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
            <div className="flex-1 space-y-4">
              <div className="flex flex-wrap items-center gap-3">
                <h3 className="m-0 text-lg font-bold leading-none tracking-tight text-foreground">{item.display_name}</h3>
                <div className="flex gap-1.5">
                  <Badge variant={item.enabled ? 'success' : 'muted'} className="h-5 px-1.5 text-[0.65rem] font-black uppercase tracking-wider">
                    {item.enabled ? t('common.status.enabled') : t('common.status.disabled')}
                  </Badge>
                  <Badge variant="outline" className="h-5 px-1.5 text-[0.65rem] font-bold opacity-60">
                    {item.id}
                  </Badge>
                </div>
              </div>
              <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
                {item.description || t('auto.card.noDescription')}
              </p>

              <div className="grid grid-cols-1 gap-4 rounded-2xl bg-black/20 p-4 sm:grid-cols-2 lg:grid-cols-4">
                <div className="space-y-1.5">
                  <div className="flex items-center gap-1.5 text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-60">
                    <Clock3 size={12} /> {t('auto.card.when')}
                  </div>
                  <div className="flex flex-col gap-1">
                    {isScheduled && (
                      <div className="text-xs font-mono font-bold text-primary">{item.schedule}</div>
                    )}
                    {isEventDriven && itemTriggers.map(trig => (
                      <div key={trig.id} className="flex items-center gap-1.5 text-xs text-foreground">
                        <Workflow size={10} className="text-info" />
                        <span className="truncate">{trig.event_type}</span>
                      </div>
                    ))}
                    {!isScheduled && !isEventDriven && (
                      <div className="text-xs italic text-muted-foreground">{t('auto.card.manualOnly')}</div>
                    )}
                  </div>
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center gap-1.5 text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-60">
                    <PlayCircle size={12} /> {t('auto.card.do')}
                  </div>
                  <div className="space-y-1">
                    <div className="text-xs font-bold text-foreground">
                      {item.type === 'skill' ? (
                        <span className="flex items-center gap-1.5">
                          <Badge variant="info" className="h-4 px-1 text-[0.6rem] uppercase">{t('auto.filter.skill')}</Badge>
                          <span className="truncate">{item.skill?.skill_id || item.target_ref}</span>
                        </span>
                      ) : (
                        <span className="flex items-center gap-1.5">
                          <Badge variant="secondary" className="h-4 px-1 text-[0.6rem] uppercase tracking-tighter">{t('auto.filter.connectorCapability')}</Badge>
                          <span className="truncate font-mono">{item.target_ref}</span>
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center gap-1.5 text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-60">
                    <Workflow size={12} /> {t('auto.card.runAs')}
                  </div>
                  <div className="text-xs font-medium text-foreground">
                    {item.agent_role_id || t('auto.form.agentRoleDefault')}
                  </div>
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center gap-1.5 text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-60">
                    <AlertTriangle size={12} /> {t('auto.card.notify')}
                  </div>
                  <div className="space-y-1">
                    {isEventDriven ? (
                      <>
                        <div className="text-[0.65rem] font-bold uppercase tracking-wider text-muted-foreground">
                          {t('auto.card.linkedTriggers')} {itemTriggers.length}
                        </div>
                        <div className="flex flex-wrap gap-1">
                          {itemTriggers.map(trig => (
                            <Badge key={trig.id} variant="outline" className="h-4 px-1 text-[0.6rem] bg-white/5 border-white/10 uppercase tracking-tighter">
                              {trig.display_name || trig.channel_id || trig.channel || 'inbox'}
                            </Badge>
                          ))}
                        </div>
                      </>
                    ) : (
                      <span className="text-xs italic text-muted-foreground">{t('auto.card.logsOnly')}</span>
                    )}
                  </div>
                </div>
              </div>
            </div>

            <div className="flex flex-row items-center gap-2 lg:flex-col lg:items-stretch">
              <Button variant="glass" size="sm" onClick={() => startEdit(item)} className="h-8 font-bold uppercase tracking-widest text-[0.65rem]">
                 {t('common.edit')}
              </Button>
              <Button variant="outline" size="sm" onClick={() => void toggleEnabled(item)} className="h-8 font-bold uppercase tracking-widest text-[0.65rem]">
                 {item.enabled ? t('common.disable') : t('common.enable')}
              </Button>
              <Button variant="amber" size="sm" onClick={() => void runNow(item)} className="h-8 font-black uppercase tracking-widest text-[0.65rem] shadow-lg shadow-amber-500/10">
                 {t('auto.card.runNow')}
              </Button>
            </div>
          </div>

          <div className="mt-6 flex flex-wrap items-center gap-x-8 gap-y-3 border-t border-white/5 pt-4">
            <StatItem label={t('auto.card.lastOutcome')} value={item.state?.last_outcome || item.last_run?.status || t('auto.card.neverRun')} accent={item.state?.last_outcome === 'failed'} />
            <StatItem label={t('auto.card.nextRun')} value={formatTime(item.state?.next_run_at, t)} />
            <StatItem label={t('auto.card.failures')} value={String(item.state?.consecutive_failures || 0)} warning={(item.state?.consecutive_failures || 0) > 0} />
            <StatItem label={t('auto.card.timeout')} value={`${item.timeout_seconds || 30}s`} />
            <div className="flex-1" />
            {item.last_run?.completed_at && (
              <div className="text-[0.65rem] font-medium text-muted-foreground opacity-40">
                {t('auto.card.finished')} {new Date(item.last_run.completed_at).toLocaleString()}
              </div>
            )}
          </div>

          {(item.last_run?.error || item.state?.last_error) && (
            <div className="mt-4 rounded-xl bg-danger/5 border border-danger/10 p-3 flex gap-3 items-start animate-fade-in">
              <AlertTriangle size={14} className="mt-0.5 text-danger shrink-0" />
              <div className="text-xs text-danger leading-relaxed font-medium">
                {item.last_run?.error || item.state?.last_error}
              </div>
            </div>
          )}
        </div>
      </Card>
    );
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle title={t('auto.title')} subtitle={t('auto.subtitle')} />

      <SummaryGrid>
        <StatCard title={t('auto.stats.total')} value={String(items.length)} subtitle={t('auto.stats.totalDesc')} icon={<Workflow size={16} />} />
        <StatCard title={t('auto.stats.enabled')} value={String(enabledCount)} subtitle={t('auto.stats.enabledDesc')} icon={<PlayCircle size={16} />} />
        <StatCard title={t('auto.stats.failing')} value={String(failingCount)} subtitle={t('auto.stats.failingDesc')} icon={<AlertTriangle size={16} />} />
        <StatCard title={t('auto.stats.dueSoon')} value={String(dueSoonCount)} subtitle={t('auto.stats.dueSoonDesc')} icon={<Clock3 size={16} />} />
      </SummaryGrid>

      <GuidedFormDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={editingID ? t('auto.edit') : t('auto.new')}
        description={t('auto.dialogDesc')}
        onConfirm={() => void saveJob()}
        confirmLabel={saving ? t('auto.saving') : editingID ? t('auto.save') : t('auto.create')}
        loading={saving}
        wide
      >
        <div className="space-y-5 max-h-[60vh] overflow-y-auto pr-1">
          <div className="space-y-4">
            <div className="text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">{t('auto.form.required')}</div>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <LabeledField label={t('auto.form.jobId')}>
                <Input value={form.id} onChange={(e) => setForm((curr) => ({ ...curr, id: e.target.value }))} placeholder={t('auto.form.jobIdPlaceholder')} disabled={!!editingID} />
              </LabeledField>
              <LabeledField label={t('auto.form.displayName')}>
                <Input value={form.display_name} onChange={(e) => setForm((curr) => ({ ...curr, display_name: e.target.value }))} placeholder={t('auto.form.displayNamePlaceholder')} />
              </LabeledField>
              <LabeledField label={t('auto.form.type')}>
                <NativeSelect value={form.type} onChange={(e) => setForm((curr) => resetType(curr, e.target.value))}>
                  <option value="skill">{t('auto.form.typeSkill')}</option>
                  <option value="connector_capability">{t('auto.form.typeConnector')}</option>
                </NativeSelect>
              </LabeledField>
              <LabeledField label={t('auto.form.schedule')}>
                <Input list="automation-schedules" value={form.schedule} onChange={(e) => setForm((curr) => ({ ...curr, schedule: e.target.value }))} placeholder={t('auto.form.schedulePlaceholder')} />
                <datalist id="automation-schedules">
                  {schedulePresets.map((it) => <option key={it} value={it} />)}
                </datalist>
              </LabeledField>
            </div>
          </div>

          <LabeledField label={t('auto.form.description')}>
            <Textarea rows={2} value={form.description || ''} onChange={(e) => setForm((curr) => ({ ...curr, description: e.target.value }))} placeholder={t('auto.form.descPlaceholder')} />
          </LabeledField>

          {form.type === 'skill' ? (
            <div className="space-y-4">
              <div className="text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">{t('auto.form.skillTarget')}</div>
              <LabeledField label={t('auto.form.skill')}>
                <NativeSelect value={form.skill?.skill_id || ''} onChange={(e) => setForm((curr) => ({ ...curr, target_ref: e.target.value, skill: { skill_id: e.target.value, context: curr.skill?.context || {} } }))}>
                  <option value="">{t('auto.form.skillSelect')}</option>
                  {skills.map((s) => <option key={s.metadata.id} value={s.metadata.id}>{s.metadata.display_name || s.metadata.name || s.metadata.id}</option>)}
                </NativeSelect>
              </LabeledField>
              <LabeledField label={t('auto.form.skillContext')}>
                <Textarea rows={6} spellCheck={false} value={skillContextText} onChange={(e) => updateSkillContext(e.target.value)} placeholder='{"key": "value"}' />
                {skillContextError ? <FieldHint>{skillContextError}</FieldHint> : <FieldHint>{t('auto.form.skillContextHint')}</FieldHint>}
              </LabeledField>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">{t('auto.form.connectorTarget')}</div>
              <LabeledField label={t('auto.form.connectorCap')}>
                <NativeSelect value={form.target_ref} onChange={(e) => setForm((curr) => ({ ...curr, target_ref: e.target.value }))}>
                  <option value="">{t('auto.form.connectorCapSelect')}</option>
                  {capabilityChoices.map((c) => (
                    <option key={c.key} value={c.key} disabled={!c.read_only}>
                      {c.label}{c.read_only ? '' : ` ${t('auto.form.connectorCapBlocked')}`}
                    </option>
                  ))}
                </NativeSelect>
              </LabeledField>
              <LabeledField label={t('auto.form.connectorParams')}>
                <Textarea rows={6} spellCheck={false} value={connectorParamsText} onChange={(e) => updateConnectorParams(e.target.value)} placeholder='{"key": "value"}' />
                {connectorParamsError ? <FieldHint>{connectorParamsError}</FieldHint> : <FieldHint>{t('auto.form.connectorParamsHint')}</FieldHint>}
              </LabeledField>
            </div>
          )}

          <div className="space-y-4">
            <div className="text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">{t('auto.form.advanced')}</div>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <LabeledField label={t('auto.form.owner')}>
                <Input value={form.owner || ''} onChange={(e) => setForm((curr) => ({ ...curr, owner: e.target.value }))} placeholder={t('auto.form.ownerPlaceholder')} />
              </LabeledField>
              <LabeledField label={t('auto.form.agentRole')}>
                <NativeSelect value={form.agent_role_id || ''} onChange={(e) => setForm((curr) => ({ ...curr, agent_role_id: e.target.value || undefined }))}>
                  <option value="">{t('auto.form.agentRoleDefault')}</option>
                  {agentRoles.map((r) => <option key={r.role_id} value={r.role_id}>{r.display_name || r.role_id}</option>)}
                </NativeSelect>
              </LabeledField>
              <LabeledField label={t('auto.form.governance')}>
                <NativeSelect value={form.governance_policy || 'auto'} onChange={(e) => setForm((curr) => ({ ...curr, governance_policy: e.target.value }))}>
                  <option value="auto">{t('auto.form.governanceAuto')}</option>
                  <option value="approval_required">{t('auto.form.governanceApproval')}</option>
                  <option value="disabled">{t('auto.form.governanceDisabled')}</option>
                </NativeSelect>
              </LabeledField>
              <LabeledField label={t('auto.form.timeout')}>
                <Input type="number" min={5} max={3600} value={form.timeout_seconds || 30} onChange={(e) => setForm((curr) => ({ ...curr, timeout_seconds: Number(e.target.value) }))} />
              </LabeledField>
              <LabeledField label={t('auto.form.retryAttempts')}>
                <Input type="number" min={1} max={5} value={form.retry_max_attempts || 1} onChange={(e) => setForm((curr) => ({ ...curr, retry_max_attempts: Number(e.target.value) }))} />
              </LabeledField>
              <LabeledField label={t('auto.form.retryBackoff')}>
                <Input value={form.retry_initial_backoff || '2s'} onChange={(e) => setForm((curr) => ({ ...curr, retry_initial_backoff: e.target.value }))} placeholder="2s" />
              </LabeledField>
            </div>
            <label className="flex items-center gap-2 text-sm text-foreground">
              <input type="checkbox" checked={form.enabled} onChange={(e) => setForm((curr) => ({ ...curr, enabled: e.target.checked }))} />
              {t('auto.form.enableImmediate')}
            </label>
          </div>
        </div>
      </GuidedFormDialog>

      <Card className="flex flex-col gap-5 p-6">
        {error && <StatusMessage message={error} type="error" />}

        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <FilterBar
            search={{ value: query, onChange: setQuery, placeholder: t('auto.search') }}
            filters={[{
              key: 'type',
              value: type,
              onChange: (v) => setType(v),
              options: [
                { value: '', label: t('auto.filter.allTypes') },
                { value: 'skill', label: t('auto.filter.skill') },
                { value: 'connector_capability', label: t('auto.filter.connectorCapability') },
              ],
              className: 'md:w-56',
            }]}
          />
          <Button variant="amber" onClick={() => {
            const next = defaultAutomationJob();
            setEditingID(null);
            setForm(next);
            setSkillContextText(JSON.stringify(next.skill?.context || {}, null, 2));
            setConnectorParamsText(JSON.stringify(next.connector_capability?.params || {}, null, 2));
            setSkillContextError('');
            setConnectorParamsError('');
            setDialogOpen(true);
          }}>
            {t('auto.new')}
          </Button>
        </div>

        {loading ? (
          <div className="rounded-2xl border border-dashed border-border px-6 py-12 text-center text-muted-foreground">{t('auto.loading')}</div>
        ) : items.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-border px-6 py-12 text-center text-muted-foreground">{t('auto.empty')}</div>
        ) : (
          <div className="space-y-12">
            {/* Scheduled Section */}
            {items.filter(i => !!i.schedule).length > 0 && (
              <div className="space-y-4">
                <div className="flex items-center gap-3">
                  <div className="h-px flex-1 bg-white/5" />
                  <div className="text-[0.65rem] font-black uppercase tracking-[0.2em] text-muted-foreground opacity-50">{t('auto.section.scheduled')}</div>
                  <div className="h-px flex-1 bg-white/5" />
                </div>
                <div className="grid gap-4">
                  {items.filter(i => !!i.schedule).map((item) => renderAutomationCard(item))}
                </div>
              </div>
            )}

            {/* Event-Driven Section */}
            {items.filter(i => !i.schedule && triggers.some(t => t.automation_job_id === i.id)).length > 0 && (
              <div className="space-y-4">
                <div className="flex items-center gap-3">
                  <div className="h-px flex-1 bg-white/5" />
                  <div className="text-[0.65rem] font-black uppercase tracking-[0.2em] text-muted-foreground opacity-50">{t('auto.section.eventDriven')}</div>
                  <div className="h-px flex-1 bg-white/5" />
                </div>
                <div className="grid gap-4">
                  {items.filter(i => !i.schedule && triggers.some(t => t.automation_job_id === i.id)).map((item) => renderAutomationCard(item))}
                </div>
              </div>
            )}

            {/* Manual / Unassigned Section */}
            {items.filter(i => !i.schedule && !triggers.some(t => t.automation_job_id === i.id)).length > 0 && (
              <div className="space-y-4">
                <div className="flex items-center gap-3">
                  <div className="h-px flex-1 bg-white/5" />
                  <div className="text-[0.65rem] font-black uppercase tracking-[0.2em] text-muted-foreground opacity-50">{t('auto.section.manual')}</div>
                  <div className="h-px flex-1 bg-white/5" />
                </div>
                <div className="grid gap-4">
                  {items.filter(i => !i.schedule && !triggers.some(t => t.automation_job_id === i.id)).map((item) => renderAutomationCard(item))}
                </div>
              </div>
            )}
          </div>
        )}

        <div className="text-sm text-muted-foreground">
          {t('auto.crosslink')} <Link to="/executions" className="text-primary hover:underline">{t('auto.crosslink.executions')}</Link>{t('auto.crosslink.and', ' and ')}<Link to="/audit" className="text-primary hover:underline">{t('auto.crosslink.audit')}</Link> {t('auto.crosslink.suffix')}
        </div>
      </Card>
    </div>
  );

  function updateSkillContext(value: string) {
    setSkillContextText(value);
    const parsed = parseJsonObject(value);
    if (parsed.error) {
      setSkillContextError(parsed.error);
      return;
    }
    setSkillContextError('');
    setForm((curr) => ({
      ...curr,
      skill: { skill_id: curr.skill?.skill_id || '', context: parsed.value },
    }));
  }

  function updateConnectorParams(value: string) {
    setConnectorParamsText(value);
    const parsed = parseJsonObject(value);
    if (parsed.error) {
      setConnectorParamsError(parsed.error);
      return;
    }
    setConnectorParamsError('');
    setForm((curr) => ({
      ...curr,
      connector_capability: {
        connector_id: curr.connector_capability?.connector_id || selectedCapability?.connector_id || '',
        capability_id: curr.connector_capability?.capability_id || selectedCapability?.capability_id || '',
        params: parsed.value,
      },
    }));
  }
};

function defaultAutomationJob(): AutomationJob {
  return {
    id: '', display_name: '', description: '', agent_role_id: '',
    governance_policy: 'auto', type: 'skill', target_ref: '', schedule: '@every 15m',
    enabled: true, owner: 'platform-ops', timeout_seconds: 30, retry_max_attempts: 1,
    retry_initial_backoff: '2s', skill: { skill_id: '', context: {} },
    connector_capability: { connector_id: '', capability_id: '', params: {} },
  };
}

function resetType(current: AutomationJob, nextType: string): AutomationJob {
  if (nextType === 'connector_capability') {
    return { ...current, type: nextType, target_ref: '', connector_capability: { connector_id: '', capability_id: '', params: {} } };
  }
  return { ...current, type: 'skill', target_ref: '', skill: { skill_id: '', context: {} } };
}

function normalizeForm(job: AutomationJob, connectorID?: string, capabilityID?: string): AutomationJob {
  if (job.type === 'connector_capability') {
    return {
      ...job,
      connector_capability: {
        connector_id: connectorID || job.connector_capability?.connector_id || '',
        capability_id: capabilityID || job.connector_capability?.capability_id || '',
        params: job.connector_capability?.params || {},
      },
    };
  }
  return {
    ...job,
    target_ref: job.skill?.skill_id || job.target_ref,
    skill: { skill_id: job.skill?.skill_id || '', context: job.skill?.context || {} },
  };
}

function parseJsonObject(raw: string): { value: Record<string, unknown>; error?: string } {
  try {
    const parsed = JSON.parse(raw || '{}');
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return { value: parsed as Record<string, unknown> };
    return { value: {}, error: 'JSON must be an object.' };
  } catch (err) {
    return { value: {}, error: err instanceof Error ? err.message : 'Invalid JSON.' };
  }
}

function isDueSoon(val?: string): boolean {
  if (!val) return false;
  const next = new Date(val).getTime();
  if (Number.isNaN(next)) return false;
  const diff = next - Date.now();
  return diff >= 0 && diff <= 15 * 60 * 1000;
}

function formatTime(val?: string, t?: (k: string) => string): string {
  if (!val) return t ? t('auto.card.notScheduled') : 'not scheduled';
  return new Date(val).toLocaleString();
}

const StatItem = ({ label, value, accent, warning }: { label: string; value: string; accent?: boolean; warning?: boolean }) => (
  <div className="flex flex-col gap-0.5">
    <div className="text-[0.6rem] font-bold uppercase tracking-wider text-muted-foreground opacity-60">{label}</div>
    <div className={cn("text-xs font-bold", accent ? "text-danger" : warning ? "text-warning" : "text-foreground")}>{value}</div>
  </div>
);
