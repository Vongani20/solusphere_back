package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"solusphere_backend/models"
)

func TestGenerateCVPDFMatchesTemplateBasics(t *testing.T) {
	svc := NewCVPDFService()
	if len(soluGrowthLogoPNG) == 0 {
		t.Fatal("expected embedded SoluGrowth logo")
	}

	pdf, err := svc.GeneratePDF(sampleCVProfile())
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("expected PDF output, got %d bytes", len(pdf))
	}
}

func TestGenerateCVPDFHandlesUnicodeWithoutPanic(t *testing.T) {
	svc := NewCVPDFService()
	profile := sampleCVProfile()
	profile.ProfileText = "Led the team’s delivery — “on time”… with café résumé skills"
	profile.ValueProposition = "Don’t settle for “good enough”"
	profile.Experience = []models.CVExperience{
		{
			Company:     "O’Reilly & Partners",
			Position:    "Senior Analyst – Insights",
			PeriodStart: "2020-01",
			ScopeOfWork: []string{
				"Owned stakeholder’s roadmap — end-to-end delivery",
				"Improved NPS from “fair” to “great”… continuously",
			},
		},
	}
	profile.ProfessionalSkills = []models.ProfessionalSkill{
		{Skill: "Communication", Details: []string{"Executive briefings — C-suite"}},
	}

	pdf, err := svc.GeneratePDF(profile)
	if err != nil {
		t.Fatalf("GeneratePDF with unicode failed: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("expected PDF output, got %d bytes", len(pdf))
	}
}

func TestPDFSafeTextMapsCommonUnicode(t *testing.T) {
	in := "team’s “quote” — café…"
	got := pdfSafeText(in)
	want := "team's \"quote\" - café..."
	if got != want {
		t.Fatalf("pdfSafeText(%q) = %q, want %q", in, got, want)
	}
}

func TestPDFHeaderClearsLogoHeight(t *testing.T) {
	if cvHeaderBottom <= cvMT+cvLogoH {
		t.Fatalf("cvHeaderBottom=%.1f must be below logo bottom %.1f", cvHeaderBottom, cvMT+cvLogoH)
	}
}

func TestGenerateCVPDFSidebarContinuationDoesNotPanic(t *testing.T) {
	svc := NewCVPDFService()
	profile := sampleCVProfile()
	// Force sidebar overflow onto a continuation page under the logo column.
	langs := make([]string, 40)
	for i := range langs {
		langs[i] = fmt.Sprintf("Language %02d with extra wording to wrap lines", i+1)
	}
	profile.Languages = langs
	profile.Qualifications = []string{"Q1", "Q2", "Q3", "Q4", "Q5", "Q6", "Q7", "Q8"}
	profile.ComputerSkills = []string{"C1", "C2", "C3", "C4", "C5", "C6", "C7", "C8"}
	profile.ProfessionalMemberships = []string{"M1", "M2", "M3", "M4", "M5"}
	profile.ProfessionalSkills = []models.ProfessionalSkill{
		{Skill: "Skill A", Details: []string{"d1", "d2", "d3", "d4"}},
		{Skill: "Skill B", Details: []string{"d1", "d2", "d3"}},
		{Skill: "Skill C", Details: []string{"d1", "d2"}},
	}

	pdf, err := svc.GeneratePDF(profile)
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("expected PDF output, got %d bytes", len(pdf))
	}
}

func TestGenerateCVWordMatchesTemplateBasics(t *testing.T) {
	svc := NewCVPDFService()
	if len(cvMasterTemplateDocx) == 0 {
		t.Fatal("expected embedded CV Master Template")
	}

	data, err := svc.GenerateWord(sampleCVProfile())
	if err != nil {
		t.Fatalf("GenerateWord failed: %v", err)
	}
	assertZipOfficeFile(t, data, "word/document.xml")

	xmlText := officeXMLContains(t, data, "word/document.xml")
	for _, want := range []string{
		"Jane Doe",
		"CURRICULUM VITAE",
		"PROFILE",
		"Female",
		"South African",
		"Process Management",
		"Lean Six Sigma",
		"BCom Information Systems",
		"Microsoft Office 365",
		"English",
		"EXPERIENCE",
		"Acme BPO",
		"Senior Analyst",
		"March 2018 - January 2024",
		"Led a team of 5 analysts",
	} {
		if !strings.Contains(xmlText, want) {
			t.Fatalf("expected document.xml to contain %q", want)
		}
	}
	for _, stale := range []string{"Name of Candidate", ">Male<", "Job Title", "VALUE PROPOSITION", "Date of Birth"} {
		if strings.Contains(xmlText, stale) {
			t.Fatalf("expected placeholder %q to be replaced", stale)
		}
	}

	// PROFILE and EXPERIENCE must share left indent 567.
	for _, heading := range []string{">PROFILE<", ">EXPERIENCE<"} {
		start, end, err := paragraphBounds(xmlText, heading, 0)
		if err != nil {
			t.Fatalf("heading %s: %v", heading, err)
		}
		para := xmlText[start:end]
		if !strings.Contains(para, `w:ind w:left="567"`) {
			t.Fatalf("heading %s missing left indent 567: %s", heading, para[:min(180, len(para))])
		}
	}

	scopeStart, scopeEnd, err := paragraphBounds(xmlText, ">Led a team of 5 analysts<", 0)
	if err != nil {
		t.Fatalf("scope item: %v", err)
	}
	scopePara := xmlText[scopeStart:scopeEnd]
	if !strings.Contains(scopePara, `w:numId w:val="2"`) {
		t.Fatalf("scope item should use list numbering, got: %s", scopePara[:min(220, len(scopePara))])
	}
	if !strings.Contains(scopePara, `w:ind w:left="1134"`) {
		t.Fatalf("scope item should keep list indent, got: %s", scopePara[:min(220, len(scopePara))])
	}
	if !strings.Contains(scopePara, `w:sz w:val="22"`) {
		t.Fatalf("scope item should use SoluGrowth 11pt (sz=22), got: %s", scopePara[:min(280, len(scopePara))])
	}

	// EXPERIENCE must start on page 2 (page break immediately before it).
	expStart, _, err := paragraphBounds(xmlText, ">EXPERIENCE<", 0)
	if err != nil {
		t.Fatalf("EXPERIENCE: %v", err)
	}
	beforeExp := xmlText[max(0, expStart-800):expStart]
	if !strings.Contains(beforeExp, `w:type="page"`) {
		t.Fatal("expected a page break immediately before EXPERIENCE")
	}

	// The personal-details sidebar table must be pinned to the page so it
	// always starts at the top, regardless of how much content it holds.
	if !strings.Contains(xmlText, `w:vertAnchor="page" w:horzAnchor="page" w:tblpX="6751" w:tblpY="1021"`) {
		t.Fatal("sidebar table should be page-anchored (personal details on top)")
	}
	if strings.Contains(xmlText, `w:vertAnchor="text"`) {
		t.Fatal("sidebar table still uses text anchoring")
	}
	if tblIdx := strings.Index(xmlText, "<w:tbl>"); tblIdx >= 0 {
		nameIdx := strings.Index(xmlText, "CURRICULUM VITAE")
		expBreakIdx := strings.Index(xmlText, `w:type="page"`)
		// Master template keeps the floating sidebar before the left column.
		if nameIdx >= 0 && expBreakIdx >= 0 && !(tblIdx < nameIdx && nameIdx < expBreakIdx) {
			t.Fatalf("sidebar should stay before title in master template order (table=%d title=%d break=%d)", tblIdx, nameIdx, expBreakIdx)
		}
	}

	// Name/photo/PROFILE live in a page-anchored left float beside the sidebar.
	if !strings.Contains(xmlText, `w:tblpX="851" w:tblpY="1100"`) {
		t.Fatal("left column should be page-anchored on page 1")
	}
}

func TestGenerateCVWordExportsFullContent(t *testing.T) {
	svc := NewCVPDFService()
	profile := &models.CVProfile{
		FirstName:        "Jane",
		LastName:         "Doe",
		ProfileText:      strings.Repeat("analyst experience ", 100),
		ValueProposition: strings.Repeat("delivers outcomes ", 100),
		Gender:           "Female",
		Nationality:      "South African",
		DateOfBirth:      "1990-06-15",
		ProfessionalSkills: []models.ProfessionalSkill{
			{Skill: "One"}, {Skill: "Two"}, {Skill: "Three"}, {Skill: "Four"}, {Skill: "Five"}, {Skill: "Six"},
		},
		Qualifications: []string{"Q1", "Q2", "Q3", "Q4", "Q5"},
		Experience: []models.CVExperience{
			{Company: "A", Position: "P", PeriodStart: "2020-01", ScopeOfWork: []string{"1", "2", "3", "4", "5"}},
			{Company: "B", Position: "P", PeriodStart: "2018-01", ScopeOfWork: []string{"1"}},
			{Company: "C", Position: "P", PeriodStart: "2016-01", ScopeOfWork: []string{"1"}},
		},
	}

	data, err := svc.GenerateWord(profile)
	if err != nil {
		t.Fatalf("GenerateWord failed: %v", err)
	}
	xmlText := officeXMLContains(t, data, "word/document.xml")
	for _, skill := range []string{"One", "Two", "Three", "Four", "Five", "Six"} {
		if !strings.Contains(xmlText, ">"+skill+"<") {
			t.Fatalf("expected skill %q in Word export", skill)
		}
	}
	if !strings.Contains(xmlText, ">Q5<") {
		t.Fatal("expected qualification Q5 in Word export")
	}
	if got := strings.Count(xmlText, ">Company:<"); got < 3 {
		t.Fatalf("company labels = %d, want at least 3", got)
	}
	if !strings.Contains(xmlText, ">C</w:t>") && !strings.Contains(xmlText, ">C<") {
		// Company C should appear as experience company value.
		if !strings.Contains(xmlText, "Company") {
			t.Fatal("expected third experience entry in Word export")
		}
	}
}

func TestGenerateCVPDFExportsAllExperienceAcrossPages(t *testing.T) {
	svc := NewCVPDFService()
	experience := make([]models.CVExperience, 0, 8)
	for i := 0; i < 8; i++ {
		scopes := make([]string, 0, 12)
		for s := 0; s < 12; s++ {
			scopes = append(scopes, fmt.Sprintf("Scope bullet %d for role %d with enough wording to wrap across lines", s+1, i+1))
		}
		experience = append(experience, models.CVExperience{
			Company:     fmt.Sprintf("Company-%d", i+1),
			Position:    fmt.Sprintf("Position-%d", i+1),
			PeriodStart: "2018-01",
			PeriodEnd:   "2020-12",
			ScopeOfWork: scopes,
		})
	}
	profile := sampleCVProfile()
	profile.Experience = experience
	profile.ProfessionalSkills = []models.ProfessionalSkill{
		{Skill: "Payroll administration and processing", Details: []string{"Full cycle payroll for large organisations with complex statutory requirements"}},
		{Skill: "Leave administration", Details: []string{"Managed leave provisions and applications across multiple business units"}},
	}

	data, err := svc.GeneratePDF(profile)
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
	if len(data) < 1000 {
		t.Fatalf("PDF too small: %d bytes", len(data))
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatal("expected PDF header")
	}
}

func sampleCVProfile() *models.CVProfile {
	return &models.CVProfile{
		FirstName:        "Jane",
		LastName:         "Doe",
		ProfileText:      "Experienced analyst.",
		ValueProposition: "I improve operations.",
		Gender:           "Female",
		Nationality:      "South African",
		DateOfBirth:      "1990-06-15",
		ProfessionalSkills: []models.ProfessionalSkill{
			{Skill: "Process Management", Details: []string{"Lean Six Sigma"}},
		},
		Qualifications: []string{"BCom Information Systems"},
		ComputerSkills: []string{"Microsoft Office 365"},
		Languages:      []string{"English"},
		Experience: []models.CVExperience{
			{
				Company:     "Acme BPO",
				Position:    "Senior Analyst",
				PeriodStart: "2018-03",
				PeriodEnd:   "2024-01",
				ScopeOfWork: []string{"Led a team of 5 analysts"},
			},
		},
	}
}

func officeXMLContains(t *testing.T, data []byte, required string) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip reader error: %v", err)
	}
	for _, file := range reader.File {
		if file.Name != required {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", required, err)
		}
		defer rc.Close()
		body, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %s: %v", required, err)
		}
		return string(body)
	}
	t.Fatalf("missing %s in docx zip", required)
	return ""
}

func TestStripLeadingBullet(t *testing.T) {
	cases := map[string]string{
		"• Led a team": "Led a team",
		"- Led a team": "Led a team",
		"* Led a team": "Led a team",
		"— Led a team": "Led a team",
		"Led a team":   "Led a team",
		"  -  Scope  ": "Scope",
	}
	for in, want := range cases {
		if got := stripLeadingBullet(in); got != want {
			t.Fatalf("stripLeadingBullet(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCVDocxPhotoPNGFitsFrameWithoutCropping(t *testing.T) {
	// Wide source: every source pixel must survive, padded to the frame ratio.
	src := image.NewRGBA(image.Rect(0, 0, 400, 100))
	draw.Draw(src, src.Bounds(), image.NewUniform(color.RGBA{R: 10, G: 20, B: 30, A: 255}), image.Point{}, draw.Src)

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatalf("encode source: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(encoded.Bytes())
	}))
	defer server.Close()

	out := cvDocxPhotoPNG(server.URL + "/photo.png")
	if out == nil {
		t.Fatal("expected photo bytes")
	}

	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	got := img.Bounds()
	if got.Dx() < src.Bounds().Dx() || got.Dy() < src.Bounds().Dy() {
		t.Fatalf("canvas %dx%d smaller than source %dx%d: image was cropped",
			got.Dx(), got.Dy(), src.Bounds().Dx(), src.Bounds().Dy())
	}

	ratio := float64(got.Dx()) / float64(got.Dy())
	if math.Abs(ratio-cvDocxPhotoRatio) > 0.01 {
		t.Fatalf("canvas ratio %.4f, want %.4f", ratio, cvDocxPhotoRatio)
	}

	// Padding is white so the frame background stays clean.
	if r, g, b, _ := img.At(got.Dx()/2, 1).RGBA(); r>>8 != 255 || g>>8 != 255 || b>>8 != 255 {
		t.Fatalf("expected white padding, got rgb(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

func TestImageTypeFromSource(t *testing.T) {
	if got := imageTypeFromSource("https://example.com/photo.png", ""); got != "PNG" {
		t.Fatalf("got %q, want PNG", got)
	}
	if got := imageTypeFromSource("https://example.com/photo", "image/jpeg"); got != "JPEG" {
		t.Fatalf("got %q, want JPEG", got)
	}
}
