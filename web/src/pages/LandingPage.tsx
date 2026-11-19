import { Link } from 'react-router-dom';

import { copy } from '@/locale';
import { ROUTE_PATHS } from '@/routes';
import './LandingPage.css';

export function LandingPage() {
  return (
    <div className="landing">
      <section className="landing__hero" aria-labelledby="landing-title">
        <p className="landing__eyebrow">{copy.landing.eyebrow}</p>
        <h1 className="landing__title" id="landing-title">
          {copy.landing.title}
        </h1>
        <p className="landing__lede">{copy.landing.lede}</p>
        <div className="landing__actions">
          <Link className="landing__cta" to={ROUTE_PATHS.login}>
            {copy.landing.signIn}
            <svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">
              <path
                d="M3 8h9M8.5 3.5 13 8l-4.5 4.5"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </Link>
          <span className="landing__hint">{copy.landing.hint}</span>
        </div>
      </section>

      <section className="landing__features" aria-label={copy.landing.featuresLabel}>
        {copy.landing.features.map((feature) => (
          <article className="landing__feature" key={feature.title}>
            <h2 className="landing__feature-title">{feature.title}</h2>
            <p className="landing__feature-body">{feature.body}</p>
          </article>
        ))}
      </section>

      <p className="landing__footnote">{copy.landing.footnote(window.location.host)}</p>
    </div>
  );
}
