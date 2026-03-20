import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';
import { navigateToBook, waitForSearchResponse } from '../helpers/navigation';

test.describe('Tags', () => {
  test('book tags are visible on book detail page', async ({ authedPage }) => {
    await navigateToBook(authedPage, SEED.books.artOfTesting.title);
    await expect(authedPage.locator(Sel.bookTagsSection)).toBeVisible();
  });

  test('tag filter on books page filters by tag', async ({ authedPage }) => {
    await authedPage.goto('/');
    await expect(authedPage.locator(Sel.bookCard)).toHaveCount(SEED.totalBooks);
    const tagChip = authedPage.locator(Sel.tagFilterChip, { hasText: /^fiction$/ });
    await expect(tagChip).toBeVisible();
    await tagChip.click();
    await waitForSearchResponse(authedPage);
    const filteredCount = await authedPage.locator(Sel.bookCard).count();
    expect(filteredCount).toBeLessThan(SEED.totalBooks);
    expect(filteredCount).toBeGreaterThan(0);
  });
});
