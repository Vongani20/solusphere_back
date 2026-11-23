package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"solusphere_backend/database"
	"solusphere_backend/models"
)

func taskViewer(c *gin.Context) (int, bool, bool) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return 0, false, false
	}
	isAdmin, err := models.IsAdmin(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user role"})
		return 0, false, false
	}
	return userID, isAdmin, true
}

func taskError(c *gin.Context, err error) {
	var verr *models.TaskValidationError
	switch {
	case errors.As(err, &verr):
		c.JSON(http.StatusBadRequest, gin.H{"error": verr.Msg})
	case errors.Is(err, models.ErrTaskNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
	case errors.Is(err, models.ErrTaskForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		log.Printf("tasks: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Task request failed"})
	}
}

func parseTaskID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("task_id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return 0, false
	}
	return id, true
}

// ListTasks GET /api/tasks?scope=mine|created|all
func ListTasks(c *gin.Context) {
	userID, isAdmin, ok := taskViewer(c)
	if !ok {
		return
	}
	tasks, err := models.ListTasks(database.DB, userID, isAdmin, c.DefaultQuery("scope", "mine"))
	if err != nil {
		taskError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "is_admin": isAdmin})
}

// TaskSummary GET /api/tasks/summary — open, overdue and due-today counts for the sidebar.
func TaskSummary(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	summary, err := models.SummarizeTasks(database.DB, userID)
	if err != nil {
		if database.IsMissingTableError(err) {
			c.JSON(http.StatusOK, models.TaskSummary{})
			return
		}
		taskError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// CreateTask POST /api/tasks
func CreateTask(c *gin.Context) {
	userID, isAdmin, ok := taskViewer(c)
	if !ok {
		return
	}
	var in models.TaskInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	task, err := models.CreateTask(database.DB, userID, isAdmin, in)
	if err != nil {
		taskError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"task": task})
}

// UpdateTask PATCH /api/tasks/:task_id
func UpdateTask(c *gin.Context) {
	userID, isAdmin, ok := taskViewer(c)
	if !ok {
		return
	}
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	var in models.TaskInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	task, err := models.UpdateTask(database.DB, id, userID, isAdmin, in)
	if err != nil {
		taskError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"task": task})
}

// DeleteTask DELETE /api/tasks/:task_id
func DeleteTask(c *gin.Context) {
	userID, isAdmin, ok := taskViewer(c)
	if !ok {
		return
	}
	id, ok := parseTaskID(c)
	if !ok {
		return
	}
	if err := models.DeleteTask(database.DB, id, userID, isAdmin); err != nil {
		taskError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
