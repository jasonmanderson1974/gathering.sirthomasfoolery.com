package routes

import (
	"strings"
	"testing"
)

// These three bodies moved onto the shared layout helper. They interpolate only
// server-generated values, so the tests are mostly about the copy surviving the
// move — plus the escaping the shared helper now gives them for free.

func TestBuildOtpEmailBody(t *testing.T) {
	body := buildOtpEmailBody("482913")

	for _, want := range []string{
		"Your sign-in code",
		"482913",
		"It expires in ten minutes.",
		"check your spam folder",
		"The Fellowship",
		"<!DOCTYPE html>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("OTP email missing %q", want)
		}
	}
}

func TestBuildEmailChangeOtpBody(t *testing.T) {
	body := buildEmailChangeOtpBody("139248")

	for _, want := range []string{
		"Confirm your new address",
		"139248",
		"It expires in ten minutes.",
		"If you did not request this change",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("email-change email missing %q", want)
		}
	}
	// The two code emails should not be confusable with each other.
	if strings.Contains(body, "Your sign-in code") {
		t.Error("email-change email should not carry the sign-in heading")
	}
}

func TestBuildInvitationEmailBody(t *testing.T) {
	const url = "https://gathering.example.test/sign-in"
	body := buildInvitationEmailBody(url)

	for _, want := range []string{
		"You are invited",
		"take your place among The Fellowship",
		"this email address", // inline emphasis must survive
		"Enter the Gathering",
		url,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("invitation email missing %q", want)
		}
	}
	if !strings.Contains(body, "<strong") {
		t.Error("invitation email lost its inline emphasis")
	}
	if !strings.Contains(body, `href="`+url+`"`) {
		t.Error("invitation email lost its sign-in link")
	}
}

// A code is server-generated, but the helper escapes regardless — a code that
// somehow contained markup must not become markup.
func TestOtpCodeIsEscaped(t *testing.T) {
	body := buildOtpEmailBody(`<script>alert(1)</script>`)
	if strings.Contains(body, "<script>") {
		t.Errorf("code was not escaped:\n%s", body)
	}
}
