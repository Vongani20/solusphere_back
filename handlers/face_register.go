package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"solusphere_backend/database"
	"solusphere_backend/models"
	"solusphere_backend/services"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	"github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/gin-gonic/gin"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

const faceCollectionID = "solusphere_user_faces_index"

var (
	errFaceAlreadyRegistered = errors.New("face already registered")
	errNoValidFaceDetected   = errors.New("no valid face detected")

	registrationFaceFieldNames = []string{"face", "image", "file", "photo", "image_file", "face_image", "upload"}
)

func RegisterFace(rekogSvc *services.RekognitionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("========================================")
		log.Println("📸 FACE REGISTRATION HANDLER CALLED")
		log.Println("========================================")

		userID := c.GetInt("userID")
		if userID == 0 {
			log.Println("❌ User not authenticated")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		log.Printf("👤 Checking face registration for user ID: %d", userID)
		var existingCount int
		err := database.DB.QueryRow(`
			SELECT COUNT(*) FROM user_faces
			WHERE user_id = ? AND status = true AND COALESCE(face_id, '') <> ''
		`, userID).Scan(&existingCount)
		if err != nil {
			log.Printf("❌ Database count error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if existingCount > 0 {
			log.Printf("🚫 User %d already has a face registered", userID)
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Face already registered for this user",
				"message": "Use PUT /api/face/update to update your face, or DELETE /api/face/delete to remove it first",
				"user_id": userID,
			})
			return
		}

		imageBytes, header, fieldName, err := readFaceImageUpload(c, registrationFaceFieldNames)
		if err != nil {
			log.Printf("❌ Failed to read face upload: %v", err)
			if !writeFaceUploadError(c, err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read uploaded file"})
			}
			return
		}
		log.Printf("✅ Found face file from field %q: %s, size: %d bytes", fieldName, header.Filename, len(imageBytes))

		if rekogSvc == nil || rekogSvc.Client == nil {
			registerFaceLocally(c, userID, imageBytes)
			return
		}

		if !ensureDetectableFace(c, rekogSvc, imageBytes) {
			return
		}

		faceID, confidence, err := indexUserFace(c, rekogSvc, userID, imageBytes)
		if err != nil {
			if errors.Is(err, errNoValidFaceDetected) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "No valid face detected",
					"details": "Please ensure the image contains a clear, front-facing face",
				})
				return
			}

			log.Printf("❌ Failed to index face: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to index face",
				"details": err.Error(),
			})
			return
		}

		imageLocation, localFilename, err := saveFaceImageToS3(userID, imageBytes)
		if err != nil {
			log.Printf("❌ Failed to save face image: %v", err)
			deleteIndexedFace(c, rekogSvc, faceID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save face image"})
			return
		}

		if err := activateRegisteredFace(userID, imageLocation, faceID); err != nil {
			log.Printf("❌ Failed to persist face registration: %v", err)
			deleteIndexedFace(c, rekogSvc, faceID)
			removeStoredFaceImage(imageLocation)
			removeSavedFaceImage(localFilename)

			if errors.Is(err, errFaceAlreadyRegistered) {
				c.JSON(http.StatusConflict, gin.H{
					"error":   "Face was registered during processing",
					"message": "Use PUT /api/face/update to update your face",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save face data"})
			return
		}

		log.Printf("✅ Successfully registered face for user ID: %d", userID)
		removeSavedFaceImage(localFilename)

		c.JSON(http.StatusOK, gin.H{
			"message":       "Face registered successfully",
			"face_id":       faceID,
			"image_url":     imageLocation,
			"confidence":    confidence,
			"status":        true,
			"face_status":   true,
			"face_required": false,
		})
	}
}

func registerFaceLocally(c *gin.Context, userID int, imageBytes []byte) {
	log.Printf("Face recognition is not configured; completing local face setup for user %d", userID)

	faceID := "local-" + strconv.Itoa(userID) + "-" + time.Now().Format("20060102150405")
	filename, err := saveFaceImage(userID, imageBytes)
	if err != nil {
		log.Printf("Failed to save local face image: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save face image"})
		return
	}

	if err := activateRegisteredFace(userID, filename, faceID); err != nil {
		log.Printf("Failed to persist local face registration: %v", err)
		removeSavedFaceImage(filename)

		if errors.Is(err, errFaceAlreadyRegistered) {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Face already registered for this user",
				"message": "Use PUT /api/face/update to update your face",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save face data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Face registered successfully",
		"warning":       "Face recognition provider is not configured; face was saved locally for onboarding.",
		"mode":          "local",
		"face_id":       faceID,
		"image_url":     filename,
		"status":        true,
		"face_status":   true,
		"face_required": false,
	})
}

// UpdateFace updates an existing face (replaces the old one)
func UpdateFace(rekogSvc *services.RekognitionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("========================================")
		log.Println("🔄 FACE UPDATE HANDLER CALLED")
		log.Println("========================================")

		if !ensureRekognitionConfigured(c, rekogSvc) {
			return
		}

		userID := c.GetInt("userID")
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		var existingFaceID, oldImageURL string
		err := database.DB.QueryRow(`
			SELECT COALESCE(face_id, ''), COALESCE(image_url, '')
			FROM user_faces
			WHERE user_id = ? AND status = true
		`, userID).Scan(&existingFaceID, &oldImageURL)
		if err != nil || existingFaceID == "" {
			log.Printf("⚠️ No existing face found for user %d", userID)
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "No face registered for this user",
				"message": "Use POST /api/face/register to register a face first",
			})
			return
		}

		imageBytes, header, fieldName, err := readFaceImageUpload(c, defaultFaceFieldNames)
		if err != nil {
			log.Printf("❌ Failed to read face upload: %v", err)
			if !writeFaceUploadError(c, err) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read uploaded file"})
			}
			return
		}
		log.Printf("✅ Found update face file from field %q: %s, size: %d bytes", fieldName, header.Filename, len(imageBytes))

		if !ensureDetectableFace(c, rekogSvc, imageBytes) {
			return
		}

		newFaceID, confidence, err := indexUserFace(c, rekogSvc, userID, imageBytes)
		if err != nil {
			if errors.Is(err, errNoValidFaceDetected) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "No valid face detected"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to index face", "details": err.Error()})
			return
		}

		imageLocation, localFilename, err := saveFaceImageToS3(userID, imageBytes)
		if err != nil {
			deleteIndexedFace(c, rekogSvc, newFaceID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save face image"})
			return
		}

		updated, err := replaceRegisteredFace(userID, existingFaceID, imageLocation, newFaceID)
		if err != nil || !updated {
			log.Printf("❌ Failed to update face row for user %d: %v", userID, err)
			deleteIndexedFace(c, rekogSvc, newFaceID)
			removeStoredFaceImage(imageLocation)
			removeSavedFaceImage(localFilename)
			c.JSON(http.StatusConflict, gin.H{"error": "Face record changed during update. Please retry."})
			return
		}

		if existingFaceID != newFaceID {
			deleteIndexedFace(c, rekogSvc, existingFaceID)
		}
		removeSavedFaceImage(localFilename)
		removeStoredFaceImage(oldImageURL)

		log.Printf("✅ Face updated successfully for user ID: %d", userID)
		c.JSON(http.StatusOK, gin.H{
			"message":    "Face updated successfully",
			"face_id":    newFaceID,
			"image_url":  imageLocation,
			"confidence": confidence,
		})
	}
}

// DeleteFace deletes a user's face
func DeleteFace(rekogSvc *services.RekognitionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("========================================")
		log.Println("🗑️ FACE DELETE HANDLER CALLED")
		log.Println("========================================")

		userID := c.GetInt("userID")
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		var faceID, imageURL string
		err := database.DB.QueryRow(`
			SELECT COALESCE(face_id, ''), COALESCE(image_url, '')
			FROM user_faces
			WHERE user_id = ? AND status = true
		`, userID).Scan(&faceID, &imageURL)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No face found for user"})
			return
		}

		log.Printf("🗑️ Deleting face for user ID: %d, FaceID: %s", userID, faceID)
		if faceID != "" {
			deleteIndexedFace(c, rekogSvc, faceID)
		}

		_, err = database.DB.Exec(`
			DELETE FROM user_faces WHERE user_id = ?
		`, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete from database"})
			return
		}

		removeStoredFaceImage(imageURL)

		log.Printf("✅ Face deleted successfully for user ID: %d", userID)
		c.JSON(http.StatusOK, gin.H{"message": "Face deleted successfully"})
	}
}

func ensureDetectableFace(c *gin.Context, rekogSvc *services.RekognitionService, imageBytes []byte) bool {
	log.Printf("🔍 Validating that image contains a face...")
	detectResp, err := rekogSvc.Client.DetectFaces(c.Request.Context(), &rekognition.DetectFacesInput{
		Image: &types.Image{
			Bytes: imageBytes,
		},
		Attributes: []types.Attribute{types.AttributeDefault},
	})
	if err != nil {
		log.Printf("⚠️ Face detection error: %v", err)
		return true
	}

	if len(detectResp.FaceDetails) == 0 {
		log.Printf("❌ No face detected in the uploaded image")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "No face detected in the image",
			"message": "Please upload a clear image containing a face",
		})
		return false
	}

	log.Printf("✅ Face detected with confidence: %.2f%%", aws.ToFloat32(detectResp.FaceDetails[0].Confidence))
	return true
}

func indexUserFace(c *gin.Context, rekogSvc *services.RekognitionService, userID int, imageBytes []byte) (string, float32, error) {
	log.Printf("🔍 Indexing face in Rekognition collection: %s", faceCollectionID)
	indexResp, err := rekogSvc.Client.IndexFaces(c.Request.Context(), &rekognition.IndexFacesInput{
		CollectionId:    aws.String(faceCollectionID),
		ExternalImageId: aws.String(strconv.Itoa(userID)),
		Image: &types.Image{
			Bytes: imageBytes,
		},
		MaxFaces:            aws.Int32(1),
		QualityFilter:       types.QualityFilterAuto,
		DetectionAttributes: []types.Attribute{types.AttributeDefault},
	})
	if err != nil {
		return "", 0, err
	}

	if len(indexResp.FaceRecords) == 0 || indexResp.FaceRecords[0].Face == nil {
		log.Printf("⚠️ No face was indexed. Unindexed reasons: %+v", indexResp.UnindexedFaces)
		return "", 0, errNoValidFaceDetected
	}

	faceID := aws.ToString(indexResp.FaceRecords[0].Face.FaceId)
	confidence := aws.ToFloat32(indexResp.FaceRecords[0].Face.Confidence)
	if faceID == "" {
		return "", 0, errNoValidFaceDetected
	}

	log.Printf("✅ Face indexed successfully! FaceID: %s, Confidence: %.2f%%", faceID, confidence)
	return faceID, confidence, nil
}

func activateRegisteredFace(userID int, filename, faceID string) error {
	result, err := database.DB.Exec(`
		UPDATE user_faces
		SET image_url = ?, face_id = ?, status = true, updated_at = NOW()
		WHERE user_id = ? AND (status = false OR COALESCE(face_id, '') = '')
	`, filename, faceID, userID)
	if err != nil {
		return err
	}

	if rows, _ := result.RowsAffected(); rows > 0 {
		return nil
	}

	var (
		currentStatus bool
		currentFaceID string
	)
	err = database.DB.QueryRow(`
		SELECT status, COALESCE(face_id, '') FROM user_faces WHERE user_id = ?
	`, userID).Scan(&currentStatus, &currentFaceID)
	if err == nil {
		if currentStatus && currentFaceID != "" {
			return errFaceAlreadyRegistered
		}
		return errors.New("inactive face row was not updated")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = database.DB.Exec(`
		INSERT INTO user_faces (user_id, image_url, face_id, status, created_at, updated_at)
		VALUES (?, ?, ?, true, NOW(), NOW())
	`, userID, filename, faceID)
	if err != nil {
		if isDuplicateKeyError(err) {
			return errFaceAlreadyRegistered
		}
		return err
	}

	return nil
}

func replaceRegisteredFace(userID int, oldFaceID, filename, newFaceID string) (bool, error) {
	result, err := database.DB.Exec(`
		UPDATE user_faces
		SET image_url = ?, face_id = ?, status = true, updated_at = NOW()
		WHERE user_id = ? AND status = true AND face_id = ?
	`, filename, newFaceID, userID, oldFaceID)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows > 0, nil
}

func saveFaceImage(userID int, imageBytes []byte) (string, error) {
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", err
	}

	filename := filepath.Join(uploadDir, "face_"+strconv.Itoa(userID)+"_"+time.Now().Format("20060102150405")+".jpg")
	if err := os.WriteFile(filename, imageBytes, 0600); err != nil {
		return "", err
	}

	log.Printf("💾 Image saved locally: %s", filename)
	return filename, nil
}

func saveFaceImageToS3(userID int, imageBytes []byte) (string, string, error) {
	localFilename, err := saveFaceImage(userID, imageBytes)
	if err != nil {
		return "", "", err
	}

	key := fmt.Sprintf("faces/user_%d/face_%d_%s.jpg", userID, userID, time.Now().UTC().Format("20060102150405"))
	contentType := http.DetectContentType(imageBytes)
	if err := models.UploadToS3WithContentType(key, imageBytes, contentType); err != nil {
		removeSavedFaceImage(localFilename)
		return "", "", err
	}

	imageURL := models.S3ObjectURL(key)
	log.Printf("Face image uploaded to S3: %s", key)
	return imageURL, localFilename, nil
}

func deleteIndexedFace(c *gin.Context, rekogSvc *services.RekognitionService, faceID string) {
	if faceID == "" {
		return
	}
	if rekogSvc == nil || rekogSvc.Client == nil {
		log.Printf("⚠️ Rekognition is not configured; skipping delete for face %s", faceID)
		return
	}

	_, err := rekogSvc.Client.DeleteFaces(c.Request.Context(), &rekognition.DeleteFacesInput{
		CollectionId: aws.String(faceCollectionID),
		FaceIds:      []string{faceID},
	})
	if err != nil {
		log.Printf("⚠️ Failed to delete face from Rekognition: %v", err)
		return
	}

	log.Printf("🗑️ Deleted Rekognition face: %s", faceID)
}

func removeStoredFaceImage(location string) {
	if location == "" {
		return
	}

	if key, ok := models.S3KeyFromObjectURL(location); ok {
		if err := models.DeleteFromS3(key); err != nil {
			log.Printf("Failed to remove face image from S3: %v", err)
		}
		return
	}

	removeSavedFaceImage(location)
}

func removeSavedFaceImage(filename string) {
	if filename == "" {
		return
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	absUploadDir, err := filepath.Abs(uploadDir)
	if err != nil {
		return
	}
	absFilename, err := filepath.Abs(filename)
	if err != nil {
		return
	}

	if absFilename == absUploadDir || !strings.HasPrefix(absFilename, absUploadDir+string(os.PathSeparator)) {
		log.Printf("⚠️ Refusing to remove face image outside upload directory: %s", filename)
		return
	}

	if err := os.Remove(absFilename); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("⚠️ Failed to remove saved face image %s: %v", filename, err)
	}
}

func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// Helper function to get available form fields
func getAvailableFields(c *gin.Context) []string {
	var fields []string
	if c.Request.MultipartForm != nil {
		for key := range c.Request.MultipartForm.File {
			fields = append(fields, key)
		}
		for key := range c.Request.MultipartForm.Value {
			fields = append(fields, key)
		}
	}
	return fields
}
