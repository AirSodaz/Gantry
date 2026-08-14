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
