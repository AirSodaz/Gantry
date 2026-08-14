package tasks

import (
	"context"
	"github.com/jackc/pgx/v5"
)

func appendEvent(ctx context.Context, tx pgx.Tx, runID, eventType string) error {
	return appendEventPayload(ctx, tx, runID, eventType, "{}")
}
func appendEventPayload(ctx context.Context, tx pgx.Tx, runID, eventType, payload string) error {
	var sequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET event_sequence=event_sequence+1 WHERE id=$1 RETURNING event_sequence`, runID).Scan(&sequence); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO gantry.run_events (run_id, sequence, event_type, payload) VALUES ($1,$2,$3,$4::jsonb)`, runID, sequence, eventType, payload)
	return err
}
