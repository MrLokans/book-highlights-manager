# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
