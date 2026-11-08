---
name: Highlights Manager
description: A white proof sheet where the only colour is the reader's own highlighter ink.
colors:
  ink: "#16181d"
  ink-soft: "#2c2f36"
  marker-yellow: "#ffe45c"
  marker-pink: "#ff9cc2"
  marker-blue: "#9ed8ff"
  marker-orange: "#ffc071"
  marker-green: "#b5e8a8"
  paper: "#ffffff"
  paper-sunk: "#f6f7f8"
  muted: "#5f646d"
  faint: "#70757e"
  line: "#e6e7ea"
  line-strong: "#d3d5d9"
  ok: "#1d6b3e"
  ok-tint: "#ecf7ef"
  bad: "#b3261e"
  bad-tint: "#fdeeed"
  warn: "#7a4f00"
  warn-tint: "#fff6d6"
  info: "#1f4f8a"
typography:
  display:
    fontFamily: "Schibsted Grotesk, ui-sans-serif, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "40px"
    fontWeight: 700
    lineHeight: 1.05
    letterSpacing: "-0.035em"
  headline:
    fontFamily: "Schibsted Grotesk, ui-sans-serif, system-ui, sans-serif"
    fontSize: "30px"
    fontWeight: 700
    lineHeight: 1.05
    letterSpacing: "-0.035em"
  title:
    fontFamily: "Schibsted Grotesk, ui-sans-serif, system-ui, sans-serif"
    fontSize: "22px"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  highlight:
    fontFamily: "Schibsted Grotesk, ui-sans-serif, system-ui, sans-serif"
    fontSize: "17px"
    fontWeight: 400
    lineHeight: 1.75
  body:
    fontFamily: "Schibsted Grotesk, ui-sans-serif, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Schibsted Grotesk, ui-sans-serif, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1
  caption:
    fontFamily: "Schibsted Grotesk, ui-sans-serif, system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 500
  mono:
    fontFamily: "ui-monospace, SF Mono, Menlo, Consolas, monospace"
    fontSize: "13px"
    fontWeight: 400
rounded:
  badge: "4px"
  md: "6px"
  lg: "10px"
  pill: "999px"
spacing:
  s1: "4px"
  s2: "8px"
  s3: "12px"
  s4: "16px"
  s5: "20px"
  s6: "24px"
  s8: "32px"
  s10: "40px"
  s12: "48px"
  s16: "64px"
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    typography: "{typography.label}"
    rounded: "{rounded.md}"
    padding: "0 16px"
    height: "38px"
  button-primary-hover:
    backgroundColor: "{colors.ink-soft}"
    textColor: "{colors.paper}"
  button-secondary:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.md}"
    padding: "0 16px"
    height: "38px"
  button-danger:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.bad}"
    rounded: "{rounded.md}"
    padding: "0 16px"
    height: "38px"
  button-danger-hover:
    backgroundColor: "{colors.bad-tint}"
    textColor: "{colors.bad}"
  icon-button:
    textColor: "{colors.faint}"
    rounded: "{rounded.md}"
    size: "32px"
  icon-button-hover:
    backgroundColor: "{colors.paper-sunk}"
    textColor: "{colors.ink}"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: "8px 12px"
    height: "38px"
  tag-chip:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    padding: "0 12px"
    height: "28px"
  tag-chip-active:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
  badge:
    textColor: "{colors.muted}"
    typography: "{typography.caption}"
    rounded: "{rounded.badge}"
    padding: "2px 8px"
  alert-success:
    backgroundColor: "{colors.ok-tint}"
    textColor: "{colors.ok}"
    rounded: "{rounded.md}"
    padding: "12px 16px"
  alert-error:
    backgroundColor: "{colors.bad-tint}"
    textColor: "{colors.bad}"
    rounded: "{rounded.md}"
    padding: "12px 16px"
  alert-warning:
    backgroundColor: "{colors.warn-tint}"
    textColor: "{colors.warn}"
    rounded: "{rounded.md}"
    padding: "12px 16px"
  popover:
    backgroundColor: "{colors.paper}"
    rounded: "{rounded.lg}"
    padding: "4px"
---

# Design System: Highlights Manager

## Overview

**Creative North Star: "Highlighter on Proof"**

Every page is a clean white proof sheet set in black ink. The interface draws in two inks only: near-black for structure, type and every action, and highlighter colour for the reader's own marks. A highlight on the Book page is drawn as a hand-laid marker swipe in the colour the reader chose on their device, so the passages are the only colourful thing on the screen and the chrome stays out of their way.

The density is that of a working tool rather than a magazine. Lists are ruled tables: an ink rule opens each list and hairlines separate the rows, numbers sit right-aligned, and pages run in a single 1120px column under a 64px top bar. There are no cards on a grey ground, no sidebar shell and no cream "book page" texture. Depth is nearly absent; the sheet stays flat, and the only lift is a soft shadow on menus that float above it.

Light theme only, by decision. Fonts are self-hosted so the app renders identically offline.

**Key Characteristics:**
- Pure white ground, ink text, grey only for secondary information.
- Highlighter swipes carry the reader's highlight colour; yellow also marks "you are here".
- Ink-filled primary buttons, hairline-outlined secondaries, 6px corners.
- One grotesk family, tight negative tracking on headings.
- Ruled lists: ink rule on top, hairline between rows, no boxes.
- Flat sheet; shadow only on floating menus and popovers.

## Colors

Monochrome ink on white paper, with a five-colour highlighter set that appears only as marker swipes.

### Primary
- **Proof Ink** (ink): all text, primary button fills, active chip and pagination fills, the rule that opens every list, focus outlines, the filled favourite heart.
- **Soft Ink** (ink-soft): primary button hover; note text set against the ink-ruled note margin.

### Secondary
- **Highlighter Yellow** (marker-yellow): the default highlight colour and the system's one wayfinding mark. Used for highlights stored yellow or without a colour, the active top-nav item, the active Settings tab, text selection, the hover sweep under book titles, and the 60% tint in the input focus halo.

### Tertiary
- **Highlighter Pink, Blue, Orange, Green** (marker-pink, marker-blue, marker-orange, marker-green): only on highlight text, and only when the reader's stored highlight colour falls in that hue range. The server maps any stored colour to the nearest of the five by hue. Greys and unparseable values fall back to yellow.

### Neutral
- **Paper** (paper): page ground, input and secondary button fill, menu surfaces.
- **Sunk Paper** (paper-sunk): hover wash on icon buttons, chips and menu items; neutral notice and code-block fill.
- **Muted Grey** (muted): authors, metadata, labels, inactive nav, column heads.
- **Faint Grey** (faint): placeholders, idle icon buttons, footer, tertiary captions.
- **Hairline** (line): row dividers, top-bar rule, menu and card borders.
- **Strong Hairline** (line-strong): input, secondary button, chip and badge strokes.

### State
- **Success** (ok on ok-tint), **Error** (bad on bad-tint), **Warning** (warn on warn-tint): notices, import results, status badges and the demo banner. Each text colour passes 4.5:1 on its tint. **Info** (info) appears only as text and dot colour on badges and status dots.

### Named Rules
**The Reader's Ink Rule.** Pink, blue, orange and green mean one thing only: the colour the reader marked this passage with. They never encode source, status, category or decoration.

**The Two Inks Rule.** Everything that is not a highlight is ink, grey or white. If a new control needs colour to be understood, rethink it before adding a hue. State colours are reserved for success, failure and warnings.

**The One Pen Rule.** Yellow is the only marker the interface itself picks up: for the current location (nav, tab), selection and focus. It is always applied as a swipe or halo, never as a flat button or panel fill.

## Typography

**Display Font:** Schibsted Grotesk, self-hosted variable woff2 (400 to 900), with ui-sans-serif and system-ui as fallback
**Body Font:** Schibsted Grotesk
**Label/Mono Font:** system monospace (ui-monospace, SF Mono, Menlo, Consolas), only for codes: ISBNs, tokens, account and task IDs, pronunciations, code blocks

**Character:** One sturdy, slightly newsy grotesk does all the work. Hierarchy comes from size and weight with tight negative tracking on headings, never from a second family or from uppercase.

### Hierarchy
- **Display** (700, 40px, 1.05, -0.035em): page titles (Library, Favourites, Settings), auth headings, the vocabulary word detail. Drops to 30px on phones.
- **Headline** (700, 30px, 1.05): book title on the Book page (max 22ch), drops to 22px on phones.
- **Title** (700, 22px, -0.02em): section heads in Settings, Favourites book groups, vocabulary words, count figures and import stats.
- **Highlight** (400, 17px, 1.75, max 68ch): the reader's highlight text. The generous leading leaves room for the swipe.
- **Body** (400, 15px, 1.5): default text, inputs, notes (max 64ch), settings descriptions.
- **Label** (600, 13px): buttons, form labels, aside section heads, sort and filter controls.
- **Caption** (500 to 600, 12px): column heads, badges, stat labels, footer, word status.

### Named Rules
**The Counted Figures Rule.** Tabular figures only on counts, stats, pagination, progress details and the audit table. The font's tnum widens punctuation, so never set it on body text or titles.

**The Sentence Case Rule.** Column heads, labels and section titles are sentence case at normal tracking. No uppercase small-label tier exists.

## Layout

A single centred column, 1120px max width with 32px side padding (16px on phones) and 64px bottom padding. A 64px top bar carries the product name, the primary nav and the Import action at the far right, separated from content by a hairline.

Spacing follows a 4px grid (s1 to s16). Page heads put the title left and page actions bottom-aligned to the right, 24px above content. List rows breathe at 16px vertical padding; highlights at 24px.

- **Library list:** a four-column grid (44px cover, flexible title block, right-aligned count, 68px actions) under a quiet caption-size column head.
- **Book page:** highlights column plus a 300px sticky aside (tags, metadata), 48px gap. Below 960px the aside moves above the highlights and flows into its own grid.
- **Settings:** a 200px sticky vertical tab list beside the panel. Below 960px the tabs become a horizontally scrolling strip with a hairline under it.
- **Vocabulary:** an auto-fill grid of 280px minimum columns, ruled like the lists.
- **Phones (640px and under):** the top bar wraps and the nav moves to its own scrolling row; book headers stack; highlight actions drop below the text; the audit table hides columns from the fourth on.

**The Ruled Sheet Rule.** Lists are ruled, never boxed: a 1px ink rule opens the list, 1px hairlines close each row.

## Elevation & Depth

The sheet is flat. Depth appears only where something actually floats above it: the delete menu, tag suggestions and the word popover use one soft two-layer shadow with a hairline border and 10px corners. Book covers carry a 1px contact shadow so thin white covers don't dissolve into the page. Everything else (cards, notices, panels) is drawn with a hairline or a tint, not lifted.

### Shadow Vocabulary
- **Pop** (`box-shadow: 0 8px 24px rgba(22, 24, 29, 0.12), 0 2px 6px rgba(22, 24, 29, 0.08)`): floating menus and popovers only.
- **Cover contact** (`box-shadow: 0 1px 2px rgba(22, 24, 29, 0.16)`): book cover images.

**The Float-Only Rule.** A shadow means "this is above the page and will go away". Resting surfaces never get one.

## Shapes

Gently softened rectangles (6px) for every control: buttons, inputs, icon buttons, nav items, notices, menu items. Containers that hold several controls (profile cards, menus, popovers, callback results) step up to 10px. Tags and tag filters are full pills (999px); badges are tighter (4px). Covers use 3px. The marker swipe is the one irregular shape: an asymmetric hand-drawn radius with feathered ends.

Missing covers become a white 44x62 tile with a 1.5px ink border and the title's initial in bold, not a grey placeholder or icon.

## Components

### Buttons
Decisive and quiet: one solid ink button per area, everything else outlined.
- **Shape:** 6px corners, 38px tall (32px small, 44px on auth forms), 13px semibold label.
- **Primary:** ink fill, white label (Import, Refresh metadata, Sign in). Hover lightens to soft ink.
- **Secondary:** white fill, strong hairline stroke, ink label. Hover darkens the stroke to ink.
- **Danger:** red label and red stroke; hover adds the red tint.
- **Icon buttons:** 32px, borderless, faint grey icon; hover turns the icon ink on a sunk-paper wash. Delete icons hover red. On the Book header they gain a hairline stroke at 38px.
- **Focus:** a 2px ink outline, 2px offset, on every focusable element.
- **Disabled:** 45% opacity.

### Chips
- **Tag filter:** 28px pill, strong hairline, ink text, with an inline remove control. The active filter fills ink with white text.
- **Tag chip:** the same pill on Book and highlights, with a small round remove that hovers red.
- **Badges:** 12px caption in a 4px outlined lozenge, grey by default; success, info, failed and admin variants change only text and stroke colour.

### Notices
- **Style:** 12px x 16px padding, 6px corners, 13px text on a tint (sunk paper, success, error, warning). No borders, no icons required.
- **Demo banner:** full-width warning tint across the top of the page.

### Inputs / Fields
- **Style:** white fill, strong hairline stroke, 6px corners, 38px tall (the main search is 44px with 17px text). Faint grey placeholders.
- **Focus:** the stroke turns ink and a 3px Highlighter Yellow halo at 60% appears. The browser outline is removed only here, because the halo replaces it.
- **Checkboxes and radios:** native, tinted ink.

### Navigation
- **Top bar:** product name (17px bold) then nav items (15px medium, muted). Hover turns ink. The active item turns ink semibold and sits on a yellow marker swipe.
- **Settings tabs:** icon plus label, muted; hover adds a sunk-paper wash; the active tab carries the same yellow swipe.
- **Pagination:** 36px squares, 6px corners; the current page fills ink.

### Library row
The signature list item: cover, title (17px semibold) and author, optional `#tag` links in muted text, then a right-aligned count (22px bold figure over a 13px "highlights" caption) and two icon actions. Hovering a row washes it nearly white and draws a yellow underline sweep across the title.

### Highlight (signature)
The reader's passage at 17px/1.75, wrapped in a marker swipe in the reader's own colour, with the favourite heart and delete to the right. Below: notes in soft ink against a 1px ink left rule, then the page/location in muted text and an "Add tag" disclosure. With motion allowed, the first three swipes draw in left to right (0.6s, staggered 80ms). The favourite heart is ink: outlined when off, filled when on.

## Do's and Don'ts

### Do:
- **Do** draw every highlight as a marker swipe in the reader's stored colour, mapped to the nearest of the five markers.
- **Do** keep one ink-filled primary button per region; make the rest outlined secondaries or icon buttons.
- **Do** open lists with an ink rule and separate rows with hairlines.
- **Do** right-align counts and set them in tabular figures.
- **Do** use the system monospace for codes (ISBNs, tokens, IDs) and nowhere else.
- **Do** put every style in `static/style.css` against tokens from `static/tokens.css`.
- **Do** keep fonts and assets self-hosted so the app works offline.

### Don't:
- **Don't** use pink, blue, orange or green for anything other than a reader's highlight: no source labels, status, categories or decoration.
- **Don't** colour the favourite heart; it is ink.
- **Don't** add a dark theme or a tinted page ground; the sheet is white.
- **Don't** box list rows into cards on a grey background.
- **Don't** set tabular figures on running text or titles.
- **Don't** use inline `style=` attributes or per-template `<style>` blocks.
- **Don't** load fonts or scripts from a CDN.
- **Don't** put shadows on resting surfaces.
