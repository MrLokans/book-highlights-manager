import { test, expect } from '../helpers/fixtures';

test.describe('Book Detail', () => {
  test('shows book metadata and highlights', async ({ authedPage }) => {
    await authedPage.goto('/');
    await authedPage.click('.book-card:has-text("The Art of Testing") .book-link');
    await authedPage.waitForURL(/\/ui\/books\/\d+/);
    await expect(authedPage.locator('h2')).toContainText('The Art of Testing');
    await expect(authedPage.locator('.author')).toContainText('Jane Smith');
    const highlightCount = await authedPage.locator('.highlight').count();
    expect(highlightCount).toBeGreaterThan(0);
  });

  test('navigating to non-existent book shows not found', async ({ authedPage }) => {
    await authedPage.goto('/ui/books/99999');
    await expect(authedPage.locator('text=Book not found')).toBeVisible();
  });
});
