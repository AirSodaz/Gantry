package sessions

import "testing"

func TestReasonProjectionRequestsNewInputForRejectedActionRun(t *testing.T) {
	outcome := "requester_input_required"
	reason := reasonProjection("approval expired", "completed", &outcome)
	if reason == nil || reason.Code != outcome || reason.NextAction != "provide_input" || reason.Message != "approval expired" {
		t.Fatalf("reason=%#v", reason)
	}
}

func TestReasonProjectionKeepsOrdinaryFailureSemantics(t *testing.T) {
	outcome := "failed"
	reason := reasonProjection("model failed", "failed", &outcome)
	if reason == nil || reason.Code != "failed" || reason.NextAction != "none" {
		t.Fatalf("reason=%#v", reason)
	}
}
