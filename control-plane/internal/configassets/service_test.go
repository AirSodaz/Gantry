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
