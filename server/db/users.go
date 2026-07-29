package db

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sirtom/server/logger"
	"sirtom/server/models"
)

// Returns a user based on their _id
func GetUserById(userId string) (*models.User, error) {
	objectId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		// userId is malformatted
		return nil, nil
	}
	result := UsersCollection.FindOne(context.Background(), bson.M{
		"_id": objectId,
	})
	if result.Err() == mongo.ErrNoDocuments {
		// User does not exist!
		return nil, nil
	}

	// Decode result
	var user models.User
	if err := result.Decode(&user); err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}

	return &user, nil
}

// SetUserCalendarAccounts replaces just the calendarAccounts field.
//
// Its three callers — addCalendarAccount, getCalendars and
// RefreshUserTokenIfNecessary — each change one thing about the calendar
// accounts and used to persist it with `$set: user`, the whole document. That
// is the lost-update pattern A16 fixed for events, still live on users: a
// calendar fetch that started before the member renamed themselves would write
// the stale name back over the new one on completion. Scoping the write also
// keeps the blast radius of the B7 token encryption to the field that holds the
// tokens.
//
// Whole-field rather than a dotted path to one account, deliberately: the map
// key is `email_TYPE` and emails contain dots, so Mongo would read
// `calendarAccounts.a.b@c.com_google` as four levels of nesting.
func SetUserCalendarAccounts(userId primitive.ObjectID, accounts map[string]models.CalendarAccount) error {
	_, err := UsersCollection.UpdateByID(context.Background(), userId, bson.M{
		"$set": bson.M{"calendarAccounts": accounts},
	})
	return err
}

func SetUserRole(email string, role models.Role) (int64, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return 0, nil
	}
	opts := options.Update().SetCollation(&options.Collation{
		Locale:   "en",
		Strength: 2, // case-insensitive match on email
	})
	res, err := UsersCollection.UpdateOne(
		context.Background(),
		bson.M{"email": e},
		bson.M{"$set": bson.M{"role": models.NormalizeRole(role)}},
		opts,
	)
	if err != nil {
		return 0, err
	}
	return res.MatchedCount, nil
}

// UserProfileUpdate carries the profile fields an admin may edit on someone
// else's behalf. Every field is a pointer so "leave this alone" is
// distinguishable from "set this to empty" — a PATCH that names only a
// nickname must not blank out the name.
//
// Nickname additionally treats a pointer to "" as *clear it*, matching the
// self-serve /user/nickname route: the field is bson-omitempty, so storing ""
// would round-trip as absent anyway, and $unset keeps the documents honest.
type UserProfileUpdate struct {
	FirstName *string
	LastName  *string
	Nickname  *string
}

// UpdateUserProfileByEmail applies a partial profile update to the account with
// the given email, returning how many documents matched (0 = no such account).
//
// Case-insensitive on email via the same collation SetUserRole uses: the
// allowlist stores whatever an admin typed, so the lookup cannot assume the
// stored casing matches.
func UpdateUserProfileByEmail(email string, fields UserProfileUpdate) (int64, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return 0, nil
	}

	set := bson.M{}
	unset := bson.M{}
	if fields.FirstName != nil {
		set["firstName"] = *fields.FirstName
	}
	if fields.LastName != nil {
		set["lastName"] = *fields.LastName
	}
	// A name edit is a deliberate one, so it pins the name against the
	// overwrite that a calendar-account sync would otherwise do.
	if fields.FirstName != nil || fields.LastName != nil {
		set["hasCustomName"] = true
	}
	if fields.Nickname != nil {
		if *fields.Nickname == "" {
			unset["nickname"] = ""
		} else {
			set["nickname"] = *fields.Nickname
		}
	}

	update := bson.M{}
	if len(set) > 0 {
		update["$set"] = set
	}
	if len(unset) > 0 {
		update["$unset"] = unset
	}
	// Mongo rejects an empty update document, and "change nothing" is a
	// legitimate request (the UI can submit an untouched form). Report it as a
	// match without a write — but only after confirming the account exists, so
	// the caller's 400-on-no-account check still fires.
	if len(update) == 0 {
		user, err := GetUserByEmail(e)
		if err != nil || user == nil {
			return 0, err
		}
		return 1, nil
	}

	opts := options.Update().SetCollation(&options.Collation{
		Locale:   "en",
		Strength: 2, // case-insensitive match on email
	})
	res, err := UsersCollection.UpdateOne(
		context.Background(),
		bson.M{"email": e},
		update,
		opts,
	)
	if err != nil {
		return 0, err
	}
	return res.MatchedCount, nil
}

// GetUsersByEmails fetches users for the given emails in a single query and
// returns them keyed by lowercased email (case-insensitive match). Avoids N+1
// lookups when enriching a list of allowlist entries.
func GetUsersByEmails(emails []string) map[string]models.User {
	result := make(map[string]models.User)
	if len(emails) == 0 {
		return result
	}
	lowered := make([]string, 0, len(emails))
	for _, e := range emails {
		lowered = append(lowered, strings.ToLower(strings.TrimSpace(e)))
	}

	opts := options.Find().SetCollation(&options.Collation{
		Locale:   "en",
		Strength: 2, // case-insensitive match on email
	})
	cursor, err := UsersCollection.Find(context.Background(), bson.M{
		"email": bson.M{"$in": lowered},
	}, opts)
	if err != nil {
		logger.StdErr.Println(err)
		return result
	}

	var users []models.User
	if err := cursor.All(context.Background(), &users); err != nil {
		logger.StdErr.Println(err)
		return result
	}
	for _, u := range users {
		result[strings.ToLower(strings.TrimSpace(u.Email))] = u
	}
	return result
}

// GetUsersByIds fetches users for the given ids in a single query and returns
// them keyed by id hex.
//
// The event page resolves three sets of denormalized author names at read time
// (comments, RSVPs, poll votes) so a nickname change is reflected on rows
// written before it. Doing that per row would be an N+1 on the hottest endpoint
// in the app, hence one $in for the union of all three.
//
// Ids that match no account are simply absent from the map — the caller falls
// back to the stored name snapshot, which is what keeps deleted users and
// legacy guest rows rendering.
func GetUsersByIds(ids []primitive.ObjectID) map[string]models.User {
	result := make(map[string]models.User)
	if len(ids) == 0 {
		return result
	}

	cursor, err := UsersCollection.Find(context.Background(), bson.M{
		"_id": bson.M{"$in": ids},
	})
	if err != nil {
		logger.StdErr.Println(err)
		return result
	}

	var users []models.User
	if err := cursor.All(context.Background(), &users); err != nil {
		logger.StdErr.Println(err)
		return result
	}
	for _, u := range users {
		result[u.Id.Hex()] = u
	}
	return result
}

// GetAllUsers returns every account on the roll, projected down to the identity
// fields needed to render a person in a picker (F7's mentionables endpoint).
//
// Fetching the whole collection is only defensible because this is a ~30–40
// person club and the projection keeps each row tiny; the fields deliberately
// left out are the ones a mention picker has no business carrying — email,
// phone, role, and the calendar-account blobs. If the roll ever grows past a
// few hundred, this wants a server-side search parameter instead.
func GetAllUsers() []models.User {
	projection := bson.M{
		"_id": 1, "firstName": 1, "lastName": 1, "nickname": 1,
		"picture": 1, "avatarUpdatedAt": 1,
	}

	cursor, err := UsersCollection.Find(
		context.Background(),
		bson.M{},
		options.Find().SetProjection(projection),
	)
	if err != nil {
		logger.StdErr.Println(err)
		return []models.User{}
	}

	var users []models.User
	if err := cursor.All(context.Background(), &users); err != nil {
		logger.StdErr.Println(err)
		return []models.User{}
	}
	return users
}

func GetUserByEmail(email string) (*models.User, error) {
	emailQuery := strings.TrimSpace(email)
	if emailQuery == "" {
		return nil, nil
	}
	opts := options.FindOne().SetCollation(&options.Collation{
		Locale:   "en",
		Strength: 2, // case-insensitive match on email
	})
	result := UsersCollection.FindOne(context.Background(), bson.M{
		"email": emailQuery,
	}, opts)
	if result.Err() == mongo.ErrNoDocuments {
		// User does not exist!
		return nil, nil
	}

	// Decode result
	var user models.User
	if err := result.Decode(&user); err != nil {
		logger.StdErr.Println(err)
		return nil, err
	}

	return &user, nil
}
