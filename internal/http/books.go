package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/exporters"
)

type BooksController struct {
	reader exporters.BookReader
}

func NewBooksController(reader exporters.BookReader) *BooksController {
	return &BooksController{
		reader: reader,
	}
}

func (controller *BooksController) GetAllBooks(c *gin.Context) {
	books, err := controller.reader.GetAllBooks()
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"books": books, "count": len(books)})
}

func (controller *BooksController) GetBookByTitleAndAuthor(c *gin.Context) {
	title := c.Query("title")
	author := c.Query("author")

	if title == "" || author == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": "title and author query parameters are required"})
		return
	}

	book, err := controller.reader.GetBookByTitleAndAuthor(title, author)
	if err != nil {
		c.IndentedJSON(http.StatusNotFound, gin.H{"error": "book not found"})
		return
	}

	c.IndentedJSON(http.StatusOK, book)
}

func (controller *BooksController) GetBookStats(c *gin.Context) {
	books, err := controller.reader.GetAllBooks()
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalHighlights := 0
	for _, book := range books {
		totalHighlights += len(book.Highlights)
	}

	stats := gin.H{
		"total_books":      len(books),
		"total_highlights": totalHighlights,
	}

	c.IndentedJSON(http.StatusOK, stats)
}
