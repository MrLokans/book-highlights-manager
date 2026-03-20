import { Page, Response } from '@playwright/test';
import { Sel } from './selectors';

/** Navigate from the books list to a specific book's detail page. */
export async function navigateToBook(page: Page, bookTitle: string): Promise<void> {
  await page.goto('/');
  await page.click(`${Sel.bookCard}:has-text("${bookTitle}") ${Sel.bookLink}`);
  await page.waitForURL(/\/ui\/books\/\d+/);
}

/** Wait for the HTMX search response to complete. */
export async function waitForSearchResponse(page: Page): Promise<Response> {
  return page.waitForResponse(resp => resp.url().includes('/ui/books/search'));
}
