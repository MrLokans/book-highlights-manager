package audit

import (
	"encoding/json"
	"log"
	"time"

	"github.com/mrlokans/assistant/internal/database/audit"
	"github.com/mrlokans/assistant/internal/entities"
)

// Service provides high-level audit logging functionality.
type Service struct {
	repo *audit.Repository
}

// NewService creates a new audit service.
func NewService(repo *audit.Repository) *Service {
	return &Service{repo: repo}
}

// Log records a generic audit event.
func (s *Service) Log(event *entities.AuditEvent) error {
	return s.repo.LogEvent(event)
}

// LogAsync records an audit event in the background (non-blocking).
func (s *Service) LogAsync(event *entities.AuditEvent) {
	go func() {
		if err := s.repo.LogEvent(event); err != nil {
			log.Printf("Failed to log audit event: %v", err)
		}
	}()
}

// LogImport records an import event.
func (s *Service) LogImport(userID uint, source, description string, booksCount, highlightsCount int, err error) {
	event := &entities.AuditEvent{
		UserID:      userID,
		EventType:   entities.AuditEventImport,
		Action:      source + "_import",
		Description: description,
		EntityType:  "book",
		Status:      entities.AuditStatusSuccess,
	}

	metadata := map[string]any{
		"books_count":      booksCount,
		"highlights_count": highlightsCount,
	}
	if mdBytes, e := json.Marshal(metadata); e == nil {
		event.Metadata = string(mdBytes)
	}

	if err != nil {
		event.Status = entities.AuditStatusFailed
		event.ErrorMsg = truncate(err.Error(), 500)
	}

	s.LogAsync(event)
}

// LogExport records an export event.
func (s *Service) LogExport(userID uint, description string, err error) {
	event := &entities.AuditEvent{
		UserID:      userID,
		EventType:   entities.AuditEventExport,
		Action:      "markdown_export",
		Description: description,
		Status:      entities.AuditStatusSuccess,
	}

	if err != nil {
		event.Status = entities.AuditStatusFailed
		event.ErrorMsg = truncate(err.Error(), 500)
	}

	s.LogAsync(event)
}

// LogDelete records a deletion event.
func (s *Service) LogDelete(userID uint, entityType string, entityID uint, entityName string, permanent bool) {
	action := entityType + "_delete"
	if permanent {
		action = entityType + "_delete_permanent"
	}

	event := &entities.AuditEvent{
		UserID:      userID,
		EventType:   entities.AuditEventDelete,
		Action:      action,
		Description: "Deleted " + entityType + ": " + entityName,
		EntityType:  entityType,
		EntityID:    &entityID,
		Status:      entities.AuditStatusSuccess,
	}

	s.LogAsync(event)
}

// LogAuth records an authentication event.
func (s *Service) LogAuth(userID uint, action string, ipAddr, userAgent string, success bool) {
	event := &entities.AuditEvent{
		UserID:    userID,
		EventType: entities.AuditEventAuth,
		Action:    action,
		IPAddress: ipAddr,
		UserAgent: truncate(userAgent, 500),
		Status:    entities.AuditStatusSuccess,
	}

	if !success {
		event.Status = entities.AuditStatusFailed
	}

	s.LogAsync(event)
}

// LogSettings records a settings change event.
func (s *Service) LogSettings(userID uint, action, description string) {
	event := &entities.AuditEvent{
		UserID:      userID,
		EventType:   entities.AuditEventSettings,
		Action:      action,
		Description: description,
		Status:      entities.AuditStatusSuccess,
	}

	s.LogAsync(event)
}

// LogMetadataEnrich records a metadata enrichment event.
func (s *Service) LogMetadataEnrich(userID uint, description string, bookID uint, err error) {
	event := &entities.AuditEvent{
		UserID:      userID,
		EventType:   entities.AuditEventMetadataEnrich,
		Action:      "book_enrich",
		Description: description,
		EntityType:  "book",
		EntityID:    &bookID,
		Status:      entities.AuditStatusSuccess,
	}

	if err != nil {
		event.Status = entities.AuditStatusFailed
		event.ErrorMsg = truncate(err.Error(), 500)
	}

	s.LogAsync(event)
}

// LogSync records a sync event.
func (s *Service) LogSync(userID uint, action, description string, err error) {
	event := &entities.AuditEvent{
		UserID:      userID,
		EventType:   entities.AuditEventSync,
		Action:      action,
		Description: description,
		Status:      entities.AuditStatusSuccess,
	}

	if err != nil {
		event.Status = entities.AuditStatusFailed
		event.ErrorMsg = truncate(err.Error(), 500)
	}

	s.LogAsync(event)
}

// GetEvents retrieves paginated audit events.
func (s *Service) GetEvents(userID uint, limit, offset int) ([]entities.AuditEvent, int64, error) {
	return s.repo.GetEvents(userID, limit, offset)
}

// GetEventsByType retrieves audit events filtered by type.
func (s *Service) GetEventsByType(eventType entities.AuditEventType, userID uint, limit, offset int) ([]entities.AuditEvent, int64, error) {
	return s.repo.GetEventsByType(eventType, userID, limit, offset)
}

// DeleteOldEvents removes events older than the specified duration.
func (s *Service) DeleteOldEvents(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	return s.repo.DeleteOldEvents(cutoff)
}

// truncate shortens a string to max length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
