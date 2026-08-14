package agentlifecycle

import "testing"

func TestValidateSpecCanonicalizesSupportedManifest(t *testing.T) {
	canonical, findings := ValidateSpec([]byte(`{"mode":"await_cancel","kind":"gantry.phase0.demo/v1"}`))
	if len(findings) != 0 || string(canonical) != `{"kind":"gantry.phase0.demo/v1","mode":"await_cancel"}` {
		t.Fatalf("canonical=%s findings=%#v", canonical, findings)
	}
}

func TestValidateSpecRejectsUnsupportedMode(t *testing.T) {
	_, findings := ValidateSpec([]byte(`{"kind":"gantry.phase0.demo/v1","mode":"anything"}`))
	if len(findings) != 1 || findings[0].Path != "/mode" {
		t.Fatalf("findings=%#v", findings)
	}
}
