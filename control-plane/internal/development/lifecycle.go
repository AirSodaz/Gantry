package development

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
)

// Lifecycle provides development-only access to the fixture agent without
// placing fixture policy in the product task service.
type Lifecycle struct{ tasks *tasks.Service }

func NewLifecycle(taskService *tasks.Service) *Lifecycle { return &Lifecycle{tasks: taskService} }

func (l *Lifecycle) Start(ctx context.Context, mode string) (tasks.TaskRun, error) {
	agentID := CompleteAgentID
	if mode == "await_cancel" {
		agentID = AwaitCancelAgentID
	} else if mode != "complete" {
		return tasks.TaskRun{}, tasks.ErrInvalidInput
	}
	task, _, err := l.tasks.Submit(ctx, demoActor(), newID(), tasks.SubmitRequest{AgentID: agentID, Message: "development lifecycle probe"})
	if err != nil {
		return tasks.TaskRun{}, err
	}
	return tasks.TaskRun{TaskID: task.ID, Run: task.CurrentRun}, nil
}
func (l *Lifecycle) Get(ctx context.Context, runID string) (tasks.TaskRun, error) {
	return l.tasks.GetRun(ctx, demoActor(), runID)
}
func (l *Lifecycle) Cancel(ctx context.Context, runID string) (tasks.CancelResult, error) {
	run, err := l.Get(ctx, runID)
	if err != nil {
		return tasks.CancelResult{}, err
	}
	return l.tasks.Cancel(ctx, demoActor(), run.TaskID, runID, newID())
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
