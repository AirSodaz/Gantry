package approvals

import (
	"context"
	"testing"

	"github.com/AirSodaz/gantry/internal/identity"
)

func TestDecideValidatesBeforeOpeningDatabase(t *testing.T) {
	service := NewService(nil)
	_, err := service.Decide(context.Background(), identity.Principal{ID: "prn-1"}, DecisionInput{ID: "", Decision: "approve"})
	if err != ErrInvalidInput {
		t.Fatalf("error = %v", err)
	}
}
