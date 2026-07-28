package calendar

import (
	"fmt"
	"time"

	"github.com/emersion/go-ical"
)

// Layouts for the DATE (no time component) form of an all-day event's
// DTSTART/DTEND. Google's API renders them as RFC 3339 dates; CalDAV and ICS
// feeds use the bare iCalendar form.
const (
	allDayLayoutRFC3339 = time.DateOnly // "2006-01-02" — Google Calendar
	allDayLayoutICal    = "20060102"    // Apple CalDAV, ICS feeds
)

// parseAllDayRange parses the DTSTART/DTEND pair of an all-day event.
//
// Either date failing fails the pair: a start without an end is not an event,
// and every caller skips the event rather than store half a range.
//
// All three callers used to discard these errors, so an unparseable date became
// year 0001 and the event travelled on to the client as a range no display
// window can contain. It was dropped either way — but silently, by accident,
// and only because the bogus date happened to fall outside every window.
func parseAllDayRange(layout, start, end string) (time.Time, time.Time, error) {
	startTime, err := time.Parse(layout, start)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("unable to parse all-day start date %q: %v", start, err)
	}

	endTime, err := time.Parse(layout, end)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("unable to parse all-day end date %q: %v", end, err)
	}

	return startTime, endTime, nil
}

// parseTimeWithTZ parses the DTSTART/DTEND of a timed event, honouring a TZID
// parameter when the value carries one. Shared by the Apple CalDAV and ICS
// feed readers.
func parseTimeWithTZ(prop *ical.Prop) (time.Time, error) {
	timeStr := prop.Value
	tzID := prop.Params.Get("TZID")

	var t time.Time
	var err error

	if tzID != "" {
		// locErr, not err: `loc, err :=` declared a SECOND err scoped to this
		// block, so the ParseInLocation failure below landed in the shadow and
		// the outer `if err != nil` read a nil. An unparseable time with a TZID
		// came back as the zero time with no error, and the suppression that
		// used to sit here asserted the opposite.
		loc, locErr := time.LoadLocation(tzID)
		if locErr != nil {
			return time.Time{}, fmt.Errorf("invalid timezone: %v", locErr)
		}
		t, err = time.ParseInLocation("20060102T150405", timeStr, loc)
	} else {
		t, err = time.Parse("20060102T150405Z", timeStr)
	}

	if err != nil {
		return time.Time{}, fmt.Errorf("unable to parse time: %v", err)
	}

	return t, nil
}
