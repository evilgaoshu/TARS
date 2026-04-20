import type { AuthSession } from '../api/types';

const SESSION_KEY = 'tars_auth_session';

export function loadStoredSession(): AuthSession | null {
  if (typeof window === 'undefined') {
    return null;
  }
  const raw = localStorage.getItem(SESSION_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as AuthSession;
  } catch {
    localStorage.removeItem(SESSION_KEY);
    return null;
  }
}

export function saveStoredSession(session: AuthSession): void {
  localStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

export function clearStoredSession(): void {
  localStorage.removeItem(SESSION_KEY);
}

export function getStoredToken(): string {
  return loadStoredSession()?.token || '';
}
