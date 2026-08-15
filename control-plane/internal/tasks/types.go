package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/objectstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrInvalidInput        = errors.New("invalid task input")
	ErrInvalidState        = errors.New("invalid task state")
	ErrIdempotencyConflict = errors.New("idempotency key reused with a different request")
)

type Service struct {
	pool      *pgxpool.Pool
	approvals *approvals.Service
	store     objectstore.ArtifactStore
	content   contentBufferRegistry
}

func NewService(pool *pgxpool.Pool, approvalService *approvals.Service) *Service {
	return &Service{pool: pool, approvals: approvalService}
}

func NewServiceWithStore(pool *pgxpool.Pool, approvalService *approvals.Service, store objectstore.ArtifactStore) *Service {
	return &Service{pool: pool, approvals: approvalService, store: store, content: contentBufferRegistry{streams: make(map[contentStreamKey]*contentStream)}}
}

type contentStreamKey struct {
	runID    string
	streamID string
}

type contentBufferRegistry struct {
	mu      sync.Mutex
	streams map[contentStreamKey]*contentStream
}

type Agent struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
type Run struct {
	ID                        string `json:"id"`
	Status                    string `json:"status"`
	Reason                    string `json:"status_reason,omitempty"`
	LeaseEpoch                uint64 `json:"-"`
	AcknowledgedEventSequence uint64 `json:"-"`
}

type Task struct {
	ID               string     `json:"id"`
	AgentID          string     `json:"agent_id"`
	AgentDisplayName string     `json:"agent_display_name,omitempty"`
	Status           string     `json:"status"`
	CurrentRun       Run        `json:"current_run"`
	Artifacts        []Artifact `json:"artifacts,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type SubmitRequest struct {
	AgentID         string          `json:"agent_id"`
	Message         string          `json:"message"`
	StructuredInput json.RawMessage `json:"structured_input"`
	AttachmentIDs   []string        `json:"attachment_ids"`
}

type CancelResult struct {
	Run     Run
	Deliver bool
}

type Assignment struct {
	RunID          string
	LeaseEpoch     uint64
	Manifest       []byte
	ManifestDigest string
}

type RunnerEvent struct {
	ClientSequence uint64
	Type           string
	Payload        string
}

type ExecutionGrant struct {
	ActionID  string
	CallID    string
	PermitID  string
	ExpiresAt time.Time
}

type RecordEventsResult struct {
	Sequence uint64
	Grant    *ExecutionGrant
}

type Artifact struct {
	ID                 string    `json:"id"`
	TaskID             string    `json:"task_id"`
	RunID              string    `json:"run_id"`
	Filename           string    `json:"filename"`
	MediaType          string    `json:"media_type"`
	SizeBytes          int64     `json:"size_bytes"`
	Digest             string    `json:"digest"`
	Classification     string    `json:"classification"`
	ScanStatus         string    `json:"scan_status"`
	State              string    `json:"state"`
	DownloadURL        string    `json:"download_url,omitempty"`
	DownloadURLExpires time.Time `json:"download_url_expires_at,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type TaskRun struct {
	TaskID string
	Run    Run
}

func newID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}

func boundedLimit(limit int) int {
	if limit < 1 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func publicStatus(status string) string {
	if status == "assigned" || status == "accepted" {
		return "running"
	}
	return status
}
