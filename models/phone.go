package models

import (
	"regexp"
	"strings"
)

var nonDigitRE = regexp.MustCompile(`\D+`)

// NormalizePhoneE164 converts common SA local numbers (e.g. 0792531102)
// into E.164 for AWS SNS. Numbers already in E.164 are returned cleaned.
// defaultCountryCode should be digits only without "+", e.g. "27".
func NormalizePhoneE164(phone, defaultCountryCode string) string {
	raw := strings.TrimSpace(phone)
	if raw == "" {
		return ""
	}

	hasPlus := strings.HasPrefix(raw, "+")
	digits := nonDigitRE.ReplaceAllString(raw, "")
	if digits == "" {
		return ""
	}

	cc := nonDigitRE.ReplaceAllString(strings.TrimSpace(defaultCountryCode), "")
	if cc == "" {
		cc = "27"
	}

	// Already international without plus: 27792531102
	if strings.HasPrefix(digits, cc) && len(digits) >= len(cc)+8 {
		return "+" + digits
	}

	// Local SA-style leading zero: 0792531102 -> +27792531102
	if strings.HasPrefix(digits, "0") && len(digits) >= 9 {
		return "+" + cc + strings.TrimPrefix(digits, "0")
	}

	if hasPlus {
		return "+" + digits
	}

	// Bare national number without leading 0 (e.g. 792531102)
	if len(digits) >= 9 && len(digits) <= 10 {
		return "+" + cc + digits
	}

	return "+" + digits
}
