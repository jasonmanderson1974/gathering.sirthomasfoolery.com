package routes

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// --- flattenMentions (F8) ---------------------------------------------------

func TestFlattenMentions_RendersTokensAsPlainText(t *testing.T) {
	tom, bill := primitive.NewObjectID(), primitive.NewObjectID()
	text := "morning " + mention("Tom Foolery", tom) + " and " + mention("Bill", bill) + "!"

	got := flattenMentions(text)
	if want := "morning @Tom Foolery and @Bill!"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFlattenMentions_LeavesOrdinaryTextAlone(t *testing.T) {
	for _, text := range []string{"", "no mentions here", "an @at sign, [brackets] and (parens)"} {
		if got := flattenMentions(text); got != text {
			t.Errorf("flattenMentions(%q) = %q, want it unchanged", text, got)
		}
	}
}

// A `$` in a display name must survive: it is a substitution marker in Go's
// replacement templates, so a naive implementation eats it.
func TestFlattenMentions_PreservesDollarInDisplayName(t *testing.T) {
	id := primitive.NewObjectID()
	got := flattenMentions(mention("Bill $ Wren", id))
	if want := "@Bill $ Wren"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- mentionRecipients (F8) -------------------------------------------------

func mentionUser(role models.Role, name string) models.User {
	return models.User{
		Id:        primitive.NewObjectID(),
		FirstName: name,
		Email:     strings.ToLower(name) + "@example.test",
		Role:      role,
	}
}

func recipientNames(users []models.User) []string {
	names := make([]string, 0, len(users))
	for _, u := range users {
		names = append(names, u.DisplayName())
	}
	return names
}

func TestMentionRecipients_MailsEveryoneNamed(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	tom := mentionUser(models.RoleMember, "Tom")
	bill := mentionUser(models.RoleGuest, "Bill")

	got := mentionRecipients([]models.User{tom, bill}, nil, author.Id, false)

	if names := recipientNames(got); len(names) != 2 || names[0] != "Tom" || names[1] != "Bill" {
		t.Errorf("got %v, want [Tom Bill] in mention order", names)
	}
}

func TestMentionRecipients_SkipsTheAuthorNamingThemselves(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	tom := mentionUser(models.RoleMember, "Tom")

	got := mentionRecipients([]models.User{author, tom}, nil, author.Id, false)

	if names := recipientNames(got); len(names) != 1 || names[0] != "Tom" {
		t.Errorf("got %v, want only [Tom]", names)
	}
}

// The edit diff: someone the previous version already named is not re-mailed,
// so fixing a typo notifies nobody.
func TestMentionRecipients_SkipsAlreadyNotified(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	tom := mentionUser(models.RoleMember, "Tom")
	bill := mentionUser(models.RoleMember, "Bill")

	notified := map[primitive.ObjectID]bool{tom.Id: true}
	got := mentionRecipients([]models.User{tom, bill}, notified, author.Id, false)

	if names := recipientNames(got); len(names) != 1 || names[0] != "Bill" {
		t.Errorf("got %v, want only the newly added [Bill]", names)
	}
}

func TestMentionRecipients_EditThatAddsNobodyMailsNobody(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	tom := mentionUser(models.RoleMember, "Tom")

	notified := map[primitive.ObjectID]bool{tom.Id: true}
	if got := mentionRecipients([]models.User{tom}, notified, author.Id, false); len(got) != 0 {
		t.Errorf("got %v, want nobody mailed", recipientNames(got))
	}
}

func TestMentionRecipients_DedupesWithinOneComment(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	tom := mentionUser(models.RoleMember, "Tom")

	if got := mentionRecipients([]models.User{tom, tom}, nil, author.Id, false); len(got) != 1 {
		t.Errorf("got %d recipients, want 1", len(got))
	}
}

// The privacy rule. A guest named inside a members-only thread is not mailed —
// the email would carry the thread's content to the person it is hidden from.
func TestMentionRecipients_MembersOnlyThreadExcludesGuests(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	member := mentionUser(models.RoleMember, "Member")
	guest := mentionUser(models.RoleGuest, "Guest")

	got := mentionRecipients([]models.User{member, guest}, nil, author.Id, true)

	if names := recipientNames(got); len(names) != 1 || names[0] != "Member" {
		t.Errorf("got %v, want only [Member] — a guest must not be mailed a hidden thread", names)
	}
}

// The same two people, in an ordinary thread, both get mailed — the exclusion
// is about the thread, not about being a guest.
func TestMentionRecipients_GuestMailedInAnOpenThread(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	guest := mentionUser(models.RoleGuest, "Guest")

	if got := mentionRecipients([]models.User{guest}, nil, author.Id, false); len(got) != 1 {
		t.Errorf("got %v, want the guest mailed", recipientNames(got))
	}
}

// Admins and superAdmins can read members-only threads, so they stay in.
func TestMentionRecipients_MembersOnlyKeepsElevatedRoles(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	admin := mentionUser(models.RoleAdmin, "Admin")
	super := mentionUser(models.RoleSuperAdmin, "Super")
	// A legacy account with no role at all normalises to member.
	legacy := mentionUser("", "Legacy")

	got := mentionRecipients([]models.User{admin, super, legacy}, nil, author.Id, true)
	if len(got) != 3 {
		t.Errorf("got %v, want all three mailed", recipientNames(got))
	}
}

func TestMentionRecipients_SkipsAccountsWithNoAddress(t *testing.T) {
	author := mentionUser(models.RoleMember, "Author")
	noEmail := mentionUser(models.RoleMember, "Ghost")
	noEmail.Email = "   "

	if got := mentionRecipients([]models.User{noEmail}, nil, author.Id, false); len(got) != 0 {
		t.Errorf("got %v, want nobody — there is no address to mail", recipientNames(got))
	}
}

// --- mentionThreadContext (F8) ----------------------------------------------

// replyAt builds a reply under root, stamped at a fixed offset so ordering is
// explicit rather than dependent on how fast the test runs.
func replyAt(root primitive.ObjectID, author string, text string, minute int) models.Comment {
	return models.Comment{
		Id:         primitive.NewObjectID(),
		UserId:     primitive.NewObjectID().Hex(),
		AuthorName: author,
		Text:       text,
		ThreadId:   &root,
		CreatedAt:  primitive.NewDateTimeFromTime(time.Date(2026, 7, 29, 10, minute, 0, 0, time.UTC)),
	}
}

func TestMentionThreadContext_TopLevelCommentHasNone(t *testing.T) {
	comment := models.Comment{Id: primitive.NewObjectID(), Text: "hello"}
	if got := mentionThreadContext(comment, []models.Comment{comment}); got != nil {
		t.Errorf("got %d quoted comments, want none for a top-level comment", len(got))
	}
}

func TestMentionThreadContext_RootPlusPrecedingReplies(t *testing.T) {
	rootId := primitive.NewObjectID()
	root := models.Comment{
		Id: rootId, AuthorName: "Planner", Text: "Budget", IsThread: true,
		CreatedAt: primitive.NewDateTimeFromTime(time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)),
	}
	first := replyAt(rootId, "Tom", "we're over", 1)
	second := replyAt(rootId, "Bill", "by how much?", 2)
	target := replyAt(rootId, "Author", "ask @[Greg](66f)", 3)
	later := replyAt(rootId, "Someone", "after the fact", 4)

	got := mentionThreadContext(target, []models.Comment{root, first, second, target, later})

	if len(got) != 3 {
		t.Fatalf("got %d quoted comments, want 3 (root + 2 preceding)", len(got))
	}
	if got[0].Id != rootId {
		t.Error("the root should be quoted first")
	}
	for i, want := range []string{"Planner", "Tom", "Bill"} {
		if got[i].AuthorName != want {
			t.Errorf("quote %d: got %q, want %q", i, got[i].AuthorName, want)
		}
	}
}

// Only the last few replies are quoted — the email is context, not a transcript.
func TestMentionThreadContext_CapsPrecedingReplies(t *testing.T) {
	rootId := primitive.NewObjectID()
	root := models.Comment{Id: rootId, AuthorName: "Planner", Text: "Budget", IsThread: true}

	discussion := []models.Comment{root}
	for i := 1; i <= 8; i++ {
		discussion = append(discussion, replyAt(rootId, "Reply", "line", i))
	}
	target := replyAt(rootId, "Author", "ask Greg", 9)
	discussion = append(discussion, target)

	got := mentionThreadContext(target, discussion)

	if len(got) != maxThreadContextReplies+1 {
		t.Fatalf("got %d quoted comments, want %d (root + cap)", len(got), maxThreadContextReplies+1)
	}
	// The ones kept are the replies nearest the mention, not the oldest.
	if got[len(got)-1].Id != discussion[len(discussion)-2].Id {
		t.Error("the last quoted reply should be the one immediately before the mention")
	}
}

// Replies to a DIFFERENT thread are not context for this one.
func TestMentionThreadContext_IgnoresOtherThreads(t *testing.T) {
	rootId, otherRootId := primitive.NewObjectID(), primitive.NewObjectID()
	root := models.Comment{Id: rootId, AuthorName: "Planner", Text: "Budget", IsThread: true}
	other := models.Comment{Id: otherRootId, AuthorName: "Planner", Text: "Menu", IsThread: true}
	elsewhere := replyAt(otherRootId, "Stranger", "chips", 1)
	target := replyAt(rootId, "Author", "ask Greg", 2)

	got := mentionThreadContext(target, []models.Comment{root, other, elsewhere, target})

	if len(got) != 1 || got[0].Id != rootId {
		t.Errorf("got %d quotes, want just this thread's root", len(got))
	}
}

// Filtering is the caller's job, and this is what it buys: a recipient who
// cannot see the root gets no quoted context at all, rather than replies
// stripped of the question they answer.
func TestMentionThreadContext_NoRootMeansNoContext(t *testing.T) {
	rootId := primitive.NewObjectID()
	orphan := replyAt(rootId, "Tom", "we're over", 1)
	target := replyAt(rootId, "Author", "ask Greg", 2)

	if got := mentionThreadContext(target, []models.Comment{orphan, target}); got != nil {
		t.Errorf("got %d quotes, want none when the root is not visible", len(got))
	}
}

// --- buildMentionEmail (F8) -------------------------------------------------

func TestBuildMentionEmail_SubjectAndBody(t *testing.T) {
	subject, body := buildMentionEmail(
		"Greg", "Tom Foolery", "Poker Night", testEventURL, "are you in?", nil,
	)

	if want := "Tom Foolery mentioned you in Poker Night"; subject != want {
		t.Errorf("subject = %q, want %q", subject, want)
	}
	for _, want := range []string{"Greg", "Tom Foolery", "Poker Night", "are you in?", testEventURL, "View the discussion"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

// The email reads as prose, so the tokens must not reach it raw.
func TestBuildMentionEmail_FlattensTokensInTheBody(t *testing.T) {
	id := primitive.NewObjectID()
	text := "morning " + mention("Greg Wallace", id) + ", are you in?"

	_, body := buildMentionEmail("Greg", "Tom", "Poker Night", testEventURL, text, nil)

	if strings.Contains(body, id.Hex()) {
		t.Error("the raw user id leaked into the email body")
	}
	if !strings.Contains(body, "@Greg Wallace") {
		t.Errorf("body should render the mention as plain text: %s", body)
	}
}

// A comment is the most attacker-controlled string the app mails anywhere.
func TestBuildMentionEmail_EscapesEveryUserControlledField(t *testing.T) {
	const evil = `<img src=x onerror=alert(1)>`

	_, body := buildMentionEmail(evil, evil, evil, testEventURL, evil, []models.Comment{
		{AuthorName: evil, Text: evil},
	})

	if strings.Contains(body, "<img") {
		t.Errorf("markup survived into the email body: %s", body)
	}
	if !strings.Contains(body, "&lt;img") {
		t.Error("expected the escaped form to be present")
	}
}

// Line breaks an author typed should survive as breaks, not collapse into one
// run-on line — but only as markup added AFTER escaping.
func TestBuildMentionEmail_PreservesLineBreaks(t *testing.T) {
	_, body := buildMentionEmail("Greg", "Tom", "Poker Night", testEventURL, "first line\nsecond line", nil)

	if !strings.Contains(body, "first line<br>second line") {
		t.Errorf("line break not preserved: %s", body)
	}
}

func TestBuildMentionEmail_QuotesThreadContext(t *testing.T) {
	rootId := primitive.NewObjectID()
	context := []models.Comment{
		{Id: rootId, AuthorName: "Planner", Text: "Budget", IsThread: true},
		{AuthorName: "Tom", Text: "we're $200 over"},
	}

	_, body := buildMentionEmail("Greg", "Tom", "Poker Night", testEventURL, "thoughts?", context)

	// Note the escaped apostrophe: quoted context goes through the same escaping
	// as the comment itself, which is the point of asserting on it here.
	for _, want := range []string{"Earlier in the thread", "Planner", "Budget", "we&#39;re $200 over"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing quoted context %q", want)
		}
	}
}

func TestBuildMentionEmail_NoContextBlockWhenNotAReply(t *testing.T) {
	_, body := buildMentionEmail("Greg", "Tom", "Poker Night", testEventURL, "are you in?", nil)

	if strings.Contains(body, "Earlier in the thread") {
		t.Error("a top-level mention should carry no thread-context block")
	}
}

// --- the env gate -----------------------------------------------------------

func TestMentionEmailsConfigured_NeedsBothHalves(t *testing.T) {
	for name, tc := range map[string]struct {
		password, from string
		want           bool
	}{
		"both set":        {"app-password", "fellowship@example.test", true},
		"no password":     {"", "fellowship@example.test", false},
		"no from address": {"app-password", "", false},
		"neither":         {"", "", false},
	} {
		t.Setenv("GMAIL_APP_PASSWORD", tc.password)
		t.Setenv("SCHEJ_EMAIL_ADDRESS", tc.from)
		if got := mentionEmailsConfigured(); got != tc.want {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

// --- mentionThreadIsMembersOnly (DB-gated) ----------------------------------

// This is the switch the whole privacy rule hangs off, and unlike the rest of
// the notify path it has to read the thread root back — a reply carries no flag
// of its own. Worth exercising against real documents.

// insertThreadRoot stores a thread root and returns it.
func insertThreadRoot(t *testing.T, eventId primitive.ObjectID, membersOnly bool) models.Comment {
	t.Helper()
	root := models.Comment{
		Id: primitive.NewObjectID(), EventId: eventId, UserId: primitive.NewObjectID().Hex(),
		Text: "Budget", IsThread: true, MembersOnly: membersOnly,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}
	if err := db.InsertComment(root); err != nil {
		t.Fatalf("insert thread root: %v", err)
	}
	return root
}

func TestMentionThreadIsMembersOnly_ResolvesTheRoot(t *testing.T) {
	requireDB(t)

	ownerId := insertTestUser(t, models.RoleMember, "mention-thread-owner@example.test")
	eventId := insertTestEvent(t, ownerId)

	hidden := insertThreadRoot(t, eventId, true)
	open := insertThreadRoot(t, eventId, false)

	reply := func(root models.Comment) models.Comment {
		return models.Comment{Id: primitive.NewObjectID(), EventId: eventId, ThreadId: &root.Id}
	}

	if !mentionThreadIsMembersOnly(reply(hidden)) {
		t.Error("a reply under a members-only root must count as members-only")
	}
	if mentionThreadIsMembersOnly(reply(open)) {
		t.Error("a reply under an open root must not")
	}
	if !mentionThreadIsMembersOnly(hidden) {
		t.Error("the members-only root itself counts")
	}
	if mentionThreadIsMembersOnly(open) {
		t.Error("an open root does not")
	}
	if mentionThreadIsMembersOnly(models.Comment{Id: primitive.NewObjectID(), EventId: eventId}) {
		t.Error("a top-level comment is in no thread at all")
	}
}

// A root that cannot be read back must fail CLOSED — withholding an email is
// recoverable, sending a hidden thread's contents to a guest is not.
func TestMentionThreadIsMembersOnly_UnresolvableRootWithholds(t *testing.T) {
	requireDB(t)

	missing := primitive.NewObjectID()
	comment := models.Comment{Id: primitive.NewObjectID(), ThreadId: &missing}

	if !mentionThreadIsMembersOnly(comment) {
		t.Error("an unresolvable thread root must be treated as members-only")
	}
}
