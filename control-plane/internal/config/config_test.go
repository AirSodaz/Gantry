package config

import "testing"

func TestDevelopmentModeRequiresPhase0Token(t *testing.T) {
	t.Setenv("GANTRY_DEVELOPMENT_MODE", "true")
	t.Setenv("GANTRY_PHASE0_DEV_API_TOKEN", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded without a development API token")
	}
}

func TestDevelopmentModeEnablesPhase0API(t *testing.T) {
	t.Setenv("GANTRY_DEVELOPMENT_MODE", "true")
	t.Setenv("GANTRY_PHASE0_DEV_API_TOKEN", "test-token")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Phase0Dev.Enabled || config.Phase0Dev.Token != "test-token" {
		t.Fatalf("phase 0 config = %#v", config.Phase0Dev)
	}
}
