package config

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
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
		handler = &prettyHandler{level: logLevel, w: os.Stdout}
	}
	slog.SetDefault(slog.New(handler))
}

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

func levelColor(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return colorCyan
	case l < slog.LevelWarn:
		return colorGreen
	case l < slog.LevelError:
		return colorYellow
	default:
		return colorRed
	}
}

type prettyHandler struct {
	level  slog.Level
	w      io.Writer
	mu     sync.Mutex
	attrs  []slog.Attr
	groups []string
}

func (h *prettyHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
	color := levelColor(r.Level)
	ts := r.Time.Format(time.Kitchen)
	line := fmt.Sprintf("%s %s[%s]%s %s\n", ts, color, r.Level, colorReset, r.Message)

	var attrs []string
	for _, a := range h.attrs {
		attrs = append(attrs, fmt.Sprintf("        %s=%v", h.groupedKey(a.Key), a.Value))
	}
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("        %s=%v", h.groupedKey(a.Key), a.Value))
		return true
	})
	if len(attrs) > 0 {
		line += strings.Join(attrs, "\n") + "\n"
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, line)
	return err
}

func (h *prettyHandler) groupedKey(key string) string {
	if len(h.groups) == 0 {
		return key
	}
	return strings.Join(h.groups, ".") + "." + key
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &prettyHandler{level: h.level, w: h.w, attrs: newAttrs, groups: h.groups}
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &prettyHandler{level: h.level, w: h.w, attrs: h.attrs, groups: newGroups}
}
