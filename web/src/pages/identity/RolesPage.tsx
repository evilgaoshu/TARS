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
  bindRole, 
  createRole, 
  fetchRoleBindings, 
  fetchRoles, 
  updateRole 
} from '../../lib/api/access';
import type { AccessRole, RoleBindingRequest } from '../../lib/api/types';
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
import { useNotify } from '@/hooks/ui/useNotify';
import { useI18n } from '@/hooks/useI18n';
import { splitCSV } from './registry-utils';
import { Plus, Shield, Key, Edit3, Link as LinkIcon, Save, X } from 'lucide-react';
import { clsx } from 'clsx';

const roleSchema = z.object({
  id: z.string().min(1, 'Role ID is required'),
  display_name: z.string().optional(),
  permissions: z.array(z.string()).default([]),
});

const bindingSchema = z.object({
  user_ids: z.array(z.string()).default([]),
  group_ids: z.array(z.string()).default([]),
});

type RoleFormValues = z.infer<typeof roleSchema>;
type BindingFormValues = z.infer<typeof bindingSchema>;

export const RolesPage = () => {
  const queryClient = useQueryClient();
  const notify = useNotify();
  const { t } = useI18n();
  const [selectedID, setSelectedID] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [panelMode, setPanelMode] = useState<'detail' | 'edit' | 'bind'>('detail');

  // Queries
  const { data: rolesData, isLoading: rolesLoading } = useQuery({
    queryKey: ['roles'],
    queryFn: () => fetchRoles(),
  });
  
  const roles = rolesData?.items || [];
  const selectedRole = roles.find(r => r.id === selectedID);

  const { data: roleBindings, isLoading: bindingsLoading } = useQuery({
    queryKey: ['role-bindings', selectedID],
    queryFn: () => fetchRoleBindings(selectedID),
    enabled: !!selectedID && panelMode !== 'edit' && !isCreating,
  });

  // Forms
  const roleForm = useForm<RoleFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(roleSchema) as any,
    defaultValues: { id: '', display_name: '', permissions: ['platform.read'] },
  });

  const bindingForm = useForm<BindingFormValues>({
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    resolver: zodResolver(bindingSchema) as any,
    defaultValues: { user_ids: [], group_ids: [] },
  });

  // Mutations
  const createMutation = useMutation({
    mutationFn: (values: RoleFormValues) => createRole(values),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      setSelectedID(data.id || '');
      setIsCreating(false);
      setPanelMode('detail');
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const updateMutation = useMutation({
    mutationFn: (values: RoleFormValues) => updateRole(selectedID, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] });
      setPanelMode('detail');
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const bindMutation = useMutation({
    mutationFn: (values: RoleBindingRequest) => bindRole(selectedID, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['role-bindings', selectedID] });
      setPanelMode('detail');
      notify.success(t('status.success'));
    },
    onError: (err) => notify.error(err, t('status.error')),
  });

  const handleSelect = useCallback((role: AccessRole) => {
    const id = role.id || '';
    if (!id) return;
    setSelectedID(id);
    setIsCreating(false);
    setPanelMode('detail');
  }, []);

  const startCreate = () => {
    setIsCreating(true);
    setSelectedID('');
    setPanelMode('edit');
    roleForm.reset({ id: '', display_name: '', permissions: ['platform.read'] });
  };

  const startEdit = () => {
    if (!selectedRole) return;
    roleForm.reset({
      id: selectedRole.id || '',
      display_name: selectedRole.display_name || '',
      permissions: selectedRole.permissions || [],
    });
    setPanelMode('edit');
  };

  const startBind = () => {
    bindingForm.reset({
      user_ids: roleBindings?.user_ids || [],
      group_ids: roleBindings?.group_ids || [],
    });
    setPanelMode('bind');
  };

  const onSaveRole = (values: RoleFormValues) => {
    if (isCreating) createMutation.mutate(values);
    else updateMutation.mutate(values);
  };

  const onSaveBindings = (values: BindingFormValues) => {
    bindMutation.mutate({ ...values, operator_reason: 'Update via UI' });
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle title={t('identity.roles.title')} subtitle={t('identity.roles.subtitle')} />

      <SummaryGrid>
        <StatCard title={t('identity.roles.stats.total')} value={String(roles.length)} subtitle={t('identity.roles.stats.totalDesc')} icon={<Shield size={16} />} />
        <StatCard title={t('identity.roles.stats.system')} value="—" subtitle={t('identity.roles.stats.systemDesc')} />
        <StatCard title={t('identity.roles.stats.custom')} value="—" subtitle={t('identity.roles.stats.customDesc')} />
      </SummaryGrid>

      <SplitLayout
        sidebar={
          <RegistrySidebar key="roles-sidebar">
            <div className="flex justify-between items-center mb-4 p-4 pb-0">
              <h2 className="text-lg font-bold m-0 text-foreground">{t('identity.roles.sidebar.title')}</h2>
              <Button size="sm" onClick={startCreate} variant="amber"><Plus size={14} /> {t('identity.roles.sidebar.new')}</Button>
            </div>
            {rolesLoading ? <div className="p-10 text-center text-muted-foreground animate-pulse">{t('common.loadingRoles')}</div> : (
              <RegistryPanel title={t('identity.roles.sidebar.panelTitle')} emptyText={t('identity.roles.sidebar.empty')} className="px-4">
                <div className="flex flex-col gap-2">
                  {roles.map((item) => {
                    const id = item.id || '';
                    const isSelected = selectedID === id;
                    return (
                      <button key={id} onClick={() => handleSelect(item)} className={clsx("text-left w-full focus:outline-none rounded-2xl transition-all", isSelected ? "ring-2 ring-primary" : "")}>
                         <RegistryCard
                          title={item.display_name || item.id || t('common.unknownRole')}
                          subtitle={id}
                          lines={[
                            `${t('common.permissions')}: ${(item.permissions || []).length}`,
                          ]}
                        />
                      </button>
                    );
                  })}
                </div>
              </RegistryPanel>
            )}
          </RegistrySidebar>
        }
        detail={
          (isCreating || selectedRole) ? (
            <RegistryDetail key={selectedID || 'new'}>
              <DetailHeader
                title={isCreating ? t('identity.roles.detail.create') : (selectedRole?.display_name || selectedRole?.id || t('identity.roles.detail.title'))}
                subtitle={isCreating ? t('identity.roles.detail.provision') : selectedID}
                actions={!isCreating && selectedRole && panelMode === 'detail' && (
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={startBind}><LinkIcon size={14} /> {t('identity.roles.detail.bindings')}</Button>
                    <Button variant="secondary" size="sm" onClick={startEdit}><Edit3 size={14} /> {t('identity.roles.detail.edit')}</Button>
                  </div>
                )}
              />

              <div className="p-6">
                {panelMode === 'edit' || isCreating ? (
                  <Form {...roleForm}>
                    <form onSubmit={roleForm.handleSubmit(onSaveRole)} className="space-y-6">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField control={roleForm.control} name="id" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.roles.detail.id')}</FormLabel>
                            <FormControl><Input {...field} placeholder="e.g. operator" disabled={!isCreating} /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={roleForm.control} name="display_name" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.roles.detail.displayName')}</FormLabel>
                            <FormControl><Input {...field} placeholder="e.g. Ops Operator" /></FormControl>
                            <FormMessage />
                          </FormItem>
                        )} />
                      </div>

                      <FormField control={roleForm.control} name="permissions" render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('identity.roles.detail.permissionsCSV')}</FormLabel>
                          <FormControl><Input value={field.value.join(', ')} onChange={e => field.onChange(splitCSV(e.target.value))} placeholder="platform.read, session.write" /></FormControl>
                          <FormDescription>{t('identity.roles.detail.permissionsDesc')}</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )} />

                      <div className="flex gap-3 pt-4 border-t border-border">
                        <Button type="submit" variant="amber" disabled={createMutation.isPending || updateMutation.isPending}>
                          <Save size={14} /> {isCreating ? t('identity.roles.detail.create') : t('identity.roles.detail.save')}
                        </Button>
                        <Button type="button" variant="outline" onClick={() => { 
                          if (isCreating) { setIsCreating(false); setSelectedID(''); } 
                          else setPanelMode('detail'); 
                        }}><X size={14} /> {t('identity.roles.detail.cancel')}</Button>
                      </div>
                    </form>
                  </Form>
                ) : panelMode === 'bind' ? (
                  <Form {...bindingForm}>
                    <form onSubmit={bindingForm.handleSubmit(onSaveBindings)} className="space-y-6">
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                        <FormField control={bindingForm.control} name="user_ids" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.roles.detail.bindUsers')}</FormLabel>
                            <FormControl><Input value={field.value.join(', ')} onChange={e => field.onChange(splitCSV(e.target.value))} placeholder="user1, user2" /></FormControl>
                            <FormDescription>{t('identity.roles.detail.bindUsersDesc')}</FormDescription>
                            <FormMessage />
                          </FormItem>
                        )} />
                        <FormField control={bindingForm.control} name="group_ids" render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('identity.roles.detail.bindGroups')}</FormLabel>
                            <FormControl><Input value={field.value.join(', ')} onChange={e => field.onChange(splitCSV(e.target.value))} placeholder="group1, group2" /></FormControl>
                            <FormDescription>{t('identity.roles.detail.bindGroupsDesc')}</FormDescription>
                            <FormMessage />
                          </FormItem>
                        )} />
                      </div>

                      <div className="flex gap-3 pt-4 border-t border-border">
                        <Button type="submit" variant="amber" disabled={bindMutation.isPending}>
                          <LinkIcon size={14} /> {t('identity.roles.detail.updateBindings')}
                        </Button>
                        <Button type="button" variant="outline" onClick={() => setPanelMode('detail')}><X size={14} /> {t('identity.roles.detail.cancel')}</Button>
                      </div>
                    </form>
                  </Form>
                ) : selectedRole && (
                  <div className="space-y-8 animate-fade-in">
                    <div className="space-y-4">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><Key size={16} /> {t('identity.roles.detail.permissionsCSV')}</h3>
                      <div className="flex flex-wrap gap-2">
                        {selectedRole.permissions?.map(p => (
                          <span key={p} className="px-3 py-1 rounded-full bg-primary/10 text-primary border border-primary/20 text-xs font-bold font-mono">{p}</span>
                        )) || <span className="text-muted-foreground italic text-sm">{t('common.noPermissions')}</span>}
                      </div>
                    </div>

                    <div className="space-y-4">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><LinkIcon size={16} /> {t('identity.roles.detail.bindUsers')}</h3>
                      {bindingsLoading ? <div className="animate-pulse text-xs text-muted-foreground">{t('common.loadingBindings')}</div> : (
                        <div className="flex flex-wrap gap-2">
                          {roleBindings?.user_ids?.map(u => (
                            <span key={u} className="px-3 py-1 rounded-full bg-muted text-foreground border border-border text-xs font-bold">{u}</span>
                          )) || <span className="text-muted-foreground italic text-sm">{t('common.noUsersbound')}</span>}
                        </div>
                      )}
                    </div>

                    <div className="space-y-4">
                      <h3 className="text-sm font-bold text-muted-foreground uppercase tracking-widest flex items-center gap-2"><LinkIcon size={16} /> {t('identity.roles.detail.bindGroups')}</h3>
                      {bindingsLoading ? <div className="animate-pulse text-xs text-muted-foreground">{t('common.loadingBindings')}</div> : (
                        <div className="flex flex-wrap gap-2">
                          {roleBindings?.group_ids?.map(g => (
                            <span key={g} className="px-3 py-1 rounded-full bg-muted text-foreground border border-border text-xs font-bold">{g}</span>
                          )) || <span className="text-muted-foreground italic text-sm">{t('common.noGroupsBound')}</span>}
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            </RegistryDetail>
          ) : (
            <EmptyDetailState title={t('identity.roles.empty.title')} description={t('identity.roles.empty.desc')} />
          )
        }
      />
    </div>
  );
};

export default RolesPage;
