import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { fetchAuthProviders, fetchMe, loginWithPassword, loginWithToken, verifyAuthChallenge, verifyAuthMFA } from '../../lib/api/access';
import { getApiErrorMessage } from '../../lib/api/ops';
import { useAuth } from '../../hooks/useAuth';
import { useI18n } from '../../hooks/useI18n';
import type { AuthProviderInfo, AuthUserSummary } from '../../lib/api/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { NativeSelect } from '@/components/ui/select';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';

const oauthProviderTypes = new Set(['oidc', 'oauth2']);

function chooseDefaultProvider(providers: AuthProviderInfo[], presetProviderID: string): string {
  if (presetProviderID) {
    return presetProviderID;
  }
  const enabledProviders = providers.filter((provider) => provider.enabled);
  const preferredProvider = enabledProviders.find((provider) => provider.type === 'local_password')
    || enabledProviders.find((provider) => provider.type !== 'local_token')
    || enabledProviders[0];
  return preferredProvider?.id || 'local_token';
}

function buildUserSummary(me: Awaited<ReturnType<typeof fetchMe>>, fallbackSource: string): AuthUserSummary {
  return {
    id: me.user.user_id || me.user.username || 'unknown',
    username: me.user.username || '',
    displayName: me.user.display_name || me.user.username || 'Ops Admin',
    email: me.user.email || '',
    roles: me.roles || [],
    permissions: me.permissions || [],
    authSource: me.auth_source || fallbackSource,
    breakGlass: Boolean(me.break_glass),
  };
}

function resolveNextPath(value: string | null): string {
  if (!value || !value.startsWith('/')) {
    return '/';
  }
  if (value.startsWith('//')) {
    return '/';
  }
  return value;
}

export const LoginView = () => {
  const [token, setToken] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [challengeCode, setChallengeCode] = useState('');
  const [mfaCode, setMfaCode] = useState('');
  const [pendingToken, setPendingToken] = useState('');
  const [pendingChallengeID, setPendingChallengeID] = useState('');
  const [step, setStep] = useState<'login' | 'challenge' | 'mfa'>('login');
  const [statusMessage, setStatusMessage] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [providerID, setProviderID] = useState('');
  const [providers, setProviders] = useState<AuthProviderInfo[]>([]);
  const [providersLoaded, setProvidersLoaded] = useState(false);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login } = useAuth();
  const { t, lang, setLang } = useI18n();
  const selectedProvider = providers.find((provider) => provider.id === providerID) || null;
  const providerType = selectedProvider?.type || providerID;
  const isRedirectProvider = oauthProviderTypes.has(providerType || '');
  const isLDAPProvider = providerType === 'ldap';
  const isPasswordProvider = providerType === 'local_password';
  const presetUsername = searchParams.get('username') || '';
  const presetProviderID = searchParams.get('provider_id') || '';
  const nextPath = resolveNextPath(searchParams.get('next'));
  const enabledProviders = providers.filter((provider) => provider.enabled);
  const providersStillLoading = !providersLoaded && !presetProviderID;
  const hasExplicitLocalTokenProvider = enabledProviders.some((provider) => provider.id === 'local_token' || provider.type === 'local_token');
  const shouldExposeBreakGlassProvider = presetProviderID === 'local_token' || hasExplicitLocalTokenProvider || (providersLoaded && enabledProviders.length === 0);
  const providerOptions = [
    ...(shouldExposeBreakGlassProvider && !hasExplicitLocalTokenProvider
      ? [{
        id: 'local_token',
        name: t('login.localToken'),
        type: 'local_token',
        enabled: true,
        client_secret_set: false,
        bind_password_set: false,
        allow_jit: false,
      } satisfies AuthProviderInfo]
      : []),
    ...enabledProviders,
  ];

  useEffect(() => {
    let active = true;
    setProvidersLoaded(false);
    const loadProviders = async () => {
      try {
        const response = await fetchAuthProviders();
        if (!active) return;
        const nextProviders = response.items || [];
        setProviders(nextProviders);
        setProvidersLoaded(true);
        setProviderID((current) => {
          if (presetProviderID) {
            return presetProviderID;
          }
          if (current && current !== 'local_token') {
            return current;
          }
          return chooseDefaultProvider(nextProviders, presetProviderID);
        });
      } catch {
        if (!active) return;
        setProviders([]);
        setProvidersLoaded(true);
        setProviderID((current) => current || chooseDefaultProvider([], presetProviderID));
      }
    };
    void loadProviders();
    return () => { active = false; };
  }, [presetProviderID]);

  useEffect(() => {
    setStep('login');
    setPendingToken('');
    setPendingChallengeID('');
    setChallengeCode('');
    setMfaCode('');
    setStatusMessage('');
    setError('');
  }, [providerID]);

  useEffect(() => {
	if (presetUsername) {
		setUsername(presetUsername);
	}
	if (presetProviderID) {
		setProviderID(presetProviderID);
	}
	// eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const sessionToken = searchParams.get('session_token');
    if (!sessionToken) return;

    let active = true;
    const completeRedirectLogin = async () => {
      try {
        setLoading(true);
        setError('');
        const me = await fetchMe(sessionToken);
        if (!active) return;
        const userSummary = buildUserSummary(me, 'redirect');
        login({
          token: sessionToken,
          user: userSummary,
          roles: me.roles || [],
          permissions: me.permissions || [],
          authSource: me.auth_source || 'redirect',
          breakGlass: Boolean(me.break_glass),
        });
        navigate(nextPath, { replace: true });
      } catch (redirectError) {
        if (!active) return;
        setError(getApiErrorMessage(redirectError, 'Failed to finish sign-in.'));
      } finally {
        if (active) setLoading(false);
      }
    };
    void completeRedirectLogin();
    return () => { active = false; };
  }, [login, navigate, nextPath, searchParams]);

  const finishSessionLogin = async (sessionToken: string, authSource: string) => {
    const me = await fetchMe(sessionToken);
    const userSummary = buildUserSummary(me, authSource);
    login({
      token: sessionToken,
      user: userSummary,
      roles: me.roles || [],
      permissions: me.permissions || [],
      authSource: me.auth_source || authSource,
      breakGlass: Boolean(me.break_glass),
    });
    navigate(nextPath, { replace: true });
  };

  const handleRedirectLogin = () => {
    const loginURL = selectedProvider?.login_url || `/api/v1/auth/login?provider_id=${encodeURIComponent(providerID)}`;
    window.location.href = loginURL;
  };

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isRedirectProvider) {
      setError('');
      handleRedirectLogin();
      return;
    }
    if (isLDAPProvider) {
      setError(t('login.ldapNotice'));
      return;
    }

    try {
      setLoading(true);
      setError('');

      if (step === 'challenge') {
        const auth = await verifyAuthChallenge(pendingToken, pendingChallengeID, challengeCode.trim());
        if (auth.session_token) {
          await finishSessionLogin(auth.session_token, providerID);
          return;
        }
        setPendingToken(auth.pending_token || pendingToken);
        setStep('mfa');
        setStatusMessage(t('login.challengeVerified'));
        return;
      }

      if (step === 'mfa') {
        const auth = await verifyAuthMFA(pendingToken, mfaCode.trim());
        if (auth.session_token) {
          await finishSessionLogin(auth.session_token, providerID);
        }
        return;
      }

      if (isPasswordProvider) {
        if (!username.trim() || !password.trim()) {
          setError(t('login.enterCredentials'));
          return;
        }
        const auth = await loginWithPassword(providerID, username.trim(), password);
        if (auth.session_token) {
          await finishSessionLogin(auth.session_token, providerID);
          return;
        }
        setPendingToken(auth.pending_token || '');
        setPendingChallengeID(auth.challenge_id || '');
        setStatusMessage(auth.challenge_code ? `${t('login.challengeCode')}: ${auth.challenge_code}` : t('login.challengeIssued'));
        setStep(auth.next_step === 'mfa' ? 'mfa' : 'challenge');
        return;
      }

      const cleanToken = token.trim();
      if (!cleanToken) {
        setError(t('login.enterToken'));
        return;
      }

      if (providerID === 'local_token') {
        try {
          await finishSessionLogin(cleanToken, providerID);
          return;
        } catch {
          // keep compatibility with session-issuing login flow
        }
      }

      const auth = await loginWithToken(cleanToken, providerID);
      if (auth.redirect_url && !auth.session_token) {
        window.location.href = auth.redirect_url;
        return;
      }
      if (auth.session_token) {
        await finishSessionLogin(auth.session_token, providerID);
        return;
      }
    } catch (err) {
      setError(getApiErrorMessage(err, t('login.authFailed')));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen grid place-items-center p-4 sm:p-8 relative">
      <div className="absolute top-6 right-6 flex bg-[var(--bg-surface-solid)] rounded-md p-0.5 border border-[var(--border-color)] z-10">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className={cn("h-7 px-2.5 rounded text-xs font-semibold", lang === 'en-US' && "bg-[var(--bg-surface)] text-[var(--text-primary)]")}
          onClick={() => setLang('en-US')}
          aria-label={t('header.languageEnglish')}
        >
          EN
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className={cn("h-7 px-2.5 rounded text-xs font-semibold", lang === 'zh-CN' && "bg-[var(--bg-surface)] text-[var(--text-primary)]")}
          onClick={() => setLang('zh-CN')}
          aria-label={t('header.languageChinese')}
        >
          中
        </Button>
      </div>
      <Card className="animate-fade-in shadow-2xl w-full max-w-lg p-6 sm:p-10">
        <div className="text-center mb-10">
          <div className="inline-flex items-center justify-center size-16 rounded-2xl bg-primary/10 text-primary text-3xl mb-4 border border-primary/20">
            ◬
          </div>
          <h1 className="tracking-[0.2em] m-0 text-2xl font-bold uppercase">{t('login.title')}</h1>
          <p className="text-text-muted mt-2 text-sm">{t('login.subtitle')}</p>
        </div>

        <form onSubmit={handleLogin} className="flex flex-col gap-5">
          <div className="flex flex-col gap-2">
            <label htmlFor="login-provider" className="text-sm font-bold text-text-secondary uppercase tracking-wider ml-1">{t('login.provider')}</label>
            <NativeSelect id="login-provider" value={providerID} onChange={e => setProviderID(e.target.value)} disabled={loading || providersStillLoading}>
              {providerOptions.map(p => (
                <option key={p.id} value={p.id}>{p.name || p.id}</option>
              ))}
            </NativeSelect>
          </div>

          {isPasswordProvider && step === 'login' ? (
            <>
              <div className="flex flex-col gap-2">
                <label htmlFor="login-username" className="text-sm font-bold text-text-secondary uppercase tracking-wider ml-1">{t('login.usernameOrEmail')}</label>
                <Input id="login-username" type="text" placeholder="alice" value={username} onChange={e => setUsername(e.target.value)} disabled={loading} autoFocus />
              </div>
              <div className="flex flex-col gap-2">
                <label htmlFor="login-password" className="text-sm font-bold text-text-secondary uppercase tracking-wider ml-1">{t('login.password')}</label>
                <Input id="login-password" type="password" placeholder="password" value={password} onChange={e => setPassword(e.target.value)} disabled={loading} />
              </div>
            </>
          ) : null}

          {!providersStillLoading && !isPasswordProvider && !isRedirectProvider && !isLDAPProvider ? (
            <div className="flex flex-col gap-2">
              <label htmlFor="login-token" className="text-sm font-bold text-text-secondary uppercase tracking-wider ml-1">{t('login.token')}</label>
              <Input id="login-token" type="password" placeholder="Paste token" value={token} onChange={e => setToken(e.target.value)} disabled={loading} autoFocus />
            </div>
          ) : null}

          {step === 'challenge' ? (
            <div className="flex flex-col gap-2">
              <label htmlFor="login-challenge" className="text-sm font-bold text-text-secondary uppercase tracking-wider ml-1">{t('login.challengeCode')}</label>
              <Input id="login-challenge" type="text" placeholder="123456" value={challengeCode} onChange={e => setChallengeCode(e.target.value)} disabled={loading} autoFocus />
            </div>
          ) : null}

          {step === 'mfa' ? (
            <div className="flex flex-col gap-2">
              <label htmlFor="login-mfa" className="text-sm font-bold text-text-secondary uppercase tracking-wider ml-1">{t('login.authenticatorCode')}</label>
              <Input id="login-mfa" type="text" placeholder="123456" value={mfaCode} onChange={e => setMfaCode(e.target.value)} disabled={loading} autoFocus />
            </div>
          ) : null}

          {isRedirectProvider ? (
            <Card className="p-4 bg-white/[0.04] text-[var(--text-secondary)]">
              {t('login.redirectNoticePrefix').replace('{{name}}', selectedProvider?.name || providerID)}
            </Card>
          ) : null}

          {isLDAPProvider ? (
            <Card className="p-4 bg-white/[0.04] text-[var(--text-secondary)]">
              {t('login.ldapNotice')}
            </Card>
          ) : null}

          {statusMessage ? <div className="text-[var(--text-secondary)] text-xs" role="status">{statusMessage}</div> : null}
          {error ? <div className="text-danger text-xs animate-shake" role="alert">{error}</div> : null}

          <Button type="submit" variant="amber" className="w-full py-3 mt-4 font-bold shadow-lg shadow-primary/20" disabled={loading || isLDAPProvider}>
            {loading ? t('login.authenticating') : isRedirectProvider ? t('login.continueWith').replace('{{name}}', selectedProvider?.name || providerID) : step === 'challenge' ? t('login.verifyChallenge') : step === 'mfa' ? t('login.verifyMfa') : t('login.signin')}
          </Button>
        </form>
      </Card>
    </div>
  );
};
