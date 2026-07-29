package routes

import (
	"context"
	"net/http"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/db"
	"sirtom/server/models"
)

// insertNicknameTestUser creates a throwaway account and returns it with a
// cleanup already registered.
func insertNicknameTestUser(t *testing.T, nickname string) *models.User {
	t.Helper()
	user := &models.User{
		Id:        primitive.NewObjectID(),
		Email:     "nickname-test-" + primitive.NewObjectID().Hex() + "@example.test",
		FirstName: "Bartholomew",
		LastName:  "Fitzwilliam",
		Nickname:  nickname,
	}
	if _, err := db.UsersCollection.InsertOne(context.Background(), user); err != nil {
		t.Fatalf("inserting test user: %v", err)
	}
	t.Cleanup(func() {
		db.UsersCollection.DeleteOne(context.Background(), bson.M{"_id": user.Id})
	})
	return user
}

func storedNickname(t *testing.T, id primitive.ObjectID) (string, bool) {
	t.Helper()
	var doc bson.M
	if err := db.UsersCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&doc); err != nil {
		t.Fatalf("re-reading test user: %v", err)
	}
	raw, present := doc["nickname"]
	if !present {
		return "", false
	}
	s, _ := raw.(string)
	return s, true
}

func TestUpdateNickname_Set(t *testing.T) {
	requireDB(t)
	user := insertNicknameTestUser(t, "")

	c, w := newAdminTestContext(`{"nickname":"  Bart  "}`, user)
	updateNickname(c)

	if w.Code != http.StatusOK {
		t.Fatalf("setting a nickname: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	got, present := storedNickname(t, user.Id)
	if !present || got != "Bart" {
		t.Errorf("stored nickname = %q (present=%v), want %q — the handler should trim", got, present, "Bart")
	}
}

// Clearing has to remove the field, not store "". Both read back as "no
// nickname" through the model, but only $unset keeps the documents honest.
func TestUpdateNickname_ClearUnsetsTheField(t *testing.T) {
	requireDB(t)
	user := insertNicknameTestUser(t, "Bart")

	c, w := newAdminTestContext(`{"nickname":""}`, user)
	updateNickname(c)

	if w.Code != http.StatusOK {
		t.Fatalf("clearing a nickname: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if got, present := storedNickname(t, user.Id); present {
		t.Errorf("nickname still present as %q after clearing; expected $unset", got)
	}
}

// Whitespace-only is a clear, not a nickname made of spaces.
func TestUpdateNickname_WhitespaceClears(t *testing.T) {
	requireDB(t)
	user := insertNicknameTestUser(t, "Bart")

	c, w := newAdminTestContext(`{"nickname":"   "}`, user)
	updateNickname(c)

	if w.Code != http.StatusOK {
		t.Fatalf("whitespace nickname: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	if got, present := storedNickname(t, user.Id); present {
		t.Errorf("nickname stored as %q; whitespace should clear it", got)
	}
}

// The cap bounds RUNES: a 40-emoji nickname is 40 characters to the user even
// though it is 160 bytes, and must not be cut mid-character.
func TestUpdateNickname_TruncatesToRuneCap(t *testing.T) {
	requireDB(t)
	user := insertNicknameTestUser(t, "")

	long := ""
	for i := 0; i < nicknameMaxRunes+10; i++ {
		long += "🎲"
	}
	c, w := newAdminTestContext(`{"nickname":"`+long+`"}`, user)
	updateNickname(c)

	if w.Code != http.StatusOK {
		t.Fatalf("long nickname: got %d, want 200 (%s)", w.Code, w.Body.String())
	}
	got, _ := storedNickname(t, user.Id)
	if runes := []rune(got); len(runes) != nicknameMaxRunes {
		t.Errorf("stored %d runes, want %d", len(runes), nicknameMaxRunes)
	}
	if got != string([]rune(long)[:nicknameMaxRunes]) {
		t.Errorf("truncation split a character: %q", got)
	}
}

// The whole point of the field: DisplayName has to reflect what was stored.
func TestUpdateNickname_FlowsThroughDisplayName(t *testing.T) {
	requireDB(t)
	user := insertNicknameTestUser(t, "")

	c, _ := newAdminTestContext(`{"nickname":"Bart"}`, user)
	updateNickname(c)

	stored, err := db.GetUserById(user.Id.Hex())
	if err != nil || stored == nil {
		t.Fatalf("re-reading user: %v", err)
	}
	if got := stored.DisplayName(); got != "Bart" {
		t.Errorf("DisplayName() = %q, want %q", got, "Bart")
	}
}
