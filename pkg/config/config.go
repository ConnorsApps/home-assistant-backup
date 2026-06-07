package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HomeAssistant HomeAssistantConfig `json:"homeAssistant" yaml:"homeAssistant" envPrefix:"HASS_"`
	Storage       StorageConfig       `json:"storage" yaml:"storage" envPrefix:"STORAGE_"`
	Logging       LoggingConfig       `json:"logging" yaml:"logging" envPrefix:"LOG_"`
	Retention     RetentionConfig     `json:"retention" yaml:"retention" envPrefix:"RETENTION_"`
}

type HomeAssistantConfig struct {
	URL                 string `json:"url" yaml:"url" env:"URL"`
	Token               string `json:"token" yaml:"token" env:"TOKEN"`
	Timeout             string `json:"timeout" yaml:"timeout" env:"TIMEOUT"`
	InsecureSkipVerify  bool   `json:"insecureSkipVerify" yaml:"insecureSkipVerify" env:"INSECURE"`
	DeleteAfterTransfer bool   `json:"deleteAfterTransfer" yaml:"deleteAfterTransfer" env:"DELETE_AFTER_TRANSFER"`
}

type StorageConfig struct {
	URL    string `json:"url" yaml:"url" env:"URL"`
	Prefix string `json:"prefix" yaml:"prefix" env:"PREFIX"`
}

type LoggingConfig struct {
	Level  string `json:"level" yaml:"level" env:"LEVEL"`
	Format string `json:"format" yaml:"format" env:"FORMAT"`
}

type RetentionConfig struct {
	KeepLast int `json:"keepLast" yaml:"keepLast" env:"KEEP_LAST"`
}

func DefaultConfig() *Config {
	return &Config{
		HomeAssistant: HomeAssistantConfig{
			Timeout:             "10m",
			InsecureSkipVerify:  false,
			DeleteAfterTransfer: true,
		},
		Storage: StorageConfig{
			URL:    "file://./backups",
			Prefix: "home-assistant/",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "pretty",
		},
		Retention: RetentionConfig{
			KeepLast: 30,
		},
	}
}

func ParseSlogLevel(v string) (slog.Level, error) {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level: %s", v)
	}
}

func (c *Config) Validate() error {
	var errs []string

	if c.HomeAssistant.URL == "" {
		errs = append(errs, "homeAssistant.url is required")
	}
	if c.HomeAssistant.Token == "" {
		errs = append(errs, "homeAssistant.token is required")
	}
	if c.HomeAssistant.Timeout == "" {
		errs = append(errs, "homeAssistant.timeout is required")
	} else if _, err := time.ParseDuration(c.HomeAssistant.Timeout); err != nil {
		errs = append(errs, fmt.Sprintf("homeAssistant.timeout invalid: %v", err))
	}
	if c.Storage.URL == "" {
		errs = append(errs, "storage.url is required")
	}
	if c.Retention.KeepLast < 0 {
		errs = append(errs, "retention.keepLast must be non-negative")
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
	if !validLevels[strings.ToLower(c.Logging.Level)] {
		errs = append(errs, "logging.level must be one of: debug, info, warn, error")
	}
	validFormats := map[string]bool{"pretty": true, "text": true, "json": true}
	if !validFormats[strings.ToLower(c.Logging.Format)] {
		errs = append(errs, "logging.format must be one of: pretty, text, json")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func applyEnv(cfg *Config) error {
	if v, ok := os.LookupEnv("HASS_URL"); ok {
		cfg.HomeAssistant.URL = v
	}
	if v, ok := os.LookupEnv("HASS_TOKEN"); ok {
		cfg.HomeAssistant.Token = v
	}
	if v, ok := os.LookupEnv("HASS_TIMEOUT"); ok {
		cfg.HomeAssistant.Timeout = v
	}
	if v, ok := os.LookupEnv("HASS_INSECURE"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("HASS_INSECURE must be a boolean: %w", err)
		}
		cfg.HomeAssistant.InsecureSkipVerify = b
	}
	if v, ok := os.LookupEnv("HASS_DELETE_AFTER_TRANSFER"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("HASS_DELETE_AFTER_TRANSFER must be a boolean: %w", err)
		}
		cfg.HomeAssistant.DeleteAfterTransfer = b
	}
	if v, ok := os.LookupEnv("STORAGE_URL"); ok {
		cfg.Storage.URL = v
	}
	if v, ok := os.LookupEnv("STORAGE_PREFIX"); ok {
		cfg.Storage.Prefix = v
	}
	if v, ok := os.LookupEnv("LOG_LEVEL"); ok {
		cfg.Logging.Level = v
	}
	if v, ok := os.LookupEnv("LOG_FORMAT"); ok {
		cfg.Logging.Format = v
	}
	if v, ok := os.LookupEnv("RETENTION_KEEP_LAST"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("RETENTION_KEEP_LAST must be an integer: %w", err)
		}
		cfg.Retention.KeepLast = n
	}
	return nil
}

func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	if err := applyEnv(cfg); err != nil {
		return nil, fmt.Errorf("parse environment variables: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration invalid: %w", err)
	}

	return cfg, nil
}
