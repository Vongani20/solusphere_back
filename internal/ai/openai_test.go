package ai

import "testing"

func TestNormalizeOpenAIModelFallsBackForSecretLikeValue(t *testing.T) {
	model := NormalizeOpenAIModel("sk-proj-this-looks-like-a-key")
	if model != DefaultOpenAIModel {
		t.Fatalf("expected default model, got %q", model)
	}
}

func TestNormalizeOpenAIModelKeepsModelName(t *testing.T) {
	model := NormalizeOpenAIModel("gpt-4.1")
	if model != "gpt-4.1" {
		t.Fatalf("expected provided model, got %q", model)
	}
}
