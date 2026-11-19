import type { ButtonHTMLAttributes, ReactNode } from 'react';

import './Button.css';

type ButtonVariant = 'primary' | 'subtle';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  loading?: boolean;
  loadingLabel?: ReactNode;
};

export function Button({
  className,
  type = 'button',
  variant = 'primary',
  loading = false,
  loadingLabel,
  disabled,
  children,
  ...rest
}: ButtonProps) {
  const classes = ['button', `button--${variant}`, className].filter(Boolean).join(' ');

  return (
    <button
      className={classes}
      type={type}
      disabled={disabled === true || loading}
      aria-busy={loading ? true : undefined}
      {...rest}
    >
      {loading ? (
        <>
          <span className="button__spinner" aria-hidden="true" />
          {loadingLabel ?? children}
        </>
      ) : (
        children
      )}
    </button>
  );
}
