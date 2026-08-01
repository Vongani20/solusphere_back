package models

import "testing"

func TestNormalizePhoneE164(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0792531102", "+27792531102"},
		{"+27792531102", "+27792531102"},
		{"27792531102", "+27792531102"},
		{"+27 79 253 1102", "+27792531102"},
		{"079 253 1102", "+27792531102"},
		{"792531102", "+27792531102"},
		{"", ""},
		{"  ", ""},
	}

	for _, tc := range tests {
		got := NormalizePhoneE164(tc.in, "27")
		if got != tc.want {
			t.Fatalf("NormalizePhoneE164(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
