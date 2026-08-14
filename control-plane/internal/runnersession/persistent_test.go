package runnersession

import (
	"context"
	"testing"

	runnerv1 "github.com/AirSodaz/gantry/gen/gantry/runner/v1"
	"github.com/AirSodaz/gantry/internal/tasks"
)

type fakeRunCoordinator struct {
	assignment tasks.Assignment
	claimed    bool
	accepted   bool
	events     []tasks.RunnerEvent
	finished   string
	failedRun  string
}

func (c *fakeRunCoordinator) ClaimNext(context.Context, string) (tasks.Assignment, bool, error) {
	if c.claimed {
		return tasks.Assignment{}, false, nil
	}
	c.claimed = true
	return c.assignment, true, nil
}

func (c *fakeRunCoordinator) Accept(_ context.Context, _ string, _ string, _ uint64, _ string) error {
	c.accepted = true
	return nil
}

func (c *fakeRunCoordinator) RecordEvents(_ context.Context, _ string, _ string, _ uint64, events []tasks.RunnerEvent) (uint64, error) {
	c.events = append(c.events, events...)
	return events[len(events)-1].ClientSequence, nil
}

func (c *fakeRunCoordinator) RecordControlEvent(context.Context, string, string, uint64, string, string) error {
	return nil
}

func (c *fakeRunCoordinator) Finish(_ context.Context, _ string, _ string, _ uint64, terminal, _ string) error {
	c.finished = terminal
	return nil
}

func (c *fakeRunCoordinator) FailActive(_ context.Context, _ string, runID, _ string) error {
	c.failedRun = runID
	return nil
}

func TestPersistentSchedulerAcknowledgesEventsAndFinishes(t *testing.T) {
	store := &fakeRunCoordinator{assignment: tasks.Assignment{RunID: "run_1", LeaseEpoch: 4, Manifest: []byte(`{}`), ManifestDigest: "sha256:demo"}}
	scheduler := NewPersistentScheduler(nil, store)
	outbound, err := scheduler.Register("runner_1", "session_1", 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment := <-outbound
	if assignment.GetAssignRun().GetRunId() != "run_1" {
		t.Fatalf("assignment = %#v", assignment)
	}
	if err := scheduler.Handle("runner_1", "session_1", persistentAccepted(2, "run_1", 4)); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Handle("runner_1", "session_1", persistentEvents(3, "run_1", 4, 1, 2)); err != nil {
		t.Fatal(err)
	}
	acknowledgement := <-outbound
	if acknowledgement.GetAcknowledgeEvents().GetLastAcknowledgedSequence() != 2 {
		t.Fatalf("acknowledgement = %#v", acknowledgement)
	}
	if err := scheduler.Handle("runner_1", "session_1", persistentFinished(4, "run_1", 4, runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_COMPLETED)); err != nil {
		t.Fatal(err)
	}
	if !store.accepted || len(store.events) != 2 || store.finished != "completed" {
		t.Fatalf("store = %#v", store)
	}
}

func TestPersistentSchedulerRejectsStaleLeaseAndFailsOnDisconnect(t *testing.T) {
	store := &fakeRunCoordinator{assignment: tasks.Assignment{RunID: "run_1", LeaseEpoch: 4, Manifest: []byte(`{}`), ManifestDigest: "sha256:demo"}}
	scheduler := NewPersistentScheduler(nil, store)
	outbound, err := scheduler.Register("runner_1", "session_1", 1)
	if err != nil {
		t.Fatal(err)
	}
	<-outbound
	if err := scheduler.Handle("runner_1", "session_1", persistentAccepted(2, "run_1", 5)); err == nil {
		t.Fatal("accepted a stale lease")
	}
	if store.accepted {
		t.Fatal("stale lease reached durable coordinator")
	}
	scheduler.Disconnect("runner_1", "session_1")
	if store.failedRun != "run_1" {
		t.Fatalf("failed run = %q", store.failedRun)
	}
}

func persistentAccepted(messageID uint64, runID string, epoch uint64) *runnerv1.RunnerMessage {
	return &runnerv1.RunnerMessage{
		RunnerId:        "runner_1",
		SessionId:       "session_1",
		MessageId:       messageID,
		ProtocolVersion: protocolVersion,
		Payload: &runnerv1.RunnerMessage_RunAccepted{RunAccepted: &runnerv1.RunAccepted{
			RunId:          runID,
			LeaseEpoch:     epoch,
			ManifestDigest: "sha256:demo",
		}},
	}
}

func persistentEvents(messageID uint64, runID string, epoch uint64, sequences ...uint64) *runnerv1.RunnerMessage {
	events := make([]*runnerv1.RunEvent, 0, len(sequences))
	for _, sequence := range sequences {
		events = append(events, &runnerv1.RunEvent{ClientSequence: sequence, EventType: "model.delta"})
	}
	return &runnerv1.RunnerMessage{
		RunnerId:        "runner_1",
		SessionId:       "session_1",
		MessageId:       messageID,
		ProtocolVersion: protocolVersion,
		Payload: &runnerv1.RunnerMessage_EventBatch{EventBatch: &runnerv1.RunEventBatch{
			RunId:      runID,
			LeaseEpoch: epoch,
			Events:     events,
		}},
	}
}

func persistentFinished(messageID uint64, runID string, epoch uint64, status runnerv1.RunTerminalStatus) *runnerv1.RunnerMessage {
	return &runnerv1.RunnerMessage{
		RunnerId:        "runner_1",
		SessionId:       "session_1",
		MessageId:       messageID,
		ProtocolVersion: protocolVersion,
		Payload: &runnerv1.RunnerMessage_RunFinished{RunFinished: &runnerv1.RunFinished{
			RunId:      runID,
			LeaseEpoch: epoch,
			Status:     status,
		}},
	}
}
