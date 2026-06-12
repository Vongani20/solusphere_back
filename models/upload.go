package models

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

// UploadFile handles general file uploads (similar to UploadFace but for any file type)
func UploadFile(db *sql.DB, file *multipart.FileHeader, userID int) (string, error) {
	// Create uploads directory if it doesn't exist
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %v", err)
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%d_%s_%s%s", userID, time.Now().Format("20060102"), generateRandomString(8), ext)
	filePath := filepath.Join(uploadDir, filename)

	// Open source file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %v", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %v", err)
	}
	defer dst.Close()

	// Copy file content
	if _, err = io.Copy(dst, src); err != nil {
		// Clean up if copy fails
		os.Remove(filePath)
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	// You can optionally save file metadata to database here
	// For example, create a file_uploads table:
	/*
		query := `INSERT INTO file_uploads (user_id, filename, file_path, file_size, mime_type, created_at)
		          VALUES (?, ?, ?, ?, ?, ?)`
		_, err = db.Exec(query, userID, file.Filename, filePath, file.Size, file.Header.Get("Content-Type"), time.Now())
		if err != nil {
			// Log the error but don't fail the upload
			fmt.Printf("Failed to save file metadata to database: %v\n", err)
		}
	*/

	return "/uploads/" + filename, nil
}

// generateRandomString generates a random string for filenames
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)

	// Simple random generation using current time
	for i := range bytes {
		bytes[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(bytes)
}

// Alternative using crypto/rand (more secure)
func generateRandomStringSecure(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, length)

	// Use crypto/rand for better security
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to simple method if crypto/rand fails
		return generateRandomString(length)
	}

	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}
	return string(bytes)
}
