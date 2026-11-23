package handlers

import (
	"net/http"

	"solusphere_backend/database"
	"solusphere_backend/models"

	"github.com/gin-gonic/gin"
)

type birthDateRequest struct {
	BirthDate string `json:"birth_date"`
}

func ListBirthdays(c *gin.Context) {
	people, err := models.ListBirthdays(database.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load birthdays"})
		return
	}
	today := make([]models.BirthdayPerson, 0)
	for _, person := range people {
		if person.IsToday {
			today = append(today, person)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"today":     today,
		"birthdays": people,
	})
}

func UpdateBirthDate(c *gin.Context) {
	userID := c.GetInt("userID")
	var req birthDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Birth date is required"})
		return
	}
	if _, err := models.ParseBirthDate(req.BirthDate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := models.SetUserBirthDate(database.DB, userID, req.BirthDate)
	if err != nil || user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save birth date"})
		return
	}

	face, faceErr := models.GetUserFaceByUserID(database.DB, userID)
	faceStatus := faceErr == nil && face != nil && face.IsRegistered()
	var imageURL interface{}
	if faceStatus && face.ImageURL != "" {
		imageURL = face.ImageURL
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Birth date saved",
		"user":    userResponse(user, faceStatus, imageURL),
	})
}
