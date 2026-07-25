package services

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"solusphere_backend/models"
)

func TestFitCVProfileForTemplateCapsListsAndWords(t *testing.T) {
	longWords := strings.Repeat("word ", 200)
	profile := FitCVProfileForTemplate(&models.CVProfile{
		FirstName:        "Jane",
		LastName:         "Doe",
		ProfileText:      longWords,
		ValueProposition: longWords,
		ProfessionalSkills: []models.ProfessionalSkill{
			{Skill: "A", Details: []string{"1", "2", "3"}},
			{Skill: "B", Details: []string{"1"}},
			{Skill: "C", Details: []string{"1"}},
			{Skill: "D", Details: []string{"1"}},
			{Skill: "E", Details: []string{"1"}},
		},
		Qualifications:          []string{"1", "2", "3", "4", "5", "6"},
		ComputerSkills:          []string{"1", "2", "3", "4", "5"},
		ProfessionalMemberships: []string{"1", "2", "3", "4"},
		Languages:               []string{"1", "2", "3", "4", "5"},
		Experience: []models.CVExperience{
			{Company: "One", ScopeOfWork: []string{"a", "b", "c", "d", "e"}},
			{Company: "Two", ScopeOfWork: []string{"a"}},
			{Company: "Three", ScopeOfWork: []string{"a"}},
		},
	})

	if got := len(strings.Fields(profile.ProfileText)); got > cvMaxProfileWords {
		t.Fatalf("profile words = %d, want <= %d", got, cvMaxProfileWords)
	}
	if got := len(strings.Fields(profile.ValueProposition)); got > cvMaxValuePropWords {
		t.Fatalf("value prop words = %d, want <= %d", got, cvMaxValuePropWords)
	}
	if len(profile.ProfessionalSkills) != cvMaxProfessionalSkills {
		t.Fatalf("skills = %d, want %d", len(profile.ProfessionalSkills), cvMaxProfessionalSkills)
	}
	if len(profile.ProfessionalSkills[0].Details) != cvMaxSkillDetails {
		t.Fatalf("skill details = %d, want %d", len(profile.ProfessionalSkills[0].Details), cvMaxSkillDetails)
	}
	if len(profile.Qualifications) != cvMaxQualifications {
		t.Fatalf("qualifications = %d", len(profile.Qualifications))
	}
	if len(profile.Experience) != cvMaxExperience {
		t.Fatalf("experience = %d, want %d", len(profile.Experience), cvMaxExperience)
	}
	if len(profile.Experience[0].ScopeOfWork) != cvMaxScopeBullets {
		t.Fatalf("scope bullets = %d, want %d", len(profile.Experience[0].ScopeOfWork), cvMaxScopeBullets)
	}
}

func TestExtractTextFromDocxBytes(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Jane Doe</w:t></w:r></w:p><w:p><w:r><w:t>Senior Analyst</w:t></w:r></w:p></w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	text, err := ExtractTextFromDocxBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("ExtractTextFromDocxBytes: %v", err)
	}
	if !strings.Contains(text, "Jane Doe") || !strings.Contains(text, "Senior Analyst") {
		t.Fatalf("unexpected text: %q", text)
	}

	if _, err := ExtractTextFromCVUpload("cv.docx", buf.Bytes()); err != nil {
		t.Fatalf("ExtractTextFromCVUpload docx: %v", err)
	}
	if _, err := ExtractTextFromCVUpload("cv.doc", buf.Bytes()); err == nil {
		t.Fatal("expected legacy .doc rejection")
	}
}
