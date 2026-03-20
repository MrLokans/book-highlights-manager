import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';
import { waitForSearchResponse } from '../helpers/navigation';

test.describe('Search', () => {
  test('typing in search filters books via HTMX', async ({ authedPage }) => {
    await authedPage.goto('/');
    await expect(authedPage.locator(Sel.bookCard)).toHaveCount(SEED.totalBooks);
    await authedPage.fill(Sel.searchInput, 'Art of Testing');
    await waitForSearchResponse(authedPage);
    await expect(authedPage.locator(Sel.bookCard)).toHaveCount(1);
    await expect(authedPage.locator(Sel.bookTitle)).toContainText(SEED.books.artOfTesting.title);
  });

  test('clearing search shows all books', async ({ authedPage }) => {
    await authedPage.goto('/');
    await authedPage.fill(Sel.searchInput, 'Art of Testing');
    await waitForSearchResponse(authedPage);
    await expect(authedPage.locator(Sel.bookCard)).toHaveCount(1);
    await authedPage.fill(Sel.searchInput, '');
    await waitForSearchResponse(authedPage);
    await expect(authedPage.locator(Sel.bookCard)).toHaveCount(SEED.totalBooks);
  });
});
