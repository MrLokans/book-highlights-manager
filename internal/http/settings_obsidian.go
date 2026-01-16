package http

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mrlokans/assistant/internal/scheduler"
	"github.com/mrlokans/assistant/internal/settingsstore"
)

// ObsidianSyncController handles Obsidian sync settings and operations
type ObsidianSyncController struct {
	settingsStore *settingsstore.SettingsStore
	scheduler     *scheduler.ObsidianSyncScheduler
}

// NewObsidianSyncController creates a new controller
func NewObsidianSyncController(store *settingsstore.SettingsStore, sched *scheduler.ObsidianSyncScheduler) *ObsidianSyncController {
	return &ObsidianSyncController{
		settingsStore: store,
		scheduler:     sched,
	}
}

// ObsidianSyncSettingsResponse is the response for GET /settings/obsidian
type ObsidianSyncSettingsResponse struct {
	Config       settingsstore.ObsidianSyncConfigInfo `json:"config"`
	Status       settingsstore.ObsidianSyncStatus     `json:"status"`
	NextRun      *time.Time                           `json:"next_run,omitempty"`
	IsRunning    bool                                 `json:"is_running"`
	Presets      []SchedulePreset                     `json:"presets"`
	EnvExportDir string                               `json:"env_export_dir"` // Show env value for UI
}

// SchedulePreset is a predefined schedule option
type SchedulePreset struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// GetSettings returns current Obsidian sync settings
func (c *ObsidianSyncController) GetSettings(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Settings store not available"})
		return
	}

	config := c.settingsStore.GetObsidianSyncConfigInfo()
	status := c.settingsStore.GetObsidianSyncStatus()

	var nextRun *time.Time
	isRunning := false
	if c.scheduler != nil {
		nextRun = c.scheduler.GetNextRunTime()
		isRunning = c.scheduler.IsRunning()
	}

	// Check both new and legacy env vars for display
	envExportDir := os.Getenv("OBSIDIAN_EXPORT_DIR")
	if envExportDir == "" {
		envExportDir = os.Getenv("OBSIDIAN_VAULT_DIR")
	}

	response := ObsidianSyncSettingsResponse{
		Config:       config,
		Status:       status,
		NextRun:      nextRun,
		IsRunning:    isRunning,
		EnvExportDir: envExportDir,
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
		ctx.HTML(http.StatusOK, "obsidian-sync-settings", response)
	}
}

// UpdateSettingsRequest is the request body for POST /settings/obsidian
type UpdateSettingsRequest struct {
	Enabled   *bool  `form:"enabled" json:"enabled"`
	ExportDir string `form:"export_dir" json:"export_dir"`
	Schedule  string `form:"schedule" json:"schedule"`
}

// UpdateSettings saves Obsidian sync settings
func (c *ObsidianSyncController) UpdateSettings(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
			"Success": false,
			"Error":   "Settings store not available",
		})
		return
	}

	var req UpdateSettingsRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.HTML(http.StatusBadRequest, "obsidian-sync-result", gin.H{
			"Success": false,
			"Error":   "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate and save export directory if provided
	if req.ExportDir != "" {
		validatedPath, err := validateExportDirectory(req.ExportDir)
		if err != nil {
			ctx.HTML(http.StatusBadRequest, "obsidian-sync-result", gin.H{
				"Success": false,
				"Error":   "Invalid export directory: " + err.Error(),
			})
			return
		}
		if err := c.settingsStore.SetObsidianSyncExportDir(validatedPath); err != nil {
			ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
				"Success": false,
				"Error":   "Failed to save export directory: " + err.Error(),
			})
			return
		}
	}

	// Validate and save schedule if provided
	if req.Schedule != "" {
		if err := settingsstore.ValidateCronSchedule(req.Schedule); err != nil {
			ctx.HTML(http.StatusBadRequest, "obsidian-sync-result", gin.H{
				"Success": false,
				"Error":   "Invalid cron schedule: " + err.Error(),
			})
			return
		}
		if err := c.settingsStore.SetObsidianSyncSchedule(req.Schedule); err != nil {
			ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
				"Success": false,
				"Error":   "Failed to save schedule: " + err.Error(),
			})
			return
		}
	}

	// Save enabled state if provided
	if req.Enabled != nil {
		if err := c.settingsStore.SetObsidianSyncEnabled(*req.Enabled); err != nil {
			ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
				"Success": false,
				"Error":   "Failed to save enabled state: " + err.Error(),
			})
			return
		}
	}

	// Reschedule the sync job if scheduler is available
	if c.scheduler != nil {
		if err := c.scheduler.Reschedule(); err != nil {
			ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
				"Success": false,
				"Error":   "Settings saved but failed to reschedule: " + err.Error(),
			})
			return
		}
	}

	// Return updated settings
	config := c.settingsStore.GetObsidianSyncConfigInfo()
	ctx.HTML(http.StatusOK, "obsidian-sync-result", gin.H{
		"Success": true,
		"Config":  config,
	})
}

// ResetSettings clears database overrides, reverting to env/defaults
func (c *ObsidianSyncController) ResetSettings(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
			"Success": false,
			"Error":   "Settings store not available",
		})
		return
	}

	if err := c.settingsStore.ClearObsidianSyncSettings(); err != nil {
		ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
			"Success": false,
			"Error":   "Failed to reset settings: " + err.Error(),
		})
		return
	}

	// Reschedule with new settings
	if c.scheduler != nil {
		_ = c.scheduler.Reschedule()
	}

	config := c.settingsStore.GetObsidianSyncConfigInfo()
	ctx.HTML(http.StatusOK, "obsidian-sync-result", gin.H{
		"Success": true,
		"Config":  config,
	})
}

// ValidateDirectoryRequest is the request body for validating a directory
type ValidateDirectoryRequest struct {
	Path string `form:"path" json:"path"`
}

// ValidateDirectory validates an export directory path
func (c *ObsidianSyncController) ValidateDirectory(ctx *gin.Context) {
	var req ValidateDirectoryRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": "Invalid request",
		})
		return
	}

	validatedPath, err := validateExportDirectory(req.Path)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"valid": true,
		"path":  validatedPath,
	})
}

// SyncNow triggers an immediate sync
func (c *ObsidianSyncController) SyncNow(ctx *gin.Context) {
	if c.scheduler == nil {
		ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
			"Success": false,
			"Error":   "Scheduler not available",
		})
		return
	}

	// Check if export directory is configured
	config := c.settingsStore.GetObsidianSyncConfig()
	if config.ExportDir == "" {
		ctx.HTML(http.StatusBadRequest, "obsidian-sync-result", gin.H{
			"Success": false,
			"Error":   "Export directory not configured. Please configure it first.",
		})
		return
	}

	if err := c.scheduler.RunNow(); err != nil {
		ctx.HTML(http.StatusInternalServerError, "obsidian-sync-result", gin.H{
			"Success": false,
			"Error":   "Failed to start sync: " + err.Error(),
		})
		return
	}

	ctx.HTML(http.StatusOK, "obsidian-sync-result", gin.H{
		"Success": true,
		"Message": "Sync started in background",
	})
}

// GetStatus returns just the sync status (for polling)
func (c *ObsidianSyncController) GetStatus(ctx *gin.Context) {
	if c.settingsStore == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Settings store not available"})
		return
	}

	status := c.settingsStore.GetObsidianSyncStatus()
	var nextRun *time.Time
	isRunning := false
	if c.scheduler != nil {
		nextRun = c.scheduler.GetNextRunTime()
		isRunning = c.scheduler.IsRunning()
	}

	response := gin.H{
		"status":     status,
		"next_run":   nextRun,
		"is_running": isRunning,
	}

	if strings.Contains(ctx.GetHeader("Accept"), "application/json") {
		ctx.JSON(http.StatusOK, response)
	} else {
		ctx.HTML(http.StatusOK, "obsidian-sync-status", response)
	}
}

// validateExportDirectory validates and normalizes an export directory path
func validateExportDirectory(rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)

	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Reject paths with null bytes
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("path contains invalid characters")
	}

	// Convert to absolute and clean
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path format: %w", err)
	}
	cleanPath := filepath.Clean(absPath)

	// Check if exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist")
		}
		if os.IsPermission(err) {
			return "", fmt.Errorf("permission denied")
		}
		return "", fmt.Errorf("cannot access path: %w", err)
	}

	// Must be a directory
	if !info.IsDir() {
		return "", fmt.Errorf("path must be a directory, not a file")
	}

	// Test write permission
	testFile := filepath.Join(cleanPath, ".obsidian_sync_test_"+fmt.Sprintf("%d", time.Now().UnixNano()))
	f, err := os.Create(testFile)
	if err != nil {
		if os.IsPermission(err) {
			return "", fmt.Errorf("no write permission")
		}
		return "", fmt.Errorf("cannot write to directory: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return cleanPath, nil
}
