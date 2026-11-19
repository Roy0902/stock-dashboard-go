import type { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';

import { AuthSplash } from '@/features/auth/AuthSplash';
import { AuthUnavailable } from '@/features/auth/AuthUnavailable';
import { useAuth } from '@/features/auth/useAuth';
import { resolvePostAuthPath, ROUTE_PATHS } from '@/routes';

type AuthGateMode = 'protected' | 'login';

type AuthGateProps = {
  mode: AuthGateMode;
  children: ReactNode;
};

export function AuthGate({ mode, children }: AuthGateProps) {
  const { phase } = useAuth();
  const location = useLocation();

  if (phase === 'loading') {
    return <AuthSplash />;
  }

  if (phase === 'unavailable') {
    return <AuthUnavailable />;
  }

  if (mode === 'login') {
    // D-29: a fresh install goes to /setup, never to a sign-in form that cannot succeed.
    if (phase === 'setupRequired') {
      // Keep the requested deep link, so activation can restore it.
      return <Navigate to={ROUTE_PATHS.setup} state={location.state as unknown} replace />;
    }
    // The guard owns post-auth navigation, including the requested deep link.
    if (phase === 'authenticated') {
      return <Navigate to={resolvePostAuthPath(location.state)} replace />;
    }
    return children;
  }

  if (phase === 'authenticated') return children;
  return (
    <Navigate
      to={ROUTE_PATHS.login}
      state={{ from: location.pathname + location.search }}
      replace
    />
  );
}
