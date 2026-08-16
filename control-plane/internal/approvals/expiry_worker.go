package approvals

import (
	"context"
	"log/slog"
	"time"
)

// resolutionDispatcher is implemented by the runner session coordinator. The
// approval package owns expiry state transitions and only depends on this
// narrow delivery port for notifying a currently connected runner.
type resolutionDispatcher interface {
	ResolveApproval(string, string, string, string, string, string, string, uint64, time.Time) bool
}

type expiryService interface {
	ExpireAll(context.Context) ([]Resolution, error)
}

type ExpiryWorker struct {
	service    expiryService
	dispatcher resolutionDispatcher
	logger     *slog.Logger
	interval   time.Duration
}

func NewExpiryWorker(service expiryService, dispatcher resolutionDispatcher, logger *slog.Logger, interval time.Duration) *ExpiryWorker {
	if logger == nil {
		logger = slog.Default()
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &ExpiryWorker{service: service, dispatcher: dispatcher, logger: logger, interval: interval}
}

func (w *ExpiryWorker) Run(ctx context.Context) {
	w.reconcile(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.reconcile(ctx)
		}
	}
}

func (w *ExpiryWorker) reconcile(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}
	resolutions, err := w.service.ExpireAll(ctx)
	if err != nil {
		w.logger.Error("approval expiry reconciliation failed", "error", err)
		return
	}
	for _, resolution := range resolutions {
		if w.dispatcher == nil || !w.dispatcher.ResolveApproval(resolution.RunID, resolution.ApprovalID, resolution.Decision, resolution.Reason, resolution.ActionID, resolution.CallID, resolution.PermitID, resolution.PermitLeaseEpoch, resolution.PermitExpiresAt) {
			w.logger.Warn("approval expiry persisted without an active runner session", "approval_id", resolution.ApprovalID, "run_id", resolution.RunID)
		}
	}
}
