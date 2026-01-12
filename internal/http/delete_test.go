package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
)

type mockDeleteStore struct {
	deletedBookID          uint
	deletedBookPermanently bool
	deletedHighlightID     uint
	deletedHighlightPerm   bool
	bookErr                error
	highlightErr           error
}

func (m *mockDeleteStore) GetBookByID(id uint) (*entities.Book, error) {
	return &entities.Book{ID: id, Title: "Test Book", Author: "Test Author"}, nil
}

func (m *mockDeleteStore) GetHighlightByID(id uint) (*entities.Highlight, error) {
	return &entities.Highlight{ID: id, BookID: 1, Text: "Test highlight"}, nil
}

func (m *mockDeleteStore) DeleteBook(id uint) error {
	m.deletedBookID = id
	return m.bookErr
}

func (m *mockDeleteStore) DeleteBookPermanently(id uint, userID uint) error {
	m.deletedBookID = id
	m.deletedBookPermanently = true
	return m.bookErr
}

func (m *mockDeleteStore) DeleteHighlight(id uint) error {
	m.deletedHighlightID = id
	return m.highlightErr
}

func (m *mockDeleteStore) DeleteHighlightPermanently(id uint, userID uint) error {
	m.deletedHighlightID = id
	m.deletedHighlightPerm = true
	return m.highlightErr
}

func TestDeleteBook(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &mockDeleteStore{}
	controller := NewDeleteController(store)

	router := gin.New()
	router.DELETE("/api/books/:id", controller.DeleteBook)

	req, _ := http.NewRequest("DELETE", "/api/books/123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if store.deletedBookID != 123 {
		t.Errorf("expected book ID 123 to be deleted, got %d", store.deletedBookID)
	}

	if store.deletedBookPermanently {
		t.Error("expected soft delete, not permanent delete")
	}
}

func TestDeleteBookPermanently(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &mockDeleteStore{}
	controller := NewDeleteController(store)

	router := gin.New()
	router.DELETE("/api/books/:id/permanent", controller.DeleteBookPermanently)

	req, _ := http.NewRequest("DELETE", "/api/books/456/permanent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if store.deletedBookID != 456 {
		t.Errorf("expected book ID 456 to be deleted, got %d", store.deletedBookID)
	}

	if !store.deletedBookPermanently {
		t.Error("expected permanent delete")
	}
}

func TestDeleteHighlight(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &mockDeleteStore{}
	controller := NewDeleteController(store)

	router := gin.New()
	router.DELETE("/api/highlights/:id", controller.DeleteHighlight)

	req, _ := http.NewRequest("DELETE", "/api/highlights/789", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if store.deletedHighlightID != 789 {
		t.Errorf("expected highlight ID 789 to be deleted, got %d", store.deletedHighlightID)
	}

	if store.deletedHighlightPerm {
		t.Error("expected soft delete, not permanent delete")
	}
}

func TestDeleteHighlightPermanently(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &mockDeleteStore{}
	controller := NewDeleteController(store)

	router := gin.New()
	router.DELETE("/api/highlights/:id/permanent", controller.DeleteHighlightPermanently)

	req, _ := http.NewRequest("DELETE", "/api/highlights/999/permanent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if store.deletedHighlightID != 999 {
		t.Errorf("expected highlight ID 999 to be deleted, got %d", store.deletedHighlightID)
	}

	if !store.deletedHighlightPerm {
		t.Error("expected permanent delete")
	}
}

func TestDeleteBookInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &mockDeleteStore{}
	controller := NewDeleteController(store)

	router := gin.New()
	router.DELETE("/api/books/:id", controller.DeleteBook)

	req, _ := http.NewRequest("DELETE", "/api/books/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
