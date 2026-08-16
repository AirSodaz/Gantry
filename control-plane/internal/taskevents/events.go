// Package taskevents owns durable Task-scoped event ordering.
package taskevents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Append assigns the next Task and Run sequences before recording an event.
func Append(ctx context.Context, tx pgx.Tx, runID, eventType string, payload any) error {
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}
	var taskID string
	var runSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET event_sequence=event_sequence+1 WHERE id=$1 RETURNING task_id, event_sequence`, runID).Scan(&taskID, &runSequence); err != nil {
		return err
	}
	var taskSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.tasks SET task_event_sequence=task_event_sequence+1 WHERE id=$1 RETURNING task_event_sequence`, taskID).Scan(&taskSequence); err != nil {
		return err
	}
	return insert(ctx, tx, runID, runSequence, taskSequence, eventType, data)
}

// AppendAtTaskSequence records an event for a resource that already owns a
// Task sequence, such as the message committed by that same transition.
func AppendAtTaskSequence(ctx context.Context, tx pgx.Tx, runID string, taskSequence int64, eventType string, payload any) error {
	if taskSequence < 1 {
		return fmt.Errorf("invalid task event sequence")
	}
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}
	var runSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET event_sequence=event_sequence+1 WHERE id=$1 RETURNING event_sequence`, runID).Scan(&runSequence); err != nil {
		return err
	}
	return insert(ctx, tx, runID, runSequence, taskSequence, eventType, data)
}

func insert(ctx context.Context, tx pgx.Tx, runID string, runSequence, taskSequence int64, eventType string, payload []byte) error {
	_, err := tx.Exec(ctx, `INSERT INTO gantry.run_events (run_id, sequence, task_sequence, event_type, payload) VALUES ($1,$2,$3,$4,$5::jsonb)`, runID, runSequence, taskSequence, eventType, string(payload))
	return err
}

func encodePayload(payload any) ([]byte, error) {
	if payload == nil {
		return []byte(`{}`), nil
	}
	return json.Marshal(payload)
}
