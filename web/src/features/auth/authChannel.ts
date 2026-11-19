/**
 * Cross-tab signal for account changes. The session lives in a shared `HttpOnly` cookie, so
 * a sign-out or account switch in one tab invalidates every other tab's cached data.
 * `BroadcastChannel` carries only a coarse event plus the sending tab id; no tokens,
 * usernames or session data.
 */
export type AuthChangeEvent = 'signed-out' | 'account-changed';

export type AuthChangeMessage = {
  event: AuthChangeEvent;
  origin: string;
};

const CHANNEL_NAME = 'stock-dashboard-auth';

function createTabId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `tab-${Math.random().toString(36).slice(2)}`;
}

/** Identifies this tab so it can ignore the echo of its own broadcast. */
export const AUTH_TAB_ID = createTabId();

function isAuthChangeMessage(value: unknown): value is AuthChangeMessage {
  if (typeof value !== 'object' || value === null) return false;
  const { event, origin } = value as { event?: unknown; origin?: unknown };
  return (event === 'signed-out' || event === 'account-changed') && typeof origin === 'string';
}

function openChannel(): BroadcastChannel | null {
  if (typeof BroadcastChannel === 'undefined') return null;
  return new BroadcastChannel(CHANNEL_NAME);
}

export function broadcastAuthChange(event: AuthChangeEvent): void {
  const channel = openChannel();
  if (channel === null) return;
  const message: AuthChangeMessage = { event, origin: AUTH_TAB_ID };
  channel.postMessage(message);
  channel.close();
}

export function subscribeAuthChange(listener: (message: AuthChangeMessage) => void): () => void {
  const channel = openChannel();
  if (channel === null) return () => undefined;

  channel.onmessage = (message: MessageEvent<unknown>) => {
    if (isAuthChangeMessage(message.data)) {
      listener(message.data);
    }
  };

  return () => {
    channel.close();
  };
}
