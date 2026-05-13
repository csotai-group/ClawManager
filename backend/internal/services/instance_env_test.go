package services

import (
	"testing"

	"clawreef/internal/models"
)

func TestNormalizeEnvironmentOverrides(t *testing.T) {
	overrides, err := normalizeEnvironmentOverrides(map[string]string{
		" FOO ": "bar",
		"BAR_2": "",
	})
	if err != nil {
		t.Fatalf("normalizeEnvironmentOverrides returned error: %v", err)
	}

	if overrides["FOO"] != "bar" {
		t.Fatalf("expected trimmed key FOO to be preserved")
	}
	if value, ok := overrides["BAR_2"]; !ok || value != "" {
		t.Fatalf("expected empty override value to be preserved")
	}
}

func TestNormalizeEnvironmentOverridesRejectsInvalidNames(t *testing.T) {
	if _, err := normalizeEnvironmentOverrides(map[string]string{
		"1INVALID": "value",
	}); err == nil {
		t.Fatalf("expected invalid environment variable name to fail validation")
	}
}

func TestBuildInstancePodEnvAppliesOverridesAfterDefaults(t *testing.T) {
	t.Setenv("CLAWMANAGER_EGRESS_PROXY_URL", "")
	t.Setenv("CLAWMANAGER_SYSTEM_NAMESPACE", "")
	t.Setenv("K8S_NAMESPACE", "")

	raw, err := marshalEnvironmentOverrides(map[string]string{
		"SUBFOLDER": "/custom-proxy",
		"CUSTOM":    "enabled",
	})
	if err != nil {
		t.Fatalf("marshalEnvironmentOverrides returned error: %v", err)
	}

	instance := &models.Instance{
		ID:                       42,
		Type:                     "webtop",
		EnvironmentOverridesJSON: raw,
	}

	env, err := buildInstancePodEnv(instance, map[string]string{
		"TITLE":     "ClawManager Webtop",
		"SUBFOLDER": "/",
	}, nil, nil)
	if err != nil {
		t.Fatalf("buildInstancePodEnv returned error: %v", err)
	}

	if env["SUBFOLDER"] != "/custom-proxy" {
		t.Fatalf("expected SUBFOLDER override to win, got %q", env["SUBFOLDER"])
	}
	if env["CUSTOM"] != "enabled" {
		t.Fatalf("expected custom environment variable to be merged")
	}
	if env["TITLE"] != "ClawManager Webtop" {
		t.Fatalf("expected default environment variable to remain available")
	}
}

func TestBuildInstancePodEnvPinsOpenClawGatewayTokenToAccessToken(t *testing.T) {
	t.Setenv("CLAWMANAGER_EGRESS_PROXY_URL", "")
	t.Setenv("CLAWMANAGER_SYSTEM_NAMESPACE", "")
	t.Setenv("K8S_NAMESPACE", "")

	raw, err := marshalEnvironmentOverrides(map[string]string{
		"OPENCLAW_GATEWAY_TOKEN": "user-supplied-token",
	})
	if err != nil {
		t.Fatalf("marshalEnvironmentOverrides returned error: %v", err)
	}

	accessToken := "igt_database_token"
	instance := &models.Instance{
		ID:                       77,
		Type:                     "openclaw",
		AccessToken:              &accessToken,
		EnvironmentOverridesJSON: raw,
	}

	env, err := buildInstancePodEnv(instance, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildInstancePodEnv returned error: %v", err)
	}

	if env["OPENCLAW_GATEWAY_TOKEN"] != accessToken {
		t.Fatalf("expected OPENCLAW_GATEWAY_TOKEN to use database access token, got %q", env["OPENCLAW_GATEWAY_TOKEN"])
	}
	if env["OPENCLAW_CONFIG_PATH"] != "/config/.openclaw/openclaw.json" {
		t.Fatalf("expected OPENCLAW_CONFIG_PATH to point at managed config, got %q", env["OPENCLAW_CONFIG_PATH"])
	}
	wantCommand := "openclaw gateway run --auth token --token " + accessToken
	if env["OPENCLAW_AGENT_OPENCLAW_COMMAND"] != wantCommand {
		t.Fatalf("expected OPENCLAW_AGENT_OPENCLAW_COMMAND to force gateway token, got %q", env["OPENCLAW_AGENT_OPENCLAW_COMMAND"])
	}
	if env["OPENCLAW_AGENT_OPENCLAW_CONFIG_PATH"] != "/config/.openclaw/openclaw.json" {
		t.Fatalf("expected OPENCLAW_AGENT_OPENCLAW_CONFIG_PATH to point at managed config, got %q", env["OPENCLAW_AGENT_OPENCLAW_CONFIG_PATH"])
	}
}

func TestPopSHMSizeGB(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		hasValue bool
		want     int
	}{
		{name: "default", want: defaultInstanceSHMSizeGB},
		{name: "disable", value: "0", hasValue: true, want: 0},
		{name: "custom", value: "4", hasValue: true, want: 4},
		{name: "clamp", value: "128", hasValue: true, want: maxInstanceSHMSizeGB},
		{name: "invalid", value: "nope", hasValue: true, want: defaultInstanceSHMSizeGB},
		{name: "negative", value: "-1", hasValue: true, want: defaultInstanceSHMSizeGB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extraEnv := map[string]string{"KEEP": "value"}
			if tt.hasValue {
				extraEnv["SHM_SIZE_GB"] = tt.value
			}

			got := popSHMSizeGB(extraEnv)
			if got != tt.want {
				t.Fatalf("expected shm size %d, got %d", tt.want, got)
			}
			if _, ok := extraEnv["SHM_SIZE_GB"]; ok {
				t.Fatalf("expected SHM_SIZE_GB to be removed from extra env")
			}
			if extraEnv["KEEP"] != "value" {
				t.Fatalf("expected unrelated env to be preserved")
			}
		})
	}
}
