package copilotapi

import (
	"testing"
	"time"
)

func TestEventCursorIsSignedAndBoundToTaskAndRun(t *testing.T) {
	key := []byte("test-event-key")
	cursor := encodeCursor(key, "tsk_1", "run_1", 42)
	sequence, runID, ok := parseAfterCursor(key, cursor, "tsk_1", "run_1")
	if !ok || sequence != 42 || runID != "run_1" {
		t.Fatalf("valid cursor parsed as sequence=%d run=%q ok=%t", sequence, runID, ok)
	}
	if _, _, ok := parseAfterCursor(key, cursor, "tsk_2", "run_1"); ok {
		t.Fatal("cursor was accepted for another task")
	}
	if _, _, ok := parseAfterCursor(key, cursor, "tsk_1", "run_2"); ok {
		t.Fatal("cursor was accepted for another run")
	}
	if _, _, ok := parseAfterCursor(key, cursor+"x", "tsk_1", "run_1"); ok {
		t.Fatal("tampered cursor was accepted")
	}
}

func TestEventTicketRejectsExpiredClaims(t *testing.T) {
	key := []byte("test-event-key")
	ticket := signPayload(key, "evt", eventTicketClaims{TaskID: "tsk_1", Actor: "principal_1", Expiry: time.Now().Add(-time.Second).Unix()})
	claims, ok := verifyPayload[eventTicketClaims](key, "evt", ticket)
	if !ok || claims.Expiry > time.Now().Unix() {
		t.Fatalf("expired ticket was not decoded as expected: %#v, %t", claims, ok)
	}
}
