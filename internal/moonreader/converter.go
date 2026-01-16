package moonreader

import (
	"github.com/mrlokans/assistant/internal/entities"
)

// ConvertToEntities converts MoonReader LocalNotes grouped by book to entities.Book slice
func ConvertToEntities(notesByBook map[string][]*LocalNote) []entities.Book {
	books := make([]entities.Book, 0, len(notesByBook))

	for bookTitle, notes := range notesByBook {
		if len(notes) == 0 {
			continue
		}

		book := entities.Book{
			Title:  bookTitle,
			Author: notes[0].GetAuthor(),
			Source: entities.Source{Name: "moonreader"},
		}

		highlights := make([]entities.Highlight, 0, len(notes))
		for _, note := range notes {
			highlight := ConvertNoteToHighlight(note)
			highlights = append(highlights, highlight)
		}

		book.Highlights = highlights
		books = append(books, book)
	}

	return books
}

// ConvertNoteToHighlight converts a single LocalNote to entities.Highlight
func ConvertNoteToHighlight(note *LocalNote) entities.Highlight {
	highlight := entities.Highlight{
		Text:          note.GetText(),
		Note:          note.NoteText,
		HighlightedAt: note.Time,
		Color:         note.GetColorHex(),
		Chapter:       note.Bookmark,
		ExternalID:    note.ExternalID,
	}

	// Set style based on formatting
	if note.Strikethrough {
		highlight.Style = entities.HighlightStyleStrikethrough
	} else if note.Underline {
		highlight.Style = entities.HighlightStyleUnderline
	} else {
		highlight.Style = entities.HighlightStyleHighlight
	}

	return highlight
}
