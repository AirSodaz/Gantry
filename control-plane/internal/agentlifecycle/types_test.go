package agentlifecycle

import (
	"strings"
	"testing"
)

func TestValidateSpecCanonicalizesSupportedManifest(t *testing.T) {
	canonical, findings := ValidateSpec([]byte(`{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"workspace_root":".","limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":false}}`))
	if len(findings) != 0 || len(canonical) == 0 {
		t.Fatalf("canonical=%s findings=%#v", canonical, findings)
	}
}

func TestValidateSpecRejectsMissingModel(t *testing.T) {
	_, findings := ValidateSpec([]byte(`{"kind":"gantry.agent/v1","workspace_root":".","limits":{"max_turns":12}}`))
	if len(findings) < 1 || findings[0].Path != "/model/provider" {
		t.Fatalf("findings=%#v", findings)
	}
}

func TestValidateSpecPreservesImmutableAssetReferences(t *testing.T) {
	canonical, findings := ValidateSpec([]byte(`{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"workspace_root":".","limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":false},"skills":[{"artifact_id":"skill_1"}],"plugins":[{"plugin_version_id":"plugin_1"}],"tool_bindings":[{"descriptor_id":"tool_1","operations":["search"]}]}`))
	if len(findings) != 0 {
		t.Fatalf("findings=%#v", findings)
	}
	if string(canonical) == "" || !strings.Contains(string(canonical), `"artifact_id":"skill_1"`) || !strings.Contains(string(canonical), `"descriptor_id":"tool_1"`) {
		t.Fatalf("canonical=%s", canonical)
	}
}

func TestCompilePromptSnapshotIsDeterministicAndOrdered(t *testing.T) {
	spec := []byte(`{"kind":"gantry.agent/v1","system_prompt":"System","user_input":"Input","model":{"provider":"scripted","model":"deterministic"},"workspace_root":".","limits":{"max_turns":1,"max_output_bytes":10},"rules":[{"name":"first","content":"Rule one"},{"name":"second","content":"Rule two"}]}`)
	first, err := CompilePromptSnapshot(spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CompilePromptSnapshot(spec)
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentDigest == "" || first.ContentDigest != second.ContentDigest {
		t.Fatalf("digest is not deterministic: %#v %#v", first, second)
	}
	if first.CompilerVersion != PromptCompilerVersion || first.CompiledText != "System\n\nInput\n\nRule one\n\nRule two" {
		t.Fatalf("snapshot=%#v", first)
	}
}

func TestBuildDiffIsSortedAndClassifiesSecurityChanges(t *testing.T) {
	diff, summary, err := buildDiff([]byte(`{"tools":{"calendar":"read"},"mode":"complete"}`), []byte(`{"tools":{"calendar":"write"},"mode":"await_cancel","new":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(diff) != 3 || diff[0].Path != "/mode" || diff[1].Path != "/new" || diff[2].Path != "/tools/calendar" {
		t.Fatalf("diff=%#v", diff)
	}
	if diff[2].Risk != "high" || diff[2].Category != "security" || summary.Total != 3 || summary.High != 1 || summary.Medium != 1 || summary.Low != 1 {
		t.Fatalf("diff=%#v summary=%#v", diff, summary)
	}
}

func TestBuildDiffEscapesJSONPointerKeys(t *testing.T) {
	diff, _, err := buildDiff([]byte(`{"a/b":1}`), []byte(`{"a/b":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(diff) != 1 || diff[0].Path != "/a~1b" || diff[0].Change != "changed" {
		t.Fatalf("diff=%#v", diff)
	}
}
