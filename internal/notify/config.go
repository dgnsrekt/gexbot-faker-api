package notify

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config holds ntfy notification configuration.
type Config struct {
	Enabled  bool   // Whether notifications are enabled
	Server   string // ntfy server URL (default: https://ntfy.sh)
	Topic    string // Topic name (required if enabled)
	Priority string // Message priority: min, low, default, high, urgent
	Tags     string // Comma-separated emoji tags (e.g., "package,rocket")
	Token    string // Optional access token for private topics
}

// LoadConfig loads notification config from environment variables.
func LoadConfig() *Config {
	return &Config{
		Enabled:  getEnvBoolOrDefault("NTFY_ENABLED", false),
		Server:   getEnvOrDefault("NTFY_SERVER", "https://ntfy.sh"),
		Topic:    os.Getenv("NTFY_TOPIC"),
		Priority: getEnvOrDefault("NTFY_PRIORITY", "default"),
		Tags:     getEnvOrDefault("NTFY_TAGS", "package"),
		Token:    os.Getenv("NTFY_TOKEN"),
	}
}

// Validate checks configuration is valid when enabled.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.Topic == "" {
		return errors.New("NTFY_TOPIC is required when NTFY_ENABLED=true")
	}

	validPriorities := map[string]bool{
		"min": true, "low": true, "default": true, "high": true, "urgent": true,
	}
	if !validPriorities[c.Priority] {
		return fmt.Errorf("invalid NTFY_PRIORITY: %s (valid: min, low, default, high, urgent)", c.Priority)
	}

	return nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvBoolOrDefault(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
