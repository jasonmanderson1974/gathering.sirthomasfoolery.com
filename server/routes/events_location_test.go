package routes

import "testing"

func TestTrimmedLocation(t *testing.T) {
	str := func(s string) *string { return &s }

	t.Run("absent field stays absent", func(t *testing.T) {
		// nil has to survive: scheduleEvent uses it to mean "leave the venue
		// alone", which is not the same as clearing it
		if got := trimmedLocation(nil); got != nil {
			t.Errorf("trimmedLocation(nil) = %q, want nil", *got)
		}
	})

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"surrounding spaces", "  The Fox & Hound  ", "The Fox & Hound"},
		{"trailing newline", "The Fox & Hound\n", "The Fox & Hound"},
		{"tabs", "\tGreg's back garden\t", "Greg's back garden"},
		{"already clean", "The Fox & Hound", "The Fox & Hound"},
		{"inner spacing preserved", "  The  Fox  &  Hound  ", "The  Fox  &  Hound"},
		{"whitespace only becomes empty", "   ", ""},
		{"empty stays empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimmedLocation(str(tt.in))
			if got == nil {
				t.Fatalf("trimmedLocation(%q) = nil, want %q", tt.in, tt.want)
			}
			if *got != tt.want {
				t.Errorf("trimmedLocation(%q) = %q, want %q", tt.in, *got, tt.want)
			}
		})
	}

	t.Run("does not mutate the caller's string", func(t *testing.T) {
		original := "  The Fox & Hound  "
		in := original
		trimmedLocation(&in)
		if in != original {
			t.Errorf("input mutated: got %q, want %q", in, original)
		}
	})
}
