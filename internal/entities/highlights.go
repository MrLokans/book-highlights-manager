package entities

import (
	"time"

	"gorm.io/gorm"
)

type LocationType string

const (
	LocationTypePage     LocationType = "page"
	LocationTypeLocation LocationType = "location" // Kindle-style location
	LocationTypePercent  LocationType = "percent"
	LocationTypeCFI      LocationType = "cfi" // EPUB Canonical Fragment Identifier
	LocationTypeTime     LocationType = "time"
	LocationTypePosition LocationType = "position"
	LocationTypeNone     LocationType = "none"
)

type HighlightStyle string

const (
	HighlightStyleHighlight     HighlightStyle = "highlight"
	HighlightStyleUnderline     HighlightStyle = "underline"
	HighlightStyleStrikethrough HighlightStyle = "strikethrough"
	HighlightStyleNoteOnly      HighlightStyle = "note_only"
)

type ImportStatus string

const (
	ImportStatusPending   ImportStatus = "pending"
	ImportStatusRunning   ImportStatus = "running"
	ImportStatusCompleted ImportStatus = "completed"
	ImportStatusFailed    ImportStatus = "failed"
)

type Source struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:50" json:"name"` // e.g., "kindle", "apple_books", "moonreader"
	DisplayName string    `gorm:"size:100" json:"display_name"`    // e.g., "Amazon Kindle", "Apple Books"
	CreatedAt   time.Time `json:"created_at"`
}

type UserRole string

const (
	UserRoleAdmin  UserRole = "admin"
	UserRoleEditor UserRole = "editor"
	UserRoleViewer UserRole = "viewer"
)

type User struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Username       string         `gorm:"uniqueIndex;size:100" json:"username"`
	Email          string         `gorm:"uniqueIndex;size:255" json:"email"`
	PasswordHash   string         `gorm:"size:72" json:"-"` // bcrypt hash, hidden from JSON
	Role           UserRole       `gorm:"size:20;default:'viewer'" json:"role"`
	Token          string         `gorm:"size:64" json:"-"`       // Deprecated: plaintext token, kept for migration
	TokenHash      string         `gorm:"index;size:64" json:"-"` // Hashed token for secure storage
	TokenCreatedAt *time.Time     `json:"-"`                      // When the current token was generated
	LastLoginAt    *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Security tracking for account lockout
	FailedLoginCount int        `gorm:"default:0" json:"-"`
	LockedUntil      *time.Time `json:"-"`
}

type Book struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"index" json:"user_id"`
	Title           string         `gorm:"index;size:512" json:"title"`
	Author          string         `gorm:"index;size:256" json:"author"`
	ISBN            string         `gorm:"index;size:20" json:"isbn,omitempty"`
	ASIN            string         `gorm:"size:20" json:"asin,omitempty"`
	CoverURL        string         `gorm:"size:2048" json:"cover_url,omitempty"`
	Publisher       string         `gorm:"size:256" json:"publisher,omitempty"`
	PublicationYear int            `json:"publication_year,omitempty"`
	FilePath        string         `gorm:"size:1024" json:"file_path,omitempty"`
	FileHash        string         `gorm:"index;size:64" json:"file_hash,omitempty"`
	ExternalID      string         `gorm:"size:256" json:"external_id,omitempty"`
	SourceID        uint           `gorm:"index" json:"source_id"`
	Source          Source         `gorm:"foreignKey:SourceID" json:"source,omitempty"`
	User            User           `gorm:"foreignKey:UserID" json:"-"`
	Highlights      []Highlight    `gorm:"foreignKey:BookID" json:"highlights,omitempty"`
	Tags            []Tag          `gorm:"many2many:book_tags;" json:"tags,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Deprecated: Use FilePath instead. Kept for backward compatibility.
	File string `gorm:"size:1024" json:"file,omitempty"`
}

type Highlight struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	BookID uint   `gorm:"index" json:"book_id"`
	UserID uint   `gorm:"index" json:"user_id"`
	Text   string `gorm:"type:text" json:"text"`
	Note   string `gorm:"type:text" json:"note,omitempty"`

	// Location information
	LocationType  LocationType `gorm:"size:20;default:'page'" json:"location_type"`
	LocationValue int          `json:"location_value,omitempty"`
	LocationEnd   int          `json:"location_end,omitempty"` // For ranges
	Percent       float64      `json:"percent,omitempty"`      // 0.0-1.0 position
	Chapter       string       `gorm:"size:256" json:"chapter,omitempty"`

	// Styling
	Color string         `gorm:"size:10" json:"color,omitempty"` // Hex color code
	Style HighlightStyle `gorm:"size:20;default:'highlight'" json:"style,omitempty"`

	// Metadata
	HighlightedAt time.Time `json:"highlighted_at,omitempty"` // When user made the highlight
	IsFavorite    bool      `gorm:"default:false" json:"is_favorite"`
	IsDiscarded   bool      `gorm:"default:false" json:"is_discarded"`

	// Context (W3C Web Annotation inspired)
	ContextPrefix string `gorm:"size:500" json:"context_prefix,omitempty"`
	ContextSuffix string `gorm:"size:500" json:"context_suffix,omitempty"`

	// Source tracking
	ExternalID string `gorm:"size:256" json:"external_id,omitempty"`
	SourceID   uint   `gorm:"index" json:"source_id"`
	Source     Source `gorm:"foreignKey:SourceID" json:"source,omitempty"`

	// Relationships
	Book Book  `gorm:"foreignKey:BookID" json:"-"`
	User User  `gorm:"foreignKey:UserID" json:"-"`
	Tags []Tag `gorm:"many2many:highlight_tags;" json:"tags,omitempty"`

	// Timestamps
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Deprecated: Use HighlightedAt instead. Kept for backward compatibility.
	Time string `json:"time,omitempty"`
	// Deprecated: Use LocationValue instead. Kept for backward compatibility.
	Page int `json:"page,omitempty"`
}

type Tag struct {
	ID         uint        `gorm:"primaryKey" json:"id"`
	UserID     uint        `gorm:"uniqueIndex:idx_tag_user_name" json:"user_id"`
	Name       string      `gorm:"uniqueIndex:idx_tag_user_name;size:100" json:"name"`
	User       User        `gorm:"foreignKey:UserID" json:"-"`
	Books      []Book      `gorm:"many2many:book_tags;" json:"-"`
	Highlights []Highlight `gorm:"many2many:highlight_tags;" json:"-"`
	CreatedAt  time.Time   `json:"created_at"`
}

type ImportSession struct {
	ID                  uint         `gorm:"primaryKey" json:"id"`
	UserID              uint         `gorm:"index" json:"user_id"`
	SourceID            uint         `gorm:"index" json:"source_id"`
	Status              ImportStatus `gorm:"size:20;default:'pending'" json:"status"`
	BooksProcessed      int          `json:"books_processed"`
	HighlightsProcessed int          `json:"highlights_processed"`
	BooksCreated        int          `json:"books_created"`
	HighlightsCreated   int          `json:"highlights_created"`
	Errors              string       `gorm:"type:text" json:"errors,omitempty"` // JSON array of errors
	StartedAt           time.Time    `json:"started_at"`
	CompletedAt         *time.Time   `json:"completed_at,omitempty"`
	User                User         `gorm:"foreignKey:UserID" json:"-"`
	Source              Source       `gorm:"foreignKey:SourceID" json:"source,omitempty"`
}

func (Tag) TableName() string {
	return "tags"
}

func (Source) TableName() string {
	return "sources"
}

func (User) TableName() string {
	return "users"
}

func (ImportSession) TableName() string {
	return "import_sessions"
}

// DeletedEntity tracks permanently deleted books and highlights to prevent re-import.
// When a user permanently deletes an entity, we store its unique identifier here
// so that future imports will skip matching entities.
type DeletedEntity struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	EntityType string    `gorm:"index;size:20" json:"entity_type"` // "book" or "highlight"
	EntityKey  string    `gorm:"index;size:512" json:"entity_key"` // Unique identifier (title+author for books, text+location for highlights)
	SourceID   uint      `gorm:"index" json:"source_id"`
	DeletedAt  time.Time `json:"deleted_at"`
}

func (DeletedEntity) TableName() string {
	return "deleted_entities"
}

// WordStatus represents the enrichment status of a vocabulary word.
type WordStatus string

const (
	WordStatusPending  WordStatus = "pending"
	WordStatusEnriched WordStatus = "enriched"
	WordStatusFailed   WordStatus = "failed"
)

// Word represents a vocabulary word saved from a highlight.
type Word struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"index" json:"user_id"`
	Word        string     `gorm:"index;size:100" json:"word"`
	HighlightID *uint      `gorm:"index" json:"highlight_id,omitempty"`
	BookID      *uint      `gorm:"index" json:"book_id,omitempty"`
	Context     string     `gorm:"type:text" json:"context,omitempty"`
	Status      WordStatus `gorm:"size:20;default:'pending'" json:"status"`

	// Denormalized source info preserved after highlight/book deletion
	SourceBookTitle     string `gorm:"size:512" json:"source_book_title,omitempty"`
	SourceBookAuthor    string `gorm:"size:256" json:"source_book_author,omitempty"`
	SourceHighlightText string `gorm:"type:text" json:"source_highlight_text,omitempty"`

	EnrichmentError string `gorm:"size:512" json:"enrichment_error,omitempty"`

	// Relationships - ON DELETE SET NULL preserves words after source deletion
	Highlight   *Highlight       `gorm:"foreignKey:HighlightID;constraint:OnDelete:SET NULL" json:"highlight,omitempty"`
	Book        *Book            `gorm:"foreignKey:BookID;constraint:OnDelete:SET NULL" json:"book,omitempty"`
	User        User             `gorm:"foreignKey:UserID" json:"-"`
	Definitions []WordDefinition `gorm:"foreignKey:WordID" json:"definitions,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Word) TableName() string {
	return "words"
}

// WordDefinition contains dictionary definition data for a word.
type WordDefinition struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	WordID        uint   `gorm:"index" json:"word_id"`
	PartOfSpeech  string `gorm:"size:50" json:"part_of_speech"`
	Definition    string `gorm:"type:text" json:"definition"`
	Example       string `gorm:"type:text" json:"example,omitempty"`
	Pronunciation string `gorm:"size:100" json:"pronunciation,omitempty"`
	AudioURL      string `gorm:"size:512" json:"audio_url,omitempty"`
	Source        string `gorm:"size:50" json:"source"`

	Word Word `gorm:"foreignKey:WordID" json:"-"`

	CreatedAt time.Time `json:"created_at"`
}

func (WordDefinition) TableName() string {
	return "word_definitions"
}
