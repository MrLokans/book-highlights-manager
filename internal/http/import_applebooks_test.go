package http

import (
	"bytes"
	"database/sql"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupTestRouter creates a gin router with minimal HTML template for testing
func setupTestRouter(controller *AppleBooksImportController) *gin.Engine {
	router := gin.New()

	// Create a minimal template for testing
	tmpl := template.Must(template.New("applebooks-import-result").Parse(`
		{{ if .Success }}SUCCESS: books={{ .BooksImported }} highlights={{ .HighlightsImported }}{{ else }}ERROR: {{ .Error }}{{ end }}
	`))
	router.SetHTMLTemplate(tmpl)

	router.POST("/settings/applebooks/import", controller.Import)
	return router
}

// createValidAnnotationDB creates a valid Apple Books annotation database for testing
func createValidAnnotationDB(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("Failed to create annotation database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
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
}

// createValidBookDB creates a valid Apple Books book database for testing
func createValidBookDB(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("Failed to create book database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
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
}

// createInvalidSQLiteFile creates a file that is not a valid SQLite database
func createInvalidSQLiteFile(t *testing.T, path string) {
	t.Helper()

	err := os.WriteFile(path, []byte("this is not a sqlite database"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid SQLite file: %v", err)
	}
}

// createMissingTableDB creates a SQLite database missing the required table
func createMissingTableDB(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE other_table (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
}

// createMissingColumnDB creates a SQLite database with the table but missing required columns
func createMissingColumnAnnotationDB(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create table with ZAEANNOTATION name but missing ZANNOTATIONASSETID column
	_, err = db.Exec(`
		CREATE TABLE ZAEANNOTATION (
			Z_PK INTEGER PRIMARY KEY,
			ZANNOTATIONSELECTEDTEXT TEXT,
			ZANNOTATIONDELETED INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
}

// createMissingColumnBookDB creates a SQLite database with the table but missing required columns
func createMissingColumnBookDB(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create table with ZBKLIBRARYASSET name but missing ZTITLE column
	_, err = db.Exec(`
		CREATE TABLE ZBKLIBRARYASSET (
			Z_PK INTEGER PRIMARY KEY,
			ZASSETID TEXT,
			ZAUTHOR TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
}

// Helper to create multipart form with files
func createMultipartRequest(t *testing.T, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for fieldName, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("Failed to open file %s: %v", filePath, err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}

		_, err = io.Copy(part, file)
		if err != nil {
			t.Fatalf("Failed to copy file to form: %v", err)
		}
	}

	err := writer.Close()
	if err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

func TestValidateSQLiteFile_Valid(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	createValidAnnotationDB(t, dbPath)

	err := validateSQLiteFile(dbPath)
	if err != nil {
		t.Errorf("Expected no error for valid SQLite file, got: %v", err)
	}
}

func TestValidateSQLiteFile_Invalid(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.sqlite")

	createInvalidSQLiteFile(t, filePath)

	err := validateSQLiteFile(filePath)
	if err == nil {
		t.Error("Expected error for invalid SQLite file, got nil")
	}
}

func TestValidateAnnotationDatabase_Valid(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "annotations.sqlite")

	createValidAnnotationDB(t, dbPath)

	err := validateAnnotationDatabase(dbPath)
	if err != nil {
		t.Errorf("Expected no error for valid annotation database, got: %v", err)
	}
}

func TestValidateAnnotationDatabase_MissingTable(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "annotations.sqlite")

	createMissingTableDB(t, dbPath)

	err := validateAnnotationDatabase(dbPath)
	if err == nil {
		t.Error("Expected error for missing table, got nil")
	}
}

func TestValidateAnnotationDatabase_MissingColumn(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "annotations.sqlite")

	createMissingColumnAnnotationDB(t, dbPath)

	err := validateAnnotationDatabase(dbPath)
	if err == nil {
		t.Error("Expected error for missing column, got nil")
	}
}

func TestValidateBookDatabase_Valid(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "books.sqlite")

	createValidBookDB(t, dbPath)

	err := validateBookDatabase(dbPath)
	if err != nil {
		t.Errorf("Expected no error for valid book database, got: %v", err)
	}
}

func TestValidateBookDatabase_MissingTable(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "books.sqlite")

	createMissingTableDB(t, dbPath)

	err := validateBookDatabase(dbPath)
	if err == nil {
		t.Error("Expected error for missing table, got nil")
	}
}

func TestValidateBookDatabase_MissingColumn(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "books.sqlite")

	createMissingColumnBookDB(t, dbPath)

	err := validateBookDatabase(dbPath)
	if err == nil {
		t.Error("Expected error for missing column, got nil")
	}
}

func TestTableExists_True(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	if !tableExists(db, "test_table") {
		t.Error("Expected tableExists to return true for existing table")
	}
}

func TestTableExists_False(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE other_table (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	if tableExists(db, "nonexistent_table") {
		t.Error("Expected tableExists to return false for non-existing table")
	}
}

func TestColumnExists_True(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	if !columnExists(db, "test_table", "name") {
		t.Error("Expected columnExists to return true for existing column")
	}
}

func TestColumnExists_False(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.sqlite")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE test_table (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	if columnExists(db, "test_table", "nonexistent_column") {
		t.Error("Expected columnExists to return false for non-existing column")
	}
}

func TestAppleBooksImport_MissingAnnotationDB(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	bookPath := filepath.Join(tempDir, "books.sqlite")
	createValidBookDB(t, bookPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"book_db": bookPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}

func TestAppleBooksImport_MissingBookDB(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	annotationPath := filepath.Join(tempDir, "annotations.sqlite")
	createValidAnnotationDB(t, annotationPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"annotation_db": annotationPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}

func TestAppleBooksImport_InvalidAnnotationDB(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	annotationPath := filepath.Join(tempDir, "annotations.sqlite")
	bookPath := filepath.Join(tempDir, "books.sqlite")

	createInvalidSQLiteFile(t, annotationPath)
	createValidBookDB(t, bookPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"annotation_db": annotationPath,
		"book_db":       bookPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}

func TestAppleBooksImport_InvalidBookDB(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	annotationPath := filepath.Join(tempDir, "annotations.sqlite")
	bookPath := filepath.Join(tempDir, "books.sqlite")

	createValidAnnotationDB(t, annotationPath)
	createInvalidSQLiteFile(t, bookPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"annotation_db": annotationPath,
		"book_db":       bookPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}

func TestAppleBooksImport_MissingAnnotationTable(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	annotationPath := filepath.Join(tempDir, "annotations.sqlite")
	bookPath := filepath.Join(tempDir, "books.sqlite")

	createMissingTableDB(t, annotationPath)
	createValidBookDB(t, bookPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"annotation_db": annotationPath,
		"book_db":       bookPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}

func TestAppleBooksImport_MissingBookTable(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	annotationPath := filepath.Join(tempDir, "annotations.sqlite")
	bookPath := filepath.Join(tempDir, "books.sqlite")

	createValidAnnotationDB(t, annotationPath)
	createMissingTableDB(t, bookPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"annotation_db": annotationPath,
		"book_db":       bookPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}

func TestAppleBooksImport_MissingRequiredAnnotationColumns(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	annotationPath := filepath.Join(tempDir, "annotations.sqlite")
	bookPath := filepath.Join(tempDir, "books.sqlite")

	createMissingColumnAnnotationDB(t, annotationPath)
	createValidBookDB(t, bookPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"annotation_db": annotationPath,
		"book_db":       bookPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}

func TestAppleBooksImport_MissingRequiredBookColumns(t *testing.T) {
	controller := NewAppleBooksImportController(nil)
	router := setupTestRouter(controller)

	tempDir := t.TempDir()
	annotationPath := filepath.Join(tempDir, "annotations.sqlite")
	bookPath := filepath.Join(tempDir, "books.sqlite")

	createValidAnnotationDB(t, annotationPath)
	createMissingColumnBookDB(t, bookPath)

	body, contentType := createMultipartRequest(t, map[string]string{
		"annotation_db": annotationPath,
		"book_db":       bookPath,
	})

	req := httptest.NewRequest(http.MethodPost, "/settings/applebooks/import", body)
	req.Header.Set("Content-Type", contentType)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR") {
		t.Errorf("Expected error in response body, got: %s", w.Body.String())
	}
}
