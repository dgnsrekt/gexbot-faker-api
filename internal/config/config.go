package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	API      APIConfig      `mapstructure:"api"`
	Download DownloadConfig `mapstructure:"download"`
	Tickers  []string       `mapstructure:"tickers"`
	Packages PackagesConfig `mapstructure:"packages"`
	Output   OutputConfig   `mapstructure:"output"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type APIConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	APIKey     string `mapstructure:"api_key"`
	TimeoutSec int    `mapstructure:"timeout_sec"`
	RetryCount int    `mapstructure:"retry_count"`
	RetryDelay int    `mapstructure:"retry_delay_sec"`
}

type DownloadConfig struct {
	Workers       int  `mapstructure:"workers"`
	RatePerSecond int  `mapstructure:"rate_per_second"`
	ResumeEnabled bool `mapstructure:"resume_enabled"`
}

type PackagesConfig struct {
	State     PackageConfig `mapstructure:"state"`
	Classic   PackageConfig `mapstructure:"classic"`
	Orderflow PackageConfig `mapstructure:"orderflow"`
}

type PackageConfig struct {
	Enabled    bool     `mapstructure:"enabled"`
	Categories []string `mapstructure:"categories"`
}

type OutputConfig struct {
	Directory          string `mapstructure:"directory"`
	AutoConvertToJSONL bool   `mapstructure:"auto_convert_to_jsonl"`
}

type LoggingConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Directory string `mapstructure:"directory"`
	Level     string `mapstructure:"level"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("api.base_url", "https://api.gex.bot")
	v.SetDefault("api.timeout_sec", 300)
	v.SetDefault("api.retry_count", 3)
	v.SetDefault("api.retry_delay_sec", 5)
	v.SetDefault("download.workers", 3)
	v.SetDefault("download.rate_per_second", 2)
	v.SetDefault("download.resume_enabled", true)
	v.SetDefault("output.directory", "data")
	v.SetDefault("output.auto_convert_to_jsonl", true)
	v.SetDefault("logging.enabled", true)
	v.SetDefault("logging.directory", "logs")
	v.SetDefault("logging.level", "info")

	// Environment variable support
	v.SetEnvPrefix("GEXBOT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Explicitly bind nested keys to env vars
	_ = v.BindEnv("api.api_key", "GEXBOT_API_KEY")

	// Load config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("default")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.API.APIKey == "" {
		return fmt.Errorf("api_key is required (set GEXBOT_API_KEY env var)")
	}
	if c.Download.Workers < 1 {
		return fmt.Errorf("workers must be >= 1")
	}
	return nil
}
