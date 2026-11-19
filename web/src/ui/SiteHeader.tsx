import { Link, useLocation, useNavigate } from 'react-router-dom';

import { useAuth } from '@/features/auth/useAuth';
import { copy } from '@/locale';
import { ROUTE_PATHS } from '@/routes';
import { BrandMark } from '@/ui/BrandMark';
import { Button } from '@/ui/Button';
import './SiteHeader.css';

export function SiteHeader() {
  const { user, signOut } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const onAuthPage =
    location.pathname === ROUTE_PATHS.setup || location.pathname === ROUTE_PATHS.login;

  const handleSignOut = async () => {
    await signOut();
    await navigate(ROUTE_PATHS.login, { replace: true });
  };

  if (onAuthPage) {
    return null;
  }

  return (
    <header className="site-header">
      <div className="site-header__inner">
        <Link className="site-header__brand" to={ROUTE_PATHS.landing}>
          <span className="site-header__mark" aria-hidden="true">
            <BrandMark />
          </span>
          <span className="site-header__name">{copy.brand.name}</span>
        </Link>
        <p className="site-header__descriptor">{copy.header.descriptor}</p>
        {user !== null && !onAuthPage ? (
          <div className="site-header__account">
            <span className="site-header__username">{user.username}</span>
            <Button
              className="site-header__signout"
              variant="subtle"
              onClick={() => {
                void handleSignOut();
              }}
            >
              {copy.header.signOut}
            </Button>
          </div>
        ) : null}
      </div>
    </header>
  );
}
