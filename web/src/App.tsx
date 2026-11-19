import { useEffect, useRef } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';

import { AppBoundary } from '@/AppBoundary';
import { AuthGate } from '@/features/auth/AuthGate';
import { AuthProvider } from '@/features/auth/AuthProvider';
import { LoginPage } from '@/features/auth/LoginPage';
import { SetupRoute } from '@/features/auth/SetupRoute';
import { ChartPage } from '@/pages/ChartPage';
import { DashboardPage } from '@/pages/DashboardPage';
import { LandingRoute } from '@/pages/LandingRoute';
import { ROUTE_PATHS } from '@/routes';
import { copy } from '@/locale';
import { SiteHeader } from '@/ui/SiteHeader';

const MAIN_CONTENT_ID = 'main-content';

export function App() {
  const location = useLocation();
  const mainRef = useRef<HTMLElement>(null);
  const previousPathRef = useRef(location.pathname);

  // A client-side route change unmounts the focused link, so focus must be moved into the
  // new view. Skip the first render so initial page load keeps normal document order.
  useEffect(() => {
    if (previousPathRef.current === location.pathname) return;
    previousPathRef.current = location.pathname;

    window.scrollTo(0, 0);
    const main = mainRef.current;
    if (main === null) return;

    const active = document.activeElement;
    if (active instanceof HTMLElement && active !== main && main.contains(active)) {
      // The page already moved focus to its own heading (auth steps do this).
      return;
    }
    main.focus({ preventScroll: true });
  }, [location.pathname]);

  return (
    <AuthProvider>
      <div className="app-shell">
        <a className="skip-link" href={`#${MAIN_CONTENT_ID}`}>
          {copy.app.skipToContent}
        </a>
        <SiteHeader />
        <main className="app-main" id={MAIN_CONTENT_ID} ref={mainRef} tabIndex={-1}>
          <AppBoundary>
            <Routes>
              <Route path={ROUTE_PATHS.landing} element={<LandingRoute />} />
              <Route path={ROUTE_PATHS.setup} element={<SetupRoute />} />
              <Route
                path={ROUTE_PATHS.login}
                element={
                  <AuthGate mode="login">
                    <LoginPage />
                  </AuthGate>
                }
              />
              <Route
                path={ROUTE_PATHS.dashboard}
                element={
                  <AuthGate mode="protected">
                    <DashboardPage />
                  </AuthGate>
                }
              />
              <Route
                path={ROUTE_PATHS.chart}
                element={
                  <AuthGate mode="protected">
                    <ChartPage />
                  </AuthGate>
                }
              />
              <Route path="*" element={<Navigate to={ROUTE_PATHS.landing} replace />} />
            </Routes>
          </AppBoundary>
        </main>
      </div>
    </AuthProvider>
  );
}
