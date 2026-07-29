package routes

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

func nicknamedUser(id primitive.ObjectID, first, last, nickname string) models.User {
	return models.User{
		Id:        id,
		Email:     "someone@example.test",
		FirstName: first,
		LastName:  last,
		Nickname:  nickname,
		Phone:     "555-0100",
		Role:      models.RoleAdmin,
		Picture:   "https://example.test/pic.png",
	}
}

func TestAttachCommentAuthorsUsesCurrentDisplayName(t *testing.T) {
	id := primitive.NewObjectID()
	users := map[string]models.User{
		id.Hex(): nicknamedUser(id, "Bartholomew", "Fitzwilliam", "Bart"),
	}
	comments := []models.Comment{
		{UserId: id.Hex(), AuthorName: "Bartholomew Fitzwilliam", Text: "hello"},
	}

	got := attachCommentAuthors(comments, users)

	if got[0].AuthorName != "Bart" {
		t.Errorf("AuthorName = %q, want the current display name %q", got[0].AuthorName, "Bart")
	}
	if got[0].Author == nil {
		t.Fatal("Author was not attached")
	}
	if got[0].Author.Nickname != "Bart" {
		t.Errorf("attached author nickname = %q", got[0].Author.Nickname)
	}
}

// The stored snapshot is what keeps a deleted account's comments readable.
func TestAttachCommentAuthorsFallsBackToTheSnapshot(t *testing.T) {
	comments := []models.Comment{
		{UserId: primitive.NewObjectID().Hex(), AuthorName: "Departed Member", Text: "hello"},
	}

	got := attachCommentAuthors(comments, map[string]models.User{})

	if got[0].AuthorName != "Departed Member" {
		t.Errorf("AuthorName = %q, want the stored snapshot to survive", got[0].AuthorName)
	}
	if got[0].Author != nil {
		t.Error("no account resolved, so no Author should be attached")
	}
}

// Legacy guest rows hold a typed-in name in UserId, which must never be
// treated as an account id.
func TestAttachCommentAuthorsLeavesGuestRowsAlone(t *testing.T) {
	comments := []models.Comment{
		{UserId: "Tom", IsGuest: true, AuthorName: "Tom", Text: "hello"},
	}

	got := attachCommentAuthors(comments, map[string]models.User{})

	if got[0].AuthorName != "Tom" || got[0].Author != nil {
		t.Errorf("guest row was modified: name=%q author=%v", got[0].AuthorName, got[0].Author)
	}
}

// The whole point of the Author field is rendering an avatar — it must not
// become a second route by which a member's phone, role or email escapes.
func TestSlimUserForDisplayCarriesNoPII(t *testing.T) {
	id := primitive.NewObjectID()
	user := nicknamedUser(id, "Bartholomew", "Fitzwilliam", "Bart")
	user.CalendarAccounts = map[string]models.CalendarAccount{"a@b.com_google": {}}

	slim := slimUserForDisplay(user)

	if slim.Email != "" {
		t.Errorf("email leaked: %q", slim.Email)
	}
	if slim.Phone != "" {
		t.Errorf("phone leaked: %q", slim.Phone)
	}
	if slim.Role != "" {
		t.Errorf("role leaked: %q", slim.Role)
	}
	if slim.CalendarAccounts != nil {
		t.Error("calendar accounts leaked")
	}
	// ...while still carrying what a client actually renders.
	if slim.Id != id || slim.Nickname != "Bart" || slim.Picture == "" {
		t.Errorf("slim user lost identity fields: %+v", slim)
	}
}

func TestResolveRsvpNames(t *testing.T) {
	id := primitive.NewObjectID()
	users := map[string]models.User{
		id.Hex(): nicknamedUser(id, "Bartholomew", "Fitzwilliam", "Bart"),
	}
	rsvps := map[string]*models.Rsvp{
		id.Hex():      {Name: "Bartholomew Fitzwilliam", Status: models.RsvpGoing},
		"Guest Tom":   {Name: "Guest Tom", Status: models.RsvpMaybe},
		"missing-acc": nil,
	}

	resolveRsvpNames(rsvps, users)

	if got := rsvps[id.Hex()].Name; got != "Bart" {
		t.Errorf("account RSVP name = %q, want %q", got, "Bart")
	}
	if got := rsvps["Guest Tom"].Name; got != "Guest Tom" {
		t.Errorf("legacy guest RSVP was rewritten to %q", got)
	}
}

func TestResolvePollVoteNames(t *testing.T) {
	id := primitive.NewObjectID()
	users := map[string]models.User{
		id.Hex(): nicknamedUser(id, "Bartholomew", "Fitzwilliam", "Bart"),
	}
	polls := []models.Poll{{
		Options: []models.PollOption{{
			Votes: map[string]string{
				id.Hex():    "Bartholomew Fitzwilliam",
				"Guest Tom": "Guest Tom",
			},
		}},
	}}

	resolvePollVoteNames(polls, users)

	votes := polls[0].Options[0].Votes
	if votes[id.Hex()] != "Bart" {
		t.Errorf("account vote name = %q, want %q", votes[id.Hex()], "Bart")
	}
	if votes["Guest Tom"] != "Guest Tom" {
		t.Errorf("legacy guest vote was rewritten to %q", votes["Guest Tom"])
	}
}

// One query for all four sources is the point — so the id set has to be the
// deduped union, and must exclude everything that isn't an account id.
func TestEventDisplayNameIdsDedupesTheUnion(t *testing.T) {
	shared := primitive.NewObjectID()
	rsvpOnly := primitive.NewObjectID()
	voteOnly := primitive.NewObjectID()
	listOnly := primitive.NewObjectID()

	comments := []models.Comment{
		{UserId: shared.Hex()},
		{UserId: shared.Hex()}, // same author twice
		{UserId: "Tom", IsGuest: true},
		{UserId: "not-an-object-id"},
	}
	rsvps := map[string]*models.Rsvp{
		shared.Hex():   {},
		rsvpOnly.Hex(): {},
		"Guest Tom":    {},
	}
	polls := []models.Poll{{Options: []models.PollOption{{
		Votes: map[string]string{shared.Hex(): "x", voteOnly.Hex(): "y", "Guest Tom": "z"},
	}}}}
	lists := []models.EventList{{Items: []models.EventListItem{
		{UserId: shared},
		{UserId: listOnly},
		{}, // an item with no author id contributes nothing
	}}}

	ids := eventDisplayNameIds(comments, rsvps, polls, lists)

	if len(ids) != 4 {
		t.Fatalf("got %d ids, want 4 (the deduped union): %v", len(ids), ids)
	}
	found := make(map[primitive.ObjectID]bool, len(ids))
	for _, id := range ids {
		if found[id] {
			t.Errorf("duplicate id %s", id.Hex())
		}
		found[id] = true
	}
	for _, want := range []primitive.ObjectID{shared, rsvpOnly, voteOnly, listOnly} {
		if !found[want] {
			t.Errorf("missing id %s", want.Hex())
		}
	}
}

func TestEventDisplayNameIdsEmptyWhenNothingToResolve(t *testing.T) {
	if ids := eventDisplayNameIds(nil, nil, nil, nil); len(ids) != 0 {
		t.Errorf("got %v, want no ids (and therefore no query)", ids)
	}
}
