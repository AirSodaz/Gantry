// Package agentlifecycle owns administrative agent drafts, immutable versions,
// and publication state.
package agentlifecycle

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const ManifestKind = "gantry.agent/v1"

var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidInput     = errors.New("invalid input")
	ErrInvalidState     = errors.New("invalid state")
	ErrRevisionConflict = errors.New("draft revision conflict")
	ErrReviewRequired   = errors.New("an approved review is required")
)

type Manifest struct {
	Kind          string         `json:"kind"`
	Model         ModelConfig    `json:"model"`
	SystemPrompt  string         `json:"system_prompt,omitempty"`
	UserInput     string         `json:"user_input,omitempty"`
	Rules         []RuleSnapshot `json:"rules,omitempty"`
	Tools         []string       `json:"tools,omitempty"`
	WorkspaceRoot string         `json:"workspace_root"`
	Limits        ResourceLimits `json:"limits"`
	Checkpoint    Checkpoint     `json:"checkpoint"`
	CommandPolicy CommandPolicy  `json:"command_policy"`
}

type ModelConfig struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	BaseURL          string `json:"base_url,omitempty"`
	MaxContextTokens int    `json:"max_context_tokens,omitempty"`
}
type RuleSnapshot struct {
	Name          string   `json:"name"`
	Content       string   `json:"content"`
	Globs         []string `json:"globs,omitempty"`
	AlwaysApply   bool     `json:"always_apply,omitempty"`
	Condition     string   `json:"condition,omitempty"`
	Scope         []string `json:"scope,omitempty"`
	InterruptMode string   `json:"interrupt_mode,omitempty"`
}
type ResourceLimits struct {
	MaxTurns         int `json:"max_turns"`
	MaxOutputBytes   int `json:"max_output_bytes"`
	ContextSoftLimit int `json:"context_soft_limit,omitempty"`
	TimeoutSeconds   int `json:"timeout_seconds,omitempty"`
}
type Checkpoint struct {
	Enabled bool   `json:"enabled,omitempty"`
	Path    string `json:"path,omitempty"`
}
type CommandPolicy struct {
	AllowShell          bool     `json:"allow_shell,omitempty"`
	InterceptorPatterns []string `json:"interceptor_patterns,omitempty"`
	DeniedPatterns      []string `json:"denied_patterns,omitempty"`
}

type Finding struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type Agent struct {
	ID                        string `json:"id"`
	OrganizationID            string `json:"organization_id"`
	WorkspaceID               string `json:"workspace_id"`
	Slug                      string `json:"slug"`
	DisplayName               string `json:"display_name"`
	Description               string `json:"description"`
	Category                  string `json:"category"`
	LifecycleStatus           string `json:"lifecycle_status"`
	CurrentPublishedVersionID string `json:"current_published_version_id,omitempty"`
}

type Draft struct {
	AgentID            string          `json:"agent_id"`
	Revision           int             `json:"revision"`
	Spec               json.RawMessage `json:"spec"`
	ValidationStatus   string          `json:"validation_status"`
	ValidationFindings []Finding       `json:"validation_findings"`
	UpdatedBy          string          `json:"updated_by"`
}

type Version struct {
	ID                  string          `json:"id"`
	AgentID             string          `json:"agent_id"`
	Version             int             `json:"version"`
	SourceDraftRevision int             `json:"source_draft_revision"`
	Spec                json.RawMessage `json:"spec"`
	SpecDigest          string          `json:"spec_digest"`
}

type CreateRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

func ValidateSpec(spec json.RawMessage) (json.RawMessage, []Finding) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(spec))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, []Finding{{Path: "", Message: "Specification must be a JSON object."}}
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, []Finding{{Path: "", Message: "Specification must contain one JSON object."}}
	}
	findings := make([]Finding, 0, 2)
	if manifest.Kind != ManifestKind {
		findings = append(findings, Finding{Path: "/kind", Message: "Unsupported manifest kind."})
	}
	if manifest.Model.Provider != "scripted" && manifest.Model.Provider != "openai" && manifest.Model.Provider != "openai-compatible" && manifest.Model.Provider != "anthropic" {
		findings = append(findings, Finding{Path: "/model/provider", Message: "Model provider must be scripted, openai-compatible, or anthropic."})
	}
	if manifest.Model.Model == "" {
		findings = append(findings, Finding{Path: "/model/model", Message: "Model name is required."})
	}
	if manifest.WorkspaceRoot == "" {
		findings = append(findings, Finding{Path: "/workspace_root", Message: "Workspace root is required."})
	}
	if manifest.Limits.MaxTurns <= 0 {
		findings = append(findings, Finding{Path: "/limits/max_turns", Message: "Max turns must be greater than zero."})
	}
	if manifest.Limits.MaxOutputBytes <= 0 {
		findings = append(findings, Finding{Path: "/limits/max_output_bytes", Message: "Max output bytes must be greater than zero."})
	}
	if len(findings) != 0 {
		return nil, findings
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		panic(fmt.Sprintf("canonical manifest encoding: %v", err))
	}
	return canonical, nil
}

func defaultSpec() json.RawMessage {
	return json.RawMessage(`{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"workspace_root":".","limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":false}}`)
}

func newID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}
