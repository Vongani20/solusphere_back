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
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// RFPMaxCombinedRunes caps the combined tender text kept for analysis.
const RFPMaxCombinedRunes = 1500000

var (
	rfpDocxParagraph = regexp.MustCompile(`(?s)<w:p[ >].*?</w:p>|<w:p/>`)
	rfpDocxBreak     = regexp.MustCompile(`<w:(?:br|tab|cr)\b[^>]*/>`)
	rfpTrailingSpace = regexp.MustCompile(`[ \t]+\n`)
	rfpManyNewlines  = regexp.MustCompile(`\n{3,}`)
)

// ExtractRFPSourceText reads a tender upload (.pdf, .docx, .txt, .md) and keeps line structure
// so section headings survive for the built-in reader.
func ExtractRFPSourceText(filename string, data []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".txt", ".md":
		if !utf8.Valid(data) {
			return "", fmt.Errorf("%s is not UTF-8 text", filename)
		}
		return CleanRFPText(string(data)), nil
	case ".docx":
		return extractRFPDocx(data)
	case ".pdf":
		return extractRFPPDF(data)
	default:
		return "", fmt.Errorf("upload a PDF, Word (.docx), or text file")
	}
}

// CleanRFPText normalises line endings and blank lines.
func CleanRFPText(text string) string {
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "\u0000", "")
	text = rfpTrailingSpace.ReplaceAllString(text, "\n")
	text = rfpManyNewlines.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func extractRFPDocx(data []byte) (string, error) {
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
			return "", err
		}
		raw, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}
		xml = string(raw)
		break
	}
	if xml == "" {
		return "", fmt.Errorf("docx is missing word/document.xml")
	}

	var b strings.Builder
	for _, para := range rfpDocxParagraph.FindAllString(xml, -1) {
		para = rfpDocxBreak.ReplaceAllString(para, "<w:t> </w:t>")
		for _, m := range docxTextRunPattern.FindAllStringSubmatch(para, -1) {
			b.WriteString(unescapeXMLText(m[1]))
		}
		b.WriteByte('\n')
	}
	return CleanRFPText(b.String()), nil
}

func extractRFPPDF(data []byte) (string, error) {
	tmp, err := os.CreateTemp("", "rfp-*.pdf")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer f.Close()

	// Read page by page, row by row, so line breaks are kept.
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			continue
		}
		for _, row := range rows {
			for j, word := range row.Content {
				if j > 0 {
					prev := row.Content[j-1]
					gap := word.X - (prev.X + prev.W)
					threshold := prev.FontSize * 0.2
					if threshold <= 0 {
						threshold = 1
					}
					if gap > threshold && !strings.HasSuffix(prev.S, " ") && !strings.HasPrefix(word.S, " ") {
						b.WriteByte(' ')
					}
				}
				b.WriteString(word.S)
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	text := CleanRFPText(b.String())
	if text == "" {
		// Fall back to the plain-text reader used elsewhere.
		plain, err := r.GetPlainText()
		if err == nil {
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, plain)
			text = CleanRFPText(buf.String())
		}
	}
	return text, nil
}
