package models

import (
	"testing"
	"time"
)

func TestParseBirthDate(t *testing.T) {
	if _, err := ParseBirthDate(""); err == nil {
		t.Fatal("empty birth date should be rejected")
	}
	if _, err := ParseBirthDate("15/06/1990"); err == nil {
		t.Fatal("non ISO birth date should be rejected")
	}
	got, err := ParseBirthDate("1990-06-15")
	if err != nil || got.Format("2006-01-02") != "1990-06-15" {
		t.Fatalf("parse: %v %v", got, err)
	}
}

func TestIsBirthdayTodayIncludesLeapDay(t *testing.T) {
	birth := time.Date(1992, time.February, 29, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)
	if !IsBirthdayToday(birth, today) {
		t.Fatal("29 February should be celebrated on 28 February in a non-leap year")
	}
	other := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	if IsBirthdayToday(birth, other) {
		t.Fatal("29 February should not also be 1 March")
	}
}

func TestBirthdayWishUsesName(t *testing.T) {
	wish := BirthdayWish("Ada")
	if wish == "" || !contains(wish, "Ada") {
		t.Fatalf("wish = %q", wish)
	}
}

func contains(value, part string) bool {
	return len(value) >= len(part) && (value == part || len(part) == 0 || (len(value) > 0 && (stringIndex(value, part) >= 0)))
}

func stringIndex(value, part string) int {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
