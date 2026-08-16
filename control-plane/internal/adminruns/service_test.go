package adminruns

import "testing"

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
	for _, status := range []string{"queued", "assigned", "accepted", "awaiting_approval", "canceling", "completed", "failed", "canceled"} {
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
