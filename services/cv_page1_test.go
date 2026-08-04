package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestGenerateCVWordKeepsProfileOnPage1(t *testing.T) {
	svc := NewCVPDFService()
	profile := sampleCVProfile()
	profile.FirstName = "Mbodi"
	profile.LastName = "Awelani"
	profile.ProfileText = "Passionate of being a key business partner where strategy is owned and executed. Finance Executive with more than 15 years experience in managing Finance Teams and ensuring all statutory and financial deadlines are kept."
	profile.Languages = nil // so template placeholder path is exercised too

	data, err := svc.GenerateWord(profile)
	if err != nil {
		t.Fatalf("GenerateWord: %v", err)
	}

	xmlText := officeXMLContains(t, data, "word/document.xml")
	nameIdx := strings.Index(xmlText, ">Mbodi Awelani<")
	if nameIdx < 0 {
		t.Fatal("expected candidate name in document")
	}
	profIdx := strings.Index(xmlText, ">PROFILE<")
	if profIdx < 0 {
		t.Fatal("expected PROFILE heading")
	}
	expIdx := strings.Index(xmlText, ">EXPERIENCE<")
	if expIdx < 0 {
		t.Fatal("expected EXPERIENCE heading")
	}

	breaks := strings.Count(xmlText, `w:type="page"`)
	if breaks != 1 {
		t.Fatalf("page breaks = %d, want exactly 1 (before EXPERIENCE)", breaks)
	}

	breakIdx := strings.Index(xmlText, `w:type="page"`)
	if !(nameIdx < breakIdx && profIdx < breakIdx && breakIdx < expIdx) {
		t.Fatalf("expected order name(%d) < PROFILE(%d) < pageBreak(%d) < EXPERIENCE(%d)", nameIdx, profIdx, breakIdx, expIdx)
	}

	// PROFILE body text must appear before the page break (page 1).
	bodyIdx := strings.Index(xmlText, "Passionate of being a key business partner")
	if bodyIdx < 0 || bodyIdx > breakIdx {
		t.Fatalf("PROFILE body must be on page 1 before EXPERIENCE page break (body=%d break=%d)", bodyIdx, breakIdx)
	}

	// Sidebar + left-column floats must both sit before the EXPERIENCE page break.
	tblIdx := strings.Index(xmlText, "<w:tbl>")
	if tblIdx < 0 || tblIdx > breakIdx {
		t.Fatalf("PERSONAL DETAILS sidebar must be before EXPERIENCE page break (table=%d break=%d)", tblIdx, breakIdx)
	}
	if !(tblIdx < nameIdx && nameIdx < profIdx && profIdx < breakIdx) {
		t.Fatalf("expected sidebar(%d) < name(%d) < PROFILE(%d) < pageBreak(%d)", tblIdx, nameIdx, profIdx, breakIdx)
	}
	if !strings.Contains(xmlText[:breakIdx], "PERSONAL") {
		t.Fatal("expected PERSONAL DETAILS content before EXPERIENCE page break")
	}
	if !strings.Contains(xmlText, `w:tblpX="851" w:tblpY="1100"`) {
		t.Fatal("expected page-1 left column floating table")
	}
	if !strings.Contains(xmlText, `w:vertAnchor="page" w:horzAnchor="page" w:tblpX="6751" w:tblpY="1021"`) {
		t.Fatal("expected page-1 sidebar floating table")
	}
}

func TestFloatCVPage1LeftColumn(t *testing.T) {
	raw, err := cvMasterTemplateDocxBytes()
	if err != nil {
		t.Fatal(err)
	}
	doc := compactCVPage1LeftColumn(pinCVSidebarTable(string(raw)))
	// Simulate filled name so CURRICULUM VITAE paragraph is still the anchor.
	out := floatCVPage1LeftColumn(doc)
	if out == doc {
		t.Fatal("expected left column to be wrapped in a floating table")
	}
	if !strings.Contains(out, `w:tblpX="851" w:tblpY="1100"`) {
		t.Fatal("missing left-column page anchor")
	}
	leftIdx := strings.Index(out, `w:tblpX="851"`)
	nameIdx := strings.Index(out, "CURRICULUM VITAE")
	profIdx := strings.Index(out, ">PROFILE<")
	brIdx := strings.Index(out, `w:type="page"`)
	if !(leftIdx < nameIdx && nameIdx < profIdx && profIdx < brIdx) {
		t.Fatalf("expected left float to contain name/PROFILE before page break (left=%d name=%d profile=%d break=%d)", leftIdx, nameIdx, profIdx, brIdx)
	}
	var node struct{}
	if err := xml.Unmarshal([]byte(out), &node); err != nil {
		t.Fatalf("floated left column must keep well-formed XML: %v", err)
	}
}

func TestGenerateCVWordProducesValidDocumentXML(t *testing.T) {
	svc := NewCVPDFService()
	data, err := svc.GenerateWord(sampleCVProfile())
	if err != nil {
		t.Fatalf("GenerateWord: %v", err)
	}

	xmlBytes := officeXMLBytesFromDocx(t, data, "word/document.xml")
	var node struct{}
	if err := xml.Unmarshal(xmlBytes, &node); err != nil {
		t.Fatalf("word/document.xml should be well-formed XML: %v", err)
	}
}

func TestCompactCVPage1LeftColumnRemovesSpacers(t *testing.T) {
	raw, err := cvMasterTemplateDocxBytes()
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	out := compactCVPage1LeftColumn(doc)
	if out == doc {
		t.Fatal("expected VALUE PROPOSITION spacer paragraphs to be removed")
	}
	if len(out) >= len(doc) {
		t.Fatalf("compact should shrink document.xml: before=%d after=%d", len(doc), len(out))
	}

	before := countEmptyParasBetween(doc, "Name of Candidate", ">PROFILE<")
	after := countEmptyParasBetween(out, "Name of Candidate", ">PROFILE<")
	if before < 5 {
		t.Fatalf("fixture expectation: master template should have many spacers before PROFILE, got %d", before)
	}
	if after > 2 {
		t.Fatalf("expected at most 2 empty top-level paragraphs between name and PROFILE, before=%d after=%d", before, after)
	}
	if !strings.Contains(out, "<w:drawing>") || !strings.Contains(out, ">PROFILE<") {
		t.Fatal("compact must keep photo drawing and PROFILE")
	}

	var node struct{}
	if err := xml.Unmarshal([]byte(out), &node); err != nil {
		t.Fatalf("compacted document.xml must stay well-formed: %v", err)
	}
}

func countEmptyParasBetween(doc, startMarker, endMarker string) int {
	start, _, err := paragraphBounds(doc, startMarker, 0)
	if err != nil {
		return -1
	}
	end, _, err := paragraphBounds(doc, endMarker, 0)
	if err != nil {
		return -1
	}
	empties := 0
	pos := start
	for pos < end {
		pStart, pEnd, err := nextParagraphDepth(doc, pos)
		if err != nil || pStart >= end {
			break
		}
		para := doc[pStart:pEnd]
		if !hasVisibleText(para) && !strings.Contains(para, "<w:drawing>") {
			empties++
		}
		pos = pEnd
	}
	return empties
}

func cvMasterTemplateDocxBytes() ([]byte, error) {
	// Read embedded template document.xml via GenerateWord empty path is heavy;
	// use zip of embedded bytes.
	r, err := zip.NewReader(bytes.NewReader(cvMasterTemplateDocx), int64(len(cvMasterTemplateDocx)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, io.ErrUnexpectedEOF
}

func officeXMLBytesFromDocx(t *testing.T, data []byte, path string) []byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open generated docx: %v", err)
	}
	for _, f := range r.File {
		if f.Name != path {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return b
	}
	t.Fatalf("missing %s", path)
	return nil
}
