package ws

import (
	"testing"
)

func TestExtractTickerAndCategory(t *testing.T) {
	tests := []struct {
		name             string
		group            string
		separator        string
		validCategories  []string
		expectedTicker   string
		expectedCategory string
	}{
		// GEX categories with _state_ separator
		{
			name:             "gex_full category",
			group:            "blue_SPX_state_gex_full",
			separator:        "_state_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			expectedTicker:   "SPX",
			expectedCategory: "gex_full",
		},
		{
			name:             "gex_zero category",
			group:            "blue_SPX_state_gex_zero",
			separator:        "_state_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			expectedTicker:   "SPX",
			expectedCategory: "gex_zero",
		},
		{
			name:             "gex_one category",
			group:            "blue_SPX_state_gex_one",
			separator:        "_state_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			expectedTicker:   "SPX",
			expectedCategory: "gex_one",
		},
		// Multi-part ticker (e.g., ES_SPX)
		{
			name:             "multi-part ticker",
			group:            "blue_ES_SPX_state_gex_full",
			separator:        "_state_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			expectedTicker:   "ES_SPX",
			expectedCategory: "gex_full",
		},
		// Classic categories with _classic_ separator
		{
			name:             "classic gex_full",
			group:            "blue_SPX_classic_gex_full",
			separator:        "_classic_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			expectedTicker:   "SPX",
			expectedCategory: "gex_full",
		},
		// Greek zero categories
		{
			name:             "delta_zero category",
			group:            "blue_SPX_state_delta_zero",
			separator:        "_state_",
			validCategories:  []string{"delta_zero", "gamma_zero", "vanna_zero", "charm_zero"},
			expectedTicker:   "SPX",
			expectedCategory: "delta_zero",
		},
		{
			name:             "gamma_zero category",
			group:            "blue_SPX_state_gamma_zero",
			separator:        "_state_",
			validCategories:  []string{"delta_zero", "gamma_zero", "vanna_zero", "charm_zero"},
			expectedTicker:   "SPX",
			expectedCategory: "gamma_zero",
		},
		// Greek one categories
		{
			name:             "delta_one category",
			group:            "blue_SPX_state_delta_one",
			separator:        "_state_",
			validCategories:  []string{"delta_one", "gamma_one", "vanna_one", "charm_one"},
			expectedTicker:   "SPX",
			expectedCategory: "delta_one",
		},
		// Invalid cases
		{
			name:             "invalid category",
			group:            "blue_SPX_state_invalid",
			separator:        "_state_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			expectedTicker:   "",
			expectedCategory: "",
		},
		{
			name:             "wrong separator",
			group:            "blue_SPX_state_gex_full",
			separator:        "_classic_",
			validCategories:  []string{"gex_full"},
			expectedTicker:   "",
			expectedCategory: "",
		},
		{
			name:             "missing prefix",
			group:            "_state_gex_full",
			separator:        "_state_",
			validCategories:  []string{"gex_full"},
			expectedTicker:   "",
			expectedCategory: "",
		},
		{
			name:             "no underscore in prefix",
			group:            "blue_state_gex_full",
			separator:        "_state_",
			validCategories:  []string{"gex_full"},
			expectedTicker:   "",
			expectedCategory: "",
		},
		// No category validation
		{
			name:             "no category validation",
			group:            "blue_SPX_state_anything",
			separator:        "_state_",
			validCategories:  nil,
			expectedTicker:   "SPX",
			expectedCategory: "anything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, category := ExtractTickerAndCategory(tt.group, tt.separator, tt.validCategories)
			if ticker != tt.expectedTicker {
				t.Errorf("ticker = %q, want %q", ticker, tt.expectedTicker)
			}
			if category != tt.expectedCategory {
				t.Errorf("category = %q, want %q", category, tt.expectedCategory)
			}
		})
	}
}

func TestMakeGroupParser(t *testing.T) {
	tests := []struct {
		name             string
		separator        string
		validCategories  []string
		group            string
		expectedTicker   string
		expectedCategory string
	}{
		{
			name:             "gex parser",
			separator:        "_state_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			group:            "blue_SPX_state_gex_zero",
			expectedTicker:   "SPX",
			expectedCategory: "gex_zero",
		},
		{
			name:             "classic parser",
			separator:        "_classic_",
			validCategories:  []string{"gex_full", "gex_zero", "gex_one"},
			group:            "blue_SPX_classic_gex_one",
			expectedTicker:   "SPX",
			expectedCategory: "gex_one",
		},
		{
			name:             "greek zero parser",
			separator:        "_state_",
			validCategories:  []string{"delta_zero", "gamma_zero", "vanna_zero", "charm_zero"},
			group:            "blue_NDX_state_vanna_zero",
			expectedTicker:   "NDX",
			expectedCategory: "vanna_zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := MakeGroupParser(tt.separator, tt.validCategories)
			result := parser(tt.group)
			if result.Ticker != tt.expectedTicker {
				t.Errorf("ticker = %q, want %q", result.Ticker, tt.expectedTicker)
			}
			if result.Category != tt.expectedCategory {
				t.Errorf("category = %q, want %q", result.Category, tt.expectedCategory)
			}
		})
	}
}

func TestMakeOrderflowGroupParser(t *testing.T) {
	tests := []struct {
		name             string
		group            string
		expectedTicker   string
		expectedCategory string
	}{
		{
			name:             "valid orderflow group",
			group:            "blue_SPX_orderflow_orderflow",
			expectedTicker:   "SPX",
			expectedCategory: "orderflow",
		},
		{
			name:             "multi-part ticker orderflow",
			group:            "blue_ES_SPX_orderflow_orderflow",
			expectedTicker:   "ES_SPX",
			expectedCategory: "orderflow",
		},
		{
			name:             "invalid orderflow group - wrong suffix",
			group:            "blue_SPX_state_gex_full",
			expectedTicker:   "",
			expectedCategory: "",
		},
		{
			name:             "invalid orderflow group - missing prefix",
			group:            "_orderflow_orderflow",
			expectedTicker:   "",
			expectedCategory: "",
		},
		{
			name:             "invalid orderflow group - no underscore",
			group:            "blue_orderflow_orderflow",
			expectedTicker:   "",
			expectedCategory: "",
		},
	}

	parser := MakeOrderflowGroupParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser(tt.group)
			if result.Ticker != tt.expectedTicker {
				t.Errorf("ticker = %q, want %q", result.Ticker, tt.expectedTicker)
			}
			if result.Category != tt.expectedCategory {
				t.Errorf("category = %q, want %q", result.Category, tt.expectedCategory)
			}
		})
	}
}

func TestStreamerConfigCategories(t *testing.T) {
	// Test that the category lists match what was originally in the extract functions
	t.Run("gexCategories", func(t *testing.T) {
		expected := map[string]bool{"gex_full": true, "gex_zero": true, "gex_one": true}
		for _, cat := range gexCategories {
			if !expected[cat] {
				t.Errorf("unexpected category in gexCategories: %s", cat)
			}
		}
		if len(gexCategories) != len(expected) {
			t.Errorf("gexCategories length = %d, want %d", len(gexCategories), len(expected))
		}
	})

	t.Run("classicCategories", func(t *testing.T) {
		expected := map[string]bool{"gex_full": true, "gex_zero": true, "gex_one": true}
		for _, cat := range classicCategories {
			if !expected[cat] {
				t.Errorf("unexpected category in classicCategories: %s", cat)
			}
		}
		if len(classicCategories) != len(expected) {
			t.Errorf("classicCategories length = %d, want %d", len(classicCategories), len(expected))
		}
	})

	t.Run("greekZeroCategories", func(t *testing.T) {
		expected := map[string]bool{"delta_zero": true, "gamma_zero": true, "vanna_zero": true, "charm_zero": true}
		for _, cat := range greekZeroCategories {
			if !expected[cat] {
				t.Errorf("unexpected category in greekZeroCategories: %s", cat)
			}
		}
		if len(greekZeroCategories) != len(expected) {
			t.Errorf("greekZeroCategories length = %d, want %d", len(greekZeroCategories), len(expected))
		}
	})

	t.Run("greekOneCategories", func(t *testing.T) {
		expected := map[string]bool{"delta_one": true, "gamma_one": true, "vanna_one": true, "charm_one": true}
		for _, cat := range greekOneCategories {
			if !expected[cat] {
				t.Errorf("unexpected category in greekOneCategories: %s", cat)
			}
		}
		if len(greekOneCategories) != len(expected) {
			t.Errorf("greekOneCategories length = %d, want %d", len(greekOneCategories), len(expected))
		}
	})
}
