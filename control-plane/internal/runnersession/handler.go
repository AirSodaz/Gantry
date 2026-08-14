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

type service struct {
	logger    *slog.Logger
	scheduler Coordinator
}

type Coordinator interface {
	Register(string, string, uint64) (<-chan *runnerv1.ControlPlaneMessage, error)
	Handle(string, string, *runnerv1.RunnerMessage) error
	Disconnect(string, string)
}

func NewHandler(logger *slog.Logger, scheduler Coordinator) (string, http.Handler) {
	return runnerv1connect.NewRunnerSessionHandler(service{logger: logger, scheduler: scheduler})
}

func (service service) Session(ctx context.Context, stream *connect.BidiStream[runnerv1.RunnerMessage, runnerv1.ControlPlaneMessage]) error {
	first, err := stream.Receive()
	if err != nil {
		return err
	}
	if first.GetRegister() == nil || first.GetProtocolVersion() != protocolVersion {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("first runner message must be a supported registration"))
	}
	outbound, err := service.scheduler.Register(first.GetRunnerId(), first.GetSessionId(), first.GetMessageId())
	if err != nil {
		return connect.NewError(connect.CodeAlreadyExists, err)
	}
	defer service.scheduler.Disconnect(first.GetRunnerId(), first.GetSessionId())
	service.logger.Info("runner registered", "runner_id", first.GetRunnerId(), "session_id", first.GetSessionId())
	received := make(chan *runnerv1.RunnerMessage, 1)
	errs := make(chan error, 1)
	go func() {
		for {
			message, err := stream.Receive()
			if err != nil {
				select {
				case errs <- err:
				case <-ctx.Done():
				}
				return
			}
			select {
			case received <- message:
			case <-ctx.Done():
				return
			}
		}
	}()
	for {
		select {
		case message := <-received:
			if err := service.scheduler.Handle(first.GetRunnerId(), first.GetSessionId(), message); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
		case message := <-outbound:
			if err := stream.Send(message); err != nil {
				return err
			}
		case err := <-errs:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
