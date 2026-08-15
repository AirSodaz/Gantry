package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

type Event struct {
	Sequence   uint64          `json:"sequence"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

type EventPage struct {
	Task        Task
	RunID       string
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
	err := s.pool.QueryRow(ctx, `SELECT t.id, t.agent_id, a.display_name, t.status, r.id, r.status, r.status_reason, t.created_at, r.event_sequence FROM gantry.tasks t JOIN gantry.agents a ON a.id=t.agent_id JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2`, taskID, actor.ID).Scan(&page.Task.ID, &page.Task.AgentID, &page.Task.AgentDisplayName, &status, &page.RunID, &runStatus, &page.Task.CurrentRun.Reason, &page.Task.CreatedAt, &page.CurrentSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return EventPage{}, ErrNotFound
	}
	if err != nil {
		return EventPage{}, err
	}
	page.Task.Status = publicStatus(status)
	page.Task.CurrentRun = Run{ID: page.RunID, Status: publicStatus(runStatus), Reason: page.Task.CurrentRun.Reason}
	var earliest *uint64
	if err := s.pool.QueryRow(ctx, `SELECT MIN(sequence) FROM gantry.run_events WHERE run_id=$1`, page.RunID).Scan(&earliest); err != nil {
		return EventPage{}, err
	}
	if earliest != nil {
		page.EarliestSeq = *earliest
	}
	rows, err := s.pool.Query(ctx, `SELECT sequence, event_type, created_at, payload FROM gantry.run_events WHERE run_id=$1 AND sequence>$2 ORDER BY sequence LIMIT $3`, page.RunID, after, limit)
	if err != nil {
		return EventPage{}, err
	}
	defer rows.Close()
	page.Events = make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.Sequence, &event.Type, &event.OccurredAt, &event.Payload); err != nil {
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
