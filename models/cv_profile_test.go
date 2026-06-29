package models

import "testing"

func TestNormalizeDateOfBirth(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1990-06-15", "1990-06-15"},
		{"1990-06-15T00:00:00.000Z", "1990-06-15"},
		{"1990-06-15T00:00:00+02:00", "1990-06-15"},
		{" 1990-06-15 ", "1990-06-15"},
		{"", ""},
	}

	for _, tc := range tests {
		if got := NormalizeDateOfBirth(tc.in); got != tc.want {
			t.Fatalf("NormalizeDateOfBirth(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitizeCVProfileRemovesEmptyListEntries(t *testing.T) {
	p := &CVProfile{
		FirstName:          " Jane ",
		LastName:           "Doe",
		DateOfBirth:        "1990-06-15T00:00:00.000Z",
		Qualifications:     []string{"", " BCom "},
		Languages:          []string{""},
		ProfessionalSkills: []ProfessionalSkill{{Skill: " Go ", Details: []string{" ", "APIs"}}},
		Experience:         []CVExperience{{Company: " ", Position: "", ScopeOfWork: []string{""}}},
	}

	SanitizeCVProfile(p)

	if p.FirstName != "Jane" {
		t.Fatalf("expected trimmed first name, got %q", p.FirstName)
	}
	if p.DateOfBirth != "1990-06-15" {
		t.Fatalf("expected normalized date, got %q", p.DateOfBirth)
	}
	if len(p.Qualifications) != 1 || p.Qualifications[0] != "BCom" {
		t.Fatalf("unexpected qualifications: %#v", p.Qualifications)
	}
	if len(p.Languages) != 0 {
		t.Fatalf("expected empty languages, got %#v", p.Languages)
	}
	if len(p.ProfessionalSkills) != 1 || p.ProfessionalSkills[0].Skill != "Go" {
		t.Fatalf("unexpected skills: %#v", p.ProfessionalSkills)
	}
	if len(p.Experience) != 0 {
		t.Fatalf("expected empty experience, got %#v", p.Experience)
	}
}
