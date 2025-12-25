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
