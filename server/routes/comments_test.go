package routes

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

func TestSanitizeCommentText(t *testing.T) {
	if _, ok := sanitizeCommentText("   "); ok {
		t.Error("whitespace-only comment should be rejected")
	}
	if _, ok := sanitizeCommentText(""); ok {
		t.Error("empty comment should be rejected")
	}
	if got, ok := sanitizeCommentText("  hi there  "); !ok || got != "hi there" {
		t.Errorf("trim: got %q ok=%v, want \"hi there\" true", got, ok)
	}
	long := strings.Repeat("a", maxCommentLength+50)
	got, ok := sanitizeCommentText(long)
	if !ok || len(got) != maxCommentLength {
		t.Errorf("over-long: len=%d ok=%v, want %d true", len(got), ok, maxCommentLength)
	}
}

// --- visibleComments (C13) --------------------------------------------------

// threadFixture builds a discussion with: a plain comment, a public thread with
// one reply, and a members-only thread with one reply.
func threadFixture() (comments []models.Comment, publicRoot, privateRoot, privateReply primitive.ObjectID) {
	plain := primitive.NewObjectID()
	publicRoot = primitive.NewObjectID()
	publicReply := primitive.NewObjectID()
	privateRoot = primitive.NewObjectID()
	privateReply = primitive.NewObjectID()

	comments = []models.Comment{
		{Id: plain, Text: "see you there"},
		{Id: publicRoot, Text: "Hiking trails", IsThread: true, ThreadedBy: "tagger"},
		{Id: publicReply, Text: "the ridge loop is nice", ThreadId: &publicRoot},
		{Id: privateRoot, Text: "Budget", IsThread: true, MembersOnly: true, ThreadedBy: "tagger"},
		{Id: privateReply, Text: "we're $200 over", ThreadId: &privateRoot},
	}
	return
}

func idSet(comments []models.Comment) map[primitive.ObjectID]bool {
	set := make(map[primitive.ObjectID]bool, len(comments))
	for _, c := range comments {
		set[c.Id] = true
	}
	return set
}

func TestVisibleComments_HidesMembersOnlyFromGuests(t *testing.T) {
	comments, publicRoot, privateRoot, privateReply := threadFixture()

	got := visibleComments(comments, commentViewer{UserId: "guest", CanSeeMembersOnly: false})
	seen := idSet(got)

	if seen[privateRoot] {
		t.Error("guest should not see a members-only thread root")
	}
	// The reply carries no flag of its own — it must be hidden via its root.
	if seen[privateReply] {
		t.Error("guest should not see replies inside a members-only thread")
	}
	if !seen[publicRoot] {
		t.Error("guest should still see public threads")
	}
	if len(got) != 3 {
		t.Errorf("guest visible count: got %d, want 3", len(got))
	}
}

func TestVisibleComments_MemberSeesEverything(t *testing.T) {
	comments, _, privateRoot, privateReply := threadFixture()

	got := visibleComments(comments, commentViewer{UserId: "member", CanSeeMembersOnly: true})
	seen := idSet(got)

	if !seen[privateRoot] || !seen[privateReply] {
		t.Error("a member should see members-only threads and their replies")
	}
	if len(got) != len(comments) {
		t.Errorf("member visible count: got %d, want %d", len(got), len(comments))
	}
}

// An anonymous caller never reaches this function in production (getEvent sends
// them an empty list), but the zero viewer must still be treated as a guest.
func TestVisibleComments_ZeroViewerIsTreatedAsGuest(t *testing.T) {
	comments, _, privateRoot, _ := threadFixture()

	got := visibleComments(comments, commentViewer{})
	if idSet(got)[privateRoot] {
		t.Error("the zero viewer must not see members-only threads")
	}
}

func TestVisibleComments_AnnotatesCanManageThread(t *testing.T) {
	comments, publicRoot, _, _ := threadFixture()

	find := func(got []models.Comment, id primitive.ObjectID) models.Comment {
		for _, c := range got {
			if c.Id == id {
				return c
			}
		}
		t.Fatalf("comment %s missing from result", id.Hex())
		return models.Comment{}
	}

	// The person who tagged it can manage it.
	got := visibleComments(comments, commentViewer{UserId: "tagger", CanSeeMembersOnly: true})
	if !find(got, publicRoot).CanManageThread {
		t.Error("the tagger should be able to manage their own thread")
	}

	// An unrelated member cannot.
	got = visibleComments(comments, commentViewer{UserId: "someone-else", CanSeeMembersOnly: true})
	if find(got, publicRoot).CanManageThread {
		t.Error("an unrelated member should not be able to manage the thread")
	}

	// The event owner and admins can.
	got = visibleComments(comments, commentViewer{UserId: "owner", CanSeeMembersOnly: true, IsEventOwner: true})
	if !find(got, publicRoot).CanManageThread {
		t.Error("the event owner should be able to manage any thread")
	}
	got = visibleComments(comments, commentViewer{UserId: "admin", CanSeeMembersOnly: true, IsAdmin: true})
	if !find(got, publicRoot).CanManageThread {
		t.Error("an admin should be able to manage any thread")
	}
}

func TestVisibleComments_EmptyInput(t *testing.T) {
	got := visibleComments(nil, commentViewer{CanSeeMembersOnly: true})
	if got == nil {
		t.Error("should return an empty slice, not nil, so it serializes as []")
	}
	if len(got) != 0 {
		t.Errorf("got %d comments, want 0", len(got))
	}
}

func TestCanSeeMembersOnly_ByRole(t *testing.T) {
	cases := map[models.Role]bool{
		models.RoleSuperAdmin: true,
		models.RoleAdmin:      true,
		models.RoleMember:     true,
		models.RoleGuest:      false,
		models.Role(""):       true, // legacy accounts normalize to member
	}
	for role, want := range cases {
		if got := role.CanSeeMembersOnly(); got != want {
			t.Errorf("role %q: got %v, want %v", role, got, want)
		}
	}
}
