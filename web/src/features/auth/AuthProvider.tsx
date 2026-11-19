import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';

import { getAuthStatus, logout, setUnauthorizedHandler } from '@/api/client';
import type { ApiError } from '@/api/client';
import type { AuthStatus, AuthUser } from '@/api/types';
import { broadcastAuthChange, subscribeAuthChange, AUTH_TAB_ID } from '@/features/auth/authChannel';
import { AuthContext } from '@/features/auth/authContext';
import type { AuthContextValue, AuthPhase } from '@/features/auth/authContext';
import { copy } from '@/locale';

type AuthProviderProps = {
  children: ReactNode;
};

/**
 * Only an unavailable service makes the session state unknown: a transport failure
 * (`status === 0`) or any 5xx, regardless of its code. A 4xx from the status endpoint is
 * an answer ("no session"). A malformed success body is `INTERNAL_ERROR` with status 200;
 * it also keeps the retry panel rather than signing the user out.
 */
function isBootstrapFailure(error: ApiError): boolean {
  return (
    error.status === 0 ||
    error.status >= 500 ||
    (error.status === 200 && error.code === 'INTERNAL_ERROR')
  );
}

export function AuthProvider({ children }: AuthProviderProps) {
  const queryClient = useQueryClient();
  const [phase, setPhase] = useState<AuthPhase>('loading');
  const [user, setUser] = useState<AuthUser | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const phaseRef = useRef<AuthPhase>('loading');

  const updatePhase = useCallback((next: AuthPhase) => {
    phaseRef.current = next;
    setPhase(next);
  }, []);

  const applyStatus = useCallback(
    (status: AuthStatus) => {
      if (status.setupRequired) {
        setUser(null);
        updatePhase('setupRequired');
        return;
      }
      if (status.authenticated && status.user !== undefined) {
        setUser(status.user);
        setNotice(null);
        updatePhase('authenticated');
        return;
      }
      setUser(null);
      updatePhase('unauthenticated');
    },
    [updatePhase],
  );

  useEffect(() => {
    const controller = new AbortController();
    let active = true;

    void getAuthStatus(controller.signal)
      .then((result) => {
        if (!active) return;
        if (result.ok) {
          applyStatus(result.value);
          return;
        }
        setUser(null);
        updatePhase(isBootstrapFailure(result.error) ? 'unavailable' : 'unauthenticated');
      })
      .catch(() => {
        if (!active) return;
        setUser(null);
        updatePhase('unavailable');
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [applyStatus, updatePhase, attempt]);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (phaseRef.current !== 'authenticated') return;
      queryClient.clear();
      setUser(null);
      setNotice(copy.errors.sessionExpired);
      updatePhase('unauthenticated');
    });

    return () => {
      setUnauthorizedHandler(null);
    };
  }, [queryClient, updatePhase]);

  // Another tab signed out or switched accounts: this tab shares the same cookie jar, so
  // its cached watchlist/quotes belong to the wrong account. Messages echoed back from this
  // tab are ignored; they would otherwise undo the sign-in that triggered them.
  useEffect(
    () =>
      subscribeAuthChange((message) => {
        if (message.origin === AUTH_TAB_ID) return;
        queryClient.clear();
        if (message.event === 'signed-out') {
          setUser(null);
          setNotice(null);
          updatePhase('unauthenticated');
          return;
        }
        setUser(null);
        updatePhase('loading');
        setAttempt((value) => value + 1);
      }),
    [queryClient, updatePhase],
  );

  const acceptUser = useCallback(
    (nextUser: AuthUser) => {
      queryClient.clear();
      setUser(nextUser);
      setNotice(null);
      updatePhase('authenticated');
      broadcastAuthChange('account-changed');
    },
    [queryClient, updatePhase],
  );

  const retryAuth = useCallback(() => {
    setUser(null);
    updatePhase('loading');
    setAttempt((value) => value + 1);
  }, [updatePhase]);

  const clearNotice = useCallback(() => {
    setNotice(null);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await logout();
    } finally {
      queryClient.clear();
      setUser(null);
      setNotice(null);
      updatePhase('unauthenticated');
      broadcastAuthChange('signed-out');
    }
  }, [queryClient, updatePhase]);

  const value = useMemo<AuthContextValue>(
    () => ({ phase, user, notice, acceptUser, clearNotice, retryAuth, signOut }),
    [phase, user, notice, acceptUser, clearNotice, retryAuth, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
