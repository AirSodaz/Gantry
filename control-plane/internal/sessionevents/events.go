// Package sessionevents owns durable Session-scoped event ordering.
package sessionevents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Append assigns the next Session and Run sequences before recording an event.
func Append(ctx context.Context, tx pgx.Tx, runID, eventType string, payload any) error {
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}
	var sessionID string
	var runSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET event_sequence=event_sequence+1 WHERE id=$1 RETURNING session_id, event_sequence`, runID).Scan(&sessionID, &runSequence); err != nil {
		return err
	}
	var sessionSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.sessions SET session_event_sequence=session_event_sequence+1 WHERE id=$1 RETURNING session_event_sequence`, sessionID).Scan(&sessionSequence); err != nil {
		return err
	}
	return insert(ctx, tx, runID, runSequence, sessionSequence, eventType, data)
}

// AppendSession records a Session-owned event. It is intentionally separate
// from Run events so a membership change never needs a synthetic Run.
func AppendSession(ctx context.Context, tx pgx.Tx, sessionID, eventType string, payload any) error {
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.sessions SET session_event_sequence=session_event_sequence+1 WHERE id=$1 RETURNING session_event_sequence`, sessionID).Scan(&sequence); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.session_events (session_id, session_sequence, event_type, payload) VALUES ($1,$2,$3,$4::jsonb)`, sessionID, sequence, eventType, string(data))
	return err
}

// AppendAtSessionSequence records an event for a resource that already owns a
// Session sequence, such as the message committed by that same transition.
func AppendAtSessionSequence(ctx context.Context, tx pgx.Tx, runID string, sessionSequence int64, eventType string, payload any) error {
	if sessionSequence < 1 {
		return fmt.Errorf("invalid session event sequence")
	}
	data, err := encodePayload(payload)
	if err != nil {
		return err
	}
	var runSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET event_sequence=event_sequence+1 WHERE id=$1 RETURNING event_sequence`, runID).Scan(&runSequence); err != nil {
		return err
	}
	return insert(ctx, tx, runID, runSequence, sessionSequence, eventType, data)
}

func insert(ctx context.Context, tx pgx.Tx, runID string, runSequence, sessionSequence int64, eventType string, payload []byte) error {
	_, err := tx.Exec(ctx, `INSERT INTO gantry.run_events (run_id, sequence, session_sequence, event_type, payload) VALUES ($1,$2,$3,$4,$5::jsonb)`, runID, runSequence, sessionSequence, eventType, string(payload))
	return err
}

func encodePayload(payload any) ([]byte, error) {
	if payload == nil {
		return []byte(`{}`), nil
	}
	return json.Marshal(payload)
}
