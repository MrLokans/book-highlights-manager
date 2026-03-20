import { test, expect } from '../helpers/fixtures';

test.describe('Vocabulary', () => {
  test('shows seeded vocabulary entries', async ({ authedPage }) => {
    await authedPage.goto('/vocabulary');
    await expect(authedPage.locator('.page-title')).toContainText('Vocabulary');
    await expect(authedPage.locator('.word-card')).toHaveCount(3);
    await expect(authedPage.locator('.word-text')).toContainText(['ephemeral']);
    await expect(authedPage.locator('.word-text')).toContainText(['ubiquitous']);
    await expect(authedPage.locator('.word-text')).toContainText(['serendipity']);
  });
});
