import { test, expect } from '../helpers/fixtures';

test.describe('Search', () => {
  test('typing in search filters books via HTMX', async ({ authedPage }) => {
    await authedPage.goto('/');
    await expect(authedPage.locator('.book-card')).toHaveCount(5);
    await authedPage.fill('.search-box input[name="q"]', 'Art of Testing');
    await authedPage.waitForResponse(resp => resp.url().includes('/ui/books/search'));
    await expect(authedPage.locator('.book-card')).toHaveCount(1);
    await expect(authedPage.locator('.book-title')).toContainText('The Art of Testing');
  });

  test('clearing search shows all books', async ({ authedPage }) => {
    await authedPage.goto('/');
    await authedPage.fill('.search-box input[name="q"]', 'Art of Testing');
    await authedPage.waitForResponse(resp => resp.url().includes('/ui/books/search'));
    await expect(authedPage.locator('.book-card')).toHaveCount(1);
    await authedPage.fill('.search-box input[name="q"]', '');
    await authedPage.waitForResponse(resp => resp.url().includes('/ui/books/search'));
    await expect(authedPage.locator('.book-card')).toHaveCount(5);
  });
});
