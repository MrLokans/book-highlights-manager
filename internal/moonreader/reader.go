package moonreader

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// BackupDBReader reads notes from a MoonReader backup database
type BackupDBReader struct {
	dbPath string
}

// NewBackupDBReader creates a new reader for the given database path
func NewBackupDBReader(dbPath string) *BackupDBReader {
	return &BackupDBReader{dbPath: dbPath}
}

// GetNotes retrieves all notes from the MoonReader backup database
func (r *BackupDBReader) GetNotes() ([]*MoonReaderNote, error) {
	db, err := sql.Open("sqlite3", r.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	query := `
		SELECT
			_id,
			book,
			filename,
			highlightColor,
			time,
			bookmark,
			note,
			original,
			underline,
			strikethrough
		FROM notes;
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	var notes []*MoonReaderNote
	for rows.Next() {
		note := &MoonReaderNote{}
		var bookmark, noteText, original sql.NullString
		var underline, strikethrough sql.NullInt64

		err := rows.Scan(
			&note.ID,
			&note.BookTitle,
			&note.Filename,
			&note.HighlightColor,
			&note.TimeMs,
			&bookmark,
			&noteText,
			&original,
			&underline,
			&strikethrough,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		note.Bookmark = bookmark.String
		note.Note = noteText.String
		note.Original = original.String
		if underline.Valid {
			note.Underline = int(underline.Int64)
		}
		if strikethrough.Valid {
			note.Strikethrough = int(strikethrough.Int64)
		}

		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return notes, nil
}

// LocalDBAccessor manages the local SQLite database for storing imported notes
type LocalDBAccessor struct {
	dbPath string
	db     *sql.DB
}

// NewLocalDBAccessor creates a new accessor for the local database
func NewLocalDBAccessor(dbPath string) (*LocalDBAccessor, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	accessor := &LocalDBAccessor{
		dbPath: dbPath,
		db:     db,
	}

	if err := accessor.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return accessor, nil
}

// Close closes the database connection
func (a *LocalDBAccessor) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// ensureSchema creates the required tables if they don't exist
func (a *LocalDBAccessor) ensureSchema() error {
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS backup_datetimes (
			id TEXT PRIMARY KEY,
			datetime DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS moonreader_notes (
			exported_id TEXT PRIMARY KEY,
			book_title TEXT NOT NULL,
			filename TEXT NOT NULL,
			color TEXT NOT NULL,
			time NUMERIC NOT NULL,
			bookmark TEXT NOT NULL,
			note TEXT,
			original TEXT,
			underline NUMERIC NOT NULL,
			strikethrough NUMERIC NOT NULL
		);`,
	}

	for _, schema := range schemas {
		if _, err := a.db.Exec(schema); err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
	}

	return nil
}

// UpsertNotes inserts or updates notes in the local database
func (a *LocalDBAccessor) UpsertNotes(notes []*MoonReaderNote) error {
	tx, err := a.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO moonreader_notes
			(exported_id, book_title, filename, color, time, bookmark, note, original, underline, strikethrough)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (exported_id) DO UPDATE SET
			book_title=excluded.book_title,
			filename=excluded.filename,
			color=excluded.color,
			time=excluded.time,
			bookmark=excluded.bookmark,
			note=excluded.note,
			original=excluded.original,
			underline=excluded.underline,
			strikethrough=excluded.strikethrough
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, note := range notes {
		_, err := stmt.Exec(
			strconv.FormatInt(note.ID, 10),
			note.BookTitle,
			note.Filename,
			note.HighlightColor,
			note.TimeMs,
			note.Bookmark,
			note.Note,
			note.Original,
			note.Underline,
			note.Strikethrough,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert note %d: %w", note.ID, err)
		}
	}

	return tx.Commit()
}

// GetNotes retrieves all notes from the local database
func (a *LocalDBAccessor) GetNotes() ([]*LocalNote, error) {
	query := `
		SELECT
			exported_id,
			book_title,
			filename,
			color,
			time,
			bookmark,
			note,
			original,
			underline,
			strikethrough
		FROM moonreader_notes;
	`

	rows, err := a.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer rows.Close()

	var notes []*LocalNote
	for rows.Next() {
		note := &LocalNote{}
		var bookmark, noteText, original sql.NullString
		var underline, strikethrough int

		err := rows.Scan(
			&note.ExternalID,
			&note.BookTitle,
			&note.Filename,
			&note.Color,
			&note.TimeMs,
			&bookmark,
			&noteText,
			&original,
			&underline,
			&strikethrough,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		note.Bookmark = bookmark.String
		note.NoteText = noteText.String
		note.Original = original.String
		note.Underline = underline != 0
		note.Strikethrough = strikethrough != 0
		note.Time = time.UnixMilli(note.TimeMs)

		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return notes, nil
}

// GetNotesByBook retrieves notes grouped by book title
func (a *LocalDBAccessor) GetNotesByBook() (map[string][]*LocalNote, error) {
	notes, err := a.GetNotes()
	if err != nil {
		return nil, err
	}

	notesByBook := make(map[string][]*LocalNote)
	for _, note := range notes {
		notesByBook[note.BookTitle] = append(notesByBook[note.BookTitle], note)
	}

	return notesByBook, nil
}

// RecordBackupImport records the timestamp of a backup import
func (a *LocalDBAccessor) RecordBackupImport(backupID string, importTime time.Time) error {
	_, err := a.db.Exec(
		`INSERT OR REPLACE INTO backup_datetimes (id, datetime) VALUES (?, ?)`,
		backupID,
		importTime,
	)
	return err
}

// GetLastBackupImport retrieves the timestamp of the last backup import
func (a *LocalDBAccessor) GetLastBackupImport(backupID string) (time.Time, error) {
	var timestamp time.Time
	err := a.db.QueryRow(
		`SELECT datetime FROM backup_datetimes WHERE id = ?`,
		backupID,
	).Scan(&timestamp)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return timestamp, err
}
