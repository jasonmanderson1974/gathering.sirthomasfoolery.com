package routes

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A17: comments and poll titles/options were truncated by BYTE slicing. A cut
// landing inside a multi-byte character leaves an invalid UTF-8 tail that
// renders as U+FFFD. Club comments carry emoji and accented names, so these
// cases are reachable rather than theoretical.
//
// The assertions that matter are utf8.ValidString — a byte-slicing
// implementation fails those, which is what makes these tests worth having.

// "🥃" is 4 bytes; "é" (U+00E9) is 2. Both cut badly on a byte boundary.
const (
	whisky = "🥃"
	eacute = "é"
)

func TestTruncateRunes_UnderCapPassesThrough(t *testing.T) {
	for _, s := range []string{"", "hello", whisky + whisky, "café"} {
		if got := truncateRunes(s, 10); got != s {
			t.Errorf("truncateRunes(%q, 10) = %q, want it unchanged", s, got)
		}
	}
}

func TestTruncateRunes_CountsRunesNotBytes(t *testing.T) {
	// 5 glasses = 20 bytes. Cutting to 3 must yield 3 glasses, not 3 bytes.
	got := truncateRunes(strings.Repeat(whisky, 5), 3)
	if got != strings.Repeat(whisky, 3) {
		t.Errorf("got %q, want 3 glasses", got)
	}
	if n := utf8.RuneCountInString(got); n != 3 {
		t.Errorf("rune count = %d, want 3", n)
	}
}

// The core A17 assertion: never leave a partial character behind. Sweep every
// cap from 0 up past the string length so a boundary can't be missed.
func TestTruncateRunes_NeverProducesInvalidUTF8(t *testing.T) {
	inputs := []string{
		strings.Repeat(whisky, 8),
		strings.Repeat(eacute, 8),
		"café " + whisky + " nightcap",
		"Seán O'Súilleabháin " + whisky,
	}
	for _, in := range inputs {
		for cap := 0; cap <= utf8.RuneCountInString(in)+2; cap++ {
			got := truncateRunes(in, cap)
			if !utf8.ValidString(got) {
				t.Fatalf("truncateRunes(%q, %d) = %q — invalid UTF-8", in, cap, got)
			}
			if strings.ContainsRune(got, utf8.RuneError) &&
				!strings.ContainsRune(in, utf8.RuneError) {
				t.Fatalf("truncateRunes(%q, %d) introduced a replacement char", in, cap)
			}
			if n := utf8.RuneCountInString(got); n > cap {
				t.Fatalf("truncateRunes(%q, %d) returned %d runes", in, cap, n)
			}
		}
	}
}

func TestTruncateRunes_NonPositiveCap(t *testing.T) {
	if got := truncateRunes("anything", 0); got != "" {
		t.Errorf("cap 0: got %q, want empty", got)
	}
	if got := truncateRunes("anything", -1); got != "" {
		t.Errorf("negative cap: got %q, want empty", got)
	}
}

func TestTrimAndTruncate_TrimsBeforeBounding(t *testing.T) {
	// Padding must not eat the budget: 5 chars of content inside whitespace,
	// bounded to 5, should survive whole.
	if got := trimAndTruncate("   hello   ", 5); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if got := trimAndTruncate("   \t\n  ", 10); got != "" {
		t.Errorf("whitespace-only: got %q, want empty", got)
	}
}

// The three A17 call sites, through their real entry points.

func TestSanitizeCommentText_RuneSafe(t *testing.T) {
	long := strings.Repeat(whisky, maxCommentLength+10)
	got, ok := sanitizeCommentText(long)
	if !ok {
		t.Fatal("a long comment should still be usable")
	}
	if !utf8.ValidString(got) {
		t.Error("comment truncation produced invalid UTF-8")
	}
	if n := utf8.RuneCountInString(got); n != maxCommentLength {
		t.Errorf("rune count = %d, want %d", n, maxCommentLength)
	}

	// Trimming behaviour must survive the rewrite.
	if got, ok := sanitizeCommentText("   "); ok || got != "" {
		t.Errorf("whitespace-only comment: got (%q, %v), want (\"\", false)", got, ok)
	}
	if got, _ := sanitizeCommentText("  bring cigars  "); got != "bring cigars" {
		t.Errorf("got %q, want it trimmed", got)
	}
}

func TestSanitizePollInput_RuneSafe(t *testing.T) {
	title := strings.Repeat(whisky, maxPollTitleLength+10)
	option := strings.Repeat(eacute, maxPollOptionLength+10)

	gotTitle, gotOptions, ok := sanitizePollInput(title, []string{option, "The Lodge"})
	if !ok {
		t.Fatal("poll should be usable")
	}
	if !utf8.ValidString(gotTitle) {
		t.Error("poll title truncation produced invalid UTF-8")
	}
	if n := utf8.RuneCountInString(gotTitle); n != maxPollTitleLength {
		t.Errorf("title rune count = %d, want %d", n, maxPollTitleLength)
	}
	if !utf8.ValidString(gotOptions[0]) {
		t.Error("poll option truncation produced invalid UTF-8")
	}
	if n := utf8.RuneCountInString(gotOptions[0]); n != maxPollOptionLength {
		t.Errorf("option rune count = %d, want %d", n, maxPollOptionLength)
	}
}

// Two options that differ only past the cap collapse to one after truncation,
// and the existing dedupe must then reject the poll as having fewer than two
// distinct choices — rather than storing a pair of identical labels.
func TestSanitizePollInput_TruncationCanCollapseOptions(t *testing.T) {
	prefix := strings.Repeat(whisky, maxPollOptionLength)
	_, _, ok := sanitizePollInput("Where?", []string{prefix + "A", prefix + "B"})
	if ok {
		t.Error("options identical after truncation should not count as two distinct choices")
	}
}

func TestSanitizeResponderName_RuneSafe(t *testing.T) {
	got := sanitizeResponderName("  " + strings.Repeat(whisky, maxResponderNameLength+10) + "  ")
	if !utf8.ValidString(got) {
		t.Error("responder name truncation produced invalid UTF-8")
	}
	if n := utf8.RuneCountInString(got); n != maxResponderNameLength {
		t.Errorf("rune count = %d, want %d", n, maxResponderNameLength)
	}
}
