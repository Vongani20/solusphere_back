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

type groupCallCreateRequest struct {
	UserIDs  []int  `json:"user_ids"`
	CallType string `json:"call_type"`
}

type groupCallInviteRequest struct {
	UserIDs []int `json:"user_ids"`
}

type groupCallSignalRequest struct {
	ToUserID int    `json:"to_user_id"`
	Kind     string `json:"kind"`
	Payload  string `json:"payload"`
}

func groupCallError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, models.ErrGroupCallNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Group call not found"})
	case errors.Is(err, models.ErrGroupCallEnded):
		c.JSON(http.StatusGone, gin.H{"error": err.Error()})
	case errors.Is(err, models.ErrGroupCallNotJoined):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, models.ErrGroupCallFull),
		errors.Is(err, models.ErrGroupCallNoInvitees),
		errors.Is(err, models.ErrGroupCallBadSignal),
		errors.Is(err, models.ErrGroupCallUnknownUsers):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("group call: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Group call request failed"})
	}
}

// StartGroupCall POST /api/group-calls {"user_ids":[2,3],"call_type":"audio"}
func StartGroupCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req groupCallCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	call, err := models.CreateGroupCall(database.DB, userID, req.UserIDs, req.CallType)
	if err != nil {
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"call": call, "signal_cursor": 0})
}

// ListIncomingGroupCalls GET /api/group-calls/incoming
func ListIncomingGroupCalls(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	setNoCacheHeaders(c)
	calls, err := models.ListIncomingGroupCalls(database.DB, userID)
	if err != nil {
		if database.IsMissingTableError(err) {
			c.JSON(http.StatusOK, gin.H{"calls": []models.GroupCall{}})
			return
		}
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"calls": calls})
}

// SyncGroupCall GET /api/group-calls/:call_id/sync?since_id=N — heartbeat, participants and new signals.
func SyncGroupCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	setNoCacheHeaders(c)
	sinceID, _ := strconv.ParseInt(c.Query("since_id"), 10, 64)
	call, signals, err := models.SyncGroupCall(database.DB, c.Param("call_id"), userID, sinceID)
	if err != nil {
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"call": call, "signals": signals})
}

// JoinGroupCall POST /api/group-calls/:call_id/join
func JoinGroupCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	call, cursor, err := models.JoinGroupCall(database.DB, c.Param("call_id"), userID)
	if err != nil {
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"call": call, "signal_cursor": cursor})
}

// DeclineGroupCall POST /api/group-calls/:call_id/decline
func DeclineGroupCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if err := models.DeclineGroupCall(database.DB, c.Param("call_id"), userID); err != nil {
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// LeaveGroupCall POST /api/group-calls/:call_id/leave
func LeaveGroupCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if err := models.LeaveGroupCall(database.DB, c.Param("call_id"), userID); err != nil {
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// InviteToGroupCall POST /api/group-calls/:call_id/invite {"user_ids":[4]}
func InviteToGroupCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req groupCallInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	call, err := models.InviteToGroupCall(database.DB, c.Param("call_id"), userID, req.UserIDs)
	if err != nil {
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"call": call})
}

// SendGroupCallSignal POST /api/group-calls/:call_id/signals {"to_user_id":2,"kind":"offer","payload":"..."}
func SendGroupCallSignal(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, models.GroupCallMaxSignalBytes+4096)
	var req groupCallSignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if err := models.AddGroupCallSignal(database.DB, c.Param("call_id"), userID, req.ToUserID, req.Kind, req.Payload); err != nil {
		groupCallError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true})
}
