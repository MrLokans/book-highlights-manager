---
version: 1
slug: "templates-base-html"
primary_target: "templates/base.html"
related_targets: ["templates"]
---

# Web app (all pages)

Scope: every server-rendered page (Books, Book, Favourites, Vocabulary, Settings, Profile, Audit, Login/Setup, 404). Mode: Operate.
Audience/job: public self-hosters re-reading highlights and managing imports/exports, equally.
Constraints: Go templates + HTMX, no build step, no inline styles, works offline (no font/script CDNs), light theme only.

## Direction contract

THESIS: A clean white proof sheet where the only colour is the reader's own highlighter ink. Refuses the grey-sidebar SaaS dashboard and the cream editorial book page.

OWN-WORLD: Pure white ground, ink #16181D text, cool grey #6B7078 secondary, hairline #E6E7EA rules. Marker yellow #FFE45C, pink #FF8FB8, blue #8CD3FF, orange #FFB65C, green #A6E3A1 used only as marker swipes. Ink-filled buttons, hairline outline secondaries, 6px radii, a self-hosted grotesk (Schibsted Grotesk, offline) with tabular figures for counts; monospace only for codes like ISBNs and tokens.

STORY: A stranger sees their library and its counts at once, knows where to import, and finds their passages drawn in the colour they marked them.

FIRST VIEWPORT: 64px white top bar: product name, nav (active item carries a yellow swipe), Import action at right. Page title at 40px with Export all at right, search, book count with sort, tag filters, then the book list under a quiet column header, counts right-aligned in tabular figures.

FORM: Highlighter on proof, IMPECCABLE'S PICK (rank 1 of 7). seed:9dfd103e28838a92

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
