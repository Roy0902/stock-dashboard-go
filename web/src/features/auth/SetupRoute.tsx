import { Navigate, useLocation } from 'react-router-dom';

import { AuthSplash } from '@/features/auth/AuthSplash';
import { AuthUnavailable } from '@/features/auth/AuthUnavailable';
import { SetupPage } from '@/features/auth/SetupPage';
import { useAuth } from '@/features/auth/useAuth';
import { resolvePostAuthPath } from '@/routes';

export function SetupRoute() {
  const { phase } = useAuth();
  const location = useLocation();

  if (phase === 'loading') {
    return <AuthSplash />;
  }
  if (phase === 'unavailable') {
    return <AuthUnavailable />;
  }
  if (phase === 'authenticated') {
    return <Navigate to={resolvePostAuthPath(location.state)} replace />;
  }
  return <SetupPage />;
}
