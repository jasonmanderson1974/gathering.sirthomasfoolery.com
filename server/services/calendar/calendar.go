package calendar

import (
	"fmt"
	"runtime/debug"
	"time"

	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/services/auth"
)

// recoveredError turns whatever came out of recover() into an error (J5).
//
// These helpers run as goroutines around three external calendar APIs plus a
// CalDAV library, and the recovery used to be a bare `err.(error)` type
// assertion. A provider panicking with anything that isn't an error — a string,
// an int, the runtime's own non-error values — made the assertion itself panic
// *inside the deferred function*, which loses the recovery entirely: an
// unrecovered panic in a goroutine takes down the whole process. So the one
// place meant to contain provider misbehaviour was the place that escalated it.
//
// The panic is logged with its stack here because the caller only ever sees it
// as a per-account error string on the calendar list — without this it would
// vanish with no way to find the provider that caused it.
func recoveredError(r interface{}) error {
	logger.StdErr.Printf("recovered panic in calendar provider: %v\n%s", r, debug.Stack())
	if err, ok := r.(error); ok {
		return err
	}
	return fmt.Errorf("calendar provider panicked: %v", r)
}

type GetCalendarListData struct {
	CalendarList       map[string]models.SubCalendar `json:"calendarList"`
	CalendarAccountKey string                        `json:"calendarAccountKey"`
	Error              error                         `json:"error"`
}

// Calls GetCalendarList but broadcasts the result to channel
func GetCalendarListAsync(calendarAccountKey string, calendarProvider *CalendarProvider, c chan GetCalendarListData) {
	// Recover from panics. The account key is set here too: the caller keys its
	// results map by it, so a panic that reported an empty key filed the error
	// against a phantom account and left the real one looking like it returned
	// no calendars at all.
	defer func() {
		if r := recover(); r != nil {
			c <- GetCalendarListData{CalendarAccountKey: calendarAccountKey, Error: recoveredError(r)}
		}
	}()

	calendarList, err := (*calendarProvider).GetCalendarList()

	c <- GetCalendarListData{CalendarList: calendarList, CalendarAccountKey: calendarAccountKey, Error: err}
}

type GetCalendarEventsData struct {
	CalendarEvents     []models.CalendarEvent `json:"calendarEvents"`
	CalendarAccountKey string                 `json:"calendarAccountKey"`
	Error              error                  `json:"error"`
}

// Get the user's list of calendar events for the given calendar
func GetCalendarEventsAsync(calendarAccountKey string, calendarProvider *CalendarProvider, calendarId string, timeMin time.Time, timeMax time.Time, c chan GetCalendarEventsData) {
	// Recover from panics (see GetCalendarListAsync on the account key).
	defer func() {
		if r := recover(); r != nil {
			c <- GetCalendarEventsData{CalendarAccountKey: calendarAccountKey, Error: recoveredError(r)}
		}
	}()

	calendarEvents, err := (*calendarProvider).GetCalendarEvents(calendarId, timeMin, timeMax)

	c <- GetCalendarEventsData{CalendarEvents: calendarEvents, CalendarAccountKey: calendarAccountKey, Error: err}
}

type CalendarEventsWithError struct {
	CalendarEvents []models.CalendarEvent `json:"calendarEvents"`
	Error          error                  `json:"error,omitempty"`
}

// Returns a map mapping email to the calendar events associated with that email, and an error if there was an error fetching events for that email
func GetUsersCalendarEvents(user *models.User, accounts models.Set[string], timeMin time.Time, timeMax time.Time) (map[string]CalendarEventsWithError, bool) {
	refreshFailures := auth.RefreshUserTokenIfNecessary(user, accounts)

	returnAllAccounts := len(accounts) == 0
	editedCalendarAccounts := false

	calendarEventsMap := make(map[string]CalendarEventsWithError)

	calendarListChan := make(chan GetCalendarListData)
	calendarEventsChan := make(chan GetCalendarEventsData)

	// Get calendar lists
	numCalendarListRequests := 0
	for calendarAccountKey, account := range user.CalendarAccounts {
		// Get secondary account calendars
		if _, ok := accounts[calendarAccountKey]; ok || returnAllAccounts {
			// This account's token is expired and could not be renewed, so the
			// fetch below is a guaranteed 401 whose message says nothing about
			// why. Report the refresh error in the slot the 401 would have
			// filled — the caller already surfaces a per-account error here
			// (that's what the branch further down exists for) — and skip the
			// round trip (H5).
			if err, failed := refreshFailures[calendarAccountKey]; failed {
				calendarEventsMap[calendarAccountKey] = CalendarEventsWithError{
					CalendarEvents: make([]models.CalendarEvent, 0),
					Error:          err,
				}
				continue
			}

			calendarProvider := GetCalendarProvider(account)
			go GetCalendarListAsync(calendarAccountKey, &calendarProvider, calendarListChan)
			numCalendarListRequests++

			calendarEventsMap[calendarAccountKey] = CalendarEventsWithError{
				CalendarEvents: make([]models.CalendarEvent, 0),
			}
		}
	}

	// After each calendar list is fetched, get the calendar events from each calendar
	numCalendarEventsRequests := 0
	for i := 0; i < numCalendarListRequests; i++ {
		calendarListData := <-calendarListChan

		if calendarListData.Error != nil {
			// This is needed to be able to send an error back to user if a given calendar account's refresh token is invalid, for example
			go func() { // needs to be async because writing to a channel is blocking
				calendarEventsChan <- GetCalendarEventsData{CalendarAccountKey: calendarListData.CalendarAccountKey, Error: calendarListData.Error}
			}()
			numCalendarEventsRequests++
			continue
		}

		// Edit subcalendars map
		account := user.CalendarAccounts[calendarListData.CalendarAccountKey]
		calendarProvider := GetCalendarProvider(account)
		if account.SubCalendars == nil {
			account.SubCalendars = &calendarListData.CalendarList
			user.CalendarAccounts[calendarListData.CalendarAccountKey] = account
			editedCalendarAccounts = true
		} else {
			// Add subCalendar if it doesn't exist
			for id, subCalendar := range calendarListData.CalendarList {
				if _, ok := (*account.SubCalendars)[id]; !ok {
					(*account.SubCalendars)[id] = subCalendar

					if !editedCalendarAccounts {
						editedCalendarAccounts = true
					}
				}
			}

			// Remove subCalendar if it no longer exists
			for id := range *account.SubCalendars {
				if _, ok := calendarListData.CalendarList[id]; !ok {
					delete(*account.SubCalendars, id)

					if !editedCalendarAccounts {
						editedCalendarAccounts = true
					}
				}
			}
		}
		user.CalendarAccounts[calendarListData.CalendarAccountKey] = account

		for id := range *account.SubCalendars {
			go GetCalendarEventsAsync(calendarListData.CalendarAccountKey, &calendarProvider, id, timeMin, timeMax, calendarEventsChan)
			numCalendarEventsRequests++
		}
	}

	// After calendar events are fetched, append to the calendarEvents array associated with the given email
	for i := 0; i < numCalendarEventsRequests; i++ {
		calendarEventsData := <-calendarEventsChan
		calendarAccountKey := calendarEventsData.CalendarAccountKey

		if _, ok := calendarEventsMap[calendarAccountKey]; !ok {
			calendarEventsMap[calendarAccountKey] = CalendarEventsWithError{}
		}

		if events, ok := calendarEventsMap[calendarAccountKey]; ok {
			if calendarEventsData.Error != nil {
				events.Error = calendarEventsData.Error
			} else {
				events.CalendarEvents = append(events.CalendarEvents, calendarEventsData.CalendarEvents...)
			}
			calendarEventsMap[calendarAccountKey] = events
		}
	}

	return calendarEventsMap, editedCalendarAccounts
}
