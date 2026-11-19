import { useState } from 'react';
import type { Ref } from 'react';

import { copy } from '@/locale';
import { Button } from '@/ui/Button';

type AuthPasswordFieldProps = {
  id: string;
  name: string;
  label: string;
  value: string;
  autoComplete: 'current-password' | 'new-password';
  onChange: (value: string) => void;
  hint?: string;
  error?: string | null;
  minLength?: number;
  inputRef?: Ref<HTMLInputElement>;
};

/**
 * Password input with a stated rule, `minLength` and a reveal toggle. The toggle is a
 * pressed button so the state is exposed without relying on the label alone.
 */
export function AuthPasswordField({
  id,
  name,
  label,
  value,
  autoComplete,
  onChange,
  hint,
  error,
  minLength,
  inputRef,
}: AuthPasswordFieldProps) {
  const [revealed, setRevealed] = useState(false);
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  // A hint that repeats the error would be announced twice; the error already states it.
  const showHint = hint !== undefined && hint !== error;
  const describedBy = [showHint ? hintId : null, error == null ? null : errorId]
    .filter((value): value is string => value !== null)
    .join(' ');

  return (
    <div className="auth-form__field">
      <label className="auth-form__label" htmlFor={id}>
        {label}
      </label>
      <div className="auth-form__secret">
        <input
          id={id}
          ref={inputRef}
          className="auth-form__input"
          name={name}
          type={revealed ? 'text' : 'password'}
          value={value}
          autoComplete={autoComplete}
          minLength={minLength}
          aria-invalid={error == null ? undefined : true}
          aria-describedby={describedBy === '' ? undefined : describedBy}
          onChange={(event) => {
            onChange(event.target.value);
          }}
        />
        <Button
          className="auth-form__reveal"
          variant="subtle"
          aria-pressed={revealed}
          aria-controls={id}
          onClick={() => {
            setRevealed((current) => !current);
          }}
        >
          {revealed ? copy.auth.password.hide : copy.auth.password.show}
        </Button>
      </div>
      {showHint ? (
        <p className="auth-form__hint" id={hintId}>
          {hint}
        </p>
      ) : null}
      {error == null ? null : (
        <p className="auth-form__field-error" id={errorId}>
          {error}
        </p>
      )}
    </div>
  );
}
