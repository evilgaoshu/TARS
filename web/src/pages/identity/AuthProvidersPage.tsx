import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  createAuthProvider,
  fetchAccessConfig,
  fetchAuthProvider,
  fetchAuthProviders,
  setAuthProviderEnabled,
  updateAuthProvider,
} from '../../lib/api/access';
import { getApiErrorMessage } from '../../lib/api/ops';
import type { AccessConfigResponse, AuthProviderInfo } from '../../lib/api/types';
import {
  RegistryCard,
  RegistryDetail,
  RegistryPanel,
  RegistrySidebar,
  SplitLayout,
} from '@/components/ui/registry-primitives';
import { joinCSV, previewList, splitCSV } from './registry-utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { ActiveBadge as StatusBadge } from '@/components/ui/active-badge';
import { DetailHeader } from '@/components/ui/detail-header';
import { EmptyDetailState } from '@/components/ui/empty-detail-state';
import { FieldHint } from '@/components/ui/field-hint';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { LabeledField } from '@/components/ui/labeled-field';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { useI18n } from '@/hooks/useI18n';
import { cn } from '@/lib/utils';

const defaultProvider = (): AuthProviderInfo => ({
  id: '',
  type: 'oidc',
  name: '',
  enabled: true,
  login_url: '',
  issuer_url: '',
  client_id: '',
  client_secret: '',
  client_secret_set: false,
  client_secret_ref: '',
  auth_url: '',
  token_url: '',
  user_info_url: '',
  session_ttl_seconds: 43200,
  ldap_url: '',
  bind_dn: '',
  bind_password: '',
  bind_password_set: false,
  bind_password_ref: '',
  base_dn: '',
  user_search_filter: '',
  group_search_filter: '',
  redirect_path: '/api/v1/auth/callback/',
  success_redirect: '/login',
  user_id_field: 'sub',
  username_field: 'preferred_username',
  display_name_field: 'name',
  email_field: 'email',
  allowed_domains: [],
  scopes: ['openid', 'profile', 'email'],
  default_roles: ['viewer'],
  allow_jit: true,
});

import { useCapabilities } from '../../lib/FeatureGateContext';
import { isEnabled, type Capability } from '../../lib/featureGates';

const providerTypeOptions: { value: string; label: string; capability?: Capability }[] = [
  { value: 'oidc', label: 'OIDC', capability: 'identity.oidc' },
  { value: 'oauth2', label: 'OAuth2', capability: 'identity.oidc' },
  { value: 'local_password', label: 'Local Password', capability: 'identity.local_password' },
  { value: 'local_token', label: 'Local Token', capability: 'identity.local_token' },
  { value: 'ldap', label: 'LDAP', capability: 'identity.ldap' },
];

export const AuthProvidersPage = () => {
  const { t } = useI18n();
  const [items, setItems] = useState<AuthProviderInfo[]>([]);
  const [config, setConfig] = useState<AccessConfigResponse | null>(null);
  const [selectedID, setSelectedID] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [form, setForm] = useState<AuthProviderInfo>(defaultProvider());
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'warning' | 'error'; text: string } | null>(null);

  const isCreatingRef = useRef(false);
  const selectedIDRef = useRef('');

  const setIsCreatingSync = (val: boolean) => { isCreatingRef.current = val; setIsCreating(val); };
  const setSelectedIDSync = (val: string) => { selectedIDRef.current = val; setSelectedID(val); };

  const { capabilities } = useCapabilities();
  const filteredTypeOptions = useMemo(() => {
    return providerTypeOptions.filter(o => !o.capability || isEnabled(capabilities, o.capability));
  }, [capabilities]);

  const loadDetail = useCallback(async (providerID: string, knownItems?: AuthProviderInfo[]) => {
    if (!providerID) return;
    try {
      setDetailLoading(true);
      const detail = await fetchAuthProvider(providerID);
      setIsCreatingSync(false);
      setSelectedIDSync(providerID);
      setForm({ ...defaultProvider(), ...detail, client_secret: '', client_secret_set: detail.client_secret_set });
      if (knownItems) setItems(knownItems);
    } catch (error) {
      setMessage({ type: 'error', text: getApiErrorMessage(error, t('identity.authProviders.detail.loadDetailFailed')) });
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const load = useCallback(async (preferredID?: string) => {
    try {
      setLoading(true);
      setMessage(null);
      const [providersResp, accessConfig] = await Promise.all([
        fetchAuthProviders(),
        fetchAccessConfig(),
      ]);
      const nextItems = providersResp.items || [];
      setItems(nextItems);
      setConfig(accessConfig);

      const nextID = preferredID || (isCreatingRef.current ? '' : selectedIDRef.current) || nextItems[0]?.id || '';
      if (!nextID) {
        if (!isCreatingRef.current) {
          setSelectedIDSync('');
          setForm(defaultProvider());
        }
        return;
      }
      await loadDetail(nextID, nextItems);
    } catch (error) {
      setMessage({ type: 'error', text: getApiErrorMessage(error, t('identity.authProviders.detail.loadListFailed')) });
    } finally {
      setLoading(false);
    }
  }, [loadDetail]);

  useEffect(() => { void load(); }, [load]);

  const filteredItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return items;
    return items.filter((item) =>
      [item.id, item.name, item.type, item.client_secret_ref, ...(item.default_roles || []), ...(item.allowed_domains || [])]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(needle)),
    );
  }, [items, query]);

  const startCreate = () => {
    setIsCreatingSync(true);
    setSelectedIDSync('');
    setMessage(null);
    setForm(defaultProvider());
  };

  const handleSave = async () => {
    if (!form.id?.trim()) { setMessage({ type: 'error', text: t('identity.authProviders.detail.idRequired') }); return; }
    try {
      setSaving(true);
      setMessage(null);
        const payload: AuthProviderInfo = {
          ...form,
          id: form.id.trim(),
          name: form.name?.trim(),
          type: form.type?.trim(),
          issuer_url: form.issuer_url?.trim(),
          client_id: form.client_id?.trim(),
          client_secret: form.client_secret?.trim(),
          client_secret_ref: form.client_secret_ref?.trim(),
          auth_url: form.auth_url?.trim(),
          token_url: form.token_url?.trim(),
          user_info_url: form.user_info_url?.trim(),
          session_ttl_seconds: form.session_ttl_seconds,
          ldap_url: form.ldap_url?.trim(),
          bind_dn: form.bind_dn?.trim(),
          bind_password: form.bind_password?.trim(),
          bind_password_ref: form.bind_password_ref?.trim(),
          base_dn: form.base_dn?.trim(),
          user_search_filter: form.user_search_filter?.trim(),
          group_search_filter: form.group_search_filter?.trim(),
          redirect_path: form.redirect_path?.trim(),
          success_redirect: form.success_redirect?.trim(),
        user_id_field: form.user_id_field?.trim(),
        username_field: form.username_field?.trim(),
        display_name_field: form.display_name_field?.trim(),
        email_field: form.email_field?.trim(),
        allowed_domains: form.allowed_domains || [],
        scopes: form.scopes || [],
        default_roles: form.default_roles || [],
      };
      const saved = selectedID
        ? await updateAuthProvider(selectedID, payload)
        : await createAuthProvider(payload);
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
      await setAuthProviderEnabled(form.id, !form.enabled);
      setMessage({ type: 'success', text: t('status.success') });
      await load(form.id);
    } catch (error) {
      setMessage({ type: 'error', text: getApiErrorMessage(error, t('status.error')) });
    } finally {
      setToggling(false);
    }
  };

  const localTokenHint = form.type === 'local_token' || form.type === 'local_password';
  const ldapHint = form.type === 'ldap';
  const missingSecret = !['local_token', 'local_password', 'ldap'].includes(form.type || '') && !form.client_secret_set && !form.client_secret && !form.client_secret_ref;
  const missingLDAPBind = form.type === 'ldap' && !form.bind_password_set && !form.bind_password && !form.bind_password_ref;
  const showDetail = isCreating || Boolean(selectedID) || Boolean(form.id);

  return (
    <div className="animate-fade-in grid gap-6">
      <SectionTitle title={t('identity.authProviders.title')} subtitle={t('identity.authProviders.subtitle')} />

      <SummaryGrid>
        <StatCard title={t('identity.authProviders.stats.total')} value={String(items.length)} subtitle={t('identity.authProviders.stats.total')} />
        <StatCard title={t('identity.authProviders.stats.enabled')} value={String(items.filter((item) => item.enabled).length)} subtitle={t('identity.authProviders.stats.enabled')} />
        <StatCard title={t('identity.authProviders.stats.secrets')} value={String(items.filter((item) => item.client_secret_set || item.type === 'local_token' || item.type === 'local_password').length)} subtitle={t('identity.authProviders.stats.secrets')} />
        <StatCard title={t('identity.authProviders.stats.configPath')} value={config?.path ? 'file' : 'memory'} subtitle={config?.path || 'in-memory'} />
      </SummaryGrid>

      {message ? <StatusMessage message={message.text} type={message.type} /> : null}

      <SplitLayout
        sidebar={
          <RegistrySidebar>
            <div className="flex flex-wrap items-center justify-between gap-3 p-4 pb-0">
              <h2 className="text-lg font-bold text-foreground">{t('identity.authProviders.sidebar.title')}</h2>
              <Button variant="amber" size="sm" type="button" onClick={startCreate}>{t('identity.authProviders.sidebar.new')}</Button>
            </div>
            <div className="px-4 pt-4">
              <Input placeholder={t('identity.authProviders.sidebar.search')} value={query} onChange={(event) => setQuery(event.target.value)} />
            </div>
            {loading ? <div className="p-10 text-center text-muted-foreground animate-pulse">{t('common.loading')}</div> : (
              <RegistryPanel title={t('identity.authProviders.sidebar.panelTitle')} emptyText={t('identity.authProviders.sidebar.empty')} className="px-4">
                <div className="flex flex-col gap-2">
                  {filteredItems.map((item) => (
                    <button key={item.id} type="button" onClick={() => void loadDetail(item.id || '')} className={cn("w-full text-left focus:outline-none rounded-2xl transition-all", selectedID === item.id ? "ring-2 ring-primary" : "")}>
                       <RegistryCard
                        title={item.name || item.id || t('common.unknownUser')}
                        subtitle={item.id || item.type || 'provider'}
                        lines={[
                          `${t('identity.authProviders.detail.type')}: ${item.type || t('common.unknownRole')}`,
                          `${t('identity.authProviders.detail.defaultRoles')}: ${previewList(item.default_roles, 3)}`,
                          `${t('identity.authProviders.detail.allowedDomains')}: ${previewList(item.allowed_domains, 3)}`,
                          `${t('identity.authProviders.sidebar.secret')}: ${item.client_secret_set ? t('identity.authProviders.sidebar.secretConfigured') : (item.type === 'local_token' || item.type === 'local_password') ? t('identity.authProviders.sidebar.tokenLogin') : t('identity.authProviders.sidebar.secretMissing')}`,
                        ]}
                        status={<StatusBadge active={item.enabled} label={item.enabled ? t('status.enabled') : t('status.disabled')} />}
                      />
                    </button>
                  ))}
                </div>
              </RegistryPanel>
            )}
          </RegistrySidebar>
        }
        detail={showDetail ? (
          <RegistryDetail>
            <DetailHeader
              title={selectedID ? (form.name || form.id || t('identity.authProviders.detail.title')) : t('identity.authProviders.detail.create')}
              subtitle={selectedID ? t('identity.authProviders.detail.provision') : t('identity.authProviders.detail.provision')}
              status={<StatusBadge active={form.enabled} label={form.enabled ? t('status.enabled') : t('status.disabled')} />}
              actions={selectedID ? (
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" type="button" onClick={startCreate}>{t('identity.authProviders.detail.duplicate')}</Button>
                  <Button variant="secondary" size="sm" type="button" onClick={() => void handleToggle()} disabled={toggling}>
                    {toggling ? '...' : form.enabled ? t('action.disable') : t('action.enable')}
                  </Button>
                </div>
              ) : undefined}
            />

            <div className="p-6 space-y-8 relative">
              {detailLoading && <div className="absolute inset-0 bg-background/40 backdrop-blur-[1px] z-50 flex items-center justify-center rounded-2xl font-bold uppercase tracking-widest text-muted-foreground">{t('common.loadingCaps')}</div>}
              
              {!config?.configured && <StatusMessage type="warning" message={t('identity.authProviders.detail.memoryWarning')} />}
              {missingSecret && <StatusMessage type="warning" message={t('identity.authProviders.detail.missingSecret')} />}
              {localTokenHint && <StatusMessage type="info" message={t('identity.authProviders.detail.localTokenHint')} />}
              {ldapHint && <StatusMessage type="info" message={t('identity.authProviders.detail.ldapHint')} />}
              {missingLDAPBind && <StatusMessage type="warning" message={t('identity.authProviders.detail.missingLdapBind')} />}

              <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
                <LabeledField label={t('identity.authProviders.detail.id')} required>
                  <Input value={form.id || ''} onChange={(event) => setForm((c) => ({ ...c, id: event.target.value }))} disabled={Boolean(selectedID)} placeholder="google-workspace" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.displayName')} required>
                  <Input value={form.name || ''} onChange={(event) => setForm((c) => ({ ...c, name: event.target.value }))} placeholder="Google Workspace" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.type')} required>
                  <NativeSelect value={form.type || 'oidc'} onChange={(event) => setForm((c) => ({ ...c, type: event.target.value }))}>
                    {filteredTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </NativeSelect>
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.clientId')}>
                  <Input value={form.client_id || ''} onChange={(event) => setForm((c) => ({ ...c, client_id: event.target.value }))} placeholder="client id" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.sessionTtl')}>
                  <Input type="number" min={300} value={form.session_ttl_seconds || 43200} onChange={(event) => setForm((c) => ({ ...c, session_ttl_seconds: Number(event.target.value) || 43200 }))} placeholder="43200" />
                </LabeledField>
              </div>

              {(form.type === 'oidc' || form.type === 'oauth2') && (
                <div className="grid gap-6 md:grid-cols-1">
                  <LabeledField label={t('identity.authProviders.detail.issuerUrl')}>
                    <Input value={form.issuer_url || ''} onChange={(event) => setForm((c) => ({ ...c, issuer_url: event.target.value }))} placeholder="https://accounts.google.com" />
                    <FieldHint>{t('identity.authProviders.detail.oidcDiscoveryHint')}</FieldHint>
                  </LabeledField>
                </div>
              )}

              <div className="grid gap-6 md:grid-cols-2">
                <LabeledField label={form.type === 'local_token' ? t('identity.authProviders.detail.tokenSecret') : t('identity.authProviders.detail.clientSecret')}>
                  <Input type="password" value={form.client_secret || ''} onChange={(event) => setForm((c) => ({ ...c, client_secret: event.target.value }))} placeholder={form.client_secret_set ? t('identity.authProviders.detail.secretSetKeep') : t('identity.authProviders.detail.writeOnlyField')} />
                  <FieldHint>{form.client_secret_set ? t('identity.authProviders.detail.secretSet') : t('identity.authProviders.detail.writeOnly')}</FieldHint>
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.clientSecretRef')}>
                  <Input value={form.client_secret_ref || ''} onChange={(event) => setForm((c) => ({ ...c, client_secret_ref: event.target.value }))} placeholder="auth/google-workspace/client_secret" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.successRedirect')}>
                  <Input value={form.success_redirect || ''} onChange={(event) => setForm((c) => ({ ...c, success_redirect: event.target.value }))} placeholder="/login" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.redirectPath')}>
                  <Input value={form.redirect_path || ''} onChange={(event) => setForm((c) => ({ ...c, redirect_path: event.target.value }))} placeholder="/api/v1/auth/callback/google-workspace" />
                </LabeledField>
              </div>

              {(form.type !== 'local_token' && form.type !== 'ldap') && (
                <div className="grid gap-6 md:grid-cols-2">
                  <LabeledField label={t('identity.authProviders.detail.authUrl')}>
                    <Input value={form.auth_url || ''} onChange={(event) => setForm((c) => ({ ...c, auth_url: event.target.value }))} placeholder="https://accounts.google.com/o/oauth2/v2/auth" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.tokenUrl')}>
                    <Input value={form.token_url || ''} onChange={(event) => setForm((c) => ({ ...c, token_url: event.target.value }))} placeholder="https://oauth2.googleapis.com/token" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.userInfoUrl')}>
                    <Input value={form.user_info_url || ''} onChange={(event) => setForm((c) => ({ ...c, user_info_url: event.target.value }))} placeholder="https://openidconnect.googleapis.com/v1/userinfo" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.scopes')}>
                    <Input value={joinCSV(form.scopes)} onChange={(event) => setForm((c) => ({ ...c, scopes: splitCSV(event.target.value) }))} placeholder="openid, profile, email" />
                  </LabeledField>
                </div>
              )}

              {form.type === 'ldap' && (
                <div className="grid gap-6 md:grid-cols-2">
                  <LabeledField label={t('identity.authProviders.detail.ldapUrl')}>
                    <Input value={form.ldap_url || ''} onChange={(event) => setForm((c) => ({ ...c, ldap_url: event.target.value }))} placeholder="ldaps://ldap.example.com:636" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.baseDn')}>
                    <Input value={form.base_dn || ''} onChange={(event) => setForm((c) => ({ ...c, base_dn: event.target.value }))} placeholder="dc=example,dc=com" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.bindDn')}>
                    <Input value={form.bind_dn || ''} onChange={(event) => setForm((c) => ({ ...c, bind_dn: event.target.value }))} placeholder="cn=svc-tars,ou=svc,dc=example,dc=com" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.bindPassword')}>
                    <Input type="password" value={form.bind_password || ''} onChange={(event) => setForm((c) => ({ ...c, bind_password: event.target.value }))} placeholder={form.bind_password_set ? t('identity.authProviders.detail.passwordSetKeep') : t('identity.authProviders.detail.writeOnlyField')} />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.bindPasswordRef')}>
                    <Input value={form.bind_password_ref || ''} onChange={(event) => setForm((c) => ({ ...c, bind_password_ref: event.target.value }))} placeholder="ldap/main/bind_password" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.userSearch')}>
                    <Input value={form.user_search_filter || ''} onChange={(event) => setForm((c) => ({ ...c, user_search_filter: event.target.value }))} placeholder="(uid={username})" />
                  </LabeledField>
                  <LabeledField label={t('identity.authProviders.detail.groupSearch')}>
                    <Input value={form.group_search_filter || ''} onChange={(event) => setForm((c) => ({ ...c, group_search_filter: event.target.value }))} placeholder="(member={dn})" />
                  </LabeledField>
                </div>
              )}

              <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
                <LabeledField label={t('identity.authProviders.detail.userIdField')}>
                  <Input value={form.user_id_field || ''} onChange={(event) => setForm((c) => ({ ...c, user_id_field: event.target.value }))} placeholder="sub" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.usernameField')}>
                  <Input value={form.username_field || ''} onChange={(event) => setForm((c) => ({ ...c, username_field: event.target.value }))} placeholder="preferred_username" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.displayNameField')}>
                  <Input value={form.display_name_field || ''} onChange={(event) => setForm((c) => ({ ...c, display_name_field: event.target.value }))} placeholder="name" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.emailField')}>
                  <Input value={form.email_field || ''} onChange={(event) => setForm((c) => ({ ...c, email_field: event.target.value }))} placeholder="email" />
                </LabeledField>
              </div>

              <div className="grid gap-6 md:grid-cols-2">
                <LabeledField label={t('identity.authProviders.detail.allowedDomains')}>
                  <Input value={joinCSV(form.allowed_domains)} onChange={(event) => setForm((c) => ({ ...c, allowed_domains: splitCSV(event.target.value) }))} placeholder="example.com, corp.example.com" />
                </LabeledField>
                <LabeledField label={t('identity.authProviders.detail.defaultRoles')}>
                  <Input value={joinCSV(form.default_roles)} onChange={(event) => setForm((c) => ({ ...c, default_roles: splitCSV(event.target.value) }))} placeholder="viewer, operator" />
                </LabeledField>
                <div className="flex flex-col gap-4">
                  <label className="flex items-center gap-3 text-sm font-bold text-foreground cursor-pointer group">
                    <input type="checkbox" className="size-4 rounded border-border text-primary focus:ring-primary/40" checked={form.enabled} onChange={(event) => setForm((c) => ({ ...c, enabled: event.target.checked }))} />
                    <span>{t('identity.authProviders.detail.enabled')}</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm font-bold text-foreground cursor-pointer group">
                    <input type="checkbox" className="size-4 rounded border-border text-primary focus:ring-primary/40" checked={form.allow_jit} onChange={(event) => setForm((c) => ({ ...c, allow_jit: event.target.checked }))} />
                    <div className="flex flex-col gap-0.5">
                      <span>{t('identity.authProviders.detail.jit')}</span>
                      <span className="text-[10px] text-muted-foreground font-medium uppercase tracking-wider">{t('identity.authProviders.detail.jitDesc')}</span>
                    </div>
                  </label>
                </div>
              </div>

              <div className="flex flex-wrap gap-3 pt-4 border-t border-border">
                <Button variant="amber" type="button" onClick={() => void handleSave()} disabled={saving}>{saving ? '...' : selectedID ? t('identity.authProviders.detail.save') : t('identity.authProviders.detail.create')}</Button>
                <Button variant="outline" type="button" onClick={startCreate}>{t('identity.authProviders.detail.reset')}</Button>
              </div>
            </div>
          </RegistryDetail>
        ) : (
          <EmptyDetailState title={t('identity.authProviders.empty.title')} description={t('identity.authProviders.empty.desc')} />
        )}
      />
    </div>
  );
};

export default AuthProvidersPage;
