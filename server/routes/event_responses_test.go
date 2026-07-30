package routes

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

func strPtrTest(s string) *string { return &s }

// getResponsesMap keys each EventResponse's Response by its UserId.
func TestGetResponsesMap(t *testing.T) {
	respA := &models.Response{Name: "A"}
	respB := &models.Response{Name: "B"}
	got := getResponsesMap([]models.EventResponse{
		{UserId: "a", Response: respA},
		{UserId: "b", Response: respB},
	})
	if len(got) != 2 {
		t.Fatalf("len: got %d, want 2", len(got))
	}
	if got["a"] != respA || got["b"] != respB {
		t.Fatalf("map did not key responses by userId correctly")
	}
}

func TestGetResponsesMap_Empty(t *testing.T) {
	got := getResponsesMap([]models.EventResponse{})
	if got == nil {
		t.Fatal("expected non-nil empty map")
	}
	if len(got) != 0 {
		t.Fatalf("len: got %d, want 0", len(got))
	}
}

// A duplicate userId should collapse to the last response for that id.
func TestGetResponsesMap_DuplicateUserIdLastWins(t *testing.T) {
	first := &models.Response{Name: "first"}
	last := &models.Response{Name: "last"}
	got := getResponsesMap([]models.EventResponse{
		{UserId: "dup", Response: first},
		{UserId: "dup", Response: last},
	})
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	if got["dup"] != last {
		t.Fatalf("expected last response to win for duplicate userId")
	}
}

// A row whose `response` field is absent unmarshals to a nil pointer. Keying it
// into the map panicked every caller that dereferences the values (F12), so it
// is dropped instead — the surrounding rows still come through.
func TestGetResponsesMap_SkipsNilResponse(t *testing.T) {
	good := &models.Response{Name: "good"}
	got := getResponsesMap([]models.EventResponse{
		{UserId: "legacy", Response: nil},
		{UserId: "good", Response: good},
	})
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1 (the nil row should be dropped)", len(got))
	}
	if _, ok := got["legacy"]; ok {
		t.Fatal("nil response was keyed into the map")
	}
	if got["good"] != good {
		t.Fatal("the non-nil response should still be present")
	}
}

func TestFindResponse_Found(t *testing.T) {
	target := &models.Response{Name: "target"}
	responses := []models.EventResponse{
		{UserId: "x", Response: &models.Response{Name: "x"}},
		{UserId: "y", Response: target},
	}
	i, resp := findResponse(responses, "y")
	if i != 1 {
		t.Fatalf("index: got %d, want 1", i)
	}
	if resp != target {
		t.Fatalf("returned wrong response pointer")
	}
}

func TestFindResponse_NotFound(t *testing.T) {
	i, resp := findResponse([]models.EventResponse{{UserId: "x", Response: &models.Response{}}}, "missing")
	if i != -1 || resp != nil {
		t.Fatalf("not-found: got (%d, %v), want (-1, nil)", i, resp)
	}
}

func TestFindResponse_Empty(t *testing.T) {
	i, resp := findResponse([]models.EventResponse{}, "anything")
	if i != -1 || resp != nil {
		t.Fatalf("empty: got (%d, %v), want (-1, nil)", i, resp)
	}
}

// stripSensitiveUserFields must clear calendar/billing fields plus phone and
// role (never exposed in event responses) while leaving identity fields (name,
// email) intact — email visibility is handled separately by the caller.
func TestStripSensitiveUserFields(t *testing.T) {
	user := &models.User{
		Email:             "user@example.test",
		FirstName:         "First",
		LastName:          "Last",
		Phone:             "+15555550123",
		Role:              models.RoleAdmin,
		CalendarAccounts:  map[string]models.CalendarAccount{"k": {}},
		CalendarOptions:   &models.CalendarOptions{},
		PrimaryAccountKey: strPtrTest("primary"),
	}
	stripSensitiveUserFields(user)

	if user.CalendarAccounts != nil {
		t.Error("CalendarAccounts should be nil after stripping")
	}
	if user.CalendarOptions != nil {
		t.Error("CalendarOptions should be nil after stripping")
	}
	if user.PrimaryAccountKey != nil {
		t.Error("PrimaryAccountKey should be nil after stripping")
	}
	// A respondent's phone number belongs to the Fellowship directory, not to
	// every event viewer; their access level is nobody's business here.
	if user.Phone != "" {
		t.Errorf("Phone should be cleared after stripping, got %q", user.Phone)
	}
	if user.Role != "" {
		t.Errorf("Role should be cleared after stripping, got %q", user.Role)
	}
	// Identity fields must be preserved.
	if user.Email != "user@example.test" || user.FirstName != "First" || user.LastName != "Last" {
		t.Errorf("identity fields must be preserved, got %+v", user)
	}
}

func TestStripSensitiveUserFields_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("stripSensitiveUserFields(nil) panicked: %v", r)
		}
	}()
	stripSensitiveUserFields(nil)
}

// filterResponsesForBlindAvailability encodes who may see whose availability —
// a privacy rule that's easy to regress. Cover the full matrix.
func boolPtrTest(b bool) *bool { return &b }

func threeResponseMap() map[string]*models.Response {
	return map[string]*models.Response{
		"owner": {Name: "owner"},
		"alice": {Name: "alice"},
		"bob":   {Name: "bob"},
	}
}

func TestFilterBlind_Disabled_ReturnsAll(t *testing.T) {
	event := &models.Event{OwnerId: primitive.NewObjectID()} // BlindAvailabilityEnabled nil => off
	got := filterResponsesForBlindAvailability(event, threeResponseMap(), "alice")
	if len(got) != 3 {
		t.Fatalf("blind off: got %d responses, want 3 (all visible)", len(got))
	}
}

func TestFilterBlind_Enabled_OwnerSeesAll(t *testing.T) {
	ownerId := primitive.NewObjectID()
	event := &models.Event{OwnerId: ownerId, BlindAvailabilityEnabled: boolPtrTest(true)}
	got := filterResponsesForBlindAvailability(event, threeResponseMap(), ownerId.Hex())
	if len(got) != 3 {
		t.Fatalf("blind on, owner: got %d, want 3 (owner sees all)", len(got))
	}
}

func TestFilterBlind_Enabled_NonOwnerSeesOnlyOwn(t *testing.T) {
	event := &models.Event{OwnerId: primitive.NewObjectID(), BlindAvailabilityEnabled: boolPtrTest(true)}
	got := filterResponsesForBlindAvailability(event, threeResponseMap(), "alice")
	if len(got) != 1 {
		t.Fatalf("blind on, non-owner: got %d, want 1", len(got))
	}
	if _, ok := got["alice"]; !ok {
		t.Fatal("non-owner must see only their own response")
	}
}

func TestFilterBlind_Enabled_NonOwnerWithoutResponseSeesNothing(t *testing.T) {
	event := &models.Event{OwnerId: primitive.NewObjectID(), BlindAvailabilityEnabled: boolPtrTest(true)}
	got := filterResponsesForBlindAvailability(event, threeResponseMap(), "carol")
	if len(got) != 0 {
		t.Fatalf("blind on, non-owner w/o a response: got %d, want 0", len(got))
	}
}

// No session means nothing visible. Unreachable through the API now (the route
// is gated) but it is the function's safe default, so pin it.
func TestFilterBlind_Enabled_AnonymousSeesNothing(t *testing.T) {
	event := &models.Event{OwnerId: primitive.NewObjectID(), BlindAvailabilityEnabled: boolPtrTest(true)}
	got := filterResponsesForBlindAvailability(event, threeResponseMap(), "")
	if len(got) != 0 {
		t.Fatalf("blind on, anonymous: got %d, want 0 (sees nothing)", len(got))
	}
}

func TestClampGuestCount(t *testing.T) {
	cases := []struct {
		status models.RsvpStatus
		in     int
		want   int
	}{
		{models.RsvpGoing, 2, 2},
		{models.RsvpMaybe, 0, 0},
		{models.RsvpGoing, -3, 0},   // negative -> 0
		{models.RsvpGoing, 999, 20}, // clamped to max
		{models.RsvpNo, 5, 0},       // decliner can't bring guests
	}
	for _, tc := range cases {
		if got := clampGuestCount(tc.status, tc.in); got != tc.want {
			t.Errorf("clampGuestCount(%q, %d) = %d, want %d", tc.status, tc.in, got, tc.want)
		}
	}
}

func TestAssignSignUpBlocks(t *testing.T) {
	cap2 := 2
	blockA := primitive.NewObjectID() // capacity 2
	blockB := primitive.NewObjectID() // unlimited (nil capacity)
	blocks := []models.SignUpBlock{
		{Id: blockA, Capacity: &cap2},
		{Id: blockB},
	}

	// blockA already has 2 confirmed (full) from other users; blockB has 1.
	event := &models.Event{
		SignUpBlocks: &blocks,
		SignUpResponses: map[string]*models.SignUpResponse{
			"u1": {SignUpBlockIds: []primitive.ObjectID{blockA}},
			"u2": {SignUpBlockIds: []primitive.ObjectID{blockA, blockB}},
		},
	}

	// New user wants both blocks: A is full -> waitlist; B unlimited -> confirmed.
	confirmed, waitlisted := assignSignUpBlocks(event, "newuser",
		[]primitive.ObjectID{blockA, blockB})
	if len(confirmed) != 1 || confirmed[0] != blockB {
		t.Errorf("confirmed = %v, want [blockB]", confirmed)
	}
	if len(waitlisted) != 1 || waitlisted[0] != blockA {
		t.Errorf("waitlisted = %v, want [blockA] (full)", waitlisted)
	}

	// A user already confirmed for the full block keeps the spot on re-submit.
	event.SignUpResponses["u1"] = &models.SignUpResponse{SignUpBlockIds: []primitive.ObjectID{blockA}}
	confirmed, waitlisted = assignSignUpBlocks(event, "u1", []primitive.ObjectID{blockA})
	if len(confirmed) != 1 || confirmed[0] != blockA {
		t.Errorf("re-submit of already-confirmed: confirmed = %v, want [blockA]", confirmed)
	}
	if len(waitlisted) != 0 {
		t.Errorf("re-submit of already-confirmed should not waitlist: %v", waitlisted)
	}

	// A brand-new user for the full block gets waitlisted.
	confirmed, waitlisted = assignSignUpBlocks(event, "fresh", []primitive.ObjectID{blockA})
	if len(confirmed) != 0 || len(waitlisted) != 1 {
		t.Errorf("fresh user on full block: confirmed=%v waitlisted=%v, want []/[blockA]", confirmed, waitlisted)
	}
}
