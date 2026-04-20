import React, { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import { AppLayout } from './components/layout/AppLayout';
import { AuthProvider } from './hooks/AuthProvider';
import { useAuth } from './hooks/useAuth';
import { ThemeProvider } from './hooks/useTheme';
import { I18nProvider } from './hooks/useI18n';
import { fetchBootstrapStatus, getApiErrorMessage } from './lib/api/ops';
import { canAccessIdentity, OPS_ROUTE_PERMISSIONS, hasAnyPermission, hasPermission } from './lib/auth/permissions';
import type { AuthUserSummary } from './lib/api/types';
import { FeatureGateProvider } from './lib/FeatureGateProvider';

import { Toaster } from './components/ui/toaster';

const DashboardView = lazy(() => import('./pages/dashboard/DashboardView').then((module) => ({ default: module.DashboardView })));
const LoginView = lazy(() => import('./pages/ops/LoginView').then((module) => ({ default: module.LoginView })));
const SessionList = lazy(() => import('./pages/sessions/SessionList').then((module) => ({ default: module.SessionList })));
const SessionDetailView = lazy(() => import('./pages/sessions/SessionDetail').then((module) => ({ default: module.SessionDetailView })));
const ExecutionList = lazy(() => import('./pages/executions/ExecutionList').then((module) => ({ default: module.ExecutionList })));
const ExecutionDetailView = lazy(() => import('./pages/executions/ExecutionDetail').then((module) => ({ default: module.ExecutionDetailView })));
const AuditList = lazy(() => import('./pages/audit/AuditList').then((module) => ({ default: module.AuditList })));
const LogsPage = lazy(() => import('./pages/logs/LogsPage').then((module) => ({ default: module.LogsPage })));
const KnowledgeList = lazy(() => import('./pages/knowledge/KnowledgeList').then((module) => ({ default: module.KnowledgeList })));
const ConnectorsList = lazy(() => import('./pages/connectors/ConnectorsList').then((module) => ({ default: module.ConnectorsList })));
const ConnectorDetailView = lazy(() => import('./pages/connectors/ConnectorDetail').then((module) => ({ default: module.ConnectorDetail })));
const SkillsList = lazy(() => import('./pages/skills/SkillsList').then((module) => ({ default: module.SkillsList })));
const SkillDetailView = lazy(() => import('./pages/skills/SkillDetail').then((module) => ({ default: module.SkillDetailView })));
const AutomationsPage = lazy(() => import('./pages/automations/AutomationsPage').then((module) => ({ default: module.AutomationsPage })));
const ExtensionsPage = lazy(() => import('./pages/extensions/ExtensionsPage').then((module) => ({ default: module.ExtensionsPage })));
const ProvidersPage = lazy(() => import('./pages/providers/ProvidersPage').then((module) => ({ default: module.ProvidersPage })));
const ChannelsPage = lazy(() => import('./pages/channels/ChannelsPage').then((module) => ({ default: module.ChannelsPage })));
const MsgTemplatesPage = lazy(() => import('./pages/msg-templates/MsgTemplatesPage').then((module) => ({ default: module.MsgTemplatesPage })));
const OutboxConsole = lazy(() => import('./pages/outbox/OutboxConsole').then((module) => ({ default: module.OutboxConsole })));
const SetupSmokeView = lazy(() => import('./pages/setup/SetupSmokeView').then((module) => ({ default: module.SetupSmokeView })));
const RuntimeChecksPage = lazy(() => import('./pages/setup/SetupSmokeView').then((module) => ({ default: module.RuntimeChecksPage })));
const InboxPage = lazy(() => import('./pages/inbox/InboxPage').then((module) => ({ default: module.InboxPage })));
const ChatPage = lazy(() => import('./pages/chat/ChatPage').then((module) => ({ default: module.ChatPage })));
const TriggersPage = lazy(() => import('./pages/triggers/TriggersPage').then((module) => ({ default: module.TriggersPage })));
const OpsActionView = lazy(() => import('./pages/ops/OpsActionView').then((module) => ({ default: module.OpsActionView })));
const ObservabilityPage = lazy(() => import('./pages/ops/ObservabilityPage').then((module) => ({ default: module.ObservabilityPage })));
const AuthProvidersPage = lazy(() => import('./pages/identity/AuthProvidersPage').then((module) => ({ default: module.AuthProvidersPage })));
const IdentityOverview = lazy(() => import('./pages/identity/IdentityOverview').then((module) => ({ default: module.IdentityOverview })));
const UsersPage = lazy(() => import('./pages/identity/UsersPage').then((module) => ({ default: module.UsersPage })));
const GroupsPage = lazy(() => import('./pages/identity/GroupsPage').then((module) => ({ default: module.GroupsPage })));
const RolesPage = lazy(() => import('./pages/identity/RolesPage').then((module) => ({ default: module.RolesPage })));
const AgentRolesPage = lazy(() => import('./pages/identity/AgentRolesPage').then((module) => ({ default: module.AgentRolesPage })));
const PeoplePage = lazy(() => import('./pages/identity/PeoplePage').then((module) => ({ default: module.PeoplePage })));
const OrgPage = lazy(() => import('./pages/org/OrgPage').then((module) => ({ default: module.OrgPage })));
const DocsView = lazy(() => import('./pages/docs/DocsView').then((module) => ({ default: module.DocsView })));

type BootstrapMode = 'loading' | 'wizard' | 'runtime';

type BootstrapStatus =
  | { state: 'loading'; mode: BootstrapMode; error: string }
  | { state: 'ready'; mode: Exclude<BootstrapMode, 'loading'>; error: string }
  | { state: 'error'; mode: BootstrapMode; error: string };

function defaultRuntimePath(user: AuthUserSummary | null | undefined) {
  if (hasPermission(user, 'sessions.read')) {
    return '/sessions';
  }
  if (hasPermission(user, 'executions.read')) {
    return '/executions';
  }
  return '/runtime';
}

function useBootstrapMode() {
  const { isAuthenticated } = useAuth();
  const [status, setStatus] = React.useState<BootstrapStatus>({ state: 'loading', mode: 'loading', error: '' });
  const [reloadKey, setReloadKey] = React.useState(0);

  React.useEffect(() => {
    let active = true;
    setStatus({ state: 'loading', mode: 'loading', error: '' });
    fetchBootstrapStatus()
      .then((result) => {
        if (!active) {
          return;
        }
        setStatus({
          state: 'ready',
          mode: result.mode === 'runtime' ? 'runtime' : 'wizard',
          error: '',
        });
      })
      .catch((error) => {
        if (!active) {
          return;
        }
        setStatus({
          state: 'error',
          mode: 'loading',
          error: getApiErrorMessage(error, 'Unable to determine setup state.'),
        });
      });
    return () => {
      active = false;
    };
  }, [isAuthenticated, reloadKey]);

  return {
    isAuthenticated,
    mode: status.mode,
    bootstrapState: status.state,
    bootstrapError: status.error,
    retryBootstrap: () => setReloadKey((current) => current + 1),
  };
}

const ProtectedRoute = () => {
  const { isAuthenticated, mode, bootstrapState, bootstrapError, retryBootstrap } = useBootstrapMode();

  if (bootstrapState === 'loading') {
    return <PageFallback />;
  }
  if (bootstrapState === 'error') {
    return <BootstrapErrorView message={bootstrapError} onRetry={retryBootstrap} />;
  }
  if (mode === 'wizard') {
    return <Navigate to="/setup" replace />;
  }
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }
  return <AppLayout />;
};

const SetupRoute = () => {
  const { isAuthenticated, mode, bootstrapState, bootstrapError, retryBootstrap } = useBootstrapMode();

  if (bootstrapState === 'loading') {
    return <PageFallback />;
  }
  if (bootstrapState === 'error') {
    return <BootstrapErrorView message={bootstrapError} onRetry={retryBootstrap} />;
  }
  if (mode === 'wizard') {
    return <SetupSmokeView />;
  }
  return <Navigate to={isAuthenticated ? '/runtime-checks' : '/login'} replace />;
};

const LoginRoute = () => {
  const { isAuthenticated, mode, bootstrapState, bootstrapError, retryBootstrap } = useBootstrapMode();
  const { user } = useAuth();

  if (bootstrapState === 'loading') {
    return <PageFallback />;
  }
  if (bootstrapState === 'error') {
    return <BootstrapErrorView message={bootstrapError} onRetry={retryBootstrap} />;
  }
  if (mode === 'wizard') {
    return <Navigate to="/setup" replace />;
  }
  if (isAuthenticated) {
    return <Navigate to={defaultRuntimePath(user)} replace />;
  }
  return <LoginView />;
};

const LandingRoute = () => {
  const { user } = useAuth();
  return <Navigate to={defaultRuntimePath(user)} replace />;
};

const PermissionRoute = ({ permission, permissionsAny, children }: { permission?: string; permissionsAny?: string[]; children: React.ReactNode }) => {
  const { user } = useAuth();
  const allowed = permission
    ? hasPermission(user, permission)
    : permissionsAny && permissionsAny.length > 0
      ? hasAnyPermission(user, permissionsAny)
      : true;
  if (!allowed) {
    return <Navigate to={defaultRuntimePath(user)} replace />;
  }
  return <>{children}</>;
};

const IdentityRoute = () => {
  const { user } = useAuth();
  if (!canAccessIdentity(user)) {
    return <Navigate to={defaultRuntimePath(user)} replace />;
  }
  return <Outlet />;
};

const PageFallback = () => (
  <div className="p-12 text-muted-foreground animate-pulse font-medium">
    Loading...
  </div>
);

const BootstrapErrorView = ({ message, onRetry }: { message: string; onRetry: () => void }) => (
  <div className="min-h-screen grid place-items-center p-6">
    <div className="w-full max-w-md rounded-2xl border border-danger/20 bg-danger/5 p-6 text-center shadow-sm">
      <div className="text-base font-semibold text-foreground">Unable to determine setup state.</div>
      <p className="mt-2 text-sm text-muted-foreground">{message}</p>
      <button
        type="button"
        className="mt-4 inline-flex items-center justify-center rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted"
        onClick={onRetry}
      >
        Retry
      </button>
    </div>
  </div>
);

function App() {
  return (
    <ThemeProvider>
      <Toaster />
      <I18nProvider>
        <AuthProvider>
          <FeatureGateProvider>
            <BrowserRouter>
              <Suspense fallback={<PageFallback />}>
                <Routes>
                  <Route path="/login" element={<LoginRoute />} />
                  <Route path="/setup" element={<SetupRoute />} />

                  <Route element={<ProtectedRoute />}>
                    <Route index element={<LandingRoute />} />
                    <Route path="runtime" element={<DashboardView />} />
                    <Route path="dashboard" element={<Navigate to="/runtime" replace />} />
                    <Route path="runtime-checks" element={<RuntimeChecksPage />} />
                    <Route path="sessions" element={<PermissionRoute permission="sessions.read"><Outlet /></PermissionRoute>}>
                      <Route index element={<SessionList />} />
                      <Route path=":id" element={<SessionDetailView />} />
                    </Route>
                    <Route path="executions" element={<PermissionRoute permission="executions.read"><Outlet /></PermissionRoute>}>
                      <Route index element={<ExecutionList />} />
                      <Route path=":id" element={<ExecutionDetailView />} />
                    </Route>
                    <Route path="inbox" element={<InboxPage />} />
                    <Route path="chat" element={<ChatPage />} />
                    <Route path="triggers" element={<PermissionRoute permission="configs.read"><TriggersPage /></PermissionRoute>} />
                    <Route path="connectors" element={<PermissionRoute permission="connectors.read"><Outlet /></PermissionRoute>}>
                      <Route index element={<ConnectorsList />} />
                      <Route path=":id" element={<ConnectorDetailView />} />
                    </Route>
                    <Route path="skills" element={<PermissionRoute permission="skills.read"><Outlet /></PermissionRoute>}>
                      <Route index element={<SkillsList />} />
                      <Route path=":id" element={<SkillDetailView />} />
                    </Route>
                    <Route path="automations" element={<PermissionRoute permission="platform.read"><AutomationsPage /></PermissionRoute>} />
                    <Route path="extensions" element={<PermissionRoute permission="skills.read"><ExtensionsPage /></PermissionRoute>} />
                    <Route path="providers" element={<PermissionRoute permission="providers.read"><ProvidersPage /></PermissionRoute>} />
                    <Route path="channels" element={<PermissionRoute permission="channels.read"><ChannelsPage /></PermissionRoute>} />
                    <Route path="notification-templates" element={<PermissionRoute permission="configs.read"><MsgTemplatesPage /></PermissionRoute>} />
                    <Route path="identity" element={<IdentityRoute />}>
                      <Route index element={<IdentityOverview />} />
                      <Route path="providers" element={<PermissionRoute permission="auth.read"><AuthProvidersPage /></PermissionRoute>} />
                      <Route path="users" element={<PermissionRoute permission="users.read"><UsersPage /></PermissionRoute>} />
                      <Route path="groups" element={<PermissionRoute permission="groups.read"><GroupsPage /></PermissionRoute>} />
                      <Route path="roles" element={<PermissionRoute permission="roles.read"><RolesPage /></PermissionRoute>} />
                      <Route path="agent-roles" element={<PermissionRoute permission="platform.read"><AgentRolesPage /></PermissionRoute>} />
                      <Route path="people" element={<PermissionRoute permission="people.read"><PeoplePage /></PermissionRoute>} />
                    </Route>
                    <Route path="auth" element={<Navigate to="/identity/providers" replace />} />
                    <Route path="audit" element={<PermissionRoute permission="audit.read"><AuditList /></PermissionRoute>} />
                    <Route path="logs" element={<PermissionRoute permission="audit.read"><LogsPage /></PermissionRoute>} />
                    <Route path="knowledge" element={<PermissionRoute permission="knowledge.read"><KnowledgeList /></PermissionRoute>} />
                    <Route path="outbox" element={<PermissionRoute permission="outbox.read"><OutboxConsole /></PermissionRoute>} />
                    <Route path="users" element={<Navigate to="/identity/users" replace />} />
                    <Route path="groups" element={<Navigate to="/identity/groups" replace />} />
                    <Route path="roles" element={<Navigate to="/identity/roles" replace />} />
                    <Route path="people" element={<Navigate to="/identity/people" replace />} />
                    <Route path="ops" element={<PermissionRoute permissionsAny={OPS_ROUTE_PERMISSIONS}><OpsActionView /></PermissionRoute>} />
                    <Route path="ops/observability" element={<PermissionRoute permission="platform.read"><ObservabilityPage /></PermissionRoute>} />
                    <Route path="org" element={<PermissionRoute permission="org.read"><OrgPage /></PermissionRoute>} />
                    <Route path="docs" element={<Outlet />}>
                      <Route index element={<DocsView />} />
                      <Route path=":slug" element={<DocsView />} />
                    </Route>
                  </Route>
                  <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
              </Suspense>
            </BrowserRouter>
          </FeatureGateProvider>
        </AuthProvider>
      </I18nProvider>
    </ThemeProvider>
  );
}

export default App;
