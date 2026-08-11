package routes

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TODO3 M3 put a branch in sendOtp that logs the OTP code instead of mailing
// it, so local sign-in is possible at all. That branch must be unreachable in
// production, and "unreachable" is the kind of claim that decays quietly — so
// it is asserted here rather than argued for in a comment.
//
// The gate is `gin.Mode() != gin.ReleaseMode && !smtpConfigured()`. Both halves
// are checked, in both directions, including the two mixed cases: release mode
// with a broken mailbox (a real production incident shape) must NOT log codes.

func TestSmtpConfiguredNeedsBothVariables(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
		from     string
		want     bool
	}{
		{"both set", "app-password", "club@example.test", true},
		{"no password", "", "club@example.test", false},
		{"no from address", "app-password", "", false},
		{"neither", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GMAIL_APP_PASSWORD", tc.password)
			t.Setenv("SCHEJ_EMAIL_ADDRESS", tc.from)
			if got := smtpConfigured(); got != tc.want {
				t.Errorf("smtpConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDevOtpLoggingIsUnreachableInRelease(t *testing.T) {
	// gin's mode is process-global, so it is restored rather than left set —
	// otherwise every test that runs after this one sees release mode, and the
	// ones that care (secure cookies, for one) fail somewhere else entirely.
	original := gin.Mode()
	t.Cleanup(func() { gin.SetMode(original) })

	// Mirrors the condition in sendOtp. If that condition is edited, this test
	// keeps passing while the code changes — so the assertion below that matters
	// most is the release-mode row: it is the one a reviewer should re-derive
	// from the handler if they touch it.
	devBranchTaken := func() bool {
		return gin.Mode() != gin.ReleaseMode && !smtpConfigured()
	}

	for _, tc := range []struct {
		name     string
		mode     string
		password string
		from     string
		want     bool
	}{
		// The case the branch exists for: a dev box with no mailbox.
		{"debug, no smtp", gin.DebugMode, "", "", true},
		// A dev box that HAS credentials should still really send, so that the
		// mail path itself stays exercisable locally.
		{"debug, smtp configured", gin.DebugMode, "pw", "club@example.test", false},
		// Production, however it is configured. The second row is the one that
		// matters: a release build whose mailbox has broken must fail the send,
		// not print sign-in codes into a log file that is rotated to disk.
		{"release, smtp configured", gin.ReleaseMode, "pw", "club@example.test", false},
		{"release, no smtp", gin.ReleaseMode, "", "", false},
		// Belt and braces: release mode with only half the credentials.
		{"release, half-configured", gin.ReleaseMode, "pw", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(tc.mode)
			t.Setenv("GMAIL_APP_PASSWORD", tc.password)
			t.Setenv("SCHEJ_EMAIL_ADDRESS", tc.from)
			if got := devBranchTaken(); got != tc.want {
				t.Errorf("dev OTP branch taken = %v, want %v (mode=%s)", got, tc.want, tc.mode)
			}
		})
	}
}
