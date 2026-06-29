package models

import "testing"

func TestNullJSONIfEmpty(t *testing.T) {
	if nullJSONIfEmpty("") != nil {
		t.Fatal("expected nil for empty JSON value")
	}
	if nullJSONIfEmpty("   ") != nil {
		t.Fatal("expected nil for whitespace JSON value")
	}
	if nullJSONIfEmpty(`{"ok":true}`) != `{"ok":true}` {
		t.Fatal("expected JSON string to pass through")
	}
}
