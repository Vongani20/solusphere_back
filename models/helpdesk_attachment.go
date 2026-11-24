package models

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"solusphere_backend/database"
)

const (
	HelpdeskMaxAttachments     = 5
	HelpdeskMaxAttachmentBytes = 8 << 20
	HelpdeskMaxAttachmentTotal = 24 << 20
)

type HelpdeskAttachmentError struct{ Msg string }

func (e *HelpdeskAttachmentError) Error() string { return e.Msg }

type HelpDeskAttachment struct {
	ID          int       `json:"id"`
	TicketID    int       `json:"ticket_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int       `json:"size_bytes"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
}

type HelpdeskUpload struct {
	Name string
	Data []byte
}

var helpdeskAttachmentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
	".pdf":  "application/pdf",
	".txt":  "text/plain; charset=utf-8",
	".csv":  "text/csv",
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".zip":  "application/zip",
}

func AddHelpDeskAttachments(userID, ticketID int, files []HelpdeskUpload) error {
	if len(files) == 0 {
		return nil
	}
	var existing int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM help_desk_attachments WHERE ticket_id = ?`, ticketID).Scan(&existing); err != nil {
		return err
	}
	if existing+len(files) > HelpdeskMaxAttachments {
		return &HelpdeskAttachmentError{Msg: "A ticket can have up to 5 attachments."}
	}

	total := 0
	prepared := make([]struct {
		name        string
		contentType string
		data        []byte
	}, 0, len(files))
	for _, file := range files {
		name, contentType, err := cleanHelpdeskUpload(file)
		if err != nil {
			return err
		}
		total += len(file.Data)
		if total > HelpdeskMaxAttachmentTotal {
			return &HelpdeskAttachmentError{Msg: "Keep the combined attachments under 24 MB."}
		}
		prepared = append(prepared, struct {
			name        string
			contentType string
			data        []byte
		}{name: name, contentType: contentType, data: file.Data})
	}

	for i, file := range prepared {
		ext := strings.ToLower(filepath.Ext(file.name))
		key := fmt.Sprintf("helpdesk-attachments/%d/%d/file_%d_%d%s", userID, ticketID, time.Now().UnixNano(), i, ext)
		if err := UploadToS3WithContentType(key, file.data, file.contentType); err != nil {
			return fmt.Errorf("upload helpdesk attachment: %w", err)
		}
		if _, err := database.DB.Exec(`
			INSERT INTO help_desk_attachments (ticket_id, file_name, content_type, size_bytes, storage_url)
			VALUES (?, ?, ?, ?, ?)`,
			ticketID, file.name, file.contentType, len(file.data), S3ObjectURL(key)); err != nil {
			return err
		}
	}
	return nil
}

func cleanHelpdeskUpload(file HelpdeskUpload) (string, string, error) {
	name := sanitizeHelpdeskFileName(file.Name)
	if name == "" || name == "." {
		return "", "", &HelpdeskAttachmentError{Msg: "Each attachment needs a file name."}
	}
	ext := strings.ToLower(filepath.Ext(name))
	contentType, ok := helpdeskAttachmentTypes[ext]
	if !ok {
		return "", "", &HelpdeskAttachmentError{Msg: name + " must be an image, PDF, Office file, text, CSV, or ZIP."}
	}
	if len(file.Data) == 0 {
		return "", "", &HelpdeskAttachmentError{Msg: name + " is empty."}
	}
	if len(file.Data) > HelpdeskMaxAttachmentBytes {
		return "", "", &HelpdeskAttachmentError{Msg: name + " is larger than 8 MB."}
	}
	if err := sniffHelpdeskFile(name, ext, file.Data); err != nil {
		return "", "", err
	}
	return name, contentType, nil
}

func sanitizeHelpdeskFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" {
		return ""
	}
	cleaned := strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`"<>:|?*`, r) {
			return '_'
		}
		return r
	}, name)
	if utf8.RuneCountInString(cleaned) > 180 {
		ext := filepath.Ext(cleaned)
		base := strings.TrimSuffix(cleaned, ext)
		keep := 180 - utf8.RuneCountInString(ext)
		if keep < 1 {
			keep = 1
		}
		runes := []rune(base)
		if len(runes) > keep {
			base = string(runes[:keep])
		}
		cleaned = base + ext
	}
	return cleaned
}

func sniffHelpdeskFile(name, ext string, data []byte) error {
	detected := http.DetectContentType(data)
	switch ext {
	case ".jpg", ".jpeg":
		if detected != "image/jpeg" {
			return &HelpdeskAttachmentError{Msg: name + " is not a JPEG image."}
		}
	case ".png":
		if detected != "image/png" {
			return &HelpdeskAttachmentError{Msg: name + " is not a PNG image."}
		}
	case ".gif":
		if detected != "image/gif" {
			return &HelpdeskAttachmentError{Msg: name + " is not a GIF image."}
		}
	case ".webp":
		if detected != "image/webp" && !isWebP(data) {
			return &HelpdeskAttachmentError{Msg: name + " is not a WEBP image."}
		}
	case ".pdf":
		if len(data) < 5 || string(data[:5]) != "%PDF-" {
			return &HelpdeskAttachmentError{Msg: name + " is not a PDF."}
		}
	case ".zip", ".docx", ".xlsx", ".pptx":
		if len(data) < 4 || data[0] != 'P' || data[1] != 'K' {
			return &HelpdeskAttachmentError{Msg: name + " is not a valid Office or ZIP file."}
		}
	}
	return nil
}

func isWebP(data []byte) bool {
	return len(data) > 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func attachFiles(tickets []HelpDeskTicket) error {
	if len(tickets) == 0 {
		return nil
	}
	ids := make([]int, len(tickets))
	index := map[int]int{}
	for i := range tickets {
		tickets[i].Attachments = []HelpDeskAttachment{}
		ids[i] = tickets[i].ID
		index[tickets[i].ID] = i
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := database.DB.Query(`
		SELECT id, ticket_id, file_name, content_type, size_bytes, storage_url, created_at
		FROM help_desk_attachments
		WHERE ticket_id IN (`+placeholders+`)
		ORDER BY id`, args...)
	if err != nil {
		if database.IsMissingTableError(err) {
			return nil
		}
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item HelpDeskAttachment
		var stored string
		if err := rows.Scan(&item.ID, &item.TicketID, &item.FileName, &item.ContentType, &item.SizeBytes, &stored, &item.CreatedAt); err != nil {
			return err
		}
		item.URL = ClientAccessiblePhotoURL(stored)
		if i, ok := index[item.TicketID]; ok {
			tickets[i].Attachments = append(tickets[i].Attachments, item)
		}
	}
	return rows.Err()
}
