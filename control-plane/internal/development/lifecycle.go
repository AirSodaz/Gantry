package development

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/sessions"
)

// Lifecycle provides development-only access to the fixture agent without
// placing fixture policy in the product task service.
type Lifecycle struct{ tasks *sessions.Service }

func NewLifecycle(taskService *sessions.Service) *Lifecycle { return &Lifecycle{tasks: taskService} }

func (l *Lifecycle) Start(ctx context.Context, mode string) (sessions.SessionRun, error) {
	agentID := CompleteAgentID
	if mode == "await_cancel" {
		agentID = AwaitCancelAgentID
	} else if mode != "complete" {
		return sessions.SessionRun{}, sessions.ErrInvalidInput
	}
	task, _, err := l.tasks.Submit(ctx, demoActor(), newID(), sessions.SubmitRequest{AgentID: agentID, Message: "development lifecycle probe"})
	if err != nil {
		return sessions.SessionRun{}, err
	}
	if task.ExecutingRun == nil {
		return sessions.SessionRun{}, sessions.ErrNotFound
	}
	return sessions.SessionRun{SessionID: task.ID, Run: *task.ExecutingRun}, nil
}
func (l *Lifecycle) Get(ctx context.Context, runID string) (sessions.SessionRun, error) {
	return l.tasks.GetRun(ctx, demoActor(), runID)
}
func (l *Lifecycle) Cancel(ctx context.Context, runID string) (sessions.CancelResult, error) {
	run, err := l.Get(ctx, runID)
	if err != nil {
		return sessions.CancelResult{}, err
	}
	return l.tasks.Cancel(ctx, demoActor(), run.SessionID, runID, newID())
}
func demoActor() identity.Principal {
	return identity.Principal{ID: DevelopmentPrincipalID, OrganizationID: OrganizationID}
}
func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return "development-" + hex.EncodeToString(bytes)
}
