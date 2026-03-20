# E2E Tests

## Approach Overview

The E2E suite uses **Playwright** (Node.js) to drive a real Chromium browser against a real running instance of the application.

Key design decisions:

- **Per-test process isolation.** Each test starts its own Go binary in a temporary directory and stops it when the test ends. Tests never share application state or a running server.
- **SQLite copy strategy.** A pre-built seed database (`e2e/fixtures/seed.db`) is copied into a fresh temp directory for each test. The copy takes milliseconds and guarantees every test sees identical, deterministic starting data.
- **Playwright Node.js.** The Playwright test runner handles browser lifecycle, retries, screenshots on failure, and trace capture. Tests are written in TypeScript and run against Chromium.
- **Single worker.** Tests run sequentially (`workers: 1`). Because each test owns its own process and port, parallelism would be possible, but sequential execution keeps output readable and avoids port exhaustion.

The full suite (21 tests across 11 spec files) completes in approximately 15 seconds.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| **Go** | Used to build the application binary and the seed generator |
| **Node.js** | Required to run Playwright (`npm ci` installs dependencies) |
| **Chromium** | Installed automatically by `npx playwright install chromium` |

The `make test-e2e` target handles all of the above in order.

---

## How to Run

### Full suite (recommended)

```
make test-e2e
```

This target:
1. Regenerates `e2e/fixtures/seed.db` via `make seed-e2e`
2. Builds the Go binary at `e2e/bin/highlights-exporter`
3. Runs `npm ci` in the `e2e/` directory
4. Installs the Chromium browser if it is missing
5. Executes all specs with `npx playwright test`

### Headed mode (watch the browser)

```
make test-e2e-headed
```

Or from inside the `e2e/` directory after the binary and seed DB exist:

```
E2E_HEADED=true npx playwright test
```

### Single spec file

```
cd e2e && npx playwright test tests/books-list.spec.ts
```

### Debug mode (Playwright inspector)

```
cd e2e && PWDEBUG=1 npx playwright test tests/auth.spec.ts
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `E2E_BASE_URL` | _(unset)_ | When set, skips process management entirely and points all tests at this URL. Useful for testing against a running dev server or staging environment. |
| `E2E_BINARY_PATH` | `e2e/bin/highlights-exporter` | Path to the compiled Go binary. Override when the binary lives elsewhere. |
| `E2E_HEADED` | `false` | Set to `true` to run Chromium in headed (visible) mode. |

---

## Server Modes

### Per-test (default)

When `E2E_BASE_URL` is not set, `startApp()` in `helpers/app.ts`:

1. Picks a random free TCP port on `127.0.0.1`
2. Copies `e2e/fixtures/seed.db` into a new temp directory
3. Creates empty `vault/` and `audit/` subdirectories
4. Spawns the Go binary with all required environment variables pointing at the temp directory
5. Polls `/health` until the server responds (10 s timeout)
6. Returns an `AppInstance` containing the base URL and a `stop()` function

After the test finishes, the fixture teardown calls `app.stop()`, which sends `SIGTERM` to the process (with a 5 s SIGKILL fallback) and deletes the temp directory.

### External server

Set `E2E_BASE_URL=http://localhost:8080` (or any reachable URL). `startApp()` returns immediately with a no-op `stop()`. No binary is spawned and no temp directory is created.

---

## Test Structure

```
e2e/
├── bin/
│   └── highlights-exporter          # Compiled Go binary (git-ignored)
├── fixtures/
│   ├── seed.db                      # Pre-built SQLite seed database
│   ├── generate-seed/
│   │   └── main.go                  # Go program that builds seed.db
│   └── test-files/
│       ├── kindle-sample.txt        # Sample Kindle clippings file
│       └── readwise-sample.csv      # Sample Readwise CSV export
├── helpers/
│   ├── app.ts                       # startApp() / AppInstance lifecycle
│   ├── auth.ts                      # login() helper and TEST_USER constants
│   └── fixtures.ts                  # Playwright fixture extensions (test, expect)
├── tests/
│   ├── auth.spec.ts                 # Login, logout, invalid credentials
│   ├── books-list.spec.ts           # Books page, sorting, pagination
│   ├── book-detail.spec.ts          # Individual book page, highlights
│   ├── search.spec.ts               # Full-text search
│   ├── favourites.spec.ts           # Favourite highlights
│   ├── tags.spec.ts                 # Tag management
│   ├── vocabulary.spec.ts           # Vocabulary / word list
│   ├── import.spec.ts               # Kindle and Readwise import flows
│   ├── export-download.spec.ts      # Markdown export / download
│   ├── audit.spec.ts                # Audit log page
│   └── settings.spec.ts             # Settings page
├── package.json
├── playwright.config.ts
└── tsconfig.json
```

---

## Adding a New Test

1. **Create a spec file** in `e2e/tests/` following the naming pattern `<feature>.spec.ts`.

2. **Import the custom fixtures** instead of `@playwright/test`:
   ```ts
   import { test, expect } from '../helpers/fixtures';
   ```

3. **Choose the right page fixture** for your test:
   - `authedPage` — a browser page that is already logged in as `testuser`
   - `unauthPage` — a browser page with no session

4. **Write your test**:
   ```ts
   test.describe('My Feature', () => {
     test('does something useful', async ({ authedPage }) => {
       await authedPage.goto('/my-page');
       await expect(authedPage.locator('.my-element')).toBeVisible();
     });
   });
   ```

5. **Use `appUrl` when you need the base URL directly** (for example, when constructing API request URLs):
   ```ts
   test('calls the API', async ({ appUrl, authedPage }) => {
     const resp = await authedPage.request.get(`${appUrl}/api/books`);
     expect(resp.ok()).toBeTruthy();
   });
   ```

6. **Run your spec** to verify it passes:
   ```
   cd e2e && npx playwright test tests/my-feature.spec.ts
   ```

7. If the test needs data that the seed database does not contain, add rows in `e2e/fixtures/generate-seed/main.go` and regenerate with `make seed-e2e`.
