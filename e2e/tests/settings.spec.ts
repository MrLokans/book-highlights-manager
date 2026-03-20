import { test, expect } from '../helpers/fixtures';
import { Sel } from '../helpers/selectors';

test.describe('Settings', () => {
  test('settings page loads with current values', async ({ authedPage }) => {
    await authedPage.goto('/settings');
    await expect(authedPage.locator('h2')).toBeVisible();
    await expect(authedPage.locator(Sel.kindleFileInput)).toBeVisible();
    await expect(authedPage.locator(Sel.readwiseCsvFileInput)).toBeVisible();
  });
});
