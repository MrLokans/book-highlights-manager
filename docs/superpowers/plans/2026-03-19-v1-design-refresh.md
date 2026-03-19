# v1.0.0 Design Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform highlights-exporter from a developer tool into a polished product with a design token system, visual refresh, bug fixes, empty states, sorting, and pagination.

**Architecture:** Server-rendered Go templates (html/template) with HTMX for dynamic updates. CSS custom properties provide the theming foundation. A new `ListBooks` repository method handles sorting/pagination/filtering. All interactive updates (search, sort, filter, paginate) use HTMX partial swaps within a single `#books-content` container.

**Tech Stack:** Go, Gin, GORM, HTMX v2.0.4, vanilla CSS/JS

**Spec:** `docs/superpowers/specs/2026-03-19-v1-design-refresh-design.md`

---

## File Structure

### New Files
- `static/tokens.css` — Design token definitions (CSS custom properties on `:root`)
- `templates/404.html` — Styled 404 page
- `templates/pagination.html` — Pagination controls partial
- `templates/empty-states.html` — Empty state partials for books, favourites, vocabulary, search
- `internal/database/books/list.go` — `ListBooksOptions` struct and `ListBooks()` method
- `internal/database/books/list_test.go` — Tests for `ListBooks()`
- `internal/http/ui_test.go` — Tests for updated UI handlers (if not existing)

### Modified Files
- `static/style.css` — Migrate from old `--bg`/`--text`/`--accent` vars to new `--color-*` token references, remove dark mode media query, update all component styles
- `templates/base.html` — Consolidate 4 header templates into 1 with `ActivePage` param, populate footer, fix Settings nav link
- `templates/books.html` — Wrap content in `#books-content` container, add sort dropdown, add pagination include, convert tag filters to HTMX, add empty states, update book card markup
- `templates/book.html` — Always-visible action icons, demo mode conditionals for write buttons
- `templates/favourites.html` — Add empty state
- `templates/vocabulary.html` — Add empty state
- `templates/settings.html` — Fix version string (remove hardcoded "v" prefix)
- `templates/demo-banner.html` — Restyle to compact amber info bar
- `internal/http/ui.go` — Update `BooksPage` and `SearchBooks` to use `ListBooks`, add sort/page params, return counts
- `internal/http/router.go` — Add `NoRoute` 404 handler, add `coverGradient` template func
- `internal/http/helpers.go` — Add page-to-offset conversion helper
- `internal/database/books/repository.go` — Minor: ensure existing methods still work alongside new `ListBooks`

---

## Task 1: Design Token System

**Files:**
- Create: `static/tokens.css`
- Modify: `static/style.css`
- Modify: `templates/base.html:1-12` (add tokens.css link)

- [ ] **Step 1: Create `static/tokens.css` with all token definitions**

```css
:root {
  /* Colors — Surfaces */
  --color-bg-page: #ffffff;
  --color-bg-card: #ffffff;
  --color-bg-input: #f8f8fa;
  --color-bg-hover: #f8f8fa;
  --color-bg-tag: #eef2ff;
  --color-bg-banner: #fefce8;

  /* Colors — Text */
  --color-text-primary: #111111;
  --color-text-secondary: #888888;
  --color-text-muted: #bbbbbb;
  --color-text-link: #6366f1;
  --color-text-tag: #6366f1;

  /* Colors — Accent */
  --color-accent: #6366f1;
  --color-accent-hover: #4f46e5;
  --color-accent-light: #eef2ff;

  /* Colors — Borders */
  --color-border: #f0f0f0;
  --color-border-input: #eeeeee;
  --color-border-active: #6366f1;

  /* Colors — Semantic */
  --color-danger: #ef4444;
  --color-success: #22c55e;
  --color-warning: #f59e0b;

  /* Colors — Shadows */
  --color-shadow: rgba(0, 0, 0, 0.04);
  --color-shadow-hover: rgba(0, 0, 0, 0.08);

  /* Border Radius */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;
  --radius-full: 9999px;

  /* Typography — Families */
  --font-family-body: system-ui, -apple-system, sans-serif;
  --font-family-heading: system-ui, -apple-system, sans-serif;

  /* Typography — Sizes */
  --font-size-xs: 11px;
  --font-size-sm: 12px;
  --font-size-base: 14px;
  --font-size-lg: 16px;
  --font-size-xl: 20px;
  --font-size-2xl: 24px;

  /* Typography — Weights */
  --font-weight-normal: 400;
  --font-weight-medium: 500;
  --font-weight-semibold: 600;
  --font-weight-bold: 700;

  /* Typography — Line Heights */
  --line-height-tight: 1.25;
  --line-height-normal: 1.5;

  /* Spacing */
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 20px;
  --space-6: 24px;
  --space-8: 32px;
  --space-10: 40px;
  --space-12: 48px;
}
```

- [ ] **Step 2: Add tokens.css link to `templates/base.html`**

In the `base-head` template, add before the existing style.css link:

```html
<link rel="stylesheet" href="/static/tokens.css">
```

- [ ] **Step 3: Migrate `static/style.css` — replace old custom properties**

Remove the existing `:root` block (lines 1-10) and the `@media (prefers-color-scheme: dark)` block (lines 12-23). Then find-and-replace all old variable references throughout the file:

| Old | New |
|-----|-----|
| `var(--bg)` | `var(--color-bg-page)` |
| `var(--bg-card)` | `var(--color-bg-card)` |
| `var(--text)` | `var(--color-text-primary)` |
| `var(--text-muted)` | `var(--color-text-muted)` |
| `var(--accent)` | `var(--color-accent)` |
| `var(--border)` | `var(--color-border)` |
| `var(--highlight-bg)` | `var(--color-bg-card)` |
| `var(--highlight-border)` | `var(--color-accent)` |

Also update hardcoded colors in component classes to use tokens where appropriate (borders, backgrounds, text colors, shadows, border-radii).

- [ ] **Step 4: Verify the app still renders correctly**

Run: `make run`

Open http://localhost:8080 and visually confirm:
- Pages load without CSS errors
- Colors are correct (indigo accent, white backgrounds)
- No broken layouts

- [ ] **Step 5: Commit**

```bash
git add static/tokens.css static/style.css templates/base.html
git commit -m "feat: add CSS design token system

Introduce tokens.css with all design tokens (colors, typography,
spacing, borders). Migrate style.css from old custom properties
to new token references. Remove basic dark mode (intentional
regression — proper dark mode comes in v1.1)."
```

---

## Task 2: Header Consolidation

**Files:**
- Modify: `templates/base.html:14-112` (replace 4 headers with 1)
- Modify: `internal/http/helpers.go` (add ActivePage to RenderPage)
- Modify: `internal/http/ui.go` (pass ActivePage)
- Modify: `internal/http/settings.go` (pass ActivePage)
- Modify: `templates/favourites.html` (use new header)
- Modify: `templates/vocabulary.html` (use new header)

- [ ] **Step 1: Replace four header templates with one in `templates/base.html`**

Remove `header`, `header-favourites`, `header-vocabulary`, `header-settings` templates (lines 14-112). Replace with a single `header` template:

```html
{{ define "header" }}
<header class="header">
  <div class="header-content">
    <a href="/" class="logo">Highlights</a>
    <nav class="nav-links">
      <a href="/" {{ if eq .ActivePage "books" }}class="active"{{ end }}>Books</a>
      <a href="/favourites" {{ if eq .ActivePage "favourites" }}class="active"{{ end }}>Favourites</a>
      <a href="/vocabulary" {{ if eq .ActivePage "vocabulary" }}class="active"{{ end }}>Vocabulary</a>
      <a href="/settings" {{ if eq .ActivePage "settings" }}class="active"{{ end }}>Settings</a>
    </nav>
    {{ if .Auth.Authenticated }}
    <div class="user-menu">
      <a href="/profile">{{ .Auth.Username }}</a>
      <form method="POST" action="/logout" style="display:inline">
        <input type="hidden" name="_csrf" value="{{ .Auth.CSRFToken }}">
        <button type="submit" class="btn-link">Logout</button>
      </form>
    </div>
    {{ end }}
  </div>
  {{ template "demo-banner" . }}
</header>
{{ end }}
```

- [ ] **Step 2: Add `ActivePage` to `RenderPage` in `internal/http/helpers.go`**

In `RenderPage()`, after the existing data injections (Auth, Demo, Analytics, Version), add:

```go
if _, exists := data["ActivePage"]; !exists {
    data["ActivePage"] = ""
}
```

- [ ] **Step 3: Pass `ActivePage` from all UI handlers**

Update each handler's `RenderPage` call to include `"ActivePage"`:

- `ui.go` `BooksPage`: add `"ActivePage": "books"`
- `ui.go` `BookPage`: add `"ActivePage": "books"`
- `ui.go` (favourites handler, wherever it is): add `"ActivePage": "favourites"`
- `ui.go` (vocabulary handler): add `"ActivePage": "vocabulary"`
- `settings.go` `SettingsPage`: add `"ActivePage": "settings"`

- [ ] **Step 4: Remove `{{ template "header-favourites" . }}` etc. from page templates**

In `favourites.html`, `vocabulary.html`, and `settings.html`, replace any specific header template calls with `{{ template "header" . }}`. The books page already uses `{{ template "header" . }}`.

- [ ] **Step 5: Verify all pages show correct active nav link**

Run: `make run`

Check each page: Books (active), Favourites (active), Vocabulary (active), Settings (active). Confirm Settings link is visible in demo mode.

- [ ] **Step 6: Commit**

```bash
git add templates/base.html templates/books.html templates/book.html templates/favourites.html templates/vocabulary.html templates/settings.html internal/http/helpers.go internal/http/ui.go internal/http/settings.go
git commit -m "refactor: consolidate four header templates into one

Use ActivePage parameter to highlight the current nav link.
Settings link now renders unconditionally (visible in demo mode).
Eliminates ~80 lines of duplicated markup."
```

---

## Task 3: Bug Fixes

**Files:**
- Create: `templates/404.html`
- Modify: `internal/http/router.go` (add NoRoute handler)
- Modify: `templates/settings.html:50` (fix version string)
- Modify: `templates/demo-banner.html` (restyle)
- Modify: `templates/book.html` (demo mode conditionals)
- Modify: `templates/books.html` (demo mode conditionals)

- [ ] **Step 1: Create styled 404 page `templates/404.html`**

```html
{{ define "404" }}
<!DOCTYPE html>
<html lang="en">
<head>
  {{ template "base-head" . }}
  <title>Page Not Found — Highlights</title>
</head>
<body>
  {{ template "header" . }}
  <div class="container">
    <div class="empty-state">
      <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-muted)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
        <line x1="8" y1="11" x2="14" y2="11"/>
      </svg>
      <h2>Page not found</h2>
      <p class="empty-state-text">The page you're looking for doesn't exist or has been moved.</p>
      <a href="/" class="btn btn-primary">Back to books</a>
    </div>
  </div>
  {{ template "footer" . }}
  {{ template "scripts-common" . }}
</body>
</html>
{{ end }}
```

- [ ] **Step 2: Register 404 handler in `internal/http/router.go`**

In `NewRouter()`, after all routes are registered, add:

```go
router.NoRoute(func(c *gin.Context) {
    RenderPage(c, http.StatusNotFound, "404", gin.H{
        "ActivePage": "",
    })
})
```

- [ ] **Step 3: Fix version string in `templates/settings.html`**

Change line 50 from:
```html
<div class="settings-version">v{{ .Version }}</div>
```
to:
```html
<div class="settings-version">{{ .Version }}</div>
```

The version string from `git describe --tags` already includes the "v" prefix (e.g., `v0.7.4`). The dev default "dev" displays as-is.

- [ ] **Step 4: Restyle demo banner in `templates/demo-banner.html`**

Replace the banner markup with a compact amber info bar:

```html
{{ define "demo-banner" }}
{{ if .Demo.Enabled }}
<div class="demo-banner" id="demo-banner">
  <span class="demo-banner-text">Demo Mode — Write operations are disabled.</span>
  <button class="demo-banner-close" onclick="dismissDemoBanner()" aria-label="Dismiss">&times;</button>
</div>
{{ end }}
{{ end }}
```

Update demo banner styles in `static/style.css` — replace the existing `.demo-banner` block (lines 2185-2254) with:

```css
.demo-banner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-4);
  background: var(--color-bg-banner);
  border-bottom: 1px solid var(--color-warning);
  font-size: var(--font-size-sm);
  color: #92400e;
}

.demo-banner-close {
  background: none;
  border: none;
  color: #92400e;
  cursor: pointer;
  font-size: var(--font-size-lg);
  padding: 0 var(--space-1);
  line-height: 1;
}
```

- [ ] **Step 5: Hide destructive buttons in demo mode**

In `templates/books.html`, wrap the delete dropdown in the book card (around lines 116-142) with:
```html
{{ if not $.Demo.Enabled }}
  ...delete dropdown markup...
{{ end }}
```

In `templates/book.html`, wrap delete buttons and ISBN edit form similarly:
```html
{{ if not .Demo.Enabled }}
  ...delete/edit markup...
{{ end }}
```

Keep the favourite toggle visible in demo mode (it's a read-like action).

- [ ] **Step 6: Verify all fixes**

Run: `make run`

Check:
- Visit `/nonexistent` → styled 404 page with nav and "Back to books" link
- Settings page shows `v0.7.4` (not `vv0.7.4`) — or `dev` in local mode
- Demo mode banner is compact amber bar (test with `DEMO_MODE=true`)
- Delete buttons are hidden in demo mode
- Delete buttons are visible in normal mode

- [ ] **Step 7: Commit**

```bash
git add templates/404.html templates/settings.html templates/demo-banner.html templates/books.html templates/book.html internal/http/router.go static/style.css
git commit -m "fix: styled 404 page, version string, demo banner, demo-mode buttons

- Add styled 404 page with navigation and CTA
- Remove duplicate 'v' prefix from version display
- Restyle demo banner as compact amber info bar
- Hide destructive buttons when demo mode is active"
```

---

## Task 4: Visual Polish — Header, Cards, Cover Placeholders

**Files:**
- Modify: `static/style.css` (header, card, cover, footer styles)
- Modify: `templates/base.html` (footer content)
- Modify: `templates/books.html` (book card hover, cover placeholder)
- Modify: `internal/http/router.go` (add `coverGradient` template func)

- [ ] **Step 1: Update header styles in `static/style.css`**

Replace existing header styles (around lines 45-109) with Clean Modern header:

```css
.header {
  border-bottom: 1px solid var(--color-border);
  background: var(--color-bg-page);
}

.header-content {
  max-width: 800px;
  margin: 0 auto;
  padding: var(--space-3) var(--space-4);
  display: flex;
  align-items: center;
  gap: var(--space-6);
}

.logo {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--color-text-primary);
  text-decoration: none;
  letter-spacing: -0.5px;
}

.nav-links {
  display: flex;
  gap: var(--space-5);
}

.nav-links a {
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  text-decoration: none;
  padding-bottom: var(--space-1);
  font-weight: var(--font-weight-medium);
  transition: color 0.15s ease;
}

.nav-links a:hover {
  color: var(--color-text-primary);
}

.nav-links a.active {
  color: var(--color-text-primary);
  border-bottom: 2px solid var(--color-accent);
}

.user-menu {
  margin-left: auto;
  font-size: var(--font-size-sm);
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
```

- [ ] **Step 2: Update book card styles in `static/style.css`**

Update `.book-card` styles to use tokens with hover effects:

```css
.book-card {
  display: flex;
  gap: var(--space-3);
  padding: var(--space-4);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  background: var(--color-bg-card);
  box-shadow: 0 1px 3px var(--color-shadow);
  transition: box-shadow 0.2s ease, transform 0.2s ease;
  cursor: pointer;
  text-decoration: none;
  color: inherit;
}

.book-card:hover {
  box-shadow: 0 4px 12px var(--color-shadow-hover);
  transform: scale(1.01);
}

.book-card-cover {
  width: 50px;
  height: 75px;
  border-radius: var(--radius-sm);
  object-fit: cover;
  flex-shrink: 0;
}

.book-title {
  font-weight: var(--font-weight-semibold);
  font-size: var(--font-size-base);
  color: var(--color-text-primary);
}

.book-author {
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  margin-top: var(--space-1);
}

.book-meta {
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
  margin-top: var(--space-1);
}
```

- [ ] **Step 3: Add cover placeholder support**

Add `coverGradient` template function in `internal/http/router.go` inside `loadTemplates()`:

```go
funcMap := template.FuncMap{
    // ...existing funcs...
    "coverGradient": func(title string) string {
        gradients := []string{
            "linear-gradient(135deg, #6366f1, #818cf8)",
            "linear-gradient(135deg, #0d9488, #2dd4bf)",
            "linear-gradient(135deg, #d97706, #fbbf24)",
            "linear-gradient(135deg, #dc2626, #f87171)",
            "linear-gradient(135deg, #7c3aed, #a78bfa)",
            "linear-gradient(135deg, #059669, #34d399)",
            "linear-gradient(135deg, #2563eb, #60a5fa)",
            "linear-gradient(135deg, #c2410c, #fb923c)",
        }
        h := 0
        for _, c := range title {
            h = h*31 + int(c)
        }
        if h < 0 {
            h = -h
        }
        return gradients[h%len(gradients)]
    },
}
```

In `templates/books.html`, update the book card cover section to handle missing covers:

```html
{{ if .CoverURL }}
  <img class="book-card-cover" src="/api/books/{{ .ID }}/cover" alt="{{ .Title }} cover">
{{ else }}
  <div class="book-card-cover cover-placeholder" style="background: {{ coverGradient .Title }};">
    <svg width="24" height="24" viewBox="0 0 24 24" fill="rgba(255,255,255,0.5)" xmlns="http://www.w3.org/2000/svg">
      <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>
    </svg>
  </div>
{{ end }}
```

Add CSS for cover placeholder:

```css
.cover-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
}
```

- [ ] **Step 4: Populate footer template in `templates/base.html`**

Replace the empty footer (line 155-156) with:

```html
{{ define "footer" }}
<footer class="footer">
  <div class="footer-content">
    <span class="footer-version">{{ .Version }}</span>
  </div>
</footer>
{{ end }}
```

Add footer CSS in `static/style.css`:

```css
.footer {
  border-top: 1px solid var(--color-border);
  margin-top: var(--space-10);
  padding: var(--space-4) 0;
}

.footer-content {
  max-width: 800px;
  margin: 0 auto;
  padding: 0 var(--space-4);
  font-size: var(--font-size-xs);
  color: var(--color-text-muted);
}
```

- [ ] **Step 5: Verify visual changes**

Run: `make run`

Check:
- Header is clean, compact, no gradient
- Book cards have subtle shadow and hover lift effect
- Books without covers show colored gradient placeholder with book icon
- Footer appears on all pages with version

- [ ] **Step 6: Commit**

```bash
git add static/style.css templates/base.html templates/books.html internal/http/router.go
git commit -m "feat: visual refresh — header, book cards, cover placeholders, footer

- Clean Modern header with accent-colored active nav underline
- Book cards with 12px radius, shadow, hover lift effect
- Generated gradient placeholders for missing covers
- Footer with version on every page"
```

---

## Task 5: Visual Polish — Tags, Highlights, Transitions

**Files:**
- Modify: `static/style.css` (tag, highlight, transition styles)
- Modify: `templates/book.html` (always-visible action icons)
- Modify: `templates/books.html` (consistent tag styling)

- [ ] **Step 1: Update tag pill styles globally in `static/style.css`**

Replace/update tag-related styles for consistency:

```css
.tag-chip,
.book-card-tag,
.tag-filter-chip {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2);
  background: var(--color-bg-tag);
  color: var(--color-text-tag);
  font-size: var(--font-size-xs);
  border-radius: var(--radius-full);
  text-decoration: none;
  font-weight: var(--font-weight-medium);
}

.tag-filter-chip .tag-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: transparent;
  border: none;
  color: var(--color-text-tag);
  cursor: pointer;
  font-size: var(--font-size-xs);
  transition: background 0.15s ease;
}

.tag-filter-chip .tag-remove:hover {
  background: var(--color-accent-light);
}
```

- [ ] **Step 2: Make highlight action icons always visible in `templates/book.html`**

Find the favourite button and action icons in the highlight cards. Remove any CSS that hides them by default and only shows on hover. Instead, render them always visible but muted:

```css
.highlight-actions {
  display: flex;
  gap: var(--space-2);
  align-items: center;
}

.highlight-actions button,
.highlight-actions a {
  color: var(--color-text-muted);
  transition: color 0.15s ease;
}

.highlight-actions button:hover,
.highlight-actions a:hover {
  color: var(--color-text-primary);
}

.favourite-btn.active {
  color: var(--color-danger);
}
```

- [ ] **Step 3: Add HTMX transition styles to `static/style.css`**

```css
#books-content {
  transition: opacity 150ms ease-in-out;
}

#books-content.htmx-swapping {
  opacity: 0;
}

#books-content.htmx-settling {
  opacity: 1;
}
```

- [ ] **Step 4: Verify**

Run: `make run`

Check:
- Tags look consistent everywhere (homepage, book detail, filter bar)
- Favourite hearts and action icons are visible without hover
- Filter "x" buttons are clearly styled and distinct from tag text

- [ ] **Step 5: Commit**

```bash
git add static/style.css templates/book.html templates/books.html
git commit -m "feat: consistent tag styling, always-visible actions, HTMX transitions

- Unified tag pill appearance across all pages
- Highlight action icons always visible (muted, not hover-only)
- Opacity fade transitions for HTMX content swaps"
```

---

## Task 6: Empty States

**Files:**
- Create: `templates/empty-states.html`
- Modify: `static/style.css` (empty state styles)
- Modify: `templates/books.html` (use empty state)
- Modify: `templates/book.html` (use empty-highlights state)
- Modify: `templates/favourites.html` (use empty state)
- Modify: `templates/vocabulary.html` (use empty state)

- [ ] **Step 1: Create `templates/empty-states.html`**

```html
{{ define "empty-books" }}
<div class="empty-state">
  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-muted)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
    <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/>
    <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>
  </svg>
  <h2>Your library is empty</h2>
  <p class="empty-state-text">Import highlights from Kindle, Apple Books, Readwise, or other sources to get started.</p>
  <a href="/settings" class="btn btn-primary">Import Highlights</a>
</div>
{{ end }}

{{ define "empty-search" }}
<div class="empty-state">
  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-muted)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
    <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
  </svg>
  <h2>No books match your search</h2>
  <p class="empty-state-text">Try a different search term or clear your filters.</p>
  <button class="btn btn-secondary" onclick="document.querySelector('input[name=q]').value=''; htmx.trigger(document.querySelector('input[name=q]'), 'search')">Clear search</button>
</div>
{{ end }}

{{ define "empty-highlights" }}
<div class="empty-state" style="min-height: 150px; padding: var(--space-6) var(--space-4);">
  <p class="empty-state-text">No highlights yet for this book.</p>
</div>
{{ end }}

{{ define "empty-favourites" }}
<div class="empty-state">
  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-muted)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
    <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/>
  </svg>
  <h2>No favourites yet</h2>
  <p class="empty-state-text">Browse your books and tap the heart icon on highlights you love.</p>
  <a href="/" class="btn btn-secondary">Browse books</a>
</div>
{{ end }}

{{ define "empty-vocabulary" }}
<div class="empty-state">
  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--color-text-muted)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
    <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/>
    <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/>
  </svg>
  <h2>No vocabulary words yet</h2>
  <p class="empty-state-text">Words from your highlights will appear here. You can also add words manually.</p>
  <a href="/" class="btn btn-secondary">Browse books</a>
</div>
{{ end }}
```

- [ ] **Step 2: Add empty state CSS to `static/style.css`**

```css
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: var(--space-12) var(--space-4);
  min-height: 300px;
}

.empty-state svg {
  margin-bottom: var(--space-4);
}

.empty-state h2 {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
  color: var(--color-text-primary);
  margin: 0 0 var(--space-2) 0;
}

.empty-state-text {
  font-size: var(--font-size-base);
  color: var(--color-text-secondary);
  max-width: 400px;
  margin: 0 0 var(--space-6) 0;
  line-height: var(--line-height-normal);
}

.btn {
  display: inline-flex;
  align-items: center;
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  font-size: var(--font-size-base);
  font-weight: var(--font-weight-medium);
  text-decoration: none;
  cursor: pointer;
  border: none;
  transition: background 0.15s ease;
}

.btn-primary {
  background: var(--color-accent);
  color: #ffffff;
}

.btn-primary:hover {
  background: var(--color-accent-hover);
}

.btn-secondary {
  background: var(--color-bg-input);
  color: var(--color-text-primary);
  border: 1px solid var(--color-border);
}

.btn-secondary:hover {
  background: var(--color-bg-hover);
}
```

- [ ] **Step 3: Use empty states in templates**

In `templates/books.html`, in the `book-list` template, add conditional:

```html
{{ define "book-list" }}
{{ if . }}
  {{ range . }}
    ...existing book card markup...
  {{ end }}
{{ else }}
  {{ template "empty-search" }}
{{ end }}
{{ end }}
```

In the main `books` template, handle zero-books case:

```html
{{ if and (not .Books) (not .SearchQuery) (eq .SelectedTagID 0) }}
  {{ template "empty-books" . }}
{{ else }}
  ...search bar, sort, book list, pagination...
{{ end }}
```

In `templates/favourites.html`:
```html
{{ if .Favourites }}
  ...existing list...
{{ else }}
  {{ template "empty-favourites" . }}
{{ end }}
```

In `templates/book.html`, wrap the highlights list:
```html
{{ if .Book.Highlights }}
  ...existing highlights list...
{{ else }}
  {{ template "empty-highlights" }}
{{ end }}
```

In `templates/vocabulary.html`:
```html
{{ if .Words }}
  ...existing grid...
{{ else }}
  {{ template "empty-vocabulary" . }}
{{ end }}
```

- [ ] **Step 4: Verify empty states**

Run: `make run` (with empty database or test with cleared data)

Check each empty state renders with icon, heading, subtext, and CTA link.

- [ ] **Step 5: Commit**

```bash
git add templates/empty-states.html static/style.css templates/books.html templates/favourites.html templates/vocabulary.html
git commit -m "feat: add empty states for books, favourites, vocabulary, and search

Each empty state shows an icon, heading, descriptive text, and
a call-to-action guiding users to the next step."
```

---

## Task 7: ListBooks Repository Method (Backend)

**Files:**
- Create: `internal/database/books/list.go`
- Create: `internal/database/books/list_test.go`

- [ ] **Step 1: Write tests for `ListBooks` in `internal/database/books/list_test.go`**

```go
package books_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    // import your books package, entities, and test DB helpers
)

// Test data seeding pattern: use the existing setupRepo(t) helper from
// repository_test.go. Seed books via repo.SaveBook(&entities.Book{...}).
// For tags, use the tag repository or direct DB inserts.
// All tests should use a consistent UserID (e.g., 1) and verify that
// books from other users (UserID: 2) are NOT returned.

func TestListBooks_DefaultSort(t *testing.T) {
    // Setup: seed DB with 3 books created at different times (UserID: 1)
    // Call ListBooks with UserID: 1, default options (Sort: "date_desc")
    // Assert: books returned in created_at DESC order
}

func TestListBooks_Pagination(t *testing.T) {
    // Setup: seed DB with 25 books
    // Call ListBooks with Page: 1, PerPage: 20
    // Assert: 20 books returned, totalCount == 25
    // Call ListBooks with Page: 2, PerPage: 20
    // Assert: 5 books returned, totalCount == 25
}

func TestListBooks_SortByTitle(t *testing.T) {
    // Setup: seed DB with books "Zebra", "Alpha", "Middle"
    // Call ListBooks with Sort: "title_asc"
    // Assert: Alpha, Middle, Zebra
}

func TestListBooks_SortByHighlightCount(t *testing.T) {
    // Setup: seed books with varying highlight counts
    // Call ListBooks with Sort: "highlights_desc"
    // Assert: ordered by highlight count descending
}

func TestListBooks_SearchFilter(t *testing.T) {
    // Setup: seed books with known titles
    // Call ListBooks with Query: "war"
    // Assert: only matching books returned, totalCount reflects filter
}

func TestListBooks_TagFilter(t *testing.T) {
    // Setup: seed books with tags
    // Call ListBooks with TagID: tagID
    // Assert: only books with that tag returned
}

func TestListBooks_InvalidPageClamped(t *testing.T) {
    // Setup: seed 5 books
    // Call ListBooks with Page: 999, PerPage: 20
    // Assert: returns page 1 results (clamped), totalCount == 5
}

func TestListBooks_CombinedFilters(t *testing.T) {
    // Setup: seed books with titles, tags
    // Call ListBooks with Query + TagID + Sort + Page
    // Assert: all filters compose correctly
}

func TestListBooks_UserScoping(t *testing.T) {
    // Setup: seed books for UserID 1 AND UserID 2
    // Call ListBooks with UserID: 1
    // Assert: only user 1's books returned, user 2's excluded
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — `ListBooks` not defined

- [ ] **Step 3: Implement `ListBooks` in `internal/database/books/list.go`**

```go
package books

import (
    "strings"

    "gorm.io/gorm"
    "your-module/internal/entities"
)

type ListBooksOptions struct {
    UserID  uint
    Query   string
    TagID   uint
    Sort    string
    Page    int
    PerPage int
}

// BookListItem is a lightweight book representation for listing pages.
// Does NOT preload full highlights — only loads count for display.
type BookListItem struct {
    entities.Book
    HighlightCount int64 `gorm:"column:highlight_count"`
}

type ListBooksResult struct {
    Books      []BookListItem
    TotalCount int64
    Page       int
    TotalPages int
}

func (r *Repository) ListBooks(opts ListBooksOptions) (*ListBooksResult, error) {
    if opts.Page < 1 {
        opts.Page = 1
    }
    if opts.PerPage < 1 {
        opts.PerPage = 20
    }

    // Base query — no Highlights preload (only need count, not full text)
    query := r.db.Model(&entities.Book{}).
        Select("books.*, (SELECT COUNT(*) FROM highlights WHERE highlights.book_id = books.id AND highlights.deleted_at IS NULL) as highlight_count").
        Preload("Tags").
        Preload("Source").
        Where("books.user_id = ? AND books.deleted_at IS NULL", opts.UserID)

    // Search filter
    if opts.Query != "" {
        search := "%" + strings.ToLower(opts.Query) + "%"
        query = query.Where("LOWER(books.title) LIKE ? OR LOWER(books.author) LIKE ?", search, search)
    }

    // Tag filter — use subquery to avoid double-counting in COUNT
    if opts.TagID > 0 {
        query = query.Where("books.id IN (SELECT book_id FROM book_tags WHERE tag_id = ?)", opts.TagID)
    }

    // Count total (before pagination) — use a clean count query
    var totalCount int64
    countQuery := r.db.Model(&entities.Book{}).
        Where("books.user_id = ? AND books.deleted_at IS NULL", opts.UserID)
    if opts.Query != "" {
        search := "%" + strings.ToLower(opts.Query) + "%"
        countQuery = countQuery.Where("LOWER(books.title) LIKE ? OR LOWER(books.author) LIKE ?", search, search)
    }
    if opts.TagID > 0 {
        countQuery = countQuery.Where("books.id IN (SELECT book_id FROM book_tags WHERE tag_id = ?)", opts.TagID)
    }
    if err := countQuery.Count(&totalCount).Error; err != nil {
        return nil, err
    }

    // Calculate total pages and clamp
    totalPages := int((totalCount + int64(opts.PerPage) - 1) / int64(opts.PerPage))
    if totalPages < 1 {
        totalPages = 1
    }
    if opts.Page > totalPages {
        opts.Page = totalPages
    }

    // Sort
    orderClause := sortToOrderClause(opts.Sort)
    query = query.Order(orderClause)

    // Pagination
    offset := (opts.Page - 1) * opts.PerPage
    query = query.Offset(offset).Limit(opts.PerPage)

    var books []BookListItem
    if err := query.Find(&books).Error; err != nil {
        return nil, err
    }

    return &ListBooksResult{
        Books:      books,
        TotalCount: totalCount,
        Page:       opts.Page,
        TotalPages: totalPages,
    }, nil
}

func sortToOrderClause(sort string) string {
    switch sort {
    case "date_asc":
        return "books.created_at ASC"
    case "title_asc":
        return "books.title ASC"
    case "title_desc":
        return "books.title DESC"
    case "author_asc":
        return "books.author ASC"
    case "author_desc":
        return "books.author DESC"
    case "highlights_desc":
        return "highlight_count DESC"
    case "highlights_asc":
        return "highlight_count ASC"
    default: // "date_desc" or unknown
        return "books.created_at DESC"
    }
}

func needsHighlightCount(sort string) bool {
    return sort == "highlights_desc" || sort == "highlights_asc"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `make test`
Expected: All `TestListBooks_*` tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/database/books/list.go internal/database/books/list_test.go
git commit -m "feat: add ListBooks repository method with sort, filter, pagination

Supports search query, tag filter, 8 sort options, and page-based
pagination with total count and page clamping."
```

---

## Task 8: Update UI Handlers for Sort & Pagination

> **Important:** Tasks 8 and 9 are tightly coupled — the handler changes (Task 8) return data for the new template structure (Task 9). They must be implemented and committed together. The app will be in a broken state between them.

**Files:**
- Modify: `internal/http/ui.go` (update BooksPage, SearchBooks, add BookLister interface)
- Modify: `internal/http/helpers.go` (add ParsePageParam helper)
- Create: `templates/pagination.html`

- [ ] **Step 1: Add `ParsePageParam` helper in `internal/http/helpers.go`**

```go
func ParsePageParam(c *gin.Context) int {
    pageStr := c.DefaultQuery("page", "1")
    page, err := strconv.Atoi(pageStr)
    if err != nil || page < 1 {
        return 1
    }
    return page
}
```

- [ ] **Step 2: Update `BooksPage` handler in `internal/http/ui.go`**

Replace the current `BooksPage` method with one that uses `ListBooks`:

```go
func (controller *UIController) BooksPage(c *gin.Context) {
    userID := GetUserID(c) // however user ID is currently extracted

    opts := books.ListBooksOptions{
        UserID:  userID,
        Query:   c.Query("q"),
        Sort:    c.DefaultQuery("sort", "date_desc"),
        Page:    ParsePageParam(c),
        PerPage: 20,
    }

    // Tag filter
    if tagIDStr := c.Query("tag"); tagIDStr != "" {
        tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
        if err == nil {
            opts.TagID = uint(tagID)
        }
    }

    result, err := controller.bookRepo.ListBooks(opts)
    if err != nil {
        c.HTML(http.StatusInternalServerError, "error", gin.H{"Error": err.Error()})
        return
    }

    // Load tags for filter UI
    tags, _ := controller.tagStore.GetAllTags()

    RenderPage(c, http.StatusOK, "books", gin.H{
        "ActivePage":     "books",
        "Books":          result.Books,
        "BookCount":      result.TotalCount,
        "Tags":           tags,
        "SelectedTagID":  opts.TagID,
        "SearchQuery":    opts.Query,
        "Sort":           opts.Sort,
        "Page":           result.Page,
        "TotalPages":     result.TotalPages,
    })
}
```

**Wiring `ListBooks` into UIController (ISP pattern):**

The codebase follows ISP — consumers define narrow interfaces. Add a `BookLister` interface in `internal/http/ui.go`:

```go
type BookLister interface {
    ListBooks(opts books.ListBooksOptions) (*books.ListBooksResult, error)
}
```

Add a `bookLister BookLister` field to `UIController` and update `NewUIController` to accept it. In `NewRouter`, pass the books repository (which already implements `ListBooks`) as the `BookLister` argument. The existing `reader` field (for `GetBookByID`, etc.) stays unchanged.

- [ ] **Step 3: Update `SearchBooks` handler to return full `#books-content`**

The `SearchBooks` handler should return the same content region (counts + sort + list + pagination) instead of just the book list:

```go
func (controller *UIController) SearchBooks(c *gin.Context) {
    // Same ListBooks call as BooksPage, but render only the "books-content" partial
    // ... (same opts construction as BooksPage)

    result, err := controller.bookRepo.ListBooks(opts)
    if err != nil {
        c.HTML(http.StatusInternalServerError, "book-list", nil)
        return
    }

    tags, _ := controller.tagStore.GetAllTags()

    c.HTML(http.StatusOK, "books-content", gin.H{
        "Books":         result.Books,
        "BookCount":     result.TotalCount,
        "Tags":          tags,
        "SelectedTagID": opts.TagID,
        "SearchQuery":   opts.Query,
        "Sort":          opts.Sort,
        "Page":          result.Page,
        "TotalPages":    result.TotalPages,
    })
}
```

- [ ] **Step 4: Create `templates/pagination.html`**

```html
{{ define "pagination" }}
{{ if gt .TotalPages 1 }}
<nav class="pagination" aria-label="Page navigation">
  {{ if gt .Page 1 }}
    <a href="?page={{ subtract .Page 1 }}&sort={{ .Sort }}{{ if .SearchQuery }}&q={{ .SearchQuery }}{{ end }}{{ if .SelectedTagID }}&tag={{ .SelectedTagID }}{{ end }}"
       class="pagination-link" hx-get="/ui/books/search?page={{ subtract .Page 1 }}&sort={{ .Sort }}{{ if .SearchQuery }}&q={{ .SearchQuery }}{{ end }}{{ if .SelectedTagID }}&tag={{ .SelectedTagID }}{{ end }}"
       hx-target="#books-content" hx-push-url="true" hx-swap="innerHTML show:top">
      &larr; Previous
    </a>
  {{ else }}
    <span class="pagination-link disabled">&larr; Previous</span>
  {{ end }}

  {{ range (pageRange .Page .TotalPages) }}
    {{ if eq . -1 }}
      <span class="pagination-ellipsis">&hellip;</span>
    {{ else if eq . $.Page }}
      <span class="pagination-link active">{{ . }}</span>
    {{ else }}
      <a href="?page={{ . }}&sort={{ $.Sort }}{{ if $.SearchQuery }}&q={{ $.SearchQuery }}{{ end }}{{ if $.SelectedTagID }}&tag={{ $.SelectedTagID }}{{ end }}"
         class="pagination-link" hx-get="/ui/books/search?page={{ . }}&sort={{ $.Sort }}{{ if $.SearchQuery }}&q={{ $.SearchQuery }}{{ end }}{{ if $.SelectedTagID }}&tag={{ $.SelectedTagID }}{{ end }}"
         hx-target="#books-content" hx-push-url="true" hx-swap="innerHTML show:top">
        {{ . }}
      </a>
    {{ end }}
  {{ end }}

  {{ if lt .Page .TotalPages }}
    <a href="?page={{ add .Page 1 }}&sort={{ .Sort }}{{ if .SearchQuery }}&q={{ .SearchQuery }}{{ end }}{{ if .SelectedTagID }}&tag={{ .SelectedTagID }}{{ end }}"
       class="pagination-link" hx-get="/ui/books/search?page={{ add .Page 1 }}&sort={{ .Sort }}{{ if .SearchQuery }}&q={{ .SearchQuery }}{{ end }}{{ if .SelectedTagID }}&tag={{ .SelectedTagID }}{{ end }}"
       hx-target="#books-content" hx-push-url="true" hx-swap="innerHTML show:top">
      Next &rarr;
    </a>
  {{ else }}
    <span class="pagination-link disabled">Next &rarr;</span>
  {{ end }}
</nav>
{{ end }}
{{ end }}
```

- [ ] **Step 5: Add `pageRange` template function in `internal/http/router.go`**

```go
"pageRange": func(current, total int) []int {
    if total <= 7 {
        r := make([]int, total)
        for i := range r {
            r[i] = i + 1
        }
        return r
    }
    pages := []int{}
    pages = append(pages, 1, 2)
    if current > 4 {
        pages = append(pages, -1) // ellipsis
    }
    for i := current - 1; i <= current+1; i++ {
        if i > 2 && i < total-1 {
            pages = append(pages, i)
        }
    }
    if current < total-3 {
        pages = append(pages, -1) // ellipsis
    }
    pages = append(pages, total-1, total)
    // Deduplicate
    seen := map[int]bool{}
    result := []int{}
    for _, p := range pages {
        if p == -1 || !seen[p] {
            if p != -1 {
                seen[p] = true
            }
            result = append(result, p)
        }
    }
    return result
},
```

Also ensure `subtract` is available (or rename to match existing `add`/`subtract` funcs).

- [ ] **Step 6: Add pagination CSS to `static/style.css`**

```css
.pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: var(--space-1);
  margin-top: var(--space-6);
  padding: var(--space-4) 0;
}

.pagination-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 32px;
  padding: 0 var(--space-2);
  border-radius: var(--radius-sm);
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
  text-decoration: none;
  background: var(--color-bg-input);
  transition: background 0.15s ease, color 0.15s ease;
}

.pagination-link:hover:not(.disabled):not(.active) {
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
}

.pagination-link.active {
  background: var(--color-accent);
  color: #ffffff;
  font-weight: var(--font-weight-medium);
}

.pagination-link.disabled {
  color: var(--color-text-muted);
  cursor: default;
  background: transparent;
}

.pagination-ellipsis {
  color: var(--color-text-muted);
  padding: 0 var(--space-1);
}
```

- [ ] **Step 7: Verify sorting and pagination**

Run: `make run`

Seed with >20 books if needed. Check:
- Sort dropdown changes book order
- Pagination controls appear when >20 books
- Page navigation works (HTMX partial swap, scroll to top)
- URL updates with sort/page/q params
- Counts update when filtering
- Sort + search + tag filter compose correctly

- [ ] **Step 8: Commit**

```bash
git add internal/http/ui.go internal/http/helpers.go internal/http/router.go templates/pagination.html static/style.css
git commit -m "feat: add sorting and pagination to books page

- 8 sort options (date, title, author, highlights — each asc/desc)
- Classic pagination with 20 books per page
- All params compose and persist in URL
- HTMX partial swap with opacity transitions"
```

---

## Task 9: Books Page Template — Sort Dropdown & Content Container

**Files:**
- Modify: `templates/books.html` (restructure for `#books-content`, add sort, HTMX attrs)

- [ ] **Step 1: Restructure `templates/books.html`**

The main `books` template should wrap the dynamic content in a `#books-content` div. Extract the dynamic portion as a `books-content` partial that can be returned by both the full page load and HTMX requests.

```html
{{ define "books" }}
<!DOCTYPE html>
<html lang="en">
<head>
  {{ template "base-head" . }}
  <title>Books — Highlights</title>
</head>
<body>
  {{ template "header" . }}
  <div class="container">
    <div class="page-header">
      <h1>Books</h1>
    </div>

    <!-- Search bar (outside swap target — stays stable) -->
    <div class="search-box">
      <input type="search" name="q" placeholder="Search books..."
             value="{{ .SearchQuery }}"
             hx-get="/ui/books/search"
             hx-trigger="input changed delay:300ms, search"
             hx-target="#books-content"
             hx-push-url="true"
             hx-include="[name='sort'],[name='tag']"
             hx-swap="innerHTML show:top">
    </div>

    <!-- Dynamic content region -->
    <div id="books-content">
      {{ template "books-content" . }}
    </div>
  </div>
  {{ template "footer" . }}
  {{ template "scripts-common" . }}
</body>
</html>
{{ end }}

{{ define "books-content" }}
{{ if and (not .Books) (not .SearchQuery) (eq .SelectedTagID 0) }}
  {{ template "empty-books" . }}
{{ else }}
  <!-- Stats row with sort -->
  <!-- Note: BookCount is the filtered total from ListBooksResult.TotalCount -->
  <!-- Individual book highlight counts come from BookListItem.HighlightCount (computed column) -->
  <!-- Use .HighlightCount on each book card instead of len(.Highlights) -->
  <div class="books-toolbar">
    <span class="books-count">{{ .BookCount }} books</span>
    <div class="sort-control">
      <label for="sort-select">Sort by:</label>
      <select id="sort-select" name="sort"
              hx-get="/ui/books/search"
              hx-trigger="change"
              hx-target="#books-content"
              hx-push-url="true"
              hx-include="[name='q'],[name='tag']"
              hx-swap="innerHTML show:top">
        <option value="date_desc" {{ if eq .Sort "date_desc" }}selected{{ end }}>Date added (newest)</option>
        <option value="date_asc" {{ if eq .Sort "date_asc" }}selected{{ end }}>Date added (oldest)</option>
        <option value="title_asc" {{ if eq .Sort "title_asc" }}selected{{ end }}>Title (A→Z)</option>
        <option value="title_desc" {{ if eq .Sort "title_desc" }}selected{{ end }}>Title (Z→A)</option>
        <option value="author_asc" {{ if eq .Sort "author_asc" }}selected{{ end }}>Author (A→Z)</option>
        <option value="author_desc" {{ if eq .Sort "author_desc" }}selected{{ end }}>Author (Z→A)</option>
        <option value="highlights_desc" {{ if eq .Sort "highlights_desc" }}selected{{ end }}>Most highlights</option>
        <option value="highlights_asc" {{ if eq .Sort "highlights_asc" }}selected{{ end }}>Fewest highlights</option>
      </select>
    </div>
  </div>

  <!-- Tag filters (HTMX-driven) -->
  {{ if .Tags }}
  <div class="tag-filters">
    <span class="tag-filter-label">Filter by tag:</span>
    <a href="/"
       class="tag-filter-chip {{ if eq .SelectedTagID 0 }}active{{ end }}"
       hx-get="/ui/books/search?tag=0"
       hx-target="#books-content"
       hx-push-url="true"
       hx-include="[name='q'],[name='sort']"
       hx-swap="innerHTML show:top">All</a>
    {{ range .Tags }}
    <a href="/?tag={{ .ID }}"
       class="tag-filter-chip {{ if eq .ID $.SelectedTagID }}active{{ end }}"
       hx-get="/ui/books/search?tag={{ .ID }}"
       hx-target="#books-content"
       hx-push-url="true"
       hx-include="[name='q'],[name='sort']"
       hx-swap="innerHTML show:top">{{ .Name }}</a>
    {{ end }}
  </div>
  {{ end }}

  <!-- Book list -->
  <div class="book-list">
    {{ if .Books }}
      {{ template "book-list" .Books }}
    {{ else }}
      {{ template "empty-search" }}
    {{ end }}
  </div>

  <!-- Pagination -->
  {{ template "pagination" . }}
{{ end }}
{{ end }}
```

- [ ] **Step 2: Add sort control CSS to `static/style.css`**

```css
.books-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-4);
}

.books-count {
  font-size: var(--font-size-sm);
  color: var(--color-text-muted);
}

.sort-control {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.sort-control label {
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
}

.sort-control select {
  font-size: var(--font-size-sm);
  padding: var(--space-1) var(--space-2);
  border: 1px solid var(--color-border-input);
  border-radius: var(--radius-sm);
  background: var(--color-bg-page);
  color: var(--color-text-primary);
  cursor: pointer;
}

.sort-control select:focus {
  outline: none;
  border-color: var(--color-border-active);
}

.tag-filters {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
  margin-bottom: var(--space-4);
}

.tag-filter-label {
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
}

.tag-filter-chip.active {
  background: var(--color-accent);
  color: #ffffff;
}
```

- [ ] **Step 3: Add accessibility attributes**

Ensure the sort select has `aria-label="Sort books"` and pagination links have `aria-label="Page N"` / `aria-label="Previous page"` / `aria-label="Next page"`.

- [ ] **Step 4: Verify the complete flow**

Run: `make run`

Full integration check:
1. Homepage loads with sort dropdown and book list
2. Search filters books, URL updates, counts update
3. Tag filter works via HTMX (no full page reload)
4. Sort changes order, resets to page 1
5. Pagination appears with >20 books
6. All params compose: `?q=war&tag=2&sort=title_asc&page=1`
7. Empty states show when no books / no search results
8. Transitions are smooth (opacity fade)

- [ ] **Step 5: Commit**

```bash
git add templates/books.html static/style.css
git commit -m "feat: restructure books page with sort dropdown, HTMX content container

- #books-content swap target for all dynamic updates
- Sort dropdown with 8 options, HTMX-driven
- Tag filters converted from full-page links to HTMX
- Search, sort, tag filter, pagination all compose via URL params"
```

---

## Task 10: Final Polish & Lint

**Files:**
- All modified files

- [ ] **Step 1: Run linter**

Run: `make lint`

Fix any issues found.

- [ ] **Step 2: Run all tests**

Run: `make test`

Fix any failures.

- [ ] **Step 3: Run test coverage**

Run: `make test_coverage`

Verify no significant coverage regression.

- [ ] **Step 4: Manual smoke test**

Run: `make run`

Walk through every page:
- Homepage (with and without books)
- Book detail page
- Favourites (with and without favourites)
- Vocabulary (with and without words)
- Settings (all tabs)
- 404 page
- Demo mode (set `DEMO_MODE=true`)

- [ ] **Step 5: Commit any remaining fixes**

```bash
git add -A
git commit -m "chore: lint fixes and final polish for v1.0.0 design refresh"
```

---

## Summary

| Task | Description | Est. Complexity |
|------|-------------|-----------------|
| 1 | Design token system | Medium |
| 2 | Header consolidation | Small |
| 3 | Bug fixes (6 items) | Small |
| 4 | Visual polish — header, cards, covers, footer | Medium |
| 5 | Visual polish — tags, highlights, transitions | Small |
| 6 | Empty states | Small |
| 7 | ListBooks repository method | Medium |
| 8 | UI handlers for sort & pagination | Medium |
| 9 | Books template restructure | Medium |
| 10 | Final polish & lint | Small |

Tasks 1-6 are frontend-focused (CSS/templates). Tasks 7-8 are backend. Task 9 ties them together. Task 10 is verification.

Tasks 1-6 can be done in sequence. Task 7 (backend) is independent of 1-6 and could be done in parallel. Tasks 8-9 depend on both 7 and 1-6.
