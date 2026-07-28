package calendar

import (
	"testing"

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
