package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListAgents(ctx context.Context, actor identity.Principal, category, search string, limit int) ([]Agent, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id, a.display_name, a.description, a.category, COALESCE(owner.display_name, '') FROM gantry.agents a JOIN gantry.agent_deployments d ON d.agent_id=a.id AND d.workspace_id=a.workspace_id AND d.environment_kind='production' AND d.status='active' JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1 LEFT JOIN gantry.principals owner ON owner.id=a.owner_principal_id WHERE a.organization_id=$2 AND ($3='' OR a.category=$3) AND ($4='' OR a.display_name ILIKE '%' || $4 || '%' OR a.description ILIKE '%' || $4 || '%') ORDER BY a.display_name, a.id LIMIT $5`, actor.ID, actor.OrganizationID, category, search, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Agent, 0)
	for rows.Next() {
		var item Agent
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Description, &item.Category, &item.OwnerName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) List(ctx context.Context, actor identity.Principal, filter ListFilter, limit int) ([]Task, error) {
	if filter.RequesterAction != "" && filter.RequesterAction != "approval" && filter.RequesterAction != "input" {
		return nil, ErrInvalidInput
	}
	rows, err := s.pool.Query(ctx, `SELECT t.id, t.agent_id, a.display_name, t.status, r.id, r.status, r.status_reason, t.conversation_revision, t.created_at FROM gantry.tasks t JOIN gantry.agents a ON a.id=t.agent_id JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.requester_principal_id=$1 AND ($2='' OR t.status=$2) AND ($3='' OR t.agent_id=$3) AND ($4='' OR ($4='approval' AND t.status='awaiting_approval') OR ($4='input' AND t.status='awaiting_requester_input')) AND ($5::timestamptz IS NULL OR t.created_at >= $5) ORDER BY t.created_at DESC, t.id DESC LIMIT $6`, actor.ID, filter.Status, filter.AgentID, filter.RequesterAction, filter.CreatedAfter, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.AgentID, &task.AgentDisplayName, &task.Status, &task.CurrentRun.ID, &task.CurrentRun.Status, &task.CurrentRun.Reason, &task.ConversationRevision, &task.CreatedAt); err != nil {
			return nil, err
		}
		task.Status = publicStatus(task.Status)
		task.CurrentRun.Status = publicStatus(task.CurrentRun.Status)
		items = append(items, task)
		if s.store != nil {
			if artifacts, artifactErr := s.ListArtifacts(ctx, actor, task.ID, 100); artifactErr == nil {
				task.Artifacts = artifacts
				items[len(items)-1] = task
			}
		}
	}
	return items, rows.Err()
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, taskID string) (Task, error) {
	task, err := loadTask(ctx, s.pool, actor, taskID)
	if err != nil {
		return Task{}, err
	}
	task.Messages, err = s.listMessages(ctx, actor, taskID)
	if err != nil {
		return Task{}, err
	}
	if s.store != nil {
		task.Artifacts, err = s.ListArtifacts(ctx, actor, taskID, 100)
		if err != nil {
			return Task{}, err
		}
	}
	return task, nil
}

func (s *Service) ListRuns(ctx context.Context, actor identity.Principal, taskID string, limit int) ([]RunAttempt, error) {
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.attempt_number, r.status, r.status_reason, r.created_at, r.started_at, r.completed_at FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.task_id=$1 AND t.requester_principal_id=$2 ORDER BY r.attempt_number DESC LIMIT $3`, taskID, actor.ID, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RunAttempt, 0)
	for rows.Next() {
		var item RunAttempt
		if err := rows.Scan(&item.ID, &item.Attempt, &item.Status, &item.Reason, &item.CreatedAt, &item.StartedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		item.Status = publicStatus(item.Status)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) listMessages(ctx context.Context, actor identity.Principal, taskID string) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT tm.id, COALESCE(tm.run_id, ''), tm.task_sequence, tm.role, tm.parts, tm.content, tm.created_at FROM gantry.task_messages tm JOIN gantry.tasks t ON t.id=tm.task_id WHERE tm.task_id=$1 AND t.requester_principal_id=$2 ORDER BY tm.task_sequence, tm.created_at, tm.id`, taskID, actor.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Message, 0)
	for rows.Next() {
		var item Message
		if err := rows.Scan(&item.ID, &item.RunID, &item.TaskSequence, &item.Role, &item.Parts, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(item.Parts) == 0 || string(item.Parts) == "[]" {
			item.Parts = json.RawMessage(`[{"type":"text","text":` + strconv.Quote(item.Content) + `}]`)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Service) GetRun(ctx context.Context, actor identity.Principal, runID string) (TaskRun, error) {
	var result TaskRun
	err := s.pool.QueryRow(ctx, `SELECT t.id, r.id, r.status, r.status_reason, r.lease_epoch, r.runner_event_sequence FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.id=$1 AND t.requester_principal_id=$2`, runID, actor.ID).Scan(&result.TaskID, &result.Run.ID, &result.Run.Status, &result.Run.Reason, &result.Run.LeaseEpoch, &result.Run.AcknowledgedEventSequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskRun{}, ErrNotFound
	}
	if err != nil {
		return TaskRun{}, err
	}
	result.Run.Status = publicStatus(result.Run.Status)
	return result, nil
}
func loadTask(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actor identity.Principal, taskID string) (Task, error) {
	var task Task
	err := querier.QueryRow(ctx, `SELECT t.id, t.agent_id, a.display_name, t.status, r.id, r.status, r.status_reason, t.conversation_revision, t.created_at FROM gantry.tasks t JOIN gantry.agents a ON a.id=t.agent_id JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2`, taskID, actor.ID).Scan(&task.ID, &task.AgentID, &task.AgentDisplayName, &task.Status, &task.CurrentRun.ID, &task.CurrentRun.Status, &task.CurrentRun.Reason, &task.ConversationRevision, &task.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	task.Status = publicStatus(task.Status)
	task.CurrentRun.Status = publicStatus(task.CurrentRun.Status)
	return task, nil
}
