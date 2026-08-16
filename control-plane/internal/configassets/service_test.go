package configassets

import "testing"

func TestCatalogInputClassifications(t *testing.T) {
	for _, value := range []string{"marketplace", "locator", "upload", "local"} {
		if !validSkillSource(value) {
			t.Fatalf("skill source %q rejected", value)
		}
	}
	if validSkillSource("remote-shell") || validServerType("shell") || validEffect("delete") || validIdempotency("unknown") {
		t.Fatal("invalid catalog classifications were accepted")
	}
}

func TestCatalogSlugValidation(t *testing.T) {
	if !validSlug("document-search-v2") {
		t.Fatal("valid slug rejected")
	}
	for _, value := range []string{"A", "a_1", "x", ""} {
		if validSlug(value) {
			t.Fatalf("invalid slug %q accepted", value)
		}
	}
}

func TestCatalogStatusTransitions(t *testing.T) {
	if !validSkillTransition("available", "deprecated") || !validSkillTransition("deprecated", "available") || !validSkillTransition("deprecated", "retired") {
		t.Fatal("valid Skill transitions were rejected")
	}
	if validSkillTransition("retired", "available") || validSkillTransition("available", "available") {
		t.Fatal("invalid Skill transition was accepted")
	}
	if !validPluginTransition("active", "deprecated") || !validPluginTransition("deprecated", "active") || validPluginTransition("retired", "active") {
		t.Fatal("Plugin transition classification is incorrect")
	}
	if !validToolTransition("proposed", "active") || !validToolTransition("active", "retired") || validToolTransition("proposed", "retired") {
		t.Fatal("Tool transition classification is incorrect")
	}
}

func TestCatalogListFiltersNormalizeUnknownStatus(t *testing.T) {
	if normalizeSkillStatus("available") != "available" || normalizePluginStatus("active") != "active" || normalizeToolStatus("proposed") != "proposed" {
		t.Fatal("valid catalog status was discarded")
	}
	if normalizeSkillStatus("running") != "" || normalizePluginStatus("available") != "" || normalizeToolStatus("unknown") != "" {
		t.Fatal("unknown catalog status was accepted")
	}
}
