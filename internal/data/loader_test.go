package data

import "testing"

func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int64
	}{
		{
			name:     "valid JSON with timestamp",
			input:    []byte(`{"timestamp":1699123456789,"data":"test"}`),
			expected: 1699123456789,
		},
		{
			name:     "valid JSON with timestamp only",
			input:    []byte(`{"timestamp":1234567890}`),
			expected: 1234567890,
		},
		{
			name:     "timestamp at different position in JSON",
			input:    []byte(`{"data":"test","timestamp":9876543210,"other":"value"}`),
			expected: 9876543210,
		},
		{
			name:     "zero timestamp",
			input:    []byte(`{"timestamp":0}`),
			expected: 0,
		},
		{
			name:     "negative timestamp",
			input:    []byte(`{"timestamp":-100}`),
			expected: -100,
		},
		{
			name:     "JSON without timestamp field",
			input:    []byte(`{"data":"test","other":123}`),
			expected: 0,
		},
		{
			name:     "empty JSON object",
			input:    []byte(`{}`),
			expected: 0,
		},
		{
			name:     "invalid JSON",
			input:    []byte(`not json`),
			expected: 0,
		},
		{
			name:     "empty input",
			input:    []byte(``),
			expected: 0,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: 0,
		},
		{
			name:     "timestamp as string (should return 0)",
			input:    []byte(`{"timestamp":"1234567890"}`),
			expected: 0,
		},
		{
			name:     "nested timestamp (should not match)",
			input:    []byte(`{"nested":{"timestamp":123}}`),
			expected: 0,
		},
		{
			name:     "large timestamp",
			input:    []byte(`{"timestamp":9223372036854775807}`),
			expected: 9223372036854775807,
		},
		{
			name:     "realistic GEX data format",
			input:    []byte(`{"timestamp":1699300800000,"ticker":"SPX","gex":[{"strike":4500,"gex":123.45}]}`),
			expected: 1699300800000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTimestamp(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractTimestamp(%q) = %d, want %d", string(tt.input), result, tt.expected)
			}
		})
	}
}

func TestDataKey(t *testing.T) {
	tests := []struct {
		ticker   string
		pkg      string
		category string
		expected string
	}{
		{"SPX", "state", "gex_full", "SPX/state/gex_full"},
		{"QQQ", "classic", "gex_zero", "QQQ/classic/gex_zero"},
		{"TSLA", "orderflow", "orderflow", "TSLA/orderflow/orderflow"},
		{"", "", "", "//"},
	}

	for _, tt := range tests {
		name := tt.ticker + "/" + tt.pkg + "/" + tt.category
		t.Run(name, func(t *testing.T) {
			result := DataKey(tt.ticker, tt.pkg, tt.category)
			if result != tt.expected {
				t.Errorf("DataKey(%q, %q, %q) = %q, want %q",
					tt.ticker, tt.pkg, tt.category, result, tt.expected)
			}
		})
	}
}
