// Centralised CSS selectors for E2E tests.
// Update here when class names change in templates.

export const Sel = {
  // Books list
  bookCard: '.book-card',
  bookTitle: '.book-title',
  bookLink: '.book-link',
  downloadAllBtn: '.download-all-btn',

  // Book detail
  highlight: '.highlight',
  bookTagsSection: '#book-tags-section',
  bookActions: '.book-actions',
  downloadBtn: '.book-actions .download-btn',
  favouriteBtnActive: '.favourite-btn-active',
  author: '.author',

  // Search
  searchInput: '.search-box input[name="q"]',
  sortSelect: '#sort-select',
  tagFilterChip: '.tag-filter-chip',

  // Auth
  authSubmit: '.auth-submit',
  authError: '.auth-error',
  usernameInput: '#username',
  passwordInput: '#password',
  userName: '.user-name',
  logoutLink: '.logout-link',

  // Settings / Import
  kindleFileInput: '#kindle-clippings-file',
  readwiseCsvFileInput: '#readwise-csv-file',
  kindleResultContainer: '#kindle-result-container:not(:empty)',
  readwiseCsvResultContainer: '#readwise-csv-result-container:not(:empty)',

  // Favourites
  favouriteHighlight: '.favourite-highlight',
  stats: '.stats',

  // Vocabulary
  wordCard: '.word-card',
  wordText: '.word-text',

  // Audit
  auditTable: '.audit-table',
  eventTypeBadge: '.event-type-badge',

  // Common
  pageTitle: '.page-title',
} as const;
