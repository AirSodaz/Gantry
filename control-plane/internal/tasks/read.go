package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListAgents(ctx context.Context, actor identity.Principal, category, search string, after *AgentCursor, limit int) (AgentPage, error) {
	var name *string
	var id string
	if after != nil {
		name, id = &after.DisplayName, after.ID
	}
	pageLimit := boundedLimit(limit)
	rows, err := s.pool.Query(ctx, `SELECT a.id, a.display_name, a.description, a.category, COALESCE(owner.display_name, '') FROM gantry.agents a JOIN gantry.agent_deployments d ON d.agent_id=a.id AND d.workspace_id=a.workspace_id AND d.environment_kind='production' AND d.status='active' JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1 LEFT JOIN gantry.principals owner ON owner.id=a.owner_principal_id WHERE a.organization_id=$2 AND ($3='' OR a.category=$3) AND ($4='' OR a.display_name ILIKE '%' || $4 || '%' OR a.description ILIKE '%' || $4 || '%') AND ($5::text IS NULL OR a.display_name > $5 OR (a.display_name=$5 AND a.id>$6)) ORDER BY a.display_name, a.id LIMIT $7`, actor.ID, actor.OrganizationID, category, search, name, id, pageLimit+1)
	if err != nil {
		return AgentPage{}, err
	}
	defer rows.Close()
	items := make([]Agent, 0)
	for rows.Next() {
		var item Agent
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Description, &item.Category, &item.OwnerName); err != nil {
			return AgentPage{}, err
		}
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

func (s *Service) List(ctx context.Context, actor identity.Principal, filter ListFilter, after *TaskCursor, limit int) (TaskPage, error) {
	if filter.RequesterAction != "" && filter.RequesterAction != "approval" && filter.RequesterAction != "input" {
		return TaskPage{}, ErrInvalidInput
	}
	var afterCreatedAt *time.Time
	var afterID string
	if after != nil {
		afterCreatedAt, afterID = &after.CreatedAt, after.ID
	}
	pageLimit := boundedLimit(limit)
	rows, err := s.pool.Query(ctx, `SELECT t.id, t.requester_principal_id, t.agent_id, a.display_name, t.status, r.id, r.status, r.status_reason, t.conversation_revision, t.created_at, COALESCE((SELECT tm.content FROM gantry.task_messages tm WHERE tm.task_id=t.id AND tm.role='requester' ORDER BY tm.task_sequence, tm.created_at, tm.id LIMIT 1), ''), GREATEST(t.created_at, COALESCE((SELECT MAX(re.created_at) FROM gantry.run_events re JOIN gantry.runs history ON history.id=re.run_id WHERE history.task_id=t.id), t.created_at), COALESCE((SELECT MAX(ar.created_at) FROM gantry.artifacts ar WHERE ar.task_id=t.id), t.created_at)) FROM gantry.tasks t JOIN gantry.agents a ON a.id=t.agent_id JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.requester_principal_id=$1 AND ($2='' OR t.status=$2) AND ($3='' OR t.agent_id=$3) AND ($4='' OR ($4='approval' AND t.status='awaiting_approval') OR ($4='input' AND t.status='awaiting_requester_input')) AND ($5::timestamptz IS NULL OR t.created_at >= $5) AND ($6::timestamptz IS NULL OR t.created_at < $6 OR (t.created_at = $6 AND t.id < $7)) ORDER BY t.created_at DESC, t.id DESC LIMIT $8`, actor.ID, filter.Status, filter.AgentID, filter.RequesterAction, filter.CreatedAfter, afterCreatedAt, afterID, pageLimit+1)
	if err != nil {
		return TaskPage{}, err
	}
	defer rows.Close()
	items := make([]Task, 0)
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.RequesterID, &task.AgentID, &task.AgentDisplayName, &task.Status, &task.CurrentRun.ID, &task.CurrentRun.Status, &task.CurrentRun.Reason, &task.ConversationRevision, &task.CreatedAt, &task.Title, &task.UpdatedAt); err != nil {
			return TaskPage{}, err
		}
		task.Status = publicStatus(task.Status)
		task.RequesterAction = requesterAction(task.Status)
		task.CurrentRun.Status = publicStatus(task.CurrentRun.Status)
		items = append(items, task)
		if s.store != nil {
			if artifacts, artifactErr := s.ListArtifacts(ctx, actor, task.ID, 100); artifactErr == nil {
				task.Artifacts = artifacts
				items[len(items)-1] = task
			}
		}
	}
	if err := rows.Err(); err != nil {
		return TaskPage{}, err
	}
	page := TaskPage{Items: items, HasMore: len(items) > pageLimit}
	if page.HasMore {
		page.Items = page.Items[:pageLimit]
	}
	return page, nil
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

func (s *Service) ListRuns(ctx context.Context, actor identity.Principal, taskID string, after *RunCursor, limit int) (RunPage, error) {
	if _, err := loadTask(ctx, s.pool, actor, taskID); err != nil {
		return RunPage{}, err
	}
	var afterAttempt *int
	var afterID string
	if after != nil {
		afterAttempt, afterID = &after.Attempt, after.ID
	}
	pageLimit := boundedLimit(limit)
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.attempt_number, r.status, r.status_reason, r.created_at, r.started_at, r.completed_at FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.task_id=$1 AND t.requester_principal_id=$2 AND ($3::integer IS NULL OR r.attempt_number < $3 OR (r.attempt_number=$3 AND r.id<$4)) ORDER BY r.attempt_number DESC, r.id DESC LIMIT $5`, taskID, actor.ID, afterAttempt, afterID, pageLimit+1)
	if err != nil {
		return RunPage{}, err
	}
	defer rows.Close()
	items := make([]RunAttempt, 0)
	for rows.Next() {
		var item RunAttempt
		if err := rows.Scan(&item.ID, &item.Attempt, &item.Status, &item.Reason, &item.CreatedAt, &item.StartedAt, &item.CompletedAt); err != nil {
			return RunPage{}, err
		}
		item.Status = publicStatus(item.Status)
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
	err := querier.QueryRow(ctx, `SELECT t.id, t.requester_principal_id, t.agent_id, a.display_name, t.status, r.id, r.status, r.status_reason, t.conversation_revision, t.created_at, COALESCE((SELECT tm.content FROM gantry.task_messages tm WHERE tm.task_id=t.id AND tm.role='requester' ORDER BY tm.task_sequence, tm.created_at, tm.id LIMIT 1), ''), GREATEST(t.created_at, COALESCE((SELECT MAX(re.created_at) FROM gantry.run_events re JOIN gantry.runs history ON history.id=re.run_id WHERE history.task_id=t.id), t.created_at), COALESCE((SELECT MAX(ar.created_at) FROM gantry.artifacts ar WHERE ar.task_id=t.id), t.created_at)) FROM gantry.tasks t JOIN gantry.agents a ON a.id=t.agent_id JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2`, taskID, actor.ID).Scan(&task.ID, &task.RequesterID, &task.AgentID, &task.AgentDisplayName, &task.Status, &task.CurrentRun.ID, &task.CurrentRun.Status, &task.CurrentRun.Reason, &task.ConversationRevision, &task.CreatedAt, &task.Title, &task.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	task.Status = publicStatus(task.Status)
	task.RequesterAction = requesterAction(task.Status)
	task.CurrentRun.Status = publicStatus(task.CurrentRun.Status)
	return task, nil
}

func requesterAction(status string) string {
	switch status {
	case "awaiting_approval":
		return "approval"
	case "awaiting_requester_input":
		return "input"
	default:
		return "none"
	}
}
