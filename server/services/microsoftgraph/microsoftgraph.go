package microsoftgraph

import (
	"encoding/json"

	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/services"
)

type UserInfo struct {
	FirstName string `json:"givenName"`
	LastName  string `json:"surname"`
	Email     string `json:"mail"`
}

func GetUserInfo(user *models.User, calendarAuth *models.OAuth2CalendarAuth) (UserInfo, error) {
	response, err := services.CallApi(
		user,
		calendarAuth,
		"GET",
		"https://graph.microsoft.com/v1.0/me?$select=givenName,surname,mail",
		nil,
	)
	if err != nil {
		return UserInfo{}, err
	}
	defer func() { _ = response.Body.Close() }()

	userResponse := struct {
		GivenName string `json:"givenName"`
		Surname   string `json:"surname"`
		Mail      string `json:"mail"`
	}{}

	if err := json.NewDecoder(response.Body).Decode(&userResponse); err != nil {
		logger.StdErr.Println(err)
		return UserInfo{}, err
	}

	return UserInfo{
		FirstName: userResponse.GivenName,
		LastName:  userResponse.Surname,
		Email:     userResponse.Mail,
	}, nil
}
