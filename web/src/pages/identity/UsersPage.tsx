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
  createUser, 
  fetchUser, 
  fetchUsers, 
  setUserEnabled, 
  updateUser 
} from '../../lib/api/access';
import type { AccessUser } from '../../lib/api/types';
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
import { ActiveBadge as StatusBadge } from '@/components/ui/active-badge';
import { DetailHeader } from '@/components/ui/detail-header';
import { EmptyDetailState } from '@/components/ui/empty-detail-state';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { PaginationControls } from '@/components/list/PaginationControls';
import { useNotify } from '@/hooks/ui/useNotify';
import { useI18n } from '@/hooks/useI18n';
import { splitCSV } from './registry-utils';
import { useRegistry } from '@/hooks/registry/useRegistry';
import { Plus, Mail, ShieldCheck, Users, Fingerprint, History, Edit3, Power, Save } from 'lucide-react';
import { clsx } from 'clsx';

const userSchema = z.object({
  user_id: z.string().optional(),
  username: z.string().min(1, 'Username is required'),
  display_name: z.string().optional(),
  email: z.string().email('Invalid email').or(z.literal('')),
  status: z.string().default('active'),
  source: z.string().default('local'),
  roles: z.array(z.string()).default([]),
  groups: z.array(z.string()).default([]),
  identities: z.array(z.object({
    provider_id: z.string().optional(),
    provider_type: z.string().optional(),
    external_subject: z.string().optional(),
    external_username: z.string().optional(),
    external_email: z.string().optional(),
  })).default([]),
});

type UserFormValues = z.infer<typeof userSchema>;

export const UsersPage = () => {
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
  } = useRegistry<AccessUser>({
    key: 'users',
    fetcher: fetchUsers,
  });

  // Queries
  const { data: userDetail, isLoading: detailLoading } = useQuery({
    queryKey: ['user', selectedID],
    queryFn: () => fetchUser(selectedID),
    enabled: !!selectedID,
  });

  // Form
  const form = useForm<UserFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(userSchema) as any,
    defaultValues: {
      username: '',
      display_name: '',
      email: '',
      status: 'active',
      source: 'local',
      roles: [],
      groups: [],
      identities: [],
    },
  });

  // Mutations
  const createMutation = useMutation({
    mutationFn: (values: UserFormValues) => createUser(values),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      setSelectedID(data.user_id || data.username || '');
      setIsCreating(false);
      setEditMode(false);
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const updateMutation = useMutation({
    mutationFn: (values: UserFormValues) => updateUser(selectedID, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      queryClient.invalidateQueries({ queryKey: ['user', selectedID] });
      setEditMode(false);
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => setUserEnabled(id, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      queryClient.invalidateQueries({ queryKey: ['user', selectedID] });
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const handleSelect = useCallback(async (user: AccessUser) => {
    const id = user.user_id || user.username || '';
    setSelectedID(id);
    setIsCreating(false);
    setEditMode(false);
  }, []);

  const startCreate = () => {
    setIsCreating(true);
    setSelectedID('');
    setEditMode(true);
    form.reset({
      username: '',
      display_name: '',
      email: '',
      status: 'active',
      source: 'local',
      roles: ['viewer'],
      groups: [],
      identities: [],
    });
  };

  const startEdit = () => {
    if (!userDetail) return;
    form.reset({
      username: userDetail.username,
      display_name: userDetail.display_name || '',
      email: userDetail.email || '',
      status: userDetail.status || 'active',
      source: userDetail.source || 'local',
      roles: userDetail.roles || [],
      groups: userDetail.groups || [],
      identities: userDetail.identities || [],
    });
    setEditMode(true);
  };

  const onSave = (values: UserFormValues) => {
    if (isCreating) createMutation.mutate(values);
    else updateMutation.mutate(values);
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle title={t('identity.users.title')} subtitle={t('identity.users.subtitle')} />

      <SummaryGrid>
        <StatCard title={t('identity.users.stats.total')} value={String(total)} subtitle={t('identity.users.stats.totalDesc')} icon={<Users size={16} />} />
        <StatCard title={t('identity.users.stats.active')} value={String(items.filter(u => u.status === 'active').length)} subtitle={t('identity.users.stats.activeDesc')} />
        <StatCard title={t('identity.users.stats.external')} value={String(items.filter(u => u.source !== 'local').length)} subtitle={t('identity.users.stats.externalDesc')} />
        <StatCard title={t('identity.users.stats.admins')} value={String(items.filter(u => u.roles?.includes('ops_admin')).length)} subtitle={t('identity.users.stats.adminsDesc')} icon={<ShieldCheck size={16} />} />
      </SummaryGrid>

      <SplitLayout
        sidebar={
          <RegistrySidebar key="users-sidebar">
            <div className="flex justify-between items-center mb-4 p-4 pb-0">
              <h2 className="text-lg font-bold m-0 text-foreground">{t('identity.users.sidebar.title')}</h2>
              <Button size="sm" onClick={startCreate} variant="amber"><Plus size={14} /> {t('identity.users.sidebar.new')}</Button>
            </div>
            <div className="relative mb-4 px-4 pt-4">
              <Input placeholder={t('identity.users.sidebar.search')} value={query} onChange={e => setQuery(e.target.value)} />
            </div>
            {loading ? <div className="p-10 text-center text-muted-foreground animate-pulse">{t('common.loading')}</div> : (
              <RegistryPanel title={t('identity.users.sidebar.panelTitle')} emptyText={t('identity.users.sidebar.empty')} className="px-4">
                <div className="flex flex-col gap-2">
                  {items.map((item) => {
                    const id = item.user_id || item.username || '';
                    const isSelected = selectedID === id;
                    return (
                      <button key={id} onClick={() => handleSelect(item)} className={clsx("text-left w-full focus:outline-none rounded-2xl transition-all", isSelected ? "ring-2 ring-primary" : "")}>
                         <RegistryCard
                          title={item.display_name || item.username || t('common.unknownUser')}
                          subtitle={id}
                          lines={[
                            `${t('common.email')}: ${item.email || t('common.na')}`,
                            `${t('common.source')}: ${item.source || 'local'}`,
                            `${t('common.groups')}: ${(item.groups || []).join(', ') || t('common.none')}`
                          ]}
                          status={<StatusBadge active={item.status === 'active'} label={item.status || 'active'} />}
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
                title={isCreating ? t('identity.users.detail.create') : (userDetail?.display_name || userDetail?.username || t('identity.users.detail.title'))}
                subtitle={isCreating ? t('identity.users.detail.provision') : selectedID}
                status={!isCreating && userDetail && <StatusBadge active={userDetail.status === 'active'} label={userDetail.status || 'active'} />}
                actions={!isCreating && userDetail && !editMode && (
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={startEdit}><Edit3 size={14} /> {t('identity.users.detail.edit')}</Button>
                    <Button variant="secondary" size="sm" onClick={() => toggleMutation.mutate({ id: selectedID, enabled: userDetail.status !== 'active' })} disabled={toggleMutation.isPending}>
                      <Power size={14} /> {userDetail.status === 'active' ? t('action.disable') : t('action.enable')}
                    </Button>
                  </div>
                )}
              />

              <div className="p-6">
                {editMode ? (
                  <Form {...form}>
                    <form onSubmit={form.handleSubmit(onSave)} className="space-y-6">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField control={form.control} name="username" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.users.detail.username')}</FormLabel>
                            <FormControl><Input {...field} placeholder="e.g. john.doe" /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="display_name" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.users.detail.displayName')}</FormLabel>
                            <FormControl><Input {...field} placeholder="e.g. John Doe" /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="email" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.users.detail.email')}</FormLabel>
                            <FormControl><Input {...field} placeholder="john@company.com" /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="source" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.users.detail.source')}</FormLabel>
                            <FormControl>
                              <NativeSelect className="bg-background" {...field}>
                                <option value="local">local</option>
                                <option value="oidc">oidc</option>
                                <option value="ldap">ldap</option>
                              </NativeSelect>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                      </div>

                      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField control={form.control} name="roles" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.users.detail.rolesCSV')}</FormLabel>
                            <FormControl><Input value={field.value.join(', ')} onChange={e => field.onChange(splitCSV(e.target.value))} placeholder="viewer, operator" /></FormControl>
                            <FormDescription>{t('identity.users.detail.rbacDesc')}</FormDescription>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={form.control} name="groups" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.users.detail.groupsCSV')}</FormLabel>
                            <FormControl><Input value={field.value.join(', ')} onChange={e => field.onChange(splitCSV(e.target.value))} placeholder="sre, dev" /></FormControl>
                            <FormDescription>{t('identity.users.detail.functionalDesc')}</FormDescription>
                            <FormMessage />
                          </FormItem>
                        )} />
                      </div>

                      <div className="flex gap-3 pt-4 border-t border-border">
                        <Button type="submit" variant="amber" disabled={createMutation.isPending || updateMutation.isPending}>
                          <Save size={14} /> {isCreating ? t('identity.users.detail.create') : t('identity.users.detail.save')}
                        </Button>
                        <Button type="button" variant="outline" onClick={() => { setEditMode(false); if (isCreating) setSelectedID(''); }}>{t('identity.users.detail.cancel')}</Button>
                      </div>
                    </form>
                  </Form>
                ) : userDetail && (
                  <div className="space-y-8 animate-fade-in">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                      <InfoBox label={t('identity.users.detail.email')} value={userDetail.email} icon={<Mail size={14} />} />
                      <InfoBox label={t('identity.users.detail.source')} value={userDetail.source} icon={<Fingerprint size={14} />} />
                      <InfoBox label={t('common.created', 'Created')} value={new Date(userDetail.created_at || '').toLocaleDateString()} icon={<History size={14} />} />
                    </div>

                    <div className="space-y-4">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><ShieldCheck size={16} /> {t('identity.users.detail.roles')}</h3>
                      <div className="flex flex-wrap gap-2">
                        {userDetail.roles?.map(r => (
                          <span key={r} className="px-3 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs font-bold font-mono">{r}</span>
                        )) || <span className="text-muted-foreground italic text-sm">{t('common.noRolesAssigned')}</span>}
                      </div>
                    </div>

                    <div className="space-y-4">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><Users size={16} /> {t('identity.users.detail.groups')}</h3>
                      <div className="flex flex-wrap gap-2">
                        {userDetail.groups?.map(g => (
                          <span key={g} className="px-3 py-1 rounded-full bg-muted text-foreground border border-border text-xs font-bold">{g}</span>
                        )) || <span className="text-muted-foreground italic text-sm">{t('common.noGroupMemberships')}</span>}
                      </div>
                    </div>

                    {userDetail.identities && userDetail.identities.length > 0 && (
                      <div className="space-y-4">
                        <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><Fingerprint size={16} /> {t('identity.users.detail.identities')}</h3>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          {userDetail.identities.map((id, idx) => (
                            <div key={idx} className="p-4 rounded-xl bg-muted border border-border">
                              <div className="text-xs font-bold text-primary mb-1 uppercase">{id.provider_id || id.provider_type}</div>
                              <div className="text-[0.7rem] font-mono text-muted-foreground truncate mb-2">{id.external_subject}</div>
                              <div className="text-sm text-foreground">{id.external_username || id.external_email || t('common.noClaimData')}</div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </RegistryDetail>
          ) : (
            <EmptyDetailState title={t('identity.users.empty.title')} description={t('identity.users.empty.desc')} />
          )
        }
      />
    </div>
  );
};

const InfoBox = ({ label, value, icon }: { label: string; value?: string; icon: React.ReactNode }) => (
  <div className="bg-muted border border-border rounded-xl p-4 space-y-1">
    <div className="flex items-center gap-2 text-[0.6rem] font-bold text-muted-foreground uppercase tracking-widest">
      {icon} {label}
    </div>
    <div className="text-sm font-medium text-foreground truncate">{value || '—'}</div>
  </div>
);

export default UsersPage;
