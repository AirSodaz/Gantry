package agentlifecycle

import (
	"testing"
	"time"
)

func TestRevisionHashBindsCommitIdentity(t *testing.T) {
	createdAt := time.Date(2026, 8, 16, 10, 0, 0, 123, time.UTC)
	first, err := revisionHash("agt_1", "drf_main", "Enable search", "prn_1", createdAt, "sha256:spec")
	if err != nil {
		t.Fatal(err)
	}
	second, err := revisionHash("agt_1", "drf_main", "Disable search", "prn_1", createdAt, "sha256:spec")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first[:7] != "sha256:" || second[:7] != "sha256:" {
		t.Fatalf("commit message must contribute to hash: %q %q", first, second)
	}
}

func TestRevisionHashChangesWhenCommitTimeChanges(t *testing.T) {
	base := time.Date(2026, 8, 16, 10, 0, 0, 123, time.UTC)
	first, err := revisionHash("agt_1", "drf_main", "same", "prn_1", base, "sha256:spec")
	if err != nil {
		t.Fatal(err)
	}
	second, err := revisionHash("agt_1", "drf_main", "same", "prn_1", base.Add(time.Nanosecond), "sha256:spec")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("independent commits must not share a revision hash")
	}
}
