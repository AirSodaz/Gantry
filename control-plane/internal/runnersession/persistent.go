package runnersession

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	runnerv1 "github.com/AirSodaz/gantry/gen/gantry/runner/v1"
	"github.com/AirSodaz/gantry/internal/tasks"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// PersistentScheduler keeps only live streams in memory. Task and run state is
// owned by the task service's PostgreSQL transactions.
type PersistentScheduler struct {
	mu      sync.Mutex
	logger  *slog.Logger
	tasks   runCoordinator
	runners map[string]*persistentRunner
	nextID  uint64
}

// runCoordinator is the durable lifecycle contract required by runner
// sessions. It keeps stream protocol handling independent of PostgreSQL.
type runCoordinator interface {
	ClaimNext(context.Context, string) (tasks.Assignment, bool, error)
	Accept(context.Context, string, string, uint64, string) error
	RecordEvents(context.Context, string, string, uint64, []tasks.RunnerEvent) (uint64, error)
	RecordControlEvent(context.Context, string, string, uint64, string, string) error
	Finish(context.Context, string, string, uint64, string, string) error
	FailActive(context.Context, string, string, string) error
}

type persistentRunner struct {
	sessionID     string
	lastMessageID uint64
	activeRunID   string
	activeEpoch   uint64
	outbound      chan *runnerv1.ControlPlaneMessage
}

type semanticEvent interface {
	ProtoReflect() protoreflect.Message
	GetRunId() string
	GetLeaseEpoch() uint64
}

func NewPersistentScheduler(logger *slog.Logger, taskService runCoordinator) *PersistentScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PersistentScheduler{logger: logger, tasks: taskService, runners: make(map[string]*persistentRunner)}
}

func (s *PersistentScheduler) Register(runnerID, sessionID string, messageID uint64) (<-chan *runnerv1.ControlPlaneMessage, error) {
	if runnerID == "" || sessionID == "" || messageID == 0 {
		return nil, fmt.Errorf("runner registration requires non-empty runner, session, and message IDs")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runners[runnerID]; exists {
		return nil, fmt.Errorf("runner %q already has an active session", runnerID)
	}
	runner := &persistentRunner{sessionID: sessionID, lastMessageID: messageID, outbound: make(chan *runnerv1.ControlPlaneMessage, 8)}
	s.runners[runnerID] = runner
	s.logger.Info("persistent runner registered", "runner_id", runnerID, "session_id", sessionID)
	if err := s.assignLocked(context.Background(), runnerID, runner); err != nil {
		delete(s.runners, runnerID)
		return nil, err
	}
	return runner.outbound, nil
}

func (s *PersistentScheduler) Dispatch(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for runnerID, runner := range s.runners {
		if runner.activeRunID != "" {
			continue
		}
		if err := s.assignLocked(ctx, runnerID, runner); err != nil {
			return err
		}
	}
	return nil
}

func (s *PersistentScheduler) assignLocked(ctx context.Context, runnerID string, runner *persistentRunner) error {
	assignment, ok, err := s.tasks.ClaimNext(ctx, runnerID)
	if err != nil || !ok {
		return err
	}
	runner.activeRunID, runner.activeEpoch = assignment.RunID, assignment.LeaseEpoch
	s.sendLocked(runner, &runnerv1.ControlPlaneMessage{CorrelationId: assignment.RunID, Payload: &runnerv1.ControlPlaneMessage_AssignRun{AssignRun: &runnerv1.AssignRun{RunId: assignment.RunID, LeaseEpoch: assignment.LeaseEpoch, Manifest: assignment.Manifest, ManifestDigest: assignment.ManifestDigest}}})
	s.logger.Info("persistent run assigned", "runner_id", runnerID, "run_id", assignment.RunID, "lease_epoch", assignment.LeaseEpoch)
	return nil
}

func (s *PersistentScheduler) Handle(runnerID, sessionID string, message *runnerv1.RunnerMessage) error {
	if message == nil {
		return fmt.Errorf("runner message is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runner, err := s.validateLocked(runnerID, sessionID, message)
	if err != nil {
		return err
	}
	switch {
	case message.GetHeartbeat() != nil:
		return s.handleHeartbeatLocked(runner, message.GetHeartbeat())
	case message.GetRunAccepted() != nil:
		accepted := message.GetRunAccepted()
		if runner.activeRunID != accepted.GetRunId() || runner.activeEpoch != accepted.GetLeaseEpoch() {
			return fmt.Errorf("invalid run acceptance")
		}
		return s.tasks.Accept(context.Background(), runnerID, accepted.GetRunId(), accepted.GetLeaseEpoch(), accepted.GetManifestDigest())
	case message.GetEventBatch() != nil:
		batch := message.GetEventBatch()
		if runner.activeRunID != batch.GetRunId() || runner.activeEpoch != batch.GetLeaseEpoch() {
			return fmt.Errorf("invalid run event lease")
		}
		events := make([]tasks.RunnerEvent, 0, len(batch.GetEvents()))
		for _, event := range batch.GetEvents() {
			payload := "{}"
			if event.GetPayload() != nil {
				serialized, err := protojson.Marshal(event.GetPayload())
				if err != nil {
					return err
				}
				payload = string(serialized)
			}
			events = append(events, tasks.RunnerEvent{ClientSequence: event.GetClientSequence(), Type: event.GetEventType(), Payload: payload})
		}
		sequence, err := s.tasks.RecordEvents(context.Background(), runnerID, batch.GetRunId(), batch.GetLeaseEpoch(), events)
		if err != nil {
			return err
		}
		s.sendLocked(runner, &runnerv1.ControlPlaneMessage{CorrelationId: batch.GetRunId(), Payload: &runnerv1.ControlPlaneMessage_AcknowledgeEvents{AcknowledgeEvents: &runnerv1.AcknowledgeEvents{RunId: batch.GetRunId(), LastAcknowledgedSequence: sequence}}})
		return nil
	case message.GetModelUsage() != nil:
		return s.recordSemanticEventLocked(runnerID, runner, "model.usage", message.GetModelUsage())
	case message.GetCheckpointMetadata() != nil:
		return s.recordSemanticEventLocked(runnerID, runner, "run.checkpoint_created", message.GetCheckpointMetadata())
	case message.GetModelDelta() != nil:
		return s.recordSemanticEventLocked(runnerID, runner, "model.delta", message.GetModelDelta())
	case message.GetToolLifecycle() != nil:
		return s.recordSemanticEventLocked(runnerID, runner, "tool.lifecycle", message.GetToolLifecycle())
	case message.GetSecurityEvent() != nil:
		return s.recordSemanticEventLocked(runnerID, runner, "security.untrusted_context", message.GetSecurityEvent())
	case message.GetCompactionEvent() != nil:
		return s.recordSemanticEventLocked(runnerID, runner, "context.compacted", message.GetCompactionEvent())
	case message.GetRunFinished() != nil:
		finished := message.GetRunFinished()
		if runner.activeRunID != finished.GetRunId() || runner.activeEpoch != finished.GetLeaseEpoch() {
			return fmt.Errorf("invalid run completion lease")
		}
		terminal, err := terminalStatus(finished.GetStatus())
		if err != nil {
			return err
		}
		if err := s.tasks.Finish(context.Background(), runnerID, finished.GetRunId(), finished.GetLeaseEpoch(), terminal, finished.GetReason()); err != nil {
			return err
		}
		runner.activeRunID, runner.activeEpoch = "", 0
		return s.assignLocked(context.Background(), runnerID, runner)
	default:
		return fmt.Errorf("unsupported runner message payload")
	}
}

func (s *PersistentScheduler) recordSemanticEventLocked(runnerID string, runner *persistentRunner, eventType string, message semanticEvent) error {
	if runner.activeRunID == "" || runner.activeEpoch == 0 || message.GetRunId() != runner.activeRunID || message.GetLeaseEpoch() != runner.activeEpoch {
		return fmt.Errorf("semantic event has no active run")
	}
	payload, err := protojson.Marshal(message)
	if err != nil {
		return err
	}
	return s.tasks.RecordControlEvent(context.Background(), runnerID, runner.activeRunID, runner.activeEpoch, eventType, string(payload))
}

func (s *PersistentScheduler) RequestCancel(runID string, epoch uint64, reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, runner := range s.runners {
		if runner.activeRunID != runID || runner.activeEpoch != epoch {
			continue
		}
		s.sendLocked(runner, &runnerv1.ControlPlaneMessage{CorrelationId: runID, Payload: &runnerv1.ControlPlaneMessage_CancelRun{CancelRun: &runnerv1.CancelRun{RunId: runID, LeaseEpoch: epoch, Reason: reason}}})
		return true
	}
	return false
}

func (s *PersistentScheduler) ResolveApproval(runID, approvalID, decision, reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, runner := range s.runners {
		if runner.activeRunID != runID {
			continue
		}
		controlDecision := runnerv1.ApprovalDecisionType_APPROVAL_DECISION_TYPE_REJECTED
		if decision == "approve" {
			controlDecision = runnerv1.ApprovalDecisionType_APPROVAL_DECISION_TYPE_APPROVED
		}
		s.sendLocked(runner, &runnerv1.ControlPlaneMessage{CorrelationId: runID, Payload: &runnerv1.ControlPlaneMessage_ApprovalResolution{ApprovalResolution: &runnerv1.ApprovalResolution{RunId: runID, LeaseEpoch: runner.activeEpoch, ApprovalRequestId: approvalID, Decision: controlDecision, Reason: reason}}})
		return true
	}
	return false
}

func (s *PersistentScheduler) Disconnect(runnerID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	runner := s.runners[runnerID]
	if runner == nil || runner.sessionID != sessionID {
		return
	}
	if runner.activeRunID != "" {
		if err := s.tasks.FailActive(context.Background(), runnerID, runner.activeRunID, "runner disconnected"); err != nil {
			s.logger.Error("could not persist runner disconnect", "error", err, "run_id", runner.activeRunID)
		}
	}
	delete(s.runners, runnerID)
}

func (s *PersistentScheduler) validateLocked(runnerID, sessionID string, message *runnerv1.RunnerMessage) (*persistentRunner, error) {
	runner := s.runners[runnerID]
	if runner == nil || runner.sessionID != sessionID {
		return nil, fmt.Errorf("runner session is not registered")
	}
	if message.GetProtocolVersion() != protocolVersion || message.GetRunnerId() != runnerID || message.GetSessionId() != sessionID || message.GetMessageId() <= runner.lastMessageID {
		return nil, fmt.Errorf("invalid runner message envelope")
	}
	runner.lastMessageID = message.GetMessageId()
	return runner, nil
}
func (s *PersistentScheduler) handleHeartbeatLocked(runner *persistentRunner, heartbeat *runnerv1.Heartbeat) error {
	if heartbeat.GetRunId() == "" {
		return nil
	}
	if heartbeat.GetRunId() != runner.activeRunID || heartbeat.GetLeaseEpoch() != runner.activeEpoch {
		return fmt.Errorf("heartbeat has an invalid run lease")
	}
	return nil
}
func (s *PersistentScheduler) sendLocked(runner *persistentRunner, message *runnerv1.ControlPlaneMessage) {
	s.nextID++
	message.MessageId = fmt.Sprintf("control-%d", s.nextID)
	runner.outbound <- message
}
func terminalStatus(status runnerv1.RunTerminalStatus) (string, error) {
	switch status {
	case runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_COMPLETED:
		return "completed", nil
	case runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_FAILED:
		return "failed", nil
	case runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_CANCELED:
		return "canceled", nil
	default:
		return "", fmt.Errorf("invalid terminal status")
	}
}
