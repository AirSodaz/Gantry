package credentials

import (
	"os"
	"testing"
	"time"
)

func TestFileBrokerEncryptsAndVendsShortLivedLease(t *testing.T) {
	path := t.TempDir() + "/credentials.enc"
	broker, err := NewFileBroker(path, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	if err := broker.Put("crm-write", "secret-value"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" || string(raw) == `{"crm-write":"secret-value"}` {
		t.Fatal("credential file is not encrypted")
	}
	lease, err := broker.Resolve("crm-write", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if lease.Value != "secret-value" || !lease.ExpiresAt.After(time.Now()) {
		t.Fatalf("lease = %#v", lease)
	}
}
