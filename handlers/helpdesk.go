package handlers

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

type helpdeskAttachmentInput struct {
	Filename string `json:"filename"`
	Data     string `json:"data"`
}

type HelpDeskRequest struct {
	Subject     string                    `json:"subject"`
	Description string                    `json:"description"`
	Attachments []helpdeskAttachmentInput `json:"attachments"`
}

type updateHelpDeskTicketRequest struct {
	Subject     *string `json:"subject"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

func SubmitTicketHandler(c *gin.Context) {
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, ok := userIDInterface.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var req HelpDeskRequest
	if !bindHelpdeskJSON(c, &req) {
		return
	}
	files, err := decodeHelpdeskAttachments(req.Attachments)
	if err != nil {
		writeHelpdeskAttachmentError(c, err)
		return
	}

	ticket, err := models.CreateTicket(userID, req.Subject, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save the ticket"})
		return
	}
	if len(files) > 0 {
		if err := models.AddHelpDeskAttachments(userID, ticket.ID, files); err != nil {
			_ = models.DeleteHelpDeskTicket(ticket.ID)
			writeHelpdeskAttachmentError(c, err)
			return
		}
		reloaded, err := models.GetHelpDeskTicketByID(ticket.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ticket was saved, but the attachments could not be loaded."})
			return
		}
		ticket = reloaded
	}

	c.JSON(http.StatusOK, gin.H{"ticket": ticket})
}

func AddHelpdeskAttachments(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	ticketID, ok := parseTicketIDParam(c)
	if !ok {
		return
	}
	ticket, err := models.GetHelpDeskTicketByID(ticketID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Helpdesk ticket not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load helpdesk ticket"})
		return
	}
	isAdmin, err := models.IsAdmin(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user role"})
		return
	}
	if ticket.UserID != userID && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "You cannot add files to this ticket"})
		return
	}

	var req struct {
		Attachments []helpdeskAttachmentInput `json:"attachments"`
	}
	if !bindHelpdeskJSON(c, &req) {
		return
	}
	files, err := decodeHelpdeskAttachments(req.Attachments)
	if err != nil {
		writeHelpdeskAttachmentError(c, err)
		return
	}
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose a file to attach."})
		return
	}
	if err := models.AddHelpDeskAttachments(userID, ticketID, files); err != nil {
		writeHelpdeskAttachmentError(c, err)
		return
	}
	updated, err := models.GetHelpDeskTicketByID(ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Files were saved, but the ticket could not be loaded."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ticket": updated})
}

func bindHelpdeskJSON(c *gin.Context, dest interface{}) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 40<<20)
	if err := c.ShouldBindJSON(dest); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) || strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Attachments must be 8 MB or smaller, with up to 5 files."})
			return false
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return false
	}
	return true
}

func decodeHelpdeskAttachments(items []helpdeskAttachmentInput) ([]models.HelpdeskUpload, error) {
	if len(items) > models.HelpdeskMaxAttachments {
		return nil, &models.HelpdeskAttachmentError{Msg: "A ticket can have up to 5 attachments."}
	}
	uploads := make([]models.HelpdeskUpload, 0, len(items))
	for _, item := range items {
		payload := strings.TrimSpace(item.Data)
		if payload == "" {
			return nil, &models.HelpdeskAttachmentError{Msg: "An attachment was empty."}
		}
		if idx := strings.Index(payload, ","); idx >= 0 && strings.Contains(strings.ToLower(payload[:idx]), "base64") {
			payload = payload[idx+1:]
		}
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			data, err = base64.RawStdEncoding.DecodeString(payload)
		}
		if err != nil || len(data) == 0 {
			return nil, &models.HelpdeskAttachmentError{Msg: "An attachment could not be read."}
		}
		uploads = append(uploads, models.HelpdeskUpload{Name: item.Filename, Data: data})
	}
	return uploads, nil
}

func writeHelpdeskAttachmentError(c *gin.Context, err error) {
	var verr *models.HelpdeskAttachmentError
	if errors.As(err, &verr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": verr.Msg})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save the attachment."})
}

func ListHelpdeskTicketsByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	tickets, err := models.ListHelpDeskTickets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load helpdesk tickets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tickets": tickets})
}

func GetHelpdeskTicketByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	ticketID, ok := parseTicketIDParam(c)
	if !ok {
		return
	}

	ticket, err := models.GetHelpDeskTicketByID(ticketID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Helpdesk ticket not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load helpdesk ticket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ticket": ticket})
}

func UpdateHelpdeskTicketByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	ticketID, ok := parseTicketIDParam(c)
	if !ok {
		return
	}

	existingTicket, err := models.GetHelpDeskTicketByID(ticketID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Helpdesk ticket not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load helpdesk ticket"})
		return
	}

	var req updateHelpDeskTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	subject := existingTicket.Subject
	description := existingTicket.Description
	status := existingTicket.Status
	if req.Subject != nil {
		subject = *req.Subject
	}
	if req.Description != nil {
		description = *req.Description
	}
	if req.Status != nil {
		status = strings.TrimSpace(strings.ToLower(*req.Status))
	}

	ticket, err := models.UpdateHelpDeskTicket(ticketID, subject, description, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Helpdesk ticket not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Helpdesk ticket updated", "ticket": ticket})
}

func DeleteHelpdeskTicketByAdmin(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if !requireAdmin(c, userID) {
		return
	}

	ticketID, ok := parseTicketIDParam(c)
	if !ok {
		return
	}

	if err := models.DeleteHelpDeskTicket(ticketID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Helpdesk ticket not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete helpdesk ticket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Helpdesk ticket deleted", "ticket_id": ticketID})
}

func parseTicketIDParam(c *gin.Context) (int, bool) {
	ticketID, err := strconv.Atoi(c.Param("ticket_id"))
	if err != nil || ticketID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid helpdesk ticket ID"})
		return 0, false
	}
	return ticketID, true
}
