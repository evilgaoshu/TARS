import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import { createChannel, fetchAccessConfig, fetchChannel, fetchChannels, setChannelEnabled, updateChannel } from '../../lib/api/access';
import type { AccessChannel, AccessConfigResponse, AccessUser, ChannelListResponse } from '../../lib/api/types';
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useNotify } from '@/hooks/ui/useNotify';
import { joinCSV, splitCSV } from '../identity/registry-utils';
import { Plus, Power, RadioTower, Users, ShieldCheck, List, Pencil, ToggleLeft, ToggleRight } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { PaginationControls } from '@/components/list/PaginationControls';
import { InlineStatus } from '@/components/ui/inline-status';
import { StatusBadge } from '@/components/ui/status-badge';
import { CollapsibleList } from '@/components/ui/collapsible-list';
import { GuidedFormDialog } from '@/components/operator/GuidedFormDialog';
import { OperatorHero, OperatorStats } from '@/components/operator/OperatorPage';
import { useI18n } from '@/hooks/useI18n';
import { Checkbox } from '@/components/ui/checkbox';

import { useCapabilities } from '../../lib/FeatureGateContext';
import { getStatus } from '../../lib/featureGates';

const channelSchema = z.object({
  id: z.string().min(1, 'Channel ID is required'),
  name: z.string().min(1, 'Display Name is required'),
  kind: z.string().min(1, 'Kind is required'),
  target: z.string().min(1, 'Target is required'),
  enabled: z.boolean(),
  linked_users: z.array(z.string()),
  usages: z.array(z.string()),
  capabilities: z.array(z.string()),
});

type ChannelFormValues = z.infer<typeof channelSchema>;

const channelKindOptions = [
  { value: 'in_app_inbox', label: 'In-app Inbox', capability: 'channels.inbox' as const, showWhen: 'always' as const },
  { value: 'telegram', label: 'Telegram', capability: 'channels.telegram' as const, showWhen: 'always' as const },
  { value: 'slack', label: 'Slack', capability: 'channels.slack' as const, showWhen: 'always' as const },
  { value: 'web_chat', label: 'Web Chat', capability: 'channels.inbox' as const, showWhen: 'always' as const },
  { value: 'email', label: 'Email', capability: 'channels.email' as const, showWhen: 'always' as const },
];
const channelUsageOptions = ['approval', 'notifications', 'conversation_entry', 'alerts'];
const channelCapabilityOptions = ['supports_session_reply', 'attachments', 'rich_content', 'file_upload'];

const FIRST_PARTY_KINDS = new Set(['in_app_inbox', 'web_chat', 'web']);

const usageFriendlyLabel: Record<string, string> = {
  approval: 'Approval',
  notifications: 'Notifications',
  conversation_entry: 'Conversation Entry',
  alerts: 'Alerts',
};

const capabilityFriendlyLabel: Record<string, string> = {
  supports_session_reply: 'Session Reply',
  attachments: 'Attachments',
  rich_content: 'Rich Content',
  file_upload: 'File Upload',
};

function friendlyLabel(map: Record<string, string>, token: string): string {
  return map[token] ?? token.replaceAll('_', ' ');
}

function toggleStringSelection(values: string[], value: string, checked: boolean): string[] {
  if (checked) {
    return values.includes(value) ? values : [...values, value];
  }
  return values.filter((item) => item !== value);
}

export const ChannelsPage = () => {
  const notify = useNotify();
  const { t } = useI18n();

  const [items, setItems] = useState<AccessChannel[]>([]);
  const [pageMeta, setPageMeta] = useState<Pick<ChannelListResponse, 'page' | 'limit' | 'total' | 'has_next'>>({ page: 1, limit: 20, total: 0, has_next: false });
  const [config, setConfig] = useState<AccessConfigResponse | null>(null);
  const [selectedID, setSelectedID] = useState('');

  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [toggling, setToggling] = useState('');

  // modal state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);

  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(channelSchema),
    defaultValues: {
      id: '',
      name: '',
      kind: 'in_app_inbox',
      target: '',
      enabled: true,
      linked_users: [],
      usages: ['approval', 'notifications'],
      capabilities: ['supports_session_reply'],
    },
  });

  const selectedIDRef = useRef('');
  const setSelectedIDSync = (val: string) => { selectedIDRef.current = val; setSelectedID(val); };

  const knownUsers = useMemo<AccessUser[]>(() => config?.config.users || [], [config]);

  const load = useCallback(async (preferredID?: string) => {
    try {
      setLoading(true);
      const [channelsResp, accessConfig] = await Promise.all([
        fetchChannels({ q: query || undefined, page, limit }),
        fetchAccessConfig(),
      ]);
      const nextItems = channelsResp.items || [];
      setItems(nextItems);
      setPageMeta({ page: channelsResp.page, limit: channelsResp.limit, total: channelsResp.total, has_next: channelsResp.has_next });
      setConfig(accessConfig);
      const nextID = preferredID || selectedIDRef.current || '';
      setSelectedIDSync(nextID);
    } catch (error) {
      notify.error(error, t('channels.loadFailed', 'Failed to load channels'));
    } finally {
      setLoading(false);
    }
  }, [limit, page, query, notify, t]);

  useEffect(() => { void load(); }, [load]);

  const filteredItems = useMemo(() => {
    const needle = (query || '').trim().toLowerCase();
    if (!needle) return items || [];
    return (items || []).filter((item) => {
      if (!item) return false;
        const searchFields = [item.id, item.name, item.kind, item.type, item.target, ...(item.usages || []), ...(item.capabilities || []), ...(item.linked_users || [])];
        return searchFields.filter((val): val is string => typeof val === 'string' && val.length > 0).some((value) => value.toLowerCase().includes(needle));
      });
  }, [items, query]);

  const openCreate = () => {
    setIsCreating(true);
    form.reset({ id: '', name: '', kind: 'in_app_inbox', target: '', enabled: true, linked_users: [], usages: ['approval', 'notifications'], capabilities: ['supports_session_reply'] });
    setDialogOpen(true);
  };

  const openEdit = async (channelID: string) => {
    try {
      const detail = await fetchChannel(channelID);
      setIsCreating(false);
      setSelectedIDSync(channelID);
      form.reset({
        id: detail.id,
        name: detail.name || '',
        kind: detail.kind || detail.type || 'in_app_inbox',
        target: detail.target || '',
        enabled: detail.enabled ?? true,
        linked_users: detail.linked_users || [],
        usages: detail.usages || ['approval', 'notifications'],
        capabilities: detail.capabilities || ['supports_session_reply'],
      });
      setDialogOpen(true);
    } catch (error) {
      notify.error(error, t('channels.loadDetailFailed', 'Failed to load channel details'));
    }
  };

  const { capabilities } = useCapabilities();
  const filteredKindOptions = useMemo(() => {
    // Show all channel kinds, but mark those that require configuration
    return channelKindOptions.map(o => {
      if (!o.capability) return o;
      const status = getStatus(capabilities, o.capability);
      return {
        ...o,
        label: status === 'requires_config' ? `${o.label} (requires config)` : o.label,
        disabled: status === 'coming_soon',
      };
    });
  }, [capabilities]);

  const onSave = async (values: ChannelFormValues) => {
    try {
      setSaving(true);
      const payload: AccessChannel = {
        ...values,
        kind: values.kind,
        type: values.kind,
        usages: values.usages,
        capabilities: values.capabilities,
      };
      const saved = selectedID && !isCreating ? await updateChannel(selectedID, payload) : await createChannel(payload);
      notify.success(isCreating ? t('channels.created', 'Channel created') : t('channels.updated', 'Channel updated'));
      setDialogOpen(false);
      await load(saved.id || values.id);
    } catch (error) {
      notify.error(error, t('channels.saveFailed', 'Failed to save channel'));
    } finally {
      setSaving(false);
    }
  };

  const handleToggle = async (item: AccessChannel) => {
    if (!item.id) return;
    try {
      setToggling(item.id);
      await setChannelEnabled(item.id, !item.enabled);
      notify.success(item.enabled ? t('channels.disabled', 'Channel disabled') : t('channels.enabled', 'Channel enabled'));
      await load(selectedID);
    } catch {
      notify.error(null, t('channels.toggleFailed', 'Failed to update channel state'));
    } finally {
      setToggling('');
    }
  };

  const watchedUsers = form.watch('linked_users');

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <OperatorHero
        eyebrow={t('channels.hero.eyebrow', 'Delivery channels')}
        title={t('channels.hero.title', 'Channels')}
        description={t('channels.hero.description', 'Manage first-party Inbox / Web Chat and external Telegram, Slack delivery endpoints in one place.')}
        chips={[
          { label: t('channels.firstParty', 'first-party'), tone: 'info' },
          { label: t('channels.external', 'external'), tone: 'muted' },
        ]}
        primaryAction={
          <Button variant="amber" size="sm" onClick={openCreate}><Plus size={14} />{t('channels.new', 'New Channel')}</Button>
        }
      />

      <OperatorStats
        stats={[
          { title: t('channels.stats.total', 'Channels'), value: pageMeta.total, description: t('channels.stats.totalDesc', 'Total registered'), icon: RadioTower, tone: 'info' },
          { title: t('channels.stats.enabled', 'Enabled'), value: items.filter(i => i.enabled).length, description: t('channels.stats.enabledDesc', 'Currently active'), icon: Power, tone: 'success' },
          { title: t('channels.stats.linked', 'Linked'), value: items.filter(i => i.linked_users?.length).length, description: t('channels.stats.linkedDesc', 'With user bindings'), icon: Users, tone: 'muted' },
          { title: t('channels.stats.config', 'Config'), value: config?.path ? t('channels.config.file', 'File') : t('channels.config.memory', 'Memory'), description: config?.path || 'in-memory', icon: ShieldCheck, tone: config?.path ? 'success' : 'warning' },
        ]}
      />

      <Card className="p-0">
        <CardHeader className="border-b border-border bg-white/[0.02]">
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle>{t('channels.registryTitle', 'Channel Registry')}</CardTitle>
              <CardDescription>{t('channels.registrySubtitle', 'First-party and external delivery endpoints')}</CardDescription>
            </div>
            <Button size="sm" onClick={openCreate}><Plus size={14} />{t('common.create', 'New')}</Button>
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-4 p-5">
          <Input placeholder={t('channels.search', 'Search channels…')} value={query} onChange={(e) => { setQuery(e.target.value); setPage(1); }} />

          {loading ? (
            <div className="rounded-2xl border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">
              {t('channels.loading', 'Loading channels…')}
            </div>
          ) : filteredItems.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">
              {t('channels.empty', 'No channels. Click "New Channel" to get started.')}
            </div>
          ) : (
            <div className="flex flex-col gap-6">
              {[
                { label: t('channels.group.firstParty', 'First-party'), items: filteredItems.filter((i) => FIRST_PARTY_KINDS.has(i.kind || i.type || '')) },
                { label: t('channels.group.external', 'External'), items: filteredItems.filter((i) => !FIRST_PARTY_KINDS.has(i.kind || i.type || '')) },
              ]
                .filter((group) => group.items.length > 0)
                .map((group) => (
                  <div key={group.label} className="flex flex-col gap-3">
                    <div className="text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">{group.label}</div>
                    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      {group.items.map((item) => {
                        const allUsages = [...(item.usages || []), ...(item.capabilities || [])];
                        const friendlyUsages = allUsages.map((token) =>
                          friendlyLabel(usageFriendlyLabel, token) || friendlyLabel(capabilityFriendlyLabel, token)
                        );
                        return (
                          <div
                            key={item.id}
                            className="group rounded-2xl border border-border bg-white/[0.03] p-4 transition-all hover:bg-white/[0.05]"
                          >
                            <div className="flex items-start justify-between gap-2 mb-3">
                              <div className="min-w-0">
                                <div className="truncate text-sm font-semibold text-foreground">{item.name || item.id}</div>
                                <div className="mt-0.5 font-mono text-[0.68rem] text-muted-foreground truncate">{item.id}</div>
                              </div>
                              <StatusBadge status={item.enabled ? 'enabled' : 'disabled'} />
                            </div>

                            <div className="space-y-1 text-xs text-muted-foreground mb-3">
                              <div className="flex items-center gap-1.5">
                                <Badge variant="outline" className="text-[0.6rem] px-1.5 py-0 rounded-full">{item.kind || item.type || 'unknown'}</Badge>
                                <span className="truncate">{item.target || t('channels.noTarget', 'no target')}</span>
                              </div>
                              {friendlyUsages.length > 0 && (
                                <div className="flex flex-wrap gap-1 pt-0.5">
                                  {friendlyUsages.slice(0, 4).map((label) => (
                                    <Badge key={label} variant="secondary" className="text-[0.6rem] px-1.5 py-0 rounded-full">{label}</Badge>
                                  ))}
                                  {friendlyUsages.length > 4 && (
                                    <span className="text-[0.6rem] text-muted-foreground">+{friendlyUsages.length - 4}</span>
                                  )}
                                </div>
                              )}
                            </div>

                            <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                              <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => void openEdit(item.id || '')}>
                                <Pencil size={11} />{t('action.edit')}
                              </Button>
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 text-xs"
                                disabled={toggling === item.id}
                                onClick={() => void handleToggle(item)}
                              >
                                {item.enabled ? <ToggleRight size={11} className="text-success" /> : <ToggleLeft size={11} />}
                                {item.enabled ? t('action.disable', 'Disable') : t('action.enable', 'Enable')}
                              </Button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ))}
            </div>
          )}

          <PaginationControls
            page={pageMeta.page}
            limit={pageMeta.limit}
            total={pageMeta.total}
            hasNext={pageMeta.has_next}
            onPageChange={setPage}
            onLimitChange={(next) => { setLimit(next); setPage(1); }}
          />
        </CardContent>
      </Card>

      {/* ── Create / Edit Modal ── */}
      <GuidedFormDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={isCreating ? t('channels.new', 'New Channel') : t('channels.edit', 'Edit Channel')}
        description={isCreating
          ? t('channels.createDesc', 'Fill in channel details to create a new delivery endpoint.')
          : t('channels.editDesc', 'Update channel routing and user bindings.')
        }
        onConfirm={form.handleSubmit(onSave)}
        confirmLabel={isCreating ? t('common.create', 'Create') : t('common.save', 'Save')}
        cancelLabel={t('common.cancel')}
        loading={saving}
        wide
      >
        <Form {...form}>
          <form className="flex flex-col gap-5">
            {!form.watch('target') && !isCreating && (
              <InlineStatus type="warning" message={t('channels.targetMissing', 'Channel has no target; delivery will fail.')} />
            )}
            {!watchedUsers?.length && !isCreating && (
              <InlineStatus type="info" message={t('channels.usersMissing', 'No users linked; channel is broadcast-only.')} />
            )}

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField control={form.control} name="id" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.channelId', 'Channel ID')}</FormLabel>
                  <FormControl><Input {...field} disabled={Boolean(selectedID) && !isCreating} placeholder="telegram-main" /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="name" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('common.name', 'Display Name')}</FormLabel>
                  <FormControl><Input {...field} placeholder="Telegram SRE Center" /></FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="kind" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('common.type', 'Kind')}</FormLabel>
                  <FormControl>
                     <NativeSelect className="bg-bg-surface-solid" {...field}>
                      {filteredKindOptions.map(o => (
                        <option key={o.value} value={o.value} disabled={(o as { disabled?: boolean }).disabled}>
                          {o.label}
                        </option>
                      ))}
                    </NativeSelect>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="target" render={({ field }) => {
                const kind = form.watch('kind');
                const isTelegram = kind === 'telegram';
                const isInbox = kind === 'in_app_inbox' || kind === 'web_chat';
                const telegramStatus = getStatus(capabilities, 'channels.telegram');
                const telegramReady = telegramStatus === 'available';
                return (
                  <FormItem>
                    <FormLabel>{t('channels.target', 'Target')}</FormLabel>
                    <FormControl>
                      <Input 
                        {...field} 
                        placeholder={isTelegram ? '-1001234567890' : isInbox ? 'default' : 'webhook_url / endpoint'} 
                      />
                    </FormControl>
                    <FormDescription className="text-xs">
                      {isTelegram 
                        ? telegramReady 
                          ? 'Telegram bot is configured. Enter the destination chat ID, for example -10012345.' 
                          : 'Telegram chat ID, for example -10012345. ⚠️ Bot token must be configured in server first (TARS_TELEGRAM_BOT_TOKEN).'
                        : isInbox 
                          ? 'Use "default" for the primary inbox, or a specific inbox ID.' 
                          : 'Protocol-specific endpoint identifier'}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                );
              }} />
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField control={form.control} name="usages" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.capabilitiesLabel', 'Usages')}</FormLabel>
                  <FormControl>
                    <div className="grid grid-cols-1 gap-2 rounded-md border border-border p-3">
                      {channelUsageOptions.map((option) => {
                        const checked = field.value.includes(option)
                        return (
                          <label key={option} className="flex items-center gap-3 text-sm text-foreground">
                            <Checkbox
                              checked={checked}
                              aria-label={option}
                              onCheckedChange={(value) => field.onChange(toggleStringSelection(field.value, option, value === true))}
                            />
                            <span>{friendlyLabel(usageFriendlyLabel, option)}</span>
                          </label>
                        )
                      })}
                    </div>
                  </FormControl>
                  <FormDescription className="text-xs">{t('channels.capabilitiesHint', 'Primary product usages for this channel')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )} />
              <FormField control={form.control} name="capabilities" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.capabilitiesFieldLabel', 'Capabilities')}</FormLabel>
                  <FormControl>
                    <div className="grid grid-cols-1 gap-2 rounded-md border border-border p-3">
                      {channelCapabilityOptions.map((option) => {
                        const checked = field.value.includes(option)
                        return (
                          <label key={option} className="flex items-center gap-3 text-sm text-foreground">
                            <Checkbox
                              checked={checked}
                              aria-label={option}
                              onCheckedChange={(value) => field.onChange(toggleStringSelection(field.value, option, value === true))}
                            />
                            <span>{friendlyLabel(capabilityFriendlyLabel, option)}</span>
                          </label>
                        )
                      })}
                    </div>
                  </FormControl>
                  <FormDescription className="text-xs">{t('channels.capabilitiesFieldHint', 'Transport capabilities preserved independently from usages')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )} />
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField control={form.control} name="linked_users" render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('channels.linkedUsers', 'Linked Users')}</FormLabel>
                  <FormControl><Input value={joinCSV(field.value)} onChange={(e) => field.onChange(splitCSV(e.target.value))} placeholder="ops-admin, user-1" /></FormControl>
                  <FormDescription className="text-xs">{t('channels.linkedUsersHint', 'Users who can receive targeted messages')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )} />
            </div>

            {knownUsers.length > 0 && (
              <div className="flex flex-col gap-2">
                  <div className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
                  <List size={12} />{t('channels.quickLink', 'Quick user linking')}
                </div>
                <CollapsibleList
                  limit={6}
                  emptyText={t('channels.noKnownUsers', 'No known users')}
                  items={knownUsers.map((user) => {
                    const userID = user.user_id || user.username || '';
                    const picked = (form.watch('linked_users') || []).includes(userID);
                    return (
                      <Button
                        key={userID}
                        type="button"
                        size="sm"
                        variant={picked ? 'amber' : 'outline'}
                        className={picked ? 'justify-start' : 'justify-start opacity-70'}
                        onClick={() => {
                          const current = form.getValues('linked_users') || [];
                          form.setValue('linked_users', picked ? current.filter(i => i !== userID) : [...current, userID]);
                        }}
                      >
                        {user.display_name || userID}
                      </Button>
                    );
                  })}
                />
              </div>
            )}
          </form>
        </Form>
      </GuidedFormDialog>
    </div>
  );
};

export default ChannelsPage;
