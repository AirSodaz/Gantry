// Package runs owns durable Run execution, lifecycle, and runner artifacts.
package runs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/objectstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid run input")
	ErrInvalidState = errors.New("invalid run state")
)

type Service struct {
	pool      *pgxpool.Pool
	approvals *approvals.Service
	store     objectstore.ArtifactStore
	content   contentBufferRegistry
}

func NewService(pool *pgxpool.Pool, approvalService *approvals.Service, store objectstore.ArtifactStore) *Service {
	return &Service{pool: pool, approvals: approvalService, store: store, content: contentBufferRegistry{streams: make(map[contentStreamKey]*contentStream)}}
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
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	RunID          string    `json:"run_id"`
	Filename       string    `json:"filename"`
	MediaType      string    `json:"media_type"`
	SizeBytes      int64     `json:"size_bytes"`
	Digest         string    `json:"digest"`
	Classification string    `json:"classification"`
	ScanStatus     string    `json:"scan_state"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"created_at"`
}
type ArtifactDownloadGrant struct {
	ArtifactID  string    `json:"artifact_id"`
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}
type ArtifactCursor struct {
	CreatedAt time.Time
	ID        string
}
type ArtifactPage struct {
	Items   []Artifact
	HasMore bool
}
type Event struct {
	Type    string
	Payload json.RawMessage
}

type Coordinator interface {
	ClaimNext(context.Context, string) (Assignment, bool, error)
	Accept(context.Context, string, string, uint64, string) error
	RecordEvents(context.Context, string, string, uint64, []RunnerEvent) (RecordEventsResult, error)
	RecordControlEvent(context.Context, string, string, uint64, string, string) error
	Finish(context.Context, string, string, uint64, string, string) error
	FailActive(context.Context, string, string, string) error
}
type ArtifactCoordinator interface {
	DeclareArtifact(context.Context, string, string, uint64, Artifact) (Artifact, string, time.Time, error)
	UploadArtifact(context.Context, string, string, io.Reader) error
}
type contentStreamKey struct{ runID, streamID string }
type contentBufferRegistry struct {
	mu      sync.Mutex
	streams map[contentStreamKey]*contentStream
}

func newID(prefix string) string { return prefix + "_" + strconv.FormatInt(time.Now().UnixNano(), 36) }
func boundedLimit(limit int) int {
	if limit < 1 {
		return 25
	}
	if limit > 100 {
		return 100
	}
	return limit
}
