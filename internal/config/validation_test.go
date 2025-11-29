package config

import (
	"strings"
	"testing"
)

func TestValidateDownloadConfig_ValidConfig(t *testing.T) {
	tickers := []string{"SPX", "SPY", "QQQ"}
	packages := PackagesConfig{
		State: PackageConfig{
			Enabled:    true,
			Categories: []string{"gex_full", "gex_zero"},
		},
		Classic: PackageConfig{
			Enabled:    true,
			Categories: []string{"gex_full"},
		},
	}

	err := ValidateDownloadConfig(tickers, packages)
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestValidateDownloadConfig_InvalidTicker(t *testing.T) {
	tickers := []string{"SPX", "INVALID_TICKER", "QQQ"}
	packages := PackagesConfig{
		State: PackageConfig{
			Enabled:    true,
			Categories: []string{"gex_full"},
		},
	}

	err := ValidateDownloadConfig(tickers, packages)
	if err == nil {
		t.Error("expected error for invalid ticker")
	}

	if !strings.Contains(err.Error(), "INVALID_TICKER") {
		t.Errorf("error should mention invalid ticker, got: %v", err)
	}
}

func TestValidateDownloadConfig_InvalidCategoryForClassic(t *testing.T) {
	tickers := []string{"SPX"}
	packages := PackagesConfig{
		Classic: PackageConfig{
			Enabled:    true,
			Categories: []string{"charm_zero"}, // Invalid for classic (greeks are state-only)
		},
	}

	err := ValidateDownloadConfig(tickers, packages)
	if err == nil {
		t.Error("expected error for invalid classic category")
	}

	if !strings.Contains(err.Error(), "classic/charm_zero") {
		t.Errorf("error should mention classic/charm_zero, got: %v", err)
	}
	if !strings.Contains(err.Error(), "classic only supports:") {
		t.Errorf("error should show valid categories, got: %v", err)
	}
}

func TestValidateDownloadConfig_DisabledPackageSkipped(t *testing.T) {
	tickers := []string{"SPX"}
	packages := PackagesConfig{
		Classic: PackageConfig{
			Enabled:    false, // Disabled, so invalid category should be ignored
			Categories: []string{"charm_zero"},
		},
		State: PackageConfig{
			Enabled:    true,
			Categories: []string{"gex_full"},
		},
	}

	err := ValidateDownloadConfig(tickers, packages)
	if err != nil {
		t.Errorf("disabled package should not be validated, got: %v", err)
	}
}

func TestValidateDownloadConfig_MultipleErrors(t *testing.T) {
	tickers := []string{"INVALID1", "INVALID2"}
	packages := PackagesConfig{
		Classic: PackageConfig{
			Enabled:    true,
			Categories: []string{"charm_zero", "delta_zero"}, // Both invalid for classic (greeks are state-only)
		},
	}

	err := ValidateDownloadConfig(tickers, packages)
	if err == nil {
		t.Error("expected error for multiple issues")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "INVALID1") || !strings.Contains(errStr, "INVALID2") {
		t.Errorf("error should list all invalid tickers, got: %v", err)
	}
	if !strings.Contains(errStr, "classic/charm_zero") || !strings.Contains(errStr, "classic/delta_zero") {
		t.Errorf("error should list all invalid categories, got: %v", err)
	}
}
