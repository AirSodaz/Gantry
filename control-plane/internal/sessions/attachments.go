package sessions

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxAttachmentBytes int64 = 64 << 20

const (
	createAttachmentRoute   = "POST /api/copilot/v1/attachments"
	completeAttachmentRoute = "POST /api/copilot/v1/attachments/{attachment_id}:complete"
)

func (s *Service) CreateAttachment(ctx context.Context, actor identity.Principal, key string, request CreateAttachmentRequest) (Attachment, bool, error) {
	if s.attachments == nil {
		return Attachment{}, false, ErrInvalidState
	}
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 {
		return Attachment{}, false, ErrInvalidInput
	}
	item, err := normalizeAttachment(request)
	if err != nil {
		return Attachment{}, false, err
	}
	requestDigest := attachmentRequestDigest(item)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Attachment{}, false, err
	}
	defer tx.Rollback(ctx)
	var storedDigest, attachmentID, uploadToken string
	var uploadExpires time.Time
	err = tx.QueryRow(ctx, `SELECT request_digest,attachment_id,upload_token,upload_expires_at FROM gantry.attachment_command_receipts WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3 FOR UPDATE`, actor.ID, createAttachmentRoute, key).Scan(&storedDigest, &attachmentID, &uploadToken, &uploadExpires)
	if err == nil {
		if storedDigest != requestDigest {
			return Attachment{}, false, ErrIdempotencyConflict
		}
		item, err = loadAttachment(ctx, tx, actor, attachmentID)
		if err != nil {
			return Attachment{}, false, err
		}
		item.UploadToken, item.UploadExpires = uploadToken, uploadExpires
		if err := tx.Commit(ctx); err != nil {
			return Attachment{}, false, err
		}
		return item, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, false, err
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Attachment{}, false, err
	}
	item.ID = newID("att")
	token := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(2 * time.Minute)
	attachmentExpiresAt := time.Now().UTC().Add(24 * time.Hour)
	objectKey := "attachments/" + actor.OrganizationID + "/" + item.ID
	_, err = tx.Exec(ctx, `INSERT INTO gantry.attachments (id, organization_id, owner_principal_id, object_key, filename, media_type, size_bytes, digest, classification, scan_status, state, upload_token_hash, upload_expires_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending','declared',$10,$11,$12)`, item.ID, actor.OrganizationID, actor.ID, objectKey, item.Filename, item.MediaType, item.SizeBytes, item.Digest, item.Classification, hex.EncodeToString(tokenHash[:]), expiresAt, attachmentExpiresAt)
	if err != nil {
		return Attachment{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.attachment_command_receipts(principal_id,route,idempotency_key,request_digest,attachment_id,upload_token,upload_expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, actor.ID, createAttachmentRoute, key, requestDigest, item.ID, token, expiresAt); err != nil {
		return Attachment{}, false, err
	}
	item.State, item.ScanStatus, item.UploadToken, item.UploadExpires = "declared", "pending", token, expiresAt
	item.UploadURL = "/api/copilot/v1/attachments/" + item.ID + "/content"
	item.CreatedAt, item.ExpiresAt = time.Now().UTC(), attachmentExpiresAt
	if err := tx.Commit(ctx); err != nil {
		return Attachment{}, false, err
	}
	return item, false, nil
}

func (s *Service) UploadAttachment(ctx context.Context, actor identity.Principal, attachmentID, token string, body io.Reader) error {
	if s.attachments == nil || strings.TrimSpace(attachmentID) == "" || strings.TrimSpace(token) == "" {
		return ErrInvalidInput
	}
	var objectKey, mediaType, expectedDigest, tokenHash, state string
	var expectedSize int64
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT object_key, media_type, size_bytes, digest, upload_token_hash, upload_expires_at, state FROM gantry.attachments WHERE id=$1 AND organization_id=$2 AND owner_principal_id=$3`, attachmentID, actor.OrganizationID, actor.ID).Scan(&objectKey, &mediaType, &expectedSize, &expectedDigest, &tokenHash, &expiresAt, &state)
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
	if err := s.attachments.Put(ctx, objectKey, bytes.NewReader(data), expectedSize, mediaType); err != nil {
		return err
	}
	result, err := s.pool.Exec(ctx, `UPDATE gantry.attachments SET state='quarantined', uploaded_at=now(), upload_token_hash='', upload_expires_at=NULL WHERE id=$1 AND state='declared'`, attachmentID)
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
func (s *Service) CompleteAttachment(ctx context.Context, actor identity.Principal, attachmentID, key string) (Attachment, bool, error) {
	if s.attachments == nil || strings.TrimSpace(attachmentID) == "" {
		return Attachment{}, false, ErrInvalidInput
	}
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 {
		return Attachment{}, false, ErrInvalidInput
	}
	return completeAttachmentCommand(ctx, actor, attachmentID, key, s.attachments, func(ctx context.Context) (attachmentCompletionTx, error) {
		return s.pool.Begin(ctx)
	})
}

type attachmentCompletionTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Commit(context.Context) error
	Rollback(context.Context) error
}

type attachmentCompletionReceipt struct {
	Attachment Attachment `json:"attachment"`
	ErrorCode  string     `json:"error_code,omitempty"`
}

func completeAttachmentCommand(ctx context.Context, actor identity.Principal, attachmentID, key string, store AttachmentStore, begin func(context.Context) (attachmentCompletionTx, error)) (Attachment, bool, error) {
	tx, err := begin(ctx)
	if err != nil {
		return Attachment{}, false, err
	}
	defer tx.Rollback(ctx)
	digest := attachmentCompletionRequestDigest(attachmentID)
	claim, err := tx.Exec(ctx, `INSERT INTO gantry.attachment_command_receipts(principal_id,route,idempotency_key,request_digest,attachment_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, actor.ID, completeAttachmentRoute, key, digest, attachmentID)
	if err != nil {
		return Attachment{}, false, err
	}
	claimed := claim.RowsAffected() == 1
	var storedDigest, storedID string
	var responseJSON []byte
	if err := tx.QueryRow(ctx, `SELECT request_digest,attachment_id,response_json FROM gantry.attachment_command_receipts WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3 FOR UPDATE`, actor.ID, completeAttachmentRoute, key).Scan(&storedDigest, &storedID, &responseJSON); err != nil {
		return Attachment{}, false, err
	}
	if storedDigest != digest || storedID != attachmentID {
		return Attachment{}, false, ErrIdempotencyConflict
	}
	if !claimed {
		receipt, err := decodeAttachmentCompletionReceipt(responseJSON)
		if err != nil {
			return Attachment{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Attachment{}, false, err
		}
		return receipt.Attachment, true, attachmentCompletionReceiptError(receipt.ErrorCode)
	}

	var item Attachment
	var objectKey string
	err = tx.QueryRow(ctx, `SELECT id,object_key,filename,media_type,size_bytes,digest,classification,scan_status,state,created_at,expires_at,bound_session_id FROM gantry.attachments WHERE id=$1 AND organization_id=$2 AND owner_principal_id=$3 FOR UPDATE`, attachmentID, actor.OrganizationID, actor.ID).Scan(&item.ID, &objectKey, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt, &item.ExpiresAt, &item.BoundSessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, false, ErrNotFound
	}
	if err != nil {
		return Attachment{}, false, err
	}
	if item.State != "quarantined" {
		return Attachment{}, false, ErrInvalidState
	}
	size, err := store.Head(ctx, objectKey)
	if err != nil {
		return Attachment{}, false, err
	}
	var commandErr error
	if size != item.SizeBytes {
		result, err := tx.Exec(ctx, `UPDATE gantry.attachments SET state='rejected',scan_status='failed',completed_at=now() WHERE id=$1 AND state='quarantined'`, item.ID)
		if err != nil {
			return Attachment{}, false, err
		}
		if result.RowsAffected() != 1 {
			return Attachment{}, false, ErrInvalidState
		}
		item.State, item.ScanStatus = "rejected", "failed"
		item.RejectionReason = &UserFacingReason{Code: "attachment_size_mismatch", Message: "Attachment content did not match the declared size.", NextAction: "none"}
		commandErr = ErrInvalidInput
	} else {
		result, err := tx.Exec(ctx, `UPDATE gantry.attachments SET state='available',scan_status='passed',completed_at=now() WHERE id=$1 AND state='quarantined'`, item.ID)
		if err != nil {
			return Attachment{}, false, err
		}
		if result.RowsAffected() != 1 {
			return Attachment{}, false, ErrInvalidState
		}
		item.State, item.ScanStatus = "available", "passed"
	}
	receipt := attachmentCompletionReceipt{Attachment: item}
	if commandErr != nil {
		receipt.ErrorCode = "invalid_input"
	}
	responseJSON, err = json.Marshal(receipt)
	if err != nil {
		return Attachment{}, false, err
	}
	result, err := tx.Exec(ctx, `UPDATE gantry.attachment_command_receipts SET response_json=$4 WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3`, actor.ID, completeAttachmentRoute, key, responseJSON)
	if err != nil {
		return Attachment{}, false, err
	}
	if result.RowsAffected() != 1 {
		return Attachment{}, false, errors.New("attachment completion receipt was not finalized")
	}
	if err := tx.Commit(ctx); err != nil {
		return Attachment{}, false, err
	}
	return item, false, commandErr
}

func attachmentCompletionRequestDigest(attachmentID string) string {
	sum := sha256.Sum256([]byte(completeAttachmentRoute + "\n" + strings.TrimSpace(attachmentID)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func decodeAttachmentCompletionReceipt(data []byte) (attachmentCompletionReceipt, error) {
	var receipt attachmentCompletionReceipt
	if len(data) == 0 {
		return receipt, errors.New("attachment completion receipt is not finalized")
	}
	if err := json.Unmarshal(data, &receipt); err != nil || receipt.Attachment.ID == "" {
		return attachmentCompletionReceipt{}, errors.New("attachment completion receipt is invalid")
	}
	return receipt, nil
}

func attachmentCompletionReceiptError(code string) error {
	switch code {
	case "":
		return nil
	case "invalid_input":
		return ErrInvalidInput
	default:
		return errors.New("attachment completion receipt has an unknown outcome")
	}
}

func (s *Service) GetAttachment(ctx context.Context, actor identity.Principal, attachmentID string) (Attachment, error) {
	if s.attachments == nil {
		return Attachment{}, ErrNotFound
	}
	var item Attachment
	err := s.pool.QueryRow(ctx, `SELECT id, filename, media_type, size_bytes, digest, classification, scan_status, state, created_at, expires_at, bound_session_id FROM gantry.attachments WHERE id=$1 AND organization_id=$2 AND owner_principal_id=$3`, attachmentID, actor.OrganizationID, actor.ID).Scan(&item.ID, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt, &item.ExpiresAt, &item.BoundSessionID)
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

func attachmentRequestDigest(item Attachment) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{item.Filename, item.MediaType, strconv.FormatInt(item.SizeBytes, 10), item.Digest, item.Classification}, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func loadAttachment(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actor identity.Principal, attachmentID string) (Attachment, error) {
	var item Attachment
	err := q.QueryRow(ctx, `SELECT id,filename,media_type,size_bytes,digest,classification,scan_status,state,created_at,expires_at,bound_session_id FROM gantry.attachments WHERE id=$1 AND organization_id=$2 AND owner_principal_id=$3`, attachmentID, actor.OrganizationID, actor.ID).Scan(&item.ID, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &item.CreatedAt, &item.ExpiresAt, &item.BoundSessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	return item, err
}

func hmacEqualHex(expected string, actual []byte) bool {
	decoded, err := hex.DecodeString(expected)
	return err == nil && len(decoded) == len(actual) && subtle.ConstantTimeCompare(decoded, actual) == 1
}
