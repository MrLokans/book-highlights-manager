import { test, expect } from '../helpers/fixtures';
import { Sel } from '../helpers/selectors';

test.describe('Settings', () => {
  test('settings page loads with current values', async ({ authedPage }) => {
    await authedPage.goto('/settings');
    await expect(authedPage.locator(Sel.pageTitle)).toHaveText('Settings');
    await authedPage.goto('/import');
    await expect(authedPage.locator(Sel.pageTitle)).toHaveText('Import');
    await expect(authedPage.locator(Sel.kindleFileInput)).toBeVisible();
    await expect(authedPage.locator(Sel.readwiseCsvFileInput)).toBeVisible();
  });
});
