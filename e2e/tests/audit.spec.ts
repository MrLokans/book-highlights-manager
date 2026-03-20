import { test, expect } from '../helpers/fixtures';

test.describe('Audit', () => {
  test('audit page shows seeded events', async ({ authedPage }) => {
    await authedPage.goto('/audit');
    await expect(authedPage.locator('.page-title')).toContainText('Audit');
    await expect(authedPage.locator('.audit-table')).toBeVisible();
    await expect(authedPage.locator('.event-type-badge')).toHaveCount(1);
  });
});
