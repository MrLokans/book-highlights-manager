package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

type mockBookReaderForCovers struct {
	book *entities.Book
}

func (m *mockBookReaderForCovers) GetAllBooks() ([]entities.Book, error) { return nil, nil }
func (m *mockBookReaderForCovers) GetBookByID(_ uint) (*entities.Book, error) {
	if m.book == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return m.book, nil
}
func (m *mockBookReaderForCovers) GetBookByTitleAndAuthor(_, _ string) (*entities.Book, error) {
	return nil, nil
}
func (m *mockBookReaderForCovers) SearchBooks(_ string) ([]entities.Book, error) { return nil, nil }

func TestCoversController_GetCover_InvalidID(t *testing.T) {
	reader := &mockBookReaderForCovers{}
	controller := NewCoversController(nil, reader)

	router := newTestRouter(t)
	router.GET("/api/books/:id/cover", controller.GetCover)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/books/abc/cover", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCoversController_GetCover_BookNotFound(t *testing.T) {
	reader := &mockBookReaderForCovers{}
	controller := NewCoversController(nil, reader)

	router := newTestRouter(t)
	router.GET("/api/books/:id/cover", controller.GetCover)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/books/999/cover", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCoversController_GetCover_NoCoverURL(t *testing.T) {
	reader := &mockBookReaderForCovers{book: &entities.Book{Title: "No Cover"}}
	controller := NewCoversController(nil, reader)

	router := newTestRouter(t)
	router.GET("/api/books/:id/cover", controller.GetCover)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/books/1/cover", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
