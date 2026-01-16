package http

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"

	"github.com/mrlokans/assistant/internal/applebooks"
	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/exporters"
)

const (
	// Maximum file size for uploads (50 MB)
	maxAppleBooksFileSize = 50 * 1024 * 1024

	// Expected table names for Apple Books databases
	annotationTable = "ZAEANNOTATION"
	bookTable       = "ZBKLIBRARYASSET"
)

type AppleBooksImportController struct {
	exporter     exporters.BookExporter
	auditService *audit.Service
}

func NewAppleBooksImportController(exporter exporters.BookExporter, auditService *audit.Service) *AppleBooksImportController {
	return &AppleBooksImportController{
		exporter:     exporter,
		auditService: auditService,
	}
}

type AppleBooksImportResult struct {
	Success           bool     `json:"success"`
	Error             string   `json:"error,omitempty"`
	BooksImported     int      `json:"books_imported"`
	HighlightsImported int      `json:"highlights_imported"`
	Errors            []string `json:"errors,omitempty"`
}

func (c *AppleBooksImportController) Import(ctx *gin.Context) {
	// Create temp directory for uploaded files
	tempDir, err := os.MkdirTemp("", "applebooks-import-*")
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   "Failed to create temporary directory",
		})
		return
	}
	defer os.RemoveAll(tempDir)

	// Process annotation database
	annotationPath, err := c.processUploadedFile(ctx, "annotation_db", tempDir, "annotation.sqlite")
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   fmt.Sprintf("Annotation database: %v", err),
		})
		return
	}

	// Validate annotation database structure
	if err := validateAnnotationDatabase(annotationPath); err != nil {
		ctx.HTML(http.StatusBadRequest, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid annotation database: %v", err),
		})
		return
	}

	// Process book database
	bookPath, err := c.processUploadedFile(ctx, "book_db", tempDir, "book.sqlite")
	if err != nil {
		ctx.HTML(http.StatusBadRequest, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   fmt.Sprintf("Book database: %v", err),
		})
		return
	}

	// Validate book database structure
	if err := validateBookDatabase(bookPath); err != nil {
		ctx.HTML(http.StatusBadRequest, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid book database: %v", err),
		})
		return
	}

	// Create reader and get books
	reader, err := applebooks.NewAppleBooksReader(annotationPath, bookPath)
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to initialize reader: %v", err),
		})
		return
	}

	books, err := reader.GetBooks()
	if err != nil {
		ctx.HTML(http.StatusInternalServerError, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to read books: %v", err),
		})
		return
	}

	if len(books) == 0 {
		ctx.HTML(http.StatusOK, "applebooks-import-result", &AppleBooksImportResult{
			Success:           true,
			BooksImported:     0,
			HighlightsImported: 0,
			Errors:            []string{"No books with highlights found in the uploaded databases"},
		})
		return
	}

	// Export to database and markdown
	result, exportErr := c.exporter.Export(books)

	// Log the import event
	if c.auditService != nil {
		desc := fmt.Sprintf("Imported %d books with %d highlights from Apple Books", result.BooksProcessed, result.HighlightsProcessed)
		c.auditService.LogImport(DefaultUserID, "applebooks", desc, result.BooksProcessed, result.HighlightsProcessed, exportErr)
	}

	if exportErr != nil {
		ctx.HTML(http.StatusInternalServerError, "applebooks-import-result", &AppleBooksImportResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to export: %v", exportErr),
		})
		return
	}

	ctx.HTML(http.StatusOK, "applebooks-import-result", &AppleBooksImportResult{
		Success:           true,
		BooksImported:     result.BooksProcessed,
		HighlightsImported: result.HighlightsProcessed,
	})
}

func (c *AppleBooksImportController) processUploadedFile(ctx *gin.Context, fieldName, tempDir, filename string) (string, error) {
	file, header, err := ctx.Request.FormFile(fieldName)
	if err != nil {
		return "", fmt.Errorf("file not provided")
	}
	defer file.Close()

	// Check file size
	if header.Size > maxAppleBooksFileSize {
		return "", fmt.Errorf("file too large (max %d MB)", maxAppleBooksFileSize/(1024*1024))
	}

	// Validate file extension
	ext := filepath.Ext(header.Filename)
	if ext != ".sqlite" && ext != ".db" && ext != "" {
		return "", fmt.Errorf("invalid file type: expected .sqlite or .db file")
	}

	// Create destination path
	destPath := filepath.Join(tempDir, filename)

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file")
	}
	defer destFile.Close()

	// Copy with size limit
	limitedReader := io.LimitReader(file, maxAppleBooksFileSize+1)
	written, err := io.Copy(destFile, limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to save file")
	}
	if written > maxAppleBooksFileSize {
		return "", fmt.Errorf("file too large (max %d MB)", maxAppleBooksFileSize/(1024*1024))
	}

	// Validate it's a valid SQLite file
	if err := validateSQLiteFile(destPath); err != nil {
		return "", fmt.Errorf("not a valid SQLite database: %v", err)
	}

	return destPath, nil
}

func validateSQLiteFile(path string) error {
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("failed to open database")
	}
	defer db.Close()

	// Try to query the sqlite_master table to verify it's a valid SQLite file
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		return fmt.Errorf("invalid SQLite file structure")
	}

	return nil
}

func validateAnnotationDatabase(path string) error {
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("failed to open database")
	}
	defer db.Close()

	// Check for required table
	if !tableExists(db, annotationTable) {
		return fmt.Errorf("missing required table: %s", annotationTable)
	}

	// Validate required columns exist
	requiredColumns := []string{
		"ZANNOTATIONASSETID",
		"ZANNOTATIONSELECTEDTEXT",
		"ZANNOTATIONDELETED",
	}

	for _, col := range requiredColumns {
		if !columnExists(db, annotationTable, col) {
			return fmt.Errorf("missing required column: %s.%s", annotationTable, col)
		}
	}

	return nil
}

func validateBookDatabase(path string) error {
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("failed to open database")
	}
	defer db.Close()

	// Check for required table
	if !tableExists(db, bookTable) {
		return fmt.Errorf("missing required table: %s", bookTable)
	}

	// Validate required columns exist
	requiredColumns := []string{
		"ZASSETID",
		"ZTITLE",
		"ZAUTHOR",
	}

	for _, col := range requiredColumns {
		if !columnExists(db, bookTable, col) {
			return fmt.Errorf("missing required column: %s.%s", bookTable, col)
		}
	}

	return nil
}

func tableExists(db *sql.DB, tableName string) bool {
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&name)
	return err == nil
}

func columnExists(db *sql.DB, tableName, columnName string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dfltValue any
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == columnName {
			return true
		}
	}

	return false
}
