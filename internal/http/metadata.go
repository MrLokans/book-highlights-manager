package http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/mrlokans/assistant/internal/tasks"
)

const (
	enrichTimeout     = 30 * time.Second
	enrichISBNTimeout = 5 * time.Second
)

// SyncProgressReader reports the state of the bulk metadata sync.
type SyncProgressReader interface {
	IsSyncRunning() (bool, error)
	GetSyncProgress() (*entities.SyncProgress, error)
}

// MetadataController handles book metadata enrichment endpoints.
type MetadataController struct {
	enricher     *metadata.Enricher
	syncProgress SyncProgressReader
	taskClient   *tasks.Client
}

// NewMetadataController creates a new MetadataController.
func NewMetadataController(enricher *metadata.Enricher, syncProgress SyncProgressReader, taskClient *tasks.Client) *MetadataController {
	return &MetadataController{
		enricher:     enricher,
		syncProgress: syncProgress,
		taskClient:   taskClient,
	}
}

// EnrichBookRequest is the request body for enriching a book.
type EnrichBookRequest struct {
	ISBN string `json:"isbn,omitempty"`
}

// EnrichBookResponse is the response for an enrichment operation.
type EnrichBookResponse struct {
	Success       bool     `json:"success"`
	Book          any      `json:"book,omitempty"`
	FieldsUpdated []string `json:"fields_updated,omitempty"`
	Source        string   `json:"source,omitempty"`
	SearchMethod  string   `json:"search_method,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// EnrichBook handles POST /api/books/:id/enrich
// It fetches metadata from OpenLibrary and updates the book.
// Supports both JSON API and HTMX (HTML fragment) responses.
func (mc *MetadataController) EnrichBook(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		mc.respondError(c, "invalid book ID")
		return
	}

	// Parse optional ISBN from request body or form data
	var isbn string
	contentType := c.ContentType()
	if contentType == "application/json" {
		var req EnrichBookRequest
		if c.ShouldBindJSON(&req) == nil {
			isbn = req.ISBN
		}
	} else {
		isbn = c.PostForm("isbn")
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), enrichTimeout)
	defer cancel()

	var result *metadata.EnrichmentResult

	if isbn != "" {
		result, err = mc.enricher.EnrichBookWithISBN(ctx, uint(id), isbn)
	} else {
		result, err = mc.enricher.EnrichBook(ctx, uint(id))
	}

	if err != nil {
		mc.respondError(c, err.Error())
		return
	}

	mc.respondSuccess(c, result)
}

func (mc *MetadataController) respondError(c *gin.Context, errorMsg string) {
	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "enrichment-result", gin.H{"Success": false, "Error": errorMsg})
		return
	}

	c.JSON(http.StatusInternalServerError, EnrichBookResponse{
		Success: false,
		Error:   errorMsg,
	})
}

func (mc *MetadataController) respondSuccess(c *gin.Context, result *metadata.EnrichmentResult) {
	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "enrichment-result", gin.H{
			"Success":       true,
			"FieldsUpdated": strings.Join(result.FieldsUpdated, ", "),
			"SearchMethod":  result.SearchMethod,
		})
		return
	}

	c.JSON(http.StatusOK, EnrichBookResponse{
		Success:       true,
		Book:          result.Book,
		FieldsUpdated: result.FieldsUpdated,
		Source:        result.Source,
		SearchMethod:  result.SearchMethod,
	})
}

// EnrichAllMissing handles POST /api/books/enrich-all
// It starts an async enrichment of all books missing metadata (cover, publisher, year).
// Requires the task queue to be enabled.
func (mc *MetadataController) EnrichAllMissing(c *gin.Context) {
	if mc.taskClient == nil {
		mc.respondBulkError(c, "task queue is not enabled")
		return
	}

	// Check if sync is already running
	if mc.syncProgress != nil {
		running, err := mc.syncProgress.IsSyncRunning()
		if err == nil && running {
			mc.respondBulkError(c, "metadata sync is already in progress")
			return
		}
	}

	task := tasks.EnrichAllBooksTask{}
	ids, err := mc.taskClient.Add(task).Save()
	if err != nil {
		slog.Error("Failed to enqueue enrichment task", "error", err)
		mc.respondBulkError(c, "failed to start enrichment task")
		return
	}
	slog.Info("Enqueued EnrichAllBooksTask", "task_id", ids[0])

	// Return immediately with a "started" response
	if isHTMXRequest(c) {
		c.Header("HX-Retarget", "#sync-status")
		c.Header("HX-Reswap", "outerHTML")
		c.HTML(http.StatusOK, "metadata-sync-started", nil)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "metadata sync started",
	})
}

func (mc *MetadataController) respondBulkError(c *gin.Context, errorMsg string) {
	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "settings-error", gin.H{"Error": errorMsg})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"success": false,
		"error":   errorMsg,
	})
}

// UpdateISBN handles PATCH /api/books/:id/isbn
// It allows setting the ISBN on a book without full enrichment.
func (mc *MetadataController) UpdateISBN(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid book ID"})
		return
	}

	var req struct {
		ISBN string `json:"isbn" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "isbn is required"})
		return
	}

	// Use the enricher's database to update just the ISBN
	ctx, cancel := context.WithTimeout(c.Request.Context(), enrichISBNTimeout)
	defer cancel()

	result, err := mc.enricher.EnrichBookWithISBN(ctx, uint(id), req.ISBN)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"book":           result.Book,
		"fields_updated": result.FieldsUpdated,
	})
}

// SyncStatusResponse represents the metadata sync status.
type SyncStatusResponse struct {
	Running     bool    `json:"running"`
	TotalItems  int     `json:"total_items,omitempty"`
	Processed   int     `json:"processed,omitempty"`
	Succeeded   int     `json:"succeeded,omitempty"`
	Failed      int     `json:"failed,omitempty"`
	Skipped     int     `json:"skipped,omitempty"`
	CurrentItem string  `json:"current_item,omitempty"`
	Progress    float64 `json:"progress,omitempty"` // 0-100 percentage
}

// GetSyncStatus handles GET /api/sync/metadata/status
// Returns the current status of metadata sync operation.
func (mc *MetadataController) GetSyncStatus(c *gin.Context) {
	resp := SyncStatusResponse{Running: false}

	if mc.syncProgress != nil {
		progress, err := mc.syncProgress.GetSyncProgress()
		if err == nil {
			resp.Running = progress.Status == entities.SyncStatusRunning
			resp.TotalItems = progress.TotalItems
			resp.Processed = progress.Processed
			resp.Succeeded = progress.Succeeded
			resp.Failed = progress.Failed
			resp.Skipped = progress.Skipped
			resp.CurrentItem = progress.CurrentItem

			if progress.TotalItems > 0 {
				resp.Progress = float64(progress.Processed) / float64(progress.TotalItems) * 100
			}
		}
	}

	// For HTMX requests, return HTML fragment with status
	if isHTMXRequest(c) {
		mc.respondSyncStatusHTML(c, resp)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (mc *MetadataController) respondSyncStatusHTML(c *gin.Context, resp SyncStatusResponse) {
	c.HTML(http.StatusOK, "metadata-sync-status", resp)
}
