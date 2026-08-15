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

func TestCanonicalizeChangesDigestWhenCallIDChanges(t *testing.T) {
	base := Action{RunID: "run-1", CallID: "call-1", ToolName: "shell", Operation: "execute", Effect: "write", RequestedBy: "prn-1", Arguments: json.RawMessage(`{"timeout":3,"command":"printf approval"}`)}
	equivalent := base
	equivalent.Arguments = json.RawMessage(`{"command":"printf approval","timeout":3}`)
	changed := base
	changed.CallID = "call-2"

	_, _, first, err := Canonicalize(base)
	if err != nil {
		t.Fatal(err)
	}
	_, _, second, err := Canonicalize(equivalent)
	if err != nil {
		t.Fatal(err)
	}
	_, _, third, err := Canonicalize(changed)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("equivalent digests differ: %s != %s", first, second)
	}
	if first == third {
		t.Fatalf("call-id change did not change digest: %s", first)
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
