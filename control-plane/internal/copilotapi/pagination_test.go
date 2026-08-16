package copilotapi

import (
	"testing"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
)

func TestTaskListCursorBindsRequesterAndFilters(t *testing.T) {
	handler := Handler{eventKey: []byte("cursor-test-key")}
	createdAt := time.Date(2026, 8, 17, 2, 0, 0, 0, time.UTC)
	filter := tasks.ListFilter{Status: "running", AgentID: "agt_1", RequesterAction: "approval"}
	cursor := handler.encodeTaskListCursor(identity.Principal{ID: "prn_1"}, filter, tasks.TaskCursor{CreatedAt: createdAt, ID: "tsk_1"})

	parsed, ok := handler.parseTaskListCursor(cursor, identity.Principal{ID: "prn_1"}, filter)
	if !ok || parsed == nil || !parsed.CreatedAt.Equal(createdAt) || parsed.ID != "tsk_1" {
		t.Fatalf("cursor = %#v, ok = %v", parsed, ok)
	}
	if _, ok := handler.parseTaskListCursor(cursor, identity.Principal{ID: "prn_2"}, filter); ok {
		t.Fatal("cursor was accepted for another requester")
	}
	if _, ok := handler.parseTaskListCursor(cursor, identity.Principal{ID: "prn_1"}, tasks.ListFilter{Status: "completed", AgentID: "agt_1", RequesterAction: "approval"}); ok {
		t.Fatal("cursor was accepted for another filter")
	}
}

func TestApprovalAndArtifactCursorsBindTheirScope(t *testing.T) {
	handler := Handler{eventKey: []byte("cursor-test-key")}
	actor := identity.Principal{ID: "prn_1"}
	createdAt := time.Date(2026, 8, 17, 2, 0, 0, 0, time.UTC)

	approvalCursor := handler.encodeApprovalListCursor(actor, approvals.Cursor{CreatedAt: createdAt, ID: "apr_1"})
	if _, ok := handler.parseApprovalListCursor(approvalCursor, identity.Principal{ID: "prn_2"}); ok {
		t.Fatal("approval cursor was accepted for another requester")
	}

	artifactCursor := handler.encodeArtifactListCursor(actor, "tsk_1", "internal", tasks.ArtifactCursor{CreatedAt: createdAt, ID: "art_1"})
	if _, ok := handler.parseArtifactListCursor(artifactCursor, actor, "tsk_1", "confidential"); ok {
		t.Fatal("artifact cursor was accepted for another filter")
	}
}
