package services

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

const maxDocumentTextRunes = 50000

// ReadPDFDocumentText reads text content from a PDF file path.
func ReadPDFDocumentText(filePath string) (string, int, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	reader, err := r.GetPlainText()
	if err != nil {
		return "", r.NumPage(), fmt.Errorf("read pdf text: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return "", r.NumPage(), fmt.Errorf("copy pdf text: %w", err)
	}

	text := normalizeDocumentText(buf.String())
	if text == "" {
		return "", r.NumPage(), fmt.Errorf("no readable text found in PDF")
	}

	return text, r.NumPage(), nil
}

// ExtractTextFromPDFBytes writes bytes to a temp file and extracts text.
func ExtractTextFromPDFBytes(data []byte) (string, int, error) {
	tmp, err := os.CreateTemp("", "document-*.pdf")
	if err != nil {
		return "", 0, err
	}
	path := tmp.Name()
	defer os.Remove(path)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", 0, err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}

	return ReadPDFDocumentText(path)
}

func normalizeDocumentText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == '\u0000'
	}), "\n")
	text = strings.TrimSpace(text)
	if len([]rune(text)) > maxDocumentTextRunes {
		runes := []rune(text)
		text = string(runes[:maxDocumentTextRunes])
	}
	return text
}
