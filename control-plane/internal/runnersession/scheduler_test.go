package runnersession

import (
	"log/slog"
	"testing"

	runnerv1 "github.com/AirSodaz/gantry/gen/gantry/runner/v1"
)

func TestSchedulerCompletesRunAndAcknowledgesContiguousEvents(t *testing.T) {
	scheduler := NewScheduler(slog.Default())
	outbound, err := scheduler.Register("runner-1", "session-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	run, err := scheduler.SubmitRun("run-1", []byte("manifest"), "sha256:manifest")
	if err != nil {
		t.Fatal(err)
	}
	assignment := <-outbound
	if assignment.GetAssignRun() == nil || assignment.GetAssignRun().GetRunId() != run.ID {
		t.Fatalf("assignment = %#v", assignment)
	}
	if err := scheduler.Handle("runner-1", "session-1", accepted("runner-1", "session-1", 2, run)); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Handle("runner-1", "session-1", events("runner-1", "session-1", 3, run, 1, 2)); err != nil {
		t.Fatal(err)
	}
	ack := <-outbound
	if ack.GetAcknowledgeEvents().GetLastAcknowledgedSequence() != 2 {
		t.Fatalf("ack = %#v", ack)
	}
	if err := scheduler.Handle("runner-1", "session-1", finished("runner-1", "session-1", 4, run, runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_COMPLETED)); err != nil {
		t.Fatal(err)
	}
	completed, _ := scheduler.Run(run.ID)
	if completed.Status != RunStatusCompleted {
		t.Fatalf("status = %s", completed.Status)
	}
	if _, err := scheduler.SubmitRun("run-2", []byte("manifest"), "sha256:next"); err != nil {
		t.Fatalf("runner was not released: %v", err)
	}
}

func TestSchedulerCancelsRunAndFailsRunOnDisconnect(t *testing.T) {
	scheduler := NewScheduler(slog.Default())
	outbound, _ := scheduler.Register("runner-1", "session-1", 1)
	run, _ := scheduler.SubmitRun("run-1", []byte("manifest"), "sha256:manifest")
	<-outbound
	if err := scheduler.Handle("runner-1", "session-1", accepted("runner-1", "session-1", 2, run)); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.CancelRun(run.ID, "requested"); err != nil {
		t.Fatal(err)
	}
	cancel := <-outbound
	if cancel.GetCancelRun() == nil || cancel.GetCancelRun().GetLeaseEpoch() != run.LeaseEpoch {
		t.Fatalf("cancel = %#v", cancel)
	}
	if err := scheduler.Handle("runner-1", "session-1", finished("runner-1", "session-1", 3, run, runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_CANCELED)); err != nil {
		t.Fatal(err)
	}
	canceled, _ := scheduler.Run(run.ID)
	if canceled.Status != RunStatusCanceled {
		t.Fatalf("status = %s", canceled.Status)
	}
	run, _ = scheduler.SubmitRun("run-2", []byte("manifest"), "sha256:next")
	<-outbound
	scheduler.Disconnect("runner-1", "session-1")
	failed, _ := scheduler.Run(run.ID)
	if failed.Status != RunStatusFailed {
		t.Fatalf("status = %s", failed.Status)
	}
}

func TestSchedulerRejectsInvalidLeaseAndEventSequences(t *testing.T) {
	scheduler := NewScheduler(slog.Default())
	outbound, _ := scheduler.Register("runner-1", "session-1", 1)
	run, _ := scheduler.SubmitRun("run-1", []byte("manifest"), "sha256:manifest")
	<-outbound
	if err := scheduler.Handle("runner-1", "session-1", accepted("runner-1", "session-1", 1, run)); err == nil {
		t.Fatal("accepted duplicate message ID")
	}
	if err := scheduler.Handle("runner-1", "session-1", accepted("runner-1", "session-1", 2, run)); err != nil {
		t.Fatal(err)
	}
	wrongLease := *run
	wrongLease.LeaseEpoch++
	if err := scheduler.Handle("runner-1", "session-1", events("runner-1", "session-1", 3, &wrongLease, 1)); err == nil {
		t.Fatal("accepted wrong lease")
	}
	if err := scheduler.Handle("runner-1", "session-1", events("runner-1", "session-1", 4, run, 2)); err == nil {
		t.Fatal("accepted skipped event sequence")
	}
}

func accepted(runnerID, sessionID string, messageID uint64, run *Run) *runnerv1.RunnerMessage {
	return &runnerv1.RunnerMessage{RunnerId: runnerID, SessionId: sessionID, MessageId: messageID, ProtocolVersion: protocolVersion, Payload: &runnerv1.RunnerMessage_RunAccepted{RunAccepted: &runnerv1.RunAccepted{RunId: run.ID, LeaseEpoch: run.LeaseEpoch, ManifestDigest: run.ManifestDigest}}}
}
func events(runnerID, sessionID string, messageID uint64, run *Run, sequences ...uint64) *runnerv1.RunnerMessage {
	events := make([]*runnerv1.RunEvent, 0, len(sequences))
	for _, sequence := range sequences {
		events = append(events, &runnerv1.RunEvent{ClientSequence: sequence, EventType: "model.delta"})
	}
	return &runnerv1.RunnerMessage{RunnerId: runnerID, SessionId: sessionID, MessageId: messageID, ProtocolVersion: protocolVersion, Payload: &runnerv1.RunnerMessage_EventBatch{EventBatch: &runnerv1.RunEventBatch{RunId: run.ID, LeaseEpoch: run.LeaseEpoch, Events: events}}}
}
func finished(runnerID, sessionID string, messageID uint64, run *Run, status runnerv1.RunTerminalStatus) *runnerv1.RunnerMessage {
	return &runnerv1.RunnerMessage{RunnerId: runnerID, SessionId: sessionID, MessageId: messageID, ProtocolVersion: protocolVersion, Payload: &runnerv1.RunnerMessage_RunFinished{RunFinished: &runnerv1.RunFinished{RunId: run.ID, LeaseEpoch: run.LeaseEpoch, Status: status}}}
}
