// Read-time resolution of the display names the event page has denormalized.
//
// Three of the event's structures snapshot the author's name at write time:
// comment.authorName, rsvp.name, and each poll option's vote values. That is
// the right shape for storage — a comment written by an account that is later
// deleted still renders — but it means a member who sets a nickname would keep
// appearing under their old name on everything written before the change.
//
// So getEvent re-resolves them: one batched user lookup for the union of all
// three id sets, then the current DisplayName() overwrites the serialized
// snapshot wherever the account still resolves. The stored values are never
// rewritten — they stay as the fallback for ids that no longer resolve
// (deleted accounts) and for entries that were never keyed by an id at all
// (legacy guest rows, which stay exactly as typed).
package routes

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// objectIdOrNil parses a hex id, returning ok=false for anything that isn't
// one. Guest keys are typed-in names, so this is the discriminator between an
// account-keyed entry and a legacy guest entry.
func objectIdOrNil(hex string) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		return primitive.NilObjectID, false
	}
	return id, true
}

// slimUserForDisplay copies only what a client needs to render an author:
// identity and picture. It starts from stripSensitiveUserFields (calendar
// accounts, phone, role) and additionally drops the email — unlike an event
// response, a comment author has no collectEmails path that would ever expose
// one, so there is no reason to serialize it.
func slimUserForDisplay(user models.User) *models.User {
	slim := &models.User{
		Id:              user.Id,
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		Nickname:        user.Nickname,
		Picture:         user.Picture,
		AvatarUpdatedAt: user.AvatarUpdatedAt,
	}
	stripSensitiveUserFields(slim)
	slim.Email = ""
	return slim
}

// eventDisplayNameIds collects the union of account ids referenced by the
// comments, RSVPs and poll votes handed to it — one id set for one query.
func eventDisplayNameIds(
	comments []models.Comment,
	rsvps map[string]*models.Rsvp,
	polls []models.Poll,
) []primitive.ObjectID {
	seen := make(map[primitive.ObjectID]bool)
	ids := make([]primitive.ObjectID, 0, len(comments)+len(rsvps))

	add := func(hex string) {
		id, ok := objectIdOrNil(hex)
		if !ok || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}

	for _, comment := range comments {
		if comment.IsGuest {
			continue
		}
		add(comment.UserId)
	}
	for key := range rsvps {
		add(key)
	}
	for _, poll := range polls {
		for _, option := range poll.Options {
			for key := range option.Votes {
				add(key)
			}
		}
	}
	return ids
}

// attachCommentAuthors resolves each comment's author, overwriting the
// serialized AuthorName with the account's current DisplayName and attaching
// the slim user for the avatar. Comments whose author doesn't resolve keep
// their stored snapshot and get no Author.
func attachCommentAuthors(comments []models.Comment, users map[string]models.User) []models.Comment {
	for i, comment := range comments {
		if comment.IsGuest {
			continue
		}
		user, ok := users[comment.UserId]
		if !ok {
			continue
		}
		comments[i].Author = slimUserForDisplay(user)
		if name := user.DisplayName(); name != "" {
			comments[i].AuthorName = name
		}
	}
	return comments
}

// resolveRsvpNames overwrites each account-keyed RSVP's serialized name with
// the account's current DisplayName. Name-keyed legacy rows are left alone.
func resolveRsvpNames(rsvps map[string]*models.Rsvp, users map[string]models.User) {
	for key, rsvp := range rsvps {
		if rsvp == nil {
			continue
		}
		user, ok := users[key]
		if !ok {
			continue
		}
		if name := user.DisplayName(); name != "" {
			rsvp.Name = name
			rsvps[key] = rsvp
		}
	}
}

// resolvePollVoteNames does the same for the voter roster, whose values are
// display names keyed by responder.
func resolvePollVoteNames(polls []models.Poll, users map[string]models.User) {
	for pollIdx := range polls {
		for optIdx := range polls[pollIdx].Options {
			votes := polls[pollIdx].Options[optIdx].Votes
			for key := range votes {
				user, ok := users[key]
				if !ok {
					continue
				}
				if name := user.DisplayName(); name != "" {
					votes[key] = name
				}
			}
		}
	}
}

// resolveEventDisplayNames applies all three in one pass over one lookup.
// Best-effort: an id that doesn't resolve just keeps its snapshot, and an
// empty id set skips the query entirely.
func resolveEventDisplayNames(event *models.Event) {
	ids := eventDisplayNameIds(event.Comments, event.Rsvps, event.Polls)
	if len(ids) == 0 {
		return
	}
	users := db.GetUsersByIds(ids)
	if len(users) == 0 {
		return
	}
	event.Comments = attachCommentAuthors(event.Comments, users)
	resolveRsvpNames(event.Rsvps, users)
	resolvePollVoteNames(event.Polls, users)
}
