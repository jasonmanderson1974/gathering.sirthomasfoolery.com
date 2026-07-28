package db

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// Returns an event based on its _id
func GetEventById(eventId string) (*models.Event, error) {
	objectId, err := primitive.ObjectIDFromHex(eventId)
	if err != nil {
		// eventId is malformatted
		return nil, nil
	}
	result := EventsCollection.FindOne(context.Background(), bson.M{
		"$and": bson.A{
			bson.M{"_id": objectId},
			bson.M{
				"$or": bson.A{
					bson.M{"isDeleted": bson.M{"$exists": false}},
					bson.M{"isDeleted": bson.M{"$eq": false}},
				},
			},
		},
	})
	if result.Err() == mongo.ErrNoDocuments {
		// Event does not exist!
		return nil, nil
	}

	// Decode result
	var event models.Event
	if err := result.Decode(&event); err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}

	return &event, nil
}

// Returns an event based on its shortId
func GetEventByShortId(shortEventId string) (*models.Event, error) {
	result := EventsCollection.FindOne(context.Background(), bson.M{
		"$and": bson.A{
			bson.M{"shortId": shortEventId},
			bson.M{
				"$or": bson.A{
					bson.M{"isDeleted": bson.M{"$exists": false}},
					bson.M{"isDeleted": bson.M{"$eq": false}},
				},
			},
		},
	})
	if result.Err() == mongo.ErrNoDocuments {
		// Event does not exist!
		return nil, nil
	}

	// Decode result
	var event models.Event
	if err := result.Decode(&event); err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}

	return &event, nil
}

// Returns an event by either its _id or shortId
func GetEventByEitherId(id string) (*models.Event, error) {
	if len(id) <= 10 {
		return GetEventByShortId(id)
	}

	return GetEventById(id)
}

func GetEventResponses(eventId string) ([]models.EventResponse, error) {
	objectId, err := primitive.ObjectIDFromHex(eventId)
	if err != nil {
		// eventId is malformatted
		return []models.EventResponse{}, nil
	}

	result, err := EventResponsesCollection.Find(context.Background(), bson.M{
		"eventId": objectId,
	})
	if err != nil {
		logger.StdErr.Println(err)
		return []models.EventResponse{}, err
	}
	if result.Err() == mongo.ErrNoDocuments {
		// Event responses do not exist!
		return []models.EventResponse{}, nil
	}

	var eventResponses []models.EventResponse
	if err := result.All(context.Background(), &eventResponses); err != nil {
		logger.StdErr.Println(err)
		return []models.EventResponse{}, err
	}

	return eventResponses, nil
}

// Returns events that have a confirmed gathering time still in the future, a
// reminder enabled, and no reminder yet sent. The reminder scheduler
// (services/reminders) does the lead-time windowing in Go on the returned set.
func GetEventsWithPendingReminders() ([]models.Event, error) {
	now := primitive.NewDateTimeFromTime(time.Now())

	result, err := EventsCollection.Find(context.Background(), bson.M{
		"gatheringReminder.enabled": true,
		"gatheringReminder.sentAt":  bson.M{"$exists": false},
		"scheduledEvent.startDate":  bson.M{"$gt": now},
	})
	if err != nil {
		logger.StdErr.Println(err)
		return []models.Event{}, err
	}

	var events []models.Event
	if err := result.All(context.Background(), &events); err != nil {
		logger.StdErr.Println(err)
		return []models.Event{}, err
	}

	return events, nil
}

// Marks a gathering's reminder as sent so it is never re-sent. Idempotent.
func MarkGatheringReminderSent(eventId primitive.ObjectID, sentAt primitive.DateTime) error {
	_, err := EventsCollection.UpdateOne(context.Background(),
		bson.M{"_id": eventId},
		bson.M{"$set": bson.M{"gatheringReminder.sentAt": sentAt}},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// --- Scoped event writes (A16) ----------------------------------------------
//
// These exist so a handler can persist the one thing it changed instead of
// writing the whole document back. `$set: event` looks harmless but re-writes
// every field from a snapshot taken at the top of the handler, so two people
// acting at once silently undo each other: a reminder goes out, everyone opens
// the event, and the last write wins for responses, RSVPs, polls and comments
// alike.
//
// Where a field is a counter or a guard, these use an atomic operator or a
// compare-and-set rather than a read-modify-write, so concurrency is handled by
// the database instead of by luck.

// IncrementNumResponses adjusts the cached response count by delta ($inc, so
// two concurrent responders both count).
func IncrementNumResponses(eventId primitive.ObjectID, delta int) error {
	_, err := EventsCollection.UpdateByID(context.Background(), eventId,
		bson.M{"$inc": bson.M{"numResponses": delta}},
	)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// signUpResponseKeyIsAddressable reports whether a map key can be written with
// a dotted path. Keys are user ids (safe) or guest-supplied names (arbitrary):
// Mongo cannot address a field containing "." through a path, and a leading "$"
// reads as an operator.
func signUpResponseKeyIsAddressable(key string) bool {
	return key != "" && !strings.Contains(key, ".") && !strings.HasPrefix(key, "$")
}

// SetSignUpResponse writes one member's sign-up response. It targets that key
// alone where the key allows it, so two people claiming different slots don't
// overwrite each other, and falls back to rewriting the map when the key can't
// be expressed as a path.
func SetSignUpResponse(eventId primitive.ObjectID, key string, response *models.SignUpResponse, all map[string]*models.SignUpResponse) error {
	var update bson.M
	if signUpResponseKeyIsAddressable(key) {
		update = bson.M{"$set": bson.M{"signUpResponses." + key: response}}
	} else {
		update = bson.M{"$set": bson.M{"signUpResponses": all}}
	}

	_, err := EventsCollection.UpdateByID(context.Background(), eventId, update)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// DeleteSignUpResponse removes one member's sign-up response, targeting that
// key alone where possible (see SetSignUpResponse).
func DeleteSignUpResponse(eventId primitive.ObjectID, key string, all map[string]*models.SignUpResponse) error {
	var update bson.M
	if signUpResponseKeyIsAddressable(key) {
		update = bson.M{"$unset": bson.M{"signUpResponses." + key: ""}}
	} else {
		update = bson.M{"$set": bson.M{"signUpResponses": all}}
	}

	_, err := EventsCollection.UpdateByID(context.Background(), eventId, update)
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// DisarmSendEmailAfterXResponses flips the threshold to -1, but only if it is
// still the value the caller saw. That compare-and-set is what makes the
// "N people have responded" email fire exactly once: previously the guard was
// set in memory and persisted by the same whole-document write that raced, so
// two simultaneous responses could each believe they were the Nth.
//
// Reports whether this caller won and should send.
func DisarmSendEmailAfterXResponses(eventId primitive.ObjectID, expected int) (bool, error) {
	res, err := EventsCollection.UpdateOne(context.Background(),
		bson.M{"_id": eventId, "sendEmailAfterXResponses": expected},
		bson.M{"$set": bson.M{"sendEmailAfterXResponses": -1}},
	)
	if err != nil {
		logger.StdErr.Println(err)
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

// MarkRemindeeResponded flips one remindee's responded flag, and only if it is
// not already set. Reports whether this caller was the one that flipped it, so
// the "everyone has responded" email can't be sent twice for the same event.
func MarkRemindeeResponded(eventId primitive.ObjectID, email string) (bool, error) {
	res, err := EventsCollection.UpdateOne(context.Background(),
		bson.M{
			"_id": eventId,
			"remindees": bson.M{"$elemMatch": bson.M{
				"email":     email,
				"responded": bson.M{"$ne": true},
			}},
		},
		bson.M{"$set": bson.M{"remindees.$.responded": true}},
	)
	if err != nil {
		logger.StdErr.Println(err)
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

// editableEventFields are the only fields the edit form owns. Everything else
// on an event — responses, RSVPs, polls, comments, the scheduled time, nudge
// bookkeeping — belongs to other handlers, and an edit has no business writing
// them back from a snapshot it loaded seconds earlier.
var editableEventFields = []string{
	"name", "description", "location", "duration", "dates", "times",
	"hasSpecificTimes", "wholeBlockSelection", "signUpBlocks", "startOnMonday",
	"notificationsEnabled", "blindAvailabilityEnabled", "daysOnly",
	"sendEmailAfterXResponses", "collectEmails", "type", "remindees",
}

// UpdateEditableEventFields persists an edit, touching only the fields above.
//
// It marshals the event and filters, rather than listing values by hand,
// because every one of those fields is `omitempty`: under the old whole-document
// `$set` a nil pointer was omitted and kept its stored value, and naming the
// fields explicitly would instead write nulls over them. Filtering the marshalled
// document preserves that behaviour exactly while narrowing what's written.
func UpdateEditableEventFields(event *models.Event) error {
	raw, err := bson.Marshal(event)
	if err != nil {
		logger.StdErr.Println(err)
		return err
	}
	var all bson.M
	if err := bson.Unmarshal(raw, &all); err != nil {
		logger.StdErr.Println(err)
		return err
	}

	set := bson.M{}
	for _, field := range editableEventFields {
		if v, ok := all[field]; ok {
			set[field] = v
		}
	}
	if len(set) == 0 {
		return nil
	}

	_, err = EventsCollection.UpdateByID(context.Background(), event.Id, bson.M{"$set": set})
	if err != nil {
		logger.StdErr.Println(err)
	}
	return err
}

// GetEventsWithPendingRemindeeNudges returns live, not-yet-scheduled events
// that still have at least one remindee who hasn't responded and hasn't had all
// their nudges. The scheduler (services/reminders) works out which stage is due
// in Go on the returned set.
//
// Once a gathering has a confirmed time there is nothing left to nudge for, so
// those are excluded here rather than filtered later.
func GetEventsWithPendingRemindeeNudges() ([]models.Event, error) {
	result, err := EventsCollection.Find(context.Background(), bson.M{
		"isDeleted":      bson.M{"$ne": true},
		"scheduledEvent": bson.M{"$exists": false},
		"remindees": bson.M{"$elemMatch": bson.M{
			"responded": bson.M{"$ne": true},
			// nil matches documents where the field was never written (it is
			// omitempty, so stage 0 is absent rather than 0).
			"nudgeStage": bson.M{"$in": bson.A{0, 1, 2, nil}},
		}},
	})
	if err != nil {
		logger.StdErr.Println(err)
		return []models.Event{}, err
	}

	var events []models.Event
	if err := result.All(context.Background(), &events); err != nil {
		logger.StdErr.Println(err)
		return []models.Event{}, err
	}

	return events, nil
}

// MarkRemindeeNudged advances one remindee's nudge stage, but only if it is
// still at expectedStage. That compare-and-set is what stops two overlapping
// ticks from both deciding the same nudge is due and sending it twice. Reports
// whether it actually advanced.
func MarkRemindeeNudged(eventId primitive.ObjectID, email string, expectedStage, newStage int, sentAt primitive.DateTime) (bool, error) {
	// Stage 0 is omitempty, so "still at 0" means 0 or absent.
	var stageMatch interface{} = expectedStage
	if expectedStage == 0 {
		stageMatch = bson.M{"$in": bson.A{0, nil}}
	}

	res, err := EventsCollection.UpdateOne(context.Background(),
		bson.M{
			"_id": eventId,
			"remindees": bson.M{"$elemMatch": bson.M{
				"email":      email,
				"nudgeStage": stageMatch,
			}},
		},
		bson.M{"$set": bson.M{
			"remindees.$.nudgeStage":   newStage,
			"remindees.$.lastNudgedAt": sentAt,
		}},
	)
	if err != nil {
		logger.StdErr.Println(err)
		return false, err
	}
	return res.ModifiedCount == 1, nil
}

// GetRecurringGatheringsToAdvance returns recurring gatherings whose current
// occurrence has already ENDED (so it's safe to roll forward without disturbing
// an in-progress gathering). The reminder scheduler computes the next occurrence
// in Go on the returned set (see advanceRecurringGatherings). (C5)
func GetRecurringGatheringsToAdvance(now primitive.DateTime) ([]models.Event, error) {
	result, err := EventsCollection.Find(context.Background(), bson.M{
		"gatheringRecurrence.frequency": bson.M{"$exists": true, "$ne": ""},
		"scheduledEvent.endDate":        bson.M{"$lte": now},
	})
	if err != nil {
		logger.StdErr.Println(err)
		return []models.Event{}, err
	}

	var events []models.Event
	if err := result.All(context.Background(), &events); err != nil {
		logger.StdErr.Println(err)
		return []models.Event{}, err
	}

	return events, nil
}

// AdvanceGathering rolls a recurring gathering forward to its next occurrence:
// sets the new start/end, clears the previous cycle's RSVPs (fresh headcount),
// and re-arms the reminder (unset sentAt). The update is guarded on the expected
// current start so concurrent scheduler ticks can't double-advance — only the
// first wins. Returns whether it actually advanced. (C5)
func AdvanceGathering(eventId primitive.ObjectID, expectedStart, newStart, newEnd primitive.DateTime) (bool, error) {
	res, err := EventsCollection.UpdateOne(context.Background(),
		bson.M{"_id": eventId, "scheduledEvent.startDate": expectedStart},
		bson.M{
			"$set": bson.M{
				"scheduledEvent.startDate": newStart,
				"scheduledEvent.endDate":   newEnd,
			},
			"$unset": bson.M{
				"rsvps":                    "",
				"gatheringReminder.sentAt": "",
			},
		},
	)
	if err != nil {
		logger.StdErr.Println(err)
		return false, err
	}
	return res.ModifiedCount > 0, nil
}

func GetEventsCreatedThisMonth(userId primitive.ObjectID) (int, error) {
	// Get the start of this month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	result, err := EventsCollection.CountDocuments(context.Background(), bson.M{
		"ownerId": userId,
		"_id": bson.M{
			"$gte": primitive.NewObjectIDFromTimestamp(startOfMonth),
		},
	})
	if err != nil {
		logger.StdErr.Println(err)
		return 0, err
	}

	return int(result), nil
}

// shortIdAlphabet omits 0/1/O/I/l so a short id can be read aloud or copied off
// a screen without ambiguity.
const shortIdAlphabet = "23456789ABCDEFabcdef"

// shortIdLength gives 20^5 ≈ 3.2M ids — ample for a club, and short enough to
// share verbally.
const shortIdLength = 5

// shortIdAttempts is how many distinct ids to try before giving up. Collisions
// are already vanishingly rare at this scale, so needing more than a couple
// means something is wrong with the database rather than with our luck.
const shortIdAttempts = 10

// randomShortId draws a short id from crypto/rand.
//
// Rejection sampling, not `% len(alphabet)`: 256 isn't a multiple of 20, so the
// modulo would make the first 16 letters measurably likelier than the last 4.
func randomShortId() (string, error) {
	max := byte(256 - (256 % len(shortIdAlphabet))) // 240
	id := make([]byte, 0, shortIdLength)
	buf := make([]byte, 1)

	for len(id) < shortIdLength {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		if buf[0] >= max {
			continue // would bias the draw — take another byte
		}
		id = append(id, shortIdAlphabet[int(buf[0])%len(shortIdAlphabet)])
	}
	return string(id), nil
}

// GenerateShortEventId returns a short id that no event is currently using.
//
// It used to seed math/rand with the event's ObjectID timestamp, which is only
// second-granular: two events created in the same second drew the SAME sequence,
// and any id was predictable from a known creation time. It also swallowed the
// lookup errors — so a transient database blip read as "no collision" — and,
// after five failed attempts, logged a line and returned the colliding id
// anyway, leaving two events sharing one short id and lookups serving whichever
// came first.
//
// Now: crypto/rand, propagated probe errors, and a hard failure rather than a
// knowingly duplicate id.
func GenerateShortEventId() (string, error) {
	for i := 0; i < shortIdAttempts; i++ {
		id, err := randomShortId()
		if err != nil {
			logger.StdErr.Println("short id generation failed:", err)
			return "", err
		}

		existing, err := GetEventByShortId(id)
		if err != nil {
			// Don't treat a failed lookup as proof the id is free.
			logger.StdErr.Println("short id collision check failed:", err)
			return "", err
		}
		if existing == nil {
			return id, nil
		}
	}

	err := fmt.Errorf("could not generate a unique short event id in %d attempts", shortIdAttempts)
	logger.StdErr.Println(err)
	return "", err
}

// Updates the name of a guest response
func UpdateGuestResponseName(eventId string, oldName string, newName string) {
	objectId, err := primitive.ObjectIDFromHex(eventId)
	if err != nil {
		// eventId is malformatted
		return
	}

	_, err = EventResponsesCollection.UpdateOne(context.Background(), bson.M{
		"eventId": objectId,
		"userId":  oldName,
	}, bson.M{
		"$set": bson.M{
			"userId":        newName,
			"response.name": newName,
		},
	})
	if err != nil {
		logger.StdErr.Println(err)
	}
}

// Checks if a guest name already exists for an event
// Returns true if the guest name already exists, false otherwise
// Also returns true if the name matches a logged-in user's ObjectID (to prevent conflicts)
// Only checks for guest users (non-logged-in users), not logged-in users
func GuestNameExists(eventId string, guestName string) bool {
	event, _ := GetEventByEitherId(eventId)
	if event == nil {
		return false
	}

	// Check if the name is a valid ObjectID that corresponds to an existing user
	// If so, block it to prevent conflicts
	//NOTE: we're checking against ALL logged in users because in case we allowed this, and a user with an account tried to
	// submit their availability, overwriting would happen and we'd lose data.
	objectId, err := primitive.ObjectIDFromHex(guestName)
	if err == nil {
		// The name is a valid ObjectID format - check if a user exists with this ID
		user, _ := GetUserById(objectId.Hex())
		if user != nil {
			// A logged-in user exists with this ObjectID, so block it
			return true
		}
	}

	// For events, check EventResponsesCollection
	eventObjectId, err := primitive.ObjectIDFromHex(event.Id.Hex())
	if err != nil {
		return false
	}

	// Check if a response exists with this userId AND it's a guest (userId is not a valid ObjectID)
	result := EventResponsesCollection.FindOne(context.Background(), bson.M{
		"eventId": eventObjectId,
		"userId":  guestName,
	})

	if result.Err() == mongo.ErrNoDocuments {
		// No response found with this userId
		return false
	}

	// Response found - verify it's a guest (userId is not a valid ObjectID)
	_, err = primitive.ObjectIDFromHex(guestName)
	if err != nil {
		// userId cannot be parsed as ObjectID, so it's a guest
		return true
	}

	// userId can be parsed as ObjectID, so it's a logged-in user, not a guest
	return false
}
