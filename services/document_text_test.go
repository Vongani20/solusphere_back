package services

import (
	"strings"
	"testing"
)

func TestNormalizeDocumentTextTrimsAndLimits(t *testing.T) {
	input := "  Hello   world  \n\nSecond line  "
	got := normalizeDocumentText(input)
	want := "Hello   world  \n\nSecond line"
	if got != want {
		t.Fatalf("normalizeDocumentText() = %q, want %q", got, want)
	}

	long := strings.Repeat("a", maxDocumentTextRunes+100)
	got = normalizeDocumentText(long)
	if len([]rune(got)) != maxDocumentTextRunes {
		t.Fatalf("expected %d runes, got %d", maxDocumentTextRunes, len([]rune(got)))
	}
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
