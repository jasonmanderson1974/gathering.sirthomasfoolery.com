package routes

import (
	"context"
	"encoding/json"
	"image/color"
	"net/http"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// The permission guards reject before touching Mongo, so those cases need no
// database. Everything that asserts on stored state gates on requireDB.

func TestUpdateMemberProfile_MemberForbidden(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"someone@example.test","nickname":"Pip"}`, member())
	updateMemberProfile(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member editing a profile: got %d, want 403 — /admin is only member+-gated, so this handler check is the real guard", w.Code)
	}
}

func TestUpdateMemberAvatar_MemberForbidden(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"someone@example.test","image":"data:image/png;base64,AAAA"}`, member())
	updateMemberAvatar(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member setting another member's photo: got %d, want 403", w.Code)
	}
}

func TestDeleteMemberAvatar_MemberForbidden(t *testing.T) {
	c, w := newAdminTestContext(`{"email":"someone@example.test"}`, member())
	deleteMemberAvatar(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("member removing another member's photo: got %d, want 403", w.Code)
	}
}

// insertProfileTestUser creates a throwaway account with a known name, and
// cleans up both it and any avatar a test hangs off it.
func insertProfileTestUser(t *testing.T, role models.Role) *models.User {
	t.Helper()
	user := &models.User{
		Id:        primitive.NewObjectID(),
		Email:     "profile-test-" + primitive.NewObjectID().Hex() + "@example.test",
		FirstName: "Barnaby",
		LastName:  "Fitchett",
		Role:      role,
	}
	if _, err := db.UsersCollection.InsertOne(context.Background(), user); err != nil {
		t.Fatalf("inserting test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": user.Id})
		_, _ = db.AvatarsCollection.DeleteOne(context.Background(), bson.M{"_id": user.Id})
	})
	return user
}

// reloadUser re-reads the account so assertions are about what was stored, not
// what the handler said it stored.
func reloadUser(t *testing.T, id primitive.ObjectID) models.User {
	t.Helper()
	var user models.User
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&user); err != nil {
		t.Fatalf("re-reading test user: %v", err)
	}
	return user
}

func TestUpdateMemberProfile_AdminEditsNameAndNickname(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleMember)

	body, _ := json.Marshal(map[string]string{
		"email":     target.Email,
		"firstName": "Barnabas",
		"lastName":  "Fitchett-Rowe",
		"nickname":  "Barny",
	})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("admin editing a member: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	stored := reloadUser(t, target.Id)
	if stored.FirstName != "Barnabas" || stored.LastName != "Fitchett-Rowe" {
		t.Errorf("stored name = %q %q, want %q %q", stored.FirstName, stored.LastName, "Barnabas", "Fitchett-Rowe")
	}
	if stored.Nickname != "Barny" {
		t.Errorf("stored nickname = %q, want %q", stored.Nickname, "Barny")
	}
	// Without this, the next calendar-account sync would overwrite the name the
	// admin just set (routes/auth.go only preserves names when it is true).
	if stored.HasCustomName == nil || !*stored.HasCustomName {
		t.Error("hasCustomName was not set — a sign-in sync would silently undo the admin's name edit")
	}
}

func TestUpdateMemberProfile_OmittedFieldsAreLeftAlone(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleMember)

	body, _ := json.Marshal(map[string]string{"email": target.Email, "nickname": "Fitch"})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("nickname-only edit: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	stored := reloadUser(t, target.Id)
	if stored.FirstName != "Barnaby" || stored.LastName != "Fitchett" {
		t.Errorf("name = %q %q, want it untouched (%q %q) — an omitted field must not blank the record",
			stored.FirstName, stored.LastName, "Barnaby", "Fitchett")
	}
	if stored.Nickname != "Fitch" {
		t.Errorf("nickname = %q, want %q", stored.Nickname, "Fitch")
	}
	// A nickname-only PATCH is not a name edit, so it must not claim one.
	if stored.HasCustomName != nil && *stored.HasCustomName {
		t.Error("hasCustomName was set by a nickname-only edit — that pins a name nobody chose")
	}
}

func TestUpdateMemberProfile_EmptyNicknameClears(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleMember)

	body, _ := json.Marshal(map[string]string{"email": target.Email, "nickname": "Barny"})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("setting a nickname: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	body, _ = json.Marshal(map[string]string{"email": target.Email, "nickname": ""})
	c, w = newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("clearing a nickname: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	if stored := reloadUser(t, target.Id); stored.Nickname != "" {
		t.Errorf("nickname = %q after clearing, want empty", stored.Nickname)
	}
	// $unset rather than storing "" — the field is bson-omitempty, so the
	// document should not carry the key at all.
	var raw bson.M
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": target.Id}).Decode(&raw); err != nil {
		t.Fatalf("re-reading raw document: %v", err)
	}
	if _, present := raw["nickname"]; present {
		t.Error("nickname key survives after clearing — should be $unset, not set to empty")
	}
}

func TestUpdateMemberProfile_BlankNameRejected(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleMember)

	body, _ := json.Marshal(map[string]string{"email": target.Email, "firstName": "   "})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("blanking a name: got %d, want 400 — DisplayName falls back to the name, so erasing it leaves the person nameless", w.Code)
	}
	if stored := reloadUser(t, target.Id); stored.FirstName != "Barnaby" {
		t.Errorf("firstName = %q after a rejected edit, want it untouched", stored.FirstName)
	}
}

func TestUpdateMemberProfile_SuperAdminTargetForbidden(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleSuperAdmin)

	body, _ := json.Marshal(map[string]string{"email": target.Email, "nickname": "Deposed"})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin editing a super admin: got %d, want 403", w.Code)
	}
	if stored := reloadUser(t, target.Id); stored.Nickname != "" {
		t.Errorf("nickname = %q, want the edit to have been refused", stored.Nickname)
	}
}

func TestUpdateMemberProfile_NoAccountRejected(t *testing.T) {
	requireDB(t)
	// An allowlist entry nobody has claimed has no user document to write to.
	body, _ := json.Marshal(map[string]string{
		"email":    "unclaimed-" + primitive.NewObjectID().Hex() + "@example.test",
		"nickname": "Ghost",
	})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("editing an unclaimed invitation: got %d, want 400", w.Code)
	}
}

func TestUpdateMemberProfile_EmailMatchIsCaseInsensitive(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleMember)

	// The allowlist stores whatever an admin typed, so the roll can hand back a
	// differently-cased address than the account holds.
	shouty := strings.ToUpper(target.Email)
	body, _ := json.Marshal(map[string]string{"email": shouty, "nickname": "Casing"})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("editing via an upper-cased email: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if stored := reloadUser(t, target.Id); stored.Nickname != "Casing" {
		t.Errorf("nickname = %q, want the edit to have landed despite the casing", stored.Nickname)
	}
}

func TestMemberAvatar_AdminRoundTripForAnotherUser(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleMember)

	body, _ := json.Marshal(map[string]string{
		"email": target.Email,
		"image": dataURL("image/png", encodePNG(t, 400, 400, color.RGBA{R: 40, G: 90, B: 200, A: 255})),
	})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberAvatar(c)
	if w.Code != http.StatusOK {
		t.Fatalf("admin uploading for another member: got %d, want 200 (%s)", w.Code, w.Body.String())
	}

	// The photo must land on the TARGET, not on the admin doing the uploading.
	if _, present := storedAvatarUpdatedAt(t, target.Id); !present {
		t.Fatal("target has no avatarUpdatedAt — the upload went somewhere else")
	}

	// --- and the same route removes it
	body, _ = json.Marshal(map[string]string{"email": target.Email})
	c, w = newAdminTestContext(string(body), admin())
	deleteMemberAvatar(c)
	if w.Code != http.StatusOK {
		t.Fatalf("admin removing another member's photo: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if _, present := storedAvatarUpdatedAt(t, target.Id); present {
		t.Error("avatarUpdatedAt survives removal — the monogram fallback keys off its absence")
	}
}

func TestUpdateMemberAvatar_SuperAdminTargetForbidden(t *testing.T) {
	requireDB(t)
	target := insertProfileTestUser(t, models.RoleSuperAdmin)

	body, _ := json.Marshal(map[string]string{
		"email": target.Email,
		"image": dataURL("image/png", encodePNG(t, 64, 64, color.RGBA{R: 1, G: 2, B: 3, A: 255})),
	})
	c, w := newAdminTestContext(string(body), admin())
	updateMemberAvatar(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin setting a super admin's photo: got %d, want 403", w.Code)
	}
	if _, present := storedAvatarUpdatedAt(t, target.Id); present {
		t.Error("a super admin's photo was written despite the 403")
	}
}
