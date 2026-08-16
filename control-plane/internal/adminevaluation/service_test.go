package adminevaluation

import (
	"encoding/json"
	"testing"
)

func TestNormalizeCaseRequiresTypedFixtureAndAssertions(t *testing.T) {
	item, err := normalizeCase(CreateCaseRequest{
		Input:           json.RawMessage(`{"prompt":"hello"}`),
		FixtureManifest: json.RawMessage(`{"files":[]}`),
		Assertions:      json.RawMessage(`[{"type":"contains","value":"hello"}]`),
	})
	if err != nil || string(item.Input) != `{"prompt":"hello"}` {
		t.Fatalf("item=%+v err=%v", item, err)
	}
	if _, err := normalizeCase(CreateCaseRequest{Input: json.RawMessage(`{}`), FixtureManifest: json.RawMessage(`{}`)}); err != ErrInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestDigestJSONIsStableForTypedMaps(t *testing.T) {
	left := digestJSON(map[string]any{"b": 2, "a": 1})
	right := digestJSON(map[string]any{"a": 1, "b": 2})
	if left != right {
		t.Fatalf("digest changed with map insertion order: %s != %s", left, right)
	}
}
