package runs

import (
	"errors"
	"testing"
)

func TestDecodeObservedActionRequiresCallIDAndObjectArguments(t *testing.T) {
	base := `{"tool_name":"shell","operation":"execute","effect":"write","call_id":"call-1","arguments":{"command":"printf approval"}}`
	action, err := decodeObservedAction(base, "run-1", "prn-1")
	if err != nil {
		t.Fatal(err)
	}
	if action.CallID != "call-1" || string(action.Arguments) != `{"command":"printf approval"}` {
		t.Fatalf("decoded action = %#v", action)
	}

	for name, payload := range map[string]string{
		"missing call id":   `{"tool_name":"shell","operation":"execute","effect":"write","arguments":{"command":"printf approval"}}`,
		"scalar arguments":  `{"tool_name":"shell","operation":"execute","effect":"write","call_id":"call-1","arguments":"command"}`,
		"missing arguments": `{"tool_name":"shell","operation":"execute","effect":"write","call_id":"call-1"}`,
		"null arguments":    `{"tool_name":"shell","operation":"execute","effect":"write","call_id":"call-1","arguments":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeObservedAction(payload, "run-1", "prn-1"); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDecodeObservedCallIDRequiresNonEmptyCallID(t *testing.T) {
	if callID, err := decodeObservedCallID(`{"call_id":" call-1 "}`); err != nil || callID != "call-1" {
		t.Fatalf("call id=%q err=%v", callID, err)
	}
	for _, payload := range []string{`{}`, `{"call_id":" "}`, `{"call_id":3}`, `{bad`} {
		if _, err := decodeObservedCallID(payload); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("payload %s error = %v", payload, err)
		}
	}
}

func TestTransitionObservedActionValidatesTerminalState(t *testing.T) {
	for _, terminalState := range []string{"", "ready", "executing", "succeeded ", "failed ", "unknown_outcome"} {
		if err := transitionObservedAction(nil, nil, "run-1", "call-1", terminalState); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("terminal state %q error = %v", terminalState, err)
		}
	}
	if err := transitionObservedAction(nil, nil, "run-1", " ", "succeeded"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty call id error = %v", err)
	}
}

func TestCompletedOutcomeRequiresRequesterInputAfterApprovalDenial(t *testing.T) {
	if got := completedOutcome(false); got != "succeeded" {
		t.Fatalf("ordinary completed outcome=%q", got)
	}
	if got := completedOutcome(true); got != "requester_input_required" {
		t.Fatalf("denied action outcome=%q", got)
	}
}

func TestCanFinishAllowsRunnerPreparationFailureBeforeAcceptance(t *testing.T) {
	for _, test := range []struct {
		status, terminal string
		want             bool
	}{
		{"assigned", "failed", true},
		{"assigned", "completed", false},
		{"assigned", "canceled", false},
		{"accepted", "completed", true},
		{"accepted", "failed", true},
		{"canceling", "canceled", true},
		{"canceling", "failed", true},
	} {
		if got := canFinish(test.status, test.terminal); got != test.want {
			t.Fatalf("canFinish(%q, %q) = %v, want %v", test.status, test.terminal, got, test.want)
		}
	}
}
