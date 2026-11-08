package http

import (
	"html/template"
	"strings"
	"testing"
)

func TestImportResultTemplateRendersEveryResultType(t *testing.T) {
	tmpl := template.Must(template.ParseFiles("../../templates/import-results.html"))
	cases := []struct {
		data any
		want []string
	}{
		{&KindleImportResult{Success: true, BooksImported: 2, HighlightsImported: 7}, []string{"Kindle import successful", ">7<", "highlights"}},
		{&AppleBooksImportResult{Success: true, BooksImported: 3}, []string{"Apple Books import successful", ">3<"}},
		{&ReadwiseCSVImportResult{Success: true, TotalRows: 11}, []string{"CSV Import Successful", ">11<", "rows processed"}},
		{KindleImportResult{Success: true, Errors: []string{"w1"}}, []string{"Warnings:", "<li>w1</li>"}},
		{&MoonReaderImportResult{Success: true, BooksExported: 5}, []string{"Import successful", "highlights", "books imported", ">5<", "books exported"}},
		{&KindleImportResult{Error: "bad file"}, []string{"Import failed", "bad file"}},
	}
	for _, c := range cases {
		var b strings.Builder
		if err := tmpl.ExecuteTemplate(&b, "import-result", c.data); err != nil {
			t.Fatalf("%T: %v", c.data, err)
		}
		// Wanted strings must appear in order, so stat order is checked too.
		out := b.String()
		for _, w := range c.want {
			i := strings.Index(out, w)
			if i < 0 {
				t.Errorf("%T: output missing %q (or out of order)", c.data, w)
				break
			}
			out = out[i+len(w):]
		}
	}
}
