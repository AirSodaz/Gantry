package agentlifecycle

import "testing"

func TestValidateToolOperationsOnlyAllowsDescriptorSubset(t *testing.T) {
	findings := validateToolOperations("/tool_bindings/0/operations", []string{"read", "search"}, []string{"search", "search", "write"})
	if len(findings) != 2 || findings[0].Message != "Tool operation must not be repeated." || findings[1].Message != "Tool binding operation is broader than the descriptor." {
		t.Fatalf("findings=%#v", findings)
	}
}
