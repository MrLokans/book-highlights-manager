import { Page } from '@playwright/test';
import { Sel } from './selectors';

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
  await page.fill(Sel.usernameInput, username);
  await page.fill(Sel.passwordInput, password);
  await page.click(Sel.authSubmit);
  // Wait for redirect to books page
  await page.waitForURL('/', { timeout: 5_000 });
}
