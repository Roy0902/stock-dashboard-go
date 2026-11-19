import { Navigate } from 'react-router-dom';

import { AuthSplash } from '@/features/auth/AuthSplash';
import { AuthUnavailable } from '@/features/auth/AuthUnavailable';
import { useAuth } from '@/features/auth/useAuth';
import { LandingPage } from '@/pages/LandingPage';
import { ROUTE_PATHS } from '@/routes';

export function LandingRoute() {
  const { phase } = useAuth();

  if (phase === 'loading') {
    return <AuthSplash />;
  }
  if (phase === 'unavailable') {
    return <AuthUnavailable />;
  }
  if (phase === 'setupRequired') {
    return <Navigate to={ROUTE_PATHS.setup} replace />;
  }
  if (phase === 'authenticated') {
    return <Navigate to={ROUTE_PATHS.dashboard} replace />;
  }
  return <LandingPage />;
}
