package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/sessionevents"
	"github.com/jackc/pgx/v5"
)

const (
	addMemberRoute     = "POST /api/copilot/v1/sessions/{session_id}/members"
	updateMemberRoute  = "PATCH /api/copilot/v1/sessions/{session_id}/members/{principal_id}"
	removeMemberRoute  = "DELETE /api/copilot/v1/sessions/{session_id}/members/{principal_id}"
	transferOwnerRoute = "POST /api/copilot/v1/sessions/{session_id}:transfer-owner"
	archiveRoute       = "POST /api/copilot/v1/sessions/{session_id}:archive"
)

func (s *Service) ListMembers(ctx context.Context, actor identity.Principal, sessionID string, after *MemberCursor, limit int) (MemberPage, error) {
	if _, err := loadSession(ctx, s.pool, actor, sessionID); err != nil {
		return MemberPage{}, err
	}
	var joinedAt *time.Time
	var principalID string
	if after != nil {
		joinedAt, principalID = &after.JoinedAt, after.PrincipalID
	}
	pageLimit := boundedLimit(limit)
	rows, err := s.pool.Query(ctx, `SELECT m.principal_id,COALESCE(p.display_name,''),m.role,m.joined_at FROM gantry.session_members m LEFT JOIN gantry.principals p ON p.id=m.principal_id WHERE m.session_id=$1 AND ($2::timestamptz IS NULL OR m.joined_at>$2 OR (m.joined_at=$2 AND m.principal_id>$3)) ORDER BY m.joined_at,m.principal_id LIMIT $4`, sessionID, joinedAt, principalID, pageLimit+1)
	if err != nil {
		return MemberPage{}, err
	}
	defer rows.Close()
	items := make([]SessionMember, 0)
	for rows.Next() {
		var item SessionMember
		if err := rows.Scan(&item.PrincipalID, &item.DisplayName, &item.Role, &item.JoinedAt); err != nil {
			return MemberPage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return MemberPage{}, err
	}
	page := MemberPage{Items: items, HasMore: len(items) > pageLimit}
	if page.HasMore {
		page.Items = page.Items[:pageLimit]
	}
	return page, nil
}

func (s *Service) AddMember(ctx context.Context, actor identity.Principal, sessionID, key string, revision int64, request AddMemberRequest) (Session, error) {
	if request.PrincipalID = strings.TrimSpace(request.PrincipalID); request.PrincipalID == "" || (request.Role != "contributor" && request.Role != "viewer") {
		return Session{}, ErrInvalidInput
	}
	return s.mutateMembers(ctx, actor, sessionID, key, revision, addMemberRoute, "add", request.PrincipalID, request.Role)
}
func (s *Service) UpdateMember(ctx context.Context, actor identity.Principal, sessionID, principalID, key string, revision int64, request UpdateMemberRequest) (Session, error) {
	if request.Role != "contributor" && request.Role != "viewer" {
		return Session{}, ErrInvalidInput
	}
	return s.mutateMembers(ctx, actor, sessionID, key, revision, updateMemberRoute, "update", strings.TrimSpace(principalID), request.Role)
}
func (s *Service) RemoveMember(ctx context.Context, actor identity.Principal, sessionID, principalID, key string, revision int64) (Session, error) {
	return s.mutateMembers(ctx, actor, sessionID, key, revision, removeMemberRoute, "remove", strings.TrimSpace(principalID), "")
}
func (s *Service) Archive(ctx context.Context, actor identity.Principal, sessionID, key string, revision int64) (Session, error) {
	return s.mutateMembers(ctx, actor, sessionID, key, revision, archiveRoute, "archive", "", "")
}
func (s *Service) TransferOwner(ctx context.Context, actor identity.Principal, sessionID, key string, revision int64, request TransferOwnerRequest) (Session, error) {
	return s.mutateMembers(ctx, actor, sessionID, key, revision, transferOwnerRoute, "transfer", strings.TrimSpace(request.NewOwnerPrincipalID), "")
}

func (s *Service) mutateMembers(ctx context.Context, actor identity.Principal, sessionID, key string, revision int64, route, op, principalID, role string) (Session, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 || revision < 1 {
		return Session{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	duplicate, err := reserveSessionCommand(ctx, tx, actor.ID, route, key, requestDigest(sessionID, op+"\n"+principalID+"\n"+role), sessionID)
	if err != nil {
		return Session{}, err
	}
	if duplicate {
		if err := tx.Commit(ctx); err != nil {
			return Session{}, err
		}
		return s.Get(ctx, actor, sessionID)
	}
	var current int64
	var ownerID, workspaceID, agentID, organizationID, state string
	err = tx.QueryRow(ctx, `SELECT s.conversation_revision,s.owner_principal_id,s.workspace_id,s.agent_id,s.organization_id,s.state FROM gantry.sessions s JOIN gantry.session_members m ON m.session_id=s.id AND m.principal_id=$2 WHERE s.id=$1 FOR UPDATE OF s`, sessionID, actor.ID).Scan(&current, &ownerID, &workspaceID, &agentID, &organizationID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	if current != revision {
		return Session{}, ErrConversationChanged
	}
	if ownerID != actor.ID {
		return Session{}, ErrNotFound
	}
	switch op {
	case "add":
		var eligible bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gantry.workspace_memberships WHERE workspace_id=$1 AND principal_id=$2)`, workspaceID, principalID).Scan(&eligible); err != nil {
			return Session{}, err
		}
		if !eligible {
			return Session{}, ErrNotFound
		}
		result, err := tx.Exec(ctx, `INSERT INTO gantry.session_members(session_id,principal_id,role) VALUES($1,$2,$3) ON CONFLICT(session_id,principal_id) DO NOTHING`, sessionID, principalID, role)
		if err != nil {
			return Session{}, err
		}
		if result.RowsAffected() != 1 {
			return Session{}, ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.sessions SET mode='shared' WHERE id=$1`, sessionID); err != nil {
			return Session{}, err
		}
	case "update":
		result, err := tx.Exec(ctx, `UPDATE gantry.session_members SET role=$3 WHERE session_id=$1 AND principal_id=$2 AND role<>'owner'`, sessionID, principalID, role)
		if err != nil {
			return Session{}, err
		}
		if result.RowsAffected() != 1 {
			return Session{}, ErrNotFound
		}
	case "remove":
		result, err := tx.Exec(ctx, `DELETE FROM gantry.session_members WHERE session_id=$1 AND principal_id=$2 AND role<>'owner'`, sessionID, principalID)
		if err != nil {
			return Session{}, err
		}
		if result.RowsAffected() != 1 {
			return Session{}, ErrNotFound
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.sessions SET mode=CASE WHEN mode='channel' THEN 'channel' WHEN EXISTS(SELECT 1 FROM gantry.session_members WHERE session_id=$1 AND role<>'owner') THEN 'shared' ELSE 'personal' END WHERE id=$1`, sessionID); err != nil {
			return Session{}, err
		}
	case "transfer":
		if principalID == "" {
			return Session{}, ErrInvalidInput
		}
		var candidateRole string
		if err := tx.QueryRow(ctx, `SELECT role FROM gantry.session_members WHERE session_id=$1 AND principal_id=$2 FOR UPDATE`, sessionID, principalID).Scan(&candidateRole); errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrNotFound
		} else if err != nil {
			return Session{}, err
		}
		if candidateRole != "contributor" {
			return Session{}, ErrInvalidState
		}
		if err := requireTransferCandidateAccess(ctx, tx, workspaceID, agentID, principalID); err != nil {
			return Session{}, err
		}
		result, err := tx.Exec(ctx, `UPDATE gantry.session_members SET role='contributor' WHERE session_id=$1 AND principal_id=$2 AND role='owner'`, sessionID, ownerID)
		if err != nil {
			return Session{}, err
		}
		if result.RowsAffected() != 1 {
			return Session{}, ErrInvalidState
		}
		result, err = tx.Exec(ctx, `UPDATE gantry.session_members SET role='owner' WHERE session_id=$1 AND principal_id=$2 AND role='contributor'`, sessionID, principalID)
		if err != nil {
			return Session{}, err
		}
		if result.RowsAffected() != 1 {
			return Session{}, ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.sessions SET owner_principal_id=$2 WHERE id=$1`, sessionID, principalID); err != nil {
			return Session{}, err
		}
		if err := appendOwnerTransferEvidence(ctx, tx, actor.ID, organizationID, workspaceID, sessionID, ownerID, principalID); err != nil {
			return Session{}, err
		}
	case "archive":
		if state == "archived" {
			return Session{}, ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.sessions SET state='archived' WHERE id=$1`, sessionID); err != nil {
			return Session{}, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.sessions SET conversation_revision=conversation_revision+1,updated_at=now() WHERE id=$1`, sessionID); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return s.Get(ctx, actor, sessionID)
}

func requireTransferCandidateAccess(ctx context.Context, tx pgx.Tx, workspaceID, agentID, principalID string) error {
	err := tx.QueryRow(ctx, `SELECT 1 FROM gantry.workspace_memberships m
		JOIN gantry.agent_access_grants g ON g.agent_id=$2 AND g.subject_type='principal' AND g.subject_id=$3 AND g.state='active' AND g.valid_from<=now() AND (g.expires_at IS NULL OR g.expires_at>now())
		JOIN gantry.agent_access_grant_capabilities c ON c.grant_id=g.id AND c.capability='metadata.read'
		WHERE m.workspace_id=$1 AND m.principal_id=$3
		FOR SHARE OF m,g,c`, workspaceID, agentID, principalID).Scan(new(int))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func appendOwnerTransferEvidence(ctx context.Context, tx pgx.Tx, actorID, organizationID, workspaceID, sessionID, previousOwnerID, newOwnerID string) error {
	payload := ownerTransferEvidencePayload(workspaceID, previousOwnerID, newOwnerID)
	if err := sessionevents.AppendSession(ctx, tx, sessionID, "session.owner_transferred", payload); err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id,actor_principal_id,resource_type,resource_id,event_type,payload) VALUES ($1,$2,'session',$3,'session.owner_transferred',$4::jsonb)`, organizationID, actorID, sessionID, string(data))
	return err
}

func ownerTransferEvidencePayload(workspaceID, previousOwnerID, newOwnerID string) map[string]string {
	return map[string]string{
		"workspace_id":                workspaceID,
		"previous_owner_principal_id": previousOwnerID,
		"new_owner_principal_id":      newOwnerID,
	}
}
