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
	reader   exporters.BookReader
	tagStore TagStore
}

func NewUIController(reader exporters.BookReader, tagStore TagStore) *UIController {
	return &UIController{
		reader:   reader,
		tagStore: tagStore,
	}
}

func (controller *UIController) BooksPage(c *gin.Context) {
	tagIDStr := c.Query("tag")
	var selectedTagID uint
	var books []any
	var highlightsCount int
	filterByTag := false

	if tagIDStr != "" && controller.tagStore != nil {
		tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
		if err == nil {
			selectedTagID = uint(tagID)
			filterByTag = true
			filteredBooks, err := controller.tagStore.GetBooksByTag(selectedTagID, 0)
			if err != nil {
				c.String(http.StatusInternalServerError, "Error loading books: %s", err.Error())
				return
			}
			for _, b := range filteredBooks {
				highlightsCount += len(b.Highlights)
				books = append(books, b)
			}
		}
	}

	if !filterByTag {
		allBooks, err := controller.reader.GetAllBooks()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error loading books: %s", err.Error())
			return
		}
		for _, b := range allBooks {
			highlightsCount += len(b.Highlights)
			books = append(books, b)
		}
	}

	// Get all tags for filter UI
	var tags []any
	if controller.tagStore != nil {
		allTags, _ := controller.tagStore.GetTagsForUser(0)
		for _, t := range allTags {
			tags = append(tags, t)
		}
	}

	c.HTML(http.StatusOK, "books", gin.H{
		"Books":           books,
		"TotalBooks":      len(books),
		"TotalHighlights": highlightsCount,
		"Tags":            tags,
		"SelectedTagID":   selectedTagID,
	})
}

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

	c.HTML(http.StatusOK, "book", gin.H{
		"Book": book,
	})
}

func (controller *UIController) SearchBooks(c *gin.Context) {
	query := c.Query("q")

	var books []any
	var err error

	if query == "" {
		allBooks, e := controller.reader.GetAllBooks()
		err = e
		for _, b := range allBooks {
			books = append(books, b)
		}
	} else {
		searchedBooks, e := controller.reader.SearchBooks(query)
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

	zipWriter.Close()

	timestamp := time.Now().Format("2006-01-02")
	zipFilename := fmt.Sprintf("highlights-%s.zip", timestamp)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))
	c.Header("Content-Type", "application/zip")
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
