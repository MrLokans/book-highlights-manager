import { test, expect } from '../helpers/fixtures';
import { join } from 'path';

const TEST_FILES = join(__dirname, '..', 'fixtures', 'test-files');

test.describe('Import', () => {
  test('Kindle clippings import adds books', async ({ authedPage }) => {
    await authedPage.goto('/settings');
    const fileInput = authedPage.locator('#kindle-clippings-file');
    await fileInput.setInputFiles(join(TEST_FILES, 'kindle-sample.txt'));
    await authedPage.click('button:has-text("Import from Kindle")');
    await authedPage.waitForSelector('#kindle-result-container:not(:empty)', { timeout: 10_000 });
    await authedPage.goto('/');
    const bookCount = await authedPage.locator('.book-card').count();
    expect(bookCount).toBeGreaterThan(5);
  });

  test('Readwise CSV import adds books', async ({ authedPage }) => {
    await authedPage.goto('/settings');
    const fileInput = authedPage.locator('#readwise-csv-file');
    await fileInput.setInputFiles(join(TEST_FILES, 'readwise-sample.csv'));
    await authedPage.click('button:has-text("Import CSV")');
    await authedPage.waitForSelector('#readwise-csv-result-container:not(:empty)', { timeout: 10_000 });
    await authedPage.goto('/');
    const bookCount = await authedPage.locator('.book-card').count();
    expect(bookCount).toBeGreaterThan(5);
  });
});
