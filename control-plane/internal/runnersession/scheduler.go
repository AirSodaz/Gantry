package runnersession

import (
	"fmt"
	"log/slog"
	"sync"

	runnerv1 "github.com/AirSodaz/gantry/gen/gantry/runner/v1"
)

const protocolVersion = 1

type RunStatus string

const (
	RunStatusAssigned  RunStatus = "assigned"
	RunStatusAccepted  RunStatus = "accepted"
	RunStatusCanceling RunStatus = "canceling"
	RunStatusCompleted RunStatus = "completed"
	RunStatusCanceled  RunStatus = "canceled"
	RunStatusFailed    RunStatus = "failed"
)

type Run struct {
	ID                string
	RunnerID          string
	LeaseEpoch        uint64
	Manifest          []byte
	ManifestDigest    string
	Status            RunStatus
	LastEventSequence uint64
}

type runner struct {
	sessionID     string
	lastMessageID uint64
	leaseEpoch    uint64
	activeRunID   string
	outbound      chan *runnerv1.ControlPlaneMessage
}

// Scheduler is an in-memory Phase 0 lifecycle coordinator. It intentionally
// does not survive a process restart and is not a public task scheduler.
type Scheduler struct {
	mu                   sync.Mutex
	logger               *slog.Logger
	runners              map[string]*runner
	runs                 map[string]*Run
	nextControlMessageID uint64
}

func NewScheduler(logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{logger: logger, runners: make(map[string]*runner), runs: make(map[string]*Run)}
}

func (s *Scheduler) Register(runnerID, sessionID string, messageID uint64) (<-chan *runnerv1.ControlPlaneMessage, error) {
	if runnerID == "" || sessionID == "" || messageID == 0 {
		return nil, fmt.Errorf("runner registration requires non-empty runner, session, and message IDs")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runners[runnerID]; exists {
		return nil, fmt.Errorf("runner %q already has an active session", runnerID)
	}
	state := &runner{sessionID: sessionID, lastMessageID: messageID, outbound: make(chan *runnerv1.ControlPlaneMessage, 8)}
	s.runners[runnerID] = state
	s.logger.Info("runner lifecycle", "event", "registered", "runner_id", runnerID, "session_id", sessionID)
	return state.outbound, nil
}

func (s *Scheduler) SubmitDemoRun(runID string, manifest []byte, manifestDigest string) (*Run, error) {
	if runID == "" || len(manifest) == 0 || manifestDigest == "" {
		return nil, fmt.Errorf("demo run requires ID, manifest, and manifest digest")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[runID]; exists {
		return nil, fmt.Errorf("run %q already exists", runID)
	}
	for runnerID, state := range s.runners {
		if state.activeRunID != "" {
			continue
		}
		state.leaseEpoch++
		run := &Run{ID: runID, RunnerID: runnerID, LeaseEpoch: state.leaseEpoch, Manifest: append([]byte(nil), manifest...), ManifestDigest: manifestDigest, Status: RunStatusAssigned}
		s.runs[runID] = run
		state.activeRunID = runID
		s.sendLocked(state, &runnerv1.ControlPlaneMessage{CorrelationId: runID, Payload: &runnerv1.ControlPlaneMessage_AssignRun{AssignRun: &runnerv1.AssignRun{RunId: runID, LeaseEpoch: run.LeaseEpoch, Manifest: run.Manifest, ManifestDigest: manifestDigest}}})
		s.logger.Info("runner lifecycle", "event", "assigned", "run_id", runID, "runner_id", runnerID, "lease_epoch", run.LeaseEpoch)
		return copyRun(run), nil
	}
	return nil, fmt.Errorf("no idle registered runner")
}

func (s *Scheduler) CancelRun(runID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, exists := s.runs[runID]
	if !exists {
		return fmt.Errorf("run %q does not exist", runID)
	}
	if isTerminal(run.Status) || run.Status == RunStatusCanceling {
		return nil
	}
	state := s.runners[run.RunnerID]
	if state == nil || state.activeRunID != runID {
		return fmt.Errorf("run %q no longer has an active runner session", runID)
	}
	run.Status = RunStatusCanceling
	s.sendLocked(state, &runnerv1.ControlPlaneMessage{CorrelationId: runID, Payload: &runnerv1.ControlPlaneMessage_CancelRun{CancelRun: &runnerv1.CancelRun{RunId: runID, LeaseEpoch: run.LeaseEpoch, Reason: reason}}})
	s.logger.Info("runner lifecycle", "event", "cancel_requested", "run_id", runID, "runner_id", run.RunnerID, "lease_epoch", run.LeaseEpoch)
	return nil
}

func (s *Scheduler) Handle(runnerID, sessionID string, message *runnerv1.RunnerMessage) error {
	if message == nil {
		return fmt.Errorf("runner message is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.validateMessageLocked(runnerID, sessionID, message)
	if err != nil {
		return err
	}
	switch {
	case message.GetHeartbeat() != nil:
		return s.handleHeartbeatLocked(state, message.GetHeartbeat())
	case message.GetRunAccepted() != nil:
		return s.handleAcceptedLocked(runnerID, state, message.GetRunAccepted())
	case message.GetEventBatch() != nil:
		return s.handleEventBatchLocked(runnerID, state, message.GetEventBatch())
	case message.GetRunFinished() != nil:
		return s.handleFinishedLocked(runnerID, state, message.GetRunFinished())
	default:
		return fmt.Errorf("unsupported runner message payload")
	}
}

func (s *Scheduler) Disconnect(runnerID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.runners[runnerID]
	if state == nil || state.sessionID != sessionID {
		return
	}
	if state.activeRunID != "" {
		if run := s.runs[state.activeRunID]; run != nil && !isTerminal(run.Status) {
			run.Status = RunStatusFailed
			s.logger.Warn("runner lifecycle", "event", "runner_disconnected", "run_id", run.ID, "runner_id", runnerID, "lease_epoch", run.LeaseEpoch)
		}
	}
	delete(s.runners, runnerID)
}

func (s *Scheduler) Run(runID string) (*Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	return copyRun(run), ok
}

func (s *Scheduler) validateMessageLocked(runnerID, sessionID string, message *runnerv1.RunnerMessage) (*runner, error) {
	state := s.runners[runnerID]
	if state == nil || state.sessionID != sessionID {
		return nil, fmt.Errorf("runner session is not registered")
	}
	if message.GetProtocolVersion() != protocolVersion || message.GetRunnerId() != runnerID || message.GetSessionId() != sessionID || message.GetMessageId() <= state.lastMessageID {
		return nil, fmt.Errorf("invalid runner message envelope")
	}
	state.lastMessageID = message.GetMessageId()
	return state, nil
}

func (s *Scheduler) handleHeartbeatLocked(state *runner, heartbeat *runnerv1.Heartbeat) error {
	if heartbeat.GetRunId() == "" {
		return nil
	}
	run := s.runs[heartbeat.GetRunId()]
	if run == nil || state.activeRunID != run.ID || heartbeat.GetLeaseEpoch() != run.LeaseEpoch {
		return fmt.Errorf("heartbeat has an invalid run lease")
	}
	return nil
}

func (s *Scheduler) handleAcceptedLocked(runnerID string, state *runner, accepted *runnerv1.RunAccepted) error {
	run, err := s.activeRunLocked(runnerID, state, accepted.GetRunId(), accepted.GetLeaseEpoch())
	if err != nil || run.Status != RunStatusAssigned || accepted.GetManifestDigest() != run.ManifestDigest {
		return fmt.Errorf("invalid run acceptance")
	}
	run.Status = RunStatusAccepted
	s.logger.Info("runner lifecycle", "event", "accepted", "run_id", run.ID, "runner_id", runnerID, "lease_epoch", run.LeaseEpoch)
	return nil
}

func (s *Scheduler) handleEventBatchLocked(runnerID string, state *runner, batch *runnerv1.RunEventBatch) error {
	run, err := s.activeRunLocked(runnerID, state, batch.GetRunId(), batch.GetLeaseEpoch())
	if err != nil || (run.Status != RunStatusAccepted && run.Status != RunStatusCanceling) || len(batch.GetEvents()) == 0 {
		return fmt.Errorf("invalid run event batch")
	}
	sequence := run.LastEventSequence
	for _, event := range batch.GetEvents() {
		if event.GetClientSequence() != sequence+1 {
			return fmt.Errorf("run event sequence is not contiguous")
		}
		sequence++
	}
	run.LastEventSequence = sequence
	s.sendLocked(state, &runnerv1.ControlPlaneMessage{CorrelationId: run.ID, Payload: &runnerv1.ControlPlaneMessage_AcknowledgeEvents{AcknowledgeEvents: &runnerv1.AcknowledgeEvents{RunId: run.ID, LastAcknowledgedSequence: sequence}}})
	s.logger.Info("runner lifecycle", "event", "events_acknowledged", "run_id", run.ID, "runner_id", runnerID, "lease_epoch", run.LeaseEpoch, "last_event_sequence", sequence)
	return nil
}

func (s *Scheduler) handleFinishedLocked(runnerID string, state *runner, finished *runnerv1.RunFinished) error {
	run, err := s.activeRunLocked(runnerID, state, finished.GetRunId(), finished.GetLeaseEpoch())
	if err != nil || (run.Status != RunStatusAccepted && run.Status != RunStatusCanceling) {
		return fmt.Errorf("invalid run completion")
	}
	switch finished.GetStatus() {
	case runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_COMPLETED:
		if run.Status == RunStatusCanceling {
			return fmt.Errorf("canceling run cannot complete")
		}
		run.Status = RunStatusCompleted
	case runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_CANCELED:
		if run.Status != RunStatusCanceling {
			return fmt.Errorf("run was not canceled")
		}
		run.Status = RunStatusCanceled
	case runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_FAILED:
		run.Status = RunStatusFailed
	default:
		return fmt.Errorf("invalid terminal status")
	}
	state.activeRunID = ""
	s.logger.Info("runner lifecycle", "event", "finished", "run_id", run.ID, "runner_id", runnerID, "lease_epoch", run.LeaseEpoch, "status", run.Status)
	return nil
}

func (s *Scheduler) activeRunLocked(runnerID string, state *runner, runID string, leaseEpoch uint64) (*Run, error) {
	run := s.runs[runID]
	if run == nil || run.RunnerID != runnerID || state.activeRunID != runID || run.LeaseEpoch != leaseEpoch {
		return nil, fmt.Errorf("run lease is not active")
	}
	return run, nil
}

func (s *Scheduler) sendLocked(state *runner, message *runnerv1.ControlPlaneMessage) {
	s.nextControlMessageID++
	message.MessageId = fmt.Sprintf("control-%d", s.nextControlMessageID)
	state.outbound <- message
}

func isTerminal(status RunStatus) bool {
	return status == RunStatusCompleted || status == RunStatusCanceled || status == RunStatusFailed
}
func copyRun(run *Run) *Run {
	if run == nil {
		return nil
	}
	clone := *run
	clone.Manifest = append([]byte(nil), run.Manifest...)
	return &clone
}
