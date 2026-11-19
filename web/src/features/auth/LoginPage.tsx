import { useEffect, useId, useRef, useState } from 'react';
import type { FormEvent, RefObject } from 'react';
import { Link } from 'react-router-dom';

import { login, loginRecovery, loginTotp } from '@/api/client';
import { AuthPanel } from '@/features/auth/AuthPanel';
import { AuthPasswordField } from '@/features/auth/AuthPasswordField';
import { AuthTextField } from '@/features/auth/AuthTextField';
import { useAuth } from '@/features/auth/useAuth';
import { VerificationCodeInput } from '@/features/auth/VerificationCodeInput';
import {
  firstInvalidField,
  hasFieldErrors,
  normalizeRecoveryCode,
  normalizeUsername,
  validateCode,
  validateCredentials,
  validateRecovery,
} from '@/features/auth/validation';
import type { AuthFieldName, FieldErrors } from '@/features/auth/validation';
import { copy } from '@/locale';
import { ROUTE_PATHS } from '@/routes';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import './AuthForms.css';

type LoginStep = 'credentials' | 'totp' | 'recovery';

export function LoginPage() {
  const { acceptUser, clearNotice, notice } = useAuth();
  const [step, setStep] = useState<LoginStep>('credentials');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [recoveryCode, setRecoveryCode] = useState('');
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [expiresInSeconds, setExpiresInSeconds] = useState<number | null>(null);
  const [pending, setPending] = useState(false);
  const stepRef = useRef<LoginStep>('credentials');
  const requestRef = useRef(0);
  const previousStepRef = useRef<LoginStep | null>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const usernameRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);
  const codeRef = useRef<HTMLInputElement>(null);
  const recoveryRef = useRef<HTMLInputElement>(null);
  const usernameId = useId();
  const passwordId = useId();
  const codeId = useId();
  const recoveryId = useId();
  const totpLedeId = useId();
  const totpExpiryId = useId();

  const fieldRefs: Record<AuthFieldName, RefObject<HTMLInputElement | null>> = {
    username: usernameRef,
    password: passwordRef,
    confirmPassword: passwordRef,
    code: codeRef,
    recoveryCode: recoveryRef,
  };

  // Focus follows a step change, not the first render: on initial load the skip link must
  // remain the first focusable target.
  useEffect(() => {
    const previous = previousStepRef.current;
    previousStepRef.current = step;
    if (previous === null || previous === step) return;

    if (step === 'recovery') {
      recoveryRef.current?.focus();
      return;
    }
    headingRef.current?.focus();
  }, [step]);

  // Switching steps invalidates any request still in flight for the previous step.
  const changeStep = (next: LoginStep) => {
    stepRef.current = next;
    requestRef.current += 1;
    setPending(false);
    setStep(next);
  };

  const clearFieldError = (field: AuthFieldName) => {
    setFieldErrors((current) =>
      current[field] === undefined ? current : { ...current, [field]: undefined },
    );
  };

  const focusInvalid = (errors: FieldErrors) => {
    const field = firstInvalidField(errors);
    if (field !== null) {
      fieldRefs[field].current?.focus();
    }
  };

  const submitCredentials = async () => {
    if (pending) return;

    const errors = validateCredentials({ username, password });
    setFieldErrors(errors);
    setError(null);
    setAttempt((value) => value + 1);
    if (hasFieldErrors(errors)) {
      focusInvalid(errors);
      return;
    }

    const requestId = ++requestRef.current;
    const submittedStep = stepRef.current;
    setPending(true);
    const result = await login(normalizeUsername(username), password);
    if (requestId !== requestRef.current || stepRef.current !== submittedStep) return;
    setPending(false);

    if (!result.ok) {
      setError(result.error.message);
      return;
    }

    setPassword('');
    setCode('');
    setRecoveryCode('');
    setExpiresInSeconds(result.value.expiresInSeconds);
    clearNotice();
    changeStep('totp');
  };

  const submitCode = async (value: string) => {
    if (pending) return;

    const errors = validateCode(value);
    setFieldErrors(errors);
    setError(null);
    setAttempt((valueCount) => valueCount + 1);
    if (hasFieldErrors(errors)) {
      focusInvalid(errors);
      return;
    }

    const requestId = ++requestRef.current;
    const submittedStep = stepRef.current;
    setPending(true);
    const result = await loginTotp(value);
    if (requestId !== requestRef.current || stepRef.current !== submittedStep) return;
    setPending(false);

    if (!result.ok) {
      if (result.error.code === 'CHALLENGE_EXPIRED') {
        setCode('');
        changeStep('credentials');
      }
      setError(result.error.message);
      return;
    }

    acceptUser(result.value.user);
  };

  const submitRecovery = async () => {
    if (pending) return;

    const errors = validateRecovery(recoveryCode);
    setFieldErrors(errors);
    setError(null);
    setAttempt((value) => value + 1);
    if (hasFieldErrors(errors)) {
      focusInvalid(errors);
      return;
    }

    const requestId = ++requestRef.current;
    const submittedStep = stepRef.current;
    setPending(true);
    const result = await loginRecovery(normalizeRecoveryCode(recoveryCode));
    if (requestId !== requestRef.current || stepRef.current !== submittedStep) return;
    setPending(false);

    if (!result.ok) {
      if (result.error.code === 'CHALLENGE_EXPIRED') {
        setRecoveryCode('');
        changeStep('credentials');
      }
      setError(result.error.message);
      return;
    }

    acceptUser(result.value.user);
  };

  const handleCredentials = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submitCredentials();
  };

  const handleCode = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submitCode(code);
  };

  const handleRecovery = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submitRecovery();
  };

  const heading =
    step === 'credentials'
      ? copy.auth.headings.signIn
      : step === 'totp'
        ? copy.auth.headings.twoFactor
        : copy.auth.headings.recovery;

  const totpDescribedBy = [totpLedeId, ...(expiresInSeconds === null ? [] : [totpExpiryId])];

  return (
    <AuthPanel>
      <h1 className="auth-panel__heading" ref={headingRef} tabIndex={-1}>
        {heading}
      </h1>

      {step === 'credentials' ? <p className="auth-panel__lede">{copy.auth.ledes.signIn}</p> : null}
      {step === 'totp' ? (
        <p className="auth-panel__lede" id={totpLedeId}>
          {copy.auth.ledes.twoFactor}
        </p>
      ) : null}
      {step === 'recovery' ? <p className="auth-panel__lede">{copy.auth.ledes.recovery}</p> : null}

      {error !== null ? (
        <div className="auth-form__error">
          <Alert key={`attempt-${attempt}`}>{error}</Alert>
        </div>
      ) : notice === null ? null : (
        <div className="auth-form__error">
          <Alert key="notice">{notice}</Alert>
        </div>
      )}

      {step === 'credentials' ? (
        <form className="auth-form" onSubmit={handleCredentials} aria-busy={pending} noValidate>
          <AuthTextField
            id={usernameId}
            label={copy.auth.fields.username}
            name="username"
            type="text"
            value={username}
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            error={fieldErrors.username}
            inputRef={usernameRef}
            onChange={(event) => {
              setUsername(event.target.value);
              clearFieldError('username');
            }}
          />

          <AuthPasswordField
            id={passwordId}
            name="password"
            label={copy.auth.fields.password}
            value={password}
            autoComplete="current-password"
            error={fieldErrors.password}
            inputRef={passwordRef}
            onChange={(value) => {
              setPassword(value);
              clearFieldError('password');
            }}
          />

          <div className="auth-form__actions">
            <Button type="submit" disabled={pending}>
              {copy.auth.actions.signIn}
            </Button>
          </div>
        </form>
      ) : step === 'totp' ? (
        <form className="auth-form" onSubmit={handleCode} aria-busy={pending} noValidate>
          <VerificationCodeInput
            id={codeId}
            value={code}
            onChange={(value) => {
              setCode(value);
              clearFieldError('code');
            }}
            onComplete={(value) => {
              void submitCode(value);
            }}
            describedBy={totpDescribedBy}
            error={fieldErrors.code}
            inputRef={codeRef}
          />

          {expiresInSeconds === null ? null : (
            <p className="auth-form__hint" id={totpExpiryId}>
              {copy.auth.codeExpiry(expiresInSeconds)}
            </p>
          )}

          <div className="auth-form__actions">
            <Button type="submit" disabled={pending || code.length !== 6}>
              {copy.auth.actions.continue}
            </Button>
          </div>

          <div className="auth-form__actions">
            <Button
              className="auth-form__secondary"
              variant="subtle"
              onClick={() => {
                setError(null);
                setFieldErrors({});
                setRecoveryCode('');
                changeStep('recovery');
              }}
            >
              {copy.auth.actions.useRecoveryCode}
            </Button>
            <Button
              className="auth-form__secondary"
              variant="subtle"
              onClick={() => {
                setError(null);
                setFieldErrors({});
                setCode('');
                changeStep('credentials');
              }}
            >
              {copy.auth.actions.backToSignIn}
            </Button>
          </div>
        </form>
      ) : (
        <form className="auth-form" onSubmit={handleRecovery} aria-busy={pending} noValidate>
          <AuthTextField
            id={recoveryId}
            label={copy.auth.fields.recoveryCode}
            name="recovery-code"
            type="text"
            value={recoveryCode}
            autoComplete="off"
            autoCapitalize="characters"
            spellCheck={false}
            error={fieldErrors.recoveryCode}
            inputRef={recoveryRef}
            onChange={(event) => {
              setRecoveryCode(event.target.value);
              clearFieldError('recoveryCode');
            }}
          />

          <div className="auth-form__actions">
            <Button type="submit" disabled={pending}>
              {copy.auth.actions.continue}
            </Button>
          </div>

          <div className="auth-form__actions">
            <Button
              className="auth-form__secondary"
              variant="subtle"
              onClick={() => {
                setError(null);
                setFieldErrors({});
                setRecoveryCode('');
                changeStep('credentials');
              }}
            >
              {copy.auth.actions.backToSignIn}
            </Button>
          </div>
        </form>
      )}

      {step === 'credentials' ? (
        <p className="auth-form__switch">
          {copy.auth.switch.prompt}{' '}
          <Link to={ROUTE_PATHS.setup}>{copy.auth.switch.createAccount}</Link>
        </p>
      ) : null}
    </AuthPanel>
  );
}
