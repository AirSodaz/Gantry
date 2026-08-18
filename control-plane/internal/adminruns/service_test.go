package adminruns

import (
	"strings"
	"testing"
)

func TestNormalizeOptionsBoundsLimitAndWhitespace(t *testing.T) {
	options := normalizeOptions(ListOptions{WorkspaceID: " ws_1 ", AgentID: " agt_1 ", RevisionHash: " sha256:abc ", Status: " failed ", Limit: 500})
	if options.WorkspaceID != "ws_1" || options.AgentID != "agt_1" || options.RevisionHash != "sha256:abc" || options.Status != "failed" || options.Limit != 100 {
		t.Fatalf("normalized options=%+v", options)
	}
	if normalizeOptions(ListOptions{}).Limit != 50 {
		t.Fatal("default limit should be 50")
	}
}

func TestRunStatusValidationMatchesPersistenceStates(t *testing.T) {
	for _, status := range []string{"queued", "assigned", "accepted", "awaiting_approval", "suspended", "canceling", "completed", "failed", "canceled", "expired"} {
		if !validStatus(status) {
			t.Fatalf("status %q was rejected", status)
		}
	}
	for _, status := range []string{"", "running", "unknown", "approved"} {
		if status != "" && validStatus(status) {
			t.Fatalf("invalid status %q was accepted", status)
		}
	}
}

func TestRunProjectionUsesSessionAndImmutableRunRequester(t *testing.T) {
	for _, fragment := range []string{
		"r.session_id",
		"r.session_sequence",
		"JOIN gantry.sessions t ON t.id=r.session_id",
		"requester.id=r.requester_principal_id",
	} {
		if !strings.Contains(runSelect, fragment) {
			t.Fatalf("run projection does not contain %q", fragment)
		}
	}
	if strings.Contains(runSelect, "gantry.tasks") || strings.Contains(runSelect, "attempt_number") {
		t.Fatalf("run projection retained task-era columns: %s", runSelect)
	}
}
