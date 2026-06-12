package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

type HelpDeskRequest struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ticket, err := models.CreateTicket(userID, req.Subject, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ticket": ticket})
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
