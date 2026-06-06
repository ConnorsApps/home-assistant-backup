package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/ConnorsApps/hass-backup/pkg/storage"
)

type backupInfo struct {
	key       string
	timestamp time.Time
}

func cleanupOldBackups(ctx context.Context, store storage.ObjectStore, prefix string, keepLast int) error {
	slog.Info("Checking for old backups to clean up", "keep_last", keepLast)

	keys, err := store.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}

	if len(keys) <= keepLast {
		slog.Info("No cleanup needed", "count", len(keys), "keep_last", keepLast)
		return nil
	}

	var backups []backupInfo
	for _, key := range keys {
		ts, err := parseTimestampFromKey(key, prefix)
		if err != nil {
			slog.Debug("Skipping unrecognized key", "key", key, "error", err)
			continue
		}
		backups = append(backups, backupInfo{key: key, timestamp: ts})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].timestamp.After(backups[j].timestamp)
	})

	deleted, failed := 0, 0
	for i := keepLast; i < len(backups); i++ {
		b := backups[i]
		slog.Info("Deleting old backup", "key", b.key, "timestamp", b.timestamp)
		if err := store.Delete(ctx, b.key); err != nil {
			slog.Warn("Failed to delete backup", "key", b.key, "error", err)
			failed++
		} else {
			deleted++
		}
	}

	slog.Info("Cleanup complete", "deleted", deleted, "failed", failed, "remaining", keepLast+failed)
	return nil
}

// parseTimestampFromKey extracts the timestamp from a key like "prefix<timestamp>-<slug>.tar".
func parseTimestampFromKey(key, prefix string) (time.Time, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(key, prefix), ".tar")
	// key format: <rfc3339-timestamp>-<slug>; split on last '-' since slug has no hyphens
	idx := strings.LastIndex(name, "-")
	if idx == -1 {
		return time.Time{}, fmt.Errorf("no separator found in key")
	}
	return time.Parse(time.RFC3339, name[:idx])
}
