package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/thoscut/scanflow/server/internal/api"
	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
	"github.com/thoscut/scanflow/server/internal/output"
	"github.com/thoscut/scanflow/server/internal/processor"
	"github.com/thoscut/scanflow/server/internal/scanner"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/etc/scanflow/server.toml", "path to config file")
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("scanflow-server", version)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup logging
	setupLogging(cfg.Logging)

	slog.Info("starting scanflow-server", "version", version)

	// Initialize components
	sc := scanner.New(cfg.Scanner.Device, cfg.Scanner.AutoOpen, scanner.ScanOptions{
		Resolution: cfg.Scanner.Defaults.Resolution,
		Mode:       cfg.Scanner.Defaults.Mode,
		Source:     cfg.Scanner.Defaults.Source,
		PageWidth:  cfg.Scanner.Defaults.PageWidth,
		PageHeight: cfg.Scanner.Defaults.PageHeight,
	})

	jobQueue := jobs.NewQueue()

	profilesDir := filepath.Join(filepath.Dir(*configPath), "profiles")
	profiles, err := config.NewProfileStore(profilesDir)
	if err != nil {
		slog.Warn("failed to load profiles from directory, using defaults", "dir", profilesDir, "error", err)
		profiles, _ = config.NewProfileStore("")
	}

	proc := processor.NewPipeline(cfg.Processing)
	outputs := output.NewManager(cfg.Output)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup button watcher if enabled
	if cfg.Button.Enabled {
		btnCfg := scanner.ButtonConfig{
			Enabled:           cfg.Button.Enabled,
			PollInterval:      cfg.Button.PollInterval.Duration(),
			LongPressDuration: cfg.Button.LongPressDuration.Duration(),
			ShortPressProfile: cfg.Button.ShortPressProfile,
			LongPressProfile:  cfg.Button.LongPressProfile,
			Output:            cfg.Button.Output,
			BeepOnLongPress:   cfg.Button.BeepOnLongPress,
		}

		onShortPress := func() {
			job := jobs.NewJob(btnCfg.ShortPressProfile, jobs.OutputConfig{
				Target: btnCfg.Output,
			}, nil, nil)
			slog.Info("button short press scan", "profile", btnCfg.ShortPressProfile)
			jobQueue.Submit(job)
		}

		onLongPress := func() {
			job := jobs.NewJob(btnCfg.LongPressProfile, jobs.OutputConfig{
				Target: btnCfg.Output,
			}, nil, nil)
			slog.Info("button long press scan", "profile", btnCfg.LongPressProfile)
			jobQueue.Submit(job)
		}

		bw := scanner.NewButtonWatcher(sc, btnCfg, onShortPress, onLongPress)
		go bw.Start(ctx)
	}

	// Create and start API server
	srv := api.NewServer(cfg, sc, jobQueue, profiles, proc, outputs)

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutdown signal received")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	if err := srv.Start(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func setupLogging(cfg config.LoggingConfig) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
