package models

import "testing"

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name string
		user User
		want string
	}{
		{"nickname wins", User{FirstName: "Bartholomew", LastName: "Fitzwilliam", Nickname: "Bart"}, "Bart"},
		{"falls back to full name", User{FirstName: "Ada", LastName: "Lovelace"}, "Ada Lovelace"},
		{"first name only", User{FirstName: "Ada"}, "Ada"},
		{"last name only", User{LastName: "Lovelace"}, "Lovelace"},
		{"nothing at all", User{}, ""},
		// A nickname of pure whitespace is not a nickname. The handler trims
		// before storing, but legacy documents and other write paths (F6's
		// admin edit) reach the model directly.
		{"whitespace nickname ignored", User{FirstName: "Ada", LastName: "Lovelace", Nickname: "   "}, "Ada Lovelace"},
		{"nickname trimmed", User{Nickname: "  Bart  "}, "Bart"},
		// Concatenating raw would leave a stray space at one end or the other.
		{"padded names collapse", User{FirstName: " Ada ", LastName: " Lovelace "}, "Ada Lovelace"},
		{"blank last name leaves no trailing space", User{FirstName: "Ada", LastName: "  "}, "Ada"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.user.DisplayName(); got != tc.want {
				t.Errorf("DisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}
