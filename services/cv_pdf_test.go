package services

import (
	"archive/zip"
	"bytes"
	"io"
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
		"VALUE PROPOSITION",
		"Date of Birth",
		"Female",
		"South African",
		"15 June 1990",
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
	for _, stale := range []string{"Name of Candidate", ">Male<", "Job Title"} {
		if strings.Contains(xmlText, stale) {
			t.Fatalf("expected placeholder %q to be replaced", stale)
		}
	}
}

func TestGenerateCVWordDoesNotOverPopulate(t *testing.T) {
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
	if strings.Contains(xmlText, ">Five<") || strings.Contains(xmlText, ">Six<") {
		t.Fatal("expected surplus skills to be omitted from Word export")
	}
	if strings.Contains(xmlText, ">Q5<") {
		t.Fatal("expected surplus qualifications to be omitted from Word export")
	}
	if got := strings.Count(xmlText, ">Company:<"); got != cvMaxExperience {
		t.Fatalf("company labels = %d, want %d", got, cvMaxExperience)
	}
	if strings.Contains(xmlText, ">C</w:t>") {
		t.Fatal("expected third experience entry to be omitted from Word export")
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

func TestImageTypeFromSource(t *testing.T) {
	if got := imageTypeFromSource("https://example.com/photo.png", ""); got != "PNG" {
		t.Fatalf("got %q, want PNG", got)
	}
	if got := imageTypeFromSource("https://example.com/photo", "image/jpeg"); got != "JPEG" {
		t.Fatalf("got %q, want JPEG", got)
	}
}
