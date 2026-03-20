import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';
import { join } from 'path';

const TEST_FILES = join(__dirname, '..', 'fixtures', 'test-files');

test.describe('Import', () => {
  test('Kindle clippings import adds books', async ({ authedPage }) => {
    await authedPage.goto('/settings');
    const fileInput = authedPage.locator(Sel.kindleFileInput);
    await fileInput.setInputFiles(join(TEST_FILES, 'kindle-sample.txt'));
    await authedPage.click('button:has-text("Import from Kindle")');
    await authedPage.waitForSelector(Sel.kindleResultContainer, { timeout: 10_000 });
    await authedPage.goto('/');
    const bookCount = await authedPage.locator(Sel.bookCard).count();
    expect(bookCount).toBeGreaterThan(SEED.totalBooks);
  });

  test('Readwise CSV import adds books', async ({ authedPage }) => {
    await authedPage.goto('/settings');
    const fileInput = authedPage.locator(Sel.readwiseCsvFileInput);
    await fileInput.setInputFiles(join(TEST_FILES, 'readwise-sample.csv'));
    await authedPage.click('button:has-text("Import CSV")');
    await authedPage.waitForSelector(Sel.readwiseCsvResultContainer, { timeout: 10_000 });
    await authedPage.goto('/');
    const bookCount = await authedPage.locator(Sel.bookCard).count();
    expect(bookCount).toBeGreaterThan(SEED.totalBooks);
  });
});
