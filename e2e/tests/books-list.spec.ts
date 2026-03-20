import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';
import { waitForSearchResponse } from '../helpers/navigation';

test.describe('Books List', () => {
  test('shows all seeded books', async ({ authedPage }) => {
    await authedPage.goto('/');
    await expect(authedPage.locator(Sel.bookCard)).toHaveCount(SEED.totalBooks);
    await expect(authedPage.locator(Sel.bookTitle)).toContainText([SEED.books.artOfTesting.title]);
    await expect(authedPage.locator(Sel.bookTitle)).toContainText([SEED.books.briefThoughts.title]);
    await expect(authedPage.locator(Sel.bookTitle)).toContainText([SEED.books.emptyReads.title]);
  });

  test('sorting by title changes order', async ({ authedPage }) => {
    await authedPage.goto('/');
    await authedPage.selectOption(Sel.sortSelect, 'title_asc');
    await waitForSearchResponse(authedPage);
    const titles = await authedPage.locator(Sel.bookTitle).allTextContents();
    const sorted = [...titles].sort((a, b) => a.localeCompare(b));
    expect(titles).toEqual(sorted);
  });

  test('pagination works', async ({ authedPage }) => {
    await authedPage.goto('/');
    const bookCount = await authedPage.locator(Sel.bookCard).count();
    expect(bookCount).toBeGreaterThan(0);
    expect(bookCount).toBeLessThanOrEqual(SEED.totalBooks);
  });
});
