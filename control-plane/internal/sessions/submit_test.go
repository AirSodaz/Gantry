package sessions

import (
	"fmt"
	"testing"
)

func TestNormalizeInputPreservesStructuredUserInput(t *testing.T) {
	input, err := normalizeInput(SubmitRequest{StructuredInput: []byte(`{"mode":"await_cancel"}`)})
	if err != nil || input == "" {
		t.Fatalf("input=%q err=%v", input, err)
	}
}
func TestNormalizeInputIncludesCanonicalAttachmentReferences(t *testing.T) {
	input, err := normalizeInput(SubmitRequest{Message: "hello", AttachmentIDs: []string{"att_b", "att_a"}})
	if err != nil {
		t.Fatalf("normalize err=%v", err)
	}
	if input != `{"attachment_ids":["att_a","att_b"],"message":"hello","structured_input":null}` {
		t.Fatalf("input=%s", input)
	}
}

func TestNormalizeInputRejectsDuplicateOrTooManyAttachmentReferences(t *testing.T) {
	if _, err := normalizeInput(SubmitRequest{Message: "hello", AttachmentIDs: []string{"att_1", "att_1"}}); err != ErrInvalidInput {
		t.Fatalf("duplicate err=%v", err)
	}
	ids := make([]string, 11)
	for index := range ids {
		ids[index] = fmt.Sprintf("att_%d", index)
	}
	if _, err := normalizeInput(SubmitRequest{Message: "hello", AttachmentIDs: ids}); err != ErrInvalidInput {
		t.Fatalf("limit err=%v", err)
	}
}
