package http

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database/books"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/exporters"
)

// BookLister retrieves a filtered, sorted, paginated list of books.
type BookLister interface {
	ListBooks(opts books.ListBooksOptions) (*books.ListBooksResult, error)
}

// UIController handles HTML page rendering with HTMX support.
type UIController struct {
	reader          exporters.BookReader
	bookLister      BookLister
	tagStore        TagStore
	vocabularyStore VocabularyStore
}

// NewUIController creates a UI controller.
func NewUIController(reader exporters.BookReader, bookLister BookLister, tagStore TagStore, vocabularyStore VocabularyStore) *UIController {
	return &UIController{
		reader:          reader,
		bookLister:      bookLister,
		tagStore:        tagStore,
		vocabularyStore: vocabularyStore,
	}
}

// BooksPage renders the main books listing page.
func (controller *UIController) BooksPage(c *gin.Context) {
	userID := GetUserID(c)

	opts := books.ListBooksOptions{
		UserID:  userID,
		Query:   c.Query("q"),
		Sort:    c.DefaultQuery("sort", "date_desc"),
		Page:    ParsePageParam(c),
		PerPage: 20,
	}

	if tagIDStr := c.Query("tag"); tagIDStr != "" {
		tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
		if err == nil {
			opts.TagID = uint(tagID)
		}
	}

	result, err := controller.bookLister.ListBooks(opts)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error loading books: %s", err.Error())
		return
	}

	var tags []entities.Tag
	if controller.tagStore != nil {
		tags, _ = controller.tagStore.GetTagsForUser(userID)
	}

	RenderPage(c, http.StatusOK, "books", gin.H{
		"ActivePage":    "books",
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

// BookPage renders a single book detail page.
func (controller *UIController) BookPage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid book ID")
		return
	}

	book, err := controller.reader.GetBookByID(uint(id))
	if err != nil {
		c.String(http.StatusNotFound, "Book not found")
		return
	}

	RenderPage(c, http.StatusOK, "book", gin.H{
		"ActivePage": "books",
		"Book":       book,
	})
}

// SearchBooks handles HTMX search requests, returning the books-content partial.
func (controller *UIController) SearchBooks(c *gin.Context) {
	userID := GetUserID(c)

	opts := books.ListBooksOptions{
		UserID:  userID,
		Query:   c.Query("q"),
		Sort:    c.DefaultQuery("sort", "date_desc"),
		Page:    ParsePageParam(c),
		PerPage: 20,
	}

	if tagIDStr := c.Query("tag"); tagIDStr != "" {
		tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
		if err == nil {
			opts.TagID = uint(tagID)
		}
	}

	result, err := controller.bookLister.ListBooks(opts)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{"Error": "Failed to load books"})
		return
	}

	var tags []entities.Tag
	if controller.tagStore != nil {
		tags, _ = controller.tagStore.GetTagsForUser(userID)
	}

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

// DownloadMarkdown exports a single book as a markdown file download.
func (controller *UIController) DownloadMarkdown(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid book ID")
		return
	}

	book, err := controller.reader.GetBookByID(uint(id))
	if err != nil {
		c.String(http.StatusNotFound, "Book not found")
		return
	}

	markdown := exporters.GenerateMarkdown(book)

	// Sanitize filename
	filename := strings.ReplaceAll(book.Title, "/", "-")
	filename = strings.ReplaceAll(filename, "\\", "-")
	filename = fmt.Sprintf("%s.md", filename)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "text/markdown; charset=utf-8")
	c.String(http.StatusOK, markdown)
}

// DownloadAllMarkdown exports all books as a zip of markdown files.
func (controller *UIController) DownloadAllMarkdown(c *gin.Context) {
	books, err := controller.reader.GetAllBooks()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error loading books: %s", err.Error())
		return
	}

	// Create ZIP in memory
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	for _, book := range books {
		markdown := exporters.GenerateMarkdown(&book)

		// Determine folder based on source
		sourceFolder := "unknown"
		if book.Source.Name != "" {
			sourceFolder = book.Source.Name
		}

		// Sanitize filename
		filename := strings.ReplaceAll(book.Title, "/", "-")
		filename = strings.ReplaceAll(filename, "\\", "-")
		filename = strings.ReplaceAll(filename, ":", "-")
		filepath := fmt.Sprintf("highlights/%s/%s.md", sourceFolder, filename)

		writer, err := zipWriter.Create(filepath)
		if err != nil {
			continue
		}
		_, _ = writer.Write([]byte(markdown))
	}

	// Add vocabulary file if store is available
	if controller.vocabularyStore != nil {
		words, _, err := controller.vocabularyStore.GetAllWords(0, 0, 0)
		if err == nil && len(words) > 0 {
			vocabularyMarkdown := exporters.GenerateVocabularyMarkdown(words)
			writer, err := zipWriter.Create("highlights/vocabulary.md")
			if err == nil {
				_, _ = writer.Write([]byte(vocabularyMarkdown))
			}
		}
	}

	_ = zipWriter.Close() //nolint:gosec // buffer already written

	timestamp := time.Now().Format("2006-01-02")
	zipFilename := fmt.Sprintf("highlights-%s.zip", timestamp)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))
	c.Header("Content-Type", "application/zip")
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
