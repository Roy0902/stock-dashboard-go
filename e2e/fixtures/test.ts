import { test as base, expect as baseExpect } from '@playwright/test';
import { bootstrapAuth, type OwnerCredentials } from './auth';
import { startApp, type AppInstance, type StartOptions } from './app-server';

export type AppOptions = StartOptions & {
  /** Perform first-run setup and keep the session. Default true. */
  bootstrap?: boolean;
  /** Keep the session cookie after bootstrap. Default true. */
  authenticated?: boolean;
};

export const test = base.extend<{
  appOptions: AppOptions;
  app: AppInstance;
  auth: OwnerCredentials | null;
}>({
  appOptions: [{}, { option: true }],

  app: async ({ appOptions }, use) => {
    const app = await startApp(appOptions);
    await use(app);
    await app.stop();
  },

  auth: [
    async ({ app, context, appOptions }, use) => {
      if (appOptions.bootstrap === false) {
        await use(null);
        return;
      }
      const credentials = await bootstrapAuth(app, context);
      if (appOptions.authenticated === false) {
        await context.clearCookies();
      }
      await use(credentials);
    },
    { auto: true },
  ],
});

export const expect = baseExpect;

export type { AppInstance, StartOptions };
