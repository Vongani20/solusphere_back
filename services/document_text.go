package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"

	"solusphere_backend/internal/ai"
)

var (
	docxTagPattern   = regexp.MustCompile(`<[^>]+>`)
	docxSpacePattern = regexp.MustCompile(`[ \t\f\v]+`)
	rtfControlPattern = regexp.MustCompile(`\\[a-zA-Z]+\d* ?|\\[^a-zA-Z]|[{}]`)
)

var docxTextRunPattern = regexp.MustCompile(`(?s)<w:t(?:\s[^>]*)?>(.*?)</w:t>`)
var odtTextRunPattern = regexp.MustCompile(`(?s)<text:p[^>]*>(.*?)</text:p>`)

const (
	cvOCRMinUsefulChars = 40
	cvOCRSystemPrompt   = `You are a document OCR specialist for CVs/resumes.
Transcribe ALL readable text from the attached document or image(s).
Rules:
- Preserve reading order as much as possible (name, contact, sections, experience bullets).
- Keep employer names, dates, degrees, and bullets exact — do not invent content.
- Output plain text only (no markdown fences, no commentary).
- If the attachment is unreadable, reply with exactly: UNREADABLE`
)

// ExtractTextFromCVUpload extracts readable text from uploaded CV documents.
// Supports PDF (text + OCR fallback), Word (.docx/.doc), OpenDocument (.odt),
// RTF, plain text, and common image formats (OCR).
func ExtractTextFromCVUpload(filename string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty document")
	}

	kind := detectCVDocumentKind(filename, data)
	text, err := extractNativeCVText(kind, data)
	if err == nil {
		text = strings.TrimSpace(text)
		if text != "" && !strings.EqualFold(text, "UNREADABLE") {
			return text, nil
		}
		err = fmt.Errorf("no readable text found in document")
	}

	nativeErr := err
	ocrText, ocrErr := extractCVTextWithOCR(context.Background(), filename, kind, data)
	if ocrErr == nil && isUsefulCVText(ocrText) {
		return ocrText, nil
	}

	if ocrErr != nil {
		return "", fmt.Errorf("%v; OCR fallback also failed: %w", nativeErr, ocrErr)
	}
	return "", nativeErr
}

func isUsefulCVText(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.EqualFold(trimmed, "UNREADABLE") {
		return false
	}
	letters := 0
	for _, r := range trimmed {
		if unicode.IsLetter(r) {
			letters++
			if letters >= cvOCRMinUsefulChars {
				return true
			}
		}
	}
	return false
}

func detectCVDocumentKind(filename string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch {
	case len(data) >= 4 && string(data[:4]) == "%PDF":
		return "pdf"
	case len(data) >= 2 && string(data[:2]) == "PK":
		// Could be docx, odt, or other zip-based formats.
		if ext == ".odt" || looksLikeODT(data) {
			return "odt"
		}
		return "docx"
	case len(data) >= 8 && string(data[:8]) == "\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1":
		return "doc"
	case len(data) >= 5 && (bytes.HasPrefix(data, []byte("{\\rtf")) || bytes.HasPrefix(data, []byte("{\\RTF"))):
		return "rtf"
	case isJPEG(data):
		return "jpeg"
	case isPNG(data):
		return "png"
	case isGIF(data):
		return "gif"
	case isWEBP(data):
		return "webp"
	case ext == ".pdf":
		return "pdf"
	case ext == ".docx":
		return "docx"
	case ext == ".doc":
		return "doc"
	case ext == ".odt":
		return "odt"
	case ext == ".rtf":
		return "rtf"
	case ext == ".txt" || ext == ".md" || ext == ".csv":
		return "text"
	case ext == ".jpg" || ext == ".jpeg":
		return "jpeg"
	case ext == ".png":
		return "png"
	case ext == ".gif":
		return "gif"
	case ext == ".webp":
		return "webp"
	case utf8.Valid(data) && looksLikePlainText(data):
		return "text"
	default:
		if ext != "" {
			return strings.TrimPrefix(ext, ".")
		}
		return "unknown"
	}
}

func looksLikeODT(data []byte) bool {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, f := range reader.File {
		if f.Name == "content.xml" || f.Name == "mimetype" {
			return true
		}
	}
	return false
}

func looksLikePlainText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 2048 {
		sample = sample[:2048]
	}
	nonPrintable := 0
	for _, b := range sample {
		if b == 0 {
			return false
		}
		if b < 9 || (b > 13 && b < 32) {
			nonPrintable++
		}
	}
	return nonPrintable*10 < len(sample)
}

func isJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
}
func isPNG(data []byte) bool {
	return len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n"
}
func isGIF(data []byte) bool {
	return len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a")
}
func isWEBP(data []byte) bool {
	return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func extractNativeCVText(kind string, data []byte) (string, error) {
	switch kind {
	case "pdf":
		text, _, err := ExtractTextFromPDFBytes(data)
		return text, err
	case "docx":
		return ExtractTextFromDocxBytes(data)
	case "odt":
		return ExtractTextFromODTBytes(data)
	case "doc":
		return ExtractTextFromLegacyDocBytes(data)
	case "rtf":
		return ExtractTextFromRTFBytes(data)
	case "text":
		return normalizeDocumentText(string(data)), nil
	case "jpeg", "png", "gif", "webp":
		return "", fmt.Errorf("image documents require OCR")
	default:
		// Best-effort: treat unknown as text if it looks readable.
		if looksLikePlainText(data) {
			return normalizeDocumentText(string(data)), nil
		}
		return "", fmt.Errorf("unsupported or unreadable document type %q", kind)
	}
}

func extractCVTextWithOCR(ctx context.Context, filename, kind string, data []byte) (string, error) {
	if !IsOpenAIInitialized() {
		return "", fmt.Errorf("OCR is unavailable because OpenAI is not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req := ai.GenerateTextRequest{
		SystemPrompt:    cvOCRSystemPrompt,
		UserPrompt:      "Extract all CV/resume text from this attachment.",
		MaxOutputTokens: 8000,
		Temperature:     0,
	}

	switch kind {
	case "jpeg", "png", "gif", "webp":
		mime := "image/" + kind
		if kind == "jpeg" {
			mime = "image/jpeg"
		}
		req.Images = []ai.ImageInput{{
			ImageURL: fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)),
			Detail:   "high",
		}}
	case "pdf":
		name := filepath.Base(filename)
		if name == "" || name == "." {
			name = "cv.pdf"
		}
		req.Files = []ai.FileInput{{
			Filename: name,
			FileData: "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(data),
		}}
		// Also attach any embedded page images as a secondary signal.
		if imgs := extractEmbeddedImagesFromPDF(data, 4); len(imgs) > 0 {
			req.Images = imgs
		}
	default:
		// For Word/unknown binaries, ask the model to OCR via file upload when possible.
		name := filepath.Base(filename)
		if name == "" || name == "." {
			name = "document.bin"
		}
		mime := mimeForCVKind(kind)
		req.Files = []ai.FileInput{{
			Filename: name,
			FileData: fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)),
		}}
	}

	text, err := ai.GenerateText(ctx, req)
	if err != nil {
		return "", err
	}
	text = normalizeDocumentText(text)
	if !isUsefulCVText(text) {
		return "", fmt.Errorf("OCR produced no usable text")
	}
	return text, nil
}

func mimeForCVKind(kind string) string {
	switch kind {
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "doc":
		return "application/msword"
	case "odt":
		return "application/vnd.oasis.opendocument.text"
	case "rtf":
		return "application/rtf"
	case "pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// extractEmbeddedImagesFromPDF finds JPEG/PNG streams embedded in a PDF.
func extractEmbeddedImagesFromPDF(data []byte, maxImages int) []ai.ImageInput {
	if maxImages <= 0 {
		maxImages = 3
	}
	out := make([]ai.ImageInput, 0, maxImages)

	// JPEG markers
	for i := 0; i+3 < len(data) && len(out) < maxImages; i++ {
		if data[i] == 0xff && data[i+1] == 0xd8 && data[i+2] == 0xff {
			end := bytes.Index(data[i+2:], []byte{0xff, 0xd9})
			if end < 0 {
				continue
			}
			end = i + 2 + end + 2
			chunk := data[i:end]
			if len(chunk) < 8<<10 {
				continue // skip tiny icons
			}
			out = append(out, ai.ImageInput{
				ImageURL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(chunk),
				Detail:   "high",
			})
			i = end - 1
		}
	}

	// PNG markers
	pngSig := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	for i := 0; i+8 < len(data) && len(out) < maxImages; i++ {
		if !bytes.Equal(data[i:i+8], pngSig) {
			continue
		}
		// Find IEND chunk.
		iend := bytes.Index(data[i+8:], []byte("IEND"))
		if iend < 0 {
			continue
		}
		end := i + 8 + iend + 8 // IEND + CRC
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		if len(chunk) < 8<<10 {
			continue
		}
		out = append(out, ai.ImageInput{
			ImageURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(chunk),
			Detail:   "high",
		})
		i = end - 1
	}

	return out
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

// ExtractTextFromODTBytes extracts plain text from an OpenDocument (.odt) file.
func ExtractTextFromODTBytes(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open odt: %w", err)
	}
	var xml string
	for _, f := range reader.File {
		if f.Name != "content.xml" {
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
		return "", fmt.Errorf("odt is missing content.xml")
	}

	matches := odtTextRunPattern.FindAllStringSubmatch(xml, -1)
	var b strings.Builder
	for _, match := range matches {
		plain := docxTagPattern.ReplaceAllString(match[1], " ")
		plain = unescapeXMLText(plain)
		plain = strings.TrimSpace(plain)
		if plain == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(plain)
	}
	text := normalizeDocumentText(b.String())
	if text == "" {
		return "", fmt.Errorf("no readable text found in ODT document")
	}
	return text, nil
}

// ExtractTextFromRTFBytes strips RTF control words and returns plain text.
func ExtractTextFromRTFBytes(data []byte) (string, error) {
	raw := string(data)
	plain := rtfControlPattern.ReplaceAllString(raw, " ")
	plain = strings.ReplaceAll(plain, "\\'a0", " ")
	text := normalizeDocumentText(plain)
	if !isUsefulCVText(text) {
		return "", fmt.Errorf("no readable text found in RTF document")
	}
	return text, nil
}

// ExtractTextFromLegacyDocBytes best-effort extracts printable text from .doc binaries.
func ExtractTextFromLegacyDocBytes(data []byte) (string, error) {
	var b strings.Builder
	run := 0
	flush := func() {
		if run >= 4 {
			b.WriteByte(' ')
		}
		run = 0
	}
	for _, c := range data {
		if c >= 32 && c < 127 || c == '\n' || c == '\r' || c == '\t' {
			b.WriteByte(c)
			run++
		} else {
			flush()
		}
	}
	text := normalizeDocumentText(b.String())
	if !isUsefulCVText(text) {
		return "", fmt.Errorf("legacy .doc had little readable text — OCR will be attempted")
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
	filename := "document" + strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") && ext != "" {
		filename = "document." + strings.ToLower(ext)
	}
	return ExtractTextFromCVUpload(filename, data)
}

func normalizeDocumentText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == '\u0000'
	}), "\n")
	return strings.TrimSpace(text)
}
