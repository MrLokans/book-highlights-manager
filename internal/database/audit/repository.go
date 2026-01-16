package audit

import (
	"time"

	"gorm.io/gorm"

	"github.com/mrlokans/assistant/internal/entities"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// LogEvent saves an audit event to the database.
func (r *Repository) LogEvent(event *entities.AuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	return r.db.Create(event).Error
}

// GetEvents retrieves paginated audit events for a user, ordered by most recent first.
func (r *Repository) GetEvents(userID uint, limit, offset int) ([]entities.AuditEvent, int64, error) {
	var events []entities.AuditEvent
	var total int64

	query := r.db.Model(&entities.AuditEvent{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&events).Error
	return events, total, err
}

// GetEventsByType retrieves audit events filtered by type.
func (r *Repository) GetEventsByType(eventType entities.AuditEventType, userID uint, limit, offset int) ([]entities.AuditEvent, int64, error) {
	var events []entities.AuditEvent
	var total int64

	query := r.db.Model(&entities.AuditEvent{}).Where("event_type = ?", eventType)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&events).Error
	return events, total, err
}

// GetRecentEvents retrieves audit events since a specific time.
func (r *Repository) GetRecentEvents(userID uint, since time.Time) ([]entities.AuditEvent, error) {
	var events []entities.AuditEvent
	query := r.db.Where("created_at > ?", since).Order("created_at DESC")
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Find(&events).Error
	return events, err
}

// DeleteOldEvents removes audit events older than the specified time.
// Returns the number of deleted events.
func (r *Repository) DeleteOldEvents(olderThan time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", olderThan).Delete(&entities.AuditEvent{})
	return result.RowsAffected, result.Error
}

// GetEventByID retrieves a single audit event by ID.
func (r *Repository) GetEventByID(id uint) (*entities.AuditEvent, error) {
	var event entities.AuditEvent
	err := r.db.First(&event, id).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}
