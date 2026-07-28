package routes

import "strings"

// Text helpers shared by the handlers. Truncation lives here rather than beside
// any one feature because four callers across four files need it — event names
// and descriptions, responder display names, comments, and poll titles/options.

// truncateRunes cuts s to at most max RUNES, not bytes.
//
// Byte slicing is the tempting version and it is wrong: a cut landing inside a
// multi-byte character leaves an invalid UTF-8 tail that renders as a
// replacement char (U+FFFD). Club comments carry emoji and accented names, so
// this is reachable, not theoretical.
//
// Note the caps that use this therefore bound CHARACTERS, which is what a user
// means by "2,000 characters" — and, being at most 4 bytes each, still bounds
// storage to a predictable multiple.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	// Cheap path: an all-ASCII string under the cap can't need cutting, and
	// this avoids allocating a rune slice for the common case.
	if len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// trimAndTruncate trims surrounding whitespace, then bounds the result to max
// runes — the order every caller wants, so that padding never eats the budget.
func trimAndTruncate(s string, max int) string {
	return truncateRunes(strings.TrimSpace(s), max)
}
