import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';

test.describe('Audit', () => {
  test('audit page shows seeded events', async ({ authedPage }) => {
    await authedPage.goto('/audit');
    await expect(authedPage.locator(Sel.pageTitle)).toContainText('Audit');
    await expect(authedPage.locator(Sel.auditTable)).toBeVisible();
    await expect(authedPage.locator(Sel.eventTypeBadge)).toHaveCount(SEED.auditEvents);
  });
});
