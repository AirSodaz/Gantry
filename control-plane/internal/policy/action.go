package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const DefaultPolicyVersion = "policy.dev.action/v1"

var ErrInvalidAction = errors.New("invalid action")

type Decision string

const (
	Allow           Decision = "allow"
	Deny            Decision = "deny"
	RequireApproval Decision = "require_approval"
)

// Action is the canonical, effect-bearing request sent to a tool gateway.
// Arguments are normalized before this value is persisted or approved.
type Action struct {
	RunID             string          `json:"run_id"`
	CallID            string          `json:"call_id,omitempty"`
	ToolName          string          `json:"tool_name"`
	Operation         string          `json:"operation"`
	Arguments         json.RawMessage `json:"arguments"`
	Target            string          `json:"target,omitempty"`
	Effect            string          `json:"effect"`
	CredentialRef     string          `json:"credential_ref,omitempty"`
	CredentialMode    string          `json:"credential_mode,omitempty"`
	PolicyVersion     string          `json:"policy_version"`
	RequestedBy       string          `json:"requested_by"`
	AllowSelfApproval bool            `json:"allow_self_approval"`
}

type Evaluation struct {
	Decision      Decision `json:"decision"`
	PolicyVersion string   `json:"policy_version"`
	Reason        string   `json:"reason"`
}

func Canonicalize(action Action) (Action, []byte, string, error) {
	action.RunID = strings.TrimSpace(action.RunID)
	action.CallID = strings.TrimSpace(action.CallID)
	action.ToolName = strings.TrimSpace(action.ToolName)
	action.Operation = strings.TrimSpace(action.Operation)
	action.Target = strings.TrimSpace(action.Target)
	action.Effect = strings.TrimSpace(action.Effect)
	action.CredentialRef = strings.TrimSpace(action.CredentialRef)
	action.CredentialMode = strings.TrimSpace(action.CredentialMode)
	action.PolicyVersion = strings.TrimSpace(action.PolicyVersion)
	action.RequestedBy = strings.TrimSpace(action.RequestedBy)
	if action.PolicyVersion == "" {
		action.PolicyVersion = DefaultPolicyVersion
	}
	if action.RunID == "" || action.ToolName == "" || action.Operation == "" || action.Effect == "" || action.RequestedBy == "" {
		return Action{}, nil, "", ErrInvalidAction
	}
	if len(action.Arguments) == 0 || string(action.Arguments) == "null" {
		action.Arguments = json.RawMessage(`{}`)
	}
	var arguments any
	if err := json.Unmarshal(action.Arguments, &arguments); err != nil {
		return Action{}, nil, "", ErrInvalidAction
	}
	if _, ok := arguments.(map[string]any); !ok {
		return Action{}, nil, "", ErrInvalidAction
	}
	canonicalArguments, err := json.Marshal(arguments)
	if err != nil {
		return Action{}, nil, "", ErrInvalidAction
	}
	action.Arguments = canonicalArguments
	canonical, err := json.Marshal(action)
	if err != nil {
		return Action{}, nil, "", err
	}
	digest := sha256.Sum256(canonical)
	return action, canonical, "sha256:" + hex.EncodeToString(digest[:]), nil
}

func Evaluate(action Action, autoApprove bool) (Evaluation, error) {
	canonical, _, _, err := Canonicalize(action)
	if err != nil {
		return Evaluation{}, err
	}
	if canonical.Effect == "read" {
		return Evaluation{Decision: Allow, PolicyVersion: canonical.PolicyVersion, Reason: "read-only action"}, nil
	}
	if canonical.Effect != "write" && canonical.Effect != "destructive" {
		return Evaluation{}, ErrInvalidAction
	}
	if autoApprove {
		return Evaluation{Decision: Allow, PolicyVersion: canonical.PolicyVersion, Reason: "approved by published policy"}, nil
	}
	return Evaluation{Decision: RequireApproval, PolicyVersion: canonical.PolicyVersion, Reason: "effect-bearing action requires current user approval"}, nil
}
