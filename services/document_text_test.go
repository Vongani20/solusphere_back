package services

import (
	"bytes"
	"strings"
	"testing"
)

func TestNormalizeDocumentTextTrims(t *testing.T) {
	input := "  Hello   world  \n\nSecond line  "
	got := normalizeDocumentText(input)
	want := "Hello   world  \n\nSecond line"
	if got != want {
		t.Fatalf("normalizeDocumentText() = %q, want %q", got, want)
	}

	long := strings.Repeat("a", 60000)
	got = normalizeDocumentText(long)
	if len([]rune(got)) != 60000 {
		t.Fatalf("expected full document text without truncation, got %d runes", len([]rune(got)))
	}
}

func TestExtractPageLikeImagesSkipsSmallLogos(t *testing.T) {
	// Build a tiny fake PDF-ish blob with one small and one larger JPEG.
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	small := fakeJPEG(2 << 10)
	large := fakeJPEG(100 << 10)
	buf.Write(small)
	buf.Write(large)

	imgs := extractPageLikeImagesFromPDF(buf.Bytes(), 4)
	if len(imgs) != 1 {
		t.Fatalf("expected 1 page-like image, got %d", len(imgs))
	}
}

func fakeJPEG(size int) []byte {
	if size < 4 {
		size = 4
	}
	b := make([]byte, size)
	b[0], b[1], b[2] = 0xff, 0xd8, 0xff
	b[size-2], b[size-1] = 0xff, 0xd9
	return b
}

func TestParseJSONObjectStripsCodeFence(t *testing.T) {
	raw := "```json\n{\"summary\":\"ok\",\"confidence_score\":0.9}\n```"
	got, err := parseJSONObject(raw)
	if err != nil {
		t.Fatalf("parseJSONObject() error = %v", err)
	}
	if got["summary"] != "ok" {
		t.Fatalf("summary = %v", got["summary"])
	}
	if confidenceFromResult(got, 0) != 0.9 {
		t.Fatalf("confidence = %v", got["confidence_score"])
	}
}

func TestParseJSONObjectRepairsTruncation(t *testing.T) {
	raw := `{"first_name":"Rozelna","last_name":"Bosch","experience":[{"company":"CBRE"`
	got, err := parseJSONObject(raw)
	if err != nil {
		t.Fatalf("parseJSONObject() error = %v", err)
	}
	if got["first_name"] != "Rozelna" {
		t.Fatalf("first_name = %v", got["first_name"])
	}
	exp, ok := got["experience"].([]interface{})
	if !ok || len(exp) != 1 {
		t.Fatalf("experience = %v", got["experience"])
	}
}

func TestBuildBPOAnalysisResultUsesPayload(t *testing.T) {
	payload := map[string]interface{}{
		"document_type":    "invoice",
		"summary":          "Vendor invoice for services.",
		"key_entities":     []interface{}{"Acme Corp"},
		"confidence_score": 0.92,
		"type_specific": map[string]interface{}{
			"invoice_number": "INV-001",
		},
	}

	result := buildBPOAnalysisResult(payload, "general")
	if result["analysis_type"] != "invoice" {
		t.Fatalf("analysis_type = %v", result["analysis_type"])
	}
	extracted := result["extracted_data"].(map[string]interface{})
	if extracted["summary"] != "Vendor invoice for services." {
		t.Fatalf("summary = %v", extracted["summary"])
	}
	if getConfidence := confidenceFromResult(extracted, 0); getConfidence != 0.92 {
		t.Fatalf("confidence = %v", getConfidence)
	}
	if result["invoice_data"] == nil {
		t.Fatal("expected invoice_data")
	}
}
