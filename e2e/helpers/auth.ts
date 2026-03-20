import { Page } from '@playwright/test';

export const TEST_USER = {
  username: 'testuser',
  email: 'test@test.com',
  password: 'testpassword12',
};

export async function login(
  page: Page,
  username = TEST_USER.username,
  password = TEST_USER.password
): Promise<void> {
  await page.goto('/login');
  await page.fill('#username', username);
  await page.fill('#password', password);
  await page.click('.auth-submit');
  // Wait for redirect to books page
  await page.waitForURL('/', { timeout: 5_000 });
}
