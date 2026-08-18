package sessions

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCompleteAttachmentConcurrentSameKeyReplaysCanonicalReceipt(t *testing.T) {
	fixture := newAttachmentCompletionFixture("att_1", 4)
	store := &completionAttachmentStore{size: 4}
	actor := identity.Principal{ID: "principal_1", OrganizationID: "org_1"}
	type result struct {
		item      Attachment
		duplicate bool
		err       error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			item, duplicate, err := completeAttachmentCommand(context.Background(), actor, "att_1", "same-key", store, fixture.begin)
			results <- result{item: item, duplicate: duplicate, err: err}
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("first err=%v second err=%v", first.err, second.err)
	}
	if first.duplicate == second.duplicate {
		t.Fatalf("duplicate flags = %v, %v", first.duplicate, second.duplicate)
	}
	if first.item.State != "available" || second.item.State != "available" || !reflect.DeepEqual(first.item, second.item) {
		t.Fatalf("first=%+v second=%+v", first.item, second.item)
	}
	if got := store.headCalls.Load(); got != 1 {
		t.Fatalf("Head calls=%d want=1", got)
	}
	if fixture.transitions != 1 {
		t.Fatalf("state transitions=%d want=1", fixture.transitions)
	}
}

func TestCompleteAttachmentSameKeyCannotTargetAnotherAttachment(t *testing.T) {
	fixture := newAttachmentCompletionFixture("att_1", 4)
	store := &completionAttachmentStore{size: 4}
	actor := identity.Principal{ID: "principal_1", OrganizationID: "org_1"}
	if _, duplicate, err := completeAttachmentCommand(context.Background(), actor, "att_1", "same-key", store, fixture.begin); err != nil || duplicate {
		t.Fatalf("initial duplicate=%v err=%v", duplicate, err)
	}
	if _, _, err := completeAttachmentCommand(context.Background(), actor, "att_2", "same-key", store, fixture.begin); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("cross-resource err=%v", err)
	}
	if got := store.headCalls.Load(); got != 1 {
		t.Fatalf("Head calls=%d want=1", got)
	}
}

func TestCompleteAttachmentReplaysRejectedOutcome(t *testing.T) {
	fixture := newAttachmentCompletionFixture("att_1", 4)
	store := &completionAttachmentStore{size: 3}
	actor := identity.Principal{ID: "principal_1", OrganizationID: "org_1"}
	first, duplicate, err := completeAttachmentCommand(context.Background(), actor, "att_1", "same-key", store, fixture.begin)
	if !errors.Is(err, ErrInvalidInput) || duplicate || first.State != "rejected" {
		t.Fatalf("first=%+v duplicate=%v err=%v", first, duplicate, err)
	}
	second, duplicate, err := completeAttachmentCommand(context.Background(), actor, "att_1", "same-key", store, fixture.begin)
	if !errors.Is(err, ErrInvalidInput) || !duplicate || !reflect.DeepEqual(first, second) {
		t.Fatalf("second=%+v duplicate=%v err=%v", second, duplicate, err)
	}
	if got := store.headCalls.Load(); got != 1 || fixture.transitions != 1 {
		t.Fatalf("Head calls=%d transitions=%d", got, fixture.transitions)
	}
}

func TestAttachmentCompletionRequestDigestBindsResource(t *testing.T) {
	first := attachmentCompletionRequestDigest("att_1")
	if first != attachmentCompletionRequestDigest("att_1") || first == attachmentCompletionRequestDigest("att_2") || !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("unexpected completion digest %q", first)
	}
}

type completionAttachmentStore struct {
	size      int64
	headCalls atomic.Int64
}

func (s *completionAttachmentStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *completionAttachmentStore) Head(context.Context, string) (int64, error) {
	s.headCalls.Add(1)
	return s.size, nil
}

type attachmentCompletionFixture struct {
	mu          sync.Mutex
	attachment  Attachment
	objectKey   string
	receipt     *fixtureCompletionReceipt
	transitions int
}

type fixtureCompletionReceipt struct {
	digest       string
	attachmentID string
	response     []byte
}

func newAttachmentCompletionFixture(id string, size int64) *attachmentCompletionFixture {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	return &attachmentCompletionFixture{
		attachment: Attachment{ID: id, Filename: "report.txt", MediaType: "text/plain", SizeBytes: size, Digest: "sha256:" + strings.Repeat("a", 64), Classification: "internal", ScanStatus: "pending", State: "quarantined", CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)},
		objectKey:  "attachments/org_1/" + id,
	}
}

func (f *attachmentCompletionFixture) begin(context.Context) (attachmentCompletionTx, error) {
	return &fixtureAttachmentTx{fixture: f}, nil
}

type fixtureAttachmentTx struct {
	fixture *attachmentCompletionFixture
	locked  bool
	done    bool
}

func (tx *fixtureAttachmentTx) lock() {
	if !tx.locked {
		tx.fixture.mu.Lock()
		tx.locked = true
	}
}

func (tx *fixtureAttachmentTx) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tx.lock()
	switch {
	case strings.Contains(query, "INSERT INTO gantry.attachment_command_receipts"):
		if tx.fixture.receipt != nil {
			return pgconn.NewCommandTag("INSERT 0 0"), nil
		}
		tx.fixture.receipt = &fixtureCompletionReceipt{digest: args[3].(string), attachmentID: args[4].(string)}
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.Contains(query, "SET state='available'"):
		if tx.fixture.attachment.State != "quarantined" {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		tx.fixture.attachment.State, tx.fixture.attachment.ScanStatus = "available", "passed"
		tx.fixture.transitions++
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "SET state='rejected'"):
		if tx.fixture.attachment.State != "quarantined" {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		tx.fixture.attachment.State, tx.fixture.attachment.ScanStatus = "rejected", "failed"
		tx.fixture.transitions++
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "SET response_json"):
		tx.fixture.receipt.response = bytes.Clone(args[3].([]byte))
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		return pgconn.CommandTag{}, errors.New("unexpected Exec query")
	}
}

func (tx *fixtureAttachmentTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	tx.lock()
	if strings.Contains(query, "FROM gantry.attachment_command_receipts") {
		return fixtureRow{values: []any{tx.fixture.receipt.digest, tx.fixture.receipt.attachmentID, bytes.Clone(tx.fixture.receipt.response)}}
	}
	if strings.Contains(query, "FROM gantry.attachments") {
		item := tx.fixture.attachment
		return fixtureRow{values: []any{item.ID, tx.fixture.objectKey, item.Filename, item.MediaType, item.SizeBytes, item.Digest, item.Classification, item.ScanStatus, item.State, item.CreatedAt, item.ExpiresAt, item.BoundSessionID}}
	}
	return fixtureRow{err: errors.New("unexpected QueryRow query")}
}

func (tx *fixtureAttachmentTx) Commit(context.Context) error {
	tx.release()
	return nil
}

func (tx *fixtureAttachmentTx) Rollback(context.Context) error {
	tx.release()
	return nil
}

func (tx *fixtureAttachmentTx) release() {
	if tx.locked && !tx.done {
		tx.done = true
		tx.fixture.mu.Unlock()
	}
}

type fixtureRow struct {
	values []any
	err    error
}

func (row fixtureRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) != len(row.values) {
		return errors.New("fixture scan arity mismatch")
	}
	for index, value := range row.values {
		target := reflect.ValueOf(dest[index]).Elem()
		if value == nil {
			target.Set(reflect.Zero(target.Type()))
			continue
		}
		target.Set(reflect.ValueOf(value))
	}
	return nil
}
