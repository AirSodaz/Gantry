package approvals

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type fakeExpiryService struct {
	resolutions []Resolution
	err         error
	calls       int
}

func (s *fakeExpiryService) ExpireAll(context.Context) ([]Resolution, error) {
	s.calls++
	return s.resolutions, s.err
}

type fakeResolutionDispatcher struct {
	resolutions []Resolution
	available   bool
}

func (d *fakeResolutionDispatcher) ResolveApproval(runID, approvalID, decision, reason, actionID, callID, permitID string, leaseEpoch uint64, permitExpiresAt time.Time) bool {
	d.resolutions = append(d.resolutions, Resolution{RunID: runID, ApprovalID: approvalID, Decision: decision, Reason: reason, ActionID: actionID, CallID: callID, PermitID: permitID, PermitLeaseEpoch: leaseEpoch, PermitExpiresAt: permitExpiresAt})
	return d.available
}

func TestExpiryWorkerReconcilesAndDeliversPersistedResolutions(t *testing.T) {
	service := &fakeExpiryService{resolutions: []Resolution{{ApprovalID: "apr_1", RunID: "run_1", ActionID: "act_1", CallID: "call_1", Decision: "reject", Reason: "approval expired"}}}
	dispatcher := &fakeResolutionDispatcher{available: true}
	worker := NewExpiryWorker(service, dispatcher, slog.Default(), time.Second)

	worker.reconcile(context.Background())

	if service.calls != 1 {
		t.Fatalf("expiry calls=%d", service.calls)
	}
	if len(dispatcher.resolutions) != 1 {
		t.Fatalf("delivered=%#v", dispatcher.resolutions)
	}
	if got := dispatcher.resolutions[0]; got.ApprovalID != "apr_1" || got.RunID != "run_1" || got.Decision != "reject" {
		t.Fatalf("resolution=%#v", got)
	}
}

func TestExpiryWorkerDoesNotDispatchWhenReconciliationFails(t *testing.T) {
	service := &fakeExpiryService{err: errors.New("database unavailable")}
	dispatcher := &fakeResolutionDispatcher{available: true}
	worker := NewExpiryWorker(service, dispatcher, slog.Default(), time.Second)

	worker.reconcile(context.Background())

	if len(dispatcher.resolutions) != 0 {
		t.Fatalf("delivered=%#v", dispatcher.resolutions)
	}
}
