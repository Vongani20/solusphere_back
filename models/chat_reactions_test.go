package models

import "testing"

func TestValidReactionEmoji(t *testing.T) {
	for _, ok := range []string{"👍", "❤️", "😂", "🙏🏽", "👨‍👩‍👧", "🎉", "✅"} {
		if !ValidReactionEmoji(ok) {
			t.Errorf("expected %q to be valid", ok)
		}
	}
	for _, bad := range []string{"", "a", "lol", "1", "👍 ", "<script>", "👍a", "😀😀😀😀😀😀😀😀😀😀😀"} {
		if ValidReactionEmoji(bad) {
			t.Errorf("expected %q to be invalid", bad)
		}
	}
}
