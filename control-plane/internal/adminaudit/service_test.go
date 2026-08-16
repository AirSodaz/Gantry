package adminaudit

import (
	"encoding/json"
	"testing"
)

func TestCursorRoundTrip(t *testing.T) {
	value := encodeCursor(42)
	if !validCursor(value) || cursorID(value) != 42 {
		t.Fatalf("cursor round trip failed: %q", value)
	}
	if validCursor("not-a-cursor") {
		t.Fatal("invalid cursor accepted")
	}
}

func TestRedactPayloadMarksSensitiveFields(t *testing.T) {
	redacted, fields := redact([]byte(`{"reason":"approved","nested":{"api_token":"secret"}}`))
	var value map[string]any
	if err := json.Unmarshal(redacted, &value); err != nil {
		t.Fatalf("redacted payload is invalid JSON: %v", err)
	}
	nested := value["nested"].(map[string]any)
	if nested["api_token"] != "[REDACTED]" || len(fields) != 1 || fields[0] != "nested.api_token" {
		t.Fatalf("unexpected redaction: payload=%s fields=%v", redacted, fields)
	}
}
