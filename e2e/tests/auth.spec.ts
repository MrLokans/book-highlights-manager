import { test, expect } from '../helpers/fixtures';
import { TEST_USER } from '../helpers/auth';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';

test.describe('Authentication', () => {
  test('login with valid credentials redirects to books page', async ({ unauthPage }) => {
    await unauthPage.goto('/login');
    await unauthPage.fill(Sel.usernameInput, TEST_USER.username);
    await unauthPage.fill(Sel.passwordInput, TEST_USER.password);
    await unauthPage.click(Sel.authSubmit);

    await unauthPage.waitForURL('/');
    await expect(unauthPage.locator(Sel.bookCard)).toHaveCount(SEED.totalBooks);
  });

  test('login with invalid credentials shows error', async ({ unauthPage }) => {
    await unauthPage.goto('/login');
    await unauthPage.fill(Sel.usernameInput, TEST_USER.username);
    await unauthPage.fill(Sel.passwordInput, 'wrongpassword1');
    await unauthPage.click(Sel.authSubmit);

    await expect(unauthPage.locator(Sel.authError)).toBeVisible();
    await expect(unauthPage).toHaveURL(/\/login/);
  });

  test('logout redirects to login and blocks access', async ({ authedPage }) => {
    // Verify we're logged in
    await authedPage.goto('/');
    await expect(authedPage.locator(Sel.userName)).toHaveText(TEST_USER.username);

    // Logout
    await authedPage.click(Sel.logoutLink);
    await authedPage.waitForURL(/\/login/);

    // Verify protected page redirects to login
    await authedPage.goto('/');
    await expect(authedPage).toHaveURL(/\/login/);
  });
});
