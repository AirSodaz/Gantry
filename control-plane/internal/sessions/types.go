package sessions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrInvalidInput         = errors.New("invalid session input")
	ErrInvalidState         = errors.New("invalid session state")
	ErrIdempotencyConflict  = errors.New("idempotency key reused with a different request")
	ErrPreconditionRequired = errors.New("session conversation precondition is required")
	ErrConversationChanged  = errors.New("session conversation changed")
)

type Service struct {
	pool        *pgxpool.Pool
	approvals   *approvals.Service
	attachments AttachmentStore
	artifacts   ArtifactReader
	content     ContentReader
}

// AttachmentStore contains only the object operations owned by Sessions.
type AttachmentStore interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Head(context.Context, string) (int64, error)
}

// ContentReader supplies Run-owned content segments for the Session event
// projection. The same object store also owns attachment writes.
type ContentReader interface {
	Get(context.Context, string) (io.ReadCloser, error)
}

// ArtifactReader lets Session projections include Run-owned artifacts without
// making Sessions responsible for artifact lifecycle or storage.
type ArtifactReader interface {
	ListSessionArtifacts(context.Context, identity.Principal, string, int) ([]runs.Artifact, error)
}

func NewService(pool *pgxpool.Pool, approvalService *approvals.Service, attachments AttachmentStore, artifacts ArtifactReader) *Service {
	content, _ := attachments.(ContentReader)
	return &Service{pool: pool, approvals: approvalService, attachments: attachments, artifacts: artifacts, content: content}
}

type Agent struct {
	ID                string            `json:"id"`
	DisplayName       string            `json:"display_name"`
	Description       string            `json:"description"`
	Category          string            `json:"category"`
	Owner             SupportContact    `json:"owner"`
	InputContract     json.RawMessage   `json:"input_contract"`
	PublishedMetadata PublishedMetadata `json:"published_metadata"`
	Availability      Availability      `json:"availability"`
	IsFavorite        bool              `json:"is_favorite"`
	LastUsedAt        *time.Time        `json:"last_used_at"`
	OwnerName         string            `json:"-"`
}

type SupportContact struct {
	PrincipalID string `json:"principal_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}
type PublishedMetadata struct {
	TypicalInputs     []any          `json:"typical_inputs"`
	ExpectedOutput    map[string]any `json:"expected_output"`
	CapabilitySummary string         `json:"capability_summary"`
	DataDisclosure    map[string]any `json:"data_disclosure"`
	ActionDisclosure  map[string]any `json:"action_disclosure"`
}
type Availability struct {
	State          string     `json:"state"`
	ReasonCode     *string    `json:"reason_code"`
	Message        *string    `json:"message"`
	EffectiveUntil *time.Time `json:"effective_until"`
}
type AgentCursor struct {
	DisplayName string
	LastUsedAt  *time.Time
	ID          string
}
type AgentPage struct {
	Items   []Agent
	HasMore bool
}
type Run struct {
	ID                        string            `json:"id"`
	SessionSequence           int64             `json:"session_sequence"`
	RequesterID               string            `json:"requester_id"`
	State                     string            `json:"state"`
	Outcome                   *string           `json:"outcome"`
	StateReason               *UserFacingReason `json:"state_reason"`
	RetryOfRunID              *string           `json:"retry_of_run_id"`
	TriggerOccurrenceID       *string           `json:"trigger_occurrence_id"`
	CreatedAt                 time.Time         `json:"created_at"`
	StartedAt                 *time.Time        `json:"started_at"`
	CompletedAt               *time.Time        `json:"completed_at"`
	Status                    string            `json:"-"`
	Reason                    string            `json:"-"`
	LeaseEpoch                uint64            `json:"-"`
	AcknowledgedEventSequence uint64            `json:"-"`
}

type UserFacingReason struct {
	Code          string  `json:"code"`
	Message       string  `json:"message"`
	NextAction    string  `json:"next_action,omitempty"`
	CorrelationID *string `json:"correlation_id"`
}

type Session struct {
	ID                   string               `json:"id"`
	OwnerPrincipalID     string               `json:"owner_principal_id"`
	Mode                 string               `json:"mode"`
	SourceTags           []string             `json:"source_tags,omitempty"`
	Agent                SessionAgentSnapshot `json:"agent"`
	Title                *string              `json:"title"`
	State                string               `json:"state"`
	ConversationRevision int64                `json:"conversation_revision"`
	MyAction             string               `json:"my_action"`
	ExecutingRun         *Run                 `json:"executing_run"`
	QueuedRunCount       int                  `json:"queued_run_count"`
	Members              []SessionMember      `json:"members"`
	Messages             []Message            `json:"messages"`
	Artifacts            []runs.Artifact      `json:"artifacts,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

type SessionAgentSnapshot struct {
	AgentID        string          `json:"agent_id"`
	DisplayName    string          `json:"display_name"`
	SupportContact *SupportContact `json:"support_contact,omitempty"`
}
type SessionMember struct {
	PrincipalID string    `json:"principal_id"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

func (t Session) ConversationETag() string {
	return `"` + strconv.FormatInt(t.ConversationRevision, 10) + `"`
}

type ListFilter struct {
	State        string
	Mode         string
	AgentID      string
	MyAction     string
	UpdatedAfter *time.Time
}

type SessionCursor struct {
	UpdatedAt time.Time
	ID        string
}

type SessionPage struct {
	Items   []Session
	HasMore bool
}

type Message struct {
	ID                string          `json:"id"`
	RunID             *string         `json:"run_id"`
	SessionSequence   int64           `json:"session_sequence"`
	AuthorKind        string          `json:"author_kind"`
	AuthorPrincipalID *string         `json:"author_principal_id"`
	TriggerID         *string         `json:"trigger_id"`
	Parts             json.RawMessage `json:"parts"`
	Content           string          `json:"-"`
	CreatedAt         time.Time       `json:"created_at"`
}

type RunCursor struct {
	SessionSequence int
	ID              string
}

type RunPage struct {
	Items   []Run
	HasMore bool
}
type SubmitRequest struct {
	AgentID         string          `json:"agent_id"`
	Message         string          `json:"message"`
	StructuredInput json.RawMessage `json:"structured_input"`
	AttachmentIDs   []string        `json:"attachment_ids"`
}

type SetAgentFavoriteRequest struct {
	IsFavorite bool `json:"is_favorite"`
}

type AppendMessageRequest struct {
	Message       string   `json:"message"`
	AttachmentIDs []string `json:"attachment_ids"`
}

type AddMemberRequest struct {
	PrincipalID string `json:"principal_id"`
	Role        string `json:"role"`
}
type UpdateMemberRequest struct {
	Role string `json:"role"`
}
type TransferOwnerRequest struct {
	NewOwnerPrincipalID string `json:"new_owner_principal_id"`
}
type MemberCursor struct {
	JoinedAt    time.Time
	PrincipalID string
}
type MemberPage struct {
	Items   []SessionMember
	HasMore bool
}

type CancelResult struct {
	Run     Run
	Deliver bool
}

// RetryResult is the durable command receipt for a retry. Run always names
// the exact queued Run created by the command or recovered from its replay.
type RetryResult struct {
	Run       Run
	Duplicate bool
}

// Attachment is a requester-owned input object. Its object key and upload
// credential are deliberately absent from regular metadata reads.
type Attachment struct {
	ID              string            `json:"id"`
	Filename        string            `json:"filename"`
	MediaType       string            `json:"media_type"`
	SizeBytes       int64             `json:"size_bytes"`
	Digest          string            `json:"digest"`
	Classification  string            `json:"classification"`
	ScanStatus      string            `json:"scan_state"`
	State           string            `json:"state"`
	RejectionReason *UserFacingReason `json:"rejection_reason,omitempty"`
	BoundSessionID  *string           `json:"bound_session_id"`
	ExpiresAt       time.Time         `json:"expires_at"`
	UploadURL       string            `json:"-"`
	UploadToken     string            `json:"upload_token,omitempty"`
	UploadExpires   time.Time         `json:"-"`
	CreatedAt       time.Time         `json:"created_at"`
}

type AttachmentUploadGrant struct {
	Attachment  Attachment `json:"attachment"`
	UploadPath  string     `json:"upload_path"`
	UploadToken string     `json:"upload_token"`
	ExpiresAt   time.Time  `json:"expires_at"`
}

type CreateAttachmentRequest struct {
	Filename       string `json:"filename"`
	MediaType      string `json:"media_type"`
	SizeBytes      int64  `json:"size_bytes"`
	Digest         string `json:"digest"`
	Classification string `json:"classification"`
}

type SessionRun struct {
	SessionID string
	Run       Run
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
