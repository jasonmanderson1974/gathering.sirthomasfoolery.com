package routes

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
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

	// F11: the roster renders an avatar, so the slim account rides along.
	slim := rsvps[id.Hex()].User
	if slim == nil {
		t.Fatal("account RSVP got no attached user")
	}
	if slim.Id != id || slim.Nickname != "Bart" {
		t.Errorf("attached user lost identity fields: %+v", slim)
	}
	if slim.Email != "" {
		t.Errorf("attached user leaked an email: %q", slim.Email)
	}
	if rsvps["Guest Tom"].User != nil {
		t.Error("legacy guest RSVP should get no attached user")
	}
}

// The RSVP write path $sets the whole rsvps map from the in-memory struct, so
// the attached user would be persisted — and go stale — without `bson:"-"`.
func TestRsvpUserIsNeverPersisted(t *testing.T) {
	id := primitive.NewObjectID()
	rsvp := models.Rsvp{
		Status: models.RsvpGoing,
		Name:   "Bart",
		User:   &models.User{Id: id, FirstName: "Bart"},
	}

	raw, err := bson.Marshal(rsvp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc bson.M
	if err := bson.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := doc["user"]; ok {
		t.Fatalf("the resolved user was serialized into the stored document: %v", doc)
	}
	if doc["name"] != "Bart" {
		t.Errorf("name should still persist, got %v", doc["name"])
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

// Whoever ticked a checklist box is displayed next to the author, so their id
// has to join the same lookup — including when they didn't write the item.
func TestEventDisplayNameIdsIncludesTheChecker(t *testing.T) {
	author := primitive.NewObjectID()
	checker := primitive.NewObjectID()
	zero := primitive.NilObjectID

	lists := []models.EventList{{Items: []models.EventListItem{
		{UserId: author, CheckedBy: &checker},
		{UserId: author},                   // never ticked: nothing extra
		{UserId: author, CheckedBy: &zero}, // a zero id contributes nothing
	}}}

	ids := eventDisplayNameIds(nil, nil, nil, lists)

	if len(ids) != 2 {
		t.Fatalf("got %d ids, want 2 (author + checker): %v", len(ids), ids)
	}
	found := make(map[primitive.ObjectID]bool, len(ids))
	for _, id := range ids {
		found[id] = true
	}
	if !found[author] || !found[checker] {
		t.Errorf("ids %v are missing the author or the checker", ids)
	}
}

// A nickname change has to reach "Checked by …" the same way it reaches the
// author line, and an id that no longer resolves keeps its stored snapshot.
func TestResolveListItemNamesResolvesAuthorAndChecker(t *testing.T) {
	author := primitive.NewObjectID()
	checker := primitive.NewObjectID()
	deleted := primitive.NewObjectID()

	lists := []models.EventList{{Items: []models.EventListItem{
		{UserId: author, AuthorName: "Ada Old", CheckedBy: &checker, CheckedByName: "Bart Old"},
		{UserId: deleted, AuthorName: "Gone Away", CheckedBy: &deleted, CheckedByName: "Gone Away"},
	}}}
	users := map[string]models.User{
		author.Hex():  {Id: author, Nickname: "Ada"},
		checker.Hex(): {Id: checker, FirstName: "Bart", LastName: "New"},
	}

	resolveListItemNames(lists, users)

	items := lists[0].Items
	if items[0].AuthorName != "Ada" {
		t.Errorf("author name = %q, want the current nickname", items[0].AuthorName)
	}
	if items[0].CheckedByName != "Bart New" {
		t.Errorf("checker name = %q, want the current display name", items[0].CheckedByName)
	}
	if items[1].AuthorName != "Gone Away" || items[1].CheckedByName != "Gone Away" {
		t.Errorf("an unresolvable id must keep its snapshot, got %+v", items[1])
	}
}

// The assignee's name (N1) is a write-time snapshot like the other two, so it
// has to be collected for the batched lookup and rewritten from it.
func TestResolveListItemNamesResolvesTheAssignee(t *testing.T) {
	author := primitive.NewObjectID()
	assignee := primitive.NewObjectID()
	deleted := primitive.NewObjectID()
	zero := primitive.NilObjectID

	lists := []models.EventList{{Items: []models.EventListItem{
		{UserId: author, AuthorName: "Ada", AssigneeId: &assignee, AssigneeName: "Bart Old"},
		{UserId: author, AuthorName: "Ada", AssigneeId: &deleted, AssigneeName: "Gone Away"},
		// Never assigned, and a zero id: neither contributes an id to look up.
		{UserId: author, AuthorName: "Ada"},
		{UserId: author, AuthorName: "Ada", AssigneeId: &zero},
	}}}

	ids := eventDisplayNameIds(nil, nil, nil, lists)
	found := make(map[primitive.ObjectID]bool, len(ids))
	for _, id := range ids {
		found[id] = true
	}
	if !found[assignee] {
		t.Errorf("ids %v are missing the assignee", ids)
	}
	if len(ids) != 3 {
		t.Errorf("got %d ids, want 3 (author + assignee + the deleted assignee): %v", len(ids), ids)
	}

	resolveListItemNames(lists, map[string]models.User{
		author.Hex():   {Id: author, Nickname: "Ada"},
		assignee.Hex(): {Id: assignee, FirstName: "Bart", LastName: "New"},
	})

	items := lists[0].Items
	if items[0].AssigneeName != "Bart New" {
		t.Errorf("assignee name = %q, want the current display name", items[0].AssigneeName)
	}
	if items[1].AssigneeName != "Gone Away" {
		t.Errorf("an unresolvable assignee must keep its snapshot, got %q", items[1].AssigneeName)
	}
	// The author line still resolves on every row — the assignee branch must not
	// have been made to short-circuit it.
	for i, item := range items {
		if item.AuthorName != "Ada" {
			t.Errorf("item %d lost its author resolution: %q", i, item.AuthorName)
		}
	}
}
