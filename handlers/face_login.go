package handlers

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"solusphere_backend/database"
	"solusphere_backend/middleware"
	"solusphere_backend/models"
	"solusphere_backend/services"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gin-gonic/gin"
)

func FaceLogin(rekogSvc *services.RekognitionService) gin.HandlerFunc {
	const mainCollectionID = "solusphere_user_faces_index"

	return func(c *gin.Context) {
		log.Println("========================================")
		log.Println("🔐 FACE LOGIN HANDLER CALLED")
		log.Println("========================================")

		if !ensureRekognitionConfigured(c, rekogSvc) {
			return
		}

		sourceBytes, header, fieldName, err := readFaceImageUpload(c, defaultFaceFieldNames)
		if err != nil {
			log.Printf("❌ Failed to read face upload: %v", err)
			if !writeFaceUploadError(c, err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read uploaded file"})
			}
			return
		}
		log.Printf("📸 Received face file from field %q: %s, size: %d bytes", fieldName, header.Filename, len(sourceBytes))

		// Search Rekognition collection
		log.Printf("🔍 Searching for face in Rekognition collection: %s", mainCollectionID)

		resp, err := rekogSvc.SearchCollectionByImage(mainCollectionID, sourceBytes)
		if err != nil {
			log.Printf("❌ Failed to search face index: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to search face index",
				"details": err.Error(),
			})
			return
		}

		if len(resp.FaceMatches) == 0 {
			log.Printf("⚠️ No face matches found")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Face not recognized"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		query := `
			SELECT u.id, u.username, u.email, u.role, uf.status, COALESCE(uf.image_url, ''), COALESCE(uf.face_id, '')
			FROM users u
			INNER JOIN user_faces uf ON uf.user_id = u.id
			WHERE u.id = ?
		`

		for _, match := range resp.FaceMatches {
			if match.Face == nil {
				log.Printf("⚠️ Rekognition returned a match without face metadata")
				continue
			}

			similarity := aws.ToFloat32(match.Similarity)
			rekognitionUserIDStr := aws.ToString(match.Face.ExternalImageId)
			rekognitionFaceID := aws.ToString(match.Face.FaceId)
			if rekognitionUserIDStr == "" || rekognitionFaceID == "" {
				log.Printf("⚠️ Rekognition match missing user or face id")
				continue
			}

			userID, err := strconv.Atoi(rekognitionUserIDStr)
			if err != nil {
				log.Printf("⚠️ Rekognition match has malformed user ID %q", rekognitionUserIDStr)
				continue
			}

			var (
				user         models.UserMinimal
				faceStatus   bool
				imageURL     string
				storedFaceID string
			)
			row := database.DB.QueryRowContext(ctx, query, userID)
			if err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Role, &faceStatus, &imageURL, &storedFaceID); err != nil {
				if err == sql.ErrNoRows {
					log.Printf("⚠️ Face match belongs to missing or unregistered user ID %d", userID)
					continue
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}

			if !faceStatus || storedFaceID == "" {
				log.Printf("⚠️ Face match belongs to user %d, but DB face is inactive or missing", userID)
				continue
			}

			if storedFaceID != rekognitionFaceID {
				log.Printf("⚠️ Rekognition face mismatch for user %d: matched=%s stored=%s", userID, rekognitionFaceID, storedFaceID)
				continue
			}

			user.ImageURL = imageURL

			token, err := middleware.GenerateToken(user.ID, user.Username)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
				return
			}

			log.Printf("✅ Face login successful - Similarity: %.2f%%, UserID: %d", similarity, user.ID)
			c.JSON(http.StatusOK, models.FaceLoginResponse{
				Message:    "Face login successful",
				Token:      token,
				Similarity: similarity,
				User:       user,
			})
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Face not recognized"})
	}
}
