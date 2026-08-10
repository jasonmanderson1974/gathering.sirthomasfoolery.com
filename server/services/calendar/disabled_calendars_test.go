package calendar

import (
	"testing"

	"sirtom/server/utils"
)

// J8: events were fetched for every sub-calendar and every account, including
// the ones the member had switched off — a live Google/Microsoft/CalDAV round
// trip each, whose result the client then discarded.
//
// The rule this pins is the nil case, which is the only one that can cause harm.
// Both flags are *bool; nil means "never toggled" and appears on legacy rows.
// Treating nil as disabled would silently drop real events out of someone's
// availability, so nil must stay enabled: a skip is only safe when the client
// would certainly have thrown the result away.
func TestIsDisabled(t *testing.T) {
	cases := []struct {
		name string
		flag *bool
		want bool
	}{
		{"explicitly off — the case worth skipping", utils.FalsePtr(), true},
		{"explicitly on", utils.TruePtr(), false},
		{"never toggled (legacy row) — fail open, fetch it", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDisabled(tc.flag); got != tc.want {
				t.Errorf("isDisabled() = %v, want %v", got, tc.want)
			}
		})
	}
}
