import { test, expect } from '../helpers/fixtures';

test.describe('Favourites', () => {
  test('favourited highlights appear on favourites page', async ({ authedPage }) => {
    await authedPage.goto('/favourites');
    await expect(authedPage.locator('.page-title')).toContainText('Favourite');
    const favCount = await authedPage.locator('.favourite-highlight').count();
    expect(favCount).toBeGreaterThan(0);
  });

  test('toggling favourite on book detail removes it from favourites page', async ({ authedPage }) => {
    // Get initial favourites count
    await authedPage.goto('/favourites');
    const statsText = await authedPage.locator('.stats').textContent();
    const initialTotal = parseInt(statsText?.match(/(\d+)/)?.[1] ?? '0', 10);
    expect(initialTotal).toBeGreaterThan(0);

    // Navigate to a book with favourited highlights and unfavourite one
    await authedPage.goto('/');
    await authedPage.click('.book-card:has-text("The Art of Testing") .book-link');
    await authedPage.waitForURL(/\/ui\/books\/\d+/);
    const favBtn = authedPage.locator('.favourite-btn-active').first();
    await expect(favBtn).toBeVisible();
    const [response] = await Promise.all([
      authedPage.waitForResponse(resp => resp.url().includes('/favourite') && resp.request().method() === 'DELETE'),
      favBtn.click(),
    ]);
    expect(response.status()).toBeLessThan(400);

    // Verify favourites page has one fewer
    await authedPage.goto('/favourites');
    const newStatsText = await authedPage.locator('.stats').textContent();
    const newTotal = parseInt(newStatsText?.match(/(\d+)/)?.[1] ?? '0', 10);
    expect(newTotal).toBe(initialTotal - 1);
  });
});
