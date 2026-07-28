package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

// parseTimeWithTZ used to declare a second `err` inside the TZID branch
// (`loc, err := ...`), so the ParseInLocation failure landed in the shadow and
// the outer check read a nil. An unparseable time came back as the zero time
// with no error, and the caller had no way to tell.
func TestParseTimeWithTZRejectsAnUnparseableValue(t *testing.T) {
	prop := &ical.Prop{Value: "not-a-timestamp", Params: ical.Params{}}
	prop.Params.Set("TZID", "America/Los_Angeles")

	got, err := parseTimeWithTZ(prop)
	if err == nil {
		t.Fatalf("expected an error for an unparseable time, got %v", got)
	}
	if !got.IsZero() {
		t.Errorf("expected the zero time alongside the error, got %v", got)
	}
}

func TestParseTimeWithTZParsesAZonedValue(t *testing.T) {
	prop := &ical.Prop{Value: "20260731T190000", Params: ical.Params{}}
	prop.Params.Set("TZID", "America/Los_Angeles")

	got, err := parseTimeWithTZ(prop)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Hour() != 19 || got.Day() != 31 {
		t.Errorf("parsed to %v, want 2026-07-31 19:00 local", got)
	}
	if zone, _ := got.Zone(); zone != "PDT" {
		t.Errorf("zone = %q, want the TZID applied", zone)
	}
}

func TestParseTimeWithTZRejectsAnInvalidZone(t *testing.T) {
	prop := &ical.Prop{Value: "20260731T190000", Params: ical.Params{}}
	prop.Params.Set("TZID", "Not/AZone")

	if _, err := parseTimeWithTZ(prop); err == nil {
		t.Error("expected an error for an unknown timezone")
	}
}

// The no-TZID branch never shadowed, but it shares the error path.
func TestParseTimeWithTZUTCBranch(t *testing.T) {
	prop := &ical.Prop{Value: "20260731T190000Z", Params: ical.Params{}}
	if _, err := parseTimeWithTZ(prop); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := &ical.Prop{Value: "nonsense", Params: ical.Params{}}
	if _, err := parseTimeWithTZ(bad); err == nil {
		t.Error("expected an error for an unparseable UTC time")
	}
}

// The load-bearing one: every all-day caller used to discard the parse error,
// so a malformed date yielded year 0001 and no error at all. These fail against
// `startTime, _ = time.Parse(...)`.
func TestParseAllDayRangeRejectsUnparseableDates(t *testing.T) {
	cases := []struct {
		name         string
		layout       string
		start, end   string
		wantErrOnEnd bool
	}{
		{"ical start is garbage", allDayLayoutICal, "not-a-date", "20260801", false},
		{"ical end is garbage", allDayLayoutICal, "20260731", "not-a-date", true},
		{"ical start is empty", allDayLayoutICal, "", "20260801", false},
		{"ical end is empty", allDayLayoutICal, "20260731", "", true},
		// The layouts are not interchangeable — a Google date fed to the iCal
		// layout must fail rather than parse to something plausible.
		{"rfc3339 value under the ical layout", allDayLayoutICal, "2026-07-31", "2026-08-01", false},
		{"ical value under the rfc3339 layout", allDayLayoutRFC3339, "20260731", "20260801", false},
		{"rfc3339 start is garbage", allDayLayoutRFC3339, "31/07/2026", "2026-08-01", false},
		{"rfc3339 end is garbage", allDayLayoutRFC3339, "2026-07-31", "31/08/2026", true},
		// A timed value carries a T — the callers route those elsewhere, but the
		// helper must not quietly accept one.
		{"timed value", allDayLayoutICal, "20260731T190000", "20260801T190000", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := parseAllDayRange(tc.layout, tc.start, tc.end)
			if err == nil {
				t.Fatalf("expected an error, got %v .. %v", start, end)
			}
			// Both must come back zero, so a caller that ignores the error
			// can't half-use the range.
			if !start.IsZero() || !end.IsZero() {
				t.Errorf("expected both zero alongside the error, got %v .. %v", start, end)
			}
			// The message must name the offending value; these are logged and
			// then the event vanishes, so the log is the only trace.
			bad := tc.start
			if tc.wantErrOnEnd {
				bad = tc.end
			}
			if !strings.Contains(err.Error(), bad) {
				t.Errorf("error %q does not quote the offending value %q", err, bad)
			}
		})
	}
}

func TestParseAllDayRangeParsesBothLayouts(t *testing.T) {
	cases := []struct {
		name       string
		layout     string
		start, end string
	}{
		{"ical", allDayLayoutICal, "20260731", "20260801"},
		{"rfc3339", allDayLayoutRFC3339, "2026-07-31", "2026-08-01"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := parseAllDayRange(tc.layout, tc.start, tc.end)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			wantStart := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)
			wantEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
			if !start.Equal(wantStart) {
				t.Errorf("start = %v, want %v", start, wantStart)
			}
			if !end.Equal(wantEnd) {
				t.Errorf("end = %v, want %v", end, wantEnd)
			}
		})
	}
}
