import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPerson, fetchAccessConfig, fetchPeople, fetchPerson, setPersonEnabled, updatePerson } from '../../lib/api/access';
import { getApiErrorMessage } from '../../lib/api/ops';
import type { AccessChannel, AccessConfigResponse, AccessPerson, PersonListResponse } from '../../lib/api/types';
import {
  RegistryDetail,
  RegistrySidebar,
  RegistryCard,
  RegistryPanel,
  SplitLayout,
} from '@/components/ui/registry-primitives';
import { formatKeyValueText, joinCSV, parseKeyValueText, previewList, splitCSV } from './registry-utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { NativeSelect } from '@/components/ui/select';
import { ActiveBadge as StatusBadge } from '@/components/ui/active-badge';
import { cn } from '@/lib/utils';
import { DetailHeader } from '@/components/ui/detail-header';
import { EmptyDetailState } from '@/components/ui/empty-detail-state';
import { FieldHint } from '@/components/ui/field-hint';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { LabeledField } from '@/components/ui/labeled-field';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { PaginationControls } from '@/components/list/PaginationControls';
import { useI18n } from '@/hooks/useI18n';

const defaultPerson = (): AccessPerson => ({
  id: '',
  display_name: '',
  email: '',
  status: 'active',
  linked_user_id: '',
  channel_ids: [],
  team: '',
  approval_target: '',
  oncall_schedule: '',
  preferences: {},
});

export const PeoplePage = () => {
  const { t } = useI18n();
  const [items, setItems] = useState<AccessPerson[]>([]);
  const [pageMeta, setPageMeta] = useState<Pick<PersonListResponse, 'page' | 'limit' | 'total' | 'has_next'>>({ page: 1, limit: 20, total: 0, has_next: false });
  const [config, setConfig] = useState<AccessConfigResponse | null>(null);
  const [selectedID, setSelectedID] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [form, setForm] = useState<AccessPerson>(defaultPerson());
  const [preferencesText, setPreferencesText] = useState('');
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'warning' | 'error'; text: string } | null>(null);

  const isCreatingRef = useRef(false);
  const selectedIDRef = useRef('');
  const setIsCreatingSync = (val: boolean) => { isCreatingRef.current = val; setIsCreating(val); };
  const setSelectedIDSync = (val: string) => { selectedIDRef.current = val; setSelectedID(val); };

  const availableChannels = useMemo<AccessChannel[]>(() => config?.config.channels || [], [config]);

  const loadDetail = useCallback(async (personID: string) => {
    if (!personID) return;
    try {
      setDetailLoading(true);
      const detail = await fetchPerson(personID);
      setIsCreatingSync(false);
      setSelectedIDSync(personID);
      setForm({ ...defaultPerson(), ...detail });
      setPreferencesText(formatKeyValueText(detail.preferences));
    } catch (error) {
      setMessage({ type: 'error', text: getApiErrorMessage(error, t('identity.people.detail.loadDetailFailed')) });
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const load = useCallback(async (preferredID?: string) => {
    try {
      setLoading(true);
      setMessage(null);
      const [peopleResp, accessConfig] = await Promise.all([
        fetchPeople({ q: query || undefined, page, limit }),
        fetchAccessConfig(),
      ]);
      const nextItems = peopleResp.items || [];
      setItems(nextItems);
      setPageMeta({ page: peopleResp.page, limit: peopleResp.limit, total: peopleResp.total, has_next: peopleResp.has_next });
      setConfig(accessConfig);

      const nextID = preferredID || (isCreatingRef.current ? '' : selectedIDRef.current) || nextItems[0]?.id || '';
      if (!nextID) {
        if (!isCreatingRef.current) {
          setSelectedIDSync('');
          setForm(defaultPerson());
          setPreferencesText('');
        }
        return;
      }
      await loadDetail(nextID);
    } catch (error) {
      setMessage({ type: 'error', text: getApiErrorMessage(error, t('identity.people.detail.loadListFailed')) });
    } finally {
      setLoading(false);
    }
  }, [limit, loadDetail, page, query]);

  useEffect(() => { void load(); }, [load]);

  const filteredItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return items;
    return items.filter((item) =>
      [item.id, item.display_name, item.email, item.team, item.approval_target, item.oncall_schedule, item.linked_user_id, ...(item.channel_ids || [])]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(needle)),
    );
  }, [items, query]);

  const startCreate = () => {
    setIsCreatingSync(true);
    setSelectedIDSync('');
    setForm(defaultPerson());
    setPreferencesText('');
    setMessage(null);
  };

  const handleSave = async () => {
    if (!form.id?.trim()) { setMessage({ type: 'error', text: t('identity.people.detail.idRequired') }); return; }
    try {
      setSaving(true);
      setMessage(null);
      const payload: AccessPerson = {
        ...form,
        id: form.id.trim(),
        display_name: form.display_name?.trim(),
        email: form.email?.trim(),
        linked_user_id: form.linked_user_id?.trim(),
        channel_ids: form.channel_ids || [],
        team: form.team?.trim(),
        approval_target: form.approval_target?.trim(),
        oncall_schedule: form.oncall_schedule?.trim(),
        preferences: parseKeyValueText(preferencesText),
      };
      const saved = selectedID ? await updatePerson(selectedID, payload) : await createPerson(payload);
      setMessage({ type: 'success', text: t('status.success') });
      await load(saved.id || payload.id);
    } catch (error) {
      setMessage({ type: 'error', text: getApiErrorMessage(error, t('status.error')) });
    } finally {
      setSaving(false);
    }
  };

  const handleToggle = async () => {
    if (!form.id) return;
    try {
      setToggling(true);
      setMessage(null);
      await setPersonEnabled(form.id, (form.status || 'active') !== 'active');
      setMessage({ type: 'success', text: t('status.success') });
      await load(form.id);
    } catch (error) {
      setMessage({ type: 'error', text: getApiErrorMessage(error, t('status.error')) });
    } finally {
      setToggling(false);
    }
  };

  const missingRouting = !form.approval_target && !form.oncall_schedule && !form.channel_ids?.length;
  const showDetail = isCreating || Boolean(selectedID) || Boolean(form.id);

  return (
    <div className="animate-fade-in grid gap-6">
      <SectionTitle title={t('identity.people.title')} subtitle={t('identity.people.subtitle')} />

      <SummaryGrid>
        <StatCard title={t('identity.people.stats.total')} value={String(pageMeta.total)} subtitle={t('identity.people.stats.totalDesc')} />
        <StatCard title={t('identity.people.stats.active')} value={String(items.filter((item) => (item.status || 'active') === 'active').length)} subtitle={t('identity.people.stats.activeDesc')} />
        <StatCard title={t('identity.people.stats.linked')} value={String(items.filter((item) => item.linked_user_id).length)} subtitle={t('identity.people.stats.linkedDesc')} />
        <StatCard title={t('identity.people.stats.channels')} value={String(availableChannels.length)} subtitle={t('identity.people.stats.channelsDesc')} />
      </SummaryGrid>

      {message ? <StatusMessage message={message.text} type={message.type} /> : null}

      <SplitLayout
        sidebar={
          <RegistrySidebar>
            <div className="flex flex-wrap items-center justify-between gap-3 p-4 pb-0">
                <h2 className="text-lg font-bold text-foreground">{t('identity.people.sidebar.title')}</h2>
                  <Button variant="amber" size="sm" type="button" onClick={startCreate}>{t('identity.people.sidebar.new')}</Button>
            </div>
            <div className="px-4 pt-4">
              <Input placeholder={t('identity.people.sidebar.search')} value={query} onChange={(event) => { setQuery(event.target.value); setPage(1); }} />
            </div>
            {loading ? <div className="p-10 text-center text-muted-foreground animate-pulse">{t('common.loading')}</div> : (
              <RegistryPanel title={t('identity.people.sidebar.panelTitle')} emptyText={t('identity.people.sidebar.empty')} className="px-4">
                <div className="flex flex-col gap-2">
                  {filteredItems.map((item) => (
                    <button key={item.id} type="button" onClick={() => void loadDetail(item.id || '')} className={cn("w-full text-left focus:outline-none rounded-2xl transition-all", selectedID === item.id ? "ring-2 ring-primary" : "")}>
                       <RegistryCard
                        title={item.display_name || item.id || t('common.unknownUser')}
                        subtitle={item.id || 'person'}
                        lines={[
                          `${t('identity.people.detail.team')}: ${item.team || t('common.na')}`,
                          `${t('identity.people.detail.linkedUser')}: ${item.linked_user_id || t('common.na')}`,
                          `${t('identity.people.detail.approvalTarget')}: ${item.approval_target || t('common.na')}`,
                          `${t('identity.people.sidebar.channels')}: ${previewList(item.channel_ids)}`,
                        ]}
                        status={<StatusBadge active={(item.status || 'active') === 'active'} label={item.status || 'active'} />}
                      />
                    </button>
                  ))}
                </div>
              </RegistryPanel>
            )}
            <div className="p-4 border-t border-border mt-auto">
              <PaginationControls page={pageMeta.page} limit={pageMeta.limit} total={pageMeta.total} hasNext={pageMeta.has_next} onPageChange={setPage} onLimitChange={(next) => { setLimit(next); setPage(1); }} limitOptions={[10, 20, 50]} />
            </div>
          </RegistrySidebar>
        }
        detail={showDetail ? (
          <RegistryDetail>
            <DetailHeader
              title={selectedID ? (form.display_name || form.id || t('identity.people.detail.title')) : t('identity.people.detail.create')}
              subtitle={selectedID ? 'Org-owned routing profile' : t('identity.people.detail.provision')}
              status={<StatusBadge active={(form.status || 'active') === 'active'} label={form.status || 'active'} />}
              actions={selectedID ? (
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" type="button" onClick={startCreate}>{t('identity.people.detail.duplicate')}</Button>
                  <Button variant="secondary" size="sm" type="button" onClick={() => void handleToggle()} disabled={toggling}>
                    {toggling ? '...' : (form.status || 'active') === 'active' ? t('action.disable') : t('action.enable')}
                  </Button>
                </div>
              ) : undefined}
            />

            <div className="p-6 space-y-8">
              {detailLoading && <div className="absolute inset-0 bg-background/40 backdrop-blur-[1px] z-50 flex items-center justify-center rounded-2xl font-bold uppercase tracking-widest text-muted-foreground">{t('common.loadingCaps')}</div>}
              
              {!config?.configured && <StatusMessage type="warning" message={t('identity.people.detail.memoryWarning')} />}
              {missingRouting && <StatusMessage type="warning" message={t('identity.people.detail.missingRouting')} />}

              <div className="grid gap-6 md:grid-cols-2">
                <LabeledField label={t('identity.people.detail.id')} required>
                  <Input value={form.id || ''} onChange={(event) => setForm((c) => ({ ...c, id: event.target.value }))} disabled={Boolean(selectedID)} placeholder="alice" />
                </LabeledField>
                <LabeledField label={t('identity.people.detail.displayName')} required>
                  <Input value={form.display_name || ''} onChange={(event) => setForm((c) => ({ ...c, display_name: event.target.value }))} placeholder="Alice" />
                </LabeledField>
                <LabeledField label={t('identity.people.detail.email')}>
                  <Input value={form.email || ''} onChange={(event) => setForm((c) => ({ ...c, email: event.target.value }))} placeholder="alice@example.com" />
                </LabeledField>
                <LabeledField label={t('identity.people.detail.linkedUser')}>
                  <Input value={form.linked_user_id || ''} onChange={(event) => setForm((c) => ({ ...c, linked_user_id: event.target.value }))} placeholder="alice-user" />
                </LabeledField>
              </div>

              <div className="grid gap-6 md:grid-cols-2">
                <LabeledField label={t('identity.people.detail.team')}>
                  <Input value={form.team || ''} onChange={(event) => setForm((c) => ({ ...c, team: event.target.value }))} placeholder="sre" />
                </LabeledField>
                <LabeledField label={t('identity.people.detail.approvalTarget')}>
                  <Input value={form.approval_target || ''} onChange={(event) => setForm((c) => ({ ...c, approval_target: event.target.value }))} placeholder="service_owner:sshd" />
                </LabeledField>
                <LabeledField label={t('identity.people.detail.oncallSchedule')}>
                  <Input value={form.oncall_schedule || ''} onChange={(event) => setForm((c) => ({ ...c, oncall_schedule: event.target.value }))} placeholder="pagerduty:sre-primary" />
                </LabeledField>
                <LabeledField label={t('identity.people.detail.status')}>
                  <NativeSelect value={form.status || 'active'} onChange={(event) => setForm((c) => ({ ...c, status: event.target.value }))}>
                    <option value="active">active</option>
                    <option value="disabled">disabled</option>
                  </NativeSelect>
                </LabeledField>
              </div>

              <LabeledField label={t('identity.people.detail.channels')}>
                <div className="grid gap-3">
                  <Input value={joinCSV(form.channel_ids)} onChange={(event) => setForm((c) => ({ ...c, channel_ids: splitCSV(event.target.value) }))} placeholder="telegram-main, slack-oncall" />
                  <FieldHint>{t('identity.people.detail.channelsHint')}</FieldHint>
                  <div className="flex flex-wrap gap-2">
                    {availableChannels.map((channel) => {
                      const channelID = channel.id || '';
                      const picked = (form.channel_ids || []).includes(channelID);
                      return (
                        <Button key={channelID} type="button" variant={picked ? "amber" : "outline"} size="sm"
                          onClick={() => setForm((c) => ({
                            ...c,
                            channel_ids: picked ? (c.channel_ids || []).filter((item) => item !== channelID) : [...(c.channel_ids || []), channelID],
                          }))}
                          className="h-8 text-xs font-bold rounded-full">
                          {channel.name || channelID}
                        </Button>
                      );
                    })}
                  </div>
                  {availableChannels.length === 0 && <p className="text-xs text-muted-foreground italic">{t('identity.people.detail.noChannels')}</p>}
                </div>
              </LabeledField>

              <LabeledField label={t('identity.people.detail.preferences')}>
                <Textarea rows={6} value={preferencesText} onChange={(event) => setPreferencesText(event.target.value)} placeholder={"timezone=Asia/Shanghai\nlocale=zh-CN\nescalation=primary"} className="font-mono text-xs rounded-xl border-border bg-muted/20" />
                <FieldHint>{t('identity.people.detail.preferencesHint')}</FieldHint>
              </LabeledField>

              <div className="flex flex-wrap gap-3 pt-4 border-t border-border">
                <Button variant="amber" type="button" onClick={() => void handleSave()} disabled={saving}>{saving ? '...' : selectedID ? t('identity.people.detail.save') : t('identity.people.detail.create')}</Button>
                <Button variant="outline" type="button" onClick={startCreate}>{t('identity.people.detail.reset')}</Button>
              </div>
            </div>
          </RegistryDetail>
        ) : (
          <EmptyDetailState title={t('identity.people.empty.title')} description={t('identity.people.empty.desc')} />
        )}
      />
    </div>
  );
};

export default PeoplePage;
