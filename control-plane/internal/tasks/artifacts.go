package tasks

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

type artifactStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Head(context.Context, string) (int64, error)
	PresignGet(context.Context, string, time.Duration) (string, time.Time, error)
}

func (s *Service) ListArtifacts(ctx context.Context, actor identity.Principal, taskID string, limit int) ([]Artifact, error) {
	if s.store == nil {
		return []Artifact{}, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT ar.id, ar.task_id, ar.run_id, ar.filename, ar.media_type, ar.size_bytes, ar.digest, ar.classification, ar.scan_status, ar.state, ar.created_at FROM gantry.artifacts ar JOIN gantry.tasks t ON t.id=ar.task_id WHERE ar.task_id=$1 AND t.requester_principal_id=$2 ORDER BY ar.created_at, ar.id LIMIT $3`, taskID, actor.ID, boundedLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Artifact, 0)
	for rows.Next() {
		var item Artifact
		if err := rows.Scan(&item.ID, &item.TaskID, &item.RunID, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetArtifact(ctx context.Context, actor identity.Principal, artifactID string) (Artifact, error) {
	if s.store == nil {
		return Artifact{}, ErrNotFound
	}
	var item Artifact
	var objectKey string
	err := s.pool.QueryRow(ctx, `SELECT ar.id, ar.task_id, ar.run_id, ar.object_key, ar.filename, ar.media_type, ar.size_bytes, ar.digest, ar.classification, ar.scan_status, ar.state, ar.created_at FROM gantry.artifacts ar JOIN gantry.tasks t ON t.id=ar.task_id WHERE ar.id=$1 AND (t.requester_principal_id=$2 OR ar.visibility='workspace' AND t.workspace_id IN (SELECT workspace_id FROM gantry.workspace_memberships WHERE principal_id=$2))`, artifactID, actor.ID).Scan(&item.ID, &item.TaskID, &item.RunID, &objectKey, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	if err != nil {
		return Artifact{}, err
	}
	if item.State != "available" || item.ScanStatus != "passed" {
		return Artifact{}, fmt.Errorf("artifact is not available")
	}
	url, expiresAt, err := s.store.PresignGet(ctx, objectKey, 2*time.Minute)
	if err != nil {
		return Artifact{}, err
	}
	item.DownloadURL = url
	item.DownloadURLExpires = expiresAt
	return item, nil
}

func (s *Service) DeclareArtifact(ctx context.Context, runnerID string, runID string, epoch uint64, input Artifact) (Artifact, string, time.Time, error) {
	if s.store == nil || strings.TrimSpace(runnerID) == "" || strings.TrimSpace(runID) == "" || input.SizeBytes < 0 || strings.TrimSpace(input.Filename) == "" || strings.TrimSpace(input.Digest) == "" {
		return Artifact{}, "", time.Time{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Artifact{}, "", time.Time{}, err
	}
	defer tx.Rollback(ctx)
	var taskID string
	if err := tx.QueryRow(ctx, `SELECT task_id FROM gantry.runs WHERE id=$1 AND runner_id=$2 AND lease_epoch=$3 AND status IN ('accepted','canceling') FOR UPDATE`, runID, runnerID, epoch).Scan(&taskID); errors.Is(err, pgx.ErrNoRows) {
		return Artifact{}, "", time.Time{}, ErrNotFound
	} else if err != nil {
		return Artifact{}, "", time.Time{}, err
	}
	if input.ID == "" {
		input.ID = newID("art")
	}
	objectKey := "artifacts/" + taskID + "/" + input.ID
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Artifact{}, "", time.Time{}, err
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token + "|" + strconv.FormatUint(epoch, 10)))
	expiresAt := time.Now().UTC().Add(2 * time.Minute)
	_, err = tx.Exec(ctx, `INSERT INTO gantry.artifacts (id, task_id, run_id, object_key, filename, media_type, size_bytes, digest, classification, scan_status, visibility, state, upload_token_hash, upload_lease_epoch, upload_expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,COALESCE(NULLIF($9,''),'internal'),'pending','requester','declared',$10,$11,$12) ON CONFLICT (id) DO UPDATE SET filename=EXCLUDED.filename, media_type=EXCLUDED.media_type, size_bytes=EXCLUDED.size_bytes, digest=EXCLUDED.digest, upload_token_hash=EXCLUDED.upload_token_hash, upload_lease_epoch=EXCLUDED.upload_lease_epoch, upload_expires_at=EXCLUDED.upload_expires_at, state='declared', scan_status='pending'`, input.ID, taskID, runID, objectKey, input.Filename, input.MediaType, input.SizeBytes, input.Digest, input.Classification, hex.EncodeToString(tokenHash[:]), epoch, expiresAt)
	if err != nil {
		return Artifact{}, "", time.Time{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Artifact{}, "", time.Time{}, err
	}
	input.TaskID, input.RunID, input.State, input.ScanStatus = taskID, runID, "declared", "pending"
	return input, token, expiresAt, nil
}

func (s *Service) UploadArtifact(ctx context.Context, artifactID, token string, body io.Reader) error {
	if s.store == nil || strings.TrimSpace(artifactID) == "" || strings.TrimSpace(token) == "" {
		return ErrInvalidInput
	}
	var objectKey, mediaType, expectedDigest, tokenHash, state string
	var expectedSize int64
	var leaseEpoch uint64
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT ar.object_key, ar.media_type, ar.size_bytes, ar.digest, ar.upload_token_hash, ar.upload_expires_at, ar.upload_lease_epoch, ar.state FROM gantry.artifacts ar JOIN gantry.runs r ON r.id=ar.run_id WHERE ar.id=$1 AND r.status IN ('accepted','canceling','completed','failed','canceled') AND r.lease_epoch=ar.upload_lease_epoch`, artifactID).Scan(&objectKey, &mediaType, &expectedSize, &expectedDigest, &tokenHash, &expiresAt, &leaseEpoch, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	providedHash := sha256.Sum256([]byte(token + "|" + strconv.FormatUint(leaseEpoch, 10)))
	if state != "declared" || expiresAt.Before(time.Now().UTC()) || !hmacEqualHex(tokenHash, providedHash[:]) {
		return ErrInvalidInput
	}
	if expectedSize > 64<<20 {
		return ErrInvalidInput
	}
	data, err := io.ReadAll(io.LimitReader(body, expectedSize+1))
	if err != nil {
		return err
	}
	if int64(len(data)) != expectedSize {
		return ErrInvalidInput
	}
	digest := sha256.Sum256(data)
	if expectedDigest != "sha256:"+hex.EncodeToString(digest[:]) {
		return ErrInvalidInput
	}
	if err := s.store.Put(ctx, objectKey, bytes.NewReader(data), int64(len(data)), mediaType); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE gantry.artifacts SET state='available', scan_status='passed', uploaded_at=now(), upload_token_hash='', upload_expires_at=NULL WHERE id=$1 AND state='declared'`, artifactID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrInvalidInput
	}
	var runID string
	if err := tx.QueryRow(ctx, `SELECT run_id FROM gantry.artifacts WHERE id=$1`, artifactID).Scan(&runID); err == nil {
		if err := appendEventPayload(ctx, tx, runID, "artifact.uploaded", `{"artifact_id":"`+artifactID+`"}`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func hmacEqualHex(expected string, actual []byte) bool {
	decoded, err := hex.DecodeString(expected)
	return err == nil && len(decoded) == len(actual) && subtle.ConstantTimeCompare(decoded, actual) == 1
}
