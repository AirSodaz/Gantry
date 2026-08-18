package sessions

import (
	"testing"
)

func TestOwnerTransferEvidenceIncludesTheSessionScopeAndBothPrincipals(t *testing.T) {
	payload := ownerTransferEvidencePayload("ws_1", "principal_previous", "principal_new")
	if payload["workspace_id"] != "ws_1" || payload["previous_owner_principal_id"] != "principal_previous" || payload["new_owner_principal_id"] != "principal_new" {
		t.Fatalf("payload=%v", payload)
	}
}
