package calendar

import (
	"errors"
	"strings"
	"testing"
	"time"

	"sirtom/server/models"
)

// panickingProvider panics with whatever it is given, standing in for a
// provider or the CalDAV library blowing up mid-request.
type panickingProvider struct{ value interface{} }

func (p *panickingProvider) GetCalendarList() (map[string]models.SubCalendar, error) {
	panic(p.value)
}

func (p *panickingProvider) GetCalendarEvents(string, time.Time, time.Time) ([]models.CalendarEvent, error) {
	panic(p.value)
}

// J5: the recovery used to be a bare `err.(error)` type assertion, so a panic
// carrying anything but an error made the assertion panic inside the deferred
// function — losing the recovery and taking the process down with the
// unrecovered goroutine panic.
//
// A panic escaping these helpers would crash the test binary outright, so the
// test failing to complete IS the regression signal; the assertions below just
// pin down what the caller receives.
func TestGetCalendarListAsync_RecoversNonErrorPanic(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value interface{}
		want  string
	}{
		{"string", "boom", "boom"},
		{"int", 42, "42"},
		{"error", errors.New("a real error"), "a real error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var provider CalendarProvider = &panickingProvider{value: tc.value}
			c := make(chan GetCalendarListData, 1)

			go GetCalendarListAsync("acct-key", &provider, c)

			select {
			case got := <-c:
				if got.Error == nil {
					t.Fatal("panic was swallowed without producing an error")
				}
				if !strings.Contains(got.Error.Error(), tc.want) {
					t.Errorf("error %q does not mention the panic value %q", got.Error, tc.want)
				}
				// The caller keys its results map by this; an empty key files
				// the error against a phantom account.
				if got.CalendarAccountKey != "acct-key" {
					t.Errorf("CalendarAccountKey = %q, want %q", got.CalendarAccountKey, "acct-key")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no result on the channel — the recovery never sent, so the caller would block forever")
			}
		})
	}
}

func TestGetCalendarEventsAsync_RecoversNonErrorPanic(t *testing.T) {
	var provider CalendarProvider = &panickingProvider{value: "events boom"}
	c := make(chan GetCalendarEventsData, 1)

	go GetCalendarEventsAsync("acct-key", &provider, "cal-id", time.Now(), time.Now().Add(time.Hour), c)

	select {
	case got := <-c:
		if got.Error == nil {
			t.Fatal("panic was swallowed without producing an error")
		}
		if !strings.Contains(got.Error.Error(), "events boom") {
			t.Errorf("error %q does not mention the panic value", got.Error)
		}
		if got.CalendarAccountKey != "acct-key" {
			t.Errorf("CalendarAccountKey = %q, want %q", got.CalendarAccountKey, "acct-key")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no result on the channel — the recovery never sent, so the caller would block forever")
	}
}
