package routes

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// F3: comment.authorName, rsvp.name and the poll vote roster are all snapshots
// taken at write time. These pin that getEvent re-resolves them against the
// account as it is NOW, so setting a nickname reaches everything already
// written — and that the fallbacks (deleted account, legacy guest row) survive.

// seedNicknameEvent inserts a member with a nickname who has a comment, an
// RSVP and a poll vote, each carrying the pre-nickname name snapshot, plus a
// viewer to fetch as.
func seedNicknameEvent(t *testing.T) (eventId, authorId, viewerId primitive.ObjectID) {
	t.Helper()
	ctx := context.Background()

	authorId = primitive.NewObjectID()
	viewerId = primitive.NewObjectID()
	eventId = primitive.NewObjectID()

	if _, err := db.UsersCollection.InsertOne(ctx, models.User{
		Id:        authorId,
		Email:     "author@example.test",
		FirstName: "Bartholomew",
		LastName:  "Fitzwilliam",
		Nickname:  "Bart",
		Phone:     "+15555550123",
		Role:      models.RoleMember,
	}); err != nil {
		t.Fatalf("insert author: %v", err)
	}
	t.Cleanup(func() { deleteTestUser(authorId) })

	if _, err := db.UsersCollection.InsertOne(ctx, models.User{
		Id:        viewerId,
		Email:     "viewer@example.test",
		FirstName: "View",
		LastName:  "Er",
		Role:      models.RoleMember,
	}); err != nil {
		t.Fatalf("insert viewer: %v", err)
	}
	t.Cleanup(func() { deleteTestUser(viewerId) })

	start := primitive.NewDateTimeFromTime(time.Now().Add(48 * time.Hour))
	if _, err := db.EventsCollection.InsertOne(ctx, models.Event{
		Id:      eventId,
		Type:    models.DOW,
		OwnerId: primitive.NewObjectID(),
		Name:    "Nickname test event",
		ScheduledEvent: &models.CalendarEvent{
			StartDate: start,
			EndDate:   primitive.NewDateTimeFromTime(start.Time().Add(2 * time.Hour)),
		},
		Rsvps: map[string]*models.Rsvp{
			// The snapshot as it was before the nickname existed.
			authorId.Hex(): {Status: models.RsvpGoing, Name: "Bartholomew Fitzwilliam", UserId: authorId},
			"Guest Tom":    {Status: models.RsvpMaybe, Name: "Guest Tom"},
		},
		Polls: []models.Poll{{
			Id:    primitive.NewObjectID(),
			Title: "Where?",
			Options: []models.PollOption{{
				Id:    primitive.NewObjectID(),
				Label: "The Tavern",
				Votes: map[string]string{
					authorId.Hex(): "Bartholomew Fitzwilliam",
					"Guest Tom":    "Guest Tom",
				},
			}},
		}},
	}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	t.Cleanup(func() { cleanupEvent(eventId) })

	comments := []interface{}{
		models.Comment{
			Id:         primitive.NewObjectID(),
			EventId:    eventId,
			UserId:     authorId.Hex(),
			AuthorName: "Bartholomew Fitzwilliam",
			Text:       "I shall attend.",
			CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
		},
		// A comment whose account no longer exists, and a legacy guest row.
		models.Comment{
			Id:         primitive.NewObjectID(),
			EventId:    eventId,
			UserId:     primitive.NewObjectID().Hex(),
			AuthorName: "Departed Member",
			Text:       "I have gone.",
			CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
		},
		models.Comment{
			Id:         primitive.NewObjectID(),
			EventId:    eventId,
			UserId:     "Tom",
			IsGuest:    true,
			AuthorName: "Tom",
			Text:       "Hullo.",
			CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
		},
	}
	if _, err := db.CommentsCollection.InsertMany(ctx, comments); err != nil {
		t.Fatalf("insert comments: %v", err)
	}
	t.Cleanup(func() {
		db.CommentsCollection.DeleteMany(ctx, bson.M{"eventId": eventId})
	})

	return eventId, authorId, viewerId
}

// commentsByText indexes the returned discussion so assertions don't depend on
// ordering.
func commentsByText(t *testing.T, body map[string]interface{}) map[string]map[string]interface{} {
	t.Helper()
	raw, ok := body["comments"].([]interface{})
	if !ok {
		t.Fatalf("comments missing or wrong shape: %#v", body["comments"])
	}
	out := make(map[string]map[string]interface{}, len(raw))
	for _, c := range raw {
		comment, ok := c.(map[string]interface{})
		if !ok {
			t.Fatalf("comment wrong shape: %#v", c)
		}
		text, _ := comment["text"].(string)
		out[text] = comment
	}
	return out
}

func TestGetEvent_ResolvesNicknameOnComment(t *testing.T) {
	requireDB(t)
	eventId, _, viewerId := seedNicknameEvent(t)

	comments := commentsByText(t, fetchEvent(t, eventId, viewerId.Hex()))

	if got := comments["I shall attend."]["authorName"]; got != "Bart" {
		t.Errorf("authorName = %v, want the current nickname %q", got, "Bart")
	}
	author, ok := comments["I shall attend."]["author"].(map[string]interface{})
	if !ok {
		t.Fatalf("author not attached: %#v", comments["I shall attend."])
	}
	if author["nickname"] != "Bart" {
		t.Errorf("attached author nickname = %v", author["nickname"])
	}
}

// The snapshot is what makes a deleted account's history readable, and a guest
// row has no account to resolve at all.
func TestGetEvent_KeepsCommentSnapshotWhenAuthorDoesNotResolve(t *testing.T) {
	requireDB(t)
	eventId, _, viewerId := seedNicknameEvent(t)

	comments := commentsByText(t, fetchEvent(t, eventId, viewerId.Hex()))

	if got := comments["I have gone."]["authorName"]; got != "Departed Member" {
		t.Errorf("deleted author's name = %v, want the stored snapshot", got)
	}
	if _, present := comments["I have gone."]["author"]; present {
		t.Error("no account resolved, so no author object should be serialized")
	}
	if got := comments["Hullo."]["authorName"]; got != "Tom" {
		t.Errorf("guest comment name = %v, want it untouched", got)
	}
}

// The author object exists so a client can render an avatar. It must not
// become a second route by which a member's phone, role or email escapes —
// the same rule stripSensitiveUserFields enforces for respondents.
func TestGetEvent_CommentAuthorCarriesNoPII(t *testing.T) {
	requireDB(t)
	eventId, _, viewerId := seedNicknameEvent(t)

	comments := commentsByText(t, fetchEvent(t, eventId, viewerId.Hex()))
	author, ok := comments["I shall attend."]["author"].(map[string]interface{})
	if !ok {
		t.Fatalf("author not attached: %#v", comments["I shall attend."])
	}

	for _, field := range []string{"email", "phone", "role"} {
		if v, _ := author[field].(string); v != "" {
			t.Errorf("comment author leaked %s = %q", field, v)
		}
	}
	if v := author["calendarAccounts"]; v != nil {
		t.Errorf("comment author leaked calendarAccounts: %#v", v)
	}
}

func TestGetEvent_ResolvesNicknameOnRsvpAndPollVote(t *testing.T) {
	requireDB(t)
	eventId, authorId, viewerId := seedNicknameEvent(t)

	body := fetchEvent(t, eventId, viewerId.Hex())

	rsvps, ok := body["rsvps"].(map[string]interface{})
	if !ok {
		t.Fatalf("rsvps missing: %#v", body["rsvps"])
	}
	rsvp, _ := rsvps[authorId.Hex()].(map[string]interface{})
	if got := rsvp["name"]; got != "Bart" {
		t.Errorf("RSVP name = %v, want the current nickname", got)
	}
	guestRsvp, _ := rsvps["Guest Tom"].(map[string]interface{})
	if got := guestRsvp["name"]; got != "Guest Tom" {
		t.Errorf("legacy guest RSVP name = %v, want it untouched", got)
	}

	polls, ok := body["polls"].([]interface{})
	if !ok || len(polls) == 0 {
		t.Fatalf("polls missing: %#v", body["polls"])
	}
	poll, _ := polls[0].(map[string]interface{})
	options, _ := poll["options"].([]interface{})
	if len(options) == 0 {
		t.Fatalf("poll options missing: %#v", poll)
	}
	option, _ := options[0].(map[string]interface{})
	votes, ok := option["votes"].(map[string]interface{})
	if !ok {
		t.Fatalf("votes missing: %#v", option)
	}
	if got := votes[authorId.Hex()]; got != "Bart" {
		t.Errorf("poll vote name = %v, want the current nickname", got)
	}
	if got := votes["Guest Tom"]; got != "Guest Tom" {
		t.Errorf("legacy guest vote = %v, want it untouched", got)
	}
}
