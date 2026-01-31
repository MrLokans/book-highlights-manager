// Package importers provides a unified pipeline for importing highlights from various sources.
//
// # Architecture
//
// The import pipeline follows a simple flow:
//
//	Source Data → Converter → RawHighlight → Pipeline → entities.Book → Exporter → Storage
//
// Each import source implements the Converter interface, which transforms source-specific
// data into a common RawHighlight format. The Pipeline then groups highlights by book
// and exports them using the configured Exporter.
//
// # Adding a New Import Source
//
// To add support for a new highlight source (e.g., Kobo):
//
//  1. Create a new file: kobo.go
//
//  2. Define your source-specific structs:
//
//     type KoboHighlight struct {
//     Text      string `json:"text"`
//     BookTitle string `json:"volumeTitle"`
//     // ... other fields
//     }
//
//  3. Implement the Converter interface:
//
//     type KoboConverter struct {
//     Highlights []KoboHighlight
//     }
//
//     func (c *KoboConverter) Convert() ([]RawHighlight, Source) {
//     // Transform KoboHighlight → RawHighlight
//     return highlights, Source{Name: "kobo"}
//     }
//
//     // Compile-time check
//     var _ Converter = (*KoboConverter)(nil)
//
//  4. Create an HTTP handler that uses the pipeline:
//
//     func (c *KoboImportController) Import(ctx *gin.Context) {
//     var req KoboImportRequest
//     if err := ctx.ShouldBindJSON(&req); err != nil {
//     // handle error
//     }
//
//     converter := importers.NewKoboConverter(req.Highlights)
//     result, err := c.pipeline.Import(converter)
//     // handle result
//     }
//
// # Existing Converters
//
//   - ReadwiseConverter: Readwise API JSON format
//   - ReadwiseCSVConverter: Readwise CSV export format
//   - MoonReaderConverter: Moon+ Reader JSON format
//
// For sources that already provide book-level grouping (like Kindle or Apple Books),
// use Pipeline.ImportBooks() directly instead of implementing a Converter.
//
// # Example Usage
//
//	pipeline := importers.NewPipeline(exporter)
//
//	// Using a converter (for highlight-level data)
//	converter := importers.NewReadwiseConverter(highlights)
//	result, err := pipeline.Import(converter)
//
//	// Using direct book import (for pre-grouped data)
//	result, err := pipeline.ImportBooks(books)
package importers
