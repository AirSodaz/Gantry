package adminpolicy

import (
	"encoding/json"
	"testing"
)

func TestValidateDocumentUsesTypedSchema(t *testing.T) {
	valid, canonical, err := validateDocument("approval", json.RawMessage(`{"rules":[],"kind":"approval","default_effect":"deny"}`))
	if err != nil || valid.State != "valid" || string(canonical) != `{"default_effect":"deny","kind":"approval","rules":[]}` {
		t.Fatalf("valid=%+v canonical=%s err=%v", valid, canonical, err)
	}
	if _, _, err := validateDocument("model", json.RawMessage(`{"kind":"model"}`)); err != ErrSchemaInvalid {
		t.Fatalf("expected schema error, got %v", err)
	}
	if _, _, err := validateDocument("approval", json.RawMessage(`{"kind":"model","allowed_routes":[]}`)); err != ErrSchemaInvalid {
		t.Fatalf("expected kind mismatch, got %v", err)
	}
}

func TestPolicyCursorRoundTrip(t *testing.T) {
	value := "pol_123"
	cursor := encodeCursor(value)
	decoded, err := decodeCursor(cursor)
	if err != nil || decoded != value {
		t.Fatalf("cursor=%q decoded=%q err=%v", cursor, decoded, err)
	}
	if _, err := decodeCursor("!"); err != ErrInvalidInput {
		t.Fatalf("expected invalid cursor, got %v", err)
	}
}
