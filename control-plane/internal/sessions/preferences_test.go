package sessions

import (
	"strings"
	"testing"
)

func TestAgentPreferenceLockKeyIsPostgresTextSafeAndUnambiguous(t *testing.T) {
	first := agentPreferenceLockKey("principal:a", "workspace")
	second := agentPreferenceLockKey("principal", "a:workspace")
	if strings.ContainsRune(first, '\x00') {
		t.Fatal("lock key contains a PostgreSQL-invalid NUL byte")
	}
	if first == second {
		t.Fatal("length-prefixed lock keys collided")
	}
}
