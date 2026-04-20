import { useCallback, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  RegistryDetail,
  RegistrySidebar,
  RegistryCard,
  RegistryPanel,
  SplitLayout,
} from '@/components/ui/registry-primitives';
import {
  createGroup, 
  fetchGroup, 
  fetchGroups, 
  setGroupEnabled,
  updateGroup 
} from '../../lib/api/access';
import type { AccessGroup } from '../../lib/api/types';
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
import { Button } from '@/components/ui/button';
import { DetailHeader } from '@/components/ui/detail-header';
import { EmptyDetailState } from '@/components/ui/empty-detail-state';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { PaginationControls } from '@/components/list/PaginationControls';
import { useNotify } from '@/hooks/ui/useNotify';
import { useI18n } from '@/hooks/useI18n';
import { splitCSV } from './registry-utils';
import { useRegistry } from '@/hooks/registry/useRegistry';
import { Plus, Users, ShieldCheck, Edit3, Save, Power } from 'lucide-react';
import { clsx } from 'clsx';

const groupSchema = z.object({
  group_id: z.string().min(1, 'Group ID is required'),
  display_name: z.string().optional(),
  description: z.string().optional(),
  roles: z.array(z.string()).default([]),
  members: z.array(z.string()).default([]),
});

type GroupFormValues = z.infer<typeof groupSchema>;

export const GroupsPage = () => {
  const queryClient = useQueryClient();
  const notify = useNotify();
  const { t } = useI18n();
  const [selectedID, setSelectedID] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [editMode, setEditMode] = useState(false);

  // Registry Hook
  const {
    items, total, page, limit, loading, query,
    setPage, setLimit, setQuery
  } = useRegistry<AccessGroup>({
    key: 'groups',
    fetcher: fetchGroups,
  });

  // Queries
  const { data: groupDetail, isLoading: detailLoading } = useQuery({
    queryKey: ['group', selectedID],
    queryFn: () => fetchGroup(selectedID),
    enabled: !!selectedID,
  });

  // Form
  const form = useForm<GroupFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(groupSchema) as any,
    defaultValues: {
      group_id: '',
      display_name: '',
      description: '',
      roles: ['viewer'],
      members: [],
    },
  });

  // Mutations
  const createMutation = useMutation({
    mutationFn: (values: GroupFormValues) => createGroup(values),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      setSelectedID(data.group_id || '');
      setIsCreating(false);
      setEditMode(false);
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const updateMutation = useMutation({
    mutationFn: (values: GroupFormValues) => updateGroup(selectedID, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      queryClient.invalidateQueries({ queryKey: ['group', selectedID] });
      setEditMode(false);
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => setGroupEnabled(id, enabled),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      queryClient.invalidateQueries({ queryKey: ['group', data.group_id] });
      if (selectedID === data.group_id) {
        setSelectedID(data.group_id || '');
      }
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const handleSelect = useCallback(async (group: AccessGroup) => {
    const id = group.group_id || '';
    setSelectedID(id);
    setIsCreating(false);
    setEditMode(false);
  }, []);

  const startCreate = () => {
    setIsCreating(true);
    setSelectedID('');
    setEditMode(true);
    form.reset({
      group_id: '',
      display_name: '',
      description: '',
      roles: ['viewer'],
      members: [],
    });
  };

  const startEdit = () => {
    if (!groupDetail) return;
    form.reset({
      group_id: groupDetail.group_id || '',
      display_name: groupDetail.display_name || '',
      description: groupDetail.description || '',
      roles: groupDetail.roles || [],
      members: groupDetail.members || [],
    });
    setEditMode(true);
  };

  const onSave = (values: GroupFormValues) => {
    if (isCreating) createMutation.mutate(values);
    else updateMutation.mutate(values);
  };

  const activeGroups = items.filter((group) => (group.status || 'active') === 'active').length;
  const inactiveGroups = Math.max(total - activeGroups, 0);

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle title={t('identity.groups.title')} subtitle={t('identity.groups.subtitle')} />

      <SummaryGrid>
        <StatCard title={t('identity.groups.stats.total')} value={String(total)} subtitle={t('identity.groups.stats.totalDesc')} icon={<Users size={16} />} />
        <StatCard title={t('identity.groups.stats.active')} value={String(activeGroups)} subtitle={t('identity.groups.stats.activeDesc')} />
        <StatCard title={t('identity.groups.stats.inactive')} value={String(inactiveGroups)} subtitle={t('identity.groups.stats.inactiveDesc')} />
        <StatCard title={t('identity.groups.stats.withMembers')} value={String(items.filter(g => g.members?.length).length)} subtitle={t('identity.groups.stats.membersDesc')} icon={<Users size={16} />} />
        <StatCard title={t('identity.groups.stats.adminGroups')} value={String(items.filter(g => g.roles?.includes('ops_admin')).length)} subtitle={t('identity.groups.stats.adminsDesc')} icon={<ShieldCheck size={16} />} />
      </SummaryGrid>

      <SplitLayout
        sidebar={
          <RegistrySidebar key="groups-sidebar">
            <div className="flex justify-between items-center mb-4 p-4 pb-0">
              <h2 className="text-lg font-bold m-0 text-foreground">{t('identity.groups.sidebar.title')}</h2>
              <Button size="sm" onClick={startCreate} variant="amber"><Plus size={14} /> {t('identity.groups.sidebar.new')}</Button>
            </div>
            <div className="relative mb-4 px-4 pt-4">
              <Input placeholder={t('identity.groups.sidebar.search')} value={query} onChange={e => setQuery(e.target.value)} />
            </div>
            {loading ? <div className="p-10 text-center text-muted-foreground animate-pulse">{t('common.loading')}</div> : (
              <RegistryPanel title={t('identity.groups.sidebar.panelTitle')} emptyText={t('identity.groups.sidebar.empty')} className="px-4">
                <div className="flex flex-col gap-2">
                  {items.map((item) => {
                    const id = item.group_id || '';
                    const isSelected = selectedID === id;
                    return (
                      <button key={id} onClick={() => handleSelect(item)} className={clsx("text-left w-full focus:outline-none rounded-2xl transition-all", isSelected ? "ring-2 ring-primary" : "")}>
                         <RegistryCard
                          title={item.display_name || item.group_id || t('common.unknownGroup')}
                          subtitle={id}
                          lines={[
                            `${t('common.status')}: ${item.status || 'active'}`,
                            `${t('common.members')}: ${(item.members || []).length}`,
                            `${t('common.roles')}: ${(item.roles || []).join(', ') || t('common.none')}`
                          ]}
                        />
                      </button>
                    );
                  })}
                </div>
              </RegistryPanel>
            )}
            <div className="p-4 border-t border-border mt-auto">
              <PaginationControls page={page} limit={limit} total={total} hasNext={items.length === limit} onPageChange={setPage} onLimitChange={setLimit} limitOptions={[10, 20, 50]} />
            </div>
          </RegistrySidebar>
        }
        detail={
          (isCreating || selectedID) ? (
            <RegistryDetail key={selectedID || 'new'}>
              {detailLoading && !isCreating && <div className="absolute inset-0 bg-background/40 backdrop-blur-[1px] z-50 flex items-center justify-center rounded-2xl font-bold">{t('common.loadingCaps')}</div>}
              
              <DetailHeader
                title={isCreating ? t('identity.groups.detail.create') : (groupDetail?.display_name || groupDetail?.group_id || t('identity.groups.detail.title'))}
                subtitle={isCreating ? t('identity.groups.detail.provision') : selectedID}
                actions={!isCreating && groupDetail && !editMode && (
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={startEdit}><Edit3 size={14} /> {t('identity.groups.detail.edit')}</Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => toggleMutation.mutate({ id: selectedID, enabled: (groupDetail.status || 'active') !== 'active' })}
                      disabled={toggleMutation.isPending}
                    >
                      <Power size={14} /> {toggleMutation.isPending ? '...' : ((groupDetail.status || 'active') === 'active' ? t('action.disable') : t('action.enable'))}
                    </Button>
                  </div>
                )}
              />

              <div className="p-6">
                {editMode ? (
                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSave)} className="space-y-6">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField control={form.control} name="group_id" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.groups.detail.id')}</FormLabel>
                            <FormControl><Input {...field} placeholder="e.g. dev-team" disabled={!isCreating} /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="display_name" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.groups.detail.displayName')}</FormLabel>
                            <FormControl><Input {...field} placeholder="e.g. Developers" /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                      </div>

                      <FormField control={form.control} name="description" render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('identity.groups.detail.description')}</FormLabel>
                          <FormControl><Input {...field} placeholder="Group description" /></FormControl>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField control={form.control} name="roles" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.groups.detail.rolesCSV')}</FormLabel>
                            <FormControl><Input value={field.value.join(', ')} onChange={e => field.onChange(splitCSV(e.target.value))} placeholder="viewer, operator" /></FormControl>
                            <FormDescription>{t('identity.groups.detail.rbacDesc')}</FormDescription>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="members" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.groups.detail.membersCSV')}</FormLabel>
                            <FormControl><Input value={field.value.join(', ')} onChange={e => field.onChange(splitCSV(e.target.value))} placeholder="user1, user2" /></FormControl>
                            <FormDescription>{t('identity.groups.detail.membersDesc')}</FormDescription>
                            <FormMessage />
                          </FormItem>
                        )} />
                      </div>

                      <div className="flex gap-3 pt-4 border-t border-border">
                        <Button type="submit" variant="amber" disabled={createMutation.isPending || updateMutation.isPending}>
                          <Save size={14} /> {isCreating ? t('identity.groups.detail.create') : t('identity.groups.detail.save')}
                        </Button>
                        <Button type="button" variant="outline" onClick={() => { setEditMode(false); if (isCreating) setSelectedID(''); }}>{t('identity.groups.detail.cancel')}</Button>
                      </div>
                    </form>
                  </Form>
                ) : groupDetail && (
                  <div className="space-y-8 animate-fade-in">
                    <div className="flex flex-wrap gap-2">
                      <span className={clsx(
                        'px-3 py-1 rounded-full border text-xs font-bold uppercase tracking-wide',
                        (groupDetail.status || 'active') === 'active'
                          ? 'bg-success/10 text-success border-success/20'
                          : 'bg-warning/10 text-warning border-warning/20'
                      )}>
                        {(groupDetail.status || 'active') === 'active' ? t('status.active') : t('status.inactive')}
                      </span>
                      <span className="px-3 py-1 rounded-full bg-muted text-foreground border border-border text-xs font-bold">
                        {(groupDetail.members || []).length} {t('identity.groups.stats.withMembers')}
                      </span>
                      <span className="px-3 py-1 rounded-full bg-muted text-foreground border border-border text-xs font-bold">
                        {(groupDetail.roles || []).length} {t('common.roles')}
                      </span>
                    </div>

                    <div className="space-y-4">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><ShieldCheck size={16} /> {t('identity.groups.detail.rolesCSV')}</h3>
                      <div className="flex flex-wrap gap-2">
                        {groupDetail.roles?.map(r => (
                          <span key={r} className="px-3 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs font-bold font-mono">{r}</span>
                        )) || <span className="text-muted-foreground italic text-sm">{t('common.noRolesAssigned')}</span>}
                      </div>
                    </div>

                    <div className="space-y-4">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><Users size={16} /> {t('identity.groups.detail.membersCSV')}</h3>
                      <div className="flex flex-wrap gap-2">
                        {groupDetail.members?.map(m => (
                          <span key={m} className="px-3 py-1 rounded-full bg-muted text-foreground border border-border text-xs font-bold">{m}</span>
                        )) || <span className="text-muted-foreground italic text-sm">{t('common.noMembers')}</span>}
                      </div>
                    </div>
                    
                    {groupDetail.description && (
                      <div className="space-y-4">
                        <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><Edit3 size={16} /> {t('identity.groups.detail.description')}</h3>
                        <p className="text-sm text-foreground leading-relaxed">{groupDetail.description}</p>
                      </div>
                    )}

                  </div>
                )}
              </div>
            </RegistryDetail>
          ) : (
            <EmptyDetailState title={t('identity.groups.empty.title')} description={t('identity.groups.empty.desc')} />
          )
        }
      />
    </div>
  );
};

export default GroupsPage;
