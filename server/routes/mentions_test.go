package routes

import (
	"fmt"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

// mention builds the persisted token form for a user id.
func mention(name string, id primitive.ObjectID) string {
	return fmt.Sprintf("@[%s](%s)", name, id.Hex())
}

// --- parseMentions (F7) -----------------------------------------------------

func TestParseMentions_ExtractsInOrder(t *testing.T) {
	first, second := primitive.NewObjectID(), primitive.NewObjectID()
	text := "morning " + mention("Tom", first) + " and " + mention("Bill", second) + ", are we on?"

	got := parseMentions(text)
	if len(got) != 2 {
		t.Fatalf("got %d mentions, want 2 (%v)", len(got), got)
	}
	if got[0] != first || got[1] != second {
		t.Errorf("order not preserved: got %v, want [%s %s]", got, first.Hex(), second.Hex())
	}
}

func TestParseMentions_NoneWhenTextHasNoTokens(t *testing.T) {
	for _, text := range []string{
		"",
		"just a plain comment",
		"an email address tom@example.test isn't a mention",
		"@Tom on its own is only text", // the composer's raw trigger, never persisted
	} {
		if got := parseMentions(text); len(got) != 0 {
			t.Errorf("parseMentions(%q) = %v, want none", text, got)
		}
	}
}

// A token has to be exactly the persisted shape. Anything looser would let a
// half-typed or hand-mangled mention resolve to an account.
func TestParseMentions_RejectsMalformedTokens(t *testing.T) {
	id := primitive.NewObjectID()
	hex := id.Hex()

	for name, text := range map[string]string{
		"uppercase hex":  "@[Tom](" + strings.ToUpper(hex) + ")",
		"short id":       "@[Tom](" + hex[:23] + ")",
		"long id":        "@[Tom](" + hex + "aa)",
		"non-hex id":     "@[Tom](zzzzzzzzzzzzzzzzzzzzzzzz)",
		"no at sign":     "[Tom](" + hex + ")",
		"empty name":     "@[](" + hex + ")",
		"newline inside": "@[To\nm](" + hex + ")",
		"unclosed":       "@[Tom(" + hex + ")",
		"spaced apart":   "@[Tom] (" + hex + ")",
	} {
		if got := parseMentions(text); len(got) != 0 {
			t.Errorf("%s: parseMentions(%q) = %v, want none", name, text, got)
		}
	}
}

func TestParseMentions_DedupesRepeatedMentionOfOnePerson(t *testing.T) {
	id := primitive.NewObjectID()
	text := mention("Tom", id) + " — sorry " + mention("Tom", id) + " one more thing"

	got := parseMentions(text)
	if len(got) != 1 || got[0] != id {
		t.Fatalf("got %v, want exactly [%s]", got, id.Hex())
	}
}

// A person mentioned twice under two different display names is still one
// person: the id is what identifies them.
func TestParseMentions_DedupesAcrossDifferingDisplayNames(t *testing.T) {
	id := primitive.NewObjectID()
	text := mention("Tom", id) + " aka " + mention("Thomas Foolery", id)

	if got := parseMentions(text); len(got) != 1 {
		t.Fatalf("got %d mentions, want 1 (%v)", len(got), got)
	}
}

// Past the cap the extra tokens stay in the text — they just stop counting as
// mentions, so nobody past the tenth is notified.
func TestParseMentions_CapsAtMaxPerComment(t *testing.T) {
	var text strings.Builder
	ids := make([]primitive.ObjectID, 0, maxMentionsPerComment+5)
	for i := 0; i < maxMentionsPerComment+5; i++ {
		id := primitive.NewObjectID()
		ids = append(ids, id)
		text.WriteString(mention(fmt.Sprintf("Member %d", i), id) + " ")
	}

	got := parseMentions(text.String())
	if len(got) != maxMentionsPerComment {
		t.Fatalf("got %d mentions, want the cap of %d", len(got), maxMentionsPerComment)
	}
	// The ones kept are the first ones written, not an arbitrary subset.
	for i, id := range got {
		if id != ids[i] {
			t.Errorf("mention %d: got %s, want %s", i, id.Hex(), ids[i].Hex())
		}
	}
}

// --- mentionableUserIds (F7) ------------------------------------------------

func TestMentionableUserIds_CollectsEveryVisibleRole(t *testing.T) {
	respondent := primitive.NewObjectID()
	rsvpByKey := primitive.NewObjectID()
	rsvpByField := primitive.NewObjectID()
	voter := primitive.NewObjectID()
	commenter := primitive.NewObjectID()

	event := &models.Event{
		Rsvps: map[string]*models.Rsvp{
			rsvpByKey.Hex(): {Status: models.RsvpGoing},
			// Legacy shape: keyed by the typed-in name, account id in the field.
			"Walk-in Guest": {Status: models.RsvpMaybe, UserId: rsvpByField},
		},
		Polls: []models.Poll{{
			Options: []models.PollOption{{Votes: map[string]string{voter.Hex(): "Voter"}}},
		}},
	}
	responses := []models.EventResponse{{UserId: respondent.Hex()}}
	comments := []models.Comment{{UserId: commenter.Hex()}}

	got := mentionableUserIds(event, responses, comments)

	for _, want := range []primitive.ObjectID{respondent, rsvpByKey, rsvpByField, voter, commenter} {
		if !got[want] {
			t.Errorf("missing %s from the visible set", want.Hex())
		}
	}
	if len(got) != 5 {
		t.Errorf("got %d ids, want 5: %v", len(got), got)
	}
}

// Legacy rows keyed by a typed-in name have no account behind them, so there is
// nobody to mention — they must not become phantom entries in the picker.
func TestMentionableUserIds_SkipsNameKeyedGuestRows(t *testing.T) {
	event := &models.Event{
		Rsvps: map[string]*models.Rsvp{"Cousin Ed": {Status: models.RsvpGoing}},
		Polls: []models.Poll{{
			Options: []models.PollOption{{Votes: map[string]string{"Passer-by": "Passer-by"}}},
		}},
	}
	responses := []models.EventResponse{{UserId: "Some Guest"}}
	comments := []models.Comment{{UserId: "Old Guest", IsGuest: true}}

	if got := mentionableUserIds(event, responses, comments); len(got) != 0 {
		t.Errorf("got %v, want an empty set", got)
	}
}

func TestMentionableUserIds_EmptyEventYieldsNobody(t *testing.T) {
	if got := mentionableUserIds(&models.Event{}, nil, nil); len(got) != 0 {
		t.Errorf("got %v, want an empty set", got)
	}
	if got := mentionableUserIds(nil, nil, nil); len(got) != 0 {
		t.Errorf("nil event: got %v, want an empty set", got)
	}
}

// The guest-visible set is only as narrow as the comments handed to it. This is
// the pairing that enforces the privacy rule: run the discussion through
// visibleComments first and the author of a members-only thread never enters
// the set; hand over the raw list and they do.
func TestMentionableUserIds_HonoursCommentFiltering(t *testing.T) {
	publicAuthor := primitive.NewObjectID()
	privateAuthor := primitive.NewObjectID()
	privateRoot := primitive.NewObjectID()

	comments := []models.Comment{
		{Id: primitive.NewObjectID(), UserId: publicAuthor.Hex()},
		{Id: privateRoot, UserId: privateAuthor.Hex(), IsThread: true, MembersOnly: true},
	}

	guest := commentViewer{UserId: primitive.NewObjectID().Hex(), CanSeeMembersOnly: false}
	filtered := mentionableUserIds(&models.Event{}, nil, visibleComments(comments, guest))
	if filtered[privateAuthor] {
		t.Error("a members-only thread's author leaked into a guest's mentionable set")
	}
	if !filtered[publicAuthor] {
		t.Error("the public comment's author should be mentionable")
	}

	member := commentViewer{UserId: primitive.NewObjectID().Hex(), CanSeeMembersOnly: true}
	unfiltered := mentionableUserIds(&models.Event{}, nil, visibleComments(comments, member))
	if !unfiltered[privateAuthor] {
		t.Error("a member sees the members-only thread, so its author is mentionable")
	}
}

// --- sortMentionables (F7) --------------------------------------------------

func TestSortMentionables_AlphabeticalByDisplayName(t *testing.T) {
	users := []models.User{
		{Id: primitive.NewObjectID(), FirstName: "Zeb", LastName: "Vance"},
		{Id: primitive.NewObjectID(), FirstName: "Ignored", LastName: "Name", Nickname: "arthur"},
		{Id: primitive.NewObjectID(), FirstName: "Bill", LastName: "Wren"},
	}

	sortMentionables(users)

	want := []string{"arthur", "Bill Wren", "Zeb Vance"}
	for i, name := range want {
		if got := users[i].DisplayName(); got != name {
			t.Errorf("position %d: got %q, want %q", i, got, name)
		}
	}
}

// Two people who genuinely share a display name still need a stable order, or
// the picker reshuffles between requests.
func TestSortMentionables_StableForDuplicateNames(t *testing.T) {
	low := primitive.ObjectID{1}
	high := primitive.ObjectID{2}
	users := []models.User{
		{Id: high, FirstName: "Tom", LastName: "Foolery"},
		{Id: low, FirstName: "Tom", LastName: "Foolery"},
	}

	sortMentionables(users)

	if users[0].Id != low {
		t.Errorf("tiebreak: got %s first, want %s", users[0].Id.Hex(), low.Hex())
	}
}
