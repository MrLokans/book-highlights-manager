package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mikestefanello/backlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTasksController_ListTaskTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	controller := NewTasksController(nil)
	router := gin.New()
	router.GET("/api/tasks/types", controller.ListTaskTypes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/tasks/types", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string][]TaskTypeInfo
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, len(resp["task_types"]), 2)
}

func TestTaskStatusToString(t *testing.T) {
	tests := []struct {
		status   backlite.TaskStatus
		expected string
	}{
		{backlite.TaskStatusPending, "pending"},
		{backlite.TaskStatusRunning, "running"},
		{backlite.TaskStatusSuccess, "success"},
		{backlite.TaskStatusFailure, "failure"},
		{backlite.TaskStatusNotFound, "not_found"},
		{backlite.TaskStatus(99), "unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, taskStatusToString(tt.status))
	}
}
