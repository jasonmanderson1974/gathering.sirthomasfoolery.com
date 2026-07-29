// Notification emails for @mentions (F8). Sent when a comment names someone,
// and on an edit when it names someone it did not name before.
//
// The whole path is best-effort: it runs after the comment is already stored,
// never blocks the request, and a mail failure is logged rather than surfaced.
// Posting a comment must not fail because SMTP is down.
//
// The privacy rule is the one thing here that is not best-effort. A mention
// sitting inside a members-only thread notifies nobody who cannot read that
// thread — otherwise the email would carry the thread's content out to exactly
// the person the members-only flag exists to keep it from.
//
// The residue that rule leaves, deliberately: the token still renders in the
// text, so members reading the hidden thread see the guest's NAME. That is a
// name leaking INTO a thread, seen only by people who may already read the whole
// roll, and never content leaking OUT of it. Accepted.
package routes

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/logger"
	"sirtom/server/models"
	"sirtom/server/utils"
)

// mentionLimiter throttles mention emails per author, so one person cannot turn
// the discussion into a mailing gun — by editing a comment in a loop, or by
// posting many comments that each name ten people. Same shape as the OTP
// limiter in auth.go.
var mentionLimiter = utils.NewRateLimiter()

const (
	// maxMentionEmailsPerHour is the per-author budget. Well clear of what a
	// lively evening in the discussion produces; nowhere near enough to bomb
	// somebody's inbox.
	maxMentionEmailsPerHour = 30

	// maxThreadContextReplies bounds how much of a thread is quoted into the
	// email. The root plus the last few replies is enough to know why you were
	// named; more turns the mail into a transcript.
	maxThreadContextReplies = 3
)

// mentionEmailsConfigured reports whether outbound mail is set up at all. The
// reminder scheduler gates on the same pair; without it a dev machine logs an
// SMTP failure for every mention posted.
func mentionEmailsConfigured() bool {
	return os.Getenv("GMAIL_APP_PASSWORD") != "" && os.Getenv("SCHEJ_EMAIL_ADDRESS") != ""
}

// mentionRecipients decides who is actually mailed about a comment.
//
// Pure, and the reason the rules are testable as a matrix. Four things are
// filtered out, in this order:
//
//   - the author naming themselves — no use to anyone;
//   - anyone already notified by an earlier version of this comment, which is
//     what makes an edit notify only the newly added;
//   - anyone who cannot read the thread this comment sits in (the privacy rule);
//   - accounts with no address to mail, and repeats within one comment.
//
// `mentioned` arrives in the order the names appear in the text, and the result
// preserves it.
func mentionRecipients(
	mentioned []models.User,
	alreadyNotified map[primitive.ObjectID]bool,
	authorId primitive.ObjectID,
	membersOnlyThread bool,
) []models.User {
	recipients := make([]models.User, 0, len(mentioned))
	seen := make(map[primitive.ObjectID]bool, len(mentioned))

	for _, user := range mentioned {
		if user.Id == authorId || seen[user.Id] || alreadyNotified[user.Id] {
			continue
		}
		if membersOnlyThread && !user.EffectiveRole().CanSeeMembersOnly() {
			continue
		}
		if strings.TrimSpace(user.Email) == "" {
			continue
		}
		seen[user.Id] = true
		recipients = append(recipients, user)
	}
	return recipients
}

// mentionThreadContext picks the part of a thread worth quoting to someone named
// in a reply: the root, then up to maxThreadContextReplies replies immediately
// preceding this one.
//
// `visible` must already be filtered through visibleComments FOR THE RECIPIENT.
// Everything here is ordering, not permission — hand it an unfiltered
// discussion and it will happily quote a thread the recipient cannot read.
//
// Selection is by timestamp rather than by position, so it behaves the same
// whether or not the comment being notified about is itself in the slice.
func mentionThreadContext(comment models.Comment, visible []models.Comment) []models.Comment {
	if comment.ThreadId == nil {
		return nil
	}

	var root *models.Comment
	preceding := make([]models.Comment, 0, len(visible))
	for i, candidate := range visible {
		if candidate.Id == *comment.ThreadId {
			root = &visible[i]
			continue
		}
		if candidate.Id == comment.Id || candidate.ThreadId == nil || *candidate.ThreadId != *comment.ThreadId {
			continue
		}
		if candidate.CreatedAt < comment.CreatedAt {
			preceding = append(preceding, candidate)
		}
	}
	if root == nil {
		// The root is hidden from this recipient, so there is no context to
		// show them — the replies alone would be quotes with no question.
		return nil
	}

	if len(preceding) > maxThreadContextReplies {
		preceding = preceding[len(preceding)-maxThreadContextReplies:]
	}
	return append([]models.Comment{*root}, preceding...)
}

// emailTextBlock renders user-written text into an email body: escaped first,
// then its line breaks restored as markup. Order matters — escaping after
// inserting the <br> would neuter them.
func emailTextBlock(text string) string {
	return strings.ReplaceAll(utils.EscapeText(text), "\n", "<br>")
}

// mentionQuote renders one quoted comment as "Author: text", muted.
func mentionQuote(authorName, text string) string {
	return utils.EmailParagraphHTML(fmt.Sprintf(
		"%s: %s", utils.EmailEmphasis(authorName), emailTextBlock(flattenMentions(text)),
	))
}

// buildMentionEmail composes the notification. Pure, so the copy and the
// escaping are testable without a mail server.
//
// Every piece of user-controlled text on this path — the event name, the
// author's and recipient's display names, the comment itself and any quoted
// context — is escaped by the layout helpers or by emailTextBlock. A comment is
// the most attacker-controlled string the app mails anywhere, so this matters
// more here than in the response notifications.
func buildMentionEmail(
	recipientName, authorName, eventName, eventURL, commentText string,
	context []models.Comment,
) (subject, body string) {
	subject = fmt.Sprintf("%s mentioned you in %s", authorName, eventName)

	var content strings.Builder
	content.WriteString(utils.EmailParagraph(
		fmt.Sprintf("%s, %s has named you in the discussion.", recipientName, authorName),
	))
	content.WriteString(utils.EmailStrongLine(eventName))

	if len(context) > 0 {
		content.WriteString(utils.EmailFootnote("Earlier in the thread"))
		for _, quoted := range context {
			content.WriteString(mentionQuote(quoted.AuthorName, quoted.Text))
		}
	}

	content.WriteString(utils.EmailRow(emailTextBlock(flattenMentions(commentText))))

	body = utils.RenderEmailWithFooter(
		"You were named",
		content.String(),
		utils.EmailFooterURL(eventURL),
		utils.EmailAction{Label: "View the discussion", URL: eventURL},
	)
	return subject, body
}

// notifyMentions mails everyone newly named by a comment.
//
// `alreadyNotified` is what the comment named BEFORE this write — nil for a new
// comment, the stored Mentions for an edit. That diff is the whole reason
// Comment.Mentions is persisted: without it, editing a typo in a comment would
// re-mail everyone it names.
//
// Never returns an error and never blocks: it is called after the comment is
// safely stored, and everything it does is a courtesy on top of that.
func notifyMentions(
	event *models.Event,
	comment models.Comment,
	author *models.User,
	alreadyNotified []primitive.ObjectID,
) {
	if len(comment.Mentions) == 0 || author == nil || !mentionEmailsConfigured() {
		return
	}

	accounts := db.GetUsersByIds(comment.Mentions)
	// Rebuild in mention order — GetUsersByIds returns a map.
	mentioned := make([]models.User, 0, len(comment.Mentions))
	for _, id := range comment.Mentions {
		if user, ok := accounts[id.Hex()]; ok {
			mentioned = append(mentioned, user)
		}
	}

	notified := make(map[primitive.ObjectID]bool, len(alreadyNotified))
	for _, id := range alreadyNotified {
		notified[id] = true
	}

	recipients := mentionRecipients(mentioned, notified, author.Id, mentionThreadIsMembersOnly(comment))
	if len(recipients) == 0 {
		return
	}

	// Only a reply needs the discussion fetched, and only once for all of its
	// recipients — the per-recipient filtering happens below.
	var discussion []models.Comment
	if comment.ThreadId != nil {
		if fetched, err := db.GetComments(event.Id.Hex()); err == nil {
			discussion = fetched
		}
	}

	authorName := author.DisplayName()
	eventURL := eventURLFor(event)
	for _, recipient := range recipients {
		// Budget is per author, counted per email: ten mentions in one comment
		// spend ten. On exhaustion the remaining sends are dropped silently —
		// the comment is already posted, and telling the author their mentions
		// were throttled is not worth failing anything over.
		if !mentionLimiter.Allow(author.Id.Hex(), maxMentionEmailsPerHour, time.Hour) {
			logger.StdErr.Println("mention emails rate-limited for author", author.Id.Hex())
			return
		}

		// Context is resolved per recipient: two people named in the same reply
		// may be able to see different amounts of the thread above it.
		context := mentionThreadContext(comment, visibleComments(discussion, newCommentViewer(&recipient, event)))

		subject, body := buildMentionEmail(
			recipient.DisplayName(), authorName, event.Name, eventURL, comment.Text, context,
		)
		utils.SendEmailAsync(recipient.Email, subject, body, "text/html")
	}
}

// mentionThreadIsMembersOnly reports whether this comment sits inside a hidden
// thread — either as a reply under a members-only root, or by being that root.
//
// A reply carries no flag of its own (replies inherit the root's), so the root
// has to be read back. Failing to resolve it is treated as members-only: the
// safe direction is to send nothing.
func mentionThreadIsMembersOnly(comment models.Comment) bool {
	if comment.IsThread {
		return comment.MembersOnly
	}
	if comment.ThreadId == nil {
		return false
	}
	root, err := db.GetCommentById(comment.ThreadId.Hex())
	if err != nil || root == nil {
		logger.StdErr.Println("mention notify: thread root unresolved, withholding emails")
		return true
	}
	return root.MembersOnly
}
