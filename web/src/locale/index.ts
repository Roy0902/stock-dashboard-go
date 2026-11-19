import { en } from '@/locale/en';
import type { Copy } from '@/locale/en';

/**
 * Single entry point for copy. Add a locale to `LOCALES` and switch `ACTIVE_LOCALE`;
 * components only ever read from `copy`.
 */
export const LOCALES: Record<string, Copy> = { en };
export const ACTIVE_LOCALE = 'en';
export const copy: Copy = LOCALES[ACTIVE_LOCALE] ?? en;

export type { Copy };
