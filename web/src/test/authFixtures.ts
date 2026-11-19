import type { AuthStatus, AuthUser, SetupMaterial } from '@/api/types';

export const OWNER_USER: AuthUser = { username: 'owner', totpEnabled: true };

export const SETUP_SECRET = 'JBSWY3DPEHPK3PXP';

export const SETUP_OTPAUTH_URL =
  'otpauth://totp/Stock%20Dashboard:owner?secret=JBSWY3DPEHPK3PXP&issuer=Stock%20Dashboard&algorithm=SHA1&digits=6&period=30';

export const RECOVERY_CODES = [
  'AAAA-BBBB-CCCC-0000',
  'BBBB-CCCC-DDDD-0001',
  'CCCC-DDDD-EEEE-0002',
  'DDDD-EEEE-FFFF-0003',
  'EEEE-FFFF-GGGG-0004',
  'FFFF-GGGG-HHHH-0005',
  'GGGG-HHHH-IIII-0006',
  'HHHH-IIII-JJJJ-0007',
  'IIII-JJJJ-KKKK-0008',
  'JJJJ-KKKK-LLLL-0009',
];

export function makeSetupMaterial(overrides: Partial<SetupMaterial> = {}): SetupMaterial {
  return {
    username: OWNER_USER.username,
    secret: SETUP_SECRET,
    otpauthUrl: SETUP_OTPAUTH_URL,
    recoveryCodes: [...RECOVERY_CODES],
    ...overrides,
  };
}

export function makeAuthenticatedStatus(): AuthStatus {
  return { setupRequired: false, authenticated: true, user: OWNER_USER };
}

export function makeUnauthenticatedStatus(): AuthStatus {
  return { setupRequired: false, authenticated: false };
}

export function makeSetupRequiredStatus(): AuthStatus {
  return { setupRequired: true, authenticated: false };
}
