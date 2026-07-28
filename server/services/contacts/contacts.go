package contacts

import (
	"encoding/json"
	"fmt"
	"net/url"

	"sirtom/server/errs"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/services"
	"sirtom/server/utils"
)

// IsAccessError reports whether err means the user simply can't be searched for
// contacts — no Google account linked, or the contacts scope never granted. There's
// nothing to surface to the caller in that case: the suggestion list is just empty.
func IsAccessError(err *errs.GoogleAPIError) bool {
	if err == nil {
		return false
	}
	switch err.Code {
	case 401, 403:
		// Not authenticated for, or not permitted, the People API.
		return true
	case 400:
		// Our own "no (Google) calendar account linked" preconditions, and Google's
		// equivalent. Other 400s are malformed requests and stay errors.
		return err.Status == "FAILED_PRECONDITION"
	}
	return false
}

func SearchContacts(user *models.User, query string) ([]models.User, *errs.GoogleAPIError) {
	type Person struct {
		Names []struct {
			FamilyName string `json:"familyName"`
			GivenName  string `json:"givenName"`
		} `json:"names"`
		Photos []struct {
			Url string `json:"url"`
		} `json:"photos"`
		EmailAddresses []struct {
			Value string `json:"value"`
		} `json:"emailAddresses"`
	}

	// Set calendar auth to the user's primary google calendar account
	if user.CalendarAccounts == nil {
		return nil, &errs.GoogleAPIError{Code: 400, Message: "No calendar accounts linked", Status: "FAILED_PRECONDITION"}
	}
	key := utils.ActualCalendarAccountMapKey(user, user.Email, models.GoogleCalendarType)
	if key == "" {
		key = utils.GetCalendarAccountKey(user.Email, models.GoogleCalendarType)
	}
	account, ok := user.CalendarAccounts[key]
	if !ok || account.OAuth2CalendarAuth == nil {
		return nil, &errs.GoogleAPIError{Code: 400, Message: "No Google calendar account linked", Status: "FAILED_PRECONDITION"}
	}
	calendarAuth := account.OAuth2CalendarAuth

	// Search contacts
	response, err := services.CallApi(
		user,
		calendarAuth,
		"GET",
		fmt.Sprintf("https://people.googleapis.com/v1/people:searchContacts?query=%s&pageSize=10&readMask=names,emailAddresses,photos", url.QueryEscape(query)),
		nil,
	)
	if err != nil {
		logger.StdErr.Println(err)
		return nil, &errs.GoogleAPIError{Code: 500, Message: err.Error()}
	}
	defer func() { _ = response.Body.Close() }()

	// Parse response
	contactsData := struct {
		Results []struct {
			Person Person `json:"person"`
		} `json:"results"`
		Error *errs.GoogleAPIError `json:"error"`
	}{}
	if err := json.NewDecoder(response.Body).Decode(&contactsData); err != nil {
		logger.StdErr.Println(err)
		return nil, &errs.GoogleAPIError{Code: 500, Message: err.Error()}
	}

	directoryData := struct {
		People []Person             `json:"people"`
		Error  *errs.GoogleAPIError `json:"error"`
	}{}
	if len(query) > 0 {
		// Search Directory
		response, err = services.CallApi(
			user,
			calendarAuth,
			"GET",
			fmt.Sprintf("https://people.googleapis.com/v1/people:searchDirectoryPeople?query=%s&pageSize=10&readMask=names,emailAddresses,photos&sources=DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE", url.QueryEscape(query)),
			nil,
		)
		if err != nil {
			logger.StdErr.Println(err)
			return nil, &errs.GoogleAPIError{Code: 500, Message: err.Error()}
		}
		defer func() { _ = response.Body.Close() }()

		// Parse response
		if err := json.NewDecoder(response.Body).Decode(&directoryData); err != nil {
			logger.StdErr.Println(err)
			return nil, &errs.GoogleAPIError{Code: 500, Message: err.Error()}
		}
	}

	// Throw error if contacts access has not been granted
	if contactsData.Error != nil {
		return nil, contactsData.Error
	} else if directoryData.Error != nil && directoryData.Error.Code == 403 {
		// Need to check if code is 403 because error 400 occurs when user is not a GSuite user (which is okay and should not throw an error)
		return nil, directoryData.Error
	}

	// Format list of contacts search results
	contacts := make([]models.User, 0)
	for _, result := range contactsData.Results {
		var userProfile models.User
		userProfile.FirstName = result.Person.Names[0].GivenName
		userProfile.LastName = result.Person.Names[0].FamilyName
		if len(result.Person.Photos) > 0 {
			userProfile.Picture = result.Person.Photos[0].Url
		}

		for _, email := range result.Person.EmailAddresses {
			userProfile.Email = email.Value
			contacts = append(contacts, userProfile)
		}
	}
	for _, person := range directoryData.People {
		var userProfile models.User
		userProfile.FirstName = person.Names[0].GivenName
		userProfile.LastName = person.Names[0].FamilyName
		if len(person.Photos) > 0 {
			userProfile.Picture = person.Photos[0].Url
		}

		for _, email := range person.EmailAddresses {
			userProfile.Email = email.Value
			contacts = append(contacts, userProfile)
		}
	}

	return contacts, nil
}
