package config

import (
	"os"
	"strings"
	"testing"
)

// clearEnv makes the variables absent, not empty: an empty HASS_INSECURE fails
// to parse.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HASS_MODE", "HASS_URL", "HASS_TOKEN", "HASS_TIMEOUT", "HASS_INSECURE",
		"HASS_DELETE_AFTER_TRANSFER", "STORAGE_URL", "STORAGE_PREFIX", "LOG_LEVEL",
		"LOG_FORMAT", "RETENTION_KEEP_LAST", "SCHEDULE", "SUPERVISOR_TOKEN",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestLoadConfigCoreRequiresURLAndToken(t *testing.T) {
	clearEnv(t)
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "homeAssistant.url is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadConfigSupervisorMode(t *testing.T) {
	clearEnv(t)
	t.Setenv("HASS_MODE", "Supervisor")
	t.Setenv("SUPERVISOR_TOKEN", "sv-token")
	t.Setenv("SCHEDULE", " 0 3 * * * ")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HomeAssistant.Mode != ModeSupervisor || cfg.HomeAssistant.URL != SupervisorURL ||
		cfg.HomeAssistant.Token != "sv-token" || cfg.Schedule != "0 3 * * *" {
		t.Errorf("got %+v, schedule %q", cfg.HomeAssistant, cfg.Schedule)
	}
}

func TestLoadConfigSupervisorModeNeedsToken(t *testing.T) {
	clearEnv(t)
	t.Setenv("HASS_MODE", "supervisor")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "SUPERVISOR_TOKEN") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadConfigExplicitURLBeatsSupervisorDefault(t *testing.T) {
	clearEnv(t)
	t.Setenv("HASS_MODE", "supervisor")
	t.Setenv("SUPERVISOR_TOKEN", "sv-token")
	t.Setenv("HASS_URL", "http://elsewhere")
	cfg, err := LoadConfig()
	if err != nil || cfg.HomeAssistant.URL != "http://elsewhere" {
		t.Errorf("url, err = %q, %v", cfg.HomeAssistant.URL, err)
	}
}

func TestValidateModeAndSchedule(t *testing.T) {
	tests := []struct {
		name, schedule, mode, wantErr string
	}{
		{"defaults", "", "", ""},
		{"daily", "0 3 * * *", "", ""},
		{"descriptor", "@daily", "", ""},
		{"timezone", "CRON_TZ=America/New_York 0 3 * * *", "", ""},
		{"six fields", "0 0 3 * * *", "", "schedule invalid"},
		{"garbage", "nightly", "", "schedule invalid"},
		{"unknown mode", "", "cloud", "mode must be one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultConfig()
			c.HomeAssistant.URL, c.HomeAssistant.Token = "http://ha", "t"
			c.Schedule = tt.schedule
			if tt.mode != "" {
				c.HomeAssistant.Mode = Mode(tt.mode)
			}
			err := c.Validate()
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
