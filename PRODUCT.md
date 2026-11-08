# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Public self-hosters who install Highlights Manager from GitHub (Docker image) to own their reading highlights. They read e-books on Kindle, Apple Books, Moon+ Reader or through Readwise, and come to the web UI for two equal jobs:

- **Re-reading:** browse their library, open a book, read its highlights and notes, search across everything, favourite and tag passages, review vocabulary they looked up.
- **Managing:** import from devices and services, enrich book metadata, export/sync to Obsidian, look after settings, tasks and the audit log.

An instance may hold one reader or several (local auth with an admin account). A public read-only demo instance exists and is many people's first impression.

## Product Purpose

Pull highlights out of reading apps into one self-hosted library the user owns, keep them readable and searchable, and push them into the user's notes (Obsidian markdown). Success: a new self-hoster gets their first import in without reading docs, and a long-time user enjoys going back to their highlights, not only syncing them.

## Positioning

Self-hosted, single binary/Docker image, no cloud account: the user's highlights live in their own SQLite database and their own Obsidian vault. It merges several readers' formats into one library rather than being a reader or a SaaS.

## Operating Context

- Server-rendered Go (gin) HTML templates with HTMX for partial updates; no front-end build step. CSS lives in `static/`.
- Imports: Kindle `My Clippings.txt` upload, Moon+ Reader (Dropbox or file), Readwise (API token or CSV), Apple Books (CLI on macOS).
- Export: Obsidian markdown with YAML frontmatter, per-book download, bulk ZIP, scheduled vault sync.
- Metadata and covers come from OpenLibrary; many books have no cover.
- Background tasks (enrichment, cleanup) report progress through polled HTMX fragments.
- Auth modes: none (localhost) or local accounts; demo mode is read-only.

## Capabilities and Constraints

- Pages: Books (library), Book detail (highlights, tags, metadata), Favourites, Vocabulary, Settings (imports, sync, export, maintenance, analytics), Profile, Audit log, Login/Setup, 404.
- Libraries range from a handful of books to hundreds, with books holding thousands of highlights.
- Highlight text is user content in any language; book titles can be very long; covers are often missing.
- No inline `style=` attributes or per-template `<style>` blocks; styles live in shared stylesheets.

## Evidence on Hand

- Public demo with public-domain books: https://highlights-demo.mrlokans.work/
- Demo seed data generator: `cmd/generate_demo/`.
- No testimonials, usage numbers or logos exist; none may be invented.

## Product Principles

1. Reading and managing are equal citizens: neither page type is an afterthought.
2. A stranger must understand each screen without the README.
3. The user's words (highlights, notes) are the content; the interface frames them, never competes.
4. Honest states: progress, errors, and destructive actions say exactly what happens.
5. Self-hosted means it runs on modest hardware and works offline (no external font or script CDNs required).

## Accessibility & Inclusion

No product-specific standard set; target WCAG 2.2 AA as the baseline for a public app.
