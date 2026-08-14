package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (s *Service) ClaimNext(ctx context.Context, runnerID string) (Assignment, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Assignment{}, false, err
	}
	defer tx.Rollback(ctx)
	var runID, mode string
	err = tx.QueryRow(ctx, `SELECT id, demo_mode FROM gantry.runs WHERE status='queued' ORDER BY created_at, id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&runID, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, false, nil
	}
	if err != nil {
		return Assignment{}, false, err
	}
	var epoch uint64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET status='assigned', runner_id=$2, lease_epoch=lease_epoch+1, started_at=COALESCE(started_at, now()) WHERE id=$1 RETURNING lease_epoch`, runID, runnerID).Scan(&epoch); err != nil {
		return Assignment{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='running' WHERE current_run_id=$1`, runID); err != nil {
		return Assignment{}, false, err
	}
	if err := appendEvent(ctx, tx, runID, "run.assigned"); err != nil {
		return Assignment{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, false, err
	}
	manifest, digest := demoManifest(mode)
	return Assignment{RunID: runID, LeaseEpoch: epoch, Manifest: manifest, ManifestDigest: digest}, true, nil
}

func (s *Service) Accept(ctx context.Context, runnerID, runID string, epoch uint64, digest string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var mode string
	err = tx.QueryRow(ctx, `SELECT demo_mode FROM gantry.runs WHERE id=$1 AND runner_id=$2 AND lease_epoch=$3 AND status='assigned' FOR UPDATE`, runID, runnerID, epoch).Scan(&mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, expected := demoManifest(mode)
	if digest != expected {
		return ErrInvalidInput
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='accepted' WHERE id=$1`, runID); err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, runID, "run.started"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) RecordEvents(ctx context.Context, runnerID, runID string, epoch uint64, events []RunnerEvent) (uint64, error) {
	if len(events) == 0 {
		return 0, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var status string
	var current uint64
	err = tx.QueryRow(ctx, `SELECT status, runner_event_sequence FROM gantry.runs WHERE id=$1 AND runner_id=$2 AND lease_epoch=$3 FOR UPDATE`, runID, runnerID, epoch).Scan(&status, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if status != "accepted" && status != "canceling" {
		return 0, ErrInvalidInput
	}
	for _, event := range events {
		if event.ClientSequence != current+1 {
			return 0, ErrInvalidInput
		}
		current++
		if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET runner_event_sequence=$2 WHERE id=$1`, runID, current); err != nil {
			return 0, err
		}
		if err := appendEventPayload(ctx, tx, runID, event.Type, event.Payload); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return current, nil
}

func (s *Service) Finish(ctx context.Context, runnerID, runID string, epoch uint64, terminal, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status, taskID string
	err = tx.QueryRow(ctx, `SELECT status, task_id FROM gantry.runs WHERE id=$1 AND runner_id=$2 AND lease_epoch=$3 FOR UPDATE`, runID, runnerID, epoch).Scan(&status, &taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "accepted" && status != "canceling" {
		return ErrInvalidInput
	}
	if (terminal == "completed" && status == "canceling") || (terminal == "canceled" && status != "canceling") {
		return ErrInvalidInput
	}
	if terminal != "completed" && terminal != "failed" && terminal != "canceled" {
		return ErrInvalidInput
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status=$2, status_reason=$3, completed_at=now() WHERE id=$1`, runID, terminal, reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status=$2 WHERE id=$1`, taskID, terminal); err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, runID, "run."+terminal); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) FailActive(ctx context.Context, runnerID, runID, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var taskID string
	err = tx.QueryRow(ctx, `UPDATE gantry.runs SET status='failed', status_reason=$3, completed_at=now() WHERE id=$1 AND runner_id=$2 AND status IN ('assigned','accepted','canceling') RETURNING task_id`, runID, runnerID, reason).Scan(&taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='failed' WHERE id=$1`, taskID); err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, runID, "run.failed"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FailInFlight records the explicit recovery contract for deterministic demo
// runs: an interrupted control plane does not resume execution.
func (s *Service) FailInFlight(ctx context.Context, reason string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `UPDATE gantry.runs SET status='failed', status_reason=$1, completed_at=now() WHERE status IN ('assigned','accepted','canceling') RETURNING id, task_id`, reason)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type failedRun struct {
		id     string
		taskID string
	}
	failed := make([]failedRun, 0)
	for rows.Next() {
		var run failedRun
		if err := rows.Scan(&run.id, &run.taskID); err != nil {
			return 0, err
		}
		failed = append(failed, run)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, run := range failed {
		if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='failed' WHERE id=$1 AND current_run_id=$2`, run.taskID, run.id); err != nil {
			return 0, err
		}
		if err := appendEvent(ctx, tx, run.id, "run.failed"); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(failed), nil
}

func demoManifest(mode string) ([]byte, string) {
	manifest := []byte(fmt.Sprintf(`{"kind":"gantry.phase0.demo/v1","mode":%q}`, mode))
	sum := sha256.Sum256(manifest)
	return manifest, "sha256:" + hex.EncodeToString(sum[:])
}
