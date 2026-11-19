import type { ReactNode } from 'react';

import './Alert.css';

type AlertTone = 'error' | 'neutral';

type AlertProps = {
  children: ReactNode;
  tone?: AlertTone;
  className?: string;
};

export function Alert({ children, tone = 'error', className }: AlertProps) {
  const classes = ['alert', `alert--${tone}`, className].filter(Boolean).join(' ');

  return (
    <div className={classes} role="alert">
      <svg className="alert__icon" viewBox="0 0 20 20" aria-hidden="true" focusable="false">
        <circle cx="10" cy="10" r="8" fill="none" stroke="currentColor" strokeWidth="1.5" />
        {tone === 'error' ? (
          <>
            <path
              d="M10 5.75v5"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
            <circle cx="10" cy="14" r="1" fill="currentColor" />
          </>
        ) : (
          <>
            <circle cx="10" cy="10" r="0.9" fill="currentColor" />
            <path
              d="M10 13.1V8.4"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </>
        )}
      </svg>
      <p className="alert__message">{children}</p>
    </div>
  );
}
