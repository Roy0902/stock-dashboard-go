import { copy } from '@/locale';

const USERNAME_PATTERN = /^[a-z0-9][a-z0-9._-]{2,31}$/;
const RECOVERY_CODE_PATTERN = /^[A-Z2-7]{16}$/;
export const MIN_PASSWORD_LENGTH = 12;

export type AuthFieldName = 'username' | 'password' | 'confirmPassword' | 'code' | 'recoveryCode';

export type FieldErrors = Partial<Record<AuthFieldName, string>>;

const FIELD_ORDER: readonly AuthFieldName[] = [
  'username',
  'password',
  'confirmPassword',
  'code',
  'recoveryCode',
];

export function hasFieldErrors(errors: FieldErrors): boolean {
  return FIELD_ORDER.some((field) => errors[field] !== undefined);
}

/** Field order as it appears on screen, for "focus the first invalid input". */
export function firstInvalidField(errors: FieldErrors): AuthFieldName | null {
  return FIELD_ORDER.find((field) => errors[field] !== undefined) ?? null;
}

export function normalizeUsername(value: string): string {
  return value.trim();
}

export function validateUsername(value: string): string | null {
  return USERNAME_PATTERN.test(normalizeUsername(value)) ? null : copy.errors.username;
}

export function validatePassword(value: string): string | null {
  const length = Array.from(value).length;
  return length < MIN_PASSWORD_LENGTH ? copy.errors.weakPassword : null;
}

export function validateVerificationCode(value: string): string | null {
  return /^\d{6}$/.test(value.trim()) ? null : copy.errors.invalidCode;
}

/** Canonical form the server compares against: no spaces or hyphens, uppercase. */
export function normalizeRecoveryCode(value: string): string {
  return value.trim().replace(/[\s-]/g, '').toUpperCase();
}

export function validateRecoveryCode(value: string): string | null {
  return RECOVERY_CODE_PATTERN.test(normalizeRecoveryCode(value))
    ? null
    : copy.errors.invalidRecoveryCode;
}

type CredentialsInput = {
  username: string;
  password: string;
};

export function validateCredentials({ username, password }: CredentialsInput): FieldErrors {
  const errors: FieldErrors = {};
  if (username.trim().length === 0) {
    errors.username = copy.errors.missingCredentials;
  }
  if (password.length === 0) {
    errors.password = copy.errors.missingCredentials;
  }
  return errors;
}

type SetupInput = {
  username: string;
  password: string;
  confirmPassword: string;
};

export function validateSetup({ username, password, confirmPassword }: SetupInput): FieldErrors {
  const errors: FieldErrors = {};
  const usernameError = validateUsername(username);
  if (usernameError !== null) {
    errors.username = usernameError;
  }
  const passwordError = validatePassword(password);
  if (passwordError !== null) {
    errors.password = passwordError;
  }
  if (password !== confirmPassword) {
    errors.confirmPassword = copy.errors.passwordMismatch;
  }
  return errors;
}

export function validateCode(value: string): FieldErrors {
  const error = validateVerificationCode(value);
  return error === null ? {} : { code: error };
}

export function validateRecovery(value: string): FieldErrors {
  const error = validateRecoveryCode(value);
  return error === null ? {} : { recoveryCode: error };
}
