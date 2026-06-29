package services

import (
	"strings"
	"testing"

	"solusphere_backend/models"
)

func TestGenerateCVPDFMatchesTemplateBasics(t *testing.T) {
	svc := NewCVPDFService()
	if len(soluGrowthLogoPNG) == 0 {
		t.Fatal("expected embedded SoluGrowth logo")
	}

	pdf, err := svc.GeneratePDF(&models.CVProfile{
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
	})
	if err != nil {
		t.Fatalf("GeneratePDF failed: %v", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatalf("expected PDF output, got %d bytes", len(pdf))
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
