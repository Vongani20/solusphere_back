package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

type startCallRequest struct {
	CallType string `json:"call_type" binding:"required"`
	Offer    struct {
		Type string `json:"type" binding:"required"`
		SDP  string `json:"sdp" binding:"required"`
	} `json:"offer" binding:"required"`
}

type acceptCallRequest struct {
	Answer struct {
		Type string `json:"type" binding:"required"`
		SDP  string `json:"sdp" binding:"required"`
	} `json:"answer" binding:"required"`
}

type callCandidateRequest struct {
	Candidate struct {
		Candidate     string `json:"candidate" binding:"required"`
		SDPMid          string `json:"sdpMid"`
		SDPMLineIndex   *int   `json:"sdpMLineIndex"`
	} `json:"candidate" binding:"required"`
}

func StartCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	calleeID, ok := parseChatUserID(c)
	if !ok {
		return
	}
	if calleeID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot call yourself"})
		return
	}
	if !ensureUserExists(c, calleeID) {
		return
	}

	var req startCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	callType := strings.ToLower(strings.TrimSpace(req.CallType))
	if callType != models.CallTypeAudio && callType != models.CallTypeVideo {
		c.JSON(http.StatusBadRequest, gin.H{"error": "call_type must be audio or video"})
		return
	}

	offerSDP := strings.TrimSpace(req.Offer.SDP)
	if offerSDP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Offer SDP is required"})
		return
	}

	call, err := models.CreateCallSession(database.DB, userID, calleeID, callType, offerSDP)
	if err != nil {
		log.Printf("StartCall failed caller=%d callee=%d: %v", userID, calleeID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start call", "details": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"call": sanitizeCallForUser(call, userID)})
}

func ListIncomingCalls(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	calls, err := models.ListIncomingCalls(database.DB, userID)
	if err != nil {
		log.Printf("ListIncomingCalls failed user=%d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load incoming calls", "details": err.Error()})
		return
	}

	setNoCacheHeaders(c)

	sanitized := make([]models.CallSession, 0, len(calls))
	for i := range calls {
		sanitized = append(sanitized, *sanitizeCallForUser(&calls[i], userID))
	}
	c.JSON(http.StatusOK, gin.H{"calls": sanitized})
}

func GetCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	call, ok := loadAccessibleCall(c, c.Param("call_id"))
	if !ok {
		return
	}

	setNoCacheHeaders(c)

	c.JSON(http.StatusOK, gin.H{"call": sanitizeCallForUser(call, userID)})
}

func setNoCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func AcceptCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	call, ok := loadAccessibleCall(c, c.Param("call_id"))
	if !ok {
		return
	}
	if call.CalleeID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the callee can accept this call"})
		return
	}

	var req acceptCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	answerSDP := strings.TrimSpace(req.Answer.SDP)
	if answerSDP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Answer SDP is required"})
		return
	}

	updated, err := models.AcceptCallSession(database.DB, call.ID, userID, answerSDP)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"call": sanitizeCallForUser(updated, userID)})
}

func RejectCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	call, ok := loadAccessibleCall(c, c.Param("call_id"))
	if !ok {
		return
	}
	if call.CalleeID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the callee can reject this call"})
		return
	}

	updated, err := models.RejectCallSession(database.DB, call.ID, userID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"call": sanitizeCallForUser(updated, userID)})
}

func EndCall(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	call, ok := loadAccessibleCall(c, c.Param("call_id"))
	if !ok {
		return
	}

	updated, err := models.EndCallSession(database.DB, call.ID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to end call"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"call": sanitizeCallForUser(updated, userID)})
}

func AddCallCandidate(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	call, ok := loadAccessibleCall(c, c.Param("call_id"))
	if !ok {
		return
	}
	if call.Status != models.CallStatusRinging && call.Status != models.CallStatusAccepted {
		c.JSON(http.StatusConflict, gin.H{"error": "Call is no longer active"})
		return
	}

	var req callCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	item, err := models.AddCallIceCandidate(
		database.DB,
		call.ID,
		userID,
		req.Candidate.Candidate,
		req.Candidate.SDPMid,
		req.Candidate.SDPMLineIndex,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"candidate": item})
}

func ListCallCandidates(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	call, ok := loadAccessibleCall(c, c.Param("call_id"))
	if !ok {
		return
	}

	setNoCacheHeaders(c)

	sinceID := int64(0)
	if raw := c.Query("since_id"); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
			sinceID = parsed
		}
	}

	items, err := models.ListCallIceCandidates(database.DB, call.ID, sinceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load ICE candidates"})
		return
	}

	filtered := make([]models.CallIceCandidate, 0, len(items))
	for _, item := range items {
		if item.SenderID != userID {
			filtered = append(filtered, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{"candidates": filtered})
}

func loadAccessibleCall(c *gin.Context, callID string) (*models.CallSession, bool) {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid call ID"})
		return nil, false
	}

	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return nil, false
	}

	call, err := models.GetCallSessionByID(database.DB, callID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Call not found"})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load call"})
		return nil, false
	}
	if !models.UserCanAccessCall(call, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return nil, false
	}
	return call, true
}

func sanitizeCallForUser(call *models.CallSession, userID int) *models.CallSession {
	if call == nil {
		return nil
	}
	if !models.UserCanAccessCall(call, userID) {
		copy := *call
		copy.OfferSDP = ""
		copy.AnswerSDP = ""
		return &copy
	}
	copy := *call
	if copy.CallerID != userID {
		copy.AnswerSDP = ""
	}
	if copy.CalleeID != userID {
		copy.OfferSDP = ""
	}
	return &copy
}
