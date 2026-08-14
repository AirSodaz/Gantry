package tasks

import "testing"

func TestNormalizeInputSelectsDeterministicMode(t *testing.T) {
	input, mode, err := normalizeInput(SubmitRequest{Message: "hello", StructuredInput: []byte(`{"mode":"await_cancel"}`)})
	if err != nil || mode != "await_cancel" || input == "" {
		t.Fatalf("input=%q mode=%q err=%v", input, mode, err)
	}
}
func TestNormalizeInputRejectsUnsupportedAttachmentsAndModes(t *testing.T) {
	if _, _, err := normalizeInput(SubmitRequest{Message: "hello", AttachmentIDs: []string{"artifact"}}); err != ErrInvalidInput {
		t.Fatalf("attachment err=%v", err)
	}
	if _, _, err := normalizeInput(SubmitRequest{StructuredInput: []byte(`{"mode":"other"}`)}); err != ErrInvalidInput {
		t.Fatalf("mode err=%v", err)
	}
}
