package audit

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	auditRepo "github.com/mrlokans/assistant/internal/database/audit"
	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestService(t *testing.T) (*Service, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&entities.AuditEvent{})
	require.NoError(t, err)

	repo := auditRepo.NewRepository(db)
	svc := NewService(repo)

	return svc, db
}

func TestService_Log(t *testing.T) {
	svc, db := setupTestService(t)

	event := &entities.AuditEvent{
		UserID:      1,
		EventType:   entities.AuditEventImport,
		Action:      "test_import",
		Description: "Test import event",
		Status:      entities.AuditStatusSuccess,
	}

	err := svc.Log(event)
	require.NoError(t, err)

	var saved entities.AuditEvent
	err = db.First(&saved, event.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "test_import", saved.Action)
}

func TestService_LogImport(t *testing.T) {
	svc, db := setupTestService(t)

	t.Run("successful import", func(t *testing.T) {
		svc.LogImport(1, "kindle", "Imported 5 books with 100 highlights", 5, 100, nil)

		// Allow async operation to complete
		time.Sleep(50 * time.Millisecond)

		var event entities.AuditEvent
		err := db.Where("action = ?", "kindle_import").First(&event).Error
		require.NoError(t, err)
		assert.Equal(t, entities.AuditStatusSuccess, event.Status)
		assert.Equal(t, "Imported 5 books with 100 highlights", event.Description)
		assert.Contains(t, event.Metadata, "books_count")
		assert.Contains(t, event.Metadata, "highlights_count")
	})

	t.Run("failed import", func(t *testing.T) {
		svc.LogImport(1, "readwise", "Import failed", 0, 0, errors.New("connection timeout"))

		time.Sleep(50 * time.Millisecond)

		var event entities.AuditEvent
		err := db.Where("action = ?", "readwise_import").First(&event).Error
		require.NoError(t, err)
		assert.Equal(t, entities.AuditStatusFailed, event.Status)
		assert.Contains(t, event.ErrorMsg, "connection timeout")
	})
}

func TestService_LogDelete(t *testing.T) {
	svc, db := setupTestService(t)

	t.Run("soft delete", func(t *testing.T) {
		svc.LogDelete(1, "book", 42, "The Great Gatsby", false)

		time.Sleep(50 * time.Millisecond)

		var event entities.AuditEvent
		err := db.Where("action = ?", "book_delete").First(&event).Error
		require.NoError(t, err)
		assert.Equal(t, entities.AuditEventDelete, event.EventType)
		assert.Equal(t, "book", event.EntityType)
		assert.NotNil(t, event.EntityID)
		assert.Equal(t, uint(42), *event.EntityID)
	})

	t.Run("permanent delete", func(t *testing.T) {
		svc.LogDelete(1, "highlight", 123, "Sample highlight", true)

		time.Sleep(50 * time.Millisecond)

		var event entities.AuditEvent
		err := db.Where("action = ?", "highlight_delete_permanent").First(&event).Error
		require.NoError(t, err)
		assert.Equal(t, entities.AuditEventDelete, event.EventType)
	})
}

func TestService_LogAuth(t *testing.T) {
	svc, db := setupTestService(t)

	t.Run("successful login", func(t *testing.T) {
		svc.LogAuth(1, "login", "192.168.1.1", "Mozilla/5.0", true)

		time.Sleep(50 * time.Millisecond)

		var event entities.AuditEvent
		err := db.Where("action = ?", "login").First(&event).Error
		require.NoError(t, err)
		assert.Equal(t, entities.AuditStatusSuccess, event.Status)
		assert.Equal(t, "192.168.1.1", event.IPAddress)
	})

	t.Run("failed login", func(t *testing.T) {
		svc.LogAuth(0, "login_failed", "10.0.0.1", "curl/7.68.0", false)

		time.Sleep(50 * time.Millisecond)

		var event entities.AuditEvent
		err := db.Where("action = ?", "login_failed").First(&event).Error
		require.NoError(t, err)
		assert.Equal(t, entities.AuditStatusFailed, event.Status)
	})
}

func TestService_LogSettings(t *testing.T) {
	svc, db := setupTestService(t)

	svc.LogSettings(1, "dropbox_connected", "Connected Dropbox account: user@example.com")

	time.Sleep(50 * time.Millisecond)

	var event entities.AuditEvent
	err := db.Where("action = ?", "dropbox_connected").First(&event).Error
	require.NoError(t, err)
	assert.Equal(t, entities.AuditEventSettings, event.EventType)
	assert.Contains(t, event.Description, "user@example.com")
}

func TestService_GetEvents(t *testing.T) {
	svc, _ := setupTestService(t)

	// Create some events synchronously
	for i := 0; i < 5; i++ {
		err := svc.Log(&entities.AuditEvent{
			UserID:    1,
			EventType: entities.AuditEventImport,
			Action:    "test",
			Status:    entities.AuditStatusSuccess,
		})
		require.NoError(t, err)
	}

	events, total, err := svc.GetEvents(1, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, events, 5)
}

func TestService_DeleteOldEvents(t *testing.T) {
	svc, db := setupTestService(t)

	// Create old event
	oldEvent := &entities.AuditEvent{
		UserID:    1,
		EventType: entities.AuditEventImport,
		Action:    "old",
		Status:    entities.AuditStatusSuccess,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}
	require.NoError(t, db.Create(oldEvent).Error)

	// Create new event
	newEvent := &entities.AuditEvent{
		UserID:    1,
		EventType: entities.AuditEventDelete,
		Action:    "new",
		Status:    entities.AuditStatusSuccess,
		CreatedAt: time.Now(),
	}
	require.NoError(t, db.Create(newEvent).Error)

	// Delete events older than 24 hours
	deleted, err := svc.DeleteOldEvents(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	var remaining []entities.AuditEvent
	db.Find(&remaining)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "new", remaining[0].Action)
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10c", 10, "exactly10c"},
		{"this is a very long string", 10, "this is..."},
		{"", 5, ""},
	}

	for _, tc := range tests {
		result := truncate(tc.input, tc.maxLen)
		assert.Equal(t, tc.expected, result)
	}
}
