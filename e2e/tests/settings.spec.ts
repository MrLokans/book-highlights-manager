import { test, expect } from '../helpers/fixtures';

test.describe('Settings', () => {
  test('settings page loads with current values', async ({ authedPage }) => {
    await authedPage.goto('/settings');
    await expect(authedPage.locator('h2')).toBeVisible();
    await expect(authedPage.locator('#kindle-clippings-file')).toBeVisible();
    await expect(authedPage.locator('#readwise-csv-file')).toBeVisible();
  });
});
