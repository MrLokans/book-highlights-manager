import { test, expect } from '../helpers/fixtures';

test.describe('Export & Download', () => {
  test('single book download returns markdown', async ({ authedPage }) => {
    await authedPage.goto('/');
    await authedPage.click('.book-card:has-text("The Art of Testing") .book-link');
    await authedPage.waitForURL(/\/ui\/books\/\d+/);
    const [download] = await Promise.all([
      authedPage.waitForEvent('download'),
      authedPage.click('.book-actions .download-btn'),
    ]);
    const filename = download.suggestedFilename();
    expect(filename).toMatch(/\.md$/);
    const path = await download.path();
    if (path) {
      const fs = await import('fs');
      const content = fs.readFileSync(path, 'utf-8');
      expect(content).toContain('The Art of Testing');
      expect(content).toContain('---');
    }
  });

  test('download all returns ZIP file', async ({ authedPage }) => {
    await authedPage.goto('/');
    const [download] = await Promise.all([
      authedPage.waitForEvent('download'),
      authedPage.click('.download-all-btn'),
    ]);
    const filename = download.suggestedFilename();
    expect(filename).toMatch(/\.zip$/);
    expect(filename).toContain('highlights');
  });
});
