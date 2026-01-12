package services

import "github.com/mrlokans/assistant/internal/entities"

// BookReader provides read-only access to books and highlights.
// Use this interface when you only need to query books.
type BookReader interface {
	GetAllBooks() ([]entities.Book, error)
	GetBookByID(id uint) (*entities.Book, error)
	GetBookByTitleAndAuthor(title, author string) (*entities.Book, error)
	SearchBooks(query string) ([]entities.Book, error)
}

// BookExporter handles exporting books to storage (database + files).
// Use this interface when you need to persist books.
type BookExporter interface {
	Export(books []entities.Book) (ExportResult, error)
}

// ExportResult contains the outcome of an export operation.
type ExportResult struct {
	BooksProcessed      int
	HighlightsProcessed int
	BooksFailed         int
	HighlightsFailed    int
}

// ImportResult contains the outcome of an import operation.
type ImportResult struct {
	BooksProcessed      int
	HighlightsProcessed int
	BooksFailed         int
	HighlightsFailed    int
}
