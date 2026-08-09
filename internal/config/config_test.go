package config

import (
	"os"
	"testing"
)

func TestLoadWithAPIKey(t *testing.T) {
	_ = os.Setenv("GEXBOT_API_KEY", "test-key-123")
	defer func() { _ = os.Unsetenv("GEXBOT_API_KEY") }()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("expected config to load with API key, got error: %v", err)
	}

	if cfg.API.APIKey != "test-key-123" {
		t.Errorf("expected API key 'test-key-123', got '%s'", cfg.API.APIKey)
	}

	if cfg.API.BaseURL != "https://api.gex.bot" {
		t.Errorf("expected default base URL, got '%s'", cfg.API.BaseURL)
	}

	if cfg.Download.Workers != 3 {
		t.Errorf("expected 3 workers by default, got %d", cfg.Download.Workers)
	}
}

func TestLoadWithoutAPIKey(t *testing.T) {
	_ = os.Unsetenv("GEXBOT_API_KEY")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

func TestValidateAutoCleanup(t *testing.T) {
	valid := func() *Config {
		c := &Config{}
		c.API.APIKey = "k"
		c.Download.Workers = 1
		return c
	}
	// auto_cleanup enabled with cleanup_after_days < 1 must error.
	c := valid()
	c.Output.AutoCleanup = true
	c.Output.CleanupAfterDays = 0
	if err := c.Validate(); err == nil {
		t.Error("expected error: auto_cleanup enabled with cleanup_after_days < 1")
	}
	// auto_cleanup enabled with a valid window is fine.
	c2 := valid()
	c2.Output.AutoCleanup = true
	c2.Output.CleanupAfterDays = 7
	if err := c2.Validate(); err != nil {
		t.Errorf("valid cleanup config rejected: %v", err)
	}
	// Disabled cleanup does not require a window.
	c3 := valid()
	c3.Output.CleanupAfterDays = 0
	if err := c3.Validate(); err != nil {
		t.Errorf("disabled cleanup should not require a window: %v", err)
	}
}
