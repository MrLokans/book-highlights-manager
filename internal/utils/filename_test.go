package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"removes invalid chars", `Book: A "Story"`, "Book A Story"},
		{"replaces newlines with spaces", "Book\nTitle", "Book Title"},
		{"replaces tabs with spaces", "Book\tTitle", "Book Title"},
		{"collapses multiple spaces", "Book   Title", "Book Title"},
		{"trims whitespace", "  Book  ", "Book"},
		{"removes hashtags", "Book #1", "Book 1"},
		{"replaces brackets", "Book [Vol 1]", "Book (Vol 1)"},
		{"removes slashes", `Book/Title\Path`, "BookTitlePath"},
		{"removes question marks", "Book?", "Book"},
		{"removes pipe", "Book|Other", "BookOther"},
		{"removes asterisks", "Book*", "Book"},
		{"returns Untitled for empty", "", "Untitled"},
		{"truncates long names", strings.Repeat("a", 250), strings.Repeat("a", 200)},
		{"passes normal filename", "Clean Book Title", "Clean Book Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SanitizeFilename(tt.input))
		})
	}
}

func TestExtractAuthorFromFilename(t *testing.T) {
	const dir = "/sdcard/Books/MoonReader/"
	tests := []struct {
		name      string
		filename  string
		bookTitle string
		expected  string
	}{
		{"title - author", "My Book - John Doe.epub", "My Book", "John Doe"},
		{"no author", "My Book.epub", "My Book", ""},
		{"fb2.zip extension", "My Book - Author.fb2.zip", "My Book", "Author"},
		{"uppercase extension", "My Book - Author.PDF", "My Book", "Author"},
		{"directory is ignored", dir + "Pamiętnik znaleziony w wannie - Stanisław Lem.fb2", "Pamiętnik znaleziony w wannie", "Stanisław Lem"},

		// Moon+ glues the publisher onto the author
		{"glued publisher", dir + "The Origins of Efficiency - Brian PotterStripe Press.epub", "The Origins of Efficiency", "Brian Potter"},
		{"glued publisher after initial", dir + "High Output Management - Andrew S. GroveKnopf Doubleday Publishing Group.epub", "High Output Management", "Andrew S. Grove"},
		{"glued publisher ending in dot", dir + "Tempo timing tactics and strategy in narrative-driven decision-making - Venkatesh RaoRibbonfarm Inc..epub", "Tempo: timing, tactics and strategy in narrative-driven decision-making", "Venkatesh Rao"},
		{"glued publisher cyrillic", dir + "Важные годы - Мэг ДжейМанн Иванов и Фербер.epub", "Важные годы", "Мэг Джей"},
		{"glued hyphenated publisher", dir + "Как быть крысой. Искусство интриг и выживания на работе - Йооп СгрийверсОлимп-Бизнес.epub", "Как быть крысой. Искусство интриг и выживания на работе", "Йооп Сгрийверс"},
		{"name prefixes are not publisher splits", "Peopleware - Tom DeMarco.epub", "Peopleware", "Tom DeMarco"},
		{"Mc prefix kept", "Code Complete - Steve McConnell.epub", "Code Complete", "Steve McConnell"},

		// Filename drops the title's punctuation or adds a subtitle
		{"colon dropped from title", dir + "Moral Mazes The World of Corporate Managers - Robert JackallOxford University Press.pdf", "Moral Mazes: The World of Corporate Managers", "Robert Jackall"},
		{"apostrophe dropped from title", dir + "The Engineering Executives Primer - Will LarsonOReilly Media Inc..epub", "The Engineering Executive's Primer", "Will Larson"},
		{"subtitle only in filename", dir + "The Midnight Library A GMA Book Club Pick A Novel - Matt HaigPenguin Publishing Group.epub", "The Midnight Library", "Matt Haig"},
		{"title unknown to Moon+", dir + "Tracking Personal Finances Using Python - Siddhant Goel.pdf", "Untitled", "Siddhant Goel"},
		{"title is the filename", dir + "PostgreSQL 14 internals - Egor Rogov.pdf", "PostgreSQL 14 internals - Egor Rogov.pdf", "Egor Rogov"},

		// Download-site naming
		{"z-library", dir + "Becoming Bulletproof Protect Yourself, Read People, Influence Situations, and Live Fearlessly (Evy Poumpouras) (z-library.sk, 1lib.sk, z-lib.sk).epub", "Becoming Bulletproof", "Evy Poumpouras"},
		{"z-lib org tag", "Deep Work (Cal Newport) (Z-Library).epub", "Deep Work", "Cal Newport"},
		{"libgen author first", "Cal Newport - Deep Work (2016, Grand Central Publishing) - libgen.li.epub", "Deep Work", "Cal Newport"},
		{"calibre author first", "Cal Newport - Deep Work.epub", "Deep Work", "Cal Newport"},
		{"anna's archive", "Deep Work -- Cal Newport -- Grand Central Publishing, 2016 -- 9781455586691 -- 0c1f9e5b -- Anna’s Archive.epub", "Deep Work", "Cal Newport"},
		{"bracketed author", "Deep Work [Cal Newport].epub", "Deep Work", "Cal Newport"},
		{"by author", "Deep Work by Cal Newport.pdf", "Deep Work", "Cal Newport"},
		{"series in parentheses is not an author", "Dune (Dune Chronicles 1).epub", "Dune", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ExtractAuthorFromFilename(tt.filename, tt.bookTitle))
		})
	}
}
