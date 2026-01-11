package applebooks

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mrlokans/assistant/internal/entities"
)

// createTestDatabases creates mock Apple Books databases for testing
func createTestDatabases(t *testing.T) (annotationDBPath, bookDBPath string, cleanup func()) {
	t.Helper()

	tempDir := t.TempDir()
	annotationDBPath = filepath.Join(tempDir, "annotations.sqlite")
	bookDBPath = filepath.Join(tempDir, "books.sqlite")

	// Create annotation database
	annotationDB, err := sql.Open("sqlite3", annotationDBPath)
	if err != nil {
		t.Fatalf("Failed to create annotation database: %v", err)
	}
	defer annotationDB.Close()

	// Create annotation table
	_, err = annotationDB.Exec(`
		CREATE TABLE ZAEANNOTATION (
			Z_PK INTEGER PRIMARY KEY,
			ZANNOTATIONASSETID TEXT,
			ZANNOTATIONLOCATION TEXT,
			ZANNOTATIONSELECTEDTEXT TEXT,
			ZANNOTATIONNOTE TEXT,
			ZANNOTATIONREPRESENTATIVETEXT TEXT,
			ZFUTUREPROOFING5 TEXT,
			ZANNOTATIONSTYLE INTEGER,
			ZANNOTATIONMODIFICATIONDATE REAL,
			ZPLLOCATIONRANGESTART INTEGER,
			ZANNOTATIONDELETED INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create annotation table: %v", err)
	}

	// Create book database
	bookDB, err := sql.Open("sqlite3", bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create book database: %v", err)
	}
	defer bookDB.Close()

	// Create book asset table
	_, err = bookDB.Exec(`
		CREATE TABLE ZBKLIBRARYASSET (
			Z_PK INTEGER PRIMARY KEY,
			ZASSETID TEXT,
			ZTITLE TEXT,
			ZAUTHOR TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create book table: %v", err)
	}

	cleanup = func() {
		// tempDir is automatically cleaned up by t.TempDir()
	}

	return annotationDBPath, bookDBPath, cleanup
}

// insertTestBook inserts a test book into the book database
func insertTestBook(t *testing.T, dbPath, assetID, title, author string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open book database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO ZBKLIBRARYASSET (ZASSETID, ZTITLE, ZAUTHOR)
		VALUES (?, ?, ?)
	`, assetID, title, author)
	if err != nil {
		t.Fatalf("Failed to insert test book: %v", err)
	}
}

// insertTestAnnotation inserts a test annotation into the annotation database
func insertTestAnnotation(t *testing.T, dbPath, assetID, text, note, chapter string, style, locationStart int, modifiedDate float64) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open annotation database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		INSERT INTO ZAEANNOTATION (
			ZANNOTATIONASSETID, ZANNOTATIONSELECTEDTEXT, ZANNOTATIONNOTE,
			ZFUTUREPROOFING5, ZANNOTATIONSTYLE, ZPLLOCATIONRANGESTART,
			ZANNOTATIONMODIFICATIONDATE, ZANNOTATIONDELETED
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0)
	`, assetID, text, note, chapter, style, locationStart, modifiedDate)
	if err != nil {
		t.Fatalf("Failed to insert test annotation: %v", err)
	}
}

func TestNewAppleBooksReader_CustomPaths(t *testing.T) {
	annotationDBPath, bookDBPath, cleanup := createTestDatabases(t)
	defer cleanup()

	reader, err := NewAppleBooksReader(annotationDBPath, bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	if reader.GetAnnotationDBPath() != annotationDBPath {
		t.Errorf("Expected annotation path %s, got %s", annotationDBPath, reader.GetAnnotationDBPath())
	}
	if reader.GetBookDBPath() != bookDBPath {
		t.Errorf("Expected book path %s, got %s", bookDBPath, reader.GetBookDBPath())
	}
}

func TestNewAppleBooksReader_NonExistentPath(t *testing.T) {
	_, err := NewAppleBooksReader("/nonexistent/path.sqlite", "/another/nonexistent.sqlite")
	if err == nil {
		t.Error("Expected error for non-existent paths, got nil")
	}
}

func TestGetHighlights_Empty(t *testing.T) {
	annotationDBPath, bookDBPath, cleanup := createTestDatabases(t)
	defer cleanup()

	reader, err := NewAppleBooksReader(annotationDBPath, bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	highlights, err := reader.GetHighlights()
	if err != nil {
		t.Fatalf("Failed to get highlights: %v", err)
	}

	if len(highlights) != 0 {
		t.Errorf("Expected 0 highlights, got %d", len(highlights))
	}
}

func TestGetHighlights_WithData(t *testing.T) {
	annotationDBPath, bookDBPath, cleanup := createTestDatabases(t)
	defer cleanup()

	// Insert test data
	insertTestBook(t, bookDBPath, "book-1", "Test Book", "Test Author")
	insertTestAnnotation(t, annotationDBPath, "book-1", "This is a highlight", "My note", "Chapter 1", 3, 100, 694224000.0)

	reader, err := NewAppleBooksReader(annotationDBPath, bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	highlights, err := reader.GetHighlights()
	if err != nil {
		t.Fatalf("Failed to get highlights: %v", err)
	}

	if len(highlights) != 1 {
		t.Fatalf("Expected 1 highlight, got %d", len(highlights))
	}

	h := highlights[0]
	if h.Title != "Test Book" {
		t.Errorf("Expected title 'Test Book', got '%s'", h.Title)
	}
	if h.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", h.Author)
	}
	if h.SelectedText != "This is a highlight" {
		t.Errorf("Expected text 'This is a highlight', got '%s'", h.SelectedText)
	}
	if h.Note != "My note" {
		t.Errorf("Expected note 'My note', got '%s'", h.Note)
	}
	if h.Chapter != "Chapter 1" {
		t.Errorf("Expected chapter 'Chapter 1', got '%s'", h.Chapter)
	}
	if h.Style != 3 {
		t.Errorf("Expected style 3, got %d", h.Style)
	}
	if h.LocationStart != 100 {
		t.Errorf("Expected location start 100, got %d", h.LocationStart)
	}
}

func TestGetBooks_GroupedByBook(t *testing.T) {
	annotationDBPath, bookDBPath, cleanup := createTestDatabases(t)
	defer cleanup()

	// Insert two books with multiple highlights each
	insertTestBook(t, bookDBPath, "book-1", "Book One", "Author One")
	insertTestBook(t, bookDBPath, "book-2", "Book Two", "Author Two")

	insertTestAnnotation(t, annotationDBPath, "book-1", "Highlight 1 from book 1", "", "", 3, 100, 0)
	insertTestAnnotation(t, annotationDBPath, "book-1", "Highlight 2 from book 1", "", "", 3, 200, 0)
	insertTestAnnotation(t, annotationDBPath, "book-2", "Highlight 1 from book 2", "", "", 3, 50, 0)

	reader, err := NewAppleBooksReader(annotationDBPath, bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	books, err := reader.GetBooks()
	if err != nil {
		t.Fatalf("Failed to get books: %v", err)
	}

	if len(books) != 2 {
		t.Fatalf("Expected 2 books, got %d", len(books))
	}

	// Find book one
	var bookOne, bookTwo *entities.Book
	for i := range books {
		if books[i].Title == "Book One" {
			bookOne = &books[i]
		} else if books[i].Title == "Book Two" {
			bookTwo = &books[i]
		}
	}

	if bookOne == nil {
		t.Fatal("Book One not found")
	}
	if bookTwo == nil {
		t.Fatal("Book Two not found")
	}

	if len(bookOne.Highlights) != 2 {
		t.Errorf("Expected 2 highlights for Book One, got %d", len(bookOne.Highlights))
	}
	if len(bookTwo.Highlights) != 1 {
		t.Errorf("Expected 1 highlight for Book Two, got %d", len(bookTwo.Highlights))
	}
}

func TestGetBooks_SourceMetadata(t *testing.T) {
	annotationDBPath, bookDBPath, cleanup := createTestDatabases(t)
	defer cleanup()

	insertTestBook(t, bookDBPath, "book-1", "Test Book", "Test Author")
	insertTestAnnotation(t, annotationDBPath, "book-1", "Test highlight", "", "", 3, 100, 0)

	reader, err := NewAppleBooksReader(annotationDBPath, bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	books, err := reader.GetBooks()
	if err != nil {
		t.Fatalf("Failed to get books: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(books))
	}

	book := books[0]
	if book.Source.Name != "apple_books" {
		t.Errorf("Expected source name 'apple_books', got '%s'", book.Source.Name)
	}
	if book.Source.DisplayName != "Apple Books" {
		t.Errorf("Expected source display name 'Apple Books', got '%s'", book.Source.DisplayName)
	}

	if len(book.Highlights) != 1 {
		t.Fatalf("Expected 1 highlight, got %d", len(book.Highlights))
	}

	h := book.Highlights[0]
	if h.Source.Name != "apple_books" {
		t.Errorf("Expected highlight source name 'apple_books', got '%s'", h.Source.Name)
	}
}

func TestConvertAnnotationStyle(t *testing.T) {
	tests := []struct {
		style    int
		expected entities.HighlightStyle
	}{
		{1, entities.HighlightStyleUnderline},
		{2, entities.HighlightStyleHighlight}, // Green
		{3, entities.HighlightStyleHighlight}, // Blue
		{4, entities.HighlightStyleHighlight}, // Yellow
		{5, entities.HighlightStyleHighlight}, // Pink
		{6, entities.HighlightStyleHighlight}, // Purple
		{99, entities.HighlightStyleHighlight}, // Unknown
	}

	for _, tt := range tests {
		result := convertAnnotationStyle(tt.style)
		if result != tt.expected {
			t.Errorf("convertAnnotationStyle(%d) = %s, expected %s", tt.style, result, tt.expected)
		}
	}
}

func TestGetColorForStyle(t *testing.T) {
	tests := []struct {
		style    int
		expected string
	}{
		{1, ""},          // Underline
		{2, "#00FF00"},   // Green
		{3, "#0000FF"},   // Blue
		{4, "#FFFF00"},   // Yellow
		{5, "#FF69B4"},   // Pink
		{6, "#800080"},   // Purple
		{99, "#FFFF00"},  // Unknown defaults to yellow
	}

	for _, tt := range tests {
		result := getColorForStyle(tt.style)
		if result != tt.expected {
			t.Errorf("getColorForStyle(%d) = %s, expected %s", tt.style, result, tt.expected)
		}
	}
}

func TestCoreDataTimestampConversion(t *testing.T) {
	// Core Data epoch is 2001-01-01 00:00:00 UTC
	// Test: 694224000.0 seconds after epoch = 2023-01-01 00:00:00 UTC
	timestamp := 694224000.0
	expected := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	result := coreDataEpoch.Add(time.Duration(timestamp * float64(time.Second)))

	if !result.Equal(expected) {
		t.Errorf("Core Data timestamp conversion: expected %v, got %v", expected, result)
	}
}

func TestGetBooks_SkipsEmptyHighlights(t *testing.T) {
	annotationDBPath, bookDBPath, cleanup := createTestDatabases(t)
	defer cleanup()

	insertTestBook(t, bookDBPath, "book-1", "Test Book", "Test Author")
	// Insert annotation with empty text and no note
	insertTestAnnotation(t, annotationDBPath, "book-1", "", "", "", 3, 100, 0)
	// Insert annotation with valid text
	insertTestAnnotation(t, annotationDBPath, "book-1", "Valid highlight", "", "", 3, 200, 0)

	reader, err := NewAppleBooksReader(annotationDBPath, bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	books, err := reader.GetBooks()
	if err != nil {
		t.Fatalf("Failed to get books: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(books))
	}

	// Should only have the valid highlight
	if len(books[0].Highlights) != 1 {
		t.Errorf("Expected 1 highlight (skipping empty), got %d", len(books[0].Highlights))
	}
}

func TestGetBooks_NoteOnlyHighlight(t *testing.T) {
	annotationDBPath, bookDBPath, cleanup := createTestDatabases(t)
	defer cleanup()

	insertTestBook(t, bookDBPath, "book-1", "Test Book", "Test Author")
	// Insert annotation with no text but has a note
	insertTestAnnotation(t, annotationDBPath, "book-1", "", "This is just a note", "", 3, 100, 0)

	reader, err := NewAppleBooksReader(annotationDBPath, bookDBPath)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	books, err := reader.GetBooks()
	if err != nil {
		t.Fatalf("Failed to get books: %v", err)
	}

	if len(books) != 1 {
		t.Fatalf("Expected 1 book, got %d", len(books))
	}

	// Should have the note-only highlight
	if len(books[0].Highlights) != 1 {
		t.Errorf("Expected 1 highlight (note-only), got %d", len(books[0].Highlights))
	}

	h := books[0].Highlights[0]
	if h.Note != "This is just a note" {
		t.Errorf("Expected note 'This is just a note', got '%s'", h.Note)
	}
}
