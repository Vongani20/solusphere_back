package services

import (
	"archive/zip"
	"bytes"
	"testing"

	"solusphere_backend/internal/ai"
)

func sampleReportContent() SIAReportContent {
	return SIAReportContent{
		Title:   "BPO Market Trends",
		Summary: "South Africa BPO growth remains strong in 2026.",
		Sections: []SIAReportSection{
			{
				Heading:    "Market Overview",
				Paragraphs: []string{"Demand for customer experience outsourcing is rising."},
				Bullets:    []string{"Voice remains dominant", "Digital channels are growing"},
			},
		},
		Tables: []SIAReportTable{
			{
				Title:   "Key Metrics",
				Headers: []string{"Metric", "Value"},
				Rows:    [][]string{{"Growth", "8%"}, {"Jobs", "120k"}},
			},
		},
		Recommendations: []string{"Invest in digital channels", "Expand training programs"},
		Sources: []SIAReportSource{
			{Title: "Industry Report", URL: "https://example.com/report"},
		},
	}
}

func TestParseReportFormat(t *testing.T) {
	tests := map[string]ReportFormat{
		"word":        ReportFormatWord,
		"docx":        ReportFormatWord,
		"excel":       ReportFormatExcel,
		"xlsx":        ReportFormatExcel,
		"powerpoint":  ReportFormatPowerPoint,
		"pptx":        ReportFormatPowerPoint,
	}

	for input, want := range tests {
		got, err := ParseReportFormat(input)
		if err != nil {
			t.Fatalf("ParseReportFormat(%q) error: %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseReportFormat(%q) = %q, want %q", input, got, want)
		}
	}

	if _, err := ParseReportFormat("pdf"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestParseReportJSON(t *testing.T) {
	raw := "```json\n{\"title\":\"Test\",\"summary\":\"Summary\",\"sections\":[],\"tables\":[],\"recommendations\":[],\"sources\":[]}\n```"
	content, err := parseReportJSON(raw)
	if err != nil {
		t.Fatalf("parseReportJSON error: %v", err)
	}
	if content.Title != "Test" {
		t.Fatalf("title = %q", content.Title)
	}
}

func TestBuildWordReport(t *testing.T) {
	data, err := BuildReportFileForTest(sampleReportContent(), ReportFormatWord)
	if err != nil {
		t.Fatalf("buildWordReport error: %v", err)
	}
	assertZipOfficeFile(t, data, "word/document.xml")
}

func TestBuildExcelReport(t *testing.T) {
	data, err := BuildReportFileForTest(sampleReportContent(), ReportFormatExcel)
	if err != nil {
		t.Fatalf("buildExcelReport error: %v", err)
	}
	assertZipOfficeFile(t, data, "xl/workbook.xml")
}

func TestBuildPowerPointReport(t *testing.T) {
	data, err := BuildReportFileForTest(sampleReportContent(), ReportFormatPowerPoint)
	if err != nil {
		t.Fatalf("buildPowerPointReport error: %v", err)
	}
	assertZipOfficeFile(t, data, "ppt/presentation.xml")
}

func assertZipOfficeFile(t *testing.T, data []byte, required string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip reader error: %v", err)
	}
	for _, file := range reader.File {
		if file.Name == required {
			return
		}
	}
	t.Fatalf("expected %s in generated file", required)
}

func TestMergeSources(t *testing.T) {
	content := SIAReportContent{
		Sources: []SIAReportSource{{Title: "A", URL: "https://a.example"}},
	}
	mergeSources(&content, []ai.Citation{
		{Title: "B", URL: "https://b.example"},
		{Title: "A duplicate", URL: "https://a.example"},
	})
	if len(content.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(content.Sources))
	}
}
