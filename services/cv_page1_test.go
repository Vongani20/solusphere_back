package services

import (
	"archive/zip"
	"bytes"
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
}

func TestPinCVProfilePhotoUsesPageAnchor(t *testing.T) {
	raw, err := cvMasterTemplateDocxBytes()
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	out := pinCVProfilePhoto(doc)
	if out == doc {
		t.Fatal("expected photo anchor to be rewritten")
	}
	if !strings.Contains(out, `relativeFrom="page"><wp:posOffset>432000</wp:posOffset>`) {
		t.Fatal("expected page-relative horizontal pin")
	}
	if !strings.Contains(out, `relativeFrom="page"><wp:posOffset>792000</wp:posOffset>`) {
		t.Fatal("expected page-relative vertical pin under the name")
	}
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
