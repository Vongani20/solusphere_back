package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"solusphere_backend/models"
)

const bpoAnalysisSystemPrompt = `You are an expert BPO (Business Process Outsourcing) document analyst.
Read the document text carefully and return ONLY valid JSON with this structure:
{
  "document_type": "invoice|contract|report|form|general",
  "summary": "2-4 sentence executive summary of the document",
  "key_entities": ["organizations, people, products, or identifiers mentioned"],
  "important_dates": [{"label": "what the date refers to", "date": "YYYY-MM-DD or original text"}],
  "financial_data": [{"label": "description", "amount": "numeric or text amount", "currency": "ISO code or symbol"}],
  "parties_involved": [{"name": "party name", "role": "vendor|customer|employer|employee|other"}],
  "risks": ["compliance, payment, legal, or operational risks found in the document"],
  "recommendations": ["actionable next steps for a BPO operations team"],
  "confidence_score": 0.0,
  "type_specific": {}
}
Rules:
- Extract only information explicitly present in the document. Do not invent amounts, dates, or parties.
- Set confidence_score between 0 and 1 based on text clarity and completeness.
- Populate type_specific with fields relevant to document_type:
  - invoice: invoice_number, due_date, line_items[], subtotal, tax, total, payment_terms
  - contract: title, effective_date, end_date, key_clauses[], payment_terms
  - report: title, period, key_findings[], metrics{}
  - form: title, required_fields[], submission_deadline
  - general: main_topics[], action_items[]
- If a field is unknown, use an empty string, empty array, or empty object.`

type PDFProcessor struct {
	openAIService *OpenAIService
}

func NewPDFProcessor(openAIService *OpenAIService) *PDFProcessor {
	return &PDFProcessor{
		openAIService: openAIService,
	}
}

// ExtractTextFromPDF extracts text content from a PDF file on disk.
func (p *PDFProcessor) ExtractTextFromPDF(filePath string) (string, int, error) {
	log.Printf("Processing PDF file: %s", filePath)
	return ReadPDFDocumentText(filePath)
}

// AnalyzeBPOContent analyzes extracted text using OpenAI structured JSON output.
func (p *PDFProcessor) AnalyzeBPOContent(ctx context.Context, text string, analysisType string) (map[string]interface{}, error) {
	if text == "" {
		return nil, fmt.Errorf("no text content to analyze")
	}
	if p.openAIService == nil && !IsOpenAIInitialized() {
		return nil, fmt.Errorf("OpenAI service is not configured")
	}

	userPrompt := fmt.Sprintf(
		"Document type hint: %s\n\nAnalyze this BPO document and return structured JSON:\n\n%s",
		analysisType,
		text,
	)

	payload, err := GenerateStructuredJSON(ctx, bpoAnalysisSystemPrompt, userPrompt, 3000)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze content with OpenAI: %w", err)
	}

	return buildBPOAnalysisResult(payload, analysisType), nil
}

func buildBPOAnalysisResult(payload map[string]interface{}, fallbackType string) map[string]interface{} {
	docType := stringFromAny(payload["document_type"])
	if docType == "" {
		docType = fallbackType
	}

	extracted := map[string]interface{}{
		"summary":          stringFromAny(payload["summary"]),
		"document_type":    docType,
		"key_entities":     payload["key_entities"],
		"important_dates":  payload["important_dates"],
		"financial_data":   payload["financial_data"],
		"parties_involved": payload["parties_involved"],
		"risks":            payload["risks"],
		"recommendations":  payload["recommendations"],
		"confidence_score": confidenceFromResult(payload, 0.75),
	}

	analysis := map[string]interface{}{
		"analysis_type":  docType,
		"extracted_data": extracted,
		"processing_info": map[string]interface{}{
			"processed_at": time.Now().Format(time.RFC3339),
			"model_used":   GetOpenAIModel(),
			"api_version":  "v2",
			"status":       "completed",
		},
	}

	if typeSpecific, ok := payload["type_specific"].(map[string]interface{}); ok && len(typeSpecific) > 0 {
		switch docType {
		case models.TypeInvoice:
			analysis["invoice_data"] = typeSpecific
		case models.TypeContract:
			analysis["contract_data"] = typeSpecific
		case models.TypeReport:
			analysis["report_data"] = typeSpecific
		case models.TypeForm:
			analysis["form_data"] = typeSpecific
		default:
			analysis["general_data"] = typeSpecific
		}
	}

	return analysis
}

// DetectDocumentType attempts to classify the document type based on content.
func (p *PDFProcessor) DetectDocumentType(text string) string {
	if text == "" {
		return models.TypeGeneral
	}

	textLower := strings.ToLower(text)

	if strings.Contains(textLower, "invoice") ||
		strings.Contains(textLower, "bill to") ||
		strings.Contains(textLower, "amount due") ||
		strings.Contains(textLower, "total amount") ||
		strings.Contains(textLower, "subtotal") ||
		(strings.Contains(textLower, "tax") && strings.Contains(textLower, "invoice")) {
		return models.TypeInvoice
	}

	if strings.Contains(textLower, "contract") ||
		strings.Contains(textLower, "agreement") ||
		strings.Contains(textLower, "party a") ||
		strings.Contains(textLower, "party b") ||
		strings.Contains(textLower, "clause") ||
		strings.Contains(textLower, "whereas") {
		return models.TypeContract
	}

	if strings.Contains(textLower, "report") ||
		strings.Contains(textLower, "analysis") ||
		strings.Contains(textLower, "findings") ||
		strings.Contains(textLower, "executive summary") ||
		strings.Contains(textLower, "recommendation") {
		return models.TypeReport
	}

	if strings.Contains(textLower, "form") ||
		strings.Contains(textLower, "application") ||
		strings.Contains(textLower, "please complete") ||
		strings.Contains(textLower, "fill out") {
		return models.TypeForm
	}

	return models.TypeGeneral
}

// ValidatePDF checks if the file is a valid PDF by extension.
func (p *PDFProcessor) ValidatePDF(filePath string) error {
	if !strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
		return fmt.Errorf("file is not a PDF")
	}
	return nil
}
