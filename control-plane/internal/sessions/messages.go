package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/sessionmessage"
	"github.com/jackc/pgx/v5"
)

const appendMessageRoute = "POST /api/copilot/v1/sessions/{session_id}/messages"

// AppendMessage records a contributor instruction and queues a following Run.
func (s *Service) AppendMessage(ctx context.Context, actor identity.Principal, sessionID, key string, expectedRevision int64, request AppendMessageRequest) (Session, bool, error) {
	key = strings.TrimSpace(key)
	message := strings.TrimSpace(request.Message)
	if key == "" || len(key) > 256 || message == "" {
		return Session{}, false, ErrInvalidInput
	}
	if expectedRevision < 1 {
		return Session{}, false, ErrPreconditionRequired
	}
	attachmentIDs, err := normalizedAttachmentIDs(request.AttachmentIDs)
	if err != nil {
		return Session{}, false, err
	}
	digestInput, err := json.Marshal(struct {
		Message       string   `json:"message"`
		AttachmentIDs []string `json:"attachment_ids"`
	}{Message: message, AttachmentIDs: attachmentIDs})
	if err != nil {
		return Session{}, false, err
	}
	digest := requestDigest(sessionID, string(digestInput))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, false, err
	}
	defer tx.Rollback(ctx)
	execution, err := lockExecutableSession(ctx, tx, actor, sessionID)
	if err != nil {
		return Session{}, false, err
	}
	if execution.ConversationRevision != expectedRevision {
		return Session{}, false, ErrConversationChanged
	}

	var storedDigest, storedSessionID string
	err = tx.QueryRow(ctx, `INSERT INTO gantry.idempotency_tombstones (principal_id, route, idempotency_key, request_digest, session_id) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING request_digest, session_id`, actor.ID, appendMessageRoute, key, digest, sessionID).Scan(&storedDigest, &storedSessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT request_digest, session_id FROM gantry.idempotency_tombstones WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3 FOR UPDATE`, actor.ID, appendMessageRoute, key).Scan(&storedDigest, &storedSessionID); err != nil {
			return Session{}, false, err
		}
		if storedDigest != digest || storedSessionID != sessionID {
			return Session{}, false, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Session{}, false, err
		}
		session, err := s.Get(ctx, actor, sessionID)
		return session, true, err
	}
	if err != nil {
		return Session{}, false, err
	}

	if execution.State != "active" {
		return Session{}, false, ErrInvalidState
	}
	manifestDigest, err := manifestDigestForRevision(ctx, tx, execution.RevisionID)
	if err != nil {
		return Session{}, false, err
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(session_sequence),0)+1 FROM gantry.runs WHERE session_id=$1`, sessionID).Scan(&sequence); err != nil {
		return Session{}, false, err
	}
	runID := newID("run")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.runs (id, session_id, requester_principal_id, agent_revision_id, deployment_id, manifest_digest, input_json, session_sequence, status) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,'queued')`, runID, sessionID, actor.ID, execution.RevisionID, execution.DeploymentID, manifestDigest, string(digestInput), sequence); err != nil {
		return Session{}, false, err
	}
	if err := bindAttachments(ctx, tx, actor, sessionID, execution.WorkspaceID, attachmentIDs); err != nil {
		return Session{}, false, err
	}
	if err := sessionmessage.Append(ctx, tx, sessionID, runID, "requester", sessionmessage.Text(message)); err != nil {
		return Session{}, false, err
	}
	if err := appendEvent(ctx, tx, runID, "run.queued"); err != nil {
		return Session{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, false, err
	}
	session, err := s.Get(ctx, actor, sessionID)
	return session, false, err
}
