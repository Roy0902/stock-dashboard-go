import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';

import { copy } from '@/locale';
import { ROUTE_PATHS } from '@/routes';
import { BrandMark } from '@/ui/BrandMark';
import { Panel } from '@/ui/Panel';
import './AuthPanel.css';

type AuthPanelProps = {
  children: ReactNode;
  wide?: boolean;
};

export function AuthPanel({ children, wide = false }: AuthPanelProps) {
  return (
    <div className="auth-page">
      <div className="auth-page__brand">
        <Link className="auth-brand" to={ROUTE_PATHS.landing} aria-label={copy.brand.homeLabel}>
          <span className="auth-brand__mark" aria-hidden="true">
            <BrandMark />
          </span>
          <span className="auth-brand__name">{copy.brand.name}</span>
        </Link>
      </div>

      <Panel className={`auth-panel${wide ? ' auth-panel--wide' : ''}`}>{children}</Panel>

      <p className="auth-page__note">{copy.auth.localOnlyNote}</p>
    </div>
  );
}
