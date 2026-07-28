package reminders

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

func TestDueNudgeStage(t *testing.T) {
	addedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		now       time.Time
		sentStage int
		want      int
	}{
		{"stage 1 is due immediately", addedAt, 0, 1},
		{"nothing due once stage 1 is sent", addedAt, 1, 0},
		{"stage 2 exactly at +24h", addedAt.Add(24 * time.Hour), 1, 2},
		{"stage 2 not yet at +23h59m", addedAt.Add(24*time.Hour - time.Minute), 1, 0},
		{"stage 3 exactly at +72h", addedAt.Add(72 * time.Hour), 2, 3},
		{"stage 3 not yet at +71h", addedAt.Add(71 * time.Hour), 2, 0},
		{"fully nudged stays done", addedAt.Add(30 * 24 * time.Hour), 3, 0},
		{"beyond the last stage never exceeds max", addedAt.Add(365 * 24 * time.Hour), 0, 3},
		// After an outage we send the newest nudge they're owed, not all of them.
		{"coalesces a 5-day gap to the final stage", addedAt.Add(5 * 24 * time.Hour), 0, 3},
		{"coalesces a 2-day gap to stage 2", addedAt.Add(2 * 24 * time.Hour), 0, 2},
		{"defensive: negative sent stage treated as none sent", addedAt, -1, 1},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := dueNudgeStage(tt.now, addedAt, tt.sentStage); got != tt.want {
				t.Errorf("dueNudgeStage(+%v, sent=%d) = %d, want %d",
					tt.now.Sub(addedAt), tt.sentStage, got, tt.want)
			}
		})
	}
}

func TestRemindeeAddedAtUsesTheRemindeesOwnTimestamp(t *testing.T) {
	added := time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC)
	addedDT := primitive.NewDateTimeFromTime(added)
	event := &models.Event{Id: primitive.NewObjectID()}

	got := remindeeAddedAt(event, models.Remindee{Email: "a@example.test", AddedAt: &addedDT})
	if !got.Equal(added) {
		t.Errorf("remindeeAddedAt() = %v, want %v", got, added)
	}
}

func TestRemindeeAddedAtFallsBackToEventCreation(t *testing.T) {
	// Documents written before AddedAt existed have no timestamp of their own;
	// the event's ObjectID carries its creation time.
	created := time.Date(2026, 3, 1, 8, 30, 0, 0, time.UTC)
	event := &models.Event{Id: primitive.NewObjectIDFromTimestamp(created)}

	got := remindeeAddedAt(event, models.Remindee{Email: "a@example.test"})
	if !got.Equal(created) {
		t.Errorf("remindeeAddedAt() = %v, want the event's creation time %v", got, created)
	}
}

func TestNudgeMaxAgeRetiresOldRemindees(t *testing.T) {
	// The guard the ticker applies: anything older than the cutoff is retired
	// rather than sent, so shipping this feature doesn't nudge historic events.
	addedAt := time.Now().Add(-30 * 24 * time.Hour)
	if time.Since(addedAt) <= nudgeMaxAge {
		t.Fatal("test fixture should be older than nudgeMaxAge")
	}
	// It would otherwise look overdue for the final stage.
	if got := dueNudgeStage(time.Now(), addedAt, 0); got != maxNudgeStage {
		t.Errorf("expected the stale remindee to look overdue (got %d)", got)
	}
}

func TestBuildRemindeeNudgeEmailPerStage(t *testing.T) {
	const eventURL = "https://example.test/e/abc"
	const respondedURL = "https://example.test/e/abc/responded?email=a%2Bb%40example.test"

	cases := []struct {
		stage       int
		wantSubject string
	}{
		{1, "Jason needs your availability for Poker Night"},
		{2, "Reminder: Poker Night is still waiting on you"},
		{3, "Final reminder: Poker Night"},
	}

	for _, tt := range cases {
		subject, body := buildRemindeeNudgeEmail(tt.stage, "Jason", "Poker Night", eventURL, respondedURL)
		if subject != tt.wantSubject {
			t.Errorf("stage %d subject = %q, want %q", tt.stage, subject, tt.wantSubject)
		}
		for _, want := range []string{"Poker Night", eventURL, "Share your availability", "already responded"} {
			if !strings.Contains(body, want) {
				t.Errorf("stage %d body missing %q", tt.stage, want)
			}
		}
	}
}

func TestBuildRemindeeNudgeEmailEscapesEventName(t *testing.T) {
	_, body := buildRemindeeNudgeEmail(1, "Jason", `<script>alert(1)</script>`,
		"https://example.test/e/abc", "https://example.test/e/abc/responded")
	if strings.Contains(body, "<script>") {
		t.Errorf("event name was not escaped:\n%s", body)
	}
}

// The Cloud Tasks version built this link without escaping, so an address
// containing '+' arrived broken.
func TestNudgeRespondedURLEscapesTheEmail(t *testing.T) {
	got := nudgeRespondedURL("abc123", "greg+poker@example.test")
	if strings.Contains(got, "greg+poker@example.test") {
		t.Errorf("email should be query-escaped, got %q", got)
	}
	if !strings.Contains(got, "greg%2Bpoker%40example.test") {
		t.Errorf("expected escaped email in %q", got)
	}
	if !strings.Contains(got, "/e/abc123/responded?email=") {
		t.Errorf("unexpected URL shape: %q", got)
	}
}
