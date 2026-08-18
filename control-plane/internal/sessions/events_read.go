package sessions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/jackc/pgx/v5"
)

type Event struct {
	RunID       *string         `json:"run_id"`
	Sequence    uint64          `json:"sequence"`
	RunSequence *uint64         `json:"run_sequence"`
	Type        string          `json:"type"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Payload     json.RawMessage `json:"payload"`
}

type EventPage struct {
	Session     Session
	Runs        []Run
	Approvals   []approvals.Request
	CurrentSeq  uint64
	EarliestSeq uint64
	Events      []Event
}

// Events reads the Session projection and event page from one repeatable
// snapshot. An event can therefore never be observed before the message, Run,
// approval, or Artifact that makes it employee-visible.
func (s *Service) Events(ctx context.Context, actor identity.Principal, sessionID string, after uint64, limit int) (EventPage, error) {
	limit = boundedLimit(limit)
	var page EventPage
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return EventPage{}, err
	}
	defer tx.Rollback(ctx)

	page.Session, err = loadSession(ctx, tx, actor, sessionID)
	if err != nil {
		return EventPage{}, err
	}
	page.Session.Members, err = eventMembers(ctx, tx, actor, sessionID, 100)
	if err != nil {
		return EventPage{}, err
	}
	page.Session.Messages, err = eventMessages(ctx, tx, actor, sessionID)
	if err != nil {
		return EventPage{}, err
	}
	page.Runs, err = eventRuns(ctx, tx, actor, sessionID, 100)
	if err != nil {
		return EventPage{}, err
	}
	page.Session.Artifacts = make([]runs.Artifact, 0)
	if s.artifacts != nil {
		page.Session.Artifacts, err = eventArtifacts(ctx, tx, actor, sessionID, 100)
		if err != nil {
			return EventPage{}, err
		}
	}
	if s.approvals != nil {
		page.Approvals, err = eventApprovals(ctx, tx, actor, sessionID, 100)
		if err != nil {
			return EventPage{}, err
		}
	}
	if err := tx.QueryRow(ctx, `SELECT session_event_sequence FROM gantry.sessions WHERE id=$1`, sessionID).Scan(&page.CurrentSeq); err != nil {
		return EventPage{}, err
	}
	var earliest *uint64
	if err := tx.QueryRow(ctx, `SELECT MIN(event.session_sequence) FROM (
		SELECT e.session_sequence FROM gantry.run_events e JOIN gantry.runs r ON r.id=e.run_id WHERE r.session_id=$1
		UNION ALL
		SELECT e.session_sequence FROM gantry.session_events e WHERE e.session_id=$1
	) event`, sessionID).Scan(&earliest); err != nil {
		return EventPage{}, err
	}
	if earliest != nil {
		page.EarliestSeq = *earliest
	}
	rows, err := tx.Query(ctx, `SELECT event.run_id,event.session_sequence,event.run_sequence,event.event_type,event.created_at,event.payload FROM (
		SELECT e.run_id,e.session_sequence,e.sequence AS run_sequence,e.event_type,e.created_at,e.payload FROM gantry.run_events e JOIN gantry.runs r ON r.id=e.run_id WHERE r.session_id=$1
		UNION ALL
		SELECT NULL::text,e.session_sequence,NULL::bigint,e.event_type,e.created_at,e.payload FROM gantry.session_events e WHERE e.session_id=$1
	) event WHERE event.session_sequence>$2 ORDER BY event.session_sequence LIMIT $3`, sessionID, after, limit)
	if err != nil {
		return EventPage{}, err
	}
	page.Events = make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.RunID, &event.Sequence, &event.RunSequence, &event.Type, &event.OccurredAt, &event.Payload); err != nil {
			return EventPage{}, err
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, err
	}
	rows.Close()
	for index := range page.Events {
		if err := s.hydrateContentSegment(ctx, tx, &page.Events[index]); err != nil {
			return EventPage{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return EventPage{}, err
	}
	return page, nil
}

func eventMembers(ctx context.Context, tx pgx.Tx, actor identity.Principal, sessionID string, limit int) ([]SessionMember, error) {
	rows, err := tx.Query(ctx, `SELECT m.principal_id,COALESCE(p.display_name,''),m.role,m.joined_at FROM gantry.session_members m JOIN gantry.session_members mine ON mine.session_id=m.session_id AND mine.principal_id=$2 LEFT JOIN gantry.principals p ON p.id=m.principal_id WHERE m.session_id=$1 ORDER BY m.joined_at,m.principal_id LIMIT $3`, sessionID, actor.ID, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]SessionMember, 0)
	for rows.Next() {
		var item SessionMember
		if err := rows.Scan(&item.PrincipalID, &item.DisplayName, &item.Role, &item.JoinedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func eventMessages(ctx context.Context, tx pgx.Tx, actor identity.Principal, sessionID string) ([]Message, error) {
	rows, err := tx.Query(ctx, `SELECT sm.id,sm.run_id,sm.session_sequence,sm.role,sm.parts,sm.content,sm.created_at,CASE WHEN sm.role='requester' THEN r.requester_principal_id ELSE NULL END FROM gantry.session_messages sm JOIN gantry.session_members m ON m.session_id=sm.session_id LEFT JOIN gantry.runs r ON r.id=sm.run_id WHERE sm.session_id=$1 AND m.principal_id=$2 ORDER BY sm.session_sequence,sm.created_at,sm.id`, sessionID, actor.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Message, 0)
	for rows.Next() {
		var item Message
		var role string
		var runID, principalID *string
		if err := rows.Scan(&item.ID, &runID, &item.SessionSequence, &role, &item.Parts, &item.Content, &item.CreatedAt, &principalID); err != nil {
			return nil, err
		}
		item.RunID, item.AuthorPrincipalID = runID, principalID
		item.AuthorKind = authorKind(role)
		if len(item.Parts) == 0 || string(item.Parts) == "[]" {
			item.Parts = json.RawMessage(`[{"type":"text","text":` + strconv.Quote(item.Content) + `}]`)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func eventRuns(ctx context.Context, tx pgx.Tx, actor identity.Principal, sessionID string, limit int) ([]Run, error) {
	rows, err := tx.Query(ctx, `SELECT r.id,r.session_sequence,r.requester_principal_id,r.status,r.status_reason,r.outcome,r.retry_of_run_id,r.created_at,r.started_at,r.completed_at FROM gantry.runs r JOIN gantry.session_members m ON m.session_id=r.session_id WHERE r.session_id=$1 AND m.principal_id=$2 ORDER BY r.session_sequence DESC,r.id DESC LIMIT $3`, sessionID, actor.ID, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Run, 0)
	for rows.Next() {
		var item Run
		var reason string
		if err := rows.Scan(&item.ID, &item.SessionSequence, &item.RequesterID, &item.Status, &reason, &item.Outcome, &item.RetryOfRunID, &item.CreatedAt, &item.StartedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		item.State = publicState(item.Status)
		item.StateReason = reasonProjection(reason, item.State, item.Outcome)
		items = append(items, item)
	}
	return items, rows.Err()
}

func eventArtifacts(ctx context.Context, tx pgx.Tx, actor identity.Principal, sessionID string, limit int) ([]runs.Artifact, error) {
	rows, err := tx.Query(ctx, `SELECT ar.id, ar.session_id, ar.run_id, ar.filename, ar.media_type, ar.size_bytes, ar.digest, ar.classification, ar.scan_status, ar.state, ar.created_at FROM gantry.artifacts ar JOIN gantry.session_members m ON m.session_id=ar.session_id WHERE ar.session_id=$1 AND m.principal_id=$2 ORDER BY ar.created_at, ar.id LIMIT $3`, sessionID, actor.ID, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]runs.Artifact, 0)
	for rows.Next() {
		var item runs.Artifact
		if err := rows.Scan(&item.ID, &item.SessionID, &item.RunID, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func eventApprovals(ctx context.Context, tx pgx.Tx, actor identity.Principal, sessionID string, limit int) ([]approvals.Request, error) {
	rows, err := tx.Query(ctx, `SELECT ar.id, ar.run_id, ar.action_id, ar.action_digest, a.revision, ar.action_preview, ar.risk_class, ar.status, ar.requested_by_principal_id, COALESCE(ar.assigned_principal_id,''), ar.expires_at, ar.created_at, a.tool_name, a.operation, a.target, a.effect, s.id, a.policy_version, agent.display_name FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id JOIN gantry.runs r ON r.id=ar.run_id JOIN gantry.sessions s ON s.id=r.session_id JOIN gantry.session_members m ON m.session_id=s.id AND m.principal_id=$2 JOIN gantry.agents agent ON agent.id=s.agent_id WHERE s.id=$1 ORDER BY ar.created_at, ar.id LIMIT $3`, sessionID, actor.ID, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	items := make([]approvals.Request, 0)
	for rows.Next() {
		var item approvals.Request
		var previewJSON []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.ActionID, &item.ActionDigest, &item.Revision, &previewJSON, &item.RiskClass, &item.Status, &item.RequestedBy, &item.AssignedTo, &item.ExpiresAt, &item.CreatedAt, &item.ToolName, &item.Operation, &item.Target, &item.Effect, &item.SessionID, &item.PolicyVersion, &item.AgentDisplayName); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(previewJSON, &item.ActionPreview); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range items {
		var decision approvals.Decision
		err := tx.QueryRow(ctx, `SELECT decision, reason, principal_id, created_at FROM gantry.approval_decisions WHERE approval_id=$1 ORDER BY created_at DESC LIMIT 1`, items[index].ID).Scan(&decision.Decision, &decision.Reason, &decision.DecidedBy, &decision.CreatedAt)
		if err == nil {
			items[index].Decision = &decision
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	return items, nil
}

func (s *Service) hydrateContentSegment(ctx context.Context, tx pgx.Tx, event *Event) error {
	if event.Type != "model.segment" || s.content == nil {
		return nil
	}
	var reference struct {
		SegmentID string `json:"segment_id"`
		MessageID string `json:"message_id"`
		StreamID  string `json:"stream_id"`
		Index     int64  `json:"segment_index"`
	}
	if err := json.Unmarshal(event.Payload, &reference); err != nil || strings.TrimSpace(reference.SegmentID) == "" {
		return ErrInvalidInput
	}
	var objectKey, digest string
	var size int64
	if err := tx.QueryRow(ctx, `SELECT object_key,digest,size_bytes FROM gantry.run_content_segments WHERE id=$1`, reference.SegmentID).Scan(&objectKey, &digest, &size); err != nil {
		return err
	}
	body, err := s.content.Get(ctx, objectKey)
	if err != nil {
		return err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil || int64(len(data)) != size {
		return ErrInvalidInput
	}
	sum := sha256.Sum256(data)
	if digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return ErrInvalidInput
	}
	event.Type = "model.delta"
	event.Payload, _ = json.Marshal(map[string]any{
		"message_id":    reference.MessageID,
		"stream_id":     strings.TrimSpace(reference.StreamID),
		"segment_index": reference.Index,
		"text":          string(data),
	})
	return nil
}
