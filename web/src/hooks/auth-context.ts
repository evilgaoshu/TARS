import { createContext, useContext } from 'react';
import type { AuthSession, AuthUserSummary } from '../lib/api/types';
import { loadStoredSession } from '../lib/auth/storage';

export interface AuthContextType {
  user: AuthUserSummary | null;
  session: AuthSession | null;
  login: (session: AuthSession) => void;
  refresh: () => Promise<void>;
  logout: () => Promise<void>;
  isAuthenticated: boolean;
}

export const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

export const loadStoredAuthSession = (): AuthSession | null => loadStoredSession();
