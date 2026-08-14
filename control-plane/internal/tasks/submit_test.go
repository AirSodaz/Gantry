package tasks

import "testing"

func TestNormalizeInputPreservesStructuredUserInput(t *testing.T) {
	input, err := normalizeInput(SubmitRequest{Message: "hello", StructuredInput: []byte(`{"mode":"await_cancel"}`)})
	if err != nil || input == "" {
		t.Fatalf("input=%q err=%v", input, err)
	}
}
func TestNormalizeInputRejectsUnsupportedAttachments(t *testing.T) {
	if _, err := normalizeInput(SubmitRequest{Message: "hello", AttachmentIDs: []string{"artifact"}}); err != ErrInvalidInput {
		t.Fatalf("attachment err=%v", err)
	}
}
