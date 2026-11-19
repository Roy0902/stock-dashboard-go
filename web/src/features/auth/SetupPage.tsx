import { useEffect, useId, useRef, useState } from 'react';
import type { FormEvent, RefObject } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { QRCodeSVG } from 'qrcode.react';

import { activateTotp, setupAccount } from '@/api/client';
import type { SetupMaterial } from '@/api/types';
import { AuthPanel } from '@/features/auth/AuthPanel';
import { AuthPasswordField } from '@/features/auth/AuthPasswordField';
import { AuthTextField } from '@/features/auth/AuthTextField';
import { RecoveryCodeList } from '@/features/auth/RecoveryCodeList';
import { useAuth } from '@/features/auth/useAuth';
import { VerificationCodeInput } from '@/features/auth/VerificationCodeInput';
import {
  firstInvalidField,
  hasFieldErrors,
  MIN_PASSWORD_LENGTH,
  normalizeUsername,
  validateCode,
  validateSetup,
} from '@/features/auth/validation';
import type { AuthFieldName, FieldErrors } from '@/features/auth/validation';
import { copy } from '@/locale';
import { resolvePostAuthPath } from '@/routes';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import './AuthForms.css';

type SetupStage = 'form' | 'verify';

export function SetupPage() {
  const { acceptUser } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [material, setMaterial] = useState<SetupMaterial | null>(null);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [code, setCode] = useState('');
  const [confirmed, setConfirmed] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [pending, setPending] = useState(false);
  const previousStageRef = useRef<SetupStage | null>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const verifyHeadingRef = useRef<HTMLHeadingElement>(null);
  const usernameRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);
  const confirmPasswordRef = useRef<HTMLInputElement>(null);
  const codeRef = useRef<HTMLInputElement>(null);
  const usernameId = useId();
  const passwordId = useId();
  const confirmPasswordId = useId();
  const codeId = useId();

  const fieldRefs: Record<AuthFieldName, RefObject<HTMLInputElement | null>> = {
    username: usernameRef,
    password: passwordRef,
    confirmPassword: confirmPasswordRef,
    code: codeRef,
    recoveryCode: codeRef,
  };

  // Focus follows a stage change, not the first render: on initial load the skip link must
  // remain the first focusable target.
  useEffect(() => {
    const stage: SetupStage = material === null ? 'form' : 'verify';
    const previous = previousStageRef.current;
    previousStageRef.current = stage;
    if (previous === null || previous === stage) return;

    if (stage === 'form') {
      headingRef.current?.focus();
      return;
    }
    verifyHeadingRef.current?.focus();
  }, [material]);

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

  const submitCreate = async () => {
    if (pending) return;

    const errors = validateSetup({ username, password, confirmPassword });
    setFieldErrors(errors);
    setError(null);
    setAttempt((value) => value + 1);
    if (hasFieldErrors(errors)) {
      focusInvalid(errors);
      return;
    }

    setPending(true);
    const result = await setupAccount(normalizeUsername(username), password);
    setPending(false);

    if (!result.ok) {
      setError(result.error.message);
      return;
    }

    setUsername(result.value.username);
    setPassword('');
    setConfirmPassword('');
    setCode('');
    setConfirmed(false);
    setMaterial(result.value);
  };

  const submitActivate = async (value: string) => {
    if (pending) return;

    const errors = validateCode(value);
    setFieldErrors(errors);
    setError(null);
    setAttempt((count) => count + 1);
    if (hasFieldErrors(errors)) {
      focusInvalid(errors);
      return;
    }

    setPending(true);
    const result = await activateTotp(value);
    setPending(false);

    if (!result.ok) {
      if (result.error.code === 'ENROLLMENT_EXPIRED') {
        setMaterial(null);
        setCode('');
        setConfirmed(false);
      }
      setError(result.error.message);
      return;
    }

    acceptUser(result.value.user);
    await navigate(resolvePostAuthPath(location.state), { replace: true });
  };

  const handleCreate = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submitCreate();
  };

  const handleActivate = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void submitActivate(code);
  };

  if (material === null) {
    return (
      <AuthPanel>
        <h1 className="auth-panel__heading" ref={headingRef} tabIndex={-1}>
          {copy.auth.headings.createAccount}
        </h1>

        {error === null ? null : (
          <div className="auth-form__error">
            <Alert key={`attempt-${attempt}`}>{error}</Alert>
          </div>
        )}

        <form className="auth-form" onSubmit={handleCreate} aria-busy={pending} noValidate>
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
            autoComplete="new-password"
            minLength={MIN_PASSWORD_LENGTH}
            hint={copy.errors.weakPassword}
            error={fieldErrors.password}
            inputRef={passwordRef}
            onChange={(value) => {
              setPassword(value);
              clearFieldError('password');
            }}
          />

          <AuthPasswordField
            id={confirmPasswordId}
            name="confirm-password"
            label={copy.auth.fields.confirmPassword}
            value={confirmPassword}
            autoComplete="new-password"
            minLength={MIN_PASSWORD_LENGTH}
            error={fieldErrors.confirmPassword}
            inputRef={confirmPasswordRef}
            onChange={(value) => {
              setConfirmPassword(value);
              clearFieldError('confirmPassword');
            }}
          />

          <div className="auth-form__actions">
            <Button type="submit" disabled={pending}>
              {copy.auth.actions.createAccount}
            </Button>
          </div>
        </form>
      </AuthPanel>
    );
  }

  return (
    <AuthPanel wide>
      <h1 className="auth-panel__heading">{copy.auth.headings.createAccount}</h1>
      <h2 className="auth-panel__subheading" ref={verifyHeadingRef} tabIndex={-1}>
        {copy.auth.headings.authenticator}
      </h2>

      {error === null ? null : (
        <div className="auth-form__error">
          <Alert key={`attempt-${attempt}`}>{error}</Alert>
        </div>
      )}

      <form className="auth-form" onSubmit={handleActivate} aria-busy={pending} noValidate>
        <div className="auth-setup">
          <div className="auth-setup__qr">
            <div className="auth-setup__qr-frame">
              <QRCodeSVG
                value={material.otpauthUrl}
                size={192}
                level="M"
                marginSize={2}
                bgColor="#ffffff"
                fgColor="#000000"
                title={copy.auth.setup.qrTitle}
              />
            </div>
          </div>
          <p className="auth-form__hint">{copy.auth.setup.scanQr}</p>

          <div className="auth-setup__key">
            <p className="auth-form__hint">{copy.auth.setup.manualKey}</p>
            <code className="auth-setup__secret">{material.secret}</code>
          </div>

          <p className="auth-form__hint">{copy.auth.setup.saveCodes}</p>
          <RecoveryCodeList codes={material.recoveryCodes} />

          <label className="auth-checkbox">
            <input
              type="checkbox"
              checked={confirmed}
              onChange={(event) => {
                setConfirmed(event.target.checked);
              }}
            />
            <span>{copy.auth.setup.savedCodes}</span>
          </label>

          <VerificationCodeInput
            id={codeId}
            value={code}
            onChange={(value) => {
              setCode(value);
              clearFieldError('code');
            }}
            onComplete={
              confirmed
                ? (value) => {
                    void submitActivate(value);
                  }
                : undefined
            }
            error={fieldErrors.code}
            inputRef={codeRef}
          />

          <div className="auth-form__actions">
            <Button type="submit" disabled={pending || !confirmed || code.length !== 6}>
              {copy.auth.actions.continue}
            </Button>
          </div>
        </div>
      </form>
    </AuthPanel>
  );
}
