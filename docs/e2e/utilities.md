# E2E Utilities Reference

## Custom Fixtures

Fixtures are defined in `e2e/helpers/fixtures.ts` and extend the standard Playwright `test` object. Import them instead of `@playwright/test`:

```ts
import { test, expect } from '../helpers/fixtures';
```

### `app`

**Type:** `AppInstance`
**Scope:** per-test

Starts the Go application before the test and stops it after. Under the hood this calls `startApp()` (see [App Lifecycle](#app-lifecycle) below). Every other fixture that needs a running server depends on `app`.

```ts
test('uses app directly', async ({ app }) => {
  const resp = await fetch(`${app.url}/health`);
  expect(resp.ok).toBe(true);
});
```

### `appUrl`

**Type:** `string`
**Scope:** per-test

Convenience shorthand for `app.url`. Use this when you only need the base URL and do not need to call `app.stop()` manually.

```ts
test('makes an API call', async ({ appUrl, authedPage }) => {
  const resp = await authedPage.request.get(`${appUrl}/api/books`);
  expect(resp.ok()).toBeTruthy();
});
```

### `authedPage`

**Type:** `Page`
**Scope:** per-test

A Playwright `Page` that is already authenticated as `testuser`. The fixture:

1. Creates a new browser context with `baseURL` set to `app.url`
2. Opens a new page in that context
3. Calls `login(page)` to complete the login flow
4. Yields the page to your test
5. Closes the context when the test ends

Use this fixture for any test that requires an authenticated session.

```ts
test('shows books', async ({ authedPage }) => {
  await authedPage.goto('/');
  await expect(authedPage.locator('.book-card')).toHaveCount(5);
});
```

### `unauthPage`

**Type:** `Page`
**Scope:** per-test

A Playwright `Page` with no session — the browser context has a fresh, empty cookie jar. Use this to test login flows, redirect behaviour for unauthenticated users, and public endpoints.

```ts
test('redirects to login', async ({ unauthPage }) => {
  await unauthPage.goto('/');
  await expect(unauthPage).toHaveURL(/\/login/);
});
```

---

## App Lifecycle

Defined in `e2e/helpers/app.ts`.

### `startApp(): Promise<AppInstance>`

Starts one isolated instance of the application. Returns an `AppInstance`:

```ts
interface AppInstance {
  url: string;       // Base URL, e.g. "http://127.0.0.1:54321"
  tmpDir: string;    // Temp directory holding the DB copy and vault (empty string for external servers)
  stop: () => Promise<void>;
}
```

What `startApp()` does in default (per-test) mode:

1. Picks a random free TCP port on `127.0.0.1`.
2. Copies `e2e/fixtures/seed.db` to `<tmpDir>/app.db`.
3. Creates `<tmpDir>/vault/` and `<tmpDir>/audit/` directories.
4. Spawns `e2e/bin/highlights-exporter` (or `E2E_BINARY_PATH`) with the required environment variables.
5. Polls `GET /health` at 100 ms intervals until the server responds (10 s timeout).

When `E2E_BASE_URL` is set, `startApp()` skips all of the above and returns immediately with a no-op `stop()`.

### `app.stop(): Promise<void>`

Gracefully shuts down the spawned process and cleans up:

1. Sends `SIGTERM` to the process.
2. Waits up to 5 seconds for the process to exit; sends `SIGKILL` if it does not.
3. Deletes the temp directory (`tmpDir`) with all its contents.

The `app` fixture calls `stop()` automatically in teardown. You do not need to call it manually unless you are managing `AppInstance` outside a fixture.

---

## Auth Helper

Defined in `e2e/helpers/auth.ts`.

### `TEST_USER`

The credentials pre-loaded into the seed database:

```ts
const TEST_USER = {
  username: 'testuser',
  email:    'test@test.com',
  password: 'testpassword12',
};
```

### `login(page, username?, password?): Promise<void>`

Performs a full browser login flow:

1. Navigates to `/login`
2. Fills `#username` and `#password`
3. Clicks `.auth-submit`
4. Waits for redirect to `/` (5 s timeout)

The `authedPage` fixture calls `login()` automatically. Call it directly only when you need to log in mid-test or with different credentials.

```ts
import { login, TEST_USER } from '../helpers/auth';

test('re-authenticates after logout', async ({ unauthPage }) => {
  await login(unauthPage);
  // unauthPage is now authenticated
});
```

---

## Seed Database

The seed database lives at `e2e/fixtures/seed.db`. It is committed to the repository so that tests can run without regenerating it.

### Contents

| Entity | Count | Details |
|---|---|---|
| Users | 1 | `testuser` / `testpassword12`, admin role |
| Sources | 1 | `readwise` |
| Books | 5 | See table below |
| Highlights | 29 | 22 in book 1, 1 in book 2, 0 in book 3, 5 in book 4, 1 in book 5 |
| Tags | 3 | `fiction`, `nonfiction`, `favourite` |
| Words | 3 | `ephemeral` (enriched), `ubiquitous` (pending), `serendipity` (enriched) |
| Audit events | 1 | Kindle import, success |
| Settings | 1 | `obsidian_vault_dir = /tmp/test-vault` |

**Books:**

| Title | Author | Highlights | Notes |
|---|---|---|---|
| The Art of Testing | Jane Smith | 22 | Has cover URL, ISBN, tags: fiction + favourite; first 2 highlights are favourites |
| Brief Thoughts | John Doe | 1 | Tag: nonfiction |
| Empty Reads | Alice Wonder | 0 | No highlights |
| Favourite Collection | Bob Builder | 5 | First 3 highlights are favourites |
| Anonymous Wisdom | _(no author)_ | 1 | |

### How to Regenerate

```
make seed-e2e
```

This runs `go run main.go` in `e2e/fixtures/generate-seed/`, which drops and recreates `e2e/fixtures/seed.db`. Commit the updated file if you change the seed data.

### How to Add Data

1. Open `e2e/fixtures/generate-seed/main.go`.
2. Add GORM `db.Create(...)` calls for the new entities.
3. Run `make seed-e2e` to regenerate the database.
4. Commit both `main.go` and the updated `seed.db`.
5. Update any tests that assert on exact counts (e.g., `toHaveCount(5)` for books).

---

## Debugging

### Headed mode

Run the browser visibly to watch what the test is doing:

```
make test-e2e-headed
# or
cd e2e && E2E_HEADED=true npx playwright test
```

### Debug mode (Playwright Inspector)

Opens the Playwright Inspector, which lets you step through actions, inspect the DOM, and generate selectors:

```
cd e2e && PWDEBUG=1 npx playwright test tests/auth.spec.ts
```

### Trace viewer

When a test fails, Playwright saves a trace to `e2e/test-results/`. Open it with:

```
cd e2e && npx playwright show-trace test-results/<test-name>/trace.zip
```

The trace viewer shows a timeline of actions, DOM snapshots at each step, network requests, and console logs.

### Screenshots on failure

Playwright is configured with `screenshot: 'only-on-failure'`. Failed-test screenshots are saved to `e2e/test-results/` alongside the trace.

### Inspecting the temp database

`startApp()` logs the temp directory path when the server fails to start. If you need to inspect the database mid-test, add a temporary `page.pause()` call (only works in headed or debug mode), then look in the OS temp directory for a directory matching the pattern `e2e-*`.
