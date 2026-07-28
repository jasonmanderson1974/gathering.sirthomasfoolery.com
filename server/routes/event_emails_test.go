package routes

import (
	"strings"
	"testing"

	"sirtom/server/models"
	"sirtom/server/utils"
)

const testEventURL = "https://example.test/e/abc123"

func TestBuildSomeoneRespondedEmail(t *testing.T) {
	subject, body := buildSomeoneRespondedEmail("Jason", "Greg Wallace", "Poker Night", testEventURL)

	if subject != "Greg Wallace responded to Poker Night" {
		t.Errorf("subject = %q", subject)
	}
	for _, want := range []string{"Jason", "Greg Wallace", "Poker Night", testEventURL, "View the Gathering"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestBuildXResponsesEmail(t *testing.T) {
	subject, body := buildXResponsesEmail("Jason", "Poker Night", testEventURL, 3)

	if subject != "3 people have responded to Poker Night" {
		t.Errorf("subject = %q", subject)
	}
	if !strings.Contains(body, "3 people have responded") {
		t.Errorf("body missing the tally: %s", body)
	}
	if !strings.Contains(body, testEventURL) {
		t.Error("body missing the event URL")
	}
}

func TestBuildXResponsesEmailSingular(t *testing.T) {
	subject, _ := buildXResponsesEmail("Jason", "Poker Night", testEventURL, 1)
	if subject != "1 person has responded to Poker Night" {
		t.Errorf("subject = %q, want singular phrasing", subject)
	}
}

func TestBuildEveryoneRespondedEmail(t *testing.T) {
	subject, body := buildEveryoneRespondedEmail("Jason", "Poker Night", testEventURL)

	if subject != "Everyone has responded to Poker Night" {
		t.Errorf("subject = %q", subject)
	}
	for _, want := range []string{"Jason", "Poker Night", testEventURL, "Settle the hour"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

// An event name is user-controlled and reaches all three bodies, so it must not
// be able to carry markup into the email.
func TestNotificationEmailsEscapeEventName(t *testing.T) {
	const evil = `<img src=x onerror=alert(1)>`

	_, someone := buildSomeoneRespondedEmail("Jason", "Greg", evil, testEventURL)
	_, tally := buildXResponsesEmail("Jason", evil, testEventURL, 2)
	_, everyone := buildEveryoneRespondedEmail("Jason", evil, testEventURL)

	for name, body := range map[string]string{
		"someone responded": someone,
		"x responses":       tally,
		"everyone":          everyone,
	} {
		if strings.Contains(body, "<img") {
			t.Errorf("%s: event name was not escaped", name)
		}
		if !strings.Contains(body, "&lt;img") {
			t.Errorf("%s: expected escaped event name", name)
		}
	}
}

func TestNotificationEmailsEscapeRespondentName(t *testing.T) {
	_, body := buildSomeoneRespondedEmail("Jason", `<b>Greg</b>`, "Poker Night", testEventURL)
	if strings.Contains(body, "<b>Greg</b>") {
		t.Errorf("respondent name was not escaped: %s", body)
	}
}

func TestRemindeeResponded(t *testing.T) {
	tests := []struct {
		name string
		in   models.Remindee
		want bool
	}{
		{"missing flag treated as not responded", models.Remindee{Email: "a@example.test"}, false},
		{"explicit false", models.Remindee{Email: "a@example.test", Responded: utils.FalsePtr()}, false},
		{"explicit true", models.Remindee{Email: "a@example.test", Responded: utils.TruePtr()}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := remindeeResponded(tt.in); got != tt.want {
				t.Errorf("remindeeResponded() = %v, want %v", got, tt.want)
			}
		})
	}
}
