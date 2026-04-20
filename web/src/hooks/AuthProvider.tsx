import { useCallback, useEffect, useMemo, useState } from 'react';
import { fetchMe, logoutSession } from '../lib/api/access';
import type { AuthSession, AuthUserSummary, MeResponse } from '../lib/api/types';
import { AuthContext, loadStoredAuthSession } from './auth-context';
import { clearStoredSession, saveStoredSession } from '../lib/auth/storage';

function sessionFromMeResponse(current: AuthSession | null, me: MeResponse): AuthSession | null {
  if (!current?.token) {
    return null;
  }
  return {
    token: current.token,
    user: toUserSummary(me),
    roles: me.roles || [],
    permissions: me.permissions || [],
    authSource: me.auth_source || '',
    breakGlass: Boolean(me.break_glass),
  };
}

function toUserSummary(me: MeResponse): AuthUserSummary {
  return {
    id: me.user.user_id || me.user.username || 'unknown',
    username: me.user.username || '',
    displayName: me.user.display_name || me.user.username || me.user.user_id || 'Unknown User',
    email: me.user.email || '',
    roles: me.roles || [],
    permissions: me.permissions || [],
    authSource: me.auth_source || '',
    breakGlass: Boolean(me.break_glass),
  };
}

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [session, setSession] = useState<AuthSession | null>(() => loadStoredAuthSession());
  const [user, setUser] = useState<AuthUserSummary | null>(() => loadStoredAuthSession()?.user || null);

  const login = useCallback((nextSession: AuthSession) => {
    saveStoredSession(nextSession);
    setSession(nextSession);
    setUser(nextSession.user);
  }, []);

  const refresh = useCallback(async () => {
    const current = loadStoredAuthSession();
    if (!current?.token) {
      clearStoredSession();
      setSession(null);
      setUser(null);
      return;
    }
    try {
      const me = await fetchMe(current.token);
      const next = sessionFromMeResponse(current, me);
      if (!next) {
        clearStoredSession();
        setSession(null);
        setUser(null);
        return;
      }
      saveStoredSession(next);
      setSession(next);
      setUser(next.user);
    } catch {
      clearStoredSession();
      setSession(null);
      setUser(null);
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      if (loadStoredAuthSession()?.token) {
        await logoutSession();
      }
    } catch {
      // ignore logout failures and clear local state anyway
    }
    clearStoredSession();
    setSession(null);
    setUser(null);
  }, []);

  useEffect(() => {
    if (session?.token) {
      const timer = window.setTimeout(() => {
        void refresh();
      }, 0);
      return () => window.clearTimeout(timer);
    }
  }, [refresh, session?.token]);

  const value = useMemo(
    () => ({
      user,
      session,
      login,
      refresh,
      logout,
      isAuthenticated: Boolean(session?.token && user),
    }),
    [login, logout, refresh, session, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};
