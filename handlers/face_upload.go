package handlers

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
)

const maxFaceUploadBytes int64 = 5 << 20

var defaultFaceFieldNames = []string{"face", "image", "file", "photo"}

type faceUploadError struct {
	status int
	body   gin.H
}

func (e *faceUploadError) Error() string {
	if msg, ok := e.body["error"].(string); ok {
		return msg
	}
	return "face upload error"
}

func readFaceImageUpload(c *gin.Context, fieldNames []string) ([]byte, *multipart.FileHeader, string, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFaceUploadBytes)

	if err := c.Request.ParseMultipartForm(maxFaceUploadBytes); err != nil {
		status := http.StatusBadRequest
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || errors.Is(err, multipart.ErrMessageTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}

		return nil, nil, "", &faceUploadError{
			status: status,
			body: gin.H{
				"error":   "Failed to parse form data",
				"message": "Upload a JPEG or PNG face image no larger than 5 MB",
			},
		}
	}

	var fileHeader *multipart.FileHeader
	var foundFieldName string

	for _, fieldName := range fieldNames {
		file, header, err := c.Request.FormFile(fieldName)
		if err == nil {
			fileHeader = header
			foundFieldName = fieldName
			file.Close()
			break
		}
	}

	if fileHeader == nil {
		return nil, nil, "", &faceUploadError{
			status: http.StatusBadRequest,
			body: gin.H{
				"error":            "No face file uploaded",
				"message":          "Please upload a JPEG or PNG file with field name: face, image, file, or photo",
				"available_fields": getAvailableFields(c),
			},
		}
	}

	if fileHeader.Size > maxFaceUploadBytes {
		return nil, nil, "", &faceUploadError{
			status: http.StatusRequestEntityTooLarge,
			body: gin.H{
				"error":   "Face image is too large",
				"message": "Upload a JPEG or PNG face image no larger than 5 MB",
			},
		}
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, "", &faceUploadError{
			status: http.StatusBadRequest,
			body:   gin.H{"error": "Failed to open uploaded file"},
		}
	}
	defer file.Close()

	imageBytes, err := io.ReadAll(io.LimitReader(file, maxFaceUploadBytes+1))
	if err != nil {
		return nil, nil, "", &faceUploadError{
			status: http.StatusBadRequest,
			body:   gin.H{"error": "Failed to read uploaded file"},
		}
	}

	if int64(len(imageBytes)) > maxFaceUploadBytes {
		return nil, nil, "", &faceUploadError{
			status: http.StatusRequestEntityTooLarge,
			body: gin.H{
				"error":   "Face image is too large",
				"message": "Upload a JPEG or PNG face image no larger than 5 MB",
			},
		}
	}

	if !isSupportedFaceImage(imageBytes) {
		return nil, nil, "", &faceUploadError{
			status: http.StatusUnsupportedMediaType,
			body: gin.H{
				"error":   "Unsupported face image type",
				"message": "Face recognition supports JPEG and PNG images only",
			},
		}
	}

	return imageBytes, fileHeader, foundFieldName, nil
}

func writeFaceUploadError(c *gin.Context, err error) bool {
	var uploadErr *faceUploadError
	if errors.As(err, &uploadErr) {
		c.JSON(uploadErr.status, uploadErr.body)
		return true
	}
	return false
}

func isSupportedFaceImage(imageBytes []byte) bool {
	if len(imageBytes) == 0 {
		return false
	}

	switch http.DetectContentType(imageBytes) {
	case "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}
