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
	"github.com/mrlokans/assistant/internal/exporters"
)

type UIController struct {
	Exporter *exporters.DatabaseMarkdownExporter
}

func NewUIController(exporter *exporters.DatabaseMarkdownExporter) *UIController {
	return &UIController{
		Exporter: exporter,
	}
}

// BooksPage renders the main books list page
func (controller *UIController) BooksPage(c *gin.Context) {
	books, err := controller.Exporter.GetAllBooksFromDatabase()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error loading books: %s", err.Error())
		return
	}

	totalHighlights := 0
	for _, book := range books {
		totalHighlights += len(book.Highlights)
	}

	c.HTML(http.StatusOK, "books", gin.H{
		"Books":           books,
		"TotalBooks":      len(books),
		"TotalHighlights": totalHighlights,
	})
}

// BookPage renders the detail page for a single book
func (controller *UIController) BookPage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid book ID")
		return
	}

	book, err := controller.Exporter.GetBookByIDFromDatabase(uint(id))
	if err != nil {
		c.String(http.StatusNotFound, "Book not found")
		return
	}

	c.HTML(http.StatusOK, "book", gin.H{
		"Book": book,
	})
}

// SearchBooks handles HTMX search requests and returns partial HTML
func (controller *UIController) SearchBooks(c *gin.Context) {
	query := c.Query("q")

	var books []any
	var err error

	if query == "" {
		allBooks, e := controller.Exporter.GetAllBooksFromDatabase()
		err = e
		for _, b := range allBooks {
			books = append(books, b)
		}
	} else {
		searchedBooks, e := controller.Exporter.SearchBooks(query)
		err = e
		for _, b := range searchedBooks {
			books = append(books, b)
		}
	}

	if err != nil {
		c.String(http.StatusInternalServerError, "Error searching books")
		return
	}

	c.HTML(http.StatusOK, "book-list", books)
}

// DownloadMarkdown generates and downloads a markdown file for a book
func (controller *UIController) DownloadMarkdown(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid book ID")
		return
	}

	book, err := controller.Exporter.GetBookByIDFromDatabase(uint(id))
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

// DownloadAllMarkdown generates and downloads a ZIP archive with all books as markdown files
func (controller *UIController) DownloadAllMarkdown(c *gin.Context) {
	books, err := controller.Exporter.GetAllBooksFromDatabase()
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
		writer.Write([]byte(markdown))
	}

	zipWriter.Close()

	timestamp := time.Now().Format("2006-01-02")
	zipFilename := fmt.Sprintf("highlights-%s.zip", timestamp)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))
	c.Header("Content-Type", "application/zip")
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
