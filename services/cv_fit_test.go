package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"solusphere_backend/models"
)

func TestFitCVProfileForTemplateCapsListsAndWords(t *testing.T) {
	longWords := strings.Repeat("word ", cvMaxProfileWords+50)
	skills := make([]models.ProfessionalSkill, 0, cvMaxProfessionalSkills+5)
	for i := 0; i < cvMaxProfessionalSkills+5; i++ {
		details := make([]string, 0, cvMaxSkillDetails+3)
		for d := 0; d < cvMaxSkillDetails+3; d++ {
			details = append(details, fmt.Sprintf("detail-%d-%d", i, d))
		}
		skills = append(skills, models.ProfessionalSkill{
			Skill:   fmt.Sprintf("Skill-%d", i),
			Details: details,
		})
	}
	qualifications := make([]string, 0, cvMaxQualifications+5)
	for i := 0; i < cvMaxQualifications+5; i++ {
		qualifications = append(qualifications, fmt.Sprintf("Qualification-%d", i))
	}
	experience := make([]models.CVExperience, 0, cvMaxExperience+5)
	for i := 0; i < cvMaxExperience+5; i++ {
		scope := make([]string, 0, cvMaxScopeBullets+3)
		for s := 0; s < cvMaxScopeBullets+3; s++ {
			scope = append(scope, fmt.Sprintf("scope-%d-%d", i, s))
		}
		experience = append(experience, models.CVExperience{
			Company:     fmt.Sprintf("Company-%d", i),
			ScopeOfWork: scope,
		})
	}

	profile := FitCVProfileForTemplate(&models.CVProfile{
		FirstName:               "Jane",
		LastName:                "Doe",
		ProfileText:             longWords,
		ValueProposition:        longWords,
		ProfessionalSkills:      skills,
		Qualifications:          qualifications,
		ComputerSkills:          qualifications,
		ProfessionalMemberships: qualifications,
		Languages:               qualifications,
		Experience:              experience,
	})

	if got := len(strings.Fields(profile.ProfileText)); got != cvMaxProfileWords {
		t.Fatalf("profile words = %d, want %d", got, cvMaxProfileWords)
	}
	if got := len(strings.Fields(profile.ValueProposition)); got != cvMaxValuePropWords {
		t.Fatalf("value prop words = %d, want %d", got, cvMaxValuePropWords)
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
	if got := detectCVDocumentKind("cv.doc", append([]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, []byte("Curriculum Vitae John Doe Experience Finance")...)); got != "doc" {
		t.Fatalf("detectCVDocumentKind(.doc)=%q, want doc", got)
	}
	if got := detectCVDocumentKind("scan.jpg", []byte{0xff, 0xd8, 0xff, 0xe0}); got != "jpeg" {
		t.Fatalf("detectCVDocumentKind(jpeg)=%q, want jpeg", got)
	}
}
