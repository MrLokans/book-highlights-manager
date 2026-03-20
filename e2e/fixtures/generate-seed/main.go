// Package main generates a seed SQLite database for e2e tests.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/testutil"
)

func main() {
	outPath := filepath.Join("..", "seed.db")
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	// Remove existing seed DB
	_ = os.Remove(outPath) // best-effort cleanup

	db, err := gorm.Open(sqlite.Open(outPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Migrate all entities (including OAuthToken which is not in testutil.AllEntities())
	allEntities := append(testutil.AllEntities(), &entities.OAuthToken{})
	if err := db.AutoMigrate(allEntities...); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	// Create test user with bcrypt-hashed password
	hashedPw, err := bcrypt.GenerateFromPassword([]byte("testpassword12"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}
	now := time.Now()
	user := &entities.User{
		Username:     "testuser",
		Email:        "test@test.com",
		PasswordHash: string(hashedPw),
		Role:         entities.UserRoleAdmin,
		LastLoginAt:  &now,
	}
	db.Create(user)

	// Create source
	source := &entities.Source{Name: "readwise", DisplayName: "Readwise"}
	db.Create(source)

	// Create tags
	tagFiction := &entities.Tag{UserID: user.ID, Name: "fiction"}
	tagNonfiction := &entities.Tag{UserID: user.ID, Name: "nonfiction"}
	tagFavourite := &entities.Tag{UserID: user.ID, Name: "favourite"}
	db.Create(tagFiction)
	db.Create(tagNonfiction)
	db.Create(tagFavourite)

	// Book 1: Many highlights, has cover, has tags
	book1 := &entities.Book{
		UserID:   user.ID,
		Title:    "The Art of Testing",
		Author:   "Jane Smith",
		SourceID: source.ID,
		CoverURL: "https://example.com/cover1.jpg",
		ISBN:     "978-0-123456-78-9",
		Tags:     []entities.Tag{*tagFiction, *tagFavourite},
	}
	db.Create(book1)
	for i := 0; i < 22; i++ {
		h := &entities.Highlight{
			BookID:        book1.ID,
			UserID:        user.ID,
			Text:          fmt.Sprintf("Highlight %d from The Art of Testing. This is a test highlight with enough text to be realistic.", i+1),
			LocationType:  entities.LocationTypePage,
			LocationValue: i + 1,
			SourceID:      source.ID,
			HighlightedAt: time.Now().Add(-time.Duration(i) * time.Hour),
			IsFavorite:    i < 2, // First 2 are favourites
		}
		db.Create(h)
		if i == 0 {
			if err := db.Model(h).Association("Tags").Append(tagFiction); err != nil {
				log.Fatalf("failed to append tag: %v", err)
			}
		}
	}

	// Book 2: Single highlight, no cover
	book2 := &entities.Book{
		UserID:   user.ID,
		Title:    "Brief Thoughts",
		Author:   "John Doe",
		SourceID: source.ID,
		Tags:     []entities.Tag{*tagNonfiction},
	}
	db.Create(book2)
	db.Create(&entities.Highlight{
		BookID:        book2.ID,
		UserID:        user.ID,
		Text:          "The only highlight in this book, but it is a good one.",
		LocationType:  entities.LocationTypePage,
		LocationValue: 42,
		SourceID:      source.ID,
		HighlightedAt: time.Now().Add(-24 * time.Hour),
	})

	// Book 3: Zero highlights
	db.Create(&entities.Book{
		UserID:   user.ID,
		Title:    "Empty Reads",
		Author:   "Alice Wonder",
		SourceID: source.ID,
	})

	// Book 4: Highlights with favourites
	book4 := &entities.Book{
		UserID:   user.ID,
		Title:    "Favourite Collection",
		Author:   "Bob Builder",
		SourceID: source.ID,
	}
	db.Create(book4)
	for i := 0; i < 5; i++ {
		db.Create(&entities.Highlight{
			BookID:        book4.ID,
			UserID:        user.ID,
			Text:          fmt.Sprintf("A memorable quote number %d from this wonderful book.", i+1),
			LocationType:  entities.LocationTypePage,
			LocationValue: (i + 1) * 10,
			SourceID:      source.ID,
			HighlightedAt: time.Now().Add(-time.Duration(i*2) * time.Hour),
			IsFavorite:    i < 3, // First 3 are favourites
		})
	}

	// Book 5: No author
	book5 := &entities.Book{
		UserID:   user.ID,
		Title:    "Anonymous Wisdom",
		Author:   "",
		SourceID: source.ID,
	}
	db.Create(book5)
	db.Create(&entities.Highlight{
		BookID:        book5.ID,
		UserID:        user.ID,
		Text:          "Words of wisdom from an unknown source.",
		LocationType:  entities.LocationTypePage,
		LocationValue: 1,
		SourceID:      source.ID,
		HighlightedAt: time.Now().Add(-48 * time.Hour),
	})

	// Create vocabulary entries
	db.Create(&entities.Word{
		UserID:              user.ID,
		Word:                "ephemeral",
		Status:              entities.WordStatusEnriched,
		SourceBookTitle:     book1.Title,
		SourceBookAuthor:    book1.Author,
		SourceHighlightText: "Highlight 1 from The Art of Testing.",
		BookID:              &book1.ID,
	})
	db.Create(&entities.Word{
		UserID:           user.ID,
		Word:             "ubiquitous",
		Status:           entities.WordStatusPending,
		SourceBookTitle:  book2.Title,
		SourceBookAuthor: book2.Author,
		BookID:           &book2.ID,
	})
	db.Create(&entities.Word{
		UserID:           user.ID,
		Word:             "serendipity",
		Status:           entities.WordStatusEnriched,
		SourceBookTitle:  book4.Title,
		SourceBookAuthor: book4.Author,
		BookID:           &book4.ID,
	})

	// Create audit event
	db.Create(&entities.AuditEvent{
		UserID:      user.ID,
		EventType:   entities.AuditEventImport,
		Action:      "kindle_import",
		Description: "Imported 3 books from Kindle clippings",
		EntityType:  "book",
		Status:      entities.AuditStatusSuccess,
	})

	// Create a setting
	db.Create(&entities.Setting{
		Key:   "obsidian_vault_dir",
		Value: "/tmp/test-vault",
	})

	fmt.Printf("Seed database created at %s\n", outPath)
}
