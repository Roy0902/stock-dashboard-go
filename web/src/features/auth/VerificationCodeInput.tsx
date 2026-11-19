import { useRef } from 'react';
import type { Ref } from 'react';

import { copy } from '@/locale';

type VerificationCodeInputProps = {
  id: string;
  value: string;
  onChange: (value: string) => void;
  /** Called once when the field reaches six digits (typing or paste). */
  onComplete?: (value: string) => void;
  /** Extra description ids, e.g. the step lede and the challenge expiry. */
  describedBy?: string[];
  error?: string | null;
  inputRef?: Ref<HTMLInputElement>;
  disabled?: boolean;
};

const CODE_LENGTH = 6;

export function VerificationCodeInput({
  id,
  value,
  onChange,
  onComplete,
  describedBy,
  error,
  inputRef,
  disabled,
}: VerificationCodeInputProps) {
  const errorId = `${id}-error`;
  const completedValue = useRef<string | null>(null);
  const ids = [...(describedBy ?? []), ...(error == null ? [] : [errorId])];

  const handleChange = (raw: string) => {
    const next = raw.replace(/\D/g, '').slice(0, CODE_LENGTH);
    onChange(next);

    if (next.length < CODE_LENGTH) {
      completedValue.current = null;
      return;
    }
    if (onComplete === undefined || completedValue.current === next) return;
    completedValue.current = next;
    onComplete(next);
  };

  return (
    <div className="auth-form__field">
      <label className="auth-form__label" htmlFor={id}>
        {copy.auth.fields.verificationCode}
      </label>
      <input
        id={id}
        ref={inputRef}
        className="auth-form__input auth-form__input--code"
        name="code"
        type="text"
        value={value}
        inputMode="numeric"
        autoComplete="one-time-code"
        pattern="[0-9]{6}"
        maxLength={CODE_LENGTH}
        disabled={disabled}
        aria-invalid={error == null ? undefined : true}
        aria-describedby={ids.length === 0 ? undefined : ids.join(' ')}
        onChange={(event) => {
          handleChange(event.target.value);
        }}
      />
      {error == null ? null : (
        <p className="auth-form__field-error" id={errorId}>
          {error}
        </p>
      )}
    </div>
  );
}
