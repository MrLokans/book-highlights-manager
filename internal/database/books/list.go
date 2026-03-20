package books

import (
	"math"
	"strings"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

// ListBooksOptions controls filtering, sorting, and pagination for book listing.
type ListBooksOptions struct {
	UserID  uint
	Query   string
	TagID   uint
	Sort    string
	Page    int
	PerPage int
}

// BookListItem is a lightweight book representation for listing pages.
// Does NOT preload full highlights — only loads count for display.
type BookListItem struct {
	entities.Book
	HighlightCount int64
}

// ListBooksResult holds a page of books plus pagination metadata.
type ListBooksResult struct {
	Books      []BookListItem
	TotalCount int64
	Page       int
	TotalPages int
}

// ListBooks returns a filtered, sorted, paginated list of books with highlight counts.
func (r *Repository) ListBooks(opts ListBooksOptions) (*ListBooksResult, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}

	baseScope := func(db *gorm.DB) *gorm.DB {
		q := db.Where("books.user_id = ?", opts.UserID)
		if opts.Query != "" {
			searchPattern := "%" + strings.ToLower(opts.Query) + "%"
			q = q.Where("LOWER(books.title) LIKE ? OR LOWER(books.author) LIKE ?", searchPattern, searchPattern)
		}
		if opts.TagID > 0 {
			q = q.Where("books.id IN (SELECT book_id FROM book_tags WHERE tag_id = ?)", opts.TagID)
		}
		return q
	}

	// Count total results (clean query)
	var totalCount int64
	if err := r.db.Model(&entities.Book{}).Scopes(baseScope).Count(&totalCount).Error; err != nil {
		return nil, err
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(opts.PerPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	if opts.Page > totalPages {
		opts.Page = totalPages
	}

	orderClause := sortToOrderClause(opts.Sort)
	offset := (opts.Page - 1) * opts.PerPage

	// Query books with highlight_count as an extra column.
	// We query into entities.Book (so GORM resolves Preloads correctly)
	// and separately fetch highlight counts.
	var foundBooks []entities.Book
	err := r.db.Model(&entities.Book{}).
		Scopes(baseScope).
		Select("books.*, (SELECT COUNT(*) FROM highlights WHERE highlights.book_id = books.id AND highlights.deleted_at IS NULL) as highlight_count").
		Preload("Tags").
		Preload("Source").
		Order(orderClause).
		Offset(offset).
		Limit(opts.PerPage).
		Find(&foundBooks).Error
	if err != nil {
		return nil, err
	}

	// Get highlight counts via a separate lightweight query
	bookIDs := make([]uint, len(foundBooks))
	for i, b := range foundBooks {
		bookIDs[i] = b.ID
	}

	countMap := make(map[uint]int64, len(bookIDs))
	if len(bookIDs) > 0 {
		type countRow struct {
			BookID         uint  `gorm:"column:book_id"`
			HighlightCount int64 `gorm:"column:highlight_count"`
		}
		var counts []countRow
		err = r.db.Model(&entities.Highlight{}).
			Select("book_id, COUNT(*) as highlight_count").
			Where("book_id IN ? AND deleted_at IS NULL", bookIDs).
			Group("book_id").
			Find(&counts).Error
		if err != nil {
			return nil, err
		}
		for _, c := range counts {
			countMap[c.BookID] = c.HighlightCount
		}
	}

	items := make([]BookListItem, len(foundBooks))
	for i, b := range foundBooks {
		items[i] = BookListItem{
			Book:           b,
			HighlightCount: countMap[b.ID],
		}
	}

	return &ListBooksResult{
		Books:      items,
		TotalCount: totalCount,
		Page:       opts.Page,
		TotalPages: totalPages,
	}, nil
}

func sortToOrderClause(sort string) string {
	switch sort {
	case "date_asc":
		return "books.created_at ASC"
	case "title_asc":
		return "books.title ASC"
	case "title_desc":
		return "books.title DESC"
	case "author_asc":
		return "books.author ASC"
	case "author_desc":
		return "books.author DESC"
	case "highlights_desc":
		return "highlight_count DESC"
	case "highlights_asc":
		return "highlight_count ASC"
	default:
		return "books.created_at DESC"
	}
}
