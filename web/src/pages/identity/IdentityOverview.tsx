import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { fetchAccessConfig, fetchAuthProviders, fetchAuthSessions, fetchGroups, fetchPeople, fetchRoles, fetchUsers, setAuthProviderEnabled } from '../../lib/api/access';
import { getApiErrorMessage } from '../../lib/api/ops';
import { useAuth } from '../../hooks/useAuth';
import { hasPermission } from '../../lib/auth/permissions';
import type { AccessConfigResponse, AuthProviderInfo, SessionInventoryItem } from '../../lib/api/types';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { InlineStatus } from '@/components/ui/inline-status';
import { CollapsibleList } from '@/components/ui/collapsible-list';
import { SectionTitle, StatCard, SummaryGrid } from '@/components/ui/page-hero';
import { ActiveBadge as StatusBadge } from '@/components/ui/active-badge';
import { useI18n } from '@/hooks/useI18n';

export const IdentityOverview = () => {
  const { user } = useAuth();
  const { t } = useI18n();
  const [providers, setProviders] = useState<AuthProviderInfo[]>([]);
  const [sessions, setSessions] = useState<SessionInventoryItem[]>([]);
  const [config, setConfig] = useState<AccessConfigResponse | null>(null);
  const [counts, setCounts] = useState({ users: 0, groups: 0, roles: 0, people: 0, sessions: 0 });
  const [loading, setLoading] = useState(true);
  const [busyKey, setBusyKey] = useState('');
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setLoading(true);
      setError('');
      const canReadAuth = hasPermission(user, 'auth.read');
      const canReadUsers = hasPermission(user, 'users.read');
      const canReadGroups = hasPermission(user, 'groups.read');
      const canReadRoles = hasPermission(user, 'roles.read');
      const canReadPeople = hasPermission(user, 'people.read');

      const [authProviders, accessConfig, usersResp, groupsResp, rolesResp, peopleResp, sessionsResp] = await Promise.all([
        canReadAuth ? fetchAuthProviders() : Promise.resolve({ items: [] }),
        canReadAuth ? fetchAccessConfig() : Promise.resolve(null),
        canReadUsers ? fetchUsers({ page: 1, limit: 1 }) : Promise.resolve({ total: 0 }),
        canReadGroups ? fetchGroups({ page: 1, limit: 1 }) : Promise.resolve({ total: 0 }),
        canReadRoles ? fetchRoles() : Promise.resolve({ items: [] }),
        canReadPeople ? fetchPeople({ page: 1, limit: 1 }) : Promise.resolve({ total: 0 }),
        fetchAuthSessions(),
      ]);
      setProviders(authProviders.items || []);
      setConfig(accessConfig);
      setSessions(sessionsResp.items || []);
      setCounts({
        users: usersResp.total || 0,
        groups: groupsResp.total || 0,
        roles: rolesResp.items?.length || 0,
        people: peopleResp.total || 0,
        sessions: sessionsResp.items?.length || 0,
      });
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, 'Failed to load identity overview.'));
    } finally {
      setLoading(false);
    }
  }, [user]);

  useEffect(() => {
    void load();
  }, [load]);

  const runToggle = useCallback(async (providerID: string, enabled: boolean) => {
    try {
      setBusyKey(providerID);
      setError('');
      await setAuthProviderEnabled(providerID, enabled);
      await load();
    } catch (toggleError) {
      setError(getApiErrorMessage(toggleError, 'Failed to update auth provider state.'));
    } finally {
      setBusyKey('');
    }
  }, [load]);

  const scopeLabel = (permitted: boolean) => permitted ? t('identity.overview.scope.visible') : t('identity.overview.scope.hidden');

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <SectionTitle
        title={t('identity.overview.title')}
        subtitle={t('identity.overview.subtitle')}
      />

      {error ? <InlineStatus type="error" message={error} /> : null}

      <SummaryGrid className="xl:grid-cols-6">
        <StatCard title={t('identity.overview.stats.providers')} value={String(providers.length)} subtitle={t('identity.overview.stats.providers')} />
        <StatCard title={t('identity.overview.stats.users')} value={String(counts.users)} subtitle={t('identity.overview.stats.users')} />
        <StatCard title={t('identity.overview.stats.groups')} value={String(counts.groups)} subtitle={t('identity.overview.stats.groups')} />
        <StatCard title={t('identity.overview.stats.roles')} value={String(counts.roles)} subtitle={t('identity.overview.stats.roles')} />
        <StatCard title={t('identity.overview.stats.people')} value={String(counts.people)} subtitle={t('identity.overview.stats.people')} />
        <StatCard title={t('identity.overview.stats.sessions')} value={String(counts.sessions)} subtitle={t('identity.overview.stats.sessions')} />
      </SummaryGrid>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[1.4fr_1fr]">
        <Card className="p-0 border-border bg-card shadow-sm rounded-2xl overflow-hidden">
          <CardHeader className="flex flex-col gap-4 border-b border-border bg-muted/30 md:flex-row md:items-start md:justify-between p-6">
            <div className="flex flex-col gap-1.5">
              <CardTitle>{t('identity.overview.providers.title')}</CardTitle>
              <CardDescription>{t('identity.overview.providers.desc')}</CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {hasPermission(user, 'auth.read') ? <QuickLink to="/identity/providers" label={t('identity.overview.stats.providers')} /> : null}
              {hasPermission(user, 'users.read') ? <QuickLink to="/identity/users" label={t('identity.users.title')} /> : null}
              {hasPermission(user, 'groups.read') ? <QuickLink to="/identity/groups" label={t('identity.groups.title')} /> : null}
              {hasPermission(user, 'roles.read') ? <QuickLink to="/identity/roles" label={t('identity.roles.title')} /> : null}
              {hasPermission(user, 'platform.read') ? <QuickLink to="/identity/agent-roles" label={t('identity.agentRoles.title')} /> : null}
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 p-6">
            {loading ? (
                <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
            ) : (
              <CollapsibleList
                limit={4}
                emptyText={t('identity.agentRoles.sidebar.empty')}
                items={providers.map((item) => (
                  <div key={item.id} className="grid gap-4 rounded-xl border border-border bg-muted/20 p-4 md:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_auto_auto] md:items-center">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-bold text-foreground">{item.name || item.id}</div>
                      <div className="mt-1 truncate font-mono text-[10px] text-muted-foreground uppercase opacity-70">{item.id}</div>
                    </div>
                    <div className="text-xs text-muted-foreground font-medium uppercase tracking-wider">{item.type || 'unknown'}</div>
                    <StatusBadge active={item.enabled} label={item.enabled ? t('status.active') : t('status.inactive')} />
                    <div className="md:justify-self-end">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 text-xs font-bold"
                        onClick={() => void runToggle(item.id || '', !item.enabled)}
                        disabled={!item.id || busyKey === item.id}
                      >
                        {busyKey === item.id ? '...' : item.enabled ? t('action.disable') : t('action.enable')}
                      </Button>
                    </div>
                  </div>
                ))}
              />
            )}
          </CardContent>
        </Card>

        <Card className="p-0 border-border bg-card shadow-sm rounded-2xl overflow-hidden">
          <CardHeader className="border-b border-border bg-muted/30 p-6">
              <CardTitle>{t('identity.overview.config.title')}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 p-6 text-sm text-muted-foreground">
            <div className="flex justify-between border-b border-border pb-2">
              <span>{t('identity.overview.config.configured')}</span>
              <strong className="text-foreground">{config?.configured ? t('common.yes') : t('common.no')}</strong>
            </div>
            <div className="flex justify-between border-b border-border pb-2">
              <span>{t('identity.overview.config.loaded')}</span>
              <strong className="text-foreground">{config?.loaded ? t('common.yes') : t('common.no')}</strong>
            </div>
            <div className="flex justify-between border-b border-border pb-2">
              <span>{t('identity.overview.config.path')}</span>
              <strong className="text-foreground truncate ml-4">{config?.path || t('identity.overview.config.inMemory')}</strong>
            </div>
            <div className="flex justify-between pb-2">
              <span>{t('identity.overview.config.updated')}</span>
              <strong className="text-foreground">{config?.updated_at || 'n/a'}</strong>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <Card className="p-0 border-border bg-card shadow-sm rounded-2xl overflow-hidden">
          <CardHeader className="flex flex-col gap-4 border-b border-border bg-muted/30 md:flex-row md:items-start md:justify-between p-6">
            <div className="flex flex-col gap-1.5">
              <CardTitle>{t('identity.overview.sessions.title')}</CardTitle>
              <CardDescription>{t('identity.overview.sessions.desc')}</CardDescription>
            </div>
          </CardHeader>
          <CardContent className="p-6">
            {loading ? (
                <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
            ) : sessions.length === 0 ? (
              <p className="text-sm text-muted-foreground italic">{t('identity.overview.sessions.empty')}</p>
            ) : (
              <CollapsibleList
                limit={4}
                items={sessions.map((session, index) => (
                  <div key={`${session.token_masked || 'session'}-${index}`} className="rounded-xl border border-border bg-muted/10 p-4 mb-2">
                    <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-bold text-foreground">{session.user_id || 'unknown'}</div>
                        <div className="mt-1 truncate font-mono text-[10px] text-muted-foreground opacity-60">{session.token_masked || 'token masked'}</div>
                      </div>
                      <StatusBadge active={true} label={session.provider_id || 'session'} />
                    </div>
                    <div className="mt-4 grid gap-3 text-[10px] font-bold uppercase tracking-wider text-muted-foreground sm:grid-cols-2">
                      <div>{t('identity.overview.sessions.sessionCreated', { time: session.created_at ? new Date(session.created_at).toLocaleString() : 'n/a' })}</div>
                      <div>{t('identity.overview.sessions.sessionExpires', { time: session.expires_at ? new Date(session.expires_at).toLocaleString() : 'n/a' })}</div>
                    </div>
                  </div>
                ))}
              />
            )}
          </CardContent>
        </Card>

        <Card className="p-0 border-border bg-card shadow-sm rounded-2xl overflow-hidden">
          <CardHeader className="border-b border-border bg-muted/30 p-6">
              <CardTitle>{t('identity.overview.scope.title')}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-4 p-6 text-sm text-muted-foreground">
            <div className="flex justify-between items-center bg-muted/20 p-2 px-3 rounded-lg border border-border">
              <span className="font-bold text-foreground uppercase text-xs tracking-widest">{t('identity.overview.scope.providers')}</span>
              <StatusBadge active={hasPermission(user, 'auth.read')} label={scopeLabel(hasPermission(user, 'auth.read'))} />
            </div>
            <div className="flex justify-between items-center bg-muted/20 p-2 px-3 rounded-lg border border-border">
              <span className="font-bold text-foreground uppercase text-xs tracking-widest">{t('identity.overview.scope.users')}</span>
              <StatusBadge active={hasPermission(user, 'users.read')} label={scopeLabel(hasPermission(user, 'users.read'))} />
            </div>
            <div className="flex justify-between items-center bg-muted/20 p-2 px-3 rounded-lg border border-border">
              <span className="font-bold text-foreground uppercase text-xs tracking-widest">{t('identity.overview.scope.groups')}</span>
              <StatusBadge active={hasPermission(user, 'groups.read')} label={scopeLabel(hasPermission(user, 'groups.read'))} />
            </div>
            <div className="flex justify-between items-center bg-muted/20 p-2 px-3 rounded-lg border border-border">
              <span className="font-bold text-foreground uppercase text-xs tracking-widest">{t('identity.overview.scope.roles')}</span>
              <StatusBadge active={hasPermission(user, 'roles.read')} label={scopeLabel(hasPermission(user, 'roles.read'))} />
            </div>
            <div className="flex justify-between items-center bg-muted/20 p-2 px-3 rounded-lg border border-border">
              <span className="font-bold text-foreground uppercase text-xs tracking-widest">{t('identity.overview.scope.people')}</span>
              <StatusBadge active={hasPermission(user, 'people.read')} label={scopeLabel(hasPermission(user, 'people.read'))} />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

const QuickLink = ({ to, label }: { to: string; label: string }) => (
  <Button variant="outline" size="sm" asChild className="h-8 text-xs font-bold">
    <Link to={to}>{label}</Link>
  </Button>
);
