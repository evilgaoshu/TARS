import { useEffect, useMemo, useState } from 'react';
import type { FormEvent, ReactNode } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  completeSetupWizard,
  checkSetupWizardProvider,
  fetchSecretsInventory,
  fetchSetupStatus,
  fetchSetupWizard,
  getApiErrorMessage,
  saveSetupWizardAdmin,
  saveSetupWizardChannel,
  saveSetupWizardProvider,
  triggerSmokeAlert,
} from '../../lib/api/ops';
import type {
  RuntimeSetupStatus,
  SecretDescriptor,
  SetupInitializationStatus,
  SetupStatusResponse,
  SetupWizardResponse,
  SmokeAlertResponse,
  SmokeSessionStatus,
} from '../../lib/api/types';
import { useAuth } from '../../hooks/useAuth';
import { loginWithPassword } from '../../lib/api/access';
import { saveStoredSession } from '../../lib/auth/storage';
import {
  Activity,
  AlertCircle,
  ArrowRight,
  CheckCircle2,
  ChevronRight,
  Cpu,
  Database,
  Eye,
  EyeOff,
  ExternalLink,
  Key,
  LayoutDashboard,
  MessageSquare,
  Play,
  RefreshCcw,
  Server,
  Settings2,
  ShieldAlert,
  Sparkles,
  UserCog,
  Zap,
} from 'lucide-react';
import { clsx } from 'clsx';
import { Button } from '../../components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Input } from '../../components/ui/input';
import { LabeledField } from '@/components/ui/labeled-field';
import { StatusBadge } from '@/components/ui/status-badge';
import { OperatorHero } from '@/components/operator/OperatorPage';
import { Checkbox } from '@/components/ui/checkbox';
import { useI18n } from '../../hooks/useI18n';

const defaultForm = {
  alertname: 'TarsSmokeManual',
  service: 'sshd',
  host: '',
  severity: 'critical',
  summary: 'Manual runtime check triggered from Runtime Checks.',
};

const defaultAdminForm = {
  username: 'admin',
  display_name: 'Platform Admin',
  email: '',
  password: '',
};

const defaultProviderForm = {
  provider_id: 'primary-openai',
  vendor: 'openai',
  protocol: 'openai_compatible',
  base_url: '',
  api_key: '',
  api_key_ref: '',
  model: 'gpt-4o-mini',
};

const defaultChannelForm = {
  channel_id: 'inbox-primary',
  name: 'Primary Inbox',
  kind: 'in_app_inbox',
  usages: ['approval', 'notifications'],
  target: 'default',
};

const setupChannelUsageOptions = ['approval', 'notifications', 'conversation_entry', 'alerts'];
const providerVendorOptions = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'dashscope', label: 'DashScope' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'ollama', label: 'Ollama' },
];
const providerVendorDefaults: Record<string, { protocol: string; base_url: string }> = {
  openai: { protocol: 'openai_compatible', base_url: 'https://api.openai.com/v1' },
  dashscope: { protocol: 'openai_compatible', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  openrouter: { protocol: 'openai_compatible', base_url: 'https://openrouter.ai/api/v1' },
  anthropic: { protocol: 'anthropic', base_url: 'https://api.anthropic.com' },
  ollama: { protocol: 'ollama', base_url: 'http://localhost:11434' },
};
const setupChannelKindOptions = [
  { value: 'in_app_inbox', label: 'In-app Inbox' },
  { value: 'telegram', label: 'Telegram' },
  { value: 'slack', label: 'Slack' },
  { value: 'email', label: 'Email' },
];

function toggleSelection(values: string[], value: string, checked: boolean): string[] {
  if (checked) {
    return values.includes(value) ? values : [...values, value];
  }
  return values.filter((item) => item !== value);
}

export const SetupSmokeView = () => {
  const navigate = useNavigate();
  const { isAuthenticated, refresh } = useAuth();
  const { lang, setLang, t } = useI18n();
  const [status, setStatus] = useState<SetupStatusResponse | null>(null);
  const [wizard, setWizard] = useState<SetupWizardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [submitError, setSubmitError] = useState('');
  const [adminError, setAdminError] = useState('');
  const [providerError, setProviderError] = useState('');
  const [channelError, setChannelError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [adminForm, setAdminForm] = useState(defaultAdminForm);
  const [providerForm, setProviderForm] = useState(defaultProviderForm);
  const [channelForm, setChannelForm] = useState(defaultChannelForm);
  const [providerCheckMessage, setProviderCheckMessage] = useState('');
  const [adminMessage, setAdminMessage] = useState('');
  const [channelMessage, setChannelMessage] = useState('');
  const [finishingLogin, setFinishingLogin] = useState(false);
  const [providerAdvancedOpen, setProviderAdvancedOpen] = useState(false);
  const [channelAdvancedOpen, setChannelAdvancedOpen] = useState(false);
  const [showAdminPassword, setShowAdminPassword] = useState(false);
  const [showProviderApiKey, setShowProviderApiKey] = useState(false);

  const loadStatus = async () => {
    try {
      setLoading(true);
      setError('');
      const wizardResponse = await fetchSetupWizard();
      setWizard(wizardResponse);
      const setupResponse = await fetchSetupStatus();
      setStatus(setupResponse);
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, 'Failed to load setup status.'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadStatus();
  }, []);

  useEffect(() => {
    if (wizard?.admin.user?.user_id) {
      setAdminForm((current) => ({
        ...current,
        username: wizard.admin.user.user_id || current.username,
        display_name: wizard.admin.user.display_name || current.display_name,
        email: wizard.admin.user.email || current.email,
      }));
    }
    if (wizard?.provider.provider?.id) {
      setProviderForm((current) => ({
        ...current,
        provider_id: wizard.provider.provider.id || current.provider_id,
        vendor: wizard.provider.provider.vendor || current.vendor,
        protocol: wizard.provider.provider.protocol || current.protocol,
        base_url: wizard.provider.provider.base_url || current.base_url,
        api_key: '',
        api_key_ref: wizard.provider.provider.api_key_ref || current.api_key_ref,
        model: wizard.provider.provider.primary_model || current.model,
      }));
    }
    if (wizard?.channel.channel?.id) {
      setChannelForm((current) => ({
        ...current,
        channel_id: wizard.channel.channel.id || current.channel_id,
        name: wizard.channel.channel.name || current.name,
        kind: wizard.channel.channel.kind || wizard.channel.channel.type || current.kind,
        usages: wizard.channel.channel.usages || wizard.channel.channel.capabilities || current.usages,
        target: wizard.channel.channel.target || current.target,
      }));
    }
  }, [wizard]);

  useEffect(() => {
    const vendorDefaults = providerVendorDefaults[providerForm.vendor];
    if (!vendorDefaults || providerForm.base_url.trim() !== '') {
      return;
    }
    setProviderForm((current) => {
      if (current.base_url.trim() !== '') {
        return current;
      }
      return {
        ...current,
        protocol: current.protocol || vendorDefaults.protocol,
        base_url: vendorDefaults.base_url,
      };
    });
  }, [providerForm.base_url, providerForm.vendor]);

  const initialization = wizard?.initialization || status?.initialization;
  const canEnterRuntime = Boolean(initialization?.initialized && isAuthenticated);
  const adminPasswordState = useMemo(() => evaluateSetupPassword(adminForm.password), [adminForm.password]);
  const canSaveAdmin = adminForm.username.trim().length > 0 && adminForm.password.trim().length > 0 && adminPasswordState.valid;
  const canCheckProvider = providerForm.provider_id.trim().length > 0
    && providerForm.base_url.trim().length > 0
    && (providerForm.api_key.trim().length > 0 || providerForm.api_key_ref.trim().length > 0);
  const canSaveProvider = canCheckProvider && providerForm.model.trim().length > 0;
  const telegramConfigured = Boolean(status?.telegram?.configured);
  const telegramRequiresToken = channelForm.kind === 'telegram' && !telegramConfigured;
  const canSaveChannel = channelForm.channel_id.trim().length > 0
    && channelForm.target.trim().length > 0
    && !telegramRequiresToken;
  const providerSecretPreview = providerForm.api_key_ref.trim() || `secret://providers/${providerForm.provider_id.trim() || 'primary-openai'}/api-key`;

  const steps = useMemo(() => {
    const state = wizard?.initialization || status?.initialization;
    const nextVisibleStep = state?.next_step === 'auth' ? 'provider' : state?.next_step;
    return [
      { id: 'admin', label: '首个管理员', done: Boolean(state?.admin_configured), icon: <UserCog size={16} /> },
      { id: 'provider', label: '默认推理提供方', done: Boolean(state?.provider_ready), icon: <Cpu size={16} />, active: nextVisibleStep === 'provider' },
      { id: 'channel', label: '第一方入口与触达', done: Boolean(state?.channel_ready), icon: <MessageSquare size={16} /> },
    ];
  }, [status?.initialization, wizard?.initialization]);

  const withStepAction = async (
    action: () => Promise<SetupWizardResponse>,
    onSuccess?: (result: SetupWizardResponse) => void,
    onError?: (message: string) => void,
    options?: { reloadStatus?: boolean },
  ) => {
    try {
      setSubmitting(true);
      setSubmitError('');
      const result = await action();
      setWizard(result);
      onSuccess?.(result);
      if (options?.reloadStatus !== false) {
        await loadStatus();
      }
    } catch (submitErr) {
      const message = getApiErrorMessage(submitErr, 'Failed to save setup step.');
      if (onError) {
        onError(message);
      } else {
        setSubmitError(message);
      }
    } finally {
      setSubmitting(false);
    }
  };

  const handleComplete = async () => {
    setChannelError('');
    await withStepAction(async () => {
      const result = await completeSetupWizard();
      const loginHint = result.initialization.login_hint;
      const adminPassword = adminForm.password.trim();
      if (!isAuthenticated && loginHint?.provider === 'local_password' && loginHint.username && adminPassword) {
        try {
          setFinishingLogin(true);
          const auth = await loginWithPassword(loginHint.provider, loginHint.username, adminPassword);
          if (auth.session_token) {
            saveStoredSession({
              token: auth.session_token,
              user: {
                id: auth.user.user_id || auth.user.username || loginHint.username,
                username: auth.user.username || loginHint.username,
                displayName: auth.user.display_name || auth.user.username || loginHint.username,
                email: auth.user.email || '',
                roles: auth.roles || [],
                permissions: auth.permissions || [],
                authSource: auth.provider_id || loginHint.provider,
                breakGlass: false,
              },
              roles: auth.roles || [],
              permissions: auth.permissions || [],
              authSource: auth.provider_id || loginHint.provider,
              breakGlass: false,
            });
            await refresh();
            navigate('/runtime-checks');
            return result;
          }
        } catch {
          // fall through to guided login route
        } finally {
          setFinishingLogin(false);
        }
      }
      if (isAuthenticated) {
        navigate('/runtime-checks');
      } else {
        navigate(loginHint?.login_url || '/login');
      }
      return result;
    }, undefined, setChannelError, { reloadStatus: false });
  };

  const handleCheckProvider = async () => {
    try {
      setSubmitting(true);
      setSubmitError('');
      const result = await checkSetupWizardProvider({
        provider_id: providerForm.provider_id,
        vendor: providerForm.vendor,
        protocol: providerForm.protocol,
        base_url: providerForm.base_url,
        api_key: providerForm.api_key,
        api_key_ref: providerForm.api_key_ref,
        model: providerForm.model,
      });
      setProviderError('');
      setProviderCheckMessage(result.available ? `Connectivity check passed: ${result.detail || 'provider is reachable'}` : `Connectivity check failed: ${result.detail || 'provider is unavailable'}`);
    } catch (submitErr) {
      setProviderCheckMessage('');
      setProviderError(getApiErrorMessage(submitErr, 'Failed to check provider connectivity.'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex justify-end">
        <div className="inline-flex items-center rounded-md border border-border bg-[var(--bg-surface-solid)] p-0.5">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className={clsx('h-7 px-2.5 rounded text-xs font-semibold', lang === 'en-US' && 'bg-[var(--bg-surface)] text-[var(--text-primary)]')}
            onClick={() => void setLang('en-US')}
            aria-label={t('header.languageEnglish')}
          >
            EN
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className={clsx('h-7 px-2.5 rounded text-xs font-semibold', lang === 'zh-CN' && 'bg-[var(--bg-surface)] text-[var(--text-primary)]')}
            onClick={() => void setLang('zh-CN')}
            aria-label={t('header.languageChinese')}
          >
            中
          </Button>
        </div>
      </div>
      <OperatorHero
        eyebrow="First-run Setup"
        title="Platform Setup"
        description="Create the first administrator, point TARS at a default provider, and choose the first delivery channel. Once setup is complete, you will continue in Runtime Checks."
        chips={[{ label: 'first-run', tone: 'warning' }, { label: 'local auth by default', tone: 'info' }]}
        className="border-white/[0.08] bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02))] shadow-[0_24px_70px_-34px_rgba(0,0,0,0.82)]"
        primaryAction={
          <Button variant="secondary" className="flex items-center gap-2" onClick={() => void loadStatus()} disabled={loading}>
            <RefreshCcw size={14} className={loading ? 'animate-spin' : ''} />
            Refresh
          </Button>
        }
      />

      {error && (
        <div className="rounded-xl border border-danger/20 bg-danger/10 px-4 py-3 flex items-center gap-3 text-sm text-danger">
          <AlertCircle size={16} /> {error}
        </div>
      )}

      {submitError && (
        <div className="rounded-xl border border-danger/20 bg-danger/10 px-4 py-3 flex items-center gap-3 text-sm text-danger">
          <AlertCircle size={16} /> {submitError}
        </div>
      )}

      <WizardMode
        loading={loading}
        initialization={initialization}
        wizard={wizard}
        steps={steps}
        adminForm={adminForm}
        setAdminForm={setAdminForm}
        adminPasswordState={adminPasswordState}
        showAdminPassword={showAdminPassword}
        setShowAdminPassword={setShowAdminPassword}
        providerForm={providerForm}
        setProviderForm={setProviderForm}
        providerAdvancedOpen={providerAdvancedOpen}
        setProviderAdvancedOpen={setProviderAdvancedOpen}
        channelForm={channelForm}
        setChannelForm={setChannelForm}
        channelAdvancedOpen={channelAdvancedOpen}
        setChannelAdvancedOpen={setChannelAdvancedOpen}
        submitting={submitting}
        onSaveAdmin={() => {
          setAdminError('');
          void withStepAction(() => saveSetupWizardAdmin(adminForm), () => {
            setAdminError('');
            setAdminMessage('Administrator saved. Local password sign-in is ready for first login.')
          }, setAdminError);
        }}
        onSaveProvider={() => {
          setProviderError('');
          void withStepAction(() => saveSetupWizardProvider(providerForm), (result) => {
            setProviderError('');
            setProviderCheckMessage(result.initialization.provider_check_note || 'Default provider saved and verified.')
            setProviderForm((current) => ({
              ...current,
              api_key: '',
              api_key_ref: result.provider.provider.api_key_ref || current.api_key_ref || providerSecretPreview,
            }))
          }, setProviderError);
        }}
        onCheckProvider={() => void handleCheckProvider()}
        onSaveChannel={() => {
          setChannelError('');
          void withStepAction(() => saveSetupWizardChannel({ ...channelForm, type: channelForm.kind }), () => {
            setChannelError('');
            setChannelMessage(`${channelForm.name || channelForm.channel_id} saved as the primary delivery path.`)
          }, setChannelError);
        }}
        onComplete={() => void handleComplete()}
        canEnterRuntime={canEnterRuntime}
        canSaveAdmin={canSaveAdmin}
        canCheckProvider={canCheckProvider}
        canSaveProvider={canSaveProvider}
        canSaveChannel={canSaveChannel}
        providerCheckMessage={providerCheckMessage}
        adminMessage={adminMessage}
        adminError={adminError}
        channelMessage={channelMessage}
        providerError={providerError}
        channelError={channelError}
        finishingLogin={finishingLogin}
        telegramConfigured={telegramConfigured}
        telegramRequiresToken={telegramRequiresToken}
        showProviderApiKey={showProviderApiKey}
        setShowProviderApiKey={setShowProviderApiKey}
        providerSecretPreview={providerSecretPreview}
      />
    </div>
  );
};

const WizardMode = ({
  loading,
  initialization,
  wizard,
  steps,
  adminForm,
  setAdminForm,
  adminPasswordState,
  showAdminPassword,
  setShowAdminPassword,
  providerForm,
  setProviderForm,
  providerAdvancedOpen,
  setProviderAdvancedOpen,
  channelForm,
  setChannelForm,
  channelAdvancedOpen,
  setChannelAdvancedOpen,
  submitting,
  onSaveAdmin,
  onSaveProvider,
  onCheckProvider,
  onSaveChannel,
  onComplete,
  canEnterRuntime,
  canSaveAdmin,
  canCheckProvider,
  canSaveProvider,
  canSaveChannel,
  providerCheckMessage,
  adminMessage,
  adminError,
  channelMessage,
  providerError,
  channelError,
  finishingLogin,
  telegramConfigured,
  telegramRequiresToken,
  showProviderApiKey,
  setShowProviderApiKey,
  providerSecretPreview,
}: {
  loading: boolean;
  initialization?: SetupInitializationStatus;
  wizard: SetupWizardResponse | null;
  steps: Array<{ id: string; label: string; done: boolean; icon: ReactNode }>;
  adminForm: typeof defaultAdminForm;
  setAdminForm: React.Dispatch<React.SetStateAction<typeof defaultAdminForm>>;
  adminPasswordState: SetupPasswordState;
  showAdminPassword: boolean;
  setShowAdminPassword: React.Dispatch<React.SetStateAction<boolean>>;
  providerForm: typeof defaultProviderForm;
  setProviderForm: React.Dispatch<React.SetStateAction<typeof defaultProviderForm>>;
  providerAdvancedOpen: boolean;
  setProviderAdvancedOpen: React.Dispatch<React.SetStateAction<boolean>>;
  channelForm: typeof defaultChannelForm;
  setChannelForm: React.Dispatch<React.SetStateAction<typeof defaultChannelForm>>;
  channelAdvancedOpen: boolean;
  setChannelAdvancedOpen: React.Dispatch<React.SetStateAction<boolean>>;
  submitting: boolean;
  onSaveAdmin: () => void;
  onSaveProvider: () => void;
  onCheckProvider: () => void;
  onSaveChannel: () => void;
  onComplete: () => void;
  canEnterRuntime: boolean;
  canSaveAdmin: boolean;
  canCheckProvider: boolean;
  canSaveProvider: boolean;
  canSaveChannel: boolean;
  providerCheckMessage: string;
  adminMessage: string;
  adminError: string;
  channelMessage: string;
  providerError: string;
  channelError: string;
  finishingLogin: boolean;
  telegramConfigured: boolean;
  telegramRequiresToken: boolean;
  showProviderApiKey: boolean;
  setShowProviderApiKey: React.Dispatch<React.SetStateAction<boolean>>;
  providerSecretPreview: string;
}) => (
  <div className="grid grid-cols-1 xl:grid-cols-[280px_1fr] gap-8">
    {/* Progress sidebar */}
    <div className="flex flex-col gap-6">
      <div className="space-y-4">
        <div className="text-[0.68rem] font-black uppercase tracking-[0.18em] text-muted-foreground flex items-center gap-2 px-1">
          <Sparkles size={12} className="text-primary" /> Progress
        </div>
        <div className="flex flex-col gap-1">
          {steps.map((step, index) => {
            const active = ('active' in step && step.active) || initialization?.next_step === step.id || (!initialization?.next_step && index === 0);
            return (
              <div
                key={step.id}
                className={clsx(
                  'group flex items-center gap-3 rounded-xl px-4 py-3 transition-all',
                  step.done 
                    ? 'text-success hover:bg-success/5' 
                    : active 
                      ? 'bg-primary/5 text-primary' 
                      : 'text-muted-foreground hover:bg-white/5',
                )}
              >
                <div className={clsx(
                  'flex size-8 items-center justify-center rounded-lg border text-sm transition-colors',
                  step.done 
                    ? 'border-success/30 bg-success/10' 
                    : active 
                      ? 'border-primary/30 bg-primary/10' 
                      : 'border-border bg-white/5',
                )}>
                  {step.done ? <CheckCircle2 size={14} /> : step.icon}
                </div>
                <div className="flex flex-col">
                  <span className="text-sm font-bold tracking-tight">{step.label}</span>
                  <span className="text-[0.62rem] uppercase tracking-widest opacity-60">
                    {step.done ? 'Completed' : active ? 'In Progress' : 'Pending'}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex flex-col gap-3 rounded-2xl border border-border bg-white/[0.02] p-5">
        <div className="text-[0.65rem] font-black uppercase tracking-[0.18em] text-muted-foreground">Next Task</div>
        <div className="text-sm font-semibold text-foreground">
          {initialization?.next_step ? steps.find(s => s.id === (initialization.next_step === 'auth' ? 'provider' : initialization.next_step))?.label : 'Ready to Complete'}
        </div>
        <p className="text-xs leading-5 text-muted-foreground">
          Local password sign-in is enabled automatically for the first administrator.
        </p>
        {canEnterRuntime ? (
          <Button variant="secondary" size="sm" className="mt-2 w-full font-bold uppercase tracking-wider text-[0.65rem]" asChild>
            <Link to="/runtime-checks">Runtime Checks <ArrowRight size={12} className="ml-1.5" /></Link>
          </Button>
        ) : null}
      </div>
    </div>

    {/* Step forms */}
    <div className="space-y-6">
      <Card className="overflow-hidden p-0">
        <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-xl border border-border bg-white/5 text-primary">
                <UserCog size={18} />
              </div>
              <CardTitle className="text-base font-bold tracking-tight">Step 1 · Primary Administrator</CardTitle>
            </div>
            {wizard?.admin.user?.user_id ? <Badge variant="success" className="rounded-full text-[0.6rem] font-black uppercase tracking-widest">Saved</Badge> : null}
          </div>
        </CardHeader>
        <CardContent className="p-6 space-y-6">
          <div className="rounded-2xl border border-info/20 bg-info/8 px-4 py-3 text-sm leading-6 text-muted-foreground">
            Local password sign-in is enabled automatically for the first administrator. External identity providers can be configured later in the Identity section.
          </div>
          {adminError ? <StepAlert tone="error">{adminError}</StepAlert> : null}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <SetupField label="Username" requirement="required" hint="Unique login identifier for the first administrator."><Input required value={adminForm.username} onChange={(e) => setAdminForm((s) => ({ ...s, username: e.target.value }))} /></SetupField>
            <SetupField label="Password" requirement="required" hint="Use at least 8 characters with uppercase, lowercase, number, and symbol.">
              <div className="space-y-3">
                <div className="relative">
                  <Input
                    required
                    type={showAdminPassword ? 'text' : 'password'}
                    value={adminForm.password}
                    onChange={(e) => setAdminForm((s) => ({ ...s, password: e.target.value }))}
                    className="pr-12"
                  />
                  <button
                    type="button"
                    className="absolute inset-y-0 right-0 inline-flex items-center justify-center px-3 text-muted-foreground transition-colors hover:text-foreground"
                    aria-label={showAdminPassword ? 'Hide password' : 'Show password'}
                    onClick={() => setShowAdminPassword((current) => !current)}
                  >
                    {showAdminPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    <span className="sr-only">{showAdminPassword ? 'Hide password' : 'Show password'}</span>
                  </button>
                </div>
                <PasswordRequirementList state={adminPasswordState} />
              </div>
            </SetupField>
            <SetupField label="Display Name" requirement="optional" hint="Shown in approvals, audit, and inbox surfaces."><Input value={adminForm.display_name} onChange={(e) => setAdminForm((s) => ({ ...s, display_name: e.target.value }))} /></SetupField>
            <SetupField label="Email Address" requirement="optional" hint="Useful for notifications and recovery flows later."><Input value={adminForm.email} onChange={(e) => setAdminForm((s) => ({ ...s, email: e.target.value }))} /></SetupField>
          </div>
          {adminMessage ? <StepAlert tone="success">{adminMessage}</StepAlert> : null}
          <div className="flex justify-end pt-2 border-t border-border/50">
            <Button variant="primary" disabled={submitting || loading || !canSaveAdmin} onClick={onSaveAdmin}>Save Administrator</Button>
          </div>
        </CardContent>
      </Card>

      <Card className="overflow-hidden p-0">
        <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-xl border border-border bg-white/5 text-primary">
                <Cpu size={18} />
              </div>
              <CardTitle className="text-base font-bold tracking-tight">Step 2 · Provider Connectivity & Baseline Model</CardTitle>
            </div>
            {wizard?.provider.provider?.id ? <Badge variant="success" className="rounded-full text-[0.6rem] font-black uppercase tracking-widest">Saved</Badge> : null}
          </div>
        </CardHeader>
        <CardContent className="p-6 space-y-6">
          {providerError ? <StepAlert tone="error">{providerError}</StepAlert> : null}
          <div className="space-y-6 rounded-2xl border border-border/70 bg-white/[0.02] p-5">
            <div className="space-y-2">
              <div className="text-sm font-bold tracking-tight text-foreground">Provider Connectivity</div>
              <p className="text-sm leading-6 text-muted-foreground">
                Verify the provider endpoint and credential first. This check should pass before you decide which starter model TARS should use by default.
              </p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <SetupField label="Vendor" requirement="required" hint="Matches the provider vendor used on the full Providers page.">
                <select
                  className="flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm ring-offset-background appearance-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                  value={providerForm.vendor}
                  onChange={(e) => {
                    const vendor = e.target.value
                    setProviderForm((state) => ({
                      ...state,
                      vendor,
                      protocol: providerVendorDefaults[vendor]?.protocol || state.protocol,
                      base_url: state.base_url || providerVendorDefaults[vendor]?.base_url || state.base_url,
                    }))
                  }}
                >
                  {providerVendorOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
              </SetupField>
              <SetupField label="Base URL" requirement="required" hint="The same provider endpoint you would save on the Providers page.">
                <Input required value={providerForm.base_url} onChange={(e) => setProviderForm((s) => ({ ...s, base_url: e.target.value }))} placeholder="https://llm.example.com" />
              </SetupField>
              <SetupField label="API Key" requirement="required" hint="Setup writes this into the secret store automatically and keeps only a secret reference in provider config.">
                <div className="space-y-3">
                  <div className="relative">
                    <Input
                      type={showProviderApiKey ? 'text' : 'password'}
                      value={providerForm.api_key}
                      onChange={(e) => setProviderForm((s) => ({ ...s, api_key: e.target.value }))}
                      className="pr-12 font-mono text-sm"
                      placeholder="sk-..."
                    />
                    <button
                      type="button"
                      className="absolute inset-y-0 right-0 inline-flex items-center justify-center px-3 text-muted-foreground transition-colors hover:text-foreground"
                      aria-label={showProviderApiKey ? 'Hide API key' : 'Show API key'}
                      onClick={() => setShowProviderApiKey((current) => !current)}
                    >
                      {showProviderApiKey ? <EyeOff size={16} /> : <Eye size={16} />}
                      <span className="sr-only">{showProviderApiKey ? 'Hide API key' : 'Show API key'}</span>
                    </button>
                  </div>
                  <div className="rounded-xl border border-border/70 bg-white/[0.02] px-3 py-2 text-xs text-muted-foreground">
                    Stored as <span className="font-mono text-foreground">{providerSecretPreview}</span>
                  </div>
                </div>
              </SetupField>
            </div>
          </div>
          <div className="space-y-6 rounded-2xl border border-border/70 bg-white/[0.02] p-5">
            <div className="space-y-2">
              <div className="text-sm font-bold tracking-tight text-foreground">Default Model Baseline</div>
              <p className="text-sm leading-6 text-muted-foreground">
                Choose the starter model TARS should bind after connectivity is verified. Role-specific model bindings can be refined later on the Providers page.
              </p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <SetupField label="Default Model" requirement="required" hint="Required when saving the setup baseline, but not for the connectivity probe itself.">
                <Input required value={providerForm.model} onChange={(e) => setProviderForm((s) => ({ ...s, model: e.target.value }))} />
              </SetupField>
            </div>
          </div>
          <div className="flex justify-between items-center gap-3 rounded-xl border border-border/70 bg-white/[0.02] px-4 py-3">
            <div className="text-sm text-muted-foreground">Keep the first setup minimal. Advanced identity fields stay aligned with the full Providers page.</div>
            <Button type="button" variant="ghost" size="sm" className="text-xs font-bold uppercase tracking-[0.14em]" onClick={() => setProviderAdvancedOpen((open) => !open)}>
              {providerAdvancedOpen ? 'Hide Advanced' : 'Advanced Settings'}
            </Button>
          </div>
          {providerAdvancedOpen ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 rounded-xl border border-border/70 bg-white/[0.015] p-5">
              <SetupField label="Provider ID" requirement="optional" hint="Stable identifier used in saved configuration.">
                <Input value={providerForm.provider_id} onChange={(e) => setProviderForm((s) => ({ ...s, provider_id: e.target.value }))} />
              </SetupField>
              <SetupField label="Protocol" requirement="optional" hint="Auto-derived from vendor, but you can override it.">
                <Input value={providerForm.protocol} onChange={(e) => setProviderForm((s) => ({ ...s, protocol: e.target.value }))} />
              </SetupField>
              <SetupField className="md:col-span-2" label="Secret Reference" requirement="optional" hint="Override the generated secret path only if you need a custom location in the secret store.">
                <Input className="font-mono text-sm" value={providerForm.api_key_ref} onChange={(e) => setProviderForm((s) => ({ ...s, api_key_ref: e.target.value }))} placeholder={providerSecretPreview} />
              </SetupField>
            </div>
          ) : null}
          <div className="rounded-xl border border-primary/20 bg-primary/5 p-4 text-sm leading-relaxed text-muted-foreground">
            Connectivity and baseline model selection intentionally stay separate here: first prove the provider is reachable, then save the starter model binding for runtime.
          </div>
          <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between pt-2 border-t border-border/50">
            <div className="text-xs text-muted-foreground italic">
              {wizard?.initialization.provider_check_note ? `Latest check: ${wizard.initialization.provider_check_note}` : 'Verification checks credential availability and provider reachability before the default model is locked in.'}
            </div>
            <div className="flex gap-3">
              <Button variant="outline" disabled={submitting || loading || !canCheckProvider} onClick={onCheckProvider}>Check Connectivity</Button>
              <Button variant="primary" disabled={submitting || loading || !canSaveProvider} onClick={onSaveProvider}>Save Provider</Button>
            </div>
          </div>
          {providerCheckMessage ? <StepAlert tone="success">{providerCheckMessage}</StepAlert> : null}
        </CardContent>
      </Card>

      <Card className="overflow-hidden p-0">
        <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-xl border border-border bg-white/5 text-primary">
                <MessageSquare size={18} />
              </div>
              <CardTitle className="text-base font-bold tracking-tight">Step 3 · Primary Entrypoint & Delivery</CardTitle>
            </div>
            {wizard?.channel.channel?.id ? <Badge variant="success" className="rounded-full text-[0.6rem] font-black uppercase tracking-widest">Saved</Badge> : null}
          </div>
        </CardHeader>
        <CardContent className="p-6 space-y-6">
          {channelError ? <StepAlert tone="error">{channelError}</StepAlert> : null}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <SetupField label="Channel Kind" requirement="required" hint="Matches the channel kind used on the full Channels page.">
              <select
                className="flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm ring-offset-background appearance-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                value={channelForm.kind}
                onChange={(e) => setChannelForm((s) => ({ ...s, kind: e.target.value }))}
              >
                {setupChannelKindOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
            </SetupField>
            <SetupField label="Target" requirement="required" hint={channelForm.kind === 'telegram' ? 'Telegram chat ID, for example -10012345.' : 'For in-app inbox, keep `default`. For external channels, use the destination identifier.'}>
              <Input required className="font-mono text-sm" value={channelForm.target} onChange={(e) => setChannelForm((s) => ({ ...s, target: e.target.value }))} placeholder="default / room-id / chat-id" />
            </SetupField>
            <SetupField label="Display Name" requirement="optional" hint="Friendly label shown in approvals and notifications.">
              <Input value={channelForm.name} onChange={(e) => setChannelForm((s) => ({ ...s, name: e.target.value }))} />
            </SetupField>
          </div>
          <div className="flex justify-between items-center gap-3 rounded-xl border border-border/70 bg-white/[0.02] px-4 py-3">
            <div className="text-sm text-muted-foreground">Advanced identifiers stay available, but they do not need to block first-run setup.</div>
            <Button type="button" variant="ghost" size="sm" className="text-xs font-bold uppercase tracking-[0.14em]" onClick={() => setChannelAdvancedOpen((open) => !open)}>
              {channelAdvancedOpen ? 'Hide Advanced' : 'Advanced Settings'}
            </Button>
          </div>
          {channelAdvancedOpen ? (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 rounded-xl border border-border/70 bg-white/[0.015] p-5">
              <SetupField label="Channel ID" requirement="optional" hint="Stable identifier used in the saved channel config.">
                <Input value={channelForm.channel_id} onChange={(e) => setChannelForm((s) => ({ ...s, channel_id: e.target.value }))} />
              </SetupField>
            </div>
          ) : null}
          <LabeledField label="Usages" hint="Capabilities enabled for this channel">
            <div className="grid grid-cols-2 gap-3 rounded-xl border border-border bg-white/[0.01] p-4">
              {setupChannelUsageOptions.map((option) => {
                const checked = channelForm.usages.includes(option)
                return (
                  <label key={option} className={clsx(
                    "flex items-center gap-3 text-sm cursor-pointer rounded-lg p-2 transition-colors",
                    checked ? "bg-primary/5 text-foreground font-medium" : "text-muted-foreground hover:bg-white/5"
                  )}>
                    <Checkbox
                      checked={checked}
                      aria-label={option}
                      onCheckedChange={(value) => setChannelForm((state) => ({
                        ...state,
                        usages: toggleSelection(state.usages, option, value === true),
                      }))}
                    />
                    <span className="capitalize">{option.replace(/_/g, ' ')}</span>
                  </label>
                )
              })}
            </div>
          </LabeledField>
          <div className="rounded-xl border border-success/20 bg-success/5 p-4 text-sm leading-relaxed text-muted-foreground">
            Using `in_app_inbox` is recommended for initial setup. It handles approvals, notifications, and execution receipts within the console. More channels can be added later.
          </div>
          {telegramRequiresToken ? (
            <div className="rounded-xl border border-warning/25 bg-warning/10 p-4 text-sm leading-relaxed text-warning">
              Requires Telegram bot token. Configure <span className="font-mono">TARS_TELEGRAM_BOT_TOKEN</span> first. Use in-app inbox until the bot token is configured.
            </div>
          ) : null}
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 pt-2 border-t border-border/50">
            <div className="max-w-md space-y-1">
              <p className="text-sm text-muted-foreground">
                Finalize initialization to switch to runtime mode.
              </p>
              {channelMessage ? <p className="text-sm font-medium text-primary">{channelMessage}</p> : null}
              {channelForm.kind === 'telegram' && telegramConfigured ? <p className="text-xs text-muted-foreground">Telegram bot token is configured. Enter the destination chat ID for this primary channel.</p> : null}
            </div>
            <div className="flex gap-3">
              <Button variant="secondary" disabled={submitting || loading || !canSaveChannel} onClick={onSaveChannel}>Save Channel</Button>
              <Button variant="primary" disabled={submitting || loading || finishingLogin || !initialization?.channel_ready || !initialization?.provider_check_ok || telegramRequiresToken} onClick={onComplete}>
                {finishingLogin ? 'Signing In...' : 'Complete Setup'}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
);

const requirementBadgeClass: Record<'required' | 'optional', string> = {
  required: 'border-danger/25 bg-danger/10 text-danger',
  optional: 'border-border bg-white/[0.04] text-muted-foreground',
};

const SetupField = ({
  label,
  requirement,
  hint,
  children,
  className,
}: {
  label: string;
  requirement: 'required' | 'optional';
  hint?: string;
  children: ReactNode;
  className?: string;
}) => (
  <label className={clsx('flex flex-col gap-2', className)}>
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-sm font-semibold text-foreground">{label}</span>
      <span className={clsx('inline-flex items-center rounded-full border px-2 py-0.5 text-[0.6rem] font-black uppercase tracking-[0.14em]', requirementBadgeClass[requirement])}>
        {requirement === 'required' ? 'Required' : 'Optional'}
      </span>
    </div>
    {children}
    {hint ? <span className="text-xs leading-5 text-muted-foreground">{hint}</span> : null}
  </label>
);

type SetupPasswordState = {
  valid: boolean;
  items: Array<{ label: string; met: boolean }>;
};

function evaluateSetupPassword(password: string): SetupPasswordState {
  const value = password || '';
  const items = [
    { label: 'Use at least 8 characters', met: value.length >= 8 },
    { label: 'Add an uppercase letter', met: /[A-Z]/.test(value) },
    { label: 'Add a lowercase letter', met: /[a-z]/.test(value) },
    { label: 'Add a number', met: /\d/.test(value) },
    { label: 'Add a symbol', met: /[^A-Za-z0-9]/.test(value) },
  ];
  return {
    valid: items.every((item) => item.met),
    items,
  };
}

const PasswordRequirementList = ({ state }: { state: SetupPasswordState }) => (
  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
    {state.items.map((item) => (
      <div
        key={item.label}
        className={clsx(
          'flex items-center gap-2 rounded-lg border px-3 py-2 text-xs transition-colors',
          item.met ? 'border-success/20 bg-success/10 text-success' : 'border-border bg-white/[0.03] text-muted-foreground',
        )}
      >
        {item.met ? <CheckCircle2 size={14} /> : <AlertCircle size={14} />}
        <span>{item.label}</span>
      </div>
    ))}
  </div>
);

const StepAlert = ({
  tone,
  children,
}: {
  tone: 'error' | 'success';
  children: ReactNode;
}) => (
  <div
    className={clsx(
      'flex items-start gap-3 rounded-xl border px-4 py-3 text-sm leading-6',
      tone === 'error'
        ? 'border-danger/20 bg-danger/10 text-danger'
        : 'border-primary/15 bg-primary/5 text-primary',
    )}
  >
    {tone === 'error' ? <AlertCircle size={16} className="mt-0.5 shrink-0" /> : <CheckCircle2 size={16} className="mt-0.5 shrink-0" />}
    <div>{children}</div>
  </div>
);

export const RuntimeChecksPage = () => {
  const { t, lang, setLang } = useI18n();
  const [status, setStatus] = useState<SetupStatusResponse | null>(null);
  const [jumpserverSecrets, setJumpserverSecrets] = useState<SecretDescriptor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [submitError, setSubmitError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [lastTrigger, setLastTrigger] = useState<SmokeAlertResponse | null>(null);
  const [form, setForm] = useState(defaultForm);

  const loadStatus = async () => {
    try {
      setLoading(true);
      setError('');
      const setupResponse = await fetchSetupStatus();
      setStatus(setupResponse);
      setForm((current) => ({
        ...current,
        host: current.host || setupResponse.smoke_defaults?.hosts?.[0] || setupResponse.latest_smoke?.host || current.host,
      }));
      const secretsResult = await fetchSecretsInventory();
      setJumpserverSecrets(
        (secretsResult.items || []).filter((item) => item.owner_type === 'connector' && item.owner_id === 'jumpserver-main'),
      );
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, 'Failed to load runtime checks.'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadStatus();
  }, []);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    try {
      setSubmitting(true);
      setSubmitError('');
      const response = await triggerSmokeAlert(form);
      setLastTrigger(response);
      await loadStatus();
    } catch (submitErr) {
      setSubmitError(getApiErrorMessage(submitErr, 'Failed to trigger runtime check.'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex justify-end">
        <div className="inline-flex items-center rounded-md border border-border bg-[var(--bg-surface-solid)] p-0.5">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className={clsx('h-7 px-2.5 rounded text-xs font-semibold', lang === 'en-US' && 'bg-[var(--bg-surface)] text-[var(--text-primary)]')}
            onClick={() => void setLang('en-US')}
            aria-label={t('header.languageEnglish')}
          >
            EN
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className={clsx('h-7 px-2.5 rounded text-xs font-semibold', lang === 'zh-CN' && 'bg-[var(--bg-surface)] text-[var(--text-primary)]')}
            onClick={() => void setLang('zh-CN')}
            aria-label={t('header.languageChinese')}
          >
            中
          </Button>
        </div>
      </div>
      <OperatorHero
        eyebrow="Runtime Checks"
        title="Runtime Checks"
        description="Validate diagnosis, approval, notification, and execution paths after the platform is initialized."
        chips={[{ label: 'runtime', tone: 'success' }, { label: 'health checks', tone: 'info' }]}
        primaryAction={
          <Button variant="secondary" className="flex items-center gap-2" onClick={() => void loadStatus()} disabled={loading}>
            <RefreshCcw size={14} className={loading ? 'animate-spin' : ''} />
            Refresh
          </Button>
        }
      />

      {error ? (
        <div className="rounded-xl border border-danger/20 bg-danger/10 px-4 py-3 flex items-center gap-3 text-sm text-danger">
          <AlertCircle size={16} /> {error}
        </div>
      ) : null}

      {submitError ? (
        <div className="rounded-xl border border-danger/20 bg-danger/10 px-4 py-3 flex items-center gap-3 text-sm text-danger">
          <AlertCircle size={16} /> {submitError}
        </div>
      ) : null}

      <RuntimeMode
        loading={loading}
        status={status}
        jumpserverSecrets={jumpserverSecrets}
        form={form}
        setForm={setForm}
        submitting={submitting}
        lastTrigger={lastTrigger}
        onSubmit={handleSubmit}
      />
    </div>
  );
};

const RuntimeMode = ({
  loading,
  status,
  jumpserverSecrets,
  form,
  setForm,
  submitting,
  lastTrigger,
  onSubmit,
}: {
  loading: boolean;
  status: SetupStatusResponse | null;
  jumpserverSecrets: SecretDescriptor[];
  form: typeof defaultForm;
  setForm: React.Dispatch<React.SetStateAction<typeof defaultForm>>;
  submitting: boolean;
  lastTrigger: SmokeAlertResponse | null;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) => (
  <div className="grid grid-cols-1 xl:grid-cols-[1fr_360px] gap-8">
    <div className="space-y-8">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <RuntimePathCard title="Metrics Pipeline" icon={<Activity size={18} />} runtime={status?.connectors.metrics_runtime} loading={loading} />
        <RuntimePathCard title="Execution Pipeline" icon={<Zap size={18} />} runtime={status?.connectors.execution_runtime} loading={loading} secrets={jumpserverSecrets} />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <ComponentStatusCard title="Telegram Bot" icon={<MessageSquare size={16} />} configured={status?.telegram.configured ?? false} status={status?.telegram.last_result} detail={`mode=${status?.telegram.mode ?? 'disabled'}`} />
        <ComponentStatusCard title="Primary Model" icon={<Cpu size={16} />} configured={status?.model.configured ?? false} status={status?.model.last_result} detail={status?.model.model_name || 'unconfigured'} />
        <ComponentStatusCard title="Assist Model" icon={<ShieldAlert size={16} />} configured={status?.assist_model.configured ?? false} status={status?.assist_model.last_result} detail={status?.assist_model.model_name || 'unconfigured'} />
      </div>

      <Card className="overflow-hidden p-0 border-primary/20 bg-primary/[0.01]">
        <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-xl border border-primary/30 bg-primary/10 text-primary">
                <Play size={18} />
              </div>
              <CardTitle className="text-base font-bold tracking-tight">Manual Runtime Check</CardTitle>
            </div>
            <Badge variant="info" className="rounded-full text-[0.6rem] font-black uppercase tracking-widest">Diagnostic</Badge>
          </div>
        </CardHeader>
        <CardContent className="p-6">
          <form onSubmit={onSubmit} className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
              <LabeledField label="Alert Name"><Input value={form.alertname} onChange={(e) => setForm((f) => ({ ...f, alertname: e.target.value }))} /></LabeledField>
              <LabeledField label="Service Scope"><Input value={form.service} onChange={(e) => setForm((f) => ({ ...f, service: e.target.value }))} /></LabeledField>
              <LabeledField label="Target Host"><Input className="font-mono text-sm" list="setup-ssh-hosts" value={form.host} onChange={(e) => setForm((f) => ({ ...f, host: e.target.value }))} /><datalist id="setup-ssh-hosts">{(status?.smoke_defaults?.hosts || []).map((h) => <option key={h} value={h} />)}</datalist></LabeledField>
              <LabeledField label="Severity">
                <select
                  className="flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm ring-offset-background appearance-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                  value={form.severity}
                  onChange={(e) => setForm((f) => ({ ...f, severity: e.target.value }))}
                >
                  <option value="critical">Critical</option>
                  <option value="warning">Warning</option>
                  <option value="info">Info</option>
                </select>
              </LabeledField>
            </div>
            <LabeledField label="Diagnostic Summary / Reasoning Prompt">
              <textarea
                className="flex min-h-[120px] w-full rounded-xl border border-border bg-background px-4 py-3 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 leading-relaxed shadow-inner"
                value={form.summary}
                onChange={(e) => setForm((f) => ({ ...f, summary: e.target.value }))}
                placeholder="Describe the issue or provide a prompt for the reasoning engine..."
              />
            </LabeledField>
            <div className="flex flex-col sm:flex-row items-center justify-between gap-6 pt-4 border-t border-border/50">
              <div className="text-muted-foreground text-xs flex items-center gap-3">
                <div className="flex items-center gap-1.5 px-2 py-1 rounded bg-white/5 border border-border">
                   <MessageSquare size={12} className="opacity-50" />
                   <span className="font-mono text-foreground uppercase tracking-tighter">
                    {status?.latest_smoke?.telegram_target || lastTrigger?.tg_target || status?.initialization.default_channel_id || 'auto-detect'}
                  </span>
                </div>
                <span>Target Channel</span>
              </div>
              <Button className="px-8 py-2.5 shadow-lg shadow-primary/20 flex items-center gap-2 font-bold group" type="submit" disabled={submitting || loading}>
                {submitting ? 'Triggering...' : <>Run Runtime Check <ArrowRight size={18} className="transition-transform group-hover:translate-x-1" /></>}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>

    <div className="space-y-8">
      <LatestSmokeCard smoke={status?.latest_smoke || lastTrigger || undefined} loading={loading} />

      <Card className="overflow-hidden p-0">
        <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
          <CardTitle className="flex items-center gap-2 text-sm font-bold tracking-tight"><RefreshCcw size={14} className="text-primary" />Control Plane</CardTitle>
        </CardHeader>
        <CardContent className="p-5 space-y-4">
          <div className="space-y-3">
            <ControlFeatureRow label="Rollout Mode" value={status?.rollout_mode || 'custom'} highlight />
            <ControlFeatureRow label="Connectors" value={`${status?.connectors.enabled_entries ?? 0}/${status?.connectors.total_entries ?? 0} enabled`} />
            <ControlFeatureRow label="Auth Policy" value={status?.authorization.loaded ? 'Active' : 'Missing'} success={status?.authorization.loaded} />
            <ControlFeatureRow label="Approval Routing" value={status?.approval.loaded ? 'Active' : 'Missing'} success={status?.approval.loaded} />
            <ControlFeatureRow label="Primary Provider" value={status?.initialization.primary_provider_id || 'unconfigured'} />
            <ControlFeatureRow label="Default Channel" value={status?.initialization.default_channel_id || 'unconfigured'} />
          </div>
          <div className="pt-4 border-t border-border/50 grid grid-cols-2 gap-3">
            <Button variant="secondary" className="h-9 text-[0.65rem] flex items-center justify-center gap-1.5 uppercase font-black tracking-widest" asChild><Link to="/ops"><Settings2 size={12} /> Ops Center</Link></Button>
            <Button variant="secondary" className="h-9 text-[0.65rem] flex items-center justify-center gap-1.5 uppercase font-black tracking-widest" asChild><Link to="/inbox"><RefreshCcw size={12} /> Inbox</Link></Button>
          </div>
        </CardContent>
      </Card>

      <Card className="overflow-hidden p-0">
        <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
          <CardTitle className="flex items-center gap-2 text-sm font-bold tracking-tight"><Server size={14} className="text-primary" />Runtime Features</CardTitle>
        </CardHeader>
        <CardContent className="p-5 space-y-4">
          <FeatureToggle label="Diagnosis" enabled={status?.features.diagnosis_enabled} />
          <FeatureToggle label="Approval Flow" enabled={status?.features.approval_enabled} />
          <FeatureToggle label="Execution" enabled={status?.features.execution_enabled} />
          <FeatureToggle label="Knowledge Index" enabled={status?.features.knowledge_ingest_enabled} />
        </CardContent>
      </Card>

      <Card className="overflow-hidden p-0 border-warning/20 bg-warning/[0.03]">
        <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
          <CardTitle className="flex items-center gap-2 text-sm font-bold tracking-tight"><ShieldAlert size={14} className="text-warning" />Post-setup Hardening</CardTitle>
        </CardHeader>
        <CardContent className="p-5 space-y-3 text-sm text-muted-foreground">
          <p>Primary sign-in is now local password. For break-glass access, add a dedicated <span className="font-mono text-foreground">local_token</span> provider after setup and keep it in a restricted operator workflow.</p>
          <Button variant="secondary" className="h-9 text-[0.65rem] flex items-center justify-center gap-1.5 uppercase font-black tracking-widest" asChild>
            <Link to="/identity/providers"><Key size={12} /> Configure Emergency Access</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  </div>
);

const RuntimePathCard = ({ title, icon, runtime, loading, secrets }: { title: string; icon: ReactNode; runtime?: RuntimeSetupStatus; loading: boolean; secrets?: SecretDescriptor[] }) => {
  const primary = runtime?.primary;
  const fallback = runtime?.fallback;
  const isHealthy = runtime?.component_runtime.last_result === 'healthy' || runtime?.component_runtime.last_result === 'success';
  const hasMissingSecrets = secrets && secrets.some((s) => !s.set);
  return (
    <Card className={clsx('overflow-hidden p-0 transition-all hover:shadow-lg', isHealthy ? 'border-l-4 border-l-success' : 'border-l-4 border-l-warning')}>
      <CardHeader className="border-b border-border bg-white/[0.02] pb-4">
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-sm font-bold tracking-tight">{icon}{title}</CardTitle>
          <Badge variant={isHealthy ? 'success' : 'warning'} className="rounded-full text-[0.6rem] font-black uppercase tracking-widest">
            {loading ? '…' : isHealthy ? 'ACTIVE' : 'DEGRADED'}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-5 p-5">
        <div className="flex items-center gap-4">
          <div className="flex flex-col min-w-0">
            <span className="text-[0.62rem] text-muted-foreground font-black uppercase tracking-[0.14em]">Primary</span>
            <span className="text-sm font-mono text-foreground truncate mt-0.5">{primary?.connector_id || 'unassigned'}</span>
          </div>
          <ArrowRight size={14} className="text-muted-foreground shrink-0 mt-4 opacity-40" />
          <div className="flex flex-col min-w-0">
            <span className="text-[0.62rem] text-muted-foreground font-black uppercase tracking-[0.14em]">Fallback</span>
            <span className="text-sm font-mono text-foreground truncate mt-0.5 opacity-60">{fallback?.runtime || 'none'}</span>
          </div>
        </div>

        {hasMissingSecrets ? (
          <Link to="/ops#secret-inventory" className="flex items-center gap-3 p-3 rounded-xl bg-danger/10 border border-danger/20 text-danger text-[0.8rem] group hover:bg-danger/15 transition-all">
            <Key size={16} className="shrink-0 animate-pulse" />
            <span className="flex-1 font-bold">JumpServer secret keys missing</span>
            <ChevronRight size={16} className="group-hover:translate-x-1 transition-transform" />
          </Link>
        ) : null}

        {!isHealthy && runtime?.component_runtime.last_error ? (
          <div className="p-3 rounded-xl bg-warning/5 border border-warning/10 text-warning text-[0.8rem] italic line-clamp-2 leading-relaxed">
            {runtime.component_runtime.last_error}
          </div>
        ) : null}

        <div className="flex items-center justify-between text-[0.7rem] text-muted-foreground border-t border-border/50 pt-3">
          <span className="font-medium">Last probe: {runtime?.component_runtime.last_changed_at ? new Date(runtime.component_runtime.last_changed_at).toLocaleTimeString() : 'never'}</span>
          {primary?.connector_id ? <Link to={`/connectors/${primary.connector_id}`} className="text-primary font-bold hover:underline flex items-center gap-1.5 uppercase tracking-wider text-[0.65rem]">Manage <ExternalLink size={12} /></Link> : null}
        </div>
      </CardContent>
    </Card>
  );
};

const ComponentStatusCard = ({ title, icon, configured, status, detail }: { title: string; icon: ReactNode; configured: boolean; status?: string; detail: string }) => {
  const isHealthy = configured && (status === 'healthy' || status === 'success');
  const tone = isHealthy ? 'text-success border-success/20 bg-success/10' : configured ? 'text-warning border-warning/20 bg-warning/10' : 'text-danger border-danger/20 bg-danger/10';
  return (
    <Card className="glass-card-interactive overflow-hidden p-0 transition-all">
      <CardContent className="flex flex-col gap-4 p-5">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-2.5 text-xs font-black uppercase tracking-[0.16em] text-muted-foreground">
            <span className="text-primary">{icon}</span>
            {title}
          </div>
          <span className={clsx('rounded-full border px-2 py-0.5 text-[0.6rem] font-black uppercase tracking-widest', tone)}>
            {isHealthy ? 'OK' : configured ? 'WARN' : 'MISSING'}
          </span>
        </div>
        <div className="space-y-1">
          <div className="text-sm font-mono text-foreground truncate tracking-tight">{detail}</div>
          <div className="text-[0.65rem] text-muted-foreground uppercase font-black tracking-[0.12em] opacity-60">{status || 'not tested'}</div>
        </div>
      </CardContent>
    </Card>
  );
};

const LatestSmokeCard = ({ smoke, loading }: { smoke?: SmokeSessionStatus | SmokeAlertResponse; loading: boolean }) => {
  const isResponse = smoke && 'accepted' in smoke;
  const session_id = smoke?.session_id || '';
  const status = !smoke ? 'none' : 'status' in smoke ? smoke.status : 'open';
  return (
    <Card className="p-0">
      <CardHeader className="border-b border-border pb-3">
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-sm"><LayoutDashboard size={14} />Latest Runtime Check</CardTitle>
          {session_id ? <StatusBadge status={status} className="text-[10px]" /> : null}
        </div>
      </CardHeader>
      <CardContent className="p-4">
        <div className="flex flex-col gap-4">
          {loading ? (
            <div className="animate-pulse space-y-3">
              <div className="h-4 bg-white/5 rounded w-3/4" />
              <div className="h-4 bg-white/5 rounded w-1/2" />
            </div>
          ) : !smoke ? (
            <EmptyState title="No smoke session recorded." icon={Database} />
          ) : (
            <>
              <div className="space-y-1.5">
                <div className="text-[0.65rem] font-black text-muted-foreground uppercase tracking-widest">Active Session ID</div>
                <div className="font-mono text-sm text-foreground bg-white/5 p-2 rounded border border-border break-all select-all">{session_id}</div>
              </div>
              {!isResponse && (smoke as SmokeSessionStatus).alertname ? (
                <div className="grid grid-cols-2 gap-4 border-t border-border pt-4">
                  <div className="flex flex-col gap-0.5 min-w-0">
                    <span className="text-[0.6rem] text-muted-foreground uppercase font-black">Alert</span>
                    <span className="text-sm truncate text-foreground">{(smoke as SmokeSessionStatus).alertname}</span>
                  </div>
                  <div className="flex flex-col gap-0.5 min-w-0">
                    <span className="text-[0.6rem] text-muted-foreground uppercase font-black">Service</span>
                    <span className="text-sm truncate text-foreground">{(smoke as SmokeSessionStatus).service}</span>
                  </div>
                </div>
              ) : null}
              <Button variant="secondary" className="w-full py-2 text-[0.65rem] flex items-center justify-center gap-2 group uppercase font-bold tracking-wider" asChild>
                <Link to={`/sessions/${session_id}`}>Open Full Diagnostics <ExternalLink size={14} className="group-hover:translate-y-[-1px] group-hover:translate-x-[1px] transition-transform" /></Link>
              </Button>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  );
};

const ControlFeatureRow = ({ label, value, highlight, danger, success }: { label: string; value: string; highlight?: boolean; danger?: boolean; success?: boolean }) => (
  <div className="flex justify-between items-center text-sm"><span className="text-text-muted">{label}</span><span className={clsx('font-medium', highlight ? 'text-primary' : danger ? 'text-danger' : success ? 'text-success' : 'text-text-secondary')}>{value}</span></div>
);

const FeatureToggle = ({ label, enabled }: { label: string; enabled?: boolean }) => (
  <div className="flex items-center justify-between"><span className="text-sm font-medium">{label}</span><div className={clsx('w-8 h-4 rounded-full relative transition-colors', enabled ? 'bg-success/40' : 'bg-white/10')}><div className={clsx('absolute top-1 w-2 h-2 rounded-full transition-all', enabled ? 'right-1 bg-success' : 'left-1 bg-text-muted')} /></div></div>
);
