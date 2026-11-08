package http

// ImportStat is one number shown in the shared "import-result" template.
type ImportStat struct {
	Value int
	Label string
}

// ImportTitle is the success heading for a Kindle import.
func (r KindleImportResult) ImportTitle() string { return "Kindle import successful" }

// ImportStats lists the numbers shown after a Kindle import.
func (r KindleImportResult) ImportStats() []ImportStat {
	return []ImportStat{{r.BooksImported, "books"}, {r.HighlightsImported, "highlights"}}
}

// ImportTitle is the success heading for an Apple Books import.
func (r AppleBooksImportResult) ImportTitle() string { return "Apple Books import successful" }

// ImportStats lists the numbers shown after an Apple Books import.
func (r AppleBooksImportResult) ImportStats() []ImportStat {
	return []ImportStat{{r.BooksImported, "books"}, {r.HighlightsImported, "highlights"}}
}

// ImportTitle is the success heading for a Readwise CSV import.
func (r ReadwiseCSVImportResult) ImportTitle() string { return "CSV Import Successful" }

// ImportStats lists the numbers shown after a Readwise CSV import.
func (r ReadwiseCSVImportResult) ImportStats() []ImportStat {
	return []ImportStat{{r.TotalRows, "rows processed"}, {r.BooksImported, "books"}, {r.HighlightsImported, "highlights"}}
}

// ImportTitle is the success heading for a Moon+ Reader import.
func (r MoonReaderImportResult) ImportTitle() string { return "Import successful" }

// ImportStats lists the numbers shown after a Moon+ Reader import.
func (r MoonReaderImportResult) ImportStats() []ImportStat {
	return []ImportStat{{r.Highlights, "highlights"}, {r.BooksImported, "books imported"}, {r.BooksExported, "books exported"}}
}
