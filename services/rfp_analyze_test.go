package services

import (
	"os"
	"testing"

	"solusphere_backend/models"
)

func TestBuildLocalRFPTemplateFromSample(t *testing.T) {
	data, err := os.ReadFile("testdata/rfp_sample.txt")
	if err != nil {
		t.Skip("sample not present")
	}
	tmpl := withRFPSolution(buildLocalRFPTemplate(CleanRFPText(string(data)), "sample.txt"))
	if tmpl.Title == "" {
		t.Fatal("expected a title")
	}
	if len(tmpl.Scope) == 0 && len(tmpl.Requirements) == 0 {
		t.Fatalf("expected scope or requirements, got %+v", tmpl)
	}
	if tmpl.Solution.Summary == "" {
		t.Fatal("expected a solution idea")
	}
	if len(tmpl.Requirements) > 0 && len(tmpl.Solution.Responses) == 0 {
		t.Fatal("expected a proposal response for each requirement")
	}
	doc := &models.RFPDocument{Template: tmpl, Sources: []models.RFPSource{{Name: "sample.txt"}}}
	out, err := RenderRFPDocx(doc)
	if err != nil || len(out) < 1000 || string(out[:2]) != "PK" {
		t.Fatalf("docx render failed: %v (len %d)", err, len(out))
	}
	t.Logf("title=%q issuer=%q closing=%q scope=%d criteria=%d gaps=%d", tmpl.Title, tmpl.Issuer, tmpl.ClosingDate, len(tmpl.Scope), len(tmpl.EvaluationCriteria), len(tmpl.Gaps))
}

func TestNormalizeRFPTemplateMapLoose(t *testing.T) {
	tmpl := models.NormalizeRFPTemplateMap(map[string]interface{}{
		"title":              "  A   tender ",
		"scope":              "single string",
		"evaluationCriteria": []interface{}{map[string]interface{}{"name": "Price", "score": "80%"}, "Functionality"},
		"requirements":       []interface{}{map[string]interface{}{"points": []interface{}{"x", ""}}},
	})
	if tmpl.Title != "A tender" || len(tmpl.Scope) != 1 || len(tmpl.EvaluationCriteria) != 2 || tmpl.EvaluationCriteria[0].Weight != "80%" {
		t.Fatalf("unexpected: %+v", tmpl)
	}
	if tmpl.Requirements[0].Category != "Requirements" || len(tmpl.Requirements[0].Items) != 1 {
		t.Fatalf("unexpected requirements: %+v", tmpl.Requirements)
	}
	if tmpl.Objectives == nil {
		t.Fatal("lists must not be nil")
	}
}

func TestSplitRFPChunksCoversText(t *testing.T) {
	text := ""
	for i := 0; i < 3000; i++ {
		text += "paragraph text here\n\n"
	}
	chunks := splitRFPChunks(text, 10000)
	if len(chunks) < 6 {
		t.Fatalf("expected several chunks, got %d", len(chunks))
	}
}
