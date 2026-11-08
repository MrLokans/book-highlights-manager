# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Connecting Dropbox from the web UI no longer logs you out on the way back: session and CSRF cookies are `SameSite=Lax`, and the login redirect keeps the full URL including the OAuth `code` and `state`.
- Disconnecting Dropbox deletes the stored tokens for good, so the same account can be connected again (previously failed with a unique-constraint error).
- Visiting `/logout` while already logged out no longer asks you to log in first.
- Disconnecting Dropbox also revokes the token at Dropbox, so the stored refresh token stops working.
- A Dropbox connect started before a restart (e.g. an automatic image update) still completes: the pending OAuth state is kept in a short-lived cookie instead of server memory.
- Login redirects keep `next` URLs whose query contains a URL, and an already-logged-in visit to `/login?next=…` goes to `next`.

### Security

- `X-Forwarded-For` is no longer trusted, so clients can't dodge the login rate limit by spoofing it. Behind a CDN, set `CLIENT_IP_HEADER` (e.g. `CF-Connecting-IP` for Cloudflare) to rate-limit and log by the real client IP.
- Stylesheet and script URLs carry the app version (`style.css?v=1.0.2`), so a CDN such as Cloudflare, which caches `/static` regardless of `Cache-Control`, serves fresh styles after every release.

## [1.0.1] - 2026-11-08

### Fixed

- `HEAD /health` returns 200, so the Docker `HEALTHCHECK` (`wget --spider`) no longer marks the container unhealthy.
- Static assets are served with `Cache-Control: no-cache`, so browsers pick up new styles after an upgrade instead of keeping a stale `style.css`.

## [1.0.0] - 2026-11-08

### Added

- Dedicated **Import** page (`/import`) for Kindle, Apple Books, Readwise CSV and connected sources (Readwise API, Moon+ via Dropbox).
- Import results show new highlights separately from ones already in the library, so a re-import no longer looks like duplicates.
- Books page sorting, pagination and filters; styled empty states and 404 page.
- Author extraction from many e-book filename patterns (fixes existing Moon+ authors once).

### Changed

- Redesigned web UI ("Highlighter on proof"); Settings and Profile rebuilt as ruled rows.
- Moon+ Dropbox imports from Settings go into the main library instead of a separate `moonreader.db`.
- Single-user model: all data belongs to the owner. Legacy `user_id` columns are dropped by a one-off migration that backs up the database first; case-duplicate tags are merged and tag names are now unique.
- Stricter Content Security Policy: no inline scripts, `unsafe-inline` or `eval`; htmx is vendored instead of loaded from unpkg.
- Go 1.27.3, Debian trixie base image, updated dependencies.
- README restructured for first-time readers; reference material moved to `docs/`.

### Fixed

- Re-import no longer wipes book metadata (cover, publisher, ISBN, date added).
- Books with more than ~2,000 highlights failed to save.
- Scheduled Readwise sync saved books invisible to the logged-in user.
- Deleted books were still exported to Markdown.
- XSS in the vocabulary word popover; HTMX result fragments are escaped.
- Broken templates, missing toasts, multiple-highlight rendering.

## [0.7.4] - 2026-02-01

### Changed

- App version shown in Settings only. Tooling and CI lint aligned with local lint.

## [0.7.3] - 2026-02-01

### Fixed

- CSP adjustments for reverse-proxy deployments.

## [0.7.2] - 2026-02-01

### Added

- App version display.

## [0.7.1] - 2026-01-31

### Fixed

- CSP behaviour behind a reverse proxy.

## [0.7.0] - 2026-01-31

### Added

- Readwise API sync.

### Fixed

- Audit events not showing.

## [0.6.4] – [0.6.10] - 2026-01

### Changed

- Reworked OAuth2 integration to make adding providers easier; demo link added.
- CSP adjustments for analytics; Docker build fixes for CI; missing audit events added.

## [0.6.3]

### Changed

- Unified markdown exports. Revamp periodic exports to markdown with UI.

## [0.6.2]

### Changed

- Docker build should now work properly with CGO_ features for sqlite driver to work properly.

## [0.6.1]

### Added

- Possibility to embed plausible analytics. Will be used to measure demo deployment success.

## [0.6.0]

### Added

- Authentication via the login and password on demand.
- Demo application with dummy data and RO-endpoints.

## [0.5.1]

Re-organize internal structure and design.

## [0.5.0]

### Added

- Vocabulary builder from you highlights: easy to manage, automatic fetching of definisitons.

## [0.4.0]

### Added

- Add Favourites management for your higlights

### Changed

- Internal task processing is now handled by a library

## [0.3.0]

### Added

- Kindle import support via `My Clippings.txt` file upload (relied upon clippings samples from https://github.com/biokraft/kindle2readwise/)
  - Parse Kindle clippings format with highlights, notes, and bookmarks
  - Web UI integration in Settings > Integrations

## [0.2.0]

### Added

- Tag management system for books and highlights
  - Books and highlights can now have tags (previously only highlights supported tags)
  - Tags have autocomplete and allow filtering the books
- Delete functionality with re-import prevention
  - Soft delete for books and highlights (recoverable)
  - Permanent delete that blocks future re-imports of the same content
- Tag display and inline management in book detail and list pages


## [0.1.1]

### Changed

- Prettify settings page styles
- Move gin router to the RELEASE mode

## [0.1.0]

### Added

  - Basic metadata sync (covers, ISBN-s, publisher, dates) via the openlibrary integration

## [0.0.1]

### Added

- Web interface for Apple Books import via SQLite file upload
  - New endpoint `POST /settings/applebooks/import` for uploading Apple Books databases
  - Secure file upload with size limits (50MB max per file)
  - SQLite file integrity validation
  - Table structure validation (required tables: `ZAEANNOTATION`, `ZBKLIBRARYASSET`)
  - Column existence validation for required columns
  - Help section in UI explaining how to find Apple Books databases on macOS
  - HTMX-powered form with loading indicators and result display
