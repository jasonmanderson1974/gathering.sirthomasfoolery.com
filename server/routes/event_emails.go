package routes

import (
	"fmt"

	"sirtom/server/models"
	"sirtom/server/utils"
)

// Notification emails sent to a gathering's owner as responses come in.
//
// These used to be Listmonk transactional templates (ids 10, 14 and 8), which
// meant the copy lived outside the repo and nothing was sent at all unless a
// Listmonk instance was configured. They are plain Gmail SMTP sends now, in the
// same shell as the OTP, invitation and pre-gathering reminder emails.
//
// The builders are pure so they can be tested without a mail server.

// eventURLFor returns the public URL of an event.
func eventURLFor(event *models.Event) string {
	return fmt.Sprintf("%s/e/%s", utils.GetBaseUrl(), event.GetId())
}

// buildSomeoneRespondedEmail tells the owner a new person has filled in their
// availability. Sent per response, when "Email me each time someone joins my
// event" is on.
func buildSomeoneRespondedEmail(ownerName, respondentName, eventName, eventURL string) (subject, body string) {
	subject = fmt.Sprintf("%s responded to %s", respondentName, eventName)

	body = utils.RenderEmailWithFooter(
		"A reply has arrived",
		utils.EmailParagraph(fmt.Sprintf("%s, another man has marked his availability.", ownerName))+
			utils.EmailStrongLine(eventName)+
			utils.EmailAccentLine(respondentName+" has responded"),
		utils.EmailFooterURL(eventURL),
		utils.EmailAction{Label: "View the Gathering", URL: eventURL},
	)
	return subject, body
}

// buildXResponsesEmail tells the owner their chosen response threshold has been
// reached. Fires once — the handler disarms the setting before sending.
func buildXResponsesEmail(ownerName, eventName, eventURL string, numResponses int) (subject, body string) {
	people := "people have"
	if numResponses == 1 {
		people = "person has"
	}
	subject = fmt.Sprintf("%d %s responded to %s", numResponses, people, eventName)

	body = utils.RenderEmailWithFooter(
		"The replies are in",
		utils.EmailParagraph(fmt.Sprintf("%s, the Gathering has reached the tally you asked to be told of.", ownerName))+
			utils.EmailStrongLine(eventName)+
			utils.EmailAccentLine(fmt.Sprintf("%d %s responded", numResponses, people)),
		utils.EmailFooterURL(eventURL),
		utils.EmailAction{Label: "View the Gathering", URL: eventURL},
	)
	return subject, body
}

// buildEveryoneRespondedEmail tells the owner every remindee has now answered.
func buildEveryoneRespondedEmail(ownerName, eventName, eventURL string) (subject, body string) {
	subject = fmt.Sprintf("Everyone has responded to %s", eventName)

	body = utils.RenderEmailWithFooter(
		"The whole Order has answered",
		utils.EmailParagraph(fmt.Sprintf("%s, every man you summoned has marked his availability. You may now settle upon an hour.", ownerName))+
			utils.EmailStrongLine(eventName),
		utils.EmailFooterURL(eventURL),
		utils.EmailAction{Label: "Settle the hour", URL: eventURL},
	)
	return subject, body
}

// remindeeResponded reports whether a remindee has answered, treating a missing
// flag as "not yet". Imported events (event_import.go) and older documents can
// both lack it, so this must never dereference blindly.
func remindeeResponded(r models.Remindee) bool {
	return r.Responded != nil && *r.Responded
}
