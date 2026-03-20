import { test, expect } from '../helpers/fixtures';
import { TEST_USER } from '../helpers/auth';

test.describe('Authentication', () => {
  test('login with valid credentials redirects to books page', async ({ unauthPage }) => {
    await unauthPage.goto('/login');
    await unauthPage.fill('#username', TEST_USER.username);
    await unauthPage.fill('#password', TEST_USER.password);
    await unauthPage.click('.auth-submit');

    await unauthPage.waitForURL('/');
    await expect(unauthPage.locator('.book-card')).toHaveCount(5);
  });

  test('login with invalid credentials shows error', async ({ unauthPage }) => {
    await unauthPage.goto('/login');
    await unauthPage.fill('#username', TEST_USER.username);
    await unauthPage.fill('#password', 'wrongpassword1');
    await unauthPage.click('.auth-submit');

    await expect(unauthPage.locator('.auth-error')).toBeVisible();
    await expect(unauthPage).toHaveURL(/\/login/);
  });

  test('logout redirects to login and blocks access', async ({ authedPage }) => {
    // Verify we're logged in
    await authedPage.goto('/');
    await expect(authedPage.locator('.user-name')).toHaveText(TEST_USER.username);

    // Logout
    await authedPage.click('.logout-link');
    await authedPage.waitForURL(/\/login/);

    // Verify protected page redirects to login
    await authedPage.goto('/');
    await expect(authedPage).toHaveURL(/\/login/);
  });
});
