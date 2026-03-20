import { test, expect } from '../helpers/fixtures';
import { SEED } from '../helpers/seed-data';
import { Sel } from '../helpers/selectors';
import { navigateToBook } from '../helpers/navigation';

test.describe('Export & Download', () => {
  test('single book download returns markdown', async ({ authedPage }) => {
    await navigateToBook(authedPage, SEED.books.artOfTesting.title);
    const [download] = await Promise.all([
      authedPage.waitForEvent('download'),
      authedPage.click(Sel.downloadBtn),
    ]);
    const filename = download.suggestedFilename();
    expect(filename).toMatch(/\.md$/);
    const path = await download.path();
    if (path) {
      const fs = await import('fs');
      const content = fs.readFileSync(path, 'utf-8');
      expect(content).toContain(SEED.books.artOfTesting.title);
      expect(content).toContain('---');
    }
  });

  test('download all returns ZIP file', async ({ authedPage }) => {
    await authedPage.goto('/');
    const [download] = await Promise.all([
      authedPage.waitForEvent('download'),
      authedPage.click(Sel.downloadAllBtn),
    ]);
    const filename = download.suggestedFilename();
    expect(filename).toMatch(/\.zip$/);
    expect(filename).toContain('highlights');
  });
});
