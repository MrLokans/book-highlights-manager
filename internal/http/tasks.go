package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikestefanello/backlite"
	"github.com/mrlokans/assistant/internal/tasks"
)

const taskStatusTimeout = 5 * time.Second

// TasksController handles task queue management endpoints.
type TasksController struct {
	client *tasks.Client
}

// NewTasksController creates a new TasksController.
func NewTasksController(client *tasks.Client) *TasksController {
	return &TasksController{client: client}
}

// TaskInfo represents basic information about a task.
type TaskInfo struct {
	ID        string `json:"id"`
	Queue     string `json:"queue"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
}

// TaskTypeInfo describes an available task type.
type TaskTypeInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Queue       string `json:"queue"`
}

// ListTaskTypes handles GET /api/tasks/types
// Returns the list of available task types that can be triggered.
func (tc *TasksController) ListTaskTypes(c *gin.Context) {
	types := []TaskTypeInfo{
		{
			Type:        "enrich_book",
			Description: "Enrich a single book's metadata from OpenLibrary",
			Queue:       "enrich_book",
		},
		{
			Type:        "enrich_all_books",
			Description: "Enrich all books missing metadata",
			Queue:       "enrich_all_books",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"task_types": types,
	})
}

// GetTaskStatus handles GET /api/tasks/:id
// Returns the status of a specific task.
func (tc *TasksController) GetTaskStatus(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task ID is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), taskStatusTimeout)
	defer cancel()

	status, err := tc.client.Status(ctx, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	statusStr := taskStatusToString(status)

	c.JSON(http.StatusOK, gin.H{
		"id":     taskID,
		"status": statusStr,
	})
}

// RunTaskRequest is the request body for running a task.
type RunTaskRequest struct {
	// BookID is required for enrich_book task
	BookID uint `json:"book_id,omitempty" form:"book_id"`
}

// RunTask handles POST /api/tasks/:type/run
// Manually triggers a task of the specified type.
// Supports both JSON API and HTMX (form) requests.
func (tc *TasksController) RunTask(c *gin.Context) {
	taskType := c.Param("type")

	var req RunTaskRequest
	// Try to bind from form data first (for HTMX), then JSON
	if c.ContentType() == "application/x-www-form-urlencoded" || c.ContentType() == "multipart/form-data" {
		_ = c.ShouldBind(&req)
	} else if c.Request.ContentLength > 0 {
		_ = c.ShouldBindJSON(&req)
	}

	var task backlite.Task
	switch taskType {
	case "enrich_book":
		if req.BookID == 0 {
			tc.respondTaskError(c, "book_id is required for enrich_book task")
			return
		}
		task = tasks.EnrichBookTask{BookID: req.BookID}

	case "enrich_all_books":
		task = tasks.EnrichAllBooksTask{}

	default:
		tc.respondTaskError(c, fmt.Sprintf("unknown task type: %s", taskType))
		return
	}

	ids, err := tc.client.Add(task).Save()
	if err != nil {
		tc.respondTaskError(c, err.Error())
		return
	}

	tc.respondTaskSuccess(c, ids[0], taskType)
}

func (tc *TasksController) respondTaskSuccess(c *gin.Context, taskID, taskType string) {
	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "task-run-result", gin.H{"Success": true, "TaskID": taskID})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"task_id": taskID,
		"type":    taskType,
		"message": "task enqueued",
	})
}

func (tc *TasksController) respondTaskError(c *gin.Context, errorMsg string) {
	if isHTMXRequest(c) {
		c.HTML(http.StatusOK, "task-run-result", gin.H{"Success": false, "Error": errorMsg})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": errorMsg})
}

func taskStatusToString(status backlite.TaskStatus) string {
	switch status {
	case backlite.TaskStatusPending:
		return "pending"
	case backlite.TaskStatusRunning:
		return "running"
	case backlite.TaskStatusSuccess:
		return "success"
	case backlite.TaskStatusFailure:
		return "failure"
	case backlite.TaskStatusNotFound:
		return "not_found"
	default:
		return "unknown"
	}
}
