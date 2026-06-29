package services

import (
	"fmt"
	"strings"

	"solusphere_backend/internal/ai"
)

type ReportFormat string

const (
	ReportFormatWord        ReportFormat = "word"
	ReportFormatExcel       ReportFormat = "excel"
	ReportFormatPowerPoint  ReportFormat = "powerpoint"
)

type SIAReportSource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type SIAReportTable struct {
	Title   string     `json:"title"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

type SIAReportSection struct {
	Heading    string   `json:"heading"`
	Paragraphs []string `json:"paragraphs"`
	Bullets    []string `json:"bullets"`
}

// SIAReportContent is the structured research report produced by SIA.
type SIAReportContent struct {
	Title           string             `json:"title"`
	Summary         string             `json:"summary"`
	Sections        []SIAReportSection `json:"sections"`
	Tables          []SIAReportTable   `json:"tables"`
	Recommendations []string           `json:"recommendations"`
	Sources         []SIAReportSource  `json:"sources"`
}

type GeneratedReport struct {
	Content     SIAReportContent
	Format      ReportFormat
	Filename    string
	ContentType string
	Data        []byte
	Model       string
	SourceCount int
}

func ParseReportFormat(raw string) (ReportFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "word", "doc", "docx":
		return ReportFormatWord, nil
	case "excel", "xls", "xlsx", "spreadsheet":
		return ReportFormatExcel, nil
	case "powerpoint", "ppt", "pptx", "presentation", "slides":
		return ReportFormatPowerPoint, nil
	default:
		return "", fmt.Errorf("unsupported report format %q (use word, excel, or powerpoint)", raw)
	}
}

func (f ReportFormat) Extension() string {
	switch f {
	case ReportFormatWord:
		return "docx"
	case ReportFormatExcel:
		return "xlsx"
	case ReportFormatPowerPoint:
		return "pptx"
	default:
		return "bin"
	}
}

func (f ReportFormat) ContentType() string {
	switch f {
	case ReportFormatWord:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ReportFormatExcel:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ReportFormatPowerPoint:
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}

func mergeSources(content *SIAReportContent, citations []ai.Citation) {
	seen := make(map[string]struct{}, len(content.Sources))
	for _, src := range content.Sources {
		url := strings.TrimSpace(src.URL)
		if url == "" {
			continue
		}
		seen[url] = struct{}{}
	}

	for _, citation := range citations {
		url := strings.TrimSpace(citation.URL)
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		content.Sources = append(content.Sources, SIAReportSource{
			Title: strings.TrimSpace(citation.Title),
			URL:   url,
		})
	}
}

func sanitizeFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "sia-report"
	}

	replacer := strings.NewReplacer(
		"\\", "-",
		"/", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	value = replacer.Replace(value)
	value = strings.Join(strings.Fields(value), "-")
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}
