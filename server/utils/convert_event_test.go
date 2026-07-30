package utils

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"sirtom/server/models"
)

func TestConvertEventToOldFormat_KeysByUserId(t *testing.T) {
	respA := &models.Response{Name: "A"}
	respB := &models.Response{Name: "B"}
	event := &models.Event{Id: primitive.NewObjectID()}

	ConvertEventToOldFormat(event, []models.EventResponse{
		{UserId: "a", Response: respA},
		{UserId: "b", Response: respB},
	})

	if len(event.ResponsesMap) != 2 {
		t.Fatalf("len: got %d, want 2", len(event.ResponsesMap))
	}
	if event.ResponsesMap["a"] != respA || event.ResponsesMap["b"] != respB {
		t.Fatal("responses were not keyed by userId correctly")
	}
}

// A row with no `response` field would otherwise serialize as a null that
// clients dereference (F12).
func TestConvertEventToOldFormat_SkipsNilResponse(t *testing.T) {
	good := &models.Response{Name: "good"}
	event := &models.Event{Id: primitive.NewObjectID()}

	ConvertEventToOldFormat(event, []models.EventResponse{
		{UserId: "legacy", Response: nil},
		{UserId: "good", Response: good},
	})

	if len(event.ResponsesMap) != 1 {
		t.Fatalf("len: got %d, want 1 (the nil row should be dropped)", len(event.ResponsesMap))
	}
	if _, ok := event.ResponsesMap["legacy"]; ok {
		t.Fatal("nil response was keyed into ResponsesMap")
	}
	if event.ResponsesMap["good"] != good {
		t.Fatal("the non-nil response should still be present")
	}
}
