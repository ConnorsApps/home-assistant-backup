package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ConnorsApps/hass-backup/pkg/config"
	"github.com/ConnorsApps/hass-backup/pkg/hass"
	"github.com/ConnorsApps/hass-backup/pkg/storage"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		slog.Info("Home Assistant Backup", "version", Version, "commit", Commit)
		os.Exit(0)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	cfg.SetupLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("Starting Home Assistant backup",
		"version", Version,
		"hassURL", cfg.HomeAssistant.URL,
		"storageURL", cfg.Storage.URL,
		"prefix", cfg.Storage.Prefix,
	)

	timeout, err := time.ParseDuration(cfg.HomeAssistant.Timeout)
	if err != nil {
		slog.Error("Invalid timeout", "error", err)
		os.Exit(1)
	}

	client := hass.NewClient(cfg.HomeAssistant.URL, cfg.HomeAssistant.Token, hass.ClientOptions{
		InsecureSkipVerify: cfg.HomeAssistant.InsecureSkipVerify,
		Timeout:            timeout,
	})

	slog.Info("Creating backup (this may take several minutes)...")
	backupCtx, backupCancel := context.WithTimeout(ctx, timeout)
	defer backupCancel()

	slug, err := client.CreateBackup(backupCtx)
	if err != nil {
		slog.Error("Failed to create backup", "error", err)
		os.Exit(1)
	}
	slog.Info("Backup created", "slug", slug)

	slog.Info("Downloading backup...")
	downloadCtx, downloadCancel := context.WithTimeout(ctx, timeout)
	defer downloadCancel()

	rc, err := client.DownloadBackup(downloadCtx, slug)
	if err != nil {
		slog.Error("Failed to download backup", "error", err)
		os.Exit(1)
	}
	defer rc.Close()

	store, err := storage.Open(ctx, cfg.Storage.URL)
	if err != nil {
		slog.Error("Failed to open storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	key := fmt.Sprintf("%s%s-%s.tar", cfg.Storage.Prefix, time.Now().UTC().Format(time.RFC3339), slug)
	slog.Info("Uploading backup", "key", key)

	written, err := store.Put(ctx, key, rc)
	if err != nil {
		slog.Error("Failed to upload backup", "error", err)
		os.Exit(1)
	}
	slog.Info("Backup uploaded successfully", "key", key, "bytes", written)

	if cfg.HomeAssistant.DeleteAfterTransfer {
		if err := client.DeleteBackup(ctx, slug); err != nil {
			slog.Warn("Failed to delete backup from Home Assistant", "slug", slug, "error", err)
		} else {
			slog.Info("Deleted backup from Home Assistant", "slug", slug)
		}
	}

	if cfg.Retention.KeepLast > 0 {
		if err := cleanupOldBackups(ctx, store, cfg.Storage.Prefix, cfg.Retention.KeepLast); err != nil {
			slog.Warn("Cleanup failed", "error", err)
		}
	}

	slog.Info("Done")
}
