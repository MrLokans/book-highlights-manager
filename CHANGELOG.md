# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
