// Constants matching e2e/fixtures/generate-seed/main.go.
// Keep in sync when the seed changes.

export const SEED = {
  totalBooks: 5,
  books: {
    artOfTesting: {
      title: 'The Art of Testing',
      author: 'Jane Smith',
      highlights: 22,
      favouritedHighlights: 2,
    },
    briefThoughts: {
      title: 'Brief Thoughts',
      author: 'John Doe',
      highlights: 1,
      singleHighlightText: 'The only highlight in this book',
    },
    emptyReads: {
      title: 'Empty Reads',
      author: 'Alice Wonder',
      highlights: 0,
    },
    favouriteCollection: {
      title: 'Favourite Collection',
      author: 'Bob Builder',
      highlights: 5,
      favouritedHighlights: 3,
    },
    anonymousWisdom: {
      title: 'Anonymous Wisdom',
      author: '',
      highlights: 1,
    },
  },
  tags: ['fiction', 'nonfiction', 'favourite'],
  vocabulary: ['ephemeral', 'ubiquitous', 'serendipity'],
  auditEvents: 1,
} as const;
