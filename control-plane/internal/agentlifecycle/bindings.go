package agentlifecycle

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// validateAssetBindings rechecks catalog state at draft-write and publication
// boundaries. A browser cannot turn an old catalog response into authority.
func validateAssetBindings(ctx context.Context, pool *pgxpool.Pool, workspaceID string, spec json.RawMessage) ([]Finding, error) {
	var manifest Manifest
	if err := json.Unmarshal(spec, &manifest); err != nil {
		return nil, nil
	}
	findings := make([]Finding, 0)
	for index, binding := range manifest.Skills {
		if binding.ArtifactID == "" {
			findings = append(findings, Finding{Path: fmt.Sprintf("/skills/%d/artifact_id", index), Message: "Skill artifact is required."})
			continue
		}
		var status string
		err := pool.QueryRow(ctx, `SELECT status FROM gantry.skills WHERE id=$1 AND workspace_id=$2`, binding.ArtifactID, workspaceID).Scan(&status)
		if err != nil || status != "available" {
			findings = append(findings, Finding{Path: fmt.Sprintf("/skills/%d/artifact_id", index), Message: "Skill artifact is unavailable in this workspace."})
		}
	}
	for index, binding := range manifest.Plugins {
		if binding.VersionID == "" {
			findings = append(findings, Finding{Path: fmt.Sprintf("/plugins/%d/plugin_version_id", index), Message: "Plugin version is required."})
			continue
		}
		var status string
		err := pool.QueryRow(ctx, `SELECT p.status FROM gantry.plugins p JOIN gantry.workspace_plugin_enablements e ON e.plugin_id=p.id WHERE p.id=$1 AND e.workspace_id=$2`, binding.VersionID, workspaceID).Scan(&status)
		if err != nil || status != "active" {
			findings = append(findings, Finding{Path: fmt.Sprintf("/plugins/%d/plugin_version_id", index), Message: "Plugin version is unavailable to this organization."})
		}
	}
	for index, binding := range manifest.ToolBindings {
		if binding.DescriptorID == "" {
			findings = append(findings, Finding{Path: fmt.Sprintf("/tool_bindings/%d/descriptor_id", index), Message: "Tool descriptor is required."})
			continue
		}
		var status string
		var schemaJSON []byte
		err := pool.QueryRow(ctx, `SELECT d.status, d.schema_json FROM gantry.tool_descriptors d JOIN gantry.tool_servers s ON s.id=d.server_id JOIN gantry.agents a ON a.organization_id=s.organization_id WHERE d.id=$1 AND a.workspace_id=$2`, binding.DescriptorID, workspaceID).Scan(&status, &schemaJSON)
		if err != nil || status != "active" {
			findings = append(findings, Finding{Path: fmt.Sprintf("/tool_bindings/%d/descriptor_id", index), Message: "Tool descriptor is unavailable to this organization."})
			continue
		}
		var descriptor struct {
			Operations []string `json:"operations"`
		}
		if len(schemaJSON) > 0 && json.Unmarshal(schemaJSON, &descriptor) == nil && len(descriptor.Operations) > 0 {
			findings = append(findings, validateToolOperations(fmt.Sprintf("/tool_bindings/%d/operations", index), descriptor.Operations, binding.Operations)...)
		}
	}
	return findings, nil
}

func validateToolOperations(path string, available, selected []string) []Finding {
	allowed := make(map[string]struct{}, len(available))
	for _, operation := range available {
		allowed[operation] = struct{}{}
	}
	seen := make(map[string]struct{}, len(selected))
	findings := make([]Finding, 0)
	for index, operation := range selected {
		if _, duplicate := seen[operation]; duplicate {
			findings = append(findings, Finding{Path: fmt.Sprintf("%s/%d", path, index), Message: "Tool operation must not be repeated."})
			continue
		}
		seen[operation] = struct{}{}
		if _, ok := allowed[operation]; !ok {
			findings = append(findings, Finding{Path: fmt.Sprintf("%s/%d", path, index), Message: "Tool binding operation is broader than the descriptor."})
		}
	}
	return findings
}
