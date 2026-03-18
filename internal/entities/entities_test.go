package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableNames(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
	}{
		{"Tag", Tag{}.TableName()},
		{"Source", Source{}.TableName()},
		{"User", User{}.TableName()},
		{"ImportSession", ImportSession{}.TableName()},
		{"DeletedEntity", DeletedEntity{}.TableName()},
		{"Word", Word{}.TableName()},
		{"WordDefinition", WordDefinition{}.TableName()},
		{"Setting", Setting{}.TableName()},
		{"SyncProgress", SyncProgress{}.TableName()},
		{"AuditEvent", AuditEvent{}.TableName()},
		{"OAuthToken", OAuthToken{}.TableName()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.tableName)
		})
	}
}

func TestLocationTypeString(t *testing.T) {
	assert.Equal(t, LocationTypePage, LocationType("page"))
	assert.Equal(t, LocationTypeLocation, LocationType("location"))
	assert.Equal(t, LocationTypeTime, LocationType("time"))
	assert.Equal(t, LocationTypePosition, LocationType("position"))
	assert.Equal(t, LocationTypeNone, LocationType("none"))
}

func TestHighlightStyleConstants(t *testing.T) {
	assert.Equal(t, HighlightStyleHighlight, HighlightStyle("highlight"))
	assert.Equal(t, HighlightStyleUnderline, HighlightStyle("underline"))
	assert.Equal(t, HighlightStyleStrikethrough, HighlightStyle("strikethrough"))
}

func TestWordStatusConstants(t *testing.T) {
	assert.Equal(t, WordStatusPending, WordStatus("pending"))
	assert.Equal(t, WordStatusEnriched, WordStatus("enriched"))
	assert.Equal(t, WordStatusFailed, WordStatus("failed"))
}

func TestSyncTypeConstants(t *testing.T) {
	assert.Equal(t, SyncTypeMetadata, SyncType("metadata"))
}

func TestSyncStatusConstants(t *testing.T) {
	assert.Equal(t, SyncStatusRunning, SyncStatus("running"))
	assert.Equal(t, SyncStatusCompleted, SyncStatus("completed"))
	assert.Equal(t, SyncStatusFailed, SyncStatus("failed"))
}

func TestImportStatusConstants(t *testing.T) {
	assert.Equal(t, ImportStatusPending, ImportStatus("pending"))
	assert.Equal(t, ImportStatusRunning, ImportStatus("running"))
	assert.Equal(t, ImportStatusCompleted, ImportStatus("completed"))
	assert.Equal(t, ImportStatusFailed, ImportStatus("failed"))
}

func TestOAuthProviderConstants(t *testing.T) {
	assert.Equal(t, OAuthProviderDropbox, OAuthProvider("dropbox"))
}
