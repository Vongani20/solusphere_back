package handlers

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"strconv"
	"time"

	"solusphere_backend/database"
	"solusphere_backend/models"
	"solusphere_backend/services"
	"solusphere_backend/utils"

	"github.com/gin-gonic/gin"
)

func FaceLogin(rekogSvc *services.RekognitionService) gin.HandlerFunc {
	const mainCollectionID = "solusphere_user_faces_index"

	return func(c *gin.Context) {
		// --- 1. Read uploaded face image ---
		file, _, err := c.Request.FormFile("face")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No face file uploaded"})
			return
		}
		defer file.Close()

		sourceBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read uploaded file"})
			return
		}

		// --- 2. Search Rekognition collection ---
		resp, err := rekogSvc.SearchCollectionByImage(mainCollectionID, sourceBytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search face index"})
			return
		}

		if len(resp.FaceMatches) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Face not recognized in the system"})
			return
		}

		match := resp.FaceMatches[0]
		similarity := *match.Similarity
		rekognitionUserIDStr := *match.Face.ExternalImageId

		userID, err := strconv.Atoi(rekognitionUserIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Malformed user ID retrieved from face index",
				"details": rekognitionUserIDStr,
			})
			return
		}

		// --- 3. Database lookup with timeout ---
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var user models.UserMinimal
		query := `SELECT id, username, email FROM users WHERE id = ?`
		row := database.DB.QueryRowContext(ctx, query, userID)

		if err := row.Scan(&user.ID, &user.Username, &user.Email); err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{
					"error":      "User found in face index but not in database",
					"id_checked": userID,
				})
				return
			}
			// handle other DB errors (e.g., timeout, connection issues)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error during user retrieval"})
			return
		}

		// --- 4. Generate JWT ---
		token, err := utils.GenerateTokenWithEmail(user.Email, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, models.FaceLoginResponse{
			Message:    "Face login successful",
			Token:      token,
			Similarity: similarity,
			User:       user,
		})
	}
}
