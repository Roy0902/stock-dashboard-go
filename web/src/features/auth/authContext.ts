import { createContext } from 'react';

import type { AuthUser } from '@/api/types';

/**
 * `unavailable` means the bootstrap request failed for a transient reason (network or 5xx),
 * so the app cannot tell whether a session exists. It must not be presented as signed out.
 */
export type AuthPhase =
  'loading' | 'setupRequired' | 'unauthenticated' | 'authenticated' | 'unavailable';

export interface AuthContextValue {
  phase: AuthPhase;
  user: AuthUser | null;
  notice: string | null;
  acceptUser: (user: AuthUser) => void;
  clearNotice: () => void;
  retryAuth: () => void;
  signOut: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue | null>(null);
