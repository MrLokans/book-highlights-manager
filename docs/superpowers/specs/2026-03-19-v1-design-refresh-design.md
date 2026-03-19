# v1.0.0 Design Refresh — Spec

## Overview

Prepare highlights-exporter for v1.0.0 by transforming it from a developer tool into a polished product. The target users are readers who love their highlights as a personal collection (browsing, rediscovering, favouriting) and Obsidian power users who want highlights flowing into their vault.

The work covers: a CSS design token system, a visual refresh ("Clean Modern" theme), bug fixes, empty states, sorting, and pagination.

Dark mode, theme engine UI, and discovery features (daily highlight, stats dashboard) are deferred to v1.1+.

---

## 1. Design Token System

### Goal

Every visual decision flows through CSS custom properties on `:root`. A future theme engine swaps them by applying a class on `<body>` (e.g., `body.theme-warm-minimal`). No CSS rewrite needed to re-skin the app.

### Token Coverage (Tier 1 + Tier 2)

**Tier 1 — Colors & Surfaces:**

| Token | Default (Clean Modern) | Purpose |
|-------|----------------------|---------|
| `--color-bg-page` | `#ffffff` | Page background |
| `--color-bg-card` | `#ffffff` | Card background |
| `--color-bg-input` | `#f8f8fa` | Input/search background |
| `--color-bg-hover` | `#f8f8fa` | Hover state background |
| `--color-bg-tag` | `#eef2ff` | Tag pill background |
| `--color-bg-banner` | `#fefce8` | Info/demo banner background |
| `--color-text-primary` | `#111111` | Headings, body text |
| `--color-text-secondary` | `#888888` | Author names, metadata |
| `--color-text-muted` | `#bbbbbb` | Placeholders, disabled text |
| `--color-text-link` | `#6366f1` | Links |
| `--color-text-tag` | `#6366f1` | Tag text |
| `--color-accent` | `#6366f1` | Buttons, active nav, CTA |
| `--color-accent-hover` | `#4f46e5` | Button hover |
| `--color-accent-light` | `#eef2ff` | Accent background tint |
| `--color-border` | `#f0f0f0` | Card and section borders |
| `--color-border-input` | `#eeeeee` | Input borders |
| `--color-border-active` | `#6366f1` | Focused/active borders |
| `--color-danger` | `#ef4444` | Delete actions, errors |
| `--color-success` | `#22c55e` | Success states |
| `--color-warning` | `#f59e0b` | Warnings |
| `--color-shadow` | `rgba(0,0,0,0.04)` | Card shadow at rest |
| `--color-shadow-hover` | `rgba(0,0,0,0.08)` | Card shadow on hover |
| `--radius-sm` | `4px` | Small elements (inputs) |
| `--radius-md` | `8px` | Buttons, search bar |
| `--radius-lg` | `12px` | Cards |
| `--radius-full` | `9999px` | Tag pills |

**Tier 2 — Typography:**

| Token | Default | Purpose |
|-------|---------|---------|
| `--font-family-body` | `system-ui, -apple-system, sans-serif` | Body text |
| `--font-family-heading` | `system-ui, -apple-system, sans-serif` | Headings |
| `--font-size-xs` | `11px` | Metadata, counts |
| `--font-size-sm` | `12px` | Secondary text |
| `--font-size-base` | `14px` | Body text |
| `--font-size-lg` | `16px` | Subheadings |
| `--font-size-xl` | `20px` | Page titles |
| `--font-size-2xl` | `24px` | Large headings |
| `--font-weight-normal` | `400` | Body |
| `--font-weight-medium` | `500` | Nav links |
| `--font-weight-semibold` | `600` | Card titles |
| `--font-weight-bold` | `700` | Page headings |
| `--line-height-tight` | `1.25` | Headings |
| `--line-height-normal` | `1.5` | Body text |

**Tier 2 — Spacing:**

| Token | Default | Purpose |
|-------|---------|---------|
| `--space-1` | `4px` | Tight gaps |
| `--space-2` | `8px` | Small gaps |
| `--space-3` | `12px` | Card internal padding |
| `--space-4` | `16px` | Standard padding |
| `--space-5` | `20px` | Section spacing |
| `--space-6` | `24px` | Large spacing |
| `--space-8` | `32px` | Section separation |
| `--space-10` | `40px` | Page-level spacing |
| `--space-12` | `48px` | Large page-level gaps |

### Implementation Approach

- Define all tokens in a single CSS file (e.g., `static/css/tokens.css`) loaded before component styles
- All component CSS references tokens exclusively — no raw color/size values
- Existing inline styles in templates must be migrated to token-referencing CSS classes
- Tag-specific tokens: `--color-bg-tag`, `--color-text-tag`, `--radius-full`, `--font-size-xs` — control pill shape, color, and size
- Border-specific tokens: `--color-border`, `--radius-lg` — control card appearance

**Token migration:** The existing `static/style.css` already defines CSS custom properties (`:root` with `--bg`, `--text`, `--accent`, etc.) plus a `prefers-color-scheme: dark` media query. The token migration is a one-time bulk rename from old names to new names — not incremental. All CSS references to `--bg`, `--text`, `--accent` etc. must update at once to the new `--color-*` naming convention.

**Dark mode regression:** The existing CSS has basic `prefers-color-scheme: dark` support. Implementing this spec as written will replace it. This is an **intentional regression** — the existing dark mode is minimal and untested. v1.1 will add a proper dark mode token set built on the new foundation. The existing dark values can be referenced when building the v1.1 dark theme.

---

## 2. Bug Fixes

### 2.1 Styled 404 Page

**Current:** Plain text "404 page not found" with no navigation or branding.

**Fix:** Create a 404 template that uses the standard app layout (header, nav, footer). Content: a centered message ("Page not found"), a brief subtext, and a "Back to books" link styled as primary CTA. Register as Gin's `NoRoute` handler.

### 2.2 Double-v Version String

**Current:** Settings footer shows "vv0.7.4" (double "v").

**Root cause:** `git describe --tags` returns `v0.7.4` (tag includes "v"), and `settings.html` line 50 prepends another "v": `<div class="settings-version">v{{ .Version }}</div>`.

**Fix:** Remove the hardcoded `v` prefix from the template in `settings.html`. The version string from `git describe` already includes it.

### 2.3 Settings Nav Link

**Current:** "Settings" disappears from the top nav when on the Settings page. Root cause: `base.html` line 109 wraps the Settings link in `{{ if not .Demo.Enabled }}`, hiding it entirely in demo mode.

**Fix:** Render the Settings link on all pages unconditionally (it's still navigable in demo mode — writes are blocked server-side). Apply active styling when on `/settings*` routes.

**Bonus:** The current codebase has four duplicate header templates (`header`, `header-favourites`, `header-vocabulary`, `header-settings`) differing only by which nav link is active. Consolidate into a single `header` template that accepts an `ActivePage` parameter. This eliminates ~80 lines of duplication and prevents future drift.

### 2.4 Dynamic Filter Counts

**Current:** "10 books · 57 highlights" stays static when search or tag filter is applied.

**Fix:** Wrap counts + sort dropdown + book list + pagination in a single HTMX swap target container (e.g., `<div id="books-content">`). All HTMX requests (search, tag filter, sort, pagination) swap this entire container. The handler returns the full fragment including updated counts, sort state, filtered book list, and pagination controls.

**Migration note:** Tag filter links currently use full-page navigation (`<a href="/?tag={{ .ID }}">`). These must be converted to HTMX requests (`hx-get`, `hx-target="#books-content"`, `hx-push-url="true"`) to compose with search/sort/pagination.

### 2.5 Demo Mode Button Visibility

**Current:** Delete and destructive action buttons are visible in demo mode (writes are blocked server-side, but buttons are confusing).

**Fix:** Template conditional: if demo mode is active, do not render delete buttons, ISBN edit fields, or other write-action UI elements. The demo banner already communicates the restriction.

### 2.6 Demo Banner Restyle

**Current:** Heavy full-width orange gradient, visually dominant.

**Fix:** Compact info bar at the very top. Subtle amber/yellow background (`--color-bg-banner`), small text, dismissable via "x" button. Similar to GitHub's "archived repository" banner. Should not dominate the viewport, especially on mobile.

---

## 3. Visual Polish

### 3.1 Header

- Remove the orange gradient header entirely (it was the demo banner bleeding into the design)
- Clean top bar: app name ("Highlights") left, nav links (Books, Favourites, Vocabulary, Settings) as horizontal links, user menu right
- Active nav link gets a bottom border in `--color-accent`
- Compact height — should not consume significant viewport space
- Bottom border: `1px solid var(--color-border)`

### 3.2 Book Cards

- Border: `1px solid var(--color-border)`
- Border radius: `var(--radius-lg)` (12px)
- Shadow at rest: `0 1px 3px var(--color-shadow)`
- Hover: elevated shadow `0 4px 12px var(--color-shadow-hover)`, subtle scale (`transform: scale(1.01)`), `cursor: pointer`
- Transition: `all 0.2s ease`
- Cover image: fixed 48x68px
- Download icon: always visible (muted), not hover-only

### 3.3 Cover Placeholders

- When a book has no cover image, display a generated placeholder
- Style: subtle linear gradient background with a centered book icon silhouette (inline SVG, white at 50% opacity)
- Color: deterministic from a simple string hash of the book title, modulo a palette of 8 predefined gradient pairs (e.g., indigo→violet, teal→cyan, amber→orange, etc.). Each book gets a unique-looking but stable color.
- Same 48x68px dimensions as real covers
- Implementation: Go template helper function `coverGradient(title string) string` returns a CSS gradient value. Template renders the SVG book icon inline.

### 3.4 Highlight Cards (Book Detail)

- Keep left accent border (it works well)
- Favourite heart icon: always visible, muted gray when not favourited, filled red when favourited
- Action icons (delete, tag): always visible in muted state, not hover-only
- Notes: slightly indented, `--color-text-secondary` color, italic or distinct font treatment
- Tag input row: compact, consistent with global tag pill styling

### 3.5 Tags

- Consistent pill styling everywhere: homepage cards, book detail, filter bar, favourites
- Low-contrast background: `var(--color-bg-tag)` at low opacity
- Text: `var(--color-text-tag)`, `var(--font-size-xs)`
- Border radius: `var(--radius-full)` (fully rounded)
- Filter bar tags: same pill style, with a clearly distinct "x" dismiss button (slightly larger, different hover state)

### 3.6 Footer

- Present on every page
- Content: version number (e.g., "v1.0.0"), optionally a link to project/docs
- Styling: `--color-text-muted`, small font, centered or left-aligned, top border `1px solid var(--color-border)`
- Provides visual page termination

### 3.7 HTMX Transitions

- Add `htmx-swapping` and `htmx-settling` CSS transitions for smooth content updates
- Apply to: search results, tag filter changes, pagination, sort changes
- Style: opacity fade (1 → 0 on swap-out, 0 → 1 on settle-in), 150ms duration
- Implementation: CSS transition on the `#books-content` swap target using HTMX's `htmx-swapping` and `htmx-settling` classes
- Scroll-to-top: pagination clicks should scroll to the top of the book list. Use `hx-swap="innerHTML show:top"` on the swap target

---

## 4. Empty States

Each empty state uses the design token system. All follow the pattern: icon/illustration → heading → subtext → CTA.

### 4.1 Books Page (Zero Books)

- Centered layout
- Simple book/stack icon (SVG, muted)
- Heading: "Your library is empty"
- Subtext: "Import highlights from Kindle, Apple Books, Readwise, or other sources to get started."
- Primary CTA button: "Import Highlights" → `/settings` (Integrations tab)

### 4.2 Book Detail (Zero Highlights)

- Defensive case (shouldn't normally occur)
- Text: "No highlights yet for this book."

### 4.3 Favourites Page (Zero Favourites)

- Heart/book icon
- Heading: "No favourites yet"
- Subtext: "Browse your books and tap the heart icon on highlights you love."
- Link: "Browse books" → `/`

### 4.4 Vocabulary Page (Zero Words)

- Dictionary/word icon
- Heading: "No vocabulary words yet"
- Subtext: "Words from your highlights will appear here. You can also add words manually."
- Link: "Browse books" → `/`

### 4.5 Search No Results

- Search icon
- Heading: "No books match your search"
- Subtext: "Try a different search term or clear your filters."
- Action link: "Clear search" → clears search input and reloads unfiltered list

---

## 5. Sorting

### UI

- Sort control on the same row as the book/highlight count, right-aligned
- Dropdown `<select>` element styled with design tokens
- Label: "Sort by:" followed by the dropdown
- Default selection: "Date added (newest)"

### Sort Options

| Display Label | Query Param Value | DB Column |
|--------------|-------------------|-----------|
| Date added (newest) | `date_desc` | `created_at DESC` |
| Date added (oldest) | `date_asc` | `created_at ASC` |
| Title (A→Z) | `title_asc` | `title ASC` |
| Title (Z→A) | `title_desc` | `title DESC` |
| Author (A→Z) | `author_asc` | `author ASC` |
| Author (Z→A) | `author_desc` | `author DESC` |
| Most highlights | `highlights_desc` | `highlight_count DESC` (computed) |
| Fewest highlights | `highlights_asc` | `highlight_count ASC` (computed) |

### Behavior

- Sort selection triggers HTMX request to reload book list
- URL updated: `?sort=title_asc&page=1`
- Changing sort always resets to page 1
- Sort composes with search and tag filters: `?q=war&tag=fiction&sort=title_asc&page=1`
- Sort preference persists in URL (shareable/bookmarkable)
- Search query also persists in URL via `hx-push-url="true"`: `?q=war&sort=title_asc&page=1` — makes search results bookmarkable/shareable

### Backend

- Add `sort` query parameter to the books list handler
- Parse sort param, default to `date_desc`
- Apply ORDER BY clause in the database query
- Sort param passed through to pagination link generation

**Interface change:** The current `BookReader` interface methods (`GetAllBooks()`, `SearchBooks()`) accept no sort/page parameters. Add a new `ListBooks(opts ListBooksOptions) ([]entities.Book, int64, error)` method that accepts:

```go
type ListBooksOptions struct {
    Query   string // search term (empty = no filter)
    TagID   uint   // tag filter (0 = no filter)
    Sort    string // sort key (e.g., "date_desc")
    Page    int    // 1-based page number
    PerPage int    // items per page (default 20)
}
```

Returns the book slice and total count (for pagination). Existing `GetAllBooks()` and `SearchBooks()` remain for non-UI callers (exporters, CLI). The UI handlers switch to `ListBooks()`.

**Sorting by highlight count:** Requires a subquery or JOIN with COUNT. Use GORM's `.Select("books.*, (SELECT COUNT(*) FROM highlights WHERE highlights.book_id = books.id AND highlights.deleted_at IS NULL) as highlight_count")` pattern.

---

## 6. Pagination

### UI

- Centered below the book list
- Pattern: `← Previous  1  2  3  ...  N  Next →`
- Current page: `--color-accent` background, white text
- Other pages: `--color-bg-input` background, clickable
- Previous: disabled (muted, no link) on page 1
- Next: disabled on last page
- Ellipsis for 7+ total pages: show first 2, ellipsis, last 2, and pages around current

### Behavior

- 20 books per page (not configurable in v1.0.0)
- URL param: `?page=2&sort=date_desc&q=&tag=`
- HTMX partial reload: only book list + pagination + count re-render
- Smooth transition via HTMX swap classes
- Count header shows **total** counts (not per-page): "57 books · 312 highlights"
- When filtered, count reflects filtered total: "3 books · 18 highlights"

### Edge Cases

- ≤20 books: no pagination controls rendered
- Invalid page number (e.g., `?page=999`): silently clamp to last valid page (no redirect — avoids flash/reload)
- Page param preserved when sorting or filtering changes (reset to page 1 on sort/filter change)

### Backend

- Pagination is handled by `ListBooks(opts)` (see Section 5, Backend) — `page` and `per_page` are fields on `ListBooksOptions`
- The UI uses `page` param (1-based). Internally converted to `offset = (page - 1) * perPage` for the GORM query. The existing `ParsePagination` helper in `helpers.go` uses `limit`/`offset` — the UI layer converts page-based params before calling it.
- `ListBooks` returns total count alongside results for pagination rendering
- Generate page links with all current query params preserved (sort, q, tag)

---

## 7. Out of Scope (v1.1+)

These are explicitly deferred. The token system is designed to make them easy to add later.

- **Dark mode** — token foundation is ready; needs a dark token set and a `prefers-color-scheme` media query or manual toggle
- **Theme engine** — settings UI for choosing between themes (Warm Minimal, Rich Literary, custom); backend stores theme preference per user
- **Discovery features** — daily highlight, "on this day", random quote, reading stats dashboard
- **Bulk operations** — multi-select books/highlights for export, delete, tag
- **Mobile hamburger nav** — responsive nav collapse for small screens
- **Onboarding wizard** — guided first-run import flow

---

## 8. Technical Constraints

- **Go templates + HTMX** — all UI is server-rendered Go `html/template` with HTMX for dynamic updates. No React/Vue/SPA.
- **No new JS dependencies** — use vanilla JS and CSS for all interactions. HTMX is the only JS library.
- **CSS migration** — existing `static/style.css` has old CSS custom properties (`--bg`, `--text`, `--accent`). The migration to the new token naming is a one-time bulk rename (see Section 1).
- **Database** — sorting and pagination require a new `ListBooks` method on the books repository (see Section 5). Existing GORM setup supports ORDER BY, LIMIT/OFFSET, COUNT.
- **Demo mode** — template conditionals already exist for demo mode; extend them to hide write-action buttons.
- **Sorting scope** — sorting and pagination apply to the Books page only in v1.0.0. Favourites and Vocabulary pages keep their current ordering.
- **Accessibility** — sort `<select>` and pagination links must have proper `aria-label` attributes. HTMX swap targets should manage focus appropriately after content updates.
