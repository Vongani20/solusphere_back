package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"solusphere_backend/models"
)

type PDFProcessor struct {
	geminiService *GeminiService
}

func NewPDFProcessor(geminiService *GeminiService) *PDFProcessor {
	return &PDFProcessor{
		geminiService: geminiService,
	}
}

// ExtractTextFromPDF extracts text content from PDF file
// Note: You'll need to implement actual PDF text extraction
// For now, this is a placeholder that returns mock data
func (p *PDFProcessor) ExtractTextFromPDF(filePath string) (string, int, error) {
	log.Printf("Processing PDF file: %s", filePath)

	// TODO: Implement actual PDF text extraction
	// You can use libraries like:
	// - github.com/unidoc/unipdf/v3
	// - github.com/ledongthuc/pdf
	// - Or call an external service

	// Mock implementation for testing
	mockText := `INVOICE
    Invoice Number: INV-2024-001
    Date: January 15, 2024
    Vendor: ABC Corporation
    Customer: XYZ Enterprises
    Amount Due: $1,500.00
    Due Date: February 15, 2024
    
    Line Items:
    - Service Fee: $1,200.00
    - Tax: $300.00
    - Total: $1,500.00`

	return mockText, 1, nil
}

// AnalyzeBPOContent analyzes extracted text using Gemini AI
func (p *PDFProcessor) AnalyzeBPOContent(ctx context.Context, text string, analysisType string) (map[string]interface{}, error) {
	if text == "" {
		return nil, fmt.Errorf("no text content to analyze")
	}

	prompt := p.generateAnalysisPrompt(text, analysisType)

	result, err := p.geminiService.AnalyzeContent(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze content with Gemini: %v", err)
	}

	return p.parseAnalysisResult(result, analysisType), nil
}

// generateAnalysisPrompt creates tailored prompts for different BPO document types
func (p *PDFProcessor) generateAnalysisPrompt(text string, analysisType string) string {
	basePrompt := `Analyze the following BPO (Business Process Outsourcing) document and extract structured information. 
    Focus on identifying key business entities, financial data, dates, parties involved, and important terms.
    Provide the response in a structured JSON format.`

	switch analysisType {
	case models.TypeInvoice:
		return basePrompt + ` Specifically for this INVOICE document, extract:
        - Invoice number, issue date, due date
        - Vendor details (name, address, contact)
        - Customer details (name, address, contact)
        - Line items (description, quantity, unit price, total)
        - Subtotals, taxes, discounts, grand total
        - Payment terms, methods, and instructions
        - Currency and tax identification numbers
        - Any special notes or terms
        Document text: ` + text

	case models.TypeContract:
		return basePrompt + ` Specifically for this CONTRACT document, extract:
        - Contract title and effective date
        - Parties involved with their roles and details
        - Contract duration and termination clauses
        - Key obligations and responsibilities
        - Payment terms, amounts, and schedules
        - Confidentiality and non-disclosure terms
        - Liability and indemnification clauses
        - Governing law and dispute resolution
        - Important deadlines and milestones
        - Renewal and termination conditions
        Document text: ` + text

	case models.TypeReport:
		return basePrompt + ` Specifically for this REPORT document, extract:
        - Report title and type
        - Report period and date
        - Authors and contributors
        - Executive summary and key findings
        - Data metrics and performance indicators
        - Recommendations and action items
        - Risk assessments and mitigation strategies
        - Financial data and analysis
        - Conclusions and next steps
        Document text: ` + text

	case models.TypeForm:
		return basePrompt + ` Specifically for this FORM document, extract:
        - Form title and purpose
        - Required fields and sections
        - Submission instructions and deadlines
        - Contact information and support details
        - Terms and conditions
        - Required attachments or documentation
        Document text: ` + text

	default:
		return basePrompt + ` For this GENERAL document, extract:
        - Document type and purpose
        - Key entities and stakeholders
        - Important dates and timelines
        - Financial figures and amounts
        - Contact information
        - Main topics and subjects
        - Action items or next steps
        Document text: ` + text
	}
}

// parseAnalysisResult processes the AI response into structured data
func (p *PDFProcessor) parseAnalysisResult(result string, analysisType string) map[string]interface{} {
	analysis := map[string]interface{}{
		"analysis_type": analysisType,
		"extracted_data": map[string]interface{}{
			"summary":          result,
			"document_type":    analysisType,
			"key_entities":     []string{},
			"important_dates":  []string{},
			"financial_data":   []string{},
			"parties_involved": []string{},
			"confidence_score": 0.85,
		},
		"processing_info": map[string]interface{}{
			"processed_at": time.Now().Format(time.RFC3339),
			"model_used":   "gemini-pro",
			"api_version":  "v1",
			"status":       "completed",
		},
	}

	// Add type-specific fields
	switch analysisType {
	case models.TypeInvoice:
		analysis["invoice_data"] = map[string]interface{}{
			"invoice_number": "",
			"vendor_info":    map[string]string{},
			"customer_info":  map[string]string{},
			"line_items":     []map[string]interface{}{},
			"total_amount":   0.0,
			"currency":       "",
		}
	case models.TypeContract:
		analysis["contract_data"] = map[string]interface{}{
			"contract_title": "",
			"parties":        []map[string]string{},
			"effective_date": "",
			"end_date":       "",
			"key_clauses":    []string{},
			"payment_terms":  map[string]interface{}{},
		}
	case models.TypeReport:
		analysis["report_data"] = map[string]interface{}{
			"report_title":    "",
			"period_covered":  "",
			"key_findings":    []string{},
			"recommendations": []string{},
			"metrics":         map[string]interface{}{},
		}
	}

	return analysis
}

// DetectDocumentType attempts to classify the document type based on content
func (p *PDFProcessor) DetectDocumentType(text string) string {
	if text == "" {
		return models.TypeGeneral
	}

	textLower := strings.ToLower(text)

	// Check for invoice indicators
	if strings.Contains(textLower, "invoice") ||
		strings.Contains(textLower, "bill to") ||
		strings.Contains(textLower, "amount due") ||
		strings.Contains(textLower, "total amount") ||
		strings.Contains(textLower, "subtotal") ||
		(strings.Contains(textLower, "tax") && strings.Contains(textLower, "invoice")) {
		return models.TypeInvoice
	}

	// Check for contract indicators
	if strings.Contains(textLower, "contract") ||
		strings.Contains(textLower, "agreement") ||
		strings.Contains(textLower, "party a") ||
		strings.Contains(textLower, "party b") ||
		strings.Contains(textLower, "clause") ||
		strings.Contains(textLower, "whereas") {
		return models.TypeContract
	}

	// Check for report indicators
	if strings.Contains(textLower, "report") ||
		strings.Contains(textLower, "analysis") ||
		strings.Contains(textLower, "findings") ||
		strings.Contains(textLower, "executive summary") ||
		strings.Contains(textLower, "recommendation") {
		return models.TypeReport
	}

	// Check for form indicators
	if strings.Contains(textLower, "form") ||
		strings.Contains(textLower, "application") ||
		strings.Contains(textLower, "please complete") ||
		strings.Contains(textLower, "fill out") {
		return models.TypeForm
	}

	return models.TypeGeneral
}

// ValidatePDF checks if the file is a valid PDF (placeholder)
func (p *PDFProcessor) ValidatePDF(filePath string) error {
	// TODO: Implement actual PDF validation
	// For now, just check if file exists and has .pdf extension
	if !strings.HasSuffix(strings.ToLower(filePath), ".pdf") {
		return fmt.Errorf("file is not a PDF")
	}
	return nil
}
