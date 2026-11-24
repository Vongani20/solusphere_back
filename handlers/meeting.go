package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"solusphere_backend/database"
	"solusphere_backend/models"
)

type meetingRequest struct {
	Title    string `json:"title"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
	Notes    string `json:"notes"`
	UserIDs  []int  `json:"user_ids"`
}

type meetingParticipantRequest struct {
	UserID int `json:"user_id"`
}

type meetingNotesRequest struct {
	Notes string `json:"notes"`
}

func meetingError(c *gin.Context, err error) {
	var verr *models.MeetingValidationError
	switch {
	case errors.As(err, &verr):
		c.JSON(http.StatusBadRequest, gin.H{"error": verr.Msg})
	case errors.Is(err, models.ErrMeetingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Meeting not found"})
	case errors.Is(err, models.ErrMeetingForbidden), errors.Is(err, models.ErrMeetingHostOnly):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, models.ErrMeetingNoPeople),
		errors.Is(err, models.ErrMeetingFull),
		errors.Is(err, models.ErrMeetingUnknownUser),
		errors.Is(err, models.ErrMeetingTimeOrder):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("meetings: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Meeting request failed"})
	}
}

func parseMeetingID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("meeting_id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid meeting"})
		return 0, false
	}
	return id, true
}

func parseMeetingTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, &models.MeetingValidationError{Msg: "Use a valid date and time."}
}

func ListMeetings(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	meetings, err := models.ListMeetings(database.DB, userID)
	if err != nil {
		if database.IsMissingTableError(err) {
			c.JSON(http.StatusOK, gin.H{"meetings": []models.Meeting{}})
			return
		}
		meetingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meetings": meetings})
}

func CreateMeeting(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req meetingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid meeting"})
		return
	}
	starts, err := parseMeetingTime(req.StartsAt)
	if err != nil {
		meetingError(c, err)
		return
	}
	var ends *time.Time
	if strings.TrimSpace(req.EndsAt) != "" {
		parsed, err := parseMeetingTime(req.EndsAt)
		if err != nil {
			meetingError(c, err)
			return
		}
		ends = &parsed
	}
	meeting, err := models.CreateMeeting(database.DB, userID, req.Title, starts, ends, req.Notes, req.UserIDs)
	if err != nil {
		meetingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"meeting": meeting})
}

func GetMeeting(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, ok := parseMeetingID(c)
	if !ok {
		return
	}
	allowed, err := models.UserCanAccessMeeting(database.DB, id, userID)
	if err != nil {
		meetingError(c, err)
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not part of this meeting"})
		return
	}
	meeting, err := models.GetMeeting(database.DB, id)
	if err != nil {
		meetingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meeting": meeting})
}

func AddMeetingParticipant(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, ok := parseMeetingID(c)
	if !ok {
		return
	}
	var req meetingParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose a person to add"})
		return
	}
	meeting, err := models.AddMeetingParticipant(database.DB, id, userID, req.UserID)
	if err != nil {
		meetingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meeting": meeting})
}

func UpdateMeetingNotes(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id, ok := parseMeetingID(c)
	if !ok {
		return
	}
	var req meetingNotesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notes"})
		return
	}
	meeting, err := models.UpdateMeetingNotes(database.DB, id, userID, req.Notes)
	if err != nil {
		meetingError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"meeting": meeting})
}
