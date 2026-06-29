package models

import (
	"strings"
	"testing"
)

func TestSplitSearchTerms(t *testing.T) {
	terms := splitSearchTerms(" python, JavaScript;excel|python ")
	if len(terms) != 3 {
		t.Fatalf("terms = %#v, want 3 unique terms", terms)
	}
	if terms[0] != "python" || terms[1] != "JavaScript" || terms[2] != "excel" {
		t.Fatalf("unexpected terms: %#v", terms)
	}
}

func TestBuildCVTermGroupClause(t *testing.T) {
	clause, args := buildCVTermGroupClause([]string{"python", "bsc"})
	if clause == "" || len(args) != len(cvSearchableColumns())*2 {
		t.Fatalf("clause=%q args=%d", clause, len(args))
	}
	if !strings.Contains(clause, "professional_skills LIKE ?") || !strings.Contains(clause, "computer_skills LIKE ?") || !strings.Contains(clause, "qualifications LIKE ?") {
		t.Fatalf("missing searchable columns in clause: %s", clause)
	}
}
