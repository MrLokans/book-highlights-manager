package applebooks

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mrlokans/assistant/internal/entities"
)

// Apple Books uses Core Data timestamp format: seconds since 2001-01-01 00:00:00 UTC
var coreDataEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

type AnnotationStyle int

const (
	AnnotationStyleUnderline AnnotationStyle = 1
	AnnotationStyleGreen     AnnotationStyle = 2
	AnnotationStyleBlue      AnnotationStyle = 3
	AnnotationStyleYellow    AnnotationStyle = 4
	AnnotationStylePink      AnnotationStyle = 5
	AnnotationStylePurple    AnnotationStyle = 6
)

type AppleBooksReader struct {
	annotationDBPath string
	bookDBPath       string
}

type AppleBooksHighlight struct {
	AssetID        string
	Title          string
	Author         string
	Location       string
	SelectedText   string
	Note           string
	RepresentText  string
	Chapter        string
	Style          int
	ModifiedDate   float64
	LocationStart  int
}

func DefaultAnnotationDBPath() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("Apple Books is only available on macOS")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	annotationDir := filepath.Join(homeDir, "Library", "Containers", "com.apple.iBooksX", "Data", "Documents", "AEAnnotation")

	// Find the .sqlite file in the directory
	entries, err := os.ReadDir(annotationDir)
	if err != nil {
		return "", fmt.Errorf("failed to read annotation directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sqlite" {
			return filepath.Join(annotationDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no .sqlite file found in %s", annotationDir)
}

func DefaultBookDBPath() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("Apple Books is only available on macOS")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	bookDir := filepath.Join(homeDir, "Library", "Containers", "com.apple.iBooksX", "Data", "Documents", "BKLibrary")

	// Find the .sqlite file in the directory
	entries, err := os.ReadDir(bookDir)
	if err != nil {
		return "", fmt.Errorf("failed to read book directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sqlite" {
			return filepath.Join(bookDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no .sqlite file found in %s", bookDir)
}

// If paths are empty, uses the default macOS paths
func NewAppleBooksReader(annotationDBPath, bookDBPath string) (*AppleBooksReader, error) {
	var err error

	if annotationDBPath == "" {
		annotationDBPath, err = DefaultAnnotationDBPath()
		if err != nil {
			return nil, fmt.Errorf("failed to find annotation database: %w", err)
		}
	}

	if bookDBPath == "" {
		bookDBPath, err = DefaultBookDBPath()
		if err != nil {
			return nil, fmt.Errorf("failed to find book database: %w", err)
		}
	}

	// Verify files exist
	if _, err := os.Stat(annotationDBPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("annotation database not found: %s", annotationDBPath)
	}
	if _, err := os.Stat(bookDBPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("book database not found: %s", bookDBPath)
	}

	return &AppleBooksReader{
		annotationDBPath: annotationDBPath,
		bookDBPath:       bookDBPath,
	}, nil
}

func (r *AppleBooksReader) GetAnnotationDBPath() string {
	return r.annotationDBPath
}

func (r *AppleBooksReader) GetBookDBPath() string {
	return r.bookDBPath
}

func (r *AppleBooksReader) GetHighlights() ([]AppleBooksHighlight, error) {
	// Open annotation database
	annotationDB, err := sql.Open("sqlite3", r.annotationDBPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open annotation database: %w", err)
	}
	defer annotationDB.Close()

	// Attach book database
	attachQuery := fmt.Sprintf("ATTACH DATABASE '%s' AS books", r.bookDBPath)
	if _, err := annotationDB.Exec(attachQuery); err != nil {
		return nil, fmt.Errorf("failed to attach book database: %w", err)
	}

	// Query for highlights joined with book metadata
	query := `
		SELECT
			ZANNOTATIONASSETID as asset_id,
			books.ZBKLIBRARYASSET.ZTITLE as title,
			books.ZBKLIBRARYASSET.ZAUTHOR as author,
			ZANNOTATIONLOCATION as location,
			ZANNOTATIONSELECTEDTEXT as selected_text,
			ZANNOTATIONNOTE as note,
			ZANNOTATIONREPRESENTATIVETEXT as represent_text,
			ZFUTUREPROOFING5 as chapter,
			ZANNOTATIONSTYLE as style,
			ZANNOTATIONMODIFICATIONDATE as modified_date,
			ZPLLOCATIONRANGESTART as location_start
		FROM ZAEANNOTATION
		LEFT JOIN books.ZBKLIBRARYASSET
			ON ZAEANNOTATION.ZANNOTATIONASSETID = books.ZBKLIBRARYASSET.ZASSETID
		WHERE ZANNOTATIONDELETED = 0
			AND (title NOT NULL AND author NOT NULL)
			AND ((selected_text != '' AND selected_text NOT NULL) OR note NOT NULL)
		ORDER BY ZANNOTATIONASSETID, ZPLLOCATIONRANGESTART
	`

	rows, err := annotationDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query highlights: %w", err)
	}
	defer rows.Close()

	var highlights []AppleBooksHighlight

	for rows.Next() {
		var h AppleBooksHighlight
		var location, selectedText, note, representText, chapter sql.NullString
		var modifiedDate sql.NullFloat64
		var locationStart sql.NullInt64

		err := rows.Scan(
			&h.AssetID,
			&h.Title,
			&h.Author,
			&location,
			&selectedText,
			&note,
			&representText,
			&chapter,
			&h.Style,
			&modifiedDate,
			&locationStart,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		h.Location = location.String
		h.SelectedText = selectedText.String
		h.Note = note.String
		h.RepresentText = representText.String
		h.Chapter = chapter.String
		if modifiedDate.Valid {
			h.ModifiedDate = modifiedDate.Float64
		}
		if locationStart.Valid {
			h.LocationStart = int(locationStart.Int64)
		}

		highlights = append(highlights, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return highlights, nil
}

func (r *AppleBooksReader) GetBooks() ([]entities.Book, error) {
	highlights, err := r.GetHighlights()
	if err != nil {
		return nil, err
	}

	// Group highlights by book (using AssetID as the grouping key)
	bookMap := make(map[string]*entities.Book)
	bookOrder := []string{} // Preserve order

	for _, h := range highlights {
		key := h.AssetID

		book, exists := bookMap[key]
		if !exists {
			book = &entities.Book{
				Title:  h.Title,
				Author: h.Author,
				Source: entities.Source{
					Name:        "apple_books",
					DisplayName: "Apple Books",
				},
				ExternalID: h.AssetID,
				Highlights: []entities.Highlight{},
			}
			bookMap[key] = book
			bookOrder = append(bookOrder, key)
		}

		// Convert Core Data timestamp to time.Time
		var highlightedAt time.Time
		if h.ModifiedDate != 0 {
			highlightedAt = coreDataEpoch.Add(time.Duration(h.ModifiedDate * float64(time.Second)))
		}

		// Determine text content (prefer selected text, fallback to representative text)
		text := h.SelectedText
		if text == "" {
			text = h.RepresentText
		}

		// Skip if no text content
		if text == "" && h.Note == "" {
			continue
		}

		highlight := entities.Highlight{
			Text:          text,
			Note:          h.Note,
			Chapter:       h.Chapter,
			LocationType:  entities.LocationTypePosition,
			LocationValue: h.LocationStart,
			HighlightedAt: highlightedAt,
			Style:         convertAnnotationStyle(h.Style),
			Color:         getColorForStyle(h.Style),
			ExternalID:    fmt.Sprintf("%s-%d", h.AssetID, h.LocationStart),
			Source: entities.Source{
				Name:        "apple_books",
				DisplayName: "Apple Books",
			},
		}

		book.Highlights = append(book.Highlights, highlight)
	}

	// Convert map to slice in original order
	var books []entities.Book
	for _, key := range bookOrder {
		book := bookMap[key]
		if len(book.Highlights) > 0 {
			books = append(books, *book)
		}
	}

	return books, nil
}

func convertAnnotationStyle(style int) entities.HighlightStyle {
	switch AnnotationStyle(style) {
	case AnnotationStyleUnderline:
		return entities.HighlightStyleUnderline
	default:
		return entities.HighlightStyleHighlight
	}
}

func getColorForStyle(style int) string {
	switch AnnotationStyle(style) {
	case AnnotationStyleUnderline:
		return "" // No color for underline
	case AnnotationStyleGreen:
		return "#00FF00"
	case AnnotationStyleBlue:
		return "#0000FF"
	case AnnotationStyleYellow:
		return "#FFFF00"
	case AnnotationStylePink:
		return "#FF69B4"
	case AnnotationStylePurple:
		return "#800080"
	default:
		return "#FFFF00" // Default to yellow
	}
}
