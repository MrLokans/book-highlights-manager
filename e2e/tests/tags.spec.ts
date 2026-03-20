import { test, expect } from '../helpers/fixtures';

test.describe('Tags', () => {
  test('book tags are visible on book detail page', async ({ authedPage }) => {
    await authedPage.goto('/');
    await authedPage.click('.book-card:has-text("The Art of Testing") .book-link');
    await authedPage.waitForURL(/\/ui\/books\/\d+/);
    await expect(authedPage.locator('#book-tags-section')).toBeVisible();
  });

  test('tag filter on books page filters by tag', async ({ authedPage }) => {
    await authedPage.goto('/');
    await expect(authedPage.locator('.book-card')).toHaveCount(5);
    const tagChip = authedPage.locator('.tag-filter-chip', { hasText: /^fiction$/ });
    await expect(tagChip).toBeVisible();
    await tagChip.click();
    await authedPage.waitForResponse(resp => resp.url().includes('/ui/books/search'));
    const filteredCount = await authedPage.locator('.book-card').count();
    expect(filteredCount).toBeLessThan(5);
    expect(filteredCount).toBeGreaterThan(0);
  });
});
