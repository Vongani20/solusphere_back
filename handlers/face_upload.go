package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

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

type identityImageRequest struct {
	Image string `json:"image"`
}

func readFaceImageUpload(c *gin.Context, fieldNames []string) ([]byte, *multipart.FileHeader, string, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFaceUploadBytes+1024)

	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if strings.HasPrefix(contentType, "application/json") {
		return readFaceImageJSON(c)
	}

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
				"message": "Upload a JPEG or PNG image no larger than 5 MB",
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
				"error":            "No image uploaded",
				"message":          "Send JSON {\"image\":\"<base64>\"} or multipart field: image, file, or photo",
				"available_fields": getAvailableFields(c),
			},
		}
	}

	if fileHeader.Size > maxFaceUploadBytes {
		return nil, nil, "", &faceUploadError{
			status: http.StatusRequestEntityTooLarge,
			body: gin.H{
				"error":   "Image is too large",
				"message": "Upload a JPEG or PNG image no larger than 5 MB",
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
				"error":   "Image is too large",
				"message": "Upload a JPEG or PNG image no larger than 5 MB",
			},
		}
	}

	if !isSupportedFaceImage(imageBytes) {
		return nil, nil, "", &faceUploadError{
			status: http.StatusUnsupportedMediaType,
			body: gin.H{
				"error":   "Unsupported image type",
				"message": "JPEG and PNG images only",
			},
		}
	}

	return imageBytes, fileHeader, foundFieldName, nil
}

func readFaceImageJSON(c *gin.Context) ([]byte, *multipart.FileHeader, string, error) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxFaceUploadBytes+8192))
	if err != nil {
		return nil, nil, "", &faceUploadError{
			status: http.StatusBadRequest,
			body:   gin.H{"error": "Failed to read request body"},
		}
	}

	var req identityImageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, "", &faceUploadError{
			status: http.StatusBadRequest,
			body: gin.H{
				"error":   "Invalid request body",
				"message": "Expected JSON {\"image\":\"<base64>\"}",
			},
		}
	}

	imageBytes, err := decodeIdentityImage(req.Image)
	if err != nil {
		return nil, nil, "", err
	}

	header := &multipart.FileHeader{
		Filename: "capture.jpg",
		Size:     int64(len(imageBytes)),
	}
	return imageBytes, header, "image", nil
}

func decodeIdentityImage(raw string) ([]byte, error) {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		return nil, &faceUploadError{
			status: http.StatusBadRequest,
			body: gin.H{
				"error":   "Image is required",
				"message": "Provide a base64-encoded JPEG or PNG in the image field",
			},
		}
	}

	if idx := strings.Index(payload, ","); idx >= 0 && strings.Contains(strings.ToLower(payload[:idx]), "base64") {
		payload = payload[idx+1:]
	}

	imageBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		imageBytes, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		return nil, &faceUploadError{
			status: http.StatusBadRequest,
			body: gin.H{
				"error":   "Invalid image encoding",
				"message": "Image must be valid base64",
			},
		}
	}

	if int64(len(imageBytes)) > maxFaceUploadBytes {
		return nil, &faceUploadError{
			status: http.StatusRequestEntityTooLarge,
			body: gin.H{
				"error":   "Image is too large",
				"message": "Upload a JPEG or PNG image no larger than 5 MB",
			},
		}
	}

	if !isSupportedFaceImage(imageBytes) {
		return nil, &faceUploadError{
			status: http.StatusUnsupportedMediaType,
			body: gin.H{
				"error":   "Unsupported image type",
				"message": "JPEG and PNG images only",
			},
		}
	}

	return imageBytes, nil
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
