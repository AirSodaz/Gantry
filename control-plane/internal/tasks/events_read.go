package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

type Event struct {
	RunID       string          `json:"run_id"`
	Sequence    uint64          `json:"sequence"`
	RunSequence uint64          `json:"run_sequence"`
	Type        string          `json:"type"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Payload     json.RawMessage `json:"payload"`
}

type EventPage struct {
	Task        Task
	Runs        []RunAttempt
	Approvals   []approvals.Request
	CurrentSeq  uint64
	EarliestSeq uint64
	Events      []Event
}

// Events returns the committed event projection for a task. Authorization is
// applied in the same query as the event read so a hidden task is indistinguishable
// from a missing task.
func (s *Service) Events(ctx context.Context, actor identity.Principal, taskID string, after uint64, limit int) (EventPage, error) {
	limit = boundedLimit(limit)
	var page EventPage
	var status, runStatus string
	err := s.pool.QueryRow(ctx, `SELECT t.id, t.agent_id, a.display_name, t.status, r.id, r.status, r.status_reason, t.conversation_revision, t.created_at, t.task_event_sequence FROM gantry.tasks t JOIN gantry.agents a ON a.id=t.agent_id JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2`, taskID, actor.ID).Scan(&page.Task.ID, &page.Task.AgentID, &page.Task.AgentDisplayName, &status, &page.Task.CurrentRun.ID, &runStatus, &page.Task.CurrentRun.Reason, &page.Task.ConversationRevision, &page.Task.CreatedAt, &page.CurrentSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return EventPage{}, ErrNotFound
	}
	if err != nil {
		return EventPage{}, err
	}
	page.Task.Status = publicStatus(status)
	page.Task.CurrentRun.Status = publicStatus(runStatus)
	page.Task.Messages, err = s.listMessages(ctx, actor, taskID)
	if err != nil {
		return EventPage{}, err
	}
	page.Task.Artifacts, err = s.ListArtifacts(ctx, actor, taskID, 100)
	if err != nil {
		return EventPage{}, err
	}
	page.Runs, err = s.ListRuns(ctx, actor, taskID, 100)
	if err != nil {
		return EventPage{}, err
	}
	if s.approvals != nil {
		page.Approvals, err = s.approvals.ListTask(ctx, actor, taskID, 100)
		if err != nil {
			return EventPage{}, err
		}
	}
	var earliest *uint64
	if err := s.pool.QueryRow(ctx, `SELECT MIN(e.task_sequence) FROM gantry.run_events e JOIN gantry.runs r ON r.id=e.run_id WHERE r.task_id=$1`, taskID).Scan(&earliest); err != nil {
		return EventPage{}, err
	}
	if earliest != nil {
		page.EarliestSeq = *earliest
	}
	rows, err := s.pool.Query(ctx, `SELECT e.run_id, e.task_sequence, e.sequence, e.event_type, e.created_at, e.payload FROM gantry.run_events e JOIN gantry.runs r ON r.id=e.run_id WHERE r.task_id=$1 AND e.task_sequence>$2 ORDER BY e.task_sequence LIMIT $3`, taskID, after, limit)
	if err != nil {
		return EventPage{}, err
	}
	defer rows.Close()
	page.Events = make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.RunID, &event.Sequence, &event.RunSequence, &event.Type, &event.OccurredAt, &event.Payload); err != nil {
			return EventPage{}, err
		}
		if err := s.hydrateContentSegment(ctx, &event); err != nil {
			return EventPage{}, err
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, err
	}
	return page, nil
}
