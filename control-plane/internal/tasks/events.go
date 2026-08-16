package tasks

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func appendEvent(ctx context.Context, tx pgx.Tx, runID, eventType string) error {
	return appendEventPayload(ctx, tx, runID, eventType, "{}")
}
func appendEventPayload(ctx context.Context, tx pgx.Tx, runID, eventType, payload string) error {
	var runSequence int64
	var taskID string
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET event_sequence=event_sequence+1 WHERE id=$1 RETURNING task_id, event_sequence`, runID).Scan(&taskID, &runSequence); err != nil {
		return err
	}
	var taskSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.tasks SET task_event_sequence=task_event_sequence+1 WHERE id=$1 RETURNING task_event_sequence`, taskID).Scan(&taskSequence); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO gantry.run_events (run_id, sequence, task_sequence, event_type, payload) VALUES ($1,$2,$3,$4,$5::jsonb)`, runID, runSequence, taskSequence, eventType, payload)
	return err
}
