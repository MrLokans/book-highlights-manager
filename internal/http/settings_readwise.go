package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/readwise"
	"github.com/mrlokans/assistant/internal/scheduler"
	"github.com/mrlokans/assistant/internal/settingsstore"
)

// ReadwiseSyncController handles Readwise sync settings and operations
type ReadwiseSyncController struct {
	settingsStore *settingsstore.SettingsStore
	scheduler     *scheduler.ReadwiseSyncScheduler
	client        *readwise.Client
}

// NewReadwiseSyncController creates a new controller
func NewReadwiseSyncController(store *settingsstore.SettingsStore, sched *scheduler.ReadwiseSyncScheduler, client *readwise.Client) *ReadwiseSyncController {
	return &ReadwiseSyncController{
		settingsStore: store,
		scheduler:     sched,
		client:        client,
	}
}

// ReadwiseSyncSettingsResponse is the response for GET /settings/readwise
type ReadwiseSyncSettingsResponse struct {
	Config    settingsstore.ReadwiseSyncConfigInfo `json:"config"`
	Status    settingsstore.ReadwiseSyncStatus     `json:"status"`
	NextRun   *time.Time                           `json:"next_run,omitempty"`
	IsRunning bool                                 `json:"is_running"`
	IsSyncing bool                                 `json:"is_syncing"`
	Presets   []SchedulePreset                     `json:"presets"`
	EnvToken  string                               `json:"env_token"` // Masked env token for UI
}

// GetSettings returns current Readwise sync settings
func (c *ReadwiseSyncController) GetSettings(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Settings store not available"})
		return
	}

	config := c.settingsStore.GetReadwiseSyncConfigInfo()
	status := c.settingsStore.GetReadwiseSyncStatus()

	var nextRun *time.Time
	isRunning := false
	isSyncing := false
	if c.scheduler != nil {
		nextRun = c.scheduler.GetNextRunTime()
		isRunning = c.scheduler.IsRunning()
		isSyncing = c.scheduler.IsSyncing()
	}

	response := ReadwiseSyncSettingsResponse{
		Config:    config,
		Status:    status,
		NextRun:   nextRun,
		IsRunning: isRunning,
		IsSyncing: isSyncing,
		Presets: []SchedulePreset{
			{Label: "Every 15 minutes", Value: "*/15 * * * *", Description: "Runs at :00, :15, :30, :45"},
			{Label: "Every 30 minutes", Value: "*/30 * * * *", Description: "Runs at :00, :30"},
			{Label: "Every hour", Value: "0 * * * *", Description: "Runs at the top of every hour"},
			{Label: "Every 6 hours", Value: "0 */6 * * *", Description: "Runs at midnight, 6am, noon, 6pm"},
			{Label: "Daily at midnight", Value: "0 0 * * *", Description: "Runs once daily at 00:00"},
			{Label: "Weekly on Sunday", Value: "0 0 * * 0", Description: "Runs every Sunday at midnight"},
		},
	}

	// Check if request prefers JSON or HTML
	if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
		ctx.JSON(http.StatusOK, response)
	} else {
		ctx.HTML(http.StatusOK, "readwise-sync-settings", response)
	}
}

// UpdateReadwiseSettingsRequest is the request body for POST /settings/readwise/save
type UpdateReadwiseSettingsRequest struct {
	Enabled  *bool  `form:"enabled" json:"enabled"`
	Token    string `form:"token" json:"token"`
	Schedule string `form:"schedule" json:"schedule"`
}

// UpdateSettings saves Readwise sync settings
func (c *ReadwiseSyncController) UpdateSettings(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Settings store not available",
		})
		return
	}

	var req UpdateReadwiseSettingsRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.HTML(http.StatusBadRequest, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Invalid request: " + err.Error(),
		})
		return
	}

	// Save token if provided
	if req.Token != "" {
		if err := c.settingsStore.SetReadwiseSyncToken(req.Token); err != nil {
			ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
				"Success": false,
				"Error":   "Failed to save token: " + err.Error(),
			})
			return
		}
	}

	// Validate and save schedule if provided
	if req.Schedule != "" {
		if err := settingsstore.ValidateCronSchedule(req.Schedule); err != nil {
			ctx.HTML(http.StatusBadRequest, "readwise-sync-result", gin.H{
				"Success": false,
				"Error":   "Invalid cron schedule: " + err.Error(),
			})
			return
		}
		if err := c.settingsStore.SetReadwiseSyncSchedule(req.Schedule); err != nil {
			ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
				"Success": false,
				"Error":   "Failed to save schedule: " + err.Error(),
			})
			return
		}
	}

	// Save enabled state if provided
	if req.Enabled != nil {
		if err := c.settingsStore.SetReadwiseSyncEnabled(*req.Enabled); err != nil {
			ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
				"Success": false,
				"Error":   "Failed to save enabled state: " + err.Error(),
			})
			return
		}
	}

	// Reschedule the sync job if scheduler is available
	if c.scheduler != nil {
		if err := c.scheduler.Reschedule(); err != nil {
			ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
				"Success": false,
				"Error":   "Settings saved but failed to reschedule: " + err.Error(),
			})
			return
		}
	}

	// Return updated settings
	config := c.settingsStore.GetReadwiseSyncConfigInfo()
	ctx.HTML(http.StatusOK, "readwise-sync-result", gin.H{
		"Success": true,
		"Config":  config,
	})
}

// ResetSettings clears database overrides, reverting to env/defaults
func (c *ReadwiseSyncController) ResetSettings(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Settings store not available",
		})
		return
	}

	if err := c.settingsStore.ClearReadwiseSyncSettings(); err != nil {
		ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Failed to reset settings: " + err.Error(),
		})
		return
	}

	// Reschedule with new settings
	if c.scheduler != nil {
		_ = c.scheduler.Reschedule()
	}

	config := c.settingsStore.GetReadwiseSyncConfigInfo()
	ctx.HTML(http.StatusOK, "readwise-sync-result", gin.H{
		"Success": true,
		"Config":  config,
	})
}

// ValidateTokenRequest is the request body for validating a token
type ValidateTokenRequest struct {
	Token string `form:"token" json:"token"`
}

// ValidateToken validates a Readwise API token
func (c *ReadwiseSyncController) ValidateToken(ctx *gin.Context) {
	if c.client == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"valid": false,
			"error": "Readwise client not available",
		})
		return
	}

	var req ValidateTokenRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": "Invalid request",
		})
		return
	}

	token := req.Token
	if token == "" {
		// If no token in request, use configured token
		token = c.settingsStore.GetReadwiseSyncToken()
	}

	if token == "" {
		ctx.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "No token provided or configured",
		})
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	if err := c.client.ValidateToken(reqCtx, token); err != nil {
		if err == readwise.ErrInvalidToken {
			ctx.JSON(http.StatusOK, gin.H{
				"valid": false,
				"error": "Invalid or expired token",
			})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"valid":   true,
		"message": "Token is valid",
	})
}

// SyncNow triggers an immediate sync
func (c *ReadwiseSyncController) SyncNow(ctx *gin.Context) {
	if c.scheduler == nil {
		ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Scheduler not available",
		})
		return
	}

	// Check if token is configured
	config := c.settingsStore.GetReadwiseSyncConfig()
	if config.Token == "" {
		ctx.HTML(http.StatusBadRequest, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Readwise token not configured. Please configure it first.",
		})
		return
	}

	// Check if already syncing
	if c.scheduler.IsSyncing() {
		ctx.HTML(http.StatusConflict, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Sync already in progress",
		})
		return
	}

	if err := c.scheduler.RunNow(); err != nil {
		ctx.HTML(http.StatusInternalServerError, "readwise-sync-result", gin.H{
			"Success": false,
			"Error":   "Failed to start sync: " + err.Error(),
		})
		return
	}

	ctx.HTML(http.StatusOK, "readwise-sync-result", gin.H{
		"Success": true,
		"Message": "Sync started in background",
	})
}

// GetStatus returns just the sync status (for polling)
func (c *ReadwiseSyncController) GetStatus(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Settings store not available"})
		return
	}

	status := c.settingsStore.GetReadwiseSyncStatus()
	var nextRun *time.Time
	isRunning := false
	isSyncing := false
	if c.scheduler != nil {
		nextRun = c.scheduler.GetNextRunTime()
		isRunning = c.scheduler.IsRunning()
		isSyncing = c.scheduler.IsSyncing()
	}

	response := gin.H{
		"status":     status,
		"next_run":   nextRun,
		"is_running": isRunning,
		"is_syncing": isSyncing,
	}

	if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
		ctx.JSON(http.StatusOK, response)
	} else {
		ctx.HTML(http.StatusOK, "readwise-sync-status", response)
	}
}
