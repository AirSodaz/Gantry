package config

import "testing"

func TestDevelopmentModeRequiresDevelopmentToken(t *testing.T) {
	t.Setenv("GANTRY_DEVELOPMENT_MODE", "true")
	t.Setenv("GANTRY_DEVELOPMENT_API_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded without a development API token")
	}
}

func TestDevelopmentModeEnablesDevelopmentAPI(t *testing.T) {
	t.Setenv("GANTRY_DEVELOPMENT_MODE", "true")
	t.Setenv("GANTRY_DEVELOPMENT_API_TOKEN", "test-token")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Development.Enabled || config.Development.Token != "test-token" {
		t.Fatalf("development config = %#v", config.Development)
	}
}

func TestAdminOIDCRequiresCompleteAudienceConfiguration(t *testing.T) {
	t.Setenv("GANTRY_ADMIN_OIDC_ISSUER", "https://issuer.example.test/realms/gantry")
	t.Setenv("GANTRY_ADMIN_OIDC_AUDIENCE", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded with an incomplete Admin OIDC configuration")
	}
}
