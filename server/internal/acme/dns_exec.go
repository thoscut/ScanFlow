package acme

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/thoscut/scanflow/server/internal/config"
)

// execSolver implements DNSSolver by calling external scripts.
// This allows integration with any DNS provider by using custom scripts
// or wrapping tools like certbot or lego.
type execSolver struct {
	createCmd  string
	cleanupCmd string
}

func newExecSolver(cfg config.ACMEExecConfig) (*execSolver, error) {
	if cfg.CreateCommand == "" {
		return nil, fmt.Errorf("exec: create_command is required")
	}
	if cfg.CleanupCommand == "" {
		return nil, fmt.Errorf("exec: cleanup_command is required")
	}
	return &execSolver{
		createCmd:  cfg.CreateCommand,
		cleanupCmd: cfg.CleanupCommand,
	}, nil
}

func (s *execSolver) Present(ctx context.Context, domain, token, keyAuth string) error {
	slog.Info("exec DNS create",
		"command", s.createCmd,
		"domain", domain)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.createCmd, domain, token, keyAuth)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec create command failed: %w: %s", err, string(output))
	}

	slog.Info("exec DNS create succeeded", "domain", domain)
	return nil
}

func (s *execSolver) CleanUp(ctx context.Context, domain, token, keyAuth string) error {
	slog.Info("exec DNS cleanup",
		"command", s.cleanupCmd,
		"domain", domain)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.cleanupCmd, domain, token, keyAuth)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exec cleanup command failed: %w: %s", err, string(output))
	}

	slog.Info("exec DNS cleanup succeeded", "domain", domain)
	return nil
}
