package policy

import (
	"encoding/json"
	"testing"
)

func TestCanonicalizeMakesEquivalentArgumentsShareDigest(t *testing.T) {
	base := Action{RunID: "run-1", ToolName: "crm", Operation: "update", Effect: "write", RequestedBy: "prn-1", Arguments: json.RawMessage(`{"b":2,"a":1}`)}
	other := base
	other.Arguments = json.RawMessage(`{"a":1,"b":2}`)
	_, _, first, err := Canonicalize(base)
	if err != nil {
		t.Fatal(err)
	}
	_, _, second, err := Canonicalize(other)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("digests differ: %s != %s", first, second)
	}
}

func TestEvaluateRequiresApprovalForWrite(t *testing.T) {
	evaluation, err := Evaluate(Action{RunID: "run-1", ToolName: "mail", Operation: "send", Effect: "write", RequestedBy: "prn-1"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.Decision != RequireApproval {
		t.Fatalf("decision = %s", evaluation.Decision)
	}
}
