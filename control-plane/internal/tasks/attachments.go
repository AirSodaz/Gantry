package tasks

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

const maxAttachmentBytes int64 = 64 << 20

func (s *Service) CreateAttachment(ctx context.Context, actor identity.Principal, request CreateAttachmentRequest) (Attachment, error) {
	if s.store == nil {
		return Attachment{}, ErrInvalidState
	}
	item, err := normalizeAttachment(request)
	if err != nil {
		return Attachment{}, err
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Attachment{}, err
	}
	item.ID = newID("att")
	token := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(2 * time.Minute)
	objectKey := "attachments/" + actor.OrganizationID + "/" + item.ID
	_, err = s.pool.Exec(ctx, `INSERT INTO gantry.attachments (id, organization_id, requester_principal_id, object_key, filename, media_type, size_bytes, digest, classification, scan_status, state, upload_token_hash, upload_expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending','declared',$10,$11)`, item.ID, actor.OrganizationID, actor.ID, objectKey, item.Filename, item.MediaType, item.SizeBytes, item.Digest, item.Classification, hex.EncodeToString(tokenHash[:]), expiresAt)
	if err != nil {
		return Attachment{}, err
	}
	item.State, item.ScanStatus, item.UploadToken, item.UploadExpires = "declared", "pending", token, expiresAt
	item.UploadURL = "/api/copilot/v1/attachments/" + item.ID + "/content"
	item.CreatedAt = time.Now().UTC()
	return item, nil
}

func (s *Service) UploadAttachment(ctx context.Context, actor identity.Principal, attachmentID, token string, body io.Reader) error {
	if s.store == nil || strings.TrimSpace(attachmentID) == "" || strings.TrimSpace(token) == "" {
		return ErrInvalidInput
	}
	var objectKey, mediaType, expectedDigest, tokenHash, state string
	var expectedSize int64
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT object_key, media_type, size_bytes, digest, upload_token_hash, upload_expires_at, state FROM gantry.attachments WHERE id=$1 AND organization_id=$2 AND requester_principal_id=$3`, attachmentID, actor.OrganizationID, actor.ID).Scan(&objectKey, &mediaType, &expectedSize, &expectedDigest, &tokenHash, &expiresAt, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	providedHash := sha256.Sum256([]byte(token))
	if state != "declared" || expiresAt.Before(time.Now().UTC()) || !hmacEqualHex(tokenHash, providedHash[:]) {
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
	if err := s.store.Put(ctx, objectKey, bytes.NewReader(data), expectedSize, mediaType); err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `UPDATE gantry.attachments SET state='uploaded', uploaded_at=now(), upload_token_hash='', upload_expires_at=NULL WHERE id=$1 AND state='declared'`, attachmentID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrInvalidInput
	}
	return nil
}

// CompleteAttachment validates the quarantined object and transitions it to
// the development scan-passed state. Production malware scanner integration is
// intentionally outside this adapter and must replace this completion policy.
func (s *Service) CompleteAttachment(ctx context.Context, actor identity.Principal, attachmentID string) (Attachment, error) {
	if s.store == nil || strings.TrimSpace(attachmentID) == "" {
		return Attachment{}, ErrInvalidInput
	}
	var item Attachment
	var objectKey string
	err := s.pool.QueryRow(ctx, `SELECT id, object_key, filename, media_type, size_bytes, digest, classification, scan_status, state, created_at FROM gantry.attachments WHERE id=$1 AND organization_id=$2 AND requester_principal_id=$3`, attachmentID, actor.OrganizationID, actor.ID).Scan(&item.ID, &objectKey, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	if err != nil {
		return Attachment{}, err
	}
	if item.State != "uploaded" {
		return Attachment{}, ErrInvalidState
	}
	size, err := s.store.Head(ctx, objectKey)
	if err != nil {
		return Attachment{}, err
	}
	if size != item.SizeBytes {
		_, _ = s.pool.Exec(ctx, `UPDATE gantry.attachments SET state='rejected', scan_status='failed', completed_at=now() WHERE id=$1`, item.ID)
		return Attachment{}, ErrInvalidInput
	}
	result, err := s.pool.Exec(ctx, `UPDATE gantry.attachments SET state='available', scan_status='passed', completed_at=now() WHERE id=$1 AND state='uploaded'`, item.ID)
	if err != nil {
		return Attachment{}, err
	}
	if result.RowsAffected() != 1 {
		return Attachment{}, ErrInvalidState
	}
	item.State, item.ScanStatus = "available", "passed"
	return item, nil
}

func (s *Service) GetAttachment(ctx context.Context, actor identity.Principal, attachmentID string) (Attachment, error) {
	if s.store == nil {
		return Attachment{}, ErrNotFound
	}
	var item Attachment
	err := s.pool.QueryRow(ctx, `SELECT id, filename, media_type, size_bytes, digest, classification, scan_status, state, created_at FROM gantry.attachments WHERE id=$1 AND organization_id=$2 AND requester_principal_id=$3`, attachmentID, actor.OrganizationID, actor.ID).Scan(&item.ID, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	return item, err
}

func normalizeAttachment(request CreateAttachmentRequest) (Attachment, error) {
	item := Attachment{
		Filename:       strings.TrimSpace(request.Filename),
		MediaType:      strings.TrimSpace(request.MediaType),
		SizeBytes:      request.SizeBytes,
		Digest:         strings.TrimSpace(request.Digest),
		Classification: strings.TrimSpace(request.Classification),
	}
	if item.Classification == "" {
		item.Classification = "internal"
	}
	if item.Filename == "" || len(item.Filename) > 255 || strings.ContainsAny(item.Filename, "\\/") || item.MediaType == "" || item.SizeBytes < 0 || item.SizeBytes > maxAttachmentBytes || !validAttachmentDigest(item.Digest) {
		return Attachment{}, ErrInvalidInput
	}
	return item, nil
}

func validAttachmentDigest(digest string) bool {
	if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(digest, "sha256:"))
	return err == nil
}
