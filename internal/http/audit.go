package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mrlokans/assistant/internal/audit"
	"github.com/mrlokans/assistant/internal/auth"
	"github.com/mrlokans/assistant/internal/entities"
)

type AuditController struct {
	auditService *audit.Service
}

func NewAuditController(auditService *audit.Service) *AuditController {
	return &AuditController{
		auditService: auditService,
	}
}

// AuditLogPage renders the audit log UI
// GET /audit
func (ac *AuditController) AuditLogPage(c *gin.Context) {
	userID := c.GetUint(auth.ContextKeyUserID)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	eventType := c.Query("type")
	limit := 25
	offset := (page - 1) * limit

	var events []entities.AuditEvent
	var total int64
	var err error

	if eventType != "" {
		events, total, err = ac.auditService.GetEventsByType(entities.AuditEventType(eventType), userID, limit, offset)
	} else {
		events, total, err = ac.auditService.GetEvents(userID, limit, offset)
	}

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error", gin.H{
			"Error": "Failed to load audit events",
		})
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	c.HTML(http.StatusOK, "audit", gin.H{
		"Events":      events,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"TotalEvents": total,
		"EventType":   eventType,
		"EventTypes":  getEventTypes(),
	})
}

// GetAuditEvents returns paginated audit events as JSON
// GET /api/audit
func (ac *AuditController) GetAuditEvents(c *gin.Context) {
	userID := c.GetUint(auth.ContextKeyUserID)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}

	eventType := c.Query("type")
	offset := (page - 1) * limit

	var events []entities.AuditEvent
	var total int64
	var err error

	if eventType != "" {
		events, total, err = ac.auditService.GetEventsByType(entities.AuditEventType(eventType), userID, limit, offset)
	} else {
		events, total, err = ac.auditService.GetEvents(userID, limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load audit events",
		})
		return
	}

	totalPages := (int(total) + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"events":       events,
		"page":         page,
		"limit":        limit,
		"total_pages":  totalPages,
		"total_events": total,
	})
}

func getEventTypes() []EventTypeOption {
	return []EventTypeOption{
		{Value: "", Label: "All Events"},
		{Value: string(entities.AuditEventImport), Label: "Import"},
		{Value: string(entities.AuditEventExport), Label: "Export"},
		{Value: string(entities.AuditEventDelete), Label: "Delete"},
		{Value: string(entities.AuditEventMetadataEnrich), Label: "Metadata Enrichment"},
		{Value: string(entities.AuditEventSync), Label: "Sync"},
		{Value: string(entities.AuditEventAuth), Label: "Authentication"},
		{Value: string(entities.AuditEventSettings), Label: "Settings"},
	}
}

type EventTypeOption struct {
	Value string
	Label string
}
