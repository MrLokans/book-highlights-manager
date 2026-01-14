package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/database"
	"github.com/mrlokans/assistant/internal/entities"
	"github.com/mrlokans/assistant/internal/metadata"
	"github.com/mrlokans/assistant/internal/tasks"
)

// MetadataController handles book metadata enrichment endpoints.
type MetadataController struct {
	enricher     *metadata.Enricher
	syncProgress *database.MetadataSyncProgress
	taskClient   *tasks.Client
}

// NewMetadataController creates a new MetadataController.
func NewMetadataController(enricher *metadata.Enricher, syncProgress *database.MetadataSyncProgress, taskClient *tasks.Client) *MetadataController {
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
	Success       bool                       `json:"success"`
	Book          any                        `json:"book,omitempty"`
	FieldsUpdated []string                   `json:"fields_updated,omitempty"`
	Source        string                     `json:"source,omitempty"`
	SearchMethod  string                     `json:"search_method,omitempty"`
	Error         string                     `json:"error,omitempty"`
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
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
		html := fmt.Sprintf(`<div class="enrichment-error">
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
			<span>%s</span>
		</div>`, errorMsg)
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
		return
	}

	c.JSON(http.StatusInternalServerError, EnrichBookResponse{
		Success: false,
		Error:   errorMsg,
	})
}

func (mc *MetadataController) respondSuccess(c *gin.Context, result *metadata.EnrichmentResult) {
	if isHTMXRequest(c) {
		fieldsMsg := "No new fields updated"
		if len(result.FieldsUpdated) > 0 {
			fieldsMsg = fmt.Sprintf("Updated: %s", strings.Join(result.FieldsUpdated, ", "))
		}

		html := fmt.Sprintf(`<div class="enrichment-success">
			<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
			<span>Metadata enriched from OpenLibrary</span>
		</div>
		<div class="enrichment-fields">%s (via %s search)</div>
		<script>setTimeout(function() { window.location.reload(); }, 1500);</script>`,
			fieldsMsg, result.SearchMethod)

		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
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

func isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
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
		log.Printf("Failed to enqueue enrichment task: %v", err)
		mc.respondBulkError(c, "failed to start enrichment task")
		return
	}
	log.Printf("Enqueued EnrichAllBooksTask with ID: %s", ids[0])

	// Return immediately with a "started" response
	if isHTMXRequest(c) {
		// Return the progress UI that will poll for updates
		html := `<div class="sync-progress" id="sync-status" hx-get="/api/sync/metadata/status" hx-trigger="every 1s" hx-swap="outerHTML">
			<div class="sync-progress-header">
				<span class="spinner"></span>
				<span>Starting metadata sync...</span>
			</div>
		</div>`
		c.Header("Content-Type", "text/html")
		c.Header("HX-Retarget", "#sync-status")
		c.Header("HX-Reswap", "outerHTML")
		c.String(http.StatusOK, html)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "metadata sync started",
	})
}

func (mc *MetadataController) respondBulkError(c *gin.Context, errorMsg string) {
	if isHTMXRequest(c) {
		html := fmt.Sprintf(`<div class="import-result import-error">
			<div class="import-result-header">
				<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
				<span>Enrichment Failed</span>
			</div>
			<p class="import-error-message">%s</p>
		</div>`, errorMsg)
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
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
		progress, err := mc.syncProgress.GetProgress()
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
	if resp.Running {
		html := fmt.Sprintf(`<div class="sync-progress" id="sync-status" hx-get="/api/sync/metadata/status" hx-trigger="every 1s" hx-swap="outerHTML">
			<div class="sync-progress-header">
				<span class="spinner"></span>
				<span>Syncing metadata...</span>
			</div>
			<div class="sync-progress-bar">
				<div class="sync-progress-fill" style="width: %.1f%%"></div>
			</div>
			<div class="sync-progress-details">
				<span>%d / %d books</span>
				<span class="sync-current-item" title="%s">%s</span>
			</div>
			<div class="sync-progress-stats">
				<span class="stat-success">%d enriched</span>
				<span class="stat-skip">%d skipped</span>
				<span class="stat-fail">%d failed</span>
			</div>
		</div>`, resp.Progress, resp.Processed, resp.TotalItems, resp.CurrentItem, truncateString(resp.CurrentItem, 40), resp.Succeeded, resp.Skipped, resp.Failed)
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
		return
	}

	// Not running - check if we have results to show (sync just completed)
	if resp.TotalItems > 0 {
		html := fmt.Sprintf(`<div id="sync-status">
			<div class="import-result import-success" style="margin-bottom: 1rem;">
				<div class="import-result-header">
					<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
					<span>Metadata Sync Complete</span>
				</div>
				<div class="import-stats">
					<div class="import-stat">
						<span class="stat-value">%d</span>
						<span class="stat-label">checked</span>
					</div>
					<div class="import-stat">
						<span class="stat-value">%d</span>
						<span class="stat-label">enriched</span>
					</div>
					<div class="import-stat">
						<span class="stat-value">%d</span>
						<span class="stat-label">skipped</span>
					</div>
					<div class="import-stat">
						<span class="stat-value">%d</span>
						<span class="stat-label">failed</span>
					</div>
				</div>
			</div>
			<button
				class="btn btn-primary"
				hx-post="/api/books/enrich-all"
				hx-target="#sync-status"
				hx-swap="outerHTML"
				hx-confirm="This will fetch metadata for all books missing data. Continue?"
			>
				Sync All Missing Metadata
			</button>
		</div>`, resp.TotalItems, resp.Succeeded, resp.Skipped, resp.Failed)
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
		return
	}

	// No previous sync - just show the button
	html := `<div id="sync-status">
		<button
			class="btn btn-primary"
			hx-post="/api/books/enrich-all"
			hx-target="#sync-status"
			hx-swap="outerHTML"
			hx-confirm="This will fetch metadata for all books missing data. Continue?"
		>
			Sync All Missing Metadata
		</button>
	</div>`
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
