package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&entities.AuditEvent{})
	require.NoError(t, err)

	return db
}

func TestRepository_LogEvent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	event := &entities.AuditEvent{
		UserID:      1,
		EventType:   entities.AuditEventImport,
		Action:      "kindle_import",
		Description: "Imported 10 books from Kindle",
		Status:      entities.AuditStatusSuccess,
	}

	err := repo.LogEvent(event)
	require.NoError(t, err)
	assert.NotZero(t, event.ID)
	assert.False(t, event.CreatedAt.IsZero())
}

func TestRepository_GetEvents(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create test events
	for i := 0; i < 15; i++ {
		event := &entities.AuditEvent{
			UserID:      1,
			EventType:   entities.AuditEventImport,
			Action:      "test_import",
			Description: "Test event",
			Status:      entities.AuditStatusSuccess,
			CreatedAt:   time.Now().Add(time.Duration(-i) * time.Hour),
		}
		err := repo.LogEvent(event)
		require.NoError(t, err)
	}

	// Add events for different user
	for i := 0; i < 5; i++ {
		event := &entities.AuditEvent{
			UserID:      2,
			EventType:   entities.AuditEventDelete,
			Action:      "test_delete",
			Description: "Test delete event",
			Status:      entities.AuditStatusSuccess,
		}
		err := repo.LogEvent(event)
		require.NoError(t, err)
	}

	t.Run("get all events", func(t *testing.T) {
		events, total, err := repo.GetEvents(0, 50, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(20), total)
		assert.Len(t, events, 20)
	})

	t.Run("get user events", func(t *testing.T) {
		events, total, err := repo.GetEvents(1, 50, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(15), total)
		assert.Len(t, events, 15)
	})

	t.Run("pagination", func(t *testing.T) {
		events, total, err := repo.GetEvents(1, 5, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(15), total)
		assert.Len(t, events, 5)

		events2, _, err := repo.GetEvents(1, 5, 5)
		require.NoError(t, err)
		assert.Len(t, events2, 5)
		assert.NotEqual(t, events[0].ID, events2[0].ID)
	})

	t.Run("order by created_at desc", func(t *testing.T) {
		events, _, err := repo.GetEvents(1, 10, 0)
		require.NoError(t, err)
		for i := 1; i < len(events); i++ {
			assert.True(t, events[i-1].CreatedAt.After(events[i].CreatedAt) || events[i-1].CreatedAt.Equal(events[i].CreatedAt))
		}
	})
}

func TestRepository_GetEventsByType(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	// Create mixed events
	importEvent := &entities.AuditEvent{
		UserID:      1,
		EventType:   entities.AuditEventImport,
		Action:      "kindle_import",
		Description: "Import event",
		Status:      entities.AuditStatusSuccess,
	}
	deleteEvent := &entities.AuditEvent{
		UserID:      1,
		EventType:   entities.AuditEventDelete,
		Action:      "book_delete",
		Description: "Delete event",
		Status:      entities.AuditStatusSuccess,
	}

	require.NoError(t, repo.LogEvent(importEvent))
	require.NoError(t, repo.LogEvent(deleteEvent))
	require.NoError(t, repo.LogEvent(&entities.AuditEvent{
		UserID:    1,
		EventType: entities.AuditEventImport,
		Action:    "apple_import",
		Status:    entities.AuditStatusSuccess,
	}))

	events, total, err := repo.GetEventsByType(entities.AuditEventImport, 1, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, events, 2)
	for _, e := range events {
		assert.Equal(t, entities.AuditEventImport, e.EventType)
	}
}

func TestRepository_GetRecentEvents(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	now := time.Now()

	// Create events at different times
	oldEvent := &entities.AuditEvent{
		UserID:    1,
		EventType: entities.AuditEventImport,
		Action:    "old_import",
		Status:    entities.AuditStatusSuccess,
		CreatedAt: now.Add(-48 * time.Hour),
	}
	recentEvent := &entities.AuditEvent{
		UserID:    1,
		EventType: entities.AuditEventDelete,
		Action:    "recent_delete",
		Status:    entities.AuditStatusSuccess,
		CreatedAt: now.Add(-1 * time.Hour),
	}

	require.NoError(t, repo.LogEvent(oldEvent))
	require.NoError(t, repo.LogEvent(recentEvent))

	events, err := repo.GetRecentEvents(1, now.Add(-24*time.Hour))
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "recent_delete", events[0].Action)
}

func TestRepository_DeleteOldEvents(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	now := time.Now()

	// Create old and new events
	oldEvent := &entities.AuditEvent{
		UserID:    1,
		EventType: entities.AuditEventImport,
		Action:    "old_import",
		Status:    entities.AuditStatusSuccess,
		CreatedAt: now.Add(-48 * time.Hour),
	}
	newEvent := &entities.AuditEvent{
		UserID:    1,
		EventType: entities.AuditEventDelete,
		Action:    "new_delete",
		Status:    entities.AuditStatusSuccess,
		CreatedAt: now.Add(-1 * time.Hour),
	}

	require.NoError(t, repo.LogEvent(oldEvent))
	require.NoError(t, repo.LogEvent(newEvent))

	// Delete events older than 24 hours
	deleted, err := repo.DeleteOldEvents(now.Add(-24 * time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Verify only new event remains
	events, total, err := repo.GetEvents(0, 50, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, events, 1)
	assert.Equal(t, "new_delete", events[0].Action)
}

func TestRepository_GetEventByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	event := &entities.AuditEvent{
		UserID:      1,
		EventType:   entities.AuditEventImport,
		Action:      "test_import",
		Description: "Test event",
		Status:      entities.AuditStatusSuccess,
	}

	require.NoError(t, repo.LogEvent(event))

	t.Run("existing event", func(t *testing.T) {
		found, err := repo.GetEventByID(event.ID)
		require.NoError(t, err)
		assert.Equal(t, event.ID, found.ID)
		assert.Equal(t, "test_import", found.Action)
	})

	t.Run("non-existing event", func(t *testing.T) {
		_, err := repo.GetEventByID(999)
		assert.Error(t, err)
	})
}
