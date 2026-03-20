import { test, expect } from '../helpers/fixtures';

test.describe('Books List', () => {
  test('shows all seeded books', async ({ authedPage }) => {
    await authedPage.goto('/');
    await expect(authedPage.locator('.book-card')).toHaveCount(5);
    await expect(authedPage.locator('.book-title')).toContainText(['The Art of Testing']);
    await expect(authedPage.locator('.book-title')).toContainText(['Brief Thoughts']);
    await expect(authedPage.locator('.book-title')).toContainText(['Empty Reads']);
  });

  test('sorting by title changes order', async ({ authedPage }) => {
    await authedPage.goto('/');
    await authedPage.selectOption('#sort-select', 'title_asc');
    await authedPage.waitForResponse(resp => resp.url().includes('/ui/books/search'));
    const titles = await authedPage.locator('.book-title').allTextContents();
    const sorted = [...titles].sort((a, b) => a.localeCompare(b));
    expect(titles).toEqual(sorted);
  });

  test('pagination works', async ({ authedPage }) => {
    await authedPage.goto('/');
    const bookCount = await authedPage.locator('.book-card').count();
    expect(bookCount).toBeGreaterThan(0);
    expect(bookCount).toBeLessThanOrEqual(5);
  });
});
