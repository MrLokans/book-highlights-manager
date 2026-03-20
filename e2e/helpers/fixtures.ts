import { test as base, Page } from '@playwright/test';
import { startApp, AppInstance } from './app';
import { login } from './auth';

type E2EFixtures = {
  app: AppInstance;
  appUrl: string;
  authedPage: Page;
  unauthPage: Page;
};

export const test = base.extend<E2EFixtures>({
  app: [
    async ({}, use) => {
      const app = await startApp();
      await use(app);
      await app.stop();
    },
    { scope: 'test' },
  ],

  appUrl: async ({ app }, use) => {
    await use(app.url);
  },

  authedPage: async ({ browser, app }, use) => {
    const context = await browser.newContext({ baseURL: app.url });
    const page = await context.newPage();
    await login(page);
    await use(page);
    await context.close();
  },

  unauthPage: async ({ browser, app }, use) => {
    const context = await browser.newContext({ baseURL: app.url });
    const page = await context.newPage();
    await use(page);
    await context.close();
  },
});

export { expect } from '@playwright/test';
