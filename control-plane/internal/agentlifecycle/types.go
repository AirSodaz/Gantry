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

const ManifestKind = "gantry.phase0.demo/v1"

var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidInput     = errors.New("invalid input")
	ErrInvalidState     = errors.New("invalid state")
	ErrRevisionConflict = errors.New("draft revision conflict")
)

type Manifest struct {
	Kind string `json:"kind"`
	Mode string `json:"mode"`
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
	if manifest.Mode != "complete" && manifest.Mode != "await_cancel" {
		findings = append(findings, Finding{Path: "/mode", Message: "Mode must be complete or await_cancel."})
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
	return json.RawMessage(`{"kind":"gantry.phase0.demo/v1","mode":"complete"}`)
}

func newID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}
