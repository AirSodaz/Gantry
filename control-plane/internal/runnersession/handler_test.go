package runnersession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	runnerv1 "github.com/AirSodaz/gantry/gen/gantry/runner/v1"
	"github.com/AirSodaz/gantry/gen/gantry/runner/v1/runnerv1connect"
	"log/slog"
)

func TestRunnerSessionHandlerPath(t *testing.T) {
	path, handler := NewHandler(slog.Default(), NewScheduler())
	if path != "/gantry.runner.v1.RunnerSession/" {
		t.Fatalf("path = %q", path)
	}
	if handler == nil {
		t.Fatal("handler is nil")
	}
	var _ runnerv1connect.RunnerSessionHandler = service{logger: slog.Default(), scheduler: NewScheduler()}
}

func TestRunnerSessionStreamsLifecycleMessages(t *testing.T) {
	scheduler := NewScheduler()
	path, handler := NewHandler(slog.Default(), scheduler)
	server := httptest.NewUnstartedServer(httpHandler(path, handler))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := runnerv1connect.NewRunnerSessionClient(server.Client(), server.URL)
	stream := client.Session(ctx)
	if err := stream.Send(registerMessage()); err != nil {
		t.Fatal(err)
	}
	waitForRunner(t, scheduler)
	run, err := scheduler.SubmitDemoRun("run-1", []byte("demo"), "sha256:demo")
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := stream.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if assignment.GetAssignRun().GetRunId() != run.ID {
		t.Fatalf("assignment = %#v", assignment)
	}
	if err := stream.Send(accepted("runner-1", "session-1", 2, run)); err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(events("runner-1", "session-1", 3, run, 1)); err != nil {
		t.Fatal(err)
	}
	ack, err := stream.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if ack.GetAcknowledgeEvents().GetLastAcknowledgedSequence() != 1 {
		t.Fatalf("ack = %#v", ack)
	}
	if err := stream.Send(finished("runner-1", "session-1", 4, run, runnerv1.RunTerminalStatus_RUN_TERMINAL_STATUS_COMPLETED)); err != nil {
		t.Fatal(err)
	}
	if err := stream.CloseRequest(); err != nil {
		t.Fatal(err)
	}
	waitForRunStatus(t, scheduler, run.ID, RunStatusCompleted)
}

func httpHandler(path string, handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	return mux
}

func waitForRunner(t *testing.T, scheduler *Scheduler) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		scheduler.mu.Lock()
		registered := len(scheduler.runners) == 1
		scheduler.mu.Unlock()
		if registered {
			return
		}
		select {
		case <-deadline:
			t.Fatal("runner did not register")
		case <-time.After(time.Millisecond):
		}
	}
}

func waitForRunStatus(t *testing.T, scheduler *Scheduler, runID string, status RunStatus) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		if run, ok := scheduler.Run(runID); ok && run.Status == status {
			return
		}
		select {
		case <-deadline:
			run, _ := scheduler.Run(runID)
			t.Fatalf("run status = %#v, want %s", run, status)
		case <-time.After(time.Millisecond):
		}
	}
}

func registerMessage() *runnerv1.RunnerMessage {
	return &runnerv1.RunnerMessage{RunnerId: "runner-1", SessionId: "session-1", MessageId: 1, ProtocolVersion: protocolVersion, Payload: &runnerv1.RunnerMessage_Register{Register: &runnerv1.RegisterRunner{RunnerVersion: "test"}}}
}
