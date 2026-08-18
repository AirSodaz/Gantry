package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListAgents(ctx context.Context, actor identity.Principal, category, search, collection string, after *AgentCursor, limit int) (AgentPage, error) {
	if collection == "" {
		collection = "all"
	}
	if collection != "all" && collection != "favorites" && collection != "recent" {
		return AgentPage{}, ErrInvalidInput
	}
	pageLimit := boundedLimit(limit)
	base := `SELECT a.id,a.display_name,a.description,a.category,COALESCE(owner.id,''),COALESCE(owner.display_name,''),COALESCE(p.is_favorite,false),p.last_used_at
		FROM gantry.agents a
		JOIN gantry.agent_deployments d ON d.agent_id=a.id AND d.workspace_id=a.workspace_id AND d.environment_kind='production' AND d.status='active'
		JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1
		JOIN gantry.agent_access_grants g ON g.agent_id=a.id AND g.subject_type='principal' AND g.subject_id=$1 AND g.state='active' AND g.valid_from<=now() AND (g.expires_at IS NULL OR g.expires_at>now())
		JOIN gantry.agent_access_grant_capabilities c ON c.grant_id=g.id AND c.capability='metadata.read'
		LEFT JOIN gantry.principals owner ON owner.id=a.owner_principal_id
		LEFT JOIN gantry.agent_preferences p ON p.principal_id=$1 AND p.workspace_id=a.workspace_id AND p.agent_id=a.id
		WHERE a.organization_id=$2 AND ($3='' OR a.category=$3) AND ($4='' OR a.display_name ILIKE '%' || $4 || '%' OR a.description ILIKE '%' || $4 || '%')`
	var rows pgx.Rows
	var err error
	if collection == "recent" {
		var lastUsedAt *time.Time
		var id string
		if after != nil {
			if after.LastUsedAt == nil {
				return AgentPage{}, ErrInvalidInput
			}
			lastUsedAt, id = after.LastUsedAt, after.ID
		}
		rows, err = s.pool.Query(ctx, base+` AND p.last_used_at IS NOT NULL AND ($5::timestamptz IS NULL OR p.last_used_at<$5 OR (p.last_used_at=$5 AND a.id<$6)) ORDER BY p.last_used_at DESC,a.id DESC LIMIT $7`, actor.ID, actor.OrganizationID, category, search, lastUsedAt, id, pageLimit+1)
	} else {
		var name *string
		var id string
		if after != nil {
			name, id = &after.DisplayName, after.ID
		}
		favoriteOnly := collection == "favorites"
		rows, err = s.pool.Query(ctx, base+` AND (NOT $5 OR COALESCE(p.is_favorite,false)) AND ($6::text IS NULL OR a.display_name>$6 OR (a.display_name=$6 AND a.id>$7)) ORDER BY a.display_name,a.id LIMIT $8`, actor.ID, actor.OrganizationID, category, search, favoriteOnly, name, id, pageLimit+1)
	}
	if err != nil {
		return AgentPage{}, err
	}
	defer rows.Close()
	items := make([]Agent, 0)
	for rows.Next() {
		var item Agent
		var ownerID string
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Description, &item.Category, &ownerID, &item.OwnerName, &item.IsFavorite, &item.LastUsedAt); err != nil {
			return AgentPage{}, err
		}
		applyCatalogDefaults(&item, ownerID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AgentPage{}, err
	}
	page := AgentPage{Items: items, HasMore: len(items) > pageLimit}
	if page.HasMore {
		page.Items = page.Items[:pageLimit]
	}
	return page, nil
}

func applyCatalogDefaults(item *Agent, ownerID string) {
	item.Owner = SupportContact{PrincipalID: ownerID, DisplayName: item.OwnerName}
	item.InputContract = json.RawMessage(`{"type":"object","additionalProperties":true}`)
	item.PublishedMetadata = PublishedMetadata{TypicalInputs: []any{}, ExpectedOutput: map[string]any{"kind": "text", "description": "Agent response."}, CapabilitySummary: item.Description, DataDisclosure: map[string]any{"input_classifications": []string{}, "output_classifications": []string{}, "summary": "Published input and output handling is governed by workspace policy."}, ActionDisclosure: map[string]any{"effect_level": "none", "summary": "May read authorized information to prepare a response.", "approval_behavior": "may_be_requested"}}
	item.Availability = Availability{State: "available"}
}

func (s *Service) List(ctx context.Context, actor identity.Principal, filter ListFilter, after *SessionCursor, limit int) (SessionPage, error) {
	if filter.MyAction != "" && filter.MyAction != "approval" {
		return SessionPage{}, ErrInvalidInput
	}
	var afterUpdatedAt *time.Time
	var afterID string
	if after != nil {
		afterUpdatedAt, afterID = &after.CreatedAt, after.ID
	}
	pageLimit := boundedLimit(limit)
	rows, err := s.pool.Query(ctx, `SELECT s.id FROM gantry.sessions s JOIN gantry.session_members m ON m.session_id=s.id AND m.principal_id=$1 WHERE ($2='' OR s.state=$2) AND ($3='' OR s.mode=$3) AND ($4='' OR s.agent_id=$4) AND ($5='' OR ($5='approval' AND EXISTS (SELECT 1 FROM gantry.runs r JOIN gantry.approval_requests ap ON ap.run_id=r.id WHERE r.session_id=s.id AND ap.status='pending' AND r.requester_principal_id=$1))) AND ($6::timestamptz IS NULL OR s.updated_at>=$6) AND ($7::timestamptz IS NULL OR s.updated_at<$7 OR (s.updated_at=$7 AND s.id<$8)) ORDER BY s.updated_at DESC,s.id DESC LIMIT $9`, actor.ID, filter.State, filter.Mode, filter.AgentID, filter.MyAction, filter.UpdatedAfter, afterUpdatedAt, afterID, pageLimit+1)
	if err != nil {
		return SessionPage{}, err
	}
	defer rows.Close()
	items := make([]Session, 0, pageLimit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return SessionPage{}, err
		}
		item, err := s.get(ctx, actor, id, false)
		if err != nil {
			return SessionPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return SessionPage{}, err
	}
	page := SessionPage{Items: items, HasMore: len(items) > pageLimit}
	if page.HasMore {
		page.Items = page.Items[:pageLimit]
	}
	return page, nil
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, sessionID string) (Session, error) {
	return s.get(ctx, actor, sessionID, true)
}
func (s *Service) get(ctx context.Context, actor identity.Principal, sessionID string, includeMessages bool) (Session, error) {
	item, err := loadSession(ctx, s.pool, actor, sessionID)
	if err != nil {
		return Session{}, err
	}
	item.Members, err = s.listMembers(ctx, actor, sessionID, 100)
	if err != nil {
		return Session{}, err
	}
	if includeMessages {
		item.Messages, err = s.listMessages(ctx, actor, sessionID)
		if err != nil {
			return Session{}, err
		}
	} else {
		item.Messages = []Message{}
	}
	item.Artifacts = []runs.Artifact{}
	if s.artifacts != nil {
		item.Artifacts, err = s.artifacts.ListSessionArtifacts(ctx, actor, sessionID, 100)
		if err != nil {
			return Session{}, err
		}
	}
	return item, nil
}

func (s *Service) ListRuns(ctx context.Context, actor identity.Principal, sessionID string, after *RunCursor, limit int) (RunPage, error) {
	if _, err := loadSession(ctx, s.pool, actor, sessionID); err != nil {
		return RunPage{}, err
	}
	var afterSequence *int
	var afterID string
	if after != nil {
		afterSequence, afterID = &after.SessionSequence, after.ID
	}
	pageLimit := boundedLimit(limit)
	rows, err := s.pool.Query(ctx, `SELECT r.id,r.session_sequence,r.requester_principal_id,r.status,r.status_reason,r.outcome,r.retry_of_run_id,r.created_at,r.started_at,r.completed_at FROM gantry.runs r JOIN gantry.session_members m ON m.session_id=r.session_id WHERE r.session_id=$1 AND m.principal_id=$2 AND ($3::integer IS NULL OR r.session_sequence<$3 OR (r.session_sequence=$3 AND r.id<$4)) ORDER BY r.session_sequence DESC,r.id DESC LIMIT $5`, sessionID, actor.ID, afterSequence, afterID, pageLimit+1)
	if err != nil {
		return RunPage{}, err
	}
	defer rows.Close()
	items := make([]Run, 0)
	for rows.Next() {
		var item Run
		var reason string
		if err := rows.Scan(&item.ID, &item.SessionSequence, &item.RequesterID, &item.Status, &reason, &item.Outcome, &item.RetryOfRunID, &item.CreatedAt, &item.StartedAt, &item.CompletedAt); err != nil {
			return RunPage{}, err
		}
		item.State = publicState(item.Status)
		item.StateReason = reasonProjection(reason, item.State, item.Outcome)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return RunPage{}, err
	}
	page := RunPage{Items: items, HasMore: len(items) > pageLimit}
	if page.HasMore {
		page.Items = page.Items[:pageLimit]
	}
	return page, nil
}

func (s *Service) listMessages(ctx context.Context, actor identity.Principal, sessionID string) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT sm.id,sm.run_id,sm.session_sequence,sm.role,sm.parts,sm.content,sm.created_at,CASE WHEN sm.role='requester' THEN r.requester_principal_id ELSE NULL END FROM gantry.session_messages sm JOIN gantry.session_members m ON m.session_id=sm.session_id LEFT JOIN gantry.runs r ON r.id=sm.run_id WHERE sm.session_id=$1 AND m.principal_id=$2 ORDER BY sm.session_sequence,sm.created_at,sm.id`, sessionID, actor.ID)
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

func (s *Service) GetRun(ctx context.Context, actor identity.Principal, runID string) (SessionRun, error) {
	var result SessionRun
	var reason string
	err := s.pool.QueryRow(ctx, `SELECT r.session_id,r.id,r.session_sequence,r.requester_principal_id,r.status,r.status_reason,r.outcome,r.retry_of_run_id,r.created_at,r.started_at,r.completed_at,r.lease_epoch,r.runner_event_sequence FROM gantry.runs r JOIN gantry.session_members m ON m.session_id=r.session_id WHERE r.id=$1 AND m.principal_id=$2`, runID, actor.ID).Scan(&result.SessionID, &result.Run.ID, &result.Run.SessionSequence, &result.Run.RequesterID, &result.Run.Status, &reason, &result.Run.Outcome, &result.Run.RetryOfRunID, &result.Run.CreatedAt, &result.Run.StartedAt, &result.Run.CompletedAt, &result.Run.LeaseEpoch, &result.Run.AcknowledgedEventSequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionRun{}, ErrNotFound
	}
	if err != nil {
		return SessionRun{}, err
	}
	result.Run.State = publicState(result.Run.Status)
	result.Run.StateReason = reasonProjection(reason, result.Run.State, result.Run.Outcome)
	return result, nil
}

func loadSession(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actor identity.Principal, sessionID string) (Session, error) {
	var item Session
	var run Run
	var reason string
	err := q.QueryRow(ctx, `SELECT s.id,s.owner_principal_id,s.mode,s.agent_id,a.display_name,s.state,s.conversation_revision,s.created_at,s.updated_at,NULLIF((SELECT sm.content FROM gantry.session_messages sm WHERE sm.session_id=s.id AND sm.role='requester' ORDER BY sm.session_sequence,sm.created_at,sm.id LIMIT 1),''),(SELECT count(*) FROM gantry.runs qr WHERE qr.session_id=s.id AND qr.status='queued'),COALESCE(r.id,''),COALESCE(r.session_sequence,0),COALESCE(r.requester_principal_id,''),COALESCE(r.status,''),COALESCE(r.status_reason,''),r.outcome,r.retry_of_run_id,COALESCE(r.created_at,s.created_at),r.started_at,r.completed_at FROM gantry.sessions s JOIN gantry.agents a ON a.id=s.agent_id JOIN gantry.session_members m ON m.session_id=s.id AND m.principal_id=$2 LEFT JOIN LATERAL (SELECT id,session_sequence,requester_principal_id,status,status_reason,outcome,retry_of_run_id,created_at,started_at,completed_at FROM gantry.runs WHERE session_id=s.id AND status IN ('assigned','accepted','awaiting_approval','suspended','canceling') ORDER BY session_sequence LIMIT 1) r ON true WHERE s.id=$1`, sessionID, actor.ID).Scan(&item.ID, &item.OwnerPrincipalID, &item.Mode, &item.Agent.AgentID, &item.Agent.DisplayName, &item.State, &item.ConversationRevision, &item.CreatedAt, &item.UpdatedAt, &item.Title, &item.QueuedRunCount, &run.ID, &run.SessionSequence, &run.RequesterID, &run.Status, &reason, &run.Outcome, &run.RetryOfRunID, &run.CreatedAt, &run.StartedAt, &run.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	item.MyAction = "none"
	if run.ID != "" {
		run.State = publicState(run.Status)
		run.StateReason = reasonProjection(reason, run.State, run.Outcome)
		item.ExecutingRun = &run
	}
	var pending bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gantry.runs r JOIN gantry.approval_requests ap ON ap.run_id=r.id WHERE r.session_id=$1 AND r.requester_principal_id=$2 AND ap.status='pending')`, sessionID, actor.ID).Scan(&pending); err != nil {
		return Session{}, err
	}
	if pending {
		item.MyAction = "approval"
	}
	return item, nil
}

func (s *Service) listMembers(ctx context.Context, actor identity.Principal, sessionID string, limit int) ([]SessionMember, error) {
	rows, err := s.pool.Query(ctx, `SELECT m.principal_id,COALESCE(p.display_name,''),m.role,m.joined_at FROM gantry.session_members m JOIN gantry.session_members mine ON mine.session_id=m.session_id AND mine.principal_id=$2 LEFT JOIN gantry.principals p ON p.id=m.principal_id WHERE m.session_id=$1 ORDER BY m.joined_at,m.principal_id LIMIT $3`, sessionID, actor.ID, boundedLimit(limit))
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
func authorKind(role string) string {
	switch role {
	case "requester":
		return "principal"
	case "agent":
		return "agent"
	case "trigger":
		return "trigger"
	default:
		return "system_summary"
	}
}
func publicState(status string) string {
	switch status {
	case "assigned":
		return "provisioning"
	case "accepted":
		return "running"
	default:
		return status
	}
}
func publicStatus(status string) string { return publicState(status) }
func reasonProjection(reason, state string, outcome *string) *UserFacingReason {
	if outcome != nil && *outcome == "requester_input_required" {
		if strings.TrimSpace(reason) == "" {
			reason = "Provide new instructions to continue this Session."
		}
		return &UserFacingReason{Code: "requester_input_required", Message: reason, NextAction: "provide_input"}
	}
	if reason == "" {
		return nil
	}
	return &UserFacingReason{Code: state, Message: reason, NextAction: "none"}
}
