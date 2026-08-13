package runnersession

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	runnerv1 "github.com/AirSodaz/gantry/gen/gantry/runner/v1"
	"github.com/AirSodaz/gantry/gen/gantry/runner/v1/runnerv1connect"
)

type service struct { logger *slog.Logger }

func NewHandler(logger *slog.Logger) (string, http.Handler) {
	return runnerv1connect.NewRunnerSessionHandler(service{logger: logger})
}

func (service service) Session(_ context.Context, stream *connect.BidiStream[runnerv1.RunnerMessage, runnerv1.ControlPlaneMessage]) error {
	for {
		message, err := stream.Receive()
		if errors.Is(err, io.EOF) { return nil }
		if err != nil { return err }
		service.logger.Info("runner session message", "runner_id", message.RunnerId, "message_id", message.MessageId)
		if heartbeat := message.GetHeartbeat(); heartbeat != nil {
			if err := stream.Send(&runnerv1.ControlPlaneMessage{Payload: &runnerv1.ControlPlaneMessage_AcknowledgeEvents{AcknowledgeEvents: &runnerv1.AcknowledgeEvents{RunId: heartbeat.RunId}}}); err != nil { return err }
		}
	}
}
