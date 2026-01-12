package metadata

import (
	"context"
	"errors"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
)

type mockMetadataProvider struct {
	searchByISBNResult  *BookMetadata
	searchByISBNError   error
	searchByTitleResult *BookMetadata
	searchByTitleError  error
}

func (m *mockMetadataProvider) SearchByISBN(ctx context.Context, isbn string) (*BookMetadata, error) {
	return m.searchByISBNResult, m.searchByISBNError
}

func (m *mockMetadataProvider) SearchByTitle(ctx context.Context, title, author string) (*BookMetadata, error) {
	return m.searchByTitleResult, m.searchByTitleError
}

type mockBookUpdater struct {
	book                 *entities.Book
	getBookError         error
	updateError          error
	updatedFields        map[string]any
	booksMissingMetadata []entities.Book
}

func (m *mockBookUpdater) GetBookByID(id uint) (*entities.Book, error) {
	if m.getBookError != nil {
		return nil, m.getBookError
	}
	return m.book, nil
}

func (m *mockBookUpdater) UpdateBookMetadata(id uint, fields BookUpdateFields) error {
	if m.updateError != nil {
		return m.updateError
	}
	m.updatedFields = make(map[string]any)
	if fields.ISBN != nil {
		m.updatedFields["isbn"] = *fields.ISBN
		m.book.ISBN = *fields.ISBN
	}
	if fields.CoverURL != nil {
		m.updatedFields["cover_url"] = *fields.CoverURL
		m.book.CoverURL = *fields.CoverURL
	}
	if fields.Publisher != nil {
		m.updatedFields["publisher"] = *fields.Publisher
		m.book.Publisher = *fields.Publisher
	}
	if fields.PublicationYear != nil {
		m.updatedFields["publication_year"] = *fields.PublicationYear
		m.book.PublicationYear = *fields.PublicationYear
	}
	return nil
}

func (m *mockBookUpdater) GetBooksMissingMetadata() ([]entities.Book, error) {
	return m.booksMissingMetadata, nil
}

func TestEnrichBook_WithISBN(t *testing.T) {
	book := &entities.Book{
		ID:     1,
		Title:  "Effective Java",
		Author: "Joshua Bloch",
		ISBN:   "9780134685991",
	}

	provider := &mockMetadataProvider{
		searchByISBNResult: &BookMetadata{
			Title:           "Effective Java",
			Author:          "Joshua Bloch",
			ISBN:            "9780134685991",
			Publisher:       "Addison-Wesley",
			PublicationYear: 2018,
			CoverURL:        "https://covers.openlibrary.org/b/isbn/9780134685991-L.jpg",
		},
	}

	updater := &mockBookUpdater{book: book}
	enricher := NewEnricher(provider, updater)

	result, err := enricher.EnrichBook(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichBook failed: %v", err)
	}

	if result.SearchMethod != "isbn" {
		t.Errorf("expected search method 'isbn', got %q", result.SearchMethod)
	}

	if result.Book.Publisher != "Addison-Wesley" {
		t.Errorf("expected publisher to be updated to 'Addison-Wesley', got %q", result.Book.Publisher)
	}

	if result.Book.PublicationYear != 2018 {
		t.Errorf("expected publication year 2018, got %d", result.Book.PublicationYear)
	}

	if len(result.FieldsUpdated) == 0 {
		t.Error("expected fields to be updated")
	}
}

func TestEnrichBook_FallbackToTitle(t *testing.T) {
	book := &entities.Book{
		ID:     1,
		Title:  "Clean Code",
		Author: "Robert Martin",
		// No ISBN
	}

	provider := &mockMetadataProvider{
		searchByTitleResult: &BookMetadata{
			Title:           "Clean Code",
			Author:          "Robert C. Martin",
			ISBN:            "9780132350884",
			Publisher:       "Prentice Hall",
			PublicationYear: 2008,
			CoverURL:        "https://covers.openlibrary.org/b/isbn/9780132350884-L.jpg",
		},
	}

	updater := &mockBookUpdater{book: book}
	enricher := NewEnricher(provider, updater)

	result, err := enricher.EnrichBook(context.Background(), 1)
	if err != nil {
		t.Fatalf("EnrichBook failed: %v", err)
	}

	if result.SearchMethod != "title" {
		t.Errorf("expected search method 'title', got %q", result.SearchMethod)
	}

	// ISBN should be updated since book didn't have one
	if result.Book.ISBN != "9780132350884" {
		t.Errorf("expected ISBN to be updated, got %q", result.Book.ISBN)
	}
}

func TestEnrichBook_BookNotFound(t *testing.T) {
	provider := &mockMetadataProvider{}
	updater := &mockBookUpdater{
		getBookError: errors.New("book not found"),
	}
	enricher := NewEnricher(provider, updater)

	_, err := enricher.EnrichBook(context.Background(), 999)
	if err == nil {
		t.Error("expected error when book not found")
	}
}

func TestEnrichBook_SearchFailed(t *testing.T) {
	book := &entities.Book{
		ID:     1,
		Title:  "Unknown Book",
		Author: "Unknown Author",
	}

	provider := &mockMetadataProvider{
		searchByTitleError: errors.New("no results found"),
	}

	updater := &mockBookUpdater{book: book}
	enricher := NewEnricher(provider, updater)

	_, err := enricher.EnrichBook(context.Background(), 1)
	if err == nil {
		t.Error("expected error when search fails")
	}
}

func TestEnrichBookWithISBN(t *testing.T) {
	book := &entities.Book{
		ID:     1,
		Title:  "Some Book",
		Author: "Some Author",
	}

	provider := &mockMetadataProvider{
		searchByISBNResult: &BookMetadata{
			Title:           "Some Book",
			Publisher:       "Test Publisher",
			PublicationYear: 2020,
			CoverURL:        "https://example.com/cover.jpg",
		},
	}

	updater := &mockBookUpdater{book: book}
	enricher := NewEnricher(provider, updater)

	result, err := enricher.EnrichBookWithISBN(context.Background(), 1, "1234567890")
	if err != nil {
		t.Fatalf("EnrichBookWithISBN failed: %v", err)
	}

	if result.SearchMethod != "isbn" {
		t.Errorf("expected search method 'isbn', got %q", result.SearchMethod)
	}

	if result.Book.ISBN != "1234567890" {
		t.Errorf("expected ISBN '1234567890', got %q", result.Book.ISBN)
	}
}

func TestEnrichBookWithISBN_FallbackToTitle(t *testing.T) {
	book := &entities.Book{
		ID:     1,
		Title:  "Some Book",
		Author: "Some Author",
	}

	provider := &mockMetadataProvider{
		// ISBN search fails
		searchByISBNError: errors.New("ISBN not found"),
		// Title search succeeds
		searchByTitleResult: &BookMetadata{
			Title:           "Some Book",
			Author:          "Some Author",
			ISBN:            "9876543210",
			Publisher:       "Title Search Publisher",
			PublicationYear: 2021,
			CoverURL:        "https://example.com/title-cover.jpg",
		},
	}

	updater := &mockBookUpdater{book: book}
	enricher := NewEnricher(provider, updater)

	result, err := enricher.EnrichBookWithISBN(context.Background(), 1, "1234567890")
	if err != nil {
		t.Fatalf("EnrichBookWithISBN failed: %v", err)
	}

	// Should fall back to title search
	if result.SearchMethod != "title" {
		t.Errorf("expected search method 'title', got %q", result.SearchMethod)
	}

	// ISBN should be from title search result, not the provided one (since ISBN search failed)
	if result.Book.ISBN != "9876543210" {
		t.Errorf("expected ISBN from title search '9876543210', got %q", result.Book.ISBN)
	}

	if result.Book.Publisher != "Title Search Publisher" {
		t.Errorf("expected publisher 'Title Search Publisher', got %q", result.Book.Publisher)
	}
}

func TestEnrichBookWithISBN_DoesNotSaveISBNOnFailure(t *testing.T) {
	book := &entities.Book{
		ID:     1,
		Title:  "Some Book",
		Author: "Some Author",
	}

	provider := &mockMetadataProvider{
		// ISBN search fails
		searchByISBNError: errors.New("ISBN not found"),
		// Title search also fails
		searchByTitleError: errors.New("no results found"),
	}

	updater := &mockBookUpdater{book: book}
	enricher := NewEnricher(provider, updater)

	_, err := enricher.EnrichBookWithISBN(context.Background(), 1, "1234567890")
	if err == nil {
		t.Fatal("expected error when both searches fail")
	}

	// ISBN should NOT be saved when search fails
	if book.ISBN != "" {
		t.Errorf("ISBN should not be saved when search fails, got %q", book.ISBN)
	}
}

func TestBuildUpdates_OnlyEmptyFields(t *testing.T) {
	book := &entities.Book{
		ID:              1,
		Title:           "Test Book",
		Author:          "Test Author",
		Publisher:       "Existing Publisher", // Already has publisher
		PublicationYear: 2019,                 // Already has year
	}

	metadata := &BookMetadata{
		Publisher:       "New Publisher",    // Should NOT update - book already has one
		PublicationYear: 2020,               // Should NOT update - book already has one
		CoverURL:        "https://cover.jpg", // Should update - book doesn't have one
	}

	enricher := NewEnricher(nil, nil)
	updates, fieldsUpdated := enricher.buildUpdates(book, metadata)

	if updates.Publisher != nil {
		t.Error("publisher should not be updated when book already has one")
	}

	if updates.PublicationYear != nil {
		t.Error("publication year should not be updated when book already has one")
	}

	if updates.CoverURL == nil || *updates.CoverURL != "https://cover.jpg" {
		t.Error("cover URL should be updated")
	}

	found := false
	for _, f := range fieldsUpdated {
		if f == "cover_url" {
			found = true
		}
	}
	if !found {
		t.Error("cover_url should be in fieldsUpdated")
	}
}
