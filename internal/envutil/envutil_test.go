package envutil

import (
	"os"
	"testing"
	"time"
)

func TestGetString(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		envValue   string
		setEnv     bool
		defaultVal string
		want       string
	}{
		{
			name:       "returns env value when set",
			key:        "TEST_STRING_VAR",
			envValue:   "custom_value",
			setEnv:     true,
			defaultVal: "default",
			want:       "custom_value",
		},
		{
			name:       "returns default when env not set",
			key:        "TEST_STRING_UNSET",
			setEnv:     false,
			defaultVal: "fallback",
			want:       "fallback",
		},
		{
			name:       "returns default when env is empty string",
			key:        "TEST_STRING_EMPTY",
			envValue:   "",
			setEnv:     true,
			defaultVal: "default_for_empty",
			want:       "default_for_empty",
		},
		{
			name:       "preserves whitespace in value",
			key:        "TEST_STRING_WHITESPACE",
			envValue:   "  spaced  ",
			setEnv:     true,
			defaultVal: "default",
			want:       "  spaced  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before and after
			if err := os.Unsetenv(tt.key); err != nil {
				t.Fatalf("Unsetenv(%q): %v", tt.key, err)
			}
			t.Cleanup(func() {
				if err := os.Unsetenv(tt.key); err != nil {
					t.Fatalf("Unsetenv(%q) cleanup: %v", tt.key, err)
				}
			})

			if tt.setEnv {
				if err := os.Setenv(tt.key, tt.envValue); err != nil {
					t.Fatalf("Setenv(%q): %v", tt.key, err)
				}
			}

			got := GetString(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetString(%q, %q) = %q, want %q", tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		envValue   string
		setEnv     bool
		defaultVal bool
		want       bool
	}{
		{
			name:       "returns true for 'true'",
			key:        "TEST_BOOL_TRUE",
			envValue:   "true",
			setEnv:     true,
			defaultVal: false,
			want:       true,
		},
		{
			name:       "returns true for 'TRUE'",
			key:        "TEST_BOOL_TRUE_UPPER",
			envValue:   "TRUE",
			setEnv:     true,
			defaultVal: false,
			want:       true,
		},
		{
			name:       "returns true for '1'",
			key:        "TEST_BOOL_ONE",
			envValue:   "1",
			setEnv:     true,
			defaultVal: false,
			want:       true,
		},
		{
			name:       "returns true for 'T'",
			key:        "TEST_BOOL_T",
			envValue:   "T",
			setEnv:     true,
			defaultVal: false,
			want:       true,
		},
		{
			name:       "returns false for 'false'",
			key:        "TEST_BOOL_FALSE",
			envValue:   "false",
			setEnv:     true,
			defaultVal: true,
			want:       false,
		},
		{
			name:       "returns false for 'FALSE'",
			key:        "TEST_BOOL_FALSE_UPPER",
			envValue:   "FALSE",
			setEnv:     true,
			defaultVal: true,
			want:       false,
		},
		{
			name:       "returns false for '0'",
			key:        "TEST_BOOL_ZERO",
			envValue:   "0",
			setEnv:     true,
			defaultVal: true,
			want:       false,
		},
		{
			name:       "returns false for 'F'",
			key:        "TEST_BOOL_F",
			envValue:   "F",
			setEnv:     true,
			defaultVal: true,
			want:       false,
		},
		{
			name:       "returns default when env not set",
			key:        "TEST_BOOL_UNSET",
			setEnv:     false,
			defaultVal: true,
			want:       true,
		},
		{
			name:       "returns default for invalid value",
			key:        "TEST_BOOL_INVALID",
			envValue:   "yes",
			setEnv:     true,
			defaultVal: false,
			want:       false,
		},
		{
			name:       "returns default for empty string",
			key:        "TEST_BOOL_EMPTY",
			envValue:   "",
			setEnv:     true,
			defaultVal: true,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Unsetenv(tt.key); err != nil {
				t.Fatalf("Unsetenv(%q): %v", tt.key, err)
			}
			t.Cleanup(func() {
				if err := os.Unsetenv(tt.key); err != nil {
					t.Fatalf("Unsetenv(%q) cleanup: %v", tt.key, err)
				}
			})

			if tt.setEnv {
				if err := os.Setenv(tt.key, tt.envValue); err != nil {
					t.Fatalf("Setenv(%q): %v", tt.key, err)
				}
			}

			got := GetBool(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetBool(%q, %v) = %v, want %v", tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		envValue   string
		setEnv     bool
		defaultVal int
		want       int
	}{
		{
			name:       "returns positive int",
			key:        "TEST_INT_POS",
			envValue:   "42",
			setEnv:     true,
			defaultVal: 0,
			want:       42,
		},
		{
			name:       "returns negative int",
			key:        "TEST_INT_NEG",
			envValue:   "-10",
			setEnv:     true,
			defaultVal: 0,
			want:       -10,
		},
		{
			name:       "returns zero",
			key:        "TEST_INT_ZERO",
			envValue:   "0",
			setEnv:     true,
			defaultVal: 99,
			want:       0,
		},
		{
			name:       "returns default when env not set",
			key:        "TEST_INT_UNSET",
			setEnv:     false,
			defaultVal: 100,
			want:       100,
		},
		{
			name:       "returns default for non-integer value",
			key:        "TEST_INT_INVALID",
			envValue:   "abc",
			setEnv:     true,
			defaultVal: 50,
			want:       50,
		},
		{
			name:       "returns default for float value",
			key:        "TEST_INT_FLOAT",
			envValue:   "3.14",
			setEnv:     true,
			defaultVal: 3,
			want:       3,
		},
		{
			name:       "returns default for empty string",
			key:        "TEST_INT_EMPTY",
			envValue:   "",
			setEnv:     true,
			defaultVal: 25,
			want:       25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Unsetenv(tt.key); err != nil {
				t.Fatalf("Unsetenv(%q): %v", tt.key, err)
			}
			t.Cleanup(func() {
				if err := os.Unsetenv(tt.key); err != nil {
					t.Fatalf("Unsetenv(%q) cleanup: %v", tt.key, err)
				}
			})

			if tt.setEnv {
				if err := os.Setenv(tt.key, tt.envValue); err != nil {
					t.Fatalf("Setenv(%q): %v", tt.key, err)
				}
			}

			got := GetInt(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetInt(%q, %d) = %d, want %d", tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestGetDuration(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		envValue   string
		setEnv     bool
		defaultVal time.Duration
		want       time.Duration
	}{
		{
			name:       "returns seconds",
			key:        "TEST_DUR_SEC",
			envValue:   "5s",
			setEnv:     true,
			defaultVal: time.Second,
			want:       5 * time.Second,
		},
		{
			name:       "returns milliseconds",
			key:        "TEST_DUR_MS",
			envValue:   "500ms",
			setEnv:     true,
			defaultVal: time.Second,
			want:       500 * time.Millisecond,
		},
		{
			name:       "returns minutes",
			key:        "TEST_DUR_MIN",
			envValue:   "2m",
			setEnv:     true,
			defaultVal: time.Second,
			want:       2 * time.Minute,
		},
		{
			name:       "returns hours",
			key:        "TEST_DUR_HOUR",
			envValue:   "1h",
			setEnv:     true,
			defaultVal: time.Second,
			want:       time.Hour,
		},
		{
			name:       "returns combined duration",
			key:        "TEST_DUR_COMBINED",
			envValue:   "1h30m",
			setEnv:     true,
			defaultVal: time.Second,
			want:       90 * time.Minute,
		},
		{
			name:       "returns fractional duration",
			key:        "TEST_DUR_FRAC",
			envValue:   "1.5s",
			setEnv:     true,
			defaultVal: time.Second,
			want:       1500 * time.Millisecond,
		},
		{
			name:       "returns default when env not set",
			key:        "TEST_DUR_UNSET",
			setEnv:     false,
			defaultVal: 10 * time.Second,
			want:       10 * time.Second,
		},
		{
			name:       "returns default for invalid value",
			key:        "TEST_DUR_INVALID",
			envValue:   "invalid",
			setEnv:     true,
			defaultVal: 5 * time.Second,
			want:       5 * time.Second,
		},
		{
			name:       "returns default for plain number without unit",
			key:        "TEST_DUR_NOUNIT",
			envValue:   "100",
			setEnv:     true,
			defaultVal: time.Minute,
			want:       time.Minute,
		},
		{
			name:       "returns default for empty string",
			key:        "TEST_DUR_EMPTY",
			envValue:   "",
			setEnv:     true,
			defaultVal: 3 * time.Second,
			want:       3 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Unsetenv(tt.key); err != nil {
				t.Fatalf("Unsetenv(%q): %v", tt.key, err)
			}
			t.Cleanup(func() {
				if err := os.Unsetenv(tt.key); err != nil {
					t.Fatalf("Unsetenv(%q) cleanup: %v", tt.key, err)
				}
			})

			if tt.setEnv {
				if err := os.Setenv(tt.key, tt.envValue); err != nil {
					t.Fatalf("Setenv(%q): %v", tt.key, err)
				}
			}

			got := GetDuration(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetDuration(%q, %v) = %v, want %v", tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}
