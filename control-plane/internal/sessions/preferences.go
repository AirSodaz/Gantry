package sessions

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

const setAgentFavoriteRoute = "PUT /api/copilot/v1/agents/{agent_id}/favorite"

func (s *Service) SetAgentFavorite(ctx context.Context, actor identity.Principal, agentID, key string, request SetAgentFavoriteRequest) (Agent, error) {
	agentID = strings.TrimSpace(agentID)
	key = strings.TrimSpace(key)
	if agentID == "" || key == "" || len(key) > 256 {
		return Agent{}, ErrInvalidInput
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Agent{}, err
	}
	defer tx.Rollback(ctx)

	item, workspaceID, err := loadCatalogAgent(ctx, tx, actor, agentID, true)
	if err != nil {
		return Agent{}, err
	}
	if err := lockAgentPreferences(ctx, tx, actor.ID, workspaceID); err != nil {
		return Agent{}, err
	}
	digest := requestDigest(agentID, strconv.FormatBool(request.IsFavorite))
	var storedDigest string
	err = tx.QueryRow(ctx, `
		INSERT INTO gantry.agent_preference_command_receipts
			(principal_id,route,idempotency_key,request_digest,workspace_id,agent_id,is_favorite)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT DO NOTHING
		RETURNING request_digest`, actor.ID, setAgentFavoriteRoute, key, digest, workspaceID, agentID, request.IsFavorite).Scan(&storedDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `
			SELECT request_digest
			FROM gantry.agent_preference_command_receipts
			WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3
			FOR UPDATE`, actor.ID, setAgentFavoriteRoute, key).Scan(&storedDigest); err != nil {
			return Agent{}, err
		}
		if storedDigest != digest {
			return Agent{}, ErrIdempotencyConflict
		}
	} else if err != nil {
		return Agent{}, err
	} else {
		if _, err := tx.Exec(ctx, `
			INSERT INTO gantry.agent_preferences (principal_id,workspace_id,agent_id,is_favorite)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (principal_id,workspace_id,agent_id)
			DO UPDATE SET is_favorite=EXCLUDED.is_favorite,updated_at=now()`, actor.ID, workspaceID, agentID, request.IsFavorite); err != nil {
			return Agent{}, err
		}
		if err := pruneRecentAgentPreferences(ctx, tx, actor.ID, workspaceID); err != nil {
			return Agent{}, err
		}
	}

	item, _, err = loadCatalogAgent(ctx, tx, actor, agentID, false)
	if err != nil {
		return Agent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Agent{}, err
	}
	return item, nil
}

func recordRecentAgentUse(ctx context.Context, tx pgx.Tx, principalID, workspaceID, agentID string) error {
	if err := lockAgentPreferences(ctx, tx, principalID, workspaceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO gantry.agent_preferences (principal_id,workspace_id,agent_id,last_used_at)
		VALUES ($1,$2,$3,now())
		ON CONFLICT (principal_id,workspace_id,agent_id)
		DO UPDATE SET last_used_at=EXCLUDED.last_used_at,updated_at=now()`, principalID, workspaceID, agentID); err != nil {
		return err
	}
	return pruneRecentAgentPreferences(ctx, tx, principalID, workspaceID)
}

func lockAgentPreferences(ctx context.Context, tx pgx.Tx, principalID, workspaceID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, agentPreferenceLockKey(principalID, workspaceID))
	return err
}

func agentPreferenceLockKey(principalID, workspaceID string) string {
	return "gantry.agent-preferences:" + strconv.Itoa(len(principalID)) + ":" + principalID + ":" + strconv.Itoa(len(workspaceID)) + ":" + workspaceID
}

func pruneRecentAgentPreferences(ctx context.Context, tx pgx.Tx, principalID, workspaceID string) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM gantry.agent_preferences p
		WHERE p.principal_id=$1 AND p.workspace_id=$2 AND NOT p.is_favorite
		  AND p.agent_id NOT IN (
			SELECT recent.agent_id
			FROM gantry.agent_preferences recent
			WHERE recent.principal_id=$1 AND recent.workspace_id=$2 AND recent.last_used_at IS NOT NULL
			ORDER BY recent.last_used_at DESC,recent.agent_id DESC
			LIMIT 8
		  )`, principalID, workspaceID)
	return err
}

func loadCatalogAgent(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actor identity.Principal, agentID string, lock bool) (Agent, string, error) {
	query := `SELECT a.workspace_id,a.id,a.display_name,a.description,a.category,COALESCE(owner.id,''),COALESCE(owner.display_name,''),COALESCE(p.is_favorite,false),p.last_used_at
		FROM gantry.agents a
		JOIN gantry.agent_deployments d ON d.agent_id=a.id AND d.workspace_id=a.workspace_id AND d.environment_kind='production' AND d.status='active'
		JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1
		JOIN gantry.agent_access_grants g ON g.agent_id=a.id AND g.subject_type='principal' AND g.subject_id=$1 AND g.state='active' AND g.valid_from<=now() AND (g.expires_at IS NULL OR g.expires_at>now())
		JOIN gantry.agent_access_grant_capabilities c ON c.grant_id=g.id AND c.capability='metadata.read'
		LEFT JOIN gantry.principals owner ON owner.id=a.owner_principal_id
		LEFT JOIN gantry.agent_preferences p ON p.principal_id=$1 AND p.workspace_id=a.workspace_id AND p.agent_id=a.id
		WHERE a.id=$2 AND a.organization_id=$3`
	if lock {
		query += ` FOR SHARE OF a,d,m,g,c`
	}
	var item Agent
	var workspaceID, ownerID string
	err := queryer.QueryRow(ctx, query, actor.ID, agentID, actor.OrganizationID).Scan(&workspaceID, &item.ID, &item.DisplayName, &item.Description, &item.Category, &ownerID, &item.OwnerName, &item.IsFavorite, &item.LastUsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, "", ErrNotFound
	}
	if err != nil {
		return Agent{}, "", err
	}
	applyCatalogDefaults(&item, ownerID)
	return item, workspaceID, nil
}
