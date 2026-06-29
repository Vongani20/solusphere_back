package services

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

func buildExcelReport(content SIAReportContent) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	_ = f.SetSheetName(sheet, "Report")
	sheet = "Report"

	row := 1
	row = writeExcelCell(f, sheet, row, "Title", content.Title)
	row++
	row = writeExcelCell(f, sheet, row, "Summary", content.Summary)
	row += 2

	for _, section := range content.Sections {
		if section.Heading != "" {
			row = writeExcelCell(f, sheet, row, "Section", section.Heading)
		}
		for _, paragraph := range section.Paragraphs {
			row = writeExcelCell(f, sheet, row, "Detail", paragraph)
		}
		for _, bullet := range section.Bullets {
			row = writeExcelCell(f, sheet, row, "Bullet", bullet)
		}
		row++
	}

	for _, table := range content.Tables {
		if table.Title != "" {
			row = writeExcelCell(f, sheet, row, "Table", table.Title)
		}
		if len(table.Headers) > 0 {
			for col, header := range table.Headers {
				cell, _ := excelize.CoordinatesToCellName(col+1, row)
				_ = f.SetCellValue(sheet, cell, header)
			}
			row++
			for _, dataRow := range table.Rows {
				for col := range table.Headers {
					value := ""
					if col < len(dataRow) {
						value = dataRow[col]
					}
					cell, _ := excelize.CoordinatesToCellName(col+1, row)
					_ = f.SetCellValue(sheet, cell, value)
				}
				row++
			}
			row++
		}
	}

	if len(content.Recommendations) > 0 {
		row = writeExcelCell(f, sheet, row, "Recommendations", "")
		for _, item := range content.Recommendations {
			row = writeExcelCell(f, sheet, row, "Recommendation", item)
		}
		row++
	}

	if len(content.Sources) > 0 {
		row = writeExcelCell(f, sheet, row, "Sources", "")
		for _, source := range content.Sources {
			row = writeExcelCell(f, sheet, row, source.Title, source.URL)
		}
	}

	_ = f.SetColWidth(sheet, "A", "A", 18)
	_ = f.SetColWidth(sheet, "B", "B", 90)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

func writeExcelCell(f *excelize.File, sheet string, row int, label, value string) int {
	labelCell, _ := excelize.CoordinatesToCellName(1, row)
	valueCell, _ := excelize.CoordinatesToCellName(2, row)
	_ = f.SetCellValue(sheet, labelCell, label)
	_ = f.SetCellValue(sheet, valueCell, value)
	return row + 1
}
