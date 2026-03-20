import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';
import { navigateToBook } from '../helpers/navigation';

const { artOfTesting, briefThoughts, favouriteCollection } = SEED.books;

test.describe('Book Detail', () => {
  test('shows book metadata and highlights', async ({ authedPage }) => {
    await navigateToBook(authedPage, artOfTesting.title);
    await expect(authedPage.locator('h2')).toContainText(artOfTesting.title);
    await expect(authedPage.locator(Sel.author)).toContainText(artOfTesting.author);
    const highlightCount = await authedPage.locator(Sel.highlight).count();
    expect(highlightCount).toBeGreaterThan(0);
  });

  // Regression: GetBookByID was returning only 1 highlight per book (assistant-kzo)
  test('shows all highlights for a book with many highlights', async ({ authedPage }) => {
    await navigateToBook(authedPage, artOfTesting.title);

    const highlights = authedPage.locator(Sel.highlight);
    await expect(highlights).toHaveCount(artOfTesting.highlights);

    await expect(authedPage.locator(`text=${artOfTesting.highlights} highlights`)).toBeVisible();
  });

  test('shows all highlights for a book with few highlights', async ({ authedPage }) => {
    await navigateToBook(authedPage, favouriteCollection.title);

    const highlights = authedPage.locator(Sel.highlight);
    await expect(highlights).toHaveCount(favouriteCollection.highlights);
  });

  test('shows single highlight for a book with one highlight', async ({ authedPage }) => {
    await navigateToBook(authedPage, briefThoughts.title);

    const highlights = authedPage.locator(Sel.highlight);
    await expect(highlights).toHaveCount(briefThoughts.highlights);
    await expect(highlights.first()).toContainText(briefThoughts.singleHighlightText);
  });

  test('navigating to non-existent book shows not found', async ({ authedPage }) => {
    await authedPage.goto('/ui/books/99999');
    await expect(authedPage.locator('text=Book not found')).toBeVisible();
  });
});
