import type { InputHTMLAttributes, Ref } from 'react';

type AuthTextFieldProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'id' | 'className' | 'aria-describedby' | 'aria-invalid'
> & {
  id: string;
  label: string;
  hint?: string;
  error?: string | null;
  inputRef?: Ref<HTMLInputElement>;
};

/**
 * Labelled text input that owns the hint/error wiring: the message is rendered next to the
 * field and referenced by `aria-describedby`, and the field is marked `aria-invalid` when it
 * carries an error. Server errors stay in the single page-level alert instead.
 */
export function AuthTextField({ id, label, hint, error, inputRef, ...rest }: AuthTextFieldProps) {
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
      <input
        {...rest}
        id={id}
        ref={inputRef}
        className="auth-form__input"
        aria-invalid={error == null ? undefined : true}
        aria-describedby={describedBy === '' ? undefined : describedBy}
      />
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
