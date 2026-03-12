package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/thoscut/scanflow/server/internal/api"
	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
	"github.com/thoscut/scanflow/server/internal/output"
	"github.com/thoscut/scanflow/server/internal/processor"
	"github.com/thoscut/scanflow/server/internal/scanner"
	"github.com/thoscut/scanflow/server/internal/service"
)

var version = "dev"

func main() {
	defaultConfigPath := "/etc/scanflow/server.toml"
	defaultBinaryPath := "/opt/scanflow/scanflow-server"
	if runtime.GOOS == "windows" {
		defaultConfigPath = filepath.Join(os.Getenv("ProgramData"), "ScanFlow", "server.toml")
		defaultBinaryPath = filepath.Join(os.Getenv("ProgramFiles"), "ScanFlow", "scanflow-server.exe")
	}

	configPath := flag.String("config", defaultConfigPath, "path to config file")
	showVersion := flag.Bool("version", false, "show version and exit")
	installService := flag.Bool("install-service", false, "install ScanFlow as a system service")
	uninstallService := flag.Bool("uninstall-service", false, "remove the installed ScanFlow system service")
	serviceName := flag.String("service-name", "scanflow", "service name")
	serviceUser := flag.String("service-user", "scanner", "service user (Linux only)")
	serviceGroup := flag.String("service-group", "scanner", "service group (Linux only)")
	serviceBinary := flag.String("service-binary", defaultBinaryPath, "installed service binary path")
	startService := flag.Bool("start-service", false, "start the service immediately after installation")
	flag.Parse()

	if *showVersion {
		fmt.Println("scanflow-server", version)
		os.Exit(0)
	}

	if *installService && *uninstallService {
		slog.Error("install-service and uninstall-service cannot be used together")
		os.Exit(1)
	}

	if *installService || *uninstallService {
		executable, err := os.Executable()
		if err != nil {
			slog.Error("failed to determine executable path", "error", err)
			os.Exit(1)
		}

		opts := service.Options{
			ServiceName: *serviceName,
			User:        *serviceUser,
			Group:       *serviceGroup,
			BinaryPath:  *serviceBinary,
			ConfigPath:  *configPath,
			StartNow:    *startService,
		}

		if *installService {
			if err := service.Install(executable, opts); err != nil {
				slog.Error("failed to install service", "error", err)
				os.Exit(1)
			}
			slog.Info("service installed successfully", "service", opts.WithDefaults().ServiceName, "config", opts.WithDefaults().ConfigPath)
			os.Exit(0)
		}

		if err := service.Uninstall(opts); err != nil {
			slog.Error("failed to uninstall service", "error", err)
			os.Exit(1)
		}
		slog.Info("service removed successfully", "service", opts.WithDefaults().ServiceName)
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

	if err := srv.Start(ctx); err != nil {
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
