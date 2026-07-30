// @Mentions in the discussion (F7) — parsing, validation, and the endpoint that
// tells a client who may be mentioned.
//
// A mention is persisted inside the comment text as `@[Display Name](userId)`.
// Keeping it in the text rather than in a parallel offset table is what makes it
// survive an edit: there are no positions to keep in sync, and a comment that is
// rewritten carries its mentions along by construction. The cost is that the
// token counts against the 2000-rune comment cap, which is accepted.
//
// Who may be mentioned depends on the caller's role, and the rule is the
// club's, not a technical one: a member may mention anyone on the roll, because
// the roll is not a secret from members. A guest may mention only people already
// visible to them on this event — otherwise the picker would enumerate the
// entire membership to someone invited to one gathering.
//
// That rule is enforced on the WRITE path, not just in the picker. A guest who
// hand-crafts a token for an id they were never shown gets it dropped from
// Mentions (so no notification in F8), while the literal text stays exactly as
// typed — refusing the comment outright would leak that the id is real.
package routes

import (
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// maxMentionsPerComment bounds how many people one comment can notify. Past
// this, the extra tokens stay in the text and simply stop counting as mentions
// — a comment that @s the whole club is a mailing list, not a mention.
const maxMentionsPerComment = 10

// mentionPattern matches one persisted mention token.
//
// The display half is bounded and excludes `]` and newlines so a token can
// never swallow the rest of a comment; the id half is exactly a 24-char lower
// hex ObjectID, which means a match is always parseable and there is no way to
// smuggle a query or a partial id through it.
var mentionPattern = regexp.MustCompile(`@\[([^\]\n]{1,60})\]\(([0-9a-f]{24})\)`)

// parseMentions extracts the accounts mentioned in a comment, in the order they
// appear, deduped, and capped at maxMentionsPerComment.
//
// This is purely syntactic — it reports who the text CLAIMS to mention. Whether
// those ids resolve to real accounts, and whether this author was allowed to
// mention them, is validateMentions' job.
//
// Note the display name inside the token is not checked against the account: it
// is a snapshot for rendering, and a hand-written token can carry the wrong one.
// That is no worse than typing the name in plain prose, but it is why a client
// rendering a mention should prefer the name it resolves from the id.
func parseMentions(text string) []primitive.ObjectID {
	matches := mentionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[primitive.ObjectID]bool, len(matches))
	mentions := make([]primitive.ObjectID, 0, len(matches))
	for _, match := range matches {
		id, err := primitive.ObjectIDFromHex(match[2])
		if err != nil {
			continue // unreachable: the pattern already guarantees 24 hex chars
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		mentions = append(mentions, id)
		if len(mentions) == maxMentionsPerComment {
			break
		}
	}
	return mentions
}

// flattenMentions rewrites the persisted tokens back to what the author typed:
// `@[Tom Foolery](66f…)` becomes `@Tom Foolery`.
//
// Lives here beside the pattern rather than with its caller (the mention emails,
// F8) because it is the inverse of the token format, and the two must change
// together. Anywhere a mention has to appear as plain text — an email body, a
// notification, a thread-title preview — goes through this.
func flattenMentions(text string) string {
	return mentionPattern.ReplaceAllString(text, "@$1")
}

// mentionableUserIds is the set of accounts visible to a guest on one event:
// everyone who has marked availability, signed up, RSVP'd, voted in a poll, or
// written a comment the caller can see.
//
// Pure, and takes its inputs rather than fetching them, so the visibility rule
// is testable as a table. `comments` must ALREADY be filtered through
// visibleComments for the caller — passing the raw list would expose the
// authors of a members-only thread to the guest it is hidden from.
//
// Entries keyed by a typed-in guest name rather than an id are skipped
// throughout: they are legacy rows with no account behind them, so there is
// nobody to mention.
func mentionableUserIds(
	event *models.Event,
	responses []models.EventResponse,
	comments []models.Comment,
) map[primitive.ObjectID]bool {
	ids := make(map[primitive.ObjectID]bool)

	add := func(hex string) {
		if id, ok := objectIdOrNil(hex); ok {
			ids[id] = true
		}
	}

	for _, response := range responses {
		add(response.UserId)
	}

	if event != nil {
		for key, rsvp := range event.Rsvps {
			add(key)
			// Older RSVPs are keyed by name but still carry the account id.
			if rsvp != nil && !rsvp.UserId.IsZero() {
				ids[rsvp.UserId] = true
			}
		}
		for _, poll := range event.Polls {
			for _, option := range poll.Options {
				for key := range option.Votes {
					add(key)
				}
			}
		}
	}

	for _, comment := range comments {
		if comment.IsGuest {
			continue
		}
		add(comment.UserId)
	}

	return ids
}

// guestMentionableIds resolves the visible set for a caller against the
// database: the event's responses plus the comments that caller may read.
//
// Only ever called for a guest-role caller — a member's answer is "the whole
// roll", which needs none of this.
func guestMentionableIds(event *models.Event, user *models.User) map[primitive.ObjectID]bool {
	responses, err := db.GetEventResponses(event.Id.Hex())
	if err != nil {
		responses = nil // best-effort: a narrower set, never a wider one
	}

	var visible []models.Comment
	if comments, err := db.GetComments(event.Id.Hex()); err == nil {
		visible = visibleComments(comments, newCommentViewer(user, event))
	}

	return mentionableUserIds(event, responses, visible)
}

// validateMentions filters the ids parsed out of a comment down to the ones
// this author may actually mention: they must resolve to a live account, and
// for a guest author they must also be visible on this event.
//
// Dropped ids are dropped silently. The comment still posts with its text
// untouched — see the file header for why refusing it would be worse.
func validateMentions(mentions []primitive.ObjectID, event *models.Event, user *models.User) []primitive.ObjectID {
	if len(mentions) == 0 {
		return nil
	}

	// One lookup for the batch: an id with no account behind it (deleted, or
	// invented) is not a mention.
	accounts := db.GetUsersByIds(mentions)

	var visible map[primitive.ObjectID]bool
	if !user.EffectiveRole().CanSeeMembersOnly() {
		visible = guestMentionableIds(event, user)
	}

	kept := make([]primitive.ObjectID, 0, len(mentions))
	for _, id := range mentions {
		if _, exists := accounts[id.Hex()]; !exists {
			continue
		}
		if visible != nil && !visible[id] {
			continue
		}
		kept = append(kept, id)
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// sortMentionables orders the picker alphabetically by the name it displays, so
// the list a client renders is stable between requests (Mongo's natural order
// is not) and scannable. Case-insensitive, with the id as the tiebreaker for
// two people who genuinely share a display name.
func sortMentionables(users []models.User) {
	sort.Slice(users, func(i, j int) bool {
		left, right := strings.ToLower(users[i].DisplayName()), strings.ToLower(users[j].DisplayName())
		if left != right {
			return left < right
		}
		return users[i].Id.Hex() < users[j].Id.Hex()
	})
}

// @Summary Lists the accounts the caller may @mention in this event's discussion
// @Description Members and above get everyone on the roll; guests get only the people already visible to them on this event (respondents, RSVPs, poll voters, and the authors of comments they can read).
// @Tags events
// @Produce json
// @Param eventId path string true "Event ID"
// @Success 200 {array} models.User
// @Router /events/{eventId}/mentionables [get]
func getMentionables(c *gin.Context) {
	event, user, _, ok := loadCommentContext(c)
	if !ok {
		return
	}

	var users []models.User
	if user.EffectiveRole().CanSeeMembersOnly() {
		users = db.GetAllUsers()
	} else {
		ids := guestMentionableIds(event, user)
		idList := make([]primitive.ObjectID, 0, len(ids))
		for id := range ids {
			idList = append(idList, id)
		}
		for _, resolved := range db.GetUsersByIds(idList) {
			users = append(users, resolved)
		}
	}

	sortMentionables(users)

	// Same slimming the event page's comment authors get — identity and picture,
	// nothing else. One endpoint serving both roles is what keeps the visibility
	// rule in a single place.
	mentionables := make([]*models.User, 0, len(users))
	for _, u := range users {
		mentionables = append(mentionables, slimUserForDisplay(u))
	}

	c.JSON(http.StatusOK, mentionables)
}
