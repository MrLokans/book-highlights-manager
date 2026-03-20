import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';

test.describe('Vocabulary', () => {
  test('shows seeded vocabulary entries', async ({ authedPage }) => {
    await authedPage.goto('/vocabulary');
    await expect(authedPage.locator(Sel.pageTitle)).toContainText('Vocabulary');
    await expect(authedPage.locator(Sel.wordCard)).toHaveCount(SEED.vocabulary.length);
    for (const word of SEED.vocabulary) {
      await expect(authedPage.locator(Sel.wordText)).toContainText([word]);
    }
  });
});
