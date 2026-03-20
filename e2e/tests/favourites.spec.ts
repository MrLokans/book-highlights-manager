import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';
import { navigateToBook } from '../helpers/navigation';

test.describe('Favourites', () => {
  test('favourited highlights appear on favourites page', async ({ authedPage }) => {
    await authedPage.goto('/favourites');
    await expect(authedPage.locator(Sel.pageTitle)).toContainText('Favourite');
    const favCount = await authedPage.locator(Sel.favouriteHighlight).count();
    expect(favCount).toBeGreaterThan(0);
  });

  test('toggling favourite on book detail removes it from favourites page', async ({ authedPage }) => {
    // Get initial favourites count
    await authedPage.goto('/favourites');
    const statsText = await authedPage.locator(Sel.stats).textContent();
    const initialTotal = parseInt(statsText?.match(/(\d+)/)?.[1] ?? '0', 10);
    expect(initialTotal).toBeGreaterThan(0);

    // Navigate to a book with favourited highlights and unfavourite one
    await navigateToBook(authedPage, SEED.books.artOfTesting.title);
    const favBtn = authedPage.locator(Sel.favouriteBtnActive).first();
    await expect(favBtn).toBeVisible();
    const [response] = await Promise.all([
      authedPage.waitForResponse(resp => resp.url().includes('/favourite') && resp.request().method() === 'DELETE'),
      favBtn.click(),
    ]);
    expect(response.status()).toBeLessThan(400);

    // Verify favourites page has one fewer
    await authedPage.goto('/favourites');
    const newStatsText = await authedPage.locator(Sel.stats).textContent();
    const newTotal = parseInt(newStatsText?.match(/(\d+)/)?.[1] ?? '0', 10);
    expect(newTotal).toBe(initialTotal - 1);
  });
});
