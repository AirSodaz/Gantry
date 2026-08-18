package copilotapi

import (
	"testing"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/AirSodaz/gantry/internal/sessions"
)

func TestSessionListCursorBindsMemberAndFilters(t *testing.T) {
	handler := Handler{eventKey: []byte("cursor-test-key")}
	createdAt := time.Date(2026, 8, 17, 2, 0, 0, 0, time.UTC)
	filter := sessions.ListFilter{State: "active", Mode: "shared", AgentID: "agt_1", MyAction: "approval"}
	cursor := handler.encodeSessionListCursor(identity.Principal{ID: "prn_1"}, filter, sessions.SessionCursor{UpdatedAt: createdAt, ID: "ses_1"})

	parsed, ok := handler.parseSessionListCursor(cursor, identity.Principal{ID: "prn_1"}, filter)
	if !ok || parsed == nil || !parsed.UpdatedAt.Equal(createdAt) || parsed.ID != "ses_1" {
		t.Fatalf("cursor = %#v, ok = %v", parsed, ok)
	}
	if _, ok := handler.parseSessionListCursor(cursor, identity.Principal{ID: "prn_2"}, filter); ok {
		t.Fatal("cursor was accepted for another requester")
	}
	if _, ok := handler.parseSessionListCursor(cursor, identity.Principal{ID: "prn_1"}, sessions.ListFilter{State: "archived", Mode: "shared", AgentID: "agt_1", MyAction: "approval"}); ok {
		t.Fatal("cursor was accepted for another filter")
	}
}

func TestApprovalAndArtifactCursorsBindTheirScope(t *testing.T) {
	handler := Handler{eventKey: []byte("cursor-test-key")}
	actor := identity.Principal{ID: "prn_1"}
	createdAt := time.Date(2026, 8, 17, 2, 0, 0, 0, time.UTC)

	approvalCursor := handler.encodeApprovalListCursor(actor, "pending", approvals.Cursor{CreatedAt: createdAt, ID: "apr_1"})
	if _, ok := handler.parseApprovalListCursor(approvalCursor, identity.Principal{ID: "prn_2"}, "pending"); ok {
		t.Fatal("approval cursor was accepted for another requester")
	}
	if _, ok := handler.parseApprovalListCursor(approvalCursor, actor, "rejected"); ok {
		t.Fatal("approval cursor was accepted for another state")
	}

	artifactCursor := handler.encodeArtifactListCursor(actor, "ses_1", "internal", "available", runs.ArtifactCursor{CreatedAt: createdAt, ID: "art_1"})
	if _, ok := handler.parseArtifactListCursor(artifactCursor, actor, "ses_1", "confidential", "available"); ok {
		t.Fatal("artifact cursor was accepted for another filter")
	}
	if _, ok := handler.parseArtifactListCursor(artifactCursor, actor, "ses_1", "internal", "expired"); ok {
		t.Fatal("artifact cursor was accepted for another state")
	}
}

func TestRunListCursorBindsRequesterAndSession(t *testing.T) {
	handler := Handler{eventKey: []byte("cursor-test-key")}
	actor := identity.Principal{ID: "prn_1"}
	cursor := handler.encodeRunListCursor(actor, "ses_1", sessions.RunCursor{SessionSequence: 4, ID: "run_4"})

	parsed, ok := handler.parseRunListCursor(cursor, actor, "ses_1")
	if !ok || parsed == nil || parsed.SessionSequence != 4 || parsed.ID != "run_4" {
		t.Fatalf("cursor = %#v, ok = %v", parsed, ok)
	}
	if _, ok := handler.parseRunListCursor(cursor, identity.Principal{ID: "prn_2"}, "ses_1"); ok {
		t.Fatal("run cursor was accepted for another requester")
	}
	if _, ok := handler.parseRunListCursor(cursor, actor, "ses_2"); ok {
		t.Fatal("run cursor was accepted for another session")
	}
}
