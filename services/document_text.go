package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

var (
	docxTagPattern   = regexp.MustCompile(`<[^>]+>`)
	docxSpacePattern = regexp.MustCompile(`[ \t\f\v]+`)
)

var docxTextRunPattern = regexp.MustCompile(`(?s)<w:t(?:\s[^>]*)?>(.*?)</w:t>`)

// ExtractTextFromCVUpload extracts readable text from an uploaded CV PDF or Word document.
func ExtractTextFromCVUpload(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		text, _, err := ExtractTextFromPDFBytes(data)
		return text, err
	case ".docx":
		return ExtractTextFromDocxBytes(data)
	case ".doc":
		return "", fmt.Errorf("legacy .doc files are not supported — please upload a .docx or PDF")
	default:
		return "", fmt.Errorf("unsupported file type %q — upload a PDF or Word (.docx) file", ext)
	}
}

// ExtractTextFromDocxBytes extracts plain text from a .docx (OOXML) document.
func ExtractTextFromDocxBytes(data []byte) (string, error) {
	if len(data) < 4 || string(data[:2]) != "PK" {
		return "", fmt.Errorf("file is not a valid Word (.docx) document")
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}

	var xml string
	for _, f := range reader.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open docx document.xml: %w", err)
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("read docx document.xml: %w", err)
		}
		xml = string(raw)
		break
	}
	if xml == "" {
		return "", fmt.Errorf("docx is missing word/document.xml")
	}

	matches := docxTextRunPattern.FindAllStringSubmatch(xml, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("no readable text found in Word document")
	}

	var b strings.Builder
	for i, match := range matches {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(unescapeXMLText(match[1]))
	}

	text := normalizeDocumentText(b.String())
	if text == "" {
		return "", fmt.Errorf("no readable text found in Word document")
	}
	return text, nil
}

func unescapeXMLText(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
	)
	return replacer.Replace(s)
}

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

// ExtractTextFromDOCXBytes is kept as an alias for callers that pass raw bytes + extension.
func ExtractTextFromDOCXBytes(data []byte) (string, error) {
	return ExtractTextFromDocxBytes(data)
}

// ExtractTextFromCVDocumentBytes extracts text from PDF or DOCX bytes.
func ExtractTextFromCVDocumentBytes(data []byte, ext string) (string, error) {
	switch strings.ToLower(ext) {
	case ".pdf":
		text, _, err := ExtractTextFromPDFBytes(data)
		return text, err
	case ".docx":
		return ExtractTextFromDocxBytes(data)
	default:
		return "", fmt.Errorf("unsupported document type %q", ext)
	}
}

func normalizeDocumentText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == '\u0000'
	}), "\n")
	return strings.TrimSpace(text)
}
