package runnersession

import (
	"testing"

	"github.com/AirSodaz/gantry/gen/gantry/runner/v1/runnerv1connect"
	"log/slog"
)

func TestRunnerSessionHandlerPath(t *testing.T) {
	path, handler := NewHandler(slog.Default())
	if path != "/gantry.runner.v1.RunnerSession/" { t.Fatalf("path = %q", path) }
	if handler == nil { t.Fatal("handler is nil") }
	var _ runnerv1connect.RunnerSessionHandler = service{logger: slog.Default()}
}
