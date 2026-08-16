package taskmessage

import "testing"

func TestContentForAcceptsAllPublicPartKinds(t *testing.T) {
	content, err := contentFor([]Part{
		Text("Agent result"),
		Artifact("art_1", "report.csv"),
		ActionSummary("act_1", "Update customer record", "succeeded"),
		Status("run.completed", "Run completed"),
	})
	if err != nil {
		t.Fatalf("contentFor() error = %v", err)
	}
	if content != "Agent result\nreport.csv\nUpdate customer record\nRun completed" {
		t.Fatalf("contentFor() = %q", content)
	}
}

func TestContentForRejectsIncompleteParts(t *testing.T) {
	if _, err := contentFor([]Part{{"type": "artifact", "artifact_id": "art_1"}}); err == nil {
		t.Fatal("contentFor() accepted incomplete artifact part")
	}
}
