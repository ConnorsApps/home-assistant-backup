package config

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Marlliton/slogpretty"
)

func (cfg *Config) SetupLogger() {
	logLevel, err := ParseSlogLevel(cfg.Logging.Level)
	if err != nil {
		slog.Error("Invalid log level, using INFO", "error", err)
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	switch strings.ToLower(cfg.Logging.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	case "text":
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	default:
		handler = slogpretty.New(os.Stdout, &slogpretty.Options{
			Level:      logLevel,
			TimeFormat: time.Kitchen,
			Colorful:   true,
			Multiline:  true,
		})
	}
	slog.SetDefault(slog.New(handler))
}
